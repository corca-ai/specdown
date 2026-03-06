package core

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func Discover(specRoot string) ([]Document, error) {
	root, err := filepath.Abs(specRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve spec root: %w", err)
	}

	var docs []Document
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".spec.md") {
			return nil
		}

		doc, err := readDocument(root, path)
		if err != nil {
			return err
		}
		docs = append(docs, doc)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("discover specs: %w", err)
	}

	return docs, nil
}

func readDocument(root string, path string) (Document, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return Document{}, fmt.Errorf("read %s: %w", path, err)
	}

	relPath, err := filepath.Rel(root, path)
	if err != nil {
		return Document{}, fmt.Errorf("relative path for %s: %w", path, err)
	}

	return ParseDocument(filepath.ToSlash(relPath), string(body))
}

func ParseDocument(relativePath string, markdown string) (Document, error) {
	lines := splitLines(markdown)

	var (
		nodes       []Node
		headingPath []string
		title       string
		ordinal     int
	)

	for i := 0; i < len(lines); {
		line := lines[i]

		if isFenceStart(line) {
			info := parseFenceInfo(line)
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

		if level, text, ok := parseHeading(line); ok {
			if title == "" {
				title = text
			}
			headingPath = nextHeadingPath(headingPath, level, text)
			nodes = append(nodes, HeadingNode{
				Level: level,
				Text:  text,
				Raw:   line,
			})
			i++
			continue
		}

		start := i
		for i < len(lines) && !isFenceStart(lines[i]) {
			if _, _, ok := parseHeading(lines[i]); ok {
				break
			}
			i++
		}

		raw := strings.Join(lines[start:i], "")
		if strings.TrimSpace(raw) == "" {
			continue
		}
		nodes = append(nodes, ProseNode{Raw: raw})
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
