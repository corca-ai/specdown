package alloy

import (
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

	"specdown/internal/specdown/core"
)

const (
	alloyVersion = "6.2.0"
	alloyJarName = "org.alloytools.alloy.dist.jar"
)

var alloyJarURL = "https://github.com/AlloyTools/org.alloytools.alloy/releases/download/v" + alloyVersion + "/" + alloyJarName

type DocumentRunner interface {
	RunDocument(plan core.DocumentPlan) ([]core.AlloyCheckResult, error)
}

type Runner struct {
	BaseDir    string
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
	Scopes   map[string]string `json:"scopes"`
	Solution []receiptSolution `json:"solution"`
}

type receiptSolution struct {
	Instances []json.RawMessage `json:"instances"`
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

func (r Runner) RunDocument(plan core.DocumentPlan) ([]core.AlloyCheckResult, error) {
	if len(plan.AlloyModels) == 0 || len(plan.AlloyChecks) == 0 {
		return nil, nil
	}

	javaPath, _ := exec.LookPath("java")
	if javaPath == "" {
		return failedChecksAll(plan.AlloyChecks, "java not found in PATH; install a JRE to run Alloy checks"), nil
	}

	jarPath, err := r.ensureAlloyJar()
	if err != nil {
		return nil, err
	}

	resultsByKey, err := r.runAllModels(plan, javaPath, jarPath)
	if err != nil {
		return nil, err
	}

	return collectOrderedResults(plan.AlloyChecks, resultsByKey)
}

func (r Runner) runAllModels(plan core.DocumentPlan, javaPath string, jarPath string) (map[string]core.AlloyCheckResult, error) {
	checksByModel := make(map[string][]core.AlloyCheckSpec)
	for _, check := range plan.AlloyChecks {
		checksByModel[check.Model] = append(checksByModel[check.Model], check)
	}

	resultsByKey := make(map[string]core.AlloyCheckResult, len(plan.AlloyChecks))
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
		for _, result := range modelResults {
			resultsByKey[result.ID.Key()] = result
		}
	}
	return resultsByKey, nil
}

func collectOrderedResults(checks []core.AlloyCheckSpec, resultsByKey map[string]core.AlloyCheckResult) ([]core.AlloyCheckResult, error) {
	results := make([]core.AlloyCheckResult, 0, len(checks))
	for _, check := range checks {
		result, ok := resultsByKey[check.ID.Key()]
		if !ok {
			return nil, fmt.Errorf("missing alloy result for %s", check.ID.Key())
		}
		results = append(results, result)
	}
	return results, nil
}

func (r Runner) writeBundle(documentPath string, model core.AlloyModelSpec, checks []core.AlloyCheckSpec) (modelBundle, error) {
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

func buildBundleSource(documentPath string, model core.AlloyModelSpec, checks []core.AlloyCheckSpec) (string, []string) {
	var (
		lines     []string
		lineRefs  []string
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
	for _, check := range checks {
		command := checkCommandSource(check)
		if _, ok := seenCheck[command]; ok {
			continue
		}
		seenCheck[command] = struct{}{}

		if !bundleContainsCommand(lines, command) {
			if !appendedHeader {
				appendLine("-- specdown-generated-checks", "")
				appendedHeader = true
			}
			appendLine(command, formatSourceRef(check.ID.File, check.ID.HeadingPath))
		}
	}

	return strings.Join(lines, "\n") + "\n", lineRefs
}

func (r Runner) runModel(javaPath string, jarPath string, bundle modelBundle, checks []core.AlloyCheckSpec) ([]core.AlloyCheckResult, error) {
	outputDir := filepath.Join(filepath.Dir(bundle.AbsolutePath), slug(bundle.Model)+"-output")
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
		return failedChecks(checks, bundle.AbsolutePath, bundle.SourceMapAbsolutePath, annotateAlloyFailure(message, location, ok), location, ok), nil
	}

	commandResults, err := parseReceipt(filepath.Join(outputDir, "receipt.json"))
	if err != nil {
		return nil, err
	}

	results := make([]core.AlloyCheckResult, 0, len(checks))
	for _, check := range checks {
		result, err := r.evaluateCheck(check, bundle, commandResults)
		if err != nil {
			return nil, err
		}
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

func (r Runner) evaluateCheck(check core.AlloyCheckSpec, bundle modelBundle, commandResults map[string]receiptCommand) (core.AlloyCheckResult, error) {
	base := baseCheckResult(check, bundle)

	commandSource := checkCommandSource(check)
	command, ok := commandResults[commandSource]
	if !ok {
		base.Status = core.StatusFailed
		base.Message = "missing Alloy result for " + strconvQuote(commandSource)
		return base, nil
	}

	if len(command.Solution) == 0 {
		base.Status = core.StatusPassed
		return base, nil
	}

	counterexamplePath, err := writeCounterexample(r.BaseDir, check, command)
	if err != nil {
		return core.AlloyCheckResult{}, err
	}
	summary := summarizeCounterexample(command)
	message := "counterexample for " + strconvQuote(check.Assertion)
	if summary != "" && summary != "counterexample found" {
		message += "\n" + summary
	}
	base.Status = core.StatusFailed
	base.Message = message
	base.CounterexamplePath = counterexamplePath
	return base, nil
}

func baseCheckResult(check core.AlloyCheckSpec, bundle modelBundle) core.AlloyCheckResult {
	return core.AlloyCheckResult{
		ID:            check.ID,
		Model:         check.Model,
		Assertion:     check.Assertion,
		Scope:         check.Scope,
		Label:         check.DefaultLabel(),
		BundlePath:    bundle.AbsolutePath,
		SourceMapPath: bundle.SourceMapAbsolutePath,
		SourceRef:     formatSourceRef(check.ID.File, check.ID.HeadingPath),
	}
}

func failedChecksAll(checks []core.AlloyCheckSpec, message string) []core.AlloyCheckResult {
	results := make([]core.AlloyCheckResult, 0, len(checks))
	for _, check := range checks {
		result := core.AlloyCheckResult{
			ID:        check.ID,
			Model:     check.Model,
			Assertion: check.Assertion,
			Scope:     check.Scope,
			Label:     check.DefaultLabel(),
			Status:    core.StatusFailed,
			Message:   message,
		}
		results = append(results, result)
	}
	return results
}

func failedChecks(checks []core.AlloyCheckSpec, bundlePath string, sourceMapPath string, message string, location failureLocation, hasLocation bool) []core.AlloyCheckResult {
	results := make([]core.AlloyCheckResult, 0, len(checks))
	for _, check := range checks {
		result := core.AlloyCheckResult{
			ID:            check.ID,
			Model:         check.Model,
			Assertion:     check.Assertion,
			Scope:         check.Scope,
			Label:         check.DefaultLabel(),
			Status:        core.StatusFailed,
			Message:       message,
			BundlePath:    bundlePath,
			SourceMapPath: sourceMapPath,
		}
		if hasLocation {
			result.BundleLine = location.BundleLine
			result.SourceRef = location.SourceRef
		}
		results = append(results, result)
	}
	return results
}


func writeCounterexample(baseDir string, check core.AlloyCheckSpec, command receiptCommand) (string, error) {
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
	return absolutePath, nil
}

func summarizeCounterexample(command receiptCommand) string {
	if len(command.Solution) == 0 {
		return "counterexample found"
	}
	solution := command.Solution[0]
	if len(solution.Instances) == 0 {
		return "counterexample found"
	}

	var instance struct {
		Values map[string]map[string][][]string `json:"values"`
	}
	if err := json.Unmarshal(solution.Instances[0], &instance); err != nil {
		return "counterexample found"
	}

	var lines []string
	for atom, relations := range instance.Values {
		if len(relations) == 0 {
			continue
		}
		for rel, tuples := range relations {
			for _, tuple := range tuples {
				lines = append(lines, atom+"."+rel+" = "+strings.Join(tuple, ", "))
			}
		}
	}

	if len(lines) == 0 {
		return "counterexample found"
	}

	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

func (r Runner) ensureAlloyJar() (_ string, err error) {
	jarPath := filepath.Join(r.BaseDir, alloyJarName)
	if _, err := os.Stat(jarPath); err == nil {
		return jarPath, nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("stat alloy jar: %w", err)
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

	file, err := os.Create(jarPath)
	if err != nil {
		return "", fmt.Errorf("create alloy jar: %w", err)
	}
	defer func() {
		if cerr := file.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("close alloy jar: %w", cerr)
		}
	}()

	if _, err = io.Copy(file, response.Body); err != nil {
		return "", fmt.Errorf("write alloy jar: %w", err)
	}
	return jarPath, nil
}

func bundleFileName(documentPath string, modelName string) string {
	return slug(documentPath) + "-" + slug(modelName) + ".als"
}

func splitBundleLines(source string) []string {
	if source == "" {
		return []string{""}
	}
	return strings.Split(strings.ReplaceAll(source, "\r\n", "\n"), "\n")
}

func writeSourceMap(outPath string, bundlePath string, lineRefs []string) error {
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

func bundleContainsCommand(lines []string, command string) bool {
	for _, line := range lines {
		if strings.TrimSpace(line) == command {
			return true
		}
	}
	return false
}

func checkCommandSource(check core.AlloyCheckSpec) string {
	return "check " + check.Assertion + " for " + check.Scope
}


var alloyLinePattern = regexp.MustCompile(`\bline\s+([0-9]+)\b`)

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

func slug(input string) string {
	input = strings.ToLower(input)
	var out strings.Builder
	lastDash := false
	for _, r := range input {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			out.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			out.WriteByte('-')
			lastDash = true
		}
	}
	result := strings.Trim(out.String(), "-")
	if result == "" {
		return "spec"
	}
	return result
}

func strconvQuote(value string) string {
	body, _ := json.Marshal(value)
	return string(body)
}
