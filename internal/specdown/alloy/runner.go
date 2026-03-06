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
	Model        string
	RelativePath string
	AbsolutePath string
	Source       string
	LineRefs     []string
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

func (r Runner) RunDocument(plan core.DocumentPlan) ([]core.AlloyCheckResult, error) {
	if len(plan.AlloyModels) == 0 || len(plan.AlloyChecks) == 0 {
		return nil, nil
	}

	javaPath, err := exec.LookPath("java")
	if err != nil {
		return nil, fmt.Errorf("find java: %w", err)
	}

	jarPath, err := r.ensureAlloyJar()
	if err != nil {
		return nil, err
	}

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

	results := make([]core.AlloyCheckResult, 0, len(plan.AlloyChecks))
	for _, check := range plan.AlloyChecks {
		result, ok := resultsByKey[check.ID.Key()]
		if !ok {
			return nil, fmt.Errorf("missing alloy result for %s", check.ID.Key())
		}
		results = append(results, result)
	}
	return results, nil
}

func (r Runner) writeBundle(documentPath string, model core.AlloyModelSpec, checks []core.AlloyCheckSpec) (modelBundle, error) {
	relativePath := filepath.ToSlash(filepath.Join(".artifacts", "specdown", "alloy", bundleFileName(documentPath, model.Name)))
	absolutePath := filepath.Join(r.BaseDir, filepath.FromSlash(relativePath))
	if err := os.MkdirAll(filepath.Dir(absolutePath), 0o755); err != nil {
		return modelBundle{}, fmt.Errorf("create alloy artifact dir: %w", err)
	}

	source, lineRefs := buildBundleSource(documentPath, model, checks)
	if err := os.WriteFile(absolutePath, []byte(source), 0o644); err != nil {
		return modelBundle{}, fmt.Errorf("write alloy bundle: %w", err)
	}

	return modelBundle{
		Model:        model.Name,
		RelativePath: relativePath,
		AbsolutePath: absolutePath,
		Source:       source,
		LineRefs:     lineRefs,
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
		return failedChecks(checks, bundle.AbsolutePath, annotateAlloyFailure(message, bundle.LineRefs)), nil
	}

	receiptPath := filepath.Join(outputDir, "receipt.json")
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

	results := make([]core.AlloyCheckResult, 0, len(checks))
	for _, check := range checks {
		commandSource := checkCommandSource(check)
		command, ok := commandResults[commandSource]
		if !ok {
			results = append(results, core.AlloyCheckResult{
				ID:         check.ID,
				Model:      check.Model,
				Assertion:  check.Assertion,
				Scope:      check.Scope,
				Label:      defaultLabel(check),
				Status:     core.StatusFailed,
				Message:    "missing Alloy result for " + strconvQuote(commandSource),
				Expected:   "Alloy command " + strconvQuote(commandSource),
				Actual:     "no matching receipt command",
				BundlePath: bundle.AbsolutePath,
			})
			continue
		}

		if len(command.Solution) == 0 {
			results = append(results, core.AlloyCheckResult{
				ID:         check.ID,
				Model:      check.Model,
				Assertion:  check.Assertion,
				Scope:      check.Scope,
				Label:      defaultLabel(check),
				Status:     core.StatusPassed,
				BundlePath: bundle.AbsolutePath,
			})
			continue
		}

		counterexamplePath, err := writeCounterexample(r.BaseDir, check, command)
		if err != nil {
			return nil, err
		}
		results = append(results, core.AlloyCheckResult{
			ID:                 check.ID,
			Model:              check.Model,
			Assertion:          check.Assertion,
			Scope:              check.Scope,
			Label:              defaultLabel(check),
			Status:             core.StatusFailed,
			Message:            "found counterexample for assertion " + strconvQuote(check.Assertion) + " at scope " + check.Scope,
			Expected:           "assertion " + strconvQuote(check.Assertion) + " holds for scope " + check.Scope,
			Actual:             "counterexample found",
			BundlePath:         bundle.RelativePath,
			CounterexamplePath: counterexamplePath,
		})
	}

	return results, nil
}

func failedChecks(checks []core.AlloyCheckSpec, bundlePath string, message string) []core.AlloyCheckResult {
	results := make([]core.AlloyCheckResult, 0, len(checks))
	for _, check := range checks {
		results = append(results, core.AlloyCheckResult{
			ID:         check.ID,
			Model:      check.Model,
			Assertion:  check.Assertion,
			Scope:      check.Scope,
			Label:      defaultLabel(check),
			Status:     core.StatusFailed,
			Message:    message,
			Expected:   "Alloy check " + strconvQuote(checkCommandSource(check)) + " succeeds",
			Actual:     "Alloy execution error",
			BundlePath: bundlePath,
		})
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

func (r Runner) ensureAlloyJar() (string, error) {
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
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download alloy jar: unexpected status %s", response.Status)
	}

	file, err := os.Create(jarPath)
	if err != nil {
		return "", fmt.Errorf("create alloy jar: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(file, response.Body); err != nil {
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

func defaultLabel(check core.AlloyCheckSpec) string {
	suffix := "alloy:ref(" + check.Model + "#" + check.Assertion + ", scope=" + check.Scope + ")"
	if len(check.ID.HeadingPath) == 0 {
		return suffix
	}
	return suffix + " @ " + check.ID.HeadingPath[len(check.ID.HeadingPath)-1]
}

var alloyLinePattern = regexp.MustCompile(`\bline\s+([0-9]+)\b`)

func annotateAlloyFailure(message string, lineRefs []string) string {
	match := alloyLinePattern.FindStringSubmatch(message)
	if len(match) != 2 {
		return message
	}

	lineNumber := 0
	for _, r := range match[1] {
		lineNumber = lineNumber*10 + int(r-'0')
	}
	if lineNumber <= 0 || lineNumber > len(lineRefs) {
		return message
	}
	sourceRef := lineRefs[lineNumber-1]
	if sourceRef == "" {
		return message
	}
	return message + " (source: " + sourceRef + ")"
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
