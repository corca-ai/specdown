package jqadapter

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/itchyny/gojq"

	"github.com/corca-ai/specdown/internal/specdown/adapterprotocol"
)

// Assert evaluates a jq expression against JSON input and compares with expected.
//
// The request columns/cells or checkParams must provide:
//   - input: JSON string to evaluate against
//   - expr:  jq expression
//   - expected: expected result
//
// Comparison: jq raw output for scalars (strings unquoted, others as JSON).
// Arrays and objects are normalized (sorted keys, compact) before comparison.
func Assert(id int, req *adapterprotocol.AssertRequest) adapterprotocol.AssertResponse {
	if req == nil {
		return adapterprotocol.AssertResponse{
			ID:      id,
			Type:    "failed",
			Message: "missing assert request",
		}
	}

	params := extractParams(req)
	input := params["input"]
	expr := params["expr"]
	expected := params["expected"]

	if input == "" {
		return adapterprotocol.AssertResponse{
			ID:      id,
			Type:    "failed",
			Message: "missing required parameter: input",
		}
	}
	if expr == "" {
		return adapterprotocol.AssertResponse{
			ID:      id,
			Type:    "failed",
			Message: "missing required parameter: expr",
		}
	}

	actual, err := evalJQ(input, expr)
	if err != nil {
		return adapterprotocol.AssertResponse{
			ID:      id,
			Type:    "failed",
			Message: err.Error(),
		}
	}

	normalizedActual := normalizeJSON(actual)
	normalizedExpected := normalizeJSON(expected)

	if normalizedActual == normalizedExpected {
		return adapterprotocol.AssertResponse{
			ID:     id,
			Type:   "passed",
			Actual: actual,
		}
	}

	return adapterprotocol.AssertResponse{
		ID:       id,
		Type:     "failed",
		Expected: expected,
		Actual:   actual,
	}
}

// extractParams merges check params and table columns/cells into a single map.
// Table columns take precedence over check params.
func extractParams(req *adapterprotocol.AssertRequest) map[string]string {
	params := make(map[string]string)
	for k, v := range req.CheckParams {
		params[k] = v
	}
	for i, col := range req.Columns {
		if i < len(req.Cells) {
			params[col] = req.Cells[i]
		}
	}
	return params
}

// evalJQ parses the input as JSON, evaluates the jq expression, and returns
// the result as a string. Multiple results are joined with newlines.
func evalJQ(input, expr string) (string, error) {
	query, err := gojq.Parse(expr)
	if err != nil {
		return "", fmt.Errorf("jq parse error: %w", err)
	}

	var inputValue interface{}
	if err := json.Unmarshal([]byte(input), &inputValue); err != nil {
		return "", fmt.Errorf("invalid JSON input: %w", err)
	}

	iter := query.Run(inputValue)
	var results []string
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, isErr := v.(error); isErr {
			return "", fmt.Errorf("jq error: %w", err)
		}
		results = append(results, formatValue(v))
	}

	return strings.Join(results, "\n"), nil
}

// formatValue converts a jq result to string using raw output semantics:
// strings are unquoted, everything else is JSON.
func formatValue(v interface{}) string {
	if v == nil {
		return "null"
	}
	if s, ok := v.(string); ok {
		return s
	}
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}

// normalizeJSON attempts to parse s as JSON. If it's an array or object,
// re-serializes with sorted keys in compact form. Otherwise returns s unchanged.
func normalizeJSON(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}

	var v interface{}
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return s
	}

	switch v.(type) {
	case []interface{}, map[string]interface{}:
		normalized, err := marshalSorted(v)
		if err != nil {
			return s
		}
		return string(normalized)
	default:
		return s
	}
}

// marshalSorted produces JSON with map keys sorted alphabetically.
//
//nolint:gocognit // recursive type-switch over JSON value types
func marshalSorted(v interface{}) ([]byte, error) {
	switch val := v.(type) {
	case map[string]interface{}:
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		var buf strings.Builder
		buf.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				buf.WriteByte(',')
			}
			keyBytes, _ := json.Marshal(k)
			buf.Write(keyBytes)
			buf.WriteByte(':')
			valBytes, err := marshalSorted(val[k])
			if err != nil {
				return nil, err
			}
			buf.Write(valBytes)
		}
		buf.WriteByte('}')
		return []byte(buf.String()), nil

	case []interface{}:
		var buf strings.Builder
		buf.WriteByte('[')
		for i, item := range val {
			if i > 0 {
				buf.WriteByte(',')
			}
			itemBytes, err := marshalSorted(item)
			if err != nil {
				return nil, err
			}
			buf.Write(itemBytes)
		}
		buf.WriteByte(']')
		return []byte(buf.String()), nil

	default:
		return json.Marshal(v)
	}
}
