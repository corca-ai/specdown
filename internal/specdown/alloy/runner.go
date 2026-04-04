package alloy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/corca-ai/specdown/internal/specdown/core"
)

const (
	alloyVersion = "6.2.0"
	alloyJarName = "org.alloytools.alloy.dist.jar"
)

var alloyJarURL = "https://github.com/AlloyTools/org.alloytools.alloy/releases/download/v" + alloyVersion + "/" + alloyJarName

type Runner struct {
	BaseDir    string
	JarPath    string // user-provided JAR path; empty means auto-download
	HTTPClient *http.Client
}

type modelBundle struct {
	Model                 string
	RelativePath          string
	AbsolutePath          string
	SourceMapRelativePath string
	SourceMapAbsolutePath string
	Source                string
	LineRefs              []string
}

type sourceMapArtifact struct {
	BundlePath string              `json:"bundlePath"`
	Lines      []sourceMapLineItem `json:"lines"`
}

type sourceMapLineItem struct {
	Number    int    `json:"number"`
	SourceRef string `json:"sourceRef,omitempty"`
}

type failureLocation struct {
	BundleLine int
	SourceRef  string
}

type receipt struct {
	Commands map[string]receiptCommand `json:"commands"`
}

type receiptCommand struct {
	Type     string            `json:"type"`
	Source   string            `json:"source"`
	Scopes   json.RawMessage   `json:"scopes"`
	Solution []receiptSolution `json:"solution"`
}

type receiptSolution struct {
	Instances []json.RawMessage `json:"instances"`
}

// ExploreResult holds the instance-level output of a single Alloy command
// within a model, as opposed to the pass/fail CaseResult used by RunDocument.
type ExploreResult struct {
	Model   string
	Command string // e.g. "run sanityCheck for 5" or "check noOrphans for 5"
	IsRun   bool
	Ok      bool   // true if the command succeeded (run found instances, check found no counterexample)
	Summary string // human-readable instance or counterexample text
}

// ExploreDocument runs all Alloy models in a document plan and returns
// instance-level results. Unlike RunDocument, it does not produce CaseResults
// and does not require alloy cases in the plan — it discovers run/check
// commands directly from the model fragments.
func (r Runner) ExploreDocument(plan core.DocumentPlan) ([]ExploreResult, error) {
	if len(plan.AlloyModels) == 0 {
		return nil, nil
	}

	javaPath, _ := exec.LookPath("java")
	if javaPath == "" {
		return nil, fmt.Errorf("java not found in PATH; install a JRE to run Alloy models")
	}

	jarPath, err := r.ensureAlloyJar()
	if err != nil {
		return nil, err
	}

	var results []ExploreResult
	for _, model := range plan.AlloyModels {
		modelResults, err := r.exploreModel(javaPath, jarPath, plan.Document.RelativeTo, model)
		if err != nil {
			return nil, err
		}
		results = append(results, modelResults...)
	}
	return results, nil
}

func (r Runner) exploreModel(javaPath, jarPath, documentPath string, model core.AlloyModelSpec) ([]ExploreResult, error) {
	bundle, err := r.writeBundle(documentPath, model, nil)
	if err != nil {
		return nil, err
	}

	outputDir := filepath.Join(filepath.Dir(bundle.AbsolutePath), core.Slug(bundle.Model)+"-output")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return nil, fmt.Errorf("create alloy output dir: %w", err)
	}

	cmd := exec.Command(javaPath, "-jar", jarPath, "exec", "-f", "-o", outputDir, bundle.AbsolutePath)
	cmd.Dir = r.BaseDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		location, ok := locateAlloyFailure(bundle.LineRefs, message)
		return nil, fmt.Errorf("alloy error in model %q: %s", model.Name, annotateAlloyFailure(message, location, ok))
	}

	commandResults, err := parseReceipt(filepath.Join(outputDir, "receipt.json"))
	if err != nil {
		return nil, err
	}

	var results []ExploreResult
	for source, rcmd := range commandResults {
		results = append(results, evaluateExplore(model.Name, source, rcmd))
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Command < results[j].Command
	})
	return results, nil
}

func evaluateExplore(modelName, source string, command receiptCommand) ExploreResult {
	isRun := command.Type == "run"
	hasInstances := len(command.Solution) > 0 && len(command.Solution[0].Instances) > 0

	if isRun {
		if hasInstances {
			summary := summarizeInstance(command)
			return ExploreResult{
				Model:   modelName,
				Command: source,
				IsRun:   true,
				Ok:      true,
				Summary: summary,
			}
		}
		return ExploreResult{
			Model:   modelName,
			Command: source,
			IsRun:   true,
			Ok:      false,
			Summary: "no instances found — model may be inconsistent",
		}
	}

	// check command
	if !hasInstances {
		return ExploreResult{
			Model:   modelName,
			Command: source,
			IsRun:   false,
			Ok:      true,
			Summary: "no counterexample — assertion holds within scope",
		}
	}
	summary := summarizeInstance(command)
	return ExploreResult{
		Model:   modelName,
		Command: source,
		IsRun:   false,
		Ok:      false,
		Summary: "counterexample found:\n" + summary,
	}
}

// summarizeInstance pretty-prints the values from the first instance
// of a receipt command as indented JSON.
func summarizeInstance(command receiptCommand) string {
	if len(command.Solution) == 0 || len(command.Solution[0].Instances) == 0 {
		return "(no instances)"
	}

	// Extract just the "values" field to avoid metadata noise.
	var wrapper struct {
		Values json.RawMessage `json:"values"`
	}
	if err := json.Unmarshal(command.Solution[0].Instances[0], &wrapper); err != nil {
		return "(unable to parse instance: " + err.Error() + ")"
	}
	if wrapper.Values == nil {
		return "(no instance data)"
	}

	var buf bytes.Buffer
	if err := json.Indent(&buf, wrapper.Values, "", "  "); err != nil {
		return string(wrapper.Values)
	}
	return buf.String()
}


func (r Runner) DumpModels(plan core.DocumentPlan) ([]string, error) {
	if len(plan.AlloyModels) == 0 {
		return nil, nil
	}
	var paths []string
	for _, model := range plan.AlloyModels {
		bundle, err := r.writeBundle(plan.Document.RelativeTo, model, nil)
		if err != nil {
			return nil, err
		}
		paths = append(paths, bundle.AbsolutePath)
	}
	return paths, nil
}

func (r Runner) RunDocument(plan core.DocumentPlan) ([]core.CaseResult, error) {
	alloyChecks := filterAlloyCases(plan.Cases)
	if len(plan.AlloyModels) == 0 || len(alloyChecks) == 0 {
		return nil, nil
	}

	javaPath, _ := exec.LookPath("java")
	if javaPath == "" {
		return failedChecksAll(alloyChecks, "java not found in PATH; install a JRE to run Alloy checks"), nil
	}

	jarPath, err := r.ensureAlloyJar()
	if err != nil {
		return nil, err
	}

	resultsByKey, err := r.runAllModels(plan, alloyChecks, javaPath, jarPath)
	if err != nil {
		return nil, err
	}

	return collectOrderedResults(alloyChecks, resultsByKey)
}

func filterAlloyCases(cases []core.CaseSpec) []core.CaseSpec {
	var result []core.CaseSpec
	for i := range cases {
		if cases[i].Alloy != nil {
			result = append(result, cases[i])
		}
	}
	return result
}

func (r Runner) runAllModels(plan core.DocumentPlan, alloyChecks []core.CaseSpec, javaPath, jarPath string) (map[string]core.CaseResult, error) {
	checksByModel := make(map[string][]core.CaseSpec)
	for i := range alloyChecks {
		checksByModel[alloyChecks[i].Alloy.Model] = append(checksByModel[alloyChecks[i].Alloy.Model], alloyChecks[i])
	}

	resultsByKey := make(map[string]core.CaseResult, len(alloyChecks))
	for _, model := range plan.AlloyModels {
		checks := checksByModel[model.Name]
		if len(checks) == 0 {
			continue
		}

		bundle, err := r.writeBundle(plan.Document.RelativeTo, model, checks)
		if err != nil {
			return nil, err
		}

		modelResults, err := r.runModel(javaPath, jarPath, bundle, checks)
		if err != nil {
			return nil, err
		}
		for j := range modelResults {
			resultsByKey[modelResults[j].ID.Key()] = modelResults[j]
		}
	}
	return resultsByKey, nil
}

func collectOrderedResults(checks []core.CaseSpec, resultsByKey map[string]core.CaseResult) ([]core.CaseResult, error) {
	results := make([]core.CaseResult, 0, len(checks))
	for i := range checks {
		result, ok := resultsByKey[checks[i].ID.Key()]
		if !ok {
			return nil, fmt.Errorf("missing alloy result for %s", checks[i].ID.Key())
		}
		results = append(results, result)
	}
	return results, nil
}

func (r Runner) writeBundle(documentPath string, model core.AlloyModelSpec, checks []core.CaseSpec) (modelBundle, error) {
	relativePath := filepath.ToSlash(filepath.Join(".artifacts", "specdown", "models", bundleFileName(documentPath, model.Name)))
	absolutePath := filepath.Join(r.BaseDir, filepath.FromSlash(relativePath))
	if err := os.MkdirAll(filepath.Dir(absolutePath), 0o755); err != nil {
		return modelBundle{}, fmt.Errorf("create alloy artifact dir: %w", err)
	}

	source, lineRefs := buildBundleSource(documentPath, model, checks)
	if err := os.WriteFile(absolutePath, []byte(source), 0o644); err != nil {
		return modelBundle{}, fmt.Errorf("write alloy bundle: %w", err)
	}

	sourceMapRelativePath := relativePath + ".map.json"
	sourceMapAbsolutePath := absolutePath + ".map.json"
	if err := writeSourceMap(sourceMapAbsolutePath, relativePath, lineRefs); err != nil {
		return modelBundle{}, err
	}

	return modelBundle{
		Model:                 model.Name,
		RelativePath:          relativePath,
		AbsolutePath:          absolutePath,
		SourceMapRelativePath: sourceMapRelativePath,
		SourceMapAbsolutePath: sourceMapAbsolutePath,
		Source:                source,
		LineRefs:              lineRefs,
	}, nil
}

func buildBundleSource(documentPath string, model core.AlloyModelSpec, checks []core.CaseSpec) (source string, lineRefs []string) {
	var (
		lines     []string
		seenCheck = make(map[string]struct{})
	)

	appendLine := func(line string, sourceRef string) {
		lines = append(lines, line)
		lineRefs = append(lineRefs, sourceRef)
	}

	for _, fragment := range model.Fragments {
		sourceRef := formatSourceRef(documentPath, fragment.HeadingPath)
		appendLine("-- specdown-source: "+sourceRef, sourceRef)
		for _, line := range splitBundleLines(fragment.Source) {
			appendLine(line, sourceRef)
		}
		appendLine("", sourceRef)
	}

	appendedHeader := false
	for i := range checks {
		command := checkCommandSource(checks[i])
		if _, ok := seenCheck[command]; ok {
			continue
		}
		seenCheck[command] = struct{}{}

		if !bundleContainsCommand(lines, command) {
			if !appendedHeader {
				appendLine("-- specdown-generated-checks", "")
				appendedHeader = true
			}
			appendLine(command, formatSourceRef(checks[i].ID.File, checks[i].ID.HeadingPath))
		}
	}

	source = strings.Join(lines, "\n") + "\n"
	return
}

func (r Runner) runModel(javaPath, jarPath string, bundle modelBundle, checks []core.CaseSpec) ([]core.CaseResult, error) {
	outputDir := filepath.Join(filepath.Dir(bundle.AbsolutePath), core.Slug(bundle.Model)+"-output")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return nil, fmt.Errorf("create alloy output dir: %w", err)
	}

	cmd := exec.Command(javaPath, "-jar", jarPath, "exec", "-f", "-o", outputDir, bundle.AbsolutePath)
	cmd.Dir = r.BaseDir
	start := time.Now()
	output, err := cmd.CombinedOutput()
	durationMs := int(time.Since(start).Milliseconds())
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		location, ok := locateAlloyFailure(bundle.LineRefs, message)
		failed := failedChecks(checks, bundle.RelativePath, bundle.SourceMapRelativePath, annotateAlloyFailure(message, location, ok), location, ok)
		for i := range failed {
			failed[i].DurationMs = durationMs
		}
		return failed, nil
	}

	commandResults, err := parseReceipt(filepath.Join(outputDir, "receipt.json"))
	if err != nil {
		return nil, err
	}

	results := make([]core.CaseResult, 0, len(checks))
	for i := range checks {
		result, err := r.evaluateCheck(checks[i], bundle, commandResults)
		if err != nil {
			return nil, err
		}
		result.DurationMs = durationMs
		results = append(results, result)
	}

	return results, nil
}

func parseReceipt(receiptPath string) (map[string]receiptCommand, error) {
	receiptBody, err := os.ReadFile(receiptPath)
	if err != nil {
		return nil, fmt.Errorf("read alloy receipt: %w", err)
	}

	var runReceipt receipt
	if err := json.Unmarshal(receiptBody, &runReceipt); err != nil {
		return nil, fmt.Errorf("decode alloy receipt: %w", err)
	}

	commandResults := make(map[string]receiptCommand)
	for _, command := range runReceipt.Commands {
		commandResults[strings.TrimSpace(command.Source)] = command
	}
	return commandResults, nil
}

func (r Runner) evaluateCheck(check core.CaseSpec, bundle modelBundle, commandResults map[string]receiptCommand) (core.CaseResult, error) {
	base := baseCheckResult(check, bundle)

	command, ok := lookupCommand(commandResults, check)
	if !ok {
		base.Status = core.StatusFailed
		base.Message = "missing Alloy result for " + strconvQuote(checkCommandSource(check))
		return base, nil
	}

	if check.Alloy.IsRun {
		return evaluateRun(base, check, command), nil
	}

	if len(command.Solution) == 0 {
		base.Status = core.StatusPassed
		return base, nil
	}

	counterexamplePath, err := writeCounterexample(r.BaseDir, check, command)
	if err != nil {
		return core.CaseResult{}, err
	}
	summary := summarizeInstance(command)
	message := "counterexample for " + strconvQuote(check.Alloy.Assertion)
	if summary != "" {
		message += "\n" + summary
	}
	base.Status = core.StatusFailed
	base.Message = message
	base.Alloy.CounterexamplePath = counterexamplePath
	return base, nil
}

func evaluateRun(base core.CaseResult, check core.CaseSpec, command receiptCommand) core.CaseResult {
	hasInstances := len(command.Solution) > 0 && len(command.Solution[0].Instances) > 0
	if hasInstances {
		base.Status = core.StatusPassed
	} else {
		base.Status = core.StatusFailed
		base.Message = "no instances found for " + strconvQuote(check.Alloy.Assertion) + " — model may be unsatisfiable"
	}
	return base
}

func baseCheckResult(check core.CaseSpec, bundle modelBundle) core.CaseResult {
	a := check.Alloy
	return core.CaseResult{
		ID:    check.ID,
		Kind:  core.CaseKindAlloy,
		Label: check.DefaultLabel(),
		Alloy: &core.AlloyResultDetail{
			Model:         a.Model,
			Assertion:     a.Assertion,
			Scope:         a.Scope,
			BundlePath:    bundle.RelativePath,
			SourceMapPath: bundle.SourceMapRelativePath,
			SourceRef:     formatSourceRef(check.ID.File, check.ID.HeadingPath),
		},
	}
}

func failedChecksAll(checks []core.CaseSpec, message string) []core.CaseResult {
	results := make([]core.CaseResult, 0, len(checks))
	for i := range checks {
		a := checks[i].Alloy
		result := core.CaseResult{
			ID:      checks[i].ID,
			Kind:    core.CaseKindAlloy,
			Label:   checks[i].DefaultLabel(),
			Status:  core.StatusFailed,
			Message: message,
			Alloy: &core.AlloyResultDetail{
				Model:     a.Model,
				Assertion: a.Assertion,
				Scope:     a.Scope,
			},
		}
		results = append(results, result)
	}
	return results
}

func failedChecks(checks []core.CaseSpec, bundlePath, sourceMapPath, message string, location failureLocation, hasLocation bool) []core.CaseResult {
	results := make([]core.CaseResult, 0, len(checks))
	for i := range checks {
		a := checks[i].Alloy
		detail := &core.AlloyResultDetail{
			Model:         a.Model,
			Assertion:     a.Assertion,
			Scope:         a.Scope,
			BundlePath:    bundlePath,
			SourceMapPath: sourceMapPath,
		}
		if hasLocation {
			detail.BundleLine = location.BundleLine
			detail.SourceRef = location.SourceRef
		}
		result := core.CaseResult{
			ID:      checks[i].ID,
			Kind:    core.CaseKindAlloy,
			Label:   checks[i].DefaultLabel(),
			Status:  core.StatusFailed,
			Message: message,
			Alloy:   detail,
		}
		results = append(results, result)
	}
	return results
}

func writeCounterexample(baseDir string, check core.CaseSpec, command receiptCommand) (string, error) {
	relativePath := filepath.ToSlash(filepath.Join(".artifacts", "specdown", "counterexamples", check.ID.Anchor()+".json"))
	absolutePath := filepath.Join(baseDir, filepath.FromSlash(relativePath))
	if err := os.MkdirAll(filepath.Dir(absolutePath), 0o755); err != nil {
		return "", fmt.Errorf("create counterexample dir: %w", err)
	}

	body, err := json.MarshalIndent(command, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode counterexample: %w", err)
	}
	if err := os.WriteFile(absolutePath, body, 0o644); err != nil {
		return "", fmt.Errorf("write counterexample: %w", err)
	}
	return relativePath, nil
}



func alloyCacheDir() (string, error) {
	cacheDir := os.Getenv("XDG_CACHE_HOME")
	if cacheDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("determine home directory: %w", err)
		}
		cacheDir = filepath.Join(home, ".cache")
	}
	return filepath.Join(cacheDir, "specdown"), nil
}

func (r Runner) ensureAlloyJar() (string, error) {
	if r.JarPath != "" {
		if _, err := os.Stat(r.JarPath); err != nil {
			return "", fmt.Errorf("configured Alloy JAR not found: %w", err)
		}
		return r.JarPath, nil
	}
	return r.downloadAlloyJar()
}

func (r Runner) downloadAlloyJar() (_ string, err error) {
	cacheDir, err := alloyCacheDir()
	if err != nil {
		return "", err
	}
	jarPath := filepath.Join(cacheDir, alloyJarName)
	if _, err := os.Stat(jarPath); err == nil {
		return jarPath, nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("stat alloy jar: %w", err)
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return "", fmt.Errorf("create cache dir: %w", err)
	}

	client := r.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}

	response, err := client.Get(alloyJarURL)
	if err != nil {
		return "", fmt.Errorf("download alloy jar: %w", err)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download alloy jar: unexpected status %s", response.Status)
	}

	tmp, err := os.CreateTemp(cacheDir, alloyJarName+".*.tmp")
	if err != nil {
		return "", fmt.Errorf("create temp file for alloy jar: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() {
		if err != nil {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err = io.Copy(tmp, response.Body); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("write alloy jar: %w", err)
	}
	if err = tmp.Close(); err != nil {
		return "", fmt.Errorf("close alloy jar: %w", err)
	}
	if err = os.Rename(tmpPath, jarPath); err != nil {
		return "", fmt.Errorf("rename alloy jar: %w", err)
	}
	return jarPath, nil
}

func bundleFileName(documentPath, modelName string) string {
	return core.Slug(documentPath) + "-" + core.Slug(modelName) + ".als"
}

func splitBundleLines(source string) []string {
	if source == "" {
		return []string{""}
	}
	return strings.Split(strings.ReplaceAll(source, "\r\n", "\n"), "\n")
}

func writeSourceMap(outPath, bundlePath string, lineRefs []string) error {
	items := make([]sourceMapLineItem, 0, len(lineRefs))
	for i, sourceRef := range lineRefs {
		items = append(items, sourceMapLineItem{
			Number:    i + 1,
			SourceRef: sourceRef,
		})
	}

	body, err := json.MarshalIndent(sourceMapArtifact{
		BundlePath: bundlePath,
		Lines:      items,
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode alloy source map: %w", err)
	}
	if err := os.WriteFile(outPath, body, 0o644); err != nil {
		return fmt.Errorf("write alloy source map: %w", err)
	}
	return nil
}

func formatSourceRef(documentPath string, headingPath []string) string {
	if len(headingPath) == 0 {
		return documentPath
	}
	return documentPath + "#" + strings.Join(headingPath, "/")
}

// bundleCommandPattern matches "check Name for ..." or "run Name { ... } for ..."
// and captures the keyword ("check"/"run") and assertion name.
var bundleCommandPattern = regexp.MustCompile(`^\s*(check|run)\s+([A-Za-z_][A-Za-z0-9_]*)`)

func bundleContainsCommand(lines []string, command string) bool {
	// Extract keyword and assertion name from the target command.
	m := bundleCommandPattern.FindStringSubmatch(command)
	if m == nil {
		return false
	}
	keyword, name := m[1], m[2]

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == command {
			return true
		}
		// For run commands with inline bodies: "run Name { ... } for scope"
		// matches the simplified "run Name for scope".
		lm := bundleCommandPattern.FindStringSubmatch(trimmed)
		if len(lm) == 3 && lm[1] == keyword && lm[2] == name {
			return true
		}
	}
	return false
}

// lookupCommand finds the receipt entry for a check. It first tries an exact
// match on the simplified command source. If that fails (e.g. for run commands
// with inline predicate bodies), it falls back to matching by keyword+name.
func lookupCommand(results map[string]receiptCommand, check core.CaseSpec) (receiptCommand, bool) {
	exact := checkCommandSource(check)
	if cmd, ok := results[exact]; ok {
		return cmd, true
	}
	m := bundleCommandPattern.FindStringSubmatch(exact)
	if m == nil {
		return receiptCommand{}, false
	}
	keyword, name := m[1], m[2]
	for source, cmd := range results {
		lm := bundleCommandPattern.FindStringSubmatch(source)
		if len(lm) == 3 && lm[1] == keyword && lm[2] == name {
			return cmd, true
		}
	}
	return receiptCommand{}, false
}

func checkCommandSource(check core.CaseSpec) string {
	if check.Alloy.IsRun {
		return "run " + check.Alloy.Assertion + " for " + check.Alloy.Scope
	}
	return "check " + check.Alloy.Assertion + " for " + check.Alloy.Scope
}

var alloyLinePattern = regexp.MustCompile(`\bline\s+(\d+)\b`)

func locateAlloyFailure(lineRefs []string, message string) (failureLocation, bool) {
	match := alloyLinePattern.FindStringSubmatch(message)
	if len(match) != 2 {
		return failureLocation{}, false
	}

	lineNumber := 0
	for _, r := range match[1] {
		lineNumber = lineNumber*10 + int(r-'0')
	}
	if lineNumber <= 0 || lineNumber > len(lineRefs) {
		return failureLocation{}, false
	}
	sourceRef := lineRefs[lineNumber-1]
	if sourceRef == "" {
		return failureLocation{}, false
	}
	return failureLocation{
		BundleLine: lineNumber,
		SourceRef:  sourceRef,
	}, true
}

func annotateAlloyFailure(message string, location failureLocation, hasLocation bool) string {
	if !hasLocation {
		return message
	}
	return message + " (bundle line " + strconv.Itoa(location.BundleLine) + ", source: " + location.SourceRef + ")"
}

func strconvQuote(value string) string {
	body, _ := json.Marshal(value)
	return string(body)
}
