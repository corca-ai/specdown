package core

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

func readDocument(baseDir string, relativePath string) (Document, error) {
	fullPath := filepath.Join(baseDir, filepath.FromSlash(relativePath))
	body, err := os.ReadFile(fullPath)
	if err != nil {
		return Document{}, fmt.Errorf("read %s: %w", fullPath, err)
	}
	return ParseDocument(relativePath, string(body))
}

func ParseDocument(relativePath string, markdown string) (Document, error) {
	lines := splitLines(markdown)

	var (
		nodes       []Node
		headingPath []string
		title       string
		ordinal     int
		fixture     string
	)

	for i := 0; i < len(lines); {
		line := lines[i]

		if ref, ok, err := parseAlloyRefDirective(line); err != nil {
			return Document{}, fmt.Errorf("%s: %w", relativePath, err)
		} else if ok {
			if fixture != "" {
				return Document{}, fmt.Errorf("%s: fixture directive %q must be followed by a table", relativePath, fixture)
			}
			ordinal++
			ref.Raw = line
			ref.HeadingPath = append([]string(nil), headingPath...)
			ref.ID = &SpecID{
				File:        relativePath,
				HeadingPath: append([]string(nil), headingPath...),
				Ordinal:     ordinal,
			}
			nodes = append(nodes, ref)
			i++
			continue
		}

		if nextFixture, ok := parseFixtureDirective(line); ok {
			if fixture != "" {
				return Document{}, fmt.Errorf("%s: fixture directive %q must be followed by a table", relativePath, fixture)
			}
			fixture = nextFixture
			i++
			continue
		}

		if isFenceStart(line) {
			if fixture != "" {
				return Document{}, fmt.Errorf("%s: fixture directive %q must be followed by a table", relativePath, fixture)
			}
			info := parseFenceInfo(line)

			if modelName, ok := parseAlloyModelInfo(info); ok {
				end := -1
				for j := i + 1; j < len(lines); j++ {
					if isFenceEnd(lines[j]) {
						end = j
						break
					}
				}
				if end == -1 {
					return Document{}, fmt.Errorf("%s: unclosed fenced code block", relativePath)
				}

				raw := strings.Join(lines[i:end+1], "")
				source := strings.Join(lines[i+1:end], "")
				nodes = append(nodes, AlloyModelNode{
					Model:       modelName,
					Source:      strings.TrimSuffix(source, "\n"),
					Raw:         raw,
					HeadingPath: append([]string(nil), headingPath...),
				})
				i = end + 1
				continue
			}

			block, err := parseBlockSpec(info)
			if err != nil {
				return Document{}, fmt.Errorf("%s: %w", relativePath, err)
			}

			end := -1
			for j := i + 1; j < len(lines); j++ {
				if isFenceEnd(lines[j]) {
					end = j
					break
				}
			}
			if end == -1 {
				return Document{}, fmt.Errorf("%s: unclosed fenced code block", relativePath)
			}

			raw := strings.Join(lines[i:end+1], "")
			source := strings.Join(lines[i+1:end], "")
			node := CodeBlockNode{
				Block:  block,
				Source: strings.TrimSuffix(source, "\n"),
				Raw:    raw,
			}
			if block.Executable() {
				ordinal++
				node.ID = &SpecID{
					File:        relativePath,
					HeadingPath: append([]string(nil), headingPath...),
					Ordinal:     ordinal,
				}
			}
			nodes = append(nodes, node)
			i = end + 1
			continue
		}

		if isTableStart(lines, i) {
			table, next, err := parseTableNode(relativePath, lines, i, fixture, &ordinal, headingPath)
			if err != nil {
				return Document{}, err
			}
			nodes = append(nodes, table)
			fixture = ""
			i = next
			continue
		}

		if level, text, ok := parseHeading(line); ok {
			if fixture != "" {
				return Document{}, fmt.Errorf("%s: fixture directive %q must be followed by a table", relativePath, fixture)
			}
			if title == "" {
				title = text
			}
			headingPath = nextHeadingPath(headingPath, level, text)
			nodes = append(nodes, HeadingNode{
				Level:       level,
				Text:        text,
				Raw:         line,
				HeadingPath: append([]string(nil), headingPath...),
			})
			i++
			continue
		}

		start := i
		for i < len(lines) && !isFenceStart(lines[i]) {
			if _, _, ok := parseHeading(lines[i]); ok {
				break
			}
			if _, ok, err := parseAlloyRefDirective(lines[i]); err != nil {
				return Document{}, fmt.Errorf("%s: %w", relativePath, err)
			} else if ok {
				break
			}
			if _, ok := parseFixtureDirective(lines[i]); ok {
				break
			}
			if isTableStart(lines, i) {
				break
			}
			i++
		}

		raw := strings.Join(lines[start:i], "")
		if strings.TrimSpace(raw) == "" {
			continue
		}
		if fixture != "" {
			return Document{}, fmt.Errorf("%s: fixture directive %q must be followed by a table", relativePath, fixture)
		}
		nodes = append(nodes, ProseNode{Raw: raw})
	}

	if fixture != "" {
		return Document{}, fmt.Errorf("%s: fixture directive %q must be followed by a table", relativePath, fixture)
	}

	if title == "" {
		title = relativePath
	}

	return Document{
		RelativeTo: relativePath,
		Title:      title,
		Markdown:   markdown,
		Nodes:      nodes,
	}, nil
}

func walkFiles(baseDir string, visit func(relativePath string) error) error {
	root, err := filepath.Abs(baseDir)
	if err != nil {
		return fmt.Errorf("resolve base dir: %w", err)
	}

	var files []string
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		relativePath, err := filepath.Rel(root, path)
		if err != nil {
			return fmt.Errorf("relative path for %s: %w", path, err)
		}
		files = append(files, filepath.ToSlash(relativePath))
		return nil
	})
	if err != nil {
		return fmt.Errorf("discover specs: %w", err)
	}

	sort.Strings(files)
	for _, relativePath := range files {
		if err := visit(relativePath); err != nil {
			return err
		}
	}
	return nil
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

var fixtureDirectivePattern = regexp.MustCompile(`^\s*<!--\s*fixture:([A-Za-z0-9_-]+)\s*-->\s*$`)
var alloyModelInfoPattern = regexp.MustCompile(`^alloy:model\(([A-Za-z_][A-Za-z0-9_-]*)\)$`)
var alloyRefDirectivePattern = regexp.MustCompile(`^\s*<!--\s*alloy:ref\(([A-Za-z_][A-Za-z0-9_-]*)#([A-Za-z_][A-Za-z0-9_]*),\s*scope=([^)]+)\)\s*-->\s*$`)

func parseFixtureDirective(line string) (string, bool) {
	matches := fixtureDirectivePattern.FindStringSubmatch(line)
	if matches == nil {
		return "", false
	}
	return matches[1], true
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
	if !strings.HasPrefix(trimmed, "<!--") || !strings.Contains(trimmed, "alloy:ref(") {
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

func parseTableNode(relativePath string, lines []string, start int, fixture string, ordinal *int, headingPath []string) (TableNode, int, error) {
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
		if fixture != "" {
			*ordinal = *ordinal + 1
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
		Fixture: fixture,
		Columns: columns,
		Rows:    rows,
		Raw:     strings.Join(lines[start:end], ""),
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
		trimmed := strings.ReplaceAll(strings.TrimSpace(cell), " ", "")
		if trimmed == "" {
			return false
		}
		hasDash := false
		for _, r := range trimmed {
			if r == '-' {
				hasDash = true
				continue
			}
			if r != ':' {
				return false
			}
		}
		if !hasDash {
			return false
		}
	}
	return true
}

func parseTableCells(line string) ([]string, error) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "|") {
		return nil, fmt.Errorf("invalid markdown table row %q", strings.TrimSpace(line))
	}
	trimmed = strings.TrimPrefix(trimmed, "|")
	trimmed = strings.TrimSuffix(trimmed, "|")
	parts := strings.Split(trimmed, "|")
	cells := make([]string, 0, len(parts))
	for _, part := range parts {
		cells = append(cells, strings.TrimSpace(part))
	}
	return cells, nil
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
