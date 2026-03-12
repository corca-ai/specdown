package core

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

func readDocument(baseDir string, relativePath string, ignorePrefixes []string) (Document, error) {
	fullPath := filepath.Join(baseDir, filepath.FromSlash(relativePath))
	body, err := os.ReadFile(fullPath)
	if err != nil {
		return Document{}, fmt.Errorf("read %s: %w", fullPath, err)
	}
	return ParseDocument(relativePath, string(body), ignorePrefixes)
}

func parseFrontmatter(markdown string) (Frontmatter, string) {
	if !strings.HasPrefix(markdown, "---\n") {
		return Frontmatter{}, markdown
	}
	end := strings.Index(markdown[4:], "\n---\n")
	if end == -1 {
		return Frontmatter{}, markdown
	}
	body := markdown[4 : 4+end]
	rest := markdown[4+end+5:]

	var fm Frontmatter
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		switch key {
		case "timeout":
			if n, err := strconv.Atoi(value); err == nil {
				fm.Timeout = n
			}
		case "type":
			fm.Type = value
		}
	}
	return fm, rest
}

// documentParser holds mutable state for a single document parse.
type documentParser struct {
	file            string
	lines           []string
	ignorePrefixSet map[string]bool
	nodes           []Node
	headingPath     []string
	title           string
	ordinal         int
	check           string
	checkParams     map[string]string
	checkRaw        string
	hookKind        HookKind
	hookEach        bool
	hookRaw         string
	warnings        []string
}

func ParseDocument(relativePath string, markdown string, ignorePrefixes []string) (Document, error) {
	fm, content := parseFrontmatter(markdown)

	ignorePrefixSet := make(map[string]bool, len(ignorePrefixes))
	for _, p := range ignorePrefixes {
		ignorePrefixSet[p] = true
	}

	p := &documentParser{
		file:            relativePath,
		lines:           splitLines(content),
		ignorePrefixSet: ignorePrefixSet,
	}

	if err := p.parse(); err != nil {
		return Document{}, err
	}

	title := p.title
	if title == "" {
		title = relativePath
	}

	return Document{
		RelativeTo:  relativePath,
		Title:       title,
		Markdown:    markdown,
		Nodes:       p.nodes,
		Frontmatter: fm,
		Warnings:    p.warnings,
	}, nil
}

func (p *documentParser) parse() error {
	for i := 0; i < len(p.lines); {
		next, err := p.parseLine(i)
		if err != nil {
			return err
		}
		i = next
	}

	if p.hookKind != "" {
		return fmt.Errorf("%s: %s directive must be followed by a code block", p.file, p.hookKind)
	}
	return p.flushCheck()
}

func (p *documentParser) parseLine(i int) (int, error) {
	line := p.lines[i]

	if ref, ok, err := parseAlloyRefDirective(line); err != nil {
		return 0, fmt.Errorf("%s: %w", p.file, err)
	} else if ok {
		return p.handleAlloyRef(i, ref)
	}

	if hk, he, ok := parseHookDirective(line); ok {
		return p.handleHookDirective(i, hk, he)
	}

	if nextCheck, nextParams, ok := parseCheckDirective(line); ok {
		return p.handleCheckDirective(i, nextCheck, nextParams)
	}

	if isFenceStart(line) {
		return p.handleFence(i)
	}

	if isTableStart(p.lines, i) {
		return p.handleTable(i)
	}

	if level, text, ok := parseHeading(line); ok {
		return p.handleHeading(i, level, text)
	}

	return p.handleProse(i)
}

func (p *documentParser) requireNoHook() error {
	if p.hookKind != "" {
		return fmt.Errorf("%s: %s directive must be followed by a code block", p.file, p.hookKind)
	}
	return nil
}

func (p *documentParser) flushCheck() error {
	if p.check == "" {
		return nil
	}
	node, flushed, err := tryFlushCheck(p.check, p.checkParams, p.checkRaw, p.file, &p.ordinal, p.headingPath)
	if err != nil {
		return err
	}
	if flushed {
		p.nodes = append(p.nodes, node)
	}
	p.check = ""
	p.checkParams = nil
	p.checkRaw = ""
	return nil
}

func (p *documentParser) handleAlloyRef(i int, ref AlloyRefNode) (int, error) {
	if err := p.requireNoHook(); err != nil {
		return 0, err
	}
	if err := p.flushCheck(); err != nil {
		return 0, err
	}
	p.ordinal++
	ref.Raw = p.lines[i]
	ref.HeadingPath = append([]string(nil), p.headingPath...)
	ref.ID = &SpecID{
		File:        p.file,
		HeadingPath: append([]string(nil), p.headingPath...),
		Ordinal:     p.ordinal,
	}
	p.nodes = append(p.nodes, ref)
	return i + 1, nil
}

func (p *documentParser) handleHookDirective(i int, hk HookKind, he bool) (int, error) {
	if err := p.requireNoHook(); err != nil {
		return 0, err
	}
	if err := p.flushCheck(); err != nil {
		return 0, err
	}
	p.hookKind = hk
	p.hookEach = he
	p.hookRaw = p.lines[i]
	return i + 1, nil
}

func (p *documentParser) handleCheckDirective(i int, check string, params map[string]string) (int, error) {
	if err := p.requireNoHook(); err != nil {
		return 0, err
	}
	if err := p.flushCheck(); err != nil {
		return 0, err
	}
	p.check = check
	p.checkParams = params
	p.checkRaw = p.lines[i]
	return i + 1, nil
}

func (p *documentParser) handleFence(i int) (int, error) {
	if err := p.flushCheck(); err != nil {
		return 0, err
	}
	info := parseFenceInfo(p.lines[i])

	if p.hookKind != "" {
		return p.parseHookBlock(i, info)
	}

	if modelName, ok := parseAlloyModelInfo(info); ok {
		return p.parseAlloyModelBlock(i, modelName)
	}

	return p.parseCodeBlock(i, info)
}

func (p *documentParser) findFenceEnd(start int) (int, error) {
	for j := start + 1; j < len(p.lines); j++ {
		if isFenceEnd(p.lines[j]) {
			return j, nil
		}
	}
	return 0, fmt.Errorf("%s: unclosed fenced code block", p.file)
}

func (p *documentParser) fenceContent(start, end int) (raw string, source string) {
	raw = strings.Join(p.lines[start:end+1], "")
	source = strings.TrimSuffix(strings.Join(p.lines[start+1:end], ""), "\n")
	return
}

func (p *documentParser) parseHookBlock(i int, info string) (int, error) {
	if _, ok := parseAlloyModelInfo(info); ok {
		return 0, fmt.Errorf("%s: %s directive must be followed by an executable code block", p.file, p.hookKind)
	}
	block, err := parseBlockSpec(info)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", p.file, err)
	}
	if !block.Executable() {
		return 0, fmt.Errorf("%s: %s directive must be followed by an executable code block", p.file, p.hookKind)
	}
	end, err := p.findFenceEnd(i)
	if err != nil {
		return 0, err
	}
	_, source := p.fenceContent(i, end)
	hookSummary := extractSummary(source)
	p.nodes = append(p.nodes, HookNode{
		Hook:        p.hookKind,
		Each:        p.hookEach,
		Block:       block,
		Source:      source,
		Raw:         p.hookRaw + strings.Join(p.lines[i:end+1], ""),
		Summary:     hookSummary,
		HeadingPath: append([]string(nil), p.headingPath...),
	})
	p.hookKind = ""
	p.hookEach = false
	p.hookRaw = ""
	return end + 1, nil
}

func (p *documentParser) parseAlloyModelBlock(i int, modelName string) (int, error) {
	end, err := p.findFenceEnd(i)
	if err != nil {
		return 0, err
	}
	raw, source := p.fenceContent(i, end)
	p.nodes = append(p.nodes, AlloyModelNode{
		Model:       modelName,
		Source:      source,
		Raw:         raw,
		HeadingPath: append([]string(nil), p.headingPath...),
	})
	return end + 1, nil
}

func (p *documentParser) parseCodeBlock(i int, info string) (int, error) {
	block, err := parseBlockSpec(info)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", p.file, err)
	}

	if !block.Executable() {
		if prefix := unknownBlockPrefix(info); prefix != "" && !p.ignorePrefixSet[prefix] {
			p.warnings = append(p.warnings, fmt.Sprintf("%s: unknown block prefix %q in code block %q (not executable)", p.file, prefix, strings.TrimSpace(info)))
		}
	}

	end, err := p.findFenceEnd(i)
	if err != nil {
		return 0, err
	}
	raw, source := p.fenceContent(i, end)
	node := CodeBlockNode{
		Block:  block,
		Source: source,
		Raw:    raw,
	}
	if block.Executable() && !IsDoctestContent(source) {
		node.Summary = extractSummary(source)
	}
	if block.Executable() {
		p.ordinal++
		node.ID = &SpecID{
			File:        p.file,
			HeadingPath: append([]string(nil), p.headingPath...),
			Ordinal:     p.ordinal,
		}
	}
	p.nodes = append(p.nodes, node)
	return end + 1, nil
}

func (p *documentParser) handleTable(i int) (int, error) {
	if err := p.requireNoHook(); err != nil {
		return 0, err
	}
	table, next, err := parseTableNode(p.file, p.lines, i, p.check, p.checkParams, &p.ordinal, p.headingPath)
	if err != nil {
		return 0, err
	}
	p.nodes = append(p.nodes, table)
	p.check = ""
	p.checkParams = nil
	p.checkRaw = ""
	return next, nil
}

func (p *documentParser) handleHeading(i int, level int, text string) (int, error) {
	if err := p.requireNoHook(); err != nil {
		return 0, err
	}
	if err := p.flushCheck(); err != nil {
		return 0, err
	}
	if p.title == "" {
		p.title = text
	}
	p.headingPath = nextHeadingPath(p.headingPath, level, text)
	p.nodes = append(p.nodes, HeadingNode{
		Level:       level,
		Text:        text,
		Raw:         p.lines[i],
		HeadingPath: append([]string(nil), p.headingPath...),
	})
	return i + 1, nil
}

func (p *documentParser) handleProse(i int) (int, error) {
	start := i
	for i < len(p.lines) {
		isBreak, err := p.isStructuralLine(i)
		if err != nil {
			return 0, err
		}
		if isBreak {
			break
		}
		i++
	}

	raw := strings.Join(p.lines[start:i], "")
	if strings.TrimSpace(raw) == "" {
		return i, nil
	}
	if err := p.requireNoHook(); err != nil {
		return 0, err
	}
	if err := p.flushCheck(); err != nil {
		return 0, err
	}
	proseNode := ProseNode{
		Raw:         raw,
		HeadingPath: append([]string(nil), p.headingPath...),
	}
	proseNode.Inlines = parseInlineElements(raw, p.file, &p.ordinal, p.headingPath)
	p.nodes = append(p.nodes, proseNode)
	return i, nil
}

// isStructuralLine returns true if the line at index starts a new structural element.
func (p *documentParser) isStructuralLine(i int) (bool, error) {
	line := p.lines[i]
	if isFenceStart(line) {
		return true, nil
	}
	if _, _, ok := parseHeading(line); ok {
		return true, nil
	}
	if _, ok, err := parseAlloyRefDirective(line); err != nil {
		return false, fmt.Errorf("%s: %w", p.file, err)
	} else if ok {
		return true, nil
	}
	if _, _, ok := parseHookDirective(line); ok {
		return true, nil
	}
	if _, _, ok := parseCheckDirective(line); ok {
		return true, nil
	}
	return isTableStart(p.lines, i), nil
}

func splitLines(markdown string) []string {
	if markdown == "" {
		return nil
	}

	lines := strings.SplitAfter(markdown, "\n")
	if !strings.HasSuffix(markdown, "\n") {
		last := lines[len(lines)-1]
		lines[len(lines)-1] = last + "\n"
	}
	return lines
}

func isFenceStart(line string) bool {
	return strings.HasPrefix(strings.TrimSpace(line), "```")
}

func isFenceEnd(line string) bool {
	return strings.TrimSpace(line) == "```"
}

func parseFenceInfo(line string) string {
	trimmed := strings.TrimSpace(line)
	return strings.TrimSpace(strings.TrimPrefix(trimmed, "```"))
}

var checkDirectivePattern = regexp.MustCompile(`^\s*>\s*check:([A-Za-z0-9_-]+)(?:\(([^)]*)\))?\s*$`)
var alloyModelInfoPattern = regexp.MustCompile(`^alloy:model\(([A-Za-z_][A-Za-z0-9_-]*)\)$`)
var alloyRefDirectivePattern = regexp.MustCompile(`^\s*>\s*alloy:ref\(([A-Za-z_][A-Za-z0-9_-]*)#([A-Za-z_][A-Za-z0-9_]*),\s*scope=([^)]+)\)\s*$`)
var hookDirectivePattern = regexp.MustCompile(`^\s*>\s*(setup|teardown)(?::(each))?\s*$`)

func parseCheckDirective(line string) (string, map[string]string, bool) {
	matches := checkDirectivePattern.FindStringSubmatch(line)
	if matches == nil {
		return "", nil, false
	}
	var params map[string]string
	if matches[2] != "" {
		params = parseCheckParams(matches[2])
	}
	return matches[1], params, true
}

func parseCheckParams(raw string) map[string]string {
	params := make(map[string]string)
	for _, pair := range strings.Split(raw, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) == 2 {
			params[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return params
}

func parseHookDirective(line string) (HookKind, bool, bool) {
	matches := hookDirectivePattern.FindStringSubmatch(line)
	if matches == nil {
		return "", false, false
	}
	return HookKind(matches[1]), matches[2] == "each", true
}

func tryFlushCheck(check string, checkParams map[string]string, checkRaw string, relativePath string, ordinal *int, headingPath []string) (CheckCallNode, bool, error) {
	if check == "" {
		return CheckCallNode{}, false, nil
	}
	if len(checkParams) == 0 {
		return CheckCallNode{}, false, fmt.Errorf("%s: check directive %q must be followed by a table", relativePath, check)
	}
	*ordinal++
	return CheckCallNode{
		Check:       check,
		CheckParams: checkParams,
		Raw:           checkRaw,
		HeadingPath:   append([]string(nil), headingPath...),
		ID: &SpecID{
			File:        relativePath,
			HeadingPath: append([]string(nil), headingPath...),
			Ordinal:     *ordinal,
		},
	}, true, nil
}

func parseAlloyModelInfo(info string) (string, bool) {
	matches := alloyModelInfoPattern.FindStringSubmatch(strings.TrimSpace(info))
	if matches == nil {
		return "", false
	}
	return matches[1], true
}

func parseAlloyRefDirective(line string) (AlloyRefNode, bool, error) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, ">") || !strings.Contains(trimmed, "alloy:ref(") {
		return AlloyRefNode{}, false, nil
	}

	matches := alloyRefDirectivePattern.FindStringSubmatch(trimmed)
	if matches == nil {
		return AlloyRefNode{}, false, fmt.Errorf("invalid alloy reference directive %q", trimmed)
	}

	scope := strings.TrimSpace(matches[3])
	if scope == "" {
		return AlloyRefNode{}, false, fmt.Errorf("invalid alloy reference directive %q", trimmed)
	}

	return AlloyRefNode{
		Model:     matches[1],
		Assertion: matches[2],
		Scope:     scope,
	}, true, nil
}

func isTableStart(lines []string, index int) bool {
	if index+1 >= len(lines) {
		return false
	}
	return looksLikeTableRow(lines[index]) && isTableSeparator(lines[index+1])
}

func parseTableNode(relativePath string, lines []string, start int, check string, checkParams map[string]string, ordinal *int, headingPath []string) (TableNode, int, error) {
	columns, err := parseTableCells(lines[start])
	if err != nil {
		return TableNode{}, 0, fmt.Errorf("%s: %w", relativePath, err)
	}
	if len(columns) == 0 {
		return TableNode{}, 0, fmt.Errorf("%s: table header must define at least one column", relativePath)
	}

	end := start + 2
	rows := make([]TableRowNode, 0)
	for end < len(lines) && looksLikeTableRow(lines[end]) {
		cells, err := parseTableCells(lines[end])
		if err != nil {
			return TableNode{}, 0, fmt.Errorf("%s: %w", relativePath, err)
		}
		if len(cells) != len(columns) {
			return TableNode{}, 0, fmt.Errorf("%s: table row has %d cells but header has %d columns", relativePath, len(cells), len(columns))
		}

		row := TableRowNode{
			Cells: cells,
			Raw:   lines[end],
		}
		if check != "" {
			*ordinal++
			row.ID = &SpecID{
				File:        relativePath,
				HeadingPath: append([]string(nil), headingPath...),
				Ordinal:     *ordinal,
			}
		}
		rows = append(rows, row)
		end++
	}

	if len(rows) == 0 {
		return TableNode{}, 0, fmt.Errorf("%s: table must define at least one row", relativePath)
	}

	return TableNode{
		Check:       check,
		CheckParams: checkParams,
		Columns:       columns,
		Rows:          rows,
		Raw:           strings.Join(lines[start:end], ""),
	}, end, nil
}

func looksLikeTableRow(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "|") && strings.Count(trimmed, "|") >= 2
}

func isTableSeparator(line string) bool {
	if !looksLikeTableRow(line) {
		return false
	}
	cells, err := parseTableCells(line)
	if err != nil || len(cells) == 0 {
		return false
	}
	for _, cell := range cells {
		if !isSeparatorCell(cell) {
			return false
		}
	}
	return true
}

func isSeparatorCell(cell string) bool {
	trimmed := strings.ReplaceAll(strings.TrimSpace(cell), " ", "")
	if trimmed == "" {
		return false
	}
	hasDash := false
	for _, r := range trimmed {
		if r == '-' {
			hasDash = true
		} else if r != ':' {
			return false
		}
	}
	return hasDash
}

func parseTableCells(line string) ([]string, error) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "|") {
		return nil, fmt.Errorf("invalid markdown table row %q", strings.TrimSpace(line))
	}
	trimmed = strings.TrimPrefix(trimmed, "|")
	trimmed = strings.TrimSuffix(trimmed, "|")
	cells := splitTableCells(trimmed)
	result := make([]string, 0, len(cells))
	for _, cell := range cells {
		result = append(result, strings.TrimSpace(cell))
	}
	return result, nil
}

// splitTableCells splits on unescaped | characters, treating \| as a literal pipe.
func splitTableCells(s string) []string {
	var cells []string
	var current strings.Builder
	for i := 0; i < len(s); i++ {
		switch {
		case s[i] == '\\' && i+1 < len(s) && s[i+1] == '|':
			current.WriteString(`\|`)
			i++ // skip the |
		case s[i] == '|':
			cells = append(cells, current.String())
			current.Reset()
		default:
			current.WriteByte(s[i])
		}
	}
	cells = append(cells, current.String())
	return cells
}

// UnescapeCell processes escape sequences in a table cell value.
// \n → newline, \| → literal pipe, \\ → literal backslash.
// This is called by the engine before sending cells to adapters.
func UnescapeCell(s string) string {
	if !strings.ContainsRune(s, '\\') {
		return s
	}
	var out strings.Builder
	out.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case 'n':
				out.WriteByte('\n')
				i++
			case '|':
				out.WriteByte('|')
				i++
			case '\\':
				out.WriteByte('\\')
				i++
			default:
				out.WriteByte(s[i])
			}
		} else {
			out.WriteByte(s[i])
		}
	}
	return out.String()
}

func parseHeading(line string) (int, string, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || trimmed[0] != '#' {
		return 0, "", false
	}

	level := 0
	for level < len(trimmed) && trimmed[level] == '#' {
		level++
	}
	if level == 0 || level > 6 {
		return 0, "", false
	}
	if level == len(trimmed) || trimmed[level] != ' ' {
		return 0, "", false
	}

	text := strings.TrimSpace(trimmed[level:])
	if text == "" {
		return 0, "", false
	}
	return level, text, true
}

var inlineExpectPattern = regexp.MustCompile("`expect:\\s*(.+?)\\s*==\\s*(.+?)\\s*(!fail\\s*)?`")
var inlineCheckPattern = regexp.MustCompile("`check:([A-Za-z0-9_-]+)\\(([^)]*)\\)`")

// insideCodeSpan checks whether a match at [start, end) in raw
// is enclosed by a Markdown code span. The match itself includes its
// surrounding backticks, so we look for additional backticks just
// outside the match boundaries (possibly separated by a space, as in
// the double-backtick form `` `...` ``).
func insideCodeSpan(raw string, start, end int) bool {
	// Immediately adjacent backtick (triple-backtick or ```)
	if start > 0 && raw[start-1] == '`' {
		return true
	}
	if end < len(raw) && raw[end] == '`' {
		return true
	}
	// Double-backtick form: `` `...` `` — space then backtick
	if start > 1 && raw[start-1] == ' ' && raw[start-2] == '`' {
		return true
	}
	if end+1 < len(raw) && raw[end] == ' ' && raw[end+1] == '`' {
		return true
	}
	return false
}

func parseInlineElements(raw string, relativePath string, ordinal *int, headingPath []string) []InlineElement {
	var elements []InlineElement

	for _, loc := range inlineExpectPattern.FindAllStringSubmatchIndex(raw, -1) {
		if insideCodeSpan(raw, loc[0], loc[1]) {
			continue
		}
		*ordinal++
		expectFail := loc[6] >= 0 && loc[7] > loc[6]
		elements = append(elements, InlineElement{
			Kind:        InlineExpect,
			Raw:         raw[loc[0]:loc[1]],
			ExpectExpr:  strings.TrimSpace(raw[loc[2]:loc[3]]),
			ExpectValue: strings.TrimSpace(raw[loc[4]:loc[5]]),
			ExpectFail:  expectFail,
			ID: &SpecID{
				File:        relativePath,
				HeadingPath: append([]string(nil), headingPath...),
				Ordinal:     *ordinal,
			},
		})
	}

	for _, loc := range inlineCheckPattern.FindAllStringSubmatchIndex(raw, -1) {
		if insideCodeSpan(raw, loc[0], loc[1]) {
			continue
		}
		*ordinal++
		elements = append(elements, InlineElement{
			Kind:          InlineCheck,
			Raw:           raw[loc[0]:loc[1]],
			Check:       raw[loc[2]:loc[3]],
			CheckParams: parseCheckParams(raw[loc[4]:loc[5]]),
			ID: &SpecID{
				File:        relativePath,
				HeadingPath: append([]string(nil), headingPath...),
				Ordinal:     *ordinal,
			},
		})
	}

	return elements
}

// commentPrefixes maps common comment markers to check for summary lines.
var commentPrefixes = []string{"# ", "// ", "-- "}

// extractSummary collects consecutive comment lines at the start of source
// and joins them with a space. If the first line is not a comment, returns
// empty string.
func extractSummary(source string) string {
	if source == "" {
		return ""
	}
	lines := strings.Split(source, "\n")
	var parts []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			break
		}
		text, ok := stripCommentPrefix(trimmed)
		if !ok {
			break
		}
		if text == "" {
			break
		}
		parts = append(parts, text)
	}
	return strings.Join(parts, " ")
}

// stripCommentPrefix returns the text after a comment prefix and true,
// or empty string and false if the line is not a comment.
func stripCommentPrefix(line string) (string, bool) {
	for _, prefix := range commentPrefixes {
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(line[len(prefix):]), true
		}
	}
	return "", false
}

func nextHeadingPath(current []string, level int, text string) []string {
	if level <= 0 {
		return append([]string(nil), current...)
	}

	next := append([]string(nil), current...)
	if len(next) < level-1 {
		for len(next) < level-1 {
			next = append(next, "")
		}
	}
	if len(next) >= level {
		next = next[:level-1]
	}
	next = append(next, text)
	return next
}
