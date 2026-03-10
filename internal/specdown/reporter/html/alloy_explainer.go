package html

import (
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/corca-ai/specdown/internal/specdown/core"
)

type parsedAlloyCheck struct {
	Assertion string
	Scope     string
	Order     int
}

type alloyModelRender struct {
	Node             core.AlloyModelNode
	Anchor           string
	LocalChecks      []parsedAlloyCheck
	OwnedResults     []core.AlloyCheckResult
	ExactResultsByID map[string]core.AlloyCheckResult
}

type alloyRenderContext struct {
	Blocks              []alloyModelRender
	ResultsByKey        map[string]core.AlloyCheckResult
	OwnerAnchorByResult map[string]string
}

type alloyGlossSection struct {
	Title string
	Items []string
}

type counterexampleArtifact struct {
	Solution []counterexampleSolution `json:"solution"`
}

type counterexampleSolution struct {
	Instances []json.RawMessage `json:"instances"`
}

type counterexampleInstance struct {
	Skolems map[string]counterexampleSkolem  `json:"skolems"`
	Values  map[string]map[string][][]string `json:"values"`
}

type counterexampleSkolem struct {
	Data [][]string `json:"data"`
}

var (
	alloyCheckPatternLocal  = regexp.MustCompile(`(?m)^\s*check\s+([A-Za-z_][A-Za-z0-9_]*)\s+for\s+(.+?)\s*$`)
	alloySigHeaderPattern   = regexp.MustCompile(`^\s*(abstract\s+)?(?:(one|lone|some)\s+)?sig\s+([^{}]+?)\s*(?:extends\s+([^{}]+?))?\s*(?:\{.*)?$`)
	alloyBlockHeaderPattern = regexp.MustCompile(`^\s*(fact|assert)(?:\s+([A-Za-z_][A-Za-z0-9_]*))?\s*(?:\{.*)?$`)
	alloyQuantifiedPattern  = regexp.MustCompile(`^(all|some|one|no)\s+([A-Za-z_][A-Za-z0-9_]*)\s*:\s*([A-Za-z_][A-Za-z0-9_]*)\s*\|\s*(.+)$`)
	alloyOverridePattern    = regexp.MustCompile(`^(\d+)\s+([A-Za-z_][A-Za-z0-9_]*)$`)
	alloyRelationPattern    = regexp.MustCompile(`^\s*([A-Za-z0-9_$]+)\.([A-Za-z0-9_]+)\s*=\s*(.+?)\s*$`)
)

func buildAlloyRenderContext(result core.DocumentResult) alloyRenderContext {
	ctx := alloyRenderContext{
		Blocks:              collectAlloyBlocks(result.Document),
		ResultsByKey:        make(map[string]core.AlloyCheckResult, len(result.AlloyChecks)),
		OwnerAnchorByResult: make(map[string]string, len(result.AlloyChecks)),
	}
	if len(ctx.Blocks) == 0 {
		return ctx
	}

	firstBlockByModel := make(map[string]int, len(ctx.Blocks))
	resultOrder := make(map[string]int, len(result.AlloyChecks))
	for i := range ctx.Blocks {
		if _, ok := firstBlockByModel[ctx.Blocks[i].Node.Model]; !ok {
			firstBlockByModel[ctx.Blocks[i].Node.Model] = i
		}
	}

	for index, item := range result.AlloyChecks {
		key := item.ID.Key()
		ctx.ResultsByKey[key] = item
		resultOrder[key] = index

		ownerIndex := -1
		for i := range ctx.Blocks {
			if ctx.Blocks[i].Node.Model != item.Model {
				continue
			}
			if blockHasExactCheck(ctx.Blocks[i], item.Assertion, item.Scope) {
				ownerIndex = i
				break
			}
		}
		if ownerIndex < 0 {
			if fallback, ok := firstBlockByModel[item.Model]; ok {
				ownerIndex = fallback
			}
		}
		if ownerIndex < 0 {
			continue
		}

		ctx.Blocks[ownerIndex].OwnedResults = append(ctx.Blocks[ownerIndex].OwnedResults, item)
		ctx.OwnerAnchorByResult[key] = ctx.Blocks[ownerIndex].Anchor
		if blockHasExactCheck(ctx.Blocks[ownerIndex], item.Assertion, item.Scope) {
			if ctx.Blocks[ownerIndex].ExactResultsByID == nil {
				ctx.Blocks[ownerIndex].ExactResultsByID = make(map[string]core.AlloyCheckResult)
			}
			ctx.Blocks[ownerIndex].ExactResultsByID[alloyCheckKey(item.Assertion, item.Scope)] = item
		}
	}

	for i := range ctx.Blocks {
		block := &ctx.Blocks[i]
		sort.SliceStable(block.OwnedResults, func(a, b int) bool {
			left := ownedResultSortKey(*block, block.OwnedResults[a], resultOrder)
			right := ownedResultSortKey(*block, block.OwnedResults[b], resultOrder)
			if left == right {
				return resultOrder[block.OwnedResults[a].ID.Key()] < resultOrder[block.OwnedResults[b].ID.Key()]
			}
			return left < right
		})
	}

	return ctx
}

func collectAlloyBlocks(document core.Document) []alloyModelRender {
	blocks := make([]alloyModelRender, 0)
	ordinal := 0
	for _, node := range document.Nodes {
		modelNode, ok := node.(core.AlloyModelNode)
		if !ok {
			continue
		}
		ordinal++
		blocks = append(blocks, alloyModelRender{
			Node:        modelNode,
			Anchor:      alloyModelAnchor(document.RelativeTo, modelNode, ordinal),
			LocalChecks: parseAlloyChecks(modelNode.Source),
		})
	}
	return blocks
}

func alloyModelAnchor(documentPath string, node core.AlloyModelNode, ordinal int) string {
	headingPath := append([]string(nil), node.HeadingPath...)
	headingPath = append(headingPath, "alloy:model("+node.Model+")")
	return "alloy-" + strings.TrimPrefix(core.SpecID{
		File:        documentPath,
		HeadingPath: headingPath,
		Ordinal:     ordinal,
	}.Anchor(), "case-")
}

func parseAlloyChecks(source string) []parsedAlloyCheck {
	matches := alloyCheckPatternLocal.FindAllStringSubmatch(source, -1)
	checks := make([]parsedAlloyCheck, 0, len(matches))
	for i, match := range matches {
		checks = append(checks, parsedAlloyCheck{
			Assertion: match[1],
			Scope:     normalizeAlloySpace(match[2]),
			Order:     i,
		})
	}
	return checks
}

func blockHasExactCheck(block alloyModelRender, assertion, scope string) bool {
	key := alloyCheckKey(assertion, scope)
	for _, check := range block.LocalChecks {
		if alloyCheckKey(check.Assertion, check.Scope) == key {
			return true
		}
	}
	return false
}

func alloyCheckKey(assertion, scope string) string {
	return assertion + "\x00" + normalizeAlloySpace(scope)
}

func ownedResultSortKey(block alloyModelRender, result core.AlloyCheckResult, resultOrder map[string]int) int {
	for _, check := range block.LocalChecks {
		if alloyCheckKey(check.Assertion, check.Scope) == alloyCheckKey(result.Assertion, result.Scope) {
			return check.Order
		}
	}
	return len(block.LocalChecks) + resultOrder[result.ID.Key()]
}

func alloyBlockStatus(block alloyModelRender) string {
	hasExactPassed := false
	hasFallbackPassed := false
	hasExact := false
	for _, result := range block.OwnedResults {
		if !blockHasExactCheck(block, result.Assertion, result.Scope) {
			switch result.Status {
			case core.StatusFailed:
				if len(block.LocalChecks) > 0 {
					return string(core.StatusFailed)
				}
				return string(core.StatusFailed)
			case core.StatusPassed:
				hasFallbackPassed = true
			}
			continue
		}
		hasExact = true
		switch result.Status {
		case core.StatusFailed:
			return string(core.StatusFailed)
		case core.StatusPassed:
			hasExactPassed = true
		}
	}
	if hasExact {
		if hasExactPassed {
			return string(core.StatusPassed)
		}
		return ""
	}
	if len(block.LocalChecks) > 0 {
		return ""
	}
	if hasFallbackPassed {
		return string(core.StatusPassed)
	}
	return ""
}

func hasFailedOwnedResult(results []core.AlloyCheckResult) bool {
	for _, result := range results {
		if result.Status == core.StatusFailed {
			return true
		}
	}
	return false
}

func renderAlloyModel(block alloyModelRender) string {
	statusClass := alloyBlockStatus(block)

	var out strings.Builder
	out.WriteString(`<section class="exec-block alloy-model`)
	if statusClass != "" {
		out.WriteString(` `)
		out.WriteString(template.HTMLEscapeString(statusClass))
	}
	out.WriteString(`" id="`)
	out.WriteString(template.HTMLEscapeString(block.Anchor))
	out.WriteString(`">`)
	out.WriteString(`<div class="exec-source">`)
	out.WriteString(`<pre><code>`)
	out.WriteString(template.HTMLEscapeString(block.Node.Source))
	out.WriteString(`</code></pre>`)
	out.WriteString(`</div>`)
	renderAlloyFailureMessages(&out, block.OwnedResults)
	renderAlloyGlossDisclosure(&out, block)
	out.WriteString(`<p class="exec-block-footer">`)
	out.WriteString(template.HTMLEscapeString("alloy:model(" + block.Node.Model + ")"))
	out.WriteString(`</p>`)
	out.WriteString(`</section>`)
	return out.String()
}

func renderAlloyFailureMessages(out *strings.Builder, results []core.AlloyCheckResult) {
	for _, result := range results {
		if result.Status != core.StatusFailed || strings.TrimSpace(result.Message) == "" {
			continue
		}
		out.WriteString(`<dl class="failure-diff compact alloy-failure-diff">`)
		out.WriteString(`<dt>check</dt><dd>`)
		out.WriteString(template.HTMLEscapeString(result.Assertion))
		if result.Scope != "" {
			out.WriteString(template.HTMLEscapeString(" (scope " + result.Scope + ")"))
		}
		out.WriteString(`</dd>`)
		out.WriteString(`<dt>error</dt><dd>`)
		out.WriteString(template.HTMLEscapeString(result.Message))
		out.WriteString(`</dd>`)
		out.WriteString(`</dl>`)
	}
}

func renderAlloyGlossDisclosure(out *strings.Builder, block alloyModelRender) {
	sections := buildAlloyGlossSections(block)
	if len(sections) == 0 {
		return
	}

	out.WriteString(`<details class="exec-detail alloy-gloss-detail"`)
	if hasFailedOwnedResult(block.OwnedResults) {
		out.WriteString(` open`)
	}
	out.WriteString(`>`)
	out.WriteString(`<summary class="exec-source alloy-gloss-summary">`)
	out.WriteString(`<span class="exec-summary-text">`)
	out.WriteString(template.HTMLEscapeString("Summary of this Alloy model (" + block.Node.Model + ")"))
	out.WriteString(`</span>`)
	out.WriteString(`<span class="exec-expand-marker"></span>`)
	out.WriteString(`</summary>`)
	out.WriteString(`<div class="exec-source exec-source-body alloy-gloss-body">`)
	for _, section := range sections {
		if len(section.Items) == 0 {
			continue
		}
		out.WriteString(`<section class="alloy-gloss-section">`)
		out.WriteString(`<p class="alloy-gloss-label">`)
		out.WriteString(template.HTMLEscapeString(section.Title))
		out.WriteString(`</p>`)
		out.WriteString(`<ul class="alloy-gloss-list">`)
		for _, item := range section.Items {
			out.WriteString(`<li>`)
			out.WriteString(template.HTMLEscapeString(item))
			out.WriteString(`</li>`)
		}
		out.WriteString(`</ul>`)
		out.WriteString(`</section>`)
	}
	out.WriteString(`</div>`)
	out.WriteString(`</details>`)
}

func buildAlloyGlossSections(block alloyModelRender) []alloyGlossSection {
	modelItems, ruleItems, checkItems := glossAlloySource(block)
	sections := make([]alloyGlossSection, 0, 4)
	if len(modelItems) > 0 {
		sections = append(sections, alloyGlossSection{Title: "Model", Items: modelItems})
	}
	if len(ruleItems) > 0 {
		sections = append(sections, alloyGlossSection{Title: "Rules", Items: ruleItems})
	}
	if len(checkItems) > 0 {
		sections = append(sections, alloyGlossSection{Title: "Checks", Items: checkItems})
	}
	if counterexampleItems := glossCounterexamples(block.OwnedResults); len(counterexampleItems) > 0 {
		sections = append(sections, alloyGlossSection{Title: "Counterexample", Items: counterexampleItems})
	}
	return sections
}

func glossAlloySource(block alloyModelRender) ([]string, []string, []string) {
	lines := strings.Split(strings.ReplaceAll(block.Node.Source, "\r\n", "\n"), "\n")
	modelItems := make([]string, 0)
	ruleItems := make([]string, 0)
	checkItems := make([]string, 0)

	for i := 0; i < len(lines); {
		line := strings.TrimSpace(stripAlloyComment(lines[i]))
		if line == "" {
			i++
			continue
		}
		switch {
		case strings.HasPrefix(line, "module "):
			modelItems = append(modelItems, "Module name is "+strings.TrimSpace(strings.TrimPrefix(line, "module "))+".")
			i++
		case alloySigHeaderPattern.MatchString(line):
			blockText, next := collectBraceBlock(lines, i)
			modelItems = append(modelItems, glossSigBlock(blockText)...)
			i = next
		case alloyBlockHeaderPattern.MatchString(line):
			blockText, next := collectBraceBlock(lines, i)
			kind, text := glossRuleBlock(blockText)
			if kind == "Model" {
				modelItems = append(modelItems, text)
			} else if text != "" {
				ruleItems = append(ruleItems, text)
			}
			i = next
		case strings.HasPrefix(line, "check "):
			if checkText := glossCheckLine(line, block.ExactResultsByID); checkText != "" {
				checkItems = append(checkItems, checkText)
			}
			i++
		default:
			i++
		}
	}

	return modelItems, ruleItems, checkItems
}

func collectBraceBlock(lines []string, start int) (string, int) {
	var parts []string
	depth := 0
	seenBrace := false
	for i := start; i < len(lines); i++ {
		part := lines[i]
		parts = append(parts, part)
		depth += strings.Count(part, "{")
		if strings.Contains(part, "{") {
			seenBrace = true
		}
		depth -= strings.Count(part, "}")
		if seenBrace && depth <= 0 {
			return strings.Join(parts, "\n"), i + 1
		}
	}
	return strings.Join(parts, "\n"), len(lines)
}

func glossSigBlock(blockText string) []string {
	header := alloySigHeaderPattern.FindStringSubmatch(leadingAlloyHeader(blockText))
	if len(header) == 0 {
		return nil
	}
	names := splitAlloyNames(header[3])
	extends := strings.TrimSpace(header[4])
	prefix := strings.TrimSpace(header[2])
	abstract := strings.TrimSpace(header[1]) != ""

	items := make([]string, 0, 1+len(names))
	headerText := strings.Join(names, ", ") + " "
	switch {
	case prefix == "one":
		headerText += "are exactly-one signatures"
	case prefix == "lone":
		headerText += "are at-most-one signatures"
	case prefix == "some":
		headerText += "are one-or-more signatures"
	default:
		headerText += "are signatures"
	}
	if len(names) == 1 {
		headerText = names[0] + " is a signature"
		switch prefix {
		case "one":
			headerText = "Exactly one " + names[0] + " exists"
		case "lone":
			headerText = "At most one " + names[0] + " exists"
		case "some":
			headerText = "One or more " + names[0] + " exist"
		}
	}
	if abstract {
		headerText += " and is abstract"
	}
	if extends != "" {
		if len(names) == 1 {
			headerText += " and extends " + extends
		} else {
			headerText += " and extend " + extends
		}
	}
	items = append(items, ensurePeriod(headerText))

	body := extractBraceBody(blockText)
	for _, field := range splitAlloyFields(body) {
		if text := glossFieldLine(field); text != "" {
			items = append(items, text)
		}
	}
	return items
}

func splitAlloyFields(body string) []string {
	lines := strings.Split(body, "\n")
	fields := make([]string, 0)
	for _, line := range lines {
		trimmed := strings.TrimSpace(strings.TrimSuffix(stripAlloyComment(line), ","))
		if trimmed == "" {
			continue
		}
		fields = append(fields, trimmed)
	}
	return fields
}

func glossFieldLine(line string) string {
	namesPart, rest, ok := strings.Cut(line, ":")
	if !ok {
		return ""
	}
	names := splitAlloyNames(namesPart)
	rest = normalizeAlloySpace(rest)
	parts := strings.SplitN(rest, " ", 2)
	multiplicity := ""
	target := rest
	if len(parts) == 2 && isAlloyMultiplicity(parts[0]) {
		multiplicity = parts[0]
		target = parts[1]
	}
	phrase := "zero or more"
	switch multiplicity {
	case "one":
		phrase = "exactly one"
	case "lone":
		phrase = "at most one"
	case "some":
		phrase = "one or more"
	}
	fieldLabel := strings.Join(names, ", ")
	if len(names) == 1 {
		return fmt.Sprintf("Each instance has %s %s in %s.", phrase, names[0], target)
	}
	return fmt.Sprintf("Each instance has %s %s in %s.", phrase, fieldLabel, target)
}

func isAlloyMultiplicity(token string) bool {
	switch token {
	case "one", "lone", "some", "set":
		return true
	default:
		return false
	}
}

func glossRuleBlock(blockText string) (string, string) {
	header := alloyBlockHeaderPattern.FindStringSubmatch(leadingAlloyHeader(blockText))
	if len(header) == 0 {
		return "", ""
	}
	kind := header[1]
	name := strings.TrimSpace(header[2])
	body := normalizeAlloySpace(extractBraceBody(blockText))
	if body == "" {
		return "Rules", ""
	}
	bodyText := glossAlloyCondition(body)
	if bodyText == "" {
		bodyText = "Constraint: " + body
	}
	label := "Fact"
	if kind == "assert" {
		label = "Assertion"
	}
	if name != "" {
		label += " " + name
	}
	return "Rules", ensurePeriod(label + ": " + bodyText)
}

func glossCheckLine(line string, exactResults map[string]core.AlloyCheckResult) string {
	match := alloyCheckPatternLocal.FindStringSubmatch(line)
	if len(match) != 3 {
		return ""
	}
	assertion := match[1]
	scope := normalizeAlloySpace(match[2])
	text := fmt.Sprintf("Check %s is explored with %s.", assertion, glossScope(scope))
	if exactResults != nil {
		if result, ok := exactResults[alloyCheckKey(assertion, scope)]; ok {
			switch result.Status {
			case core.StatusPassed:
				text += " Result: passed."
			case core.StatusFailed:
				text += " Result: failed."
			default:
				text += " Result: not executed."
			}
		}
	}
	return text
}

func glossScope(scope string) string {
	scope = normalizeAlloySpace(scope)
	base, override, ok := strings.Cut(scope, " but ")
	if !ok {
		return "a scope of " + scope + ", so Alloy searches examples up to size " + scope
	}
	overrideParts := strings.Split(override, ",")
	glosses := make([]string, 0, len(overrideParts))
	for _, part := range overrideParts {
		glosses = append(glosses, glossOverrideClause(part))
	}
	return "a default scope of " + base + ", so Alloy first searches examples up to size " + base + ", and " + strings.Join(glosses, ", and ")
}

func glossOverrideClause(raw string) string {
	override := normalizeAlloySpace(raw)
	if match := regexp.MustCompile(`^exactly\s+(\d+)\s+([A-Za-z_][A-Za-z0-9_]*)$`).FindStringSubmatch(override); len(match) == 3 {
		return match[2] + " is fixed to exactly " + match[1] + " atoms"
	}
	if match := alloyOverridePattern.FindStringSubmatch(override); len(match) == 3 {
		switch match[2] {
		case "Int":
			return "Int uses " + match[1] + "-bit integers"
		case "steps":
			return "the step bound is set to " + match[1] + " steps"
		default:
			return match[2] + " is limited to " + match[1] + " atoms"
		}
	}
	return "override " + override
}

func glossAlloyCondition(body string) string {
	body = normalizeAlloySpace(body)
	match := alloyQuantifiedPattern.FindStringSubmatch(body)
	if len(match) != 5 {
		return "Constraint: " + body
	}

	quantifier := match[1]
	name := match[2]
	typ := match[3]
	rest := glossPredicate(match[4])
	switch quantifier {
	case "all":
		return fmt.Sprintf("For every %s in %s, %s.", name, typ, rest)
	case "some":
		return fmt.Sprintf("There exists a %s in %s such that %s.", name, typ, rest)
	case "one":
		return fmt.Sprintf("Exactly one %s in %s satisfies %s.", name, typ, rest)
	case "no":
		return fmt.Sprintf("No %s in %s satisfies %s.", name, typ, rest)
	default:
		return "Constraint: " + body
	}
}

func glossPredicate(expr string) string {
	expr = normalizeAlloySpace(expr)
	if strings.HasPrefix(expr, "one ") {
		return strings.TrimSpace(strings.TrimPrefix(expr, "one ")) + " has exactly one value"
	}
	if strings.HasPrefix(expr, "no ") {
		return strings.TrimSpace(strings.TrimPrefix(expr, "no ")) + " has no value"
	}
	if left, right, ok := strings.Cut(expr, " implies "); ok {
		return fmt.Sprintf("if %s, then %s", glossPredicate(left), glossPredicate(right))
	}
	if parts := strings.Split(expr, " and "); len(parts) > 1 {
		for i := range parts {
			parts[i] = glossPredicate(parts[i])
		}
		return strings.Join(parts, ", and ")
	}
	if parts := strings.Split(expr, " or "); len(parts) > 1 {
		for i := range parts {
			parts[i] = glossPredicate(parts[i])
		}
		return strings.Join(parts, ", or ")
	}
	if left, right, ok := strings.Cut(expr, " in "); ok {
		return fmt.Sprintf("%s is in %s", normalizeAlloySpace(left), normalizeAlloySpace(right))
	}
	if left, right, ok := strings.Cut(expr, " = "); ok {
		return fmt.Sprintf("%s equals %s", normalizeAlloySpace(left), normalizeAlloySpace(right))
	}
	return expr
}

func glossCounterexamples(results []core.AlloyCheckResult) []string {
	items := make([]string, 0)
	for _, result := range results {
		if !isSolverCounterexample(result) {
			continue
		}
		for _, item := range glossCounterexampleResult(result) {
			items = append(items, item)
		}
	}
	return items
}

func glossCounterexampleResult(result core.AlloyCheckResult) []string {
	lines := extractCounterexampleRelationLines(result.Message)
	items := make([]string, 0, len(lines))
	for _, line := range lines {
		items = append(items, glossCounterexampleRelation(result.Assertion, line))
	}
	if len(items) > 0 {
		return items
	}
	return glossCounterexampleArtifactFile(result.CounterexamplePath, result.Assertion)
}

func isSolverCounterexample(result core.AlloyCheckResult) bool {
	if result.Status != core.StatusFailed {
		return false
	}
	if result.CounterexamplePath != "" {
		return true
	}
	return strings.HasPrefix(strings.TrimSpace(result.Message), "counterexample for ")
}

func extractCounterexampleRelationLines(message string) []string {
	lines := strings.Split(strings.ReplaceAll(message, "\r\n", "\n"), "\n")
	relations := make([]string, 0)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || !strings.Contains(trimmed, "=") {
			continue
		}
		relations = append(relations, trimmed)
	}
	return relations
}

func glossCounterexampleRelation(assertion, line string) string {
	match := alloyRelationPattern.FindStringSubmatch(line)
	if len(match) != 4 {
		return "Observed relation for " + assertion + ": " + line
	}
	subject := match[1]
	relation := match[2]
	value := normalizeAlloySpace(match[3])
	switch relation {
	case "board":
		return subject + " belongs to " + value + "."
	case "column":
		return subject + " is in column " + value + "."
	case "next":
		return subject + " points to " + value + " through next."
	default:
		return "Observed relation for " + assertion + ": " + line
	}
}

func glossCounterexampleArtifactFile(path string, assertion string) []string {
	if strings.TrimSpace(path) == "" {
		return nil
	}

	body, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var artifact counterexampleArtifact
	if err := json.Unmarshal(body, &artifact); err != nil {
		return nil
	}
	if len(artifact.Solution) == 0 || len(artifact.Solution[0].Instances) == 0 {
		return []string{"The solver found a counterexample instance."}
	}

	var instance counterexampleInstance
	if err := json.Unmarshal(artifact.Solution[0].Instances[0], &instance); err != nil {
		return []string{"The solver found a counterexample instance."}
	}

	if relationItems := glossCounterexampleValues(assertion, instance.Values); len(relationItems) > 0 {
		return relationItems
	}
	if witnessItems := glossCounterexampleWitnesses(assertion, instance.Skolems); len(witnessItems) > 0 {
		return witnessItems
	}
	return []string{"The solver found a counterexample instance."}
}

func glossCounterexampleValues(assertion string, values map[string]map[string][][]string) []string {
	if len(values) == 0 {
		return nil
	}

	atoms := make([]string, 0, len(values))
	for atom := range values {
		atoms = append(atoms, atom)
	}
	sort.Strings(atoms)

	lines := make([]string, 0)
	for _, atom := range atoms {
		relations := values[atom]
		if len(relations) == 0 {
			continue
		}
		names := make([]string, 0, len(relations))
		for name := range relations {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			for _, tuple := range relations[name] {
				if len(tuple) == 0 {
					continue
				}
				lines = append(lines, atom+"."+name+" = "+strings.Join(tuple, ", "))
			}
		}
	}

	items := make([]string, 0, len(lines))
	for _, line := range lines {
		items = append(items, glossCounterexampleRelation(assertion, line))
	}
	return items
}

func glossCounterexampleWitnesses(assertion string, skolems map[string]counterexampleSkolem) []string {
	if len(skolems) == 0 {
		return nil
	}

	names := make([]string, 0, len(skolems))
	for name := range skolems {
		names = append(names, name)
	}
	sort.Strings(names)

	items := make([]string, 0, len(names)+1)
	items = append(items, "The solver found witness bindings that make the assertion fail.")
	for _, name := range names {
		skolem := skolems[name]
		for _, tuple := range skolem.Data {
			if len(tuple) == 0 {
				continue
			}
			items = append(items, fmt.Sprintf("Witness %s = %s.", glossSkolemName(assertion, name), strings.Join(tuple, ", ")))
		}
	}
	return items
}

func glossSkolemName(assertion string, raw string) string {
	name := strings.TrimPrefix(strings.TrimSpace(raw), "$")
	prefix := assertion + "_"
	if strings.HasPrefix(name, prefix) {
		return strings.TrimPrefix(name, prefix)
	}
	return name
}

func renderAlloyRef(node core.AlloyRefNode, ctx alloyRenderContext) (string, error) {
	if node.ID == nil {
		return "", nil
	}

	label := "alloy:ref(" + node.Model + "#" + node.Assertion + ", scope=" + node.Scope + ")"
	result, ok := ctx.ResultsByKey[node.ID.Key()]
	statusClass := ""
	if ok && result.Status != "" {
		statusClass = string(result.Status)
	}

	var out strings.Builder
	out.WriteString(`<section class="exec-block alloy-ref`)
	if statusClass != "" {
		out.WriteString(` `)
		out.WriteString(template.HTMLEscapeString(statusClass))
	}
	out.WriteString(`" id="`)
	out.WriteString(template.HTMLEscapeString(node.ID.Anchor()))
	out.WriteString(`">`)
	out.WriteString(`<div class="exec-source">`)
	out.WriteString(`<code>`)
	out.WriteString(template.HTMLEscapeString(label))
	out.WriteString(`</code>`)
	out.WriteString(`<div class="alloy-ref-summary">`)
	out.WriteString(template.HTMLEscapeString("References Alloy check " + node.Assertion + " at scope " + node.Scope + ". Result: " + alloyRefStatusText(result.Status) + "."))
	out.WriteString(`</div>`)
	if ok && result.Status == core.StatusFailed && result.Message != "" {
		out.WriteString(`<div class="cell-actual">`)
		out.WriteString(template.HTMLEscapeString(firstAlloyFailureLine(result.Message)))
		out.WriteString(`</div>`)
	}
	if anchor := ctx.OwnerAnchorByResult[node.ID.Key()]; anchor != "" {
		out.WriteString(`<div class="alloy-ref-link-row">`)
		out.WriteString(`<a class="alloy-ref-link" href="#`)
		out.WriteString(template.HTMLEscapeString(anchor))
		out.WriteString(`">See Alloy model summary</a>`)
		out.WriteString(`</div>`)
	}
	out.WriteString(`</div>`)
	out.WriteString(`<p class="exec-block-footer">alloy reference</p>`)
	out.WriteString(`</section>`)
	return out.String(), nil
}

func firstAlloyFailureLine(message string) string {
	for _, line := range strings.Split(strings.ReplaceAll(message, "\r\n", "\n"), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			return trimmed
		}
	}
	return strings.TrimSpace(message)
}

func alloyRefStatusText(status core.Status) string {
	switch status {
	case core.StatusPassed:
		return "passed"
	case core.StatusFailed:
		return "failed"
	default:
		return "not executed"
	}
}

func splitAlloyNames(input string) []string {
	rawParts := strings.Split(input, ",")
	names := make([]string, 0, len(rawParts))
	for _, part := range rawParts {
		trimmed := normalizeAlloySpace(part)
		if trimmed != "" {
			names = append(names, trimmed)
		}
	}
	return names
}

func extractBraceBody(blockText string) string {
	start := strings.Index(blockText, "{")
	if start < 0 {
		return ""
	}
	depth := 0
	for index := start; index < len(blockText); index++ {
		switch blockText[index] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return blockText[start+1 : index]
			}
		}
	}
	return ""
}

func leadingAlloyHeader(blockText string) string {
	if start := strings.Index(blockText, "{"); start >= 0 {
		return normalizeAlloySpace(blockText[:start])
	}
	return normalizeAlloySpace(blockText)
}

func stripAlloyComment(line string) string {
	if idx := strings.Index(line, "--"); idx >= 0 {
		return line[:idx]
	}
	return line
}

func normalizeAlloySpace(text string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
}

func ensurePeriod(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	if strings.HasSuffix(text, ".") {
		return text
	}
	return text + "."
}
