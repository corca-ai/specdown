package adapterprotocol

import (
	"encoding/json"
	"fmt"
)

// ExecRequest asks the adapter to execute source code.
type ExecRequest struct {
	Type   string `json:"type"` // "exec"
	ID     int    `json:"id"`
	Source string `json:"source"`
}

// AssertRequest asks the adapter to run a check.
type AssertRequest struct {
	Type        string            `json:"type"` // "assert"
	ID          int               `json:"id"`
	Check       string            `json:"check"`
	CheckParams map[string]string `json:"checkParams,omitempty"`
	Columns     []string          `json:"columns,omitempty"`
	Cells       []string          `json:"cells,omitempty"`
}

// ExecResponse — parsed via map[string]json.RawMessage to detect key presence.
// "output" key present → success (value can be any JSON: "", null, {}, etc.)
// "error" key present → failure
// Both present or both absent → protocol error
type ExecResponse struct {
	ID        int
	HasOutput bool
	Output    json.RawMessage // raw JSON value
	Error     string
	ExitCode  *int   // optional; nil when not reported by adapter
	Stderr    string // optional; separate stderr stream when adapter reports it
}

// AssertResponse is the adapter's response to an assert request.
type AssertResponse struct {
	ID       int    `json:"id"`
	Type     string `json:"type"` // "passed" or "failed"
	Message  string `json:"message,omitempty"`
	Expected string `json:"expected,omitempty"`
	Actual   string `json:"actual,omitempty"`
	Label    string `json:"label,omitempty"`
}

const (
	AssertResponsePassed = "passed"
	AssertResponseFailed = "failed"
)

// ParseExecResponse parses a raw JSON line into an ExecResponse.
// It uses key-presence detection: "output" means success, "error" means failure.
func ParseExecResponse(raw []byte) (ExecResponse, error) {
	fields, err := parseResponseFields(raw, "exec")
	if err != nil {
		return ExecResponse{}, err
	}
	id, err := parseResponseID(fields, "exec")
	if err != nil {
		return ExecResponse{}, err
	}
	resp := ExecResponse{ID: id}

	outputRaw, hasOutput := fields["output"]
	errorRaw, hasError := fields["error"]

	if hasOutput == hasError {
		return ExecResponse{}, fmt.Errorf("exec response must have exactly one of \"output\" or \"error\" keys")
	}

	if hasOutput {
		resp.HasOutput = true
		resp.Output = outputRaw
	} else {
		var errMsg string
		if err := json.Unmarshal(errorRaw, &errMsg); err != nil {
			return ExecResponse{}, fmt.Errorf("decode exec response error: %w", err)
		}
		resp.Error = errMsg
	}

	// Optional fields — backward-compatible; missing keys are silently ignored.
	if raw, ok := fields["exitCode"]; ok {
		var code int
		if err := json.Unmarshal(raw, &code); err == nil {
			resp.ExitCode = &code
		}
	}
	if raw, ok := fields["stderr"]; ok {
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			resp.Stderr = s
		}
	}

	return resp, nil
}

// ParseAssertResponse parses and validates a raw assert response.
// Unknown fields are ignored so adapters can add optional data compatibly.
func ParseAssertResponse(raw []byte) (AssertResponse, error) {
	fields, err := parseResponseFields(raw, "assert")
	if err != nil {
		return AssertResponse{}, err
	}
	id, err := parseResponseID(fields, "assert")
	if err != nil {
		return AssertResponse{}, err
	}
	typeRaw, ok := fields["type"]
	if !ok {
		return AssertResponse{}, fmt.Errorf("assert response must include \"type\"")
	}
	var responseType *string
	if err := json.Unmarshal(typeRaw, &responseType); err != nil || responseType == nil {
		return AssertResponse{}, fmt.Errorf("decode assert response type: must be a string")
	}
	resp := AssertResponse{ID: id, Type: *responseType}
	switch resp.Type {
	case AssertResponsePassed, AssertResponseFailed:
	default:
		return AssertResponse{}, fmt.Errorf(
			"assert response type must be %q or %q, got %q",
			AssertResponsePassed,
			AssertResponseFailed,
			resp.Type,
		)
	}
	for _, field := range []struct {
		key         string
		destination *string
	}{
		{key: "message", destination: &resp.Message},
		{key: "expected", destination: &resp.Expected},
		{key: "actual", destination: &resp.Actual},
		{key: "label", destination: &resp.Label},
	} {
		if err := decodeOptionalString(fields, field.key, field.destination); err != nil {
			return AssertResponse{}, err
		}
	}
	return resp, nil
}

func parseResponseFields(raw []byte, responseKind string) (map[string]json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, fmt.Errorf("decode %s response: %w", responseKind, err)
	}
	if fields == nil {
		return nil, fmt.Errorf("%s response must be a JSON object", responseKind)
	}
	return fields, nil
}

func parseResponseID(fields map[string]json.RawMessage, responseKind string) (int, error) {
	idRaw, ok := fields["id"]
	if !ok {
		return 0, fmt.Errorf("%s response must include \"id\"", responseKind)
	}
	var id int
	if err := json.Unmarshal(idRaw, &id); err != nil {
		return 0, fmt.Errorf("decode %s response id: %w", responseKind, err)
	}
	if id <= 0 {
		return 0, fmt.Errorf("%s response id must be a positive integer", responseKind)
	}
	return id, nil
}

func decodeOptionalString(fields map[string]json.RawMessage, key string, destination *string) error {
	raw, ok := fields[key]
	if !ok {
		return nil
	}
	if err := json.Unmarshal(raw, destination); err != nil {
		return fmt.Errorf("decode assert response %s: %w", key, err)
	}
	return nil
}
