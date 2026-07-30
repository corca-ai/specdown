package binding

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/corca-ai/specdown/internal/specdown/core"
)

// Resolver resolves binding references while retaining structured values.
type Resolver struct {
	values map[string]any
}

// New creates a resolver from the currently visible bindings. Later bindings
// with the same name take precedence.
func New(bindings []core.Binding) Resolver {
	values := make(map[string]any, len(bindings))
	for _, item := range bindings {
		values[item.Name] = item.Value
	}
	return Resolver{values: values}
}

// UndefinedError reports a reference whose root binding is not available.
type UndefinedError struct {
	Name string
}

func (err *UndefinedError) Error() string {
	return fmt.Sprintf("binding %q is not defined", err.Name)
}

// Resolve returns the typed value addressed by a root or dotted reference.
func (resolver Resolver) Resolve(reference string) (any, error) {
	parts := strings.Split(reference, ".")
	rootName := parts[0]
	value, ok := resolver.values[rootName]
	if !ok {
		return nil, &UndefinedError{Name: rootName}
	}
	return ResolvePath(value, parts[1:])
}

// Names returns the available root binding names in deterministic order.
func (resolver Resolver) Names() []string {
	names := make([]string, 0, len(resolver.values))
	for name := range resolver.values {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ResolvePath traverses object keys without converting typed values to text.
func ResolvePath(value any, path []string) (any, error) {
	current := value
	for _, key := range path {
		object, ok := current.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("cannot access %q on non-object value", key)
		}
		next, exists := object[key]
		if !exists {
			return nil, fmt.Errorf("key %q not found", key)
		}
		current = next
	}
	return current, nil
}

// Format renders a binding value consistently for execution and presentation.
func Format(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case nil:
		return ""
	default:
		data, err := json.Marshal(typed)
		if err != nil {
			return fmt.Sprintf("%v", typed)
		}
		return string(data)
	}
}

type referenceMatch struct {
	start     int
	end       int
	reference string
	escaped   bool
}

func findReferences(input string) []referenceMatch {
	indexes := core.VariablePattern.FindAllStringSubmatchIndex(input, -1)
	matches := make([]referenceMatch, 0, len(indexes))
	for _, match := range indexes {
		matches = append(matches, referenceMatch{
			start:     match[0],
			end:       match[1],
			reference: input[match[4]:match[5]],
			escaped:   match[2] >= 0 && match[3] > match[2],
		})
	}
	return matches
}

// ReplaceReferences replaces unescaped ${...} references and turns \${...}
// into a literal ${...}. The callback owns unresolved-reference policy.
func ReplaceReferences(input string, replace func(reference string) (string, error)) (string, error) {
	matches := findReferences(input)
	if len(matches) == 0 {
		return input, nil
	}

	var output strings.Builder
	lastEnd := 0
	var replacementErr error
	for _, match := range matches {
		output.WriteString(input[lastEnd:match.start])
		if match.escaped {
			output.WriteString("${")
			output.WriteString(match.reference)
			output.WriteByte('}')
		} else {
			replacement, err := replace(match.reference)
			if err != nil {
				replacementErr = err
				output.WriteString(input[match.start:match.end])
			} else {
				output.WriteString(replacement)
			}
		}
		lastEnd = match.end
	}
	output.WriteString(input[lastEnd:])
	if replacementErr != nil {
		return "", replacementErr
	}
	return output.String(), nil
}

// UnescapeReferences turns escaped references into literal ${...} text without
// resolving unescaped references.
func UnescapeReferences(input string) string {
	matches := findReferences(input)
	if len(matches) == 0 {
		return input
	}

	var output strings.Builder
	lastEnd := 0
	for _, match := range matches {
		if !match.escaped {
			continue
		}
		output.WriteString(input[lastEnd:match.start])
		output.WriteString("${")
		output.WriteString(match.reference)
		output.WriteByte('}')
		lastEnd = match.end
	}
	if lastEnd == 0 {
		return input
	}
	output.WriteString(input[lastEnd:])
	return output.String()
}

type protectedReference struct {
	token   string
	literal string
}

// ProtectedReferences restores escaped variable literals after an intermediate
// transformation, such as Markdown rendering, that would consume backslashes.
type ProtectedReferences struct {
	items []protectedReference
}

// ProtectEscapedReferences replaces escaped references with opaque tokens.
func ProtectEscapedReferences(input string) (string, ProtectedReferences) {
	matches := findReferences(input)
	prefix := "SPECDOWNESCAPEDREFERENCETOKEN"
	for strings.Contains(input, prefix) {
		prefix = "X" + prefix
	}

	var output strings.Builder
	lastEnd := 0
	var protected ProtectedReferences
	for _, match := range matches {
		if !match.escaped {
			continue
		}
		output.WriteString(input[lastEnd:match.start])
		token := prefix + strconv.Itoa(len(protected.items)) + "END"
		output.WriteString(token)
		protected.items = append(protected.items, protectedReference{
			token:   token,
			literal: "${" + match.reference + "}",
		})
		lastEnd = match.end
	}
	if len(protected.items) == 0 {
		return input, protected
	}
	output.WriteString(input[lastEnd:])
	return output.String(), protected
}

// Restore replaces protected tokens with their literal ${...} references.
func (protected ProtectedReferences) Restore(input string) string {
	for _, item := range protected.items {
		input = strings.ReplaceAll(input, item.token, item.literal)
	}
	return input
}
