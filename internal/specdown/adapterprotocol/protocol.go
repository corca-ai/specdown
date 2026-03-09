package adapterprotocol

import (
	"encoding/json"
	"fmt"
)

// ExecRequest asks the adapter to execute source code.
type ExecRequest struct {
	Type   string `json:"type"`   // "exec"
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

// ParseExecResponse parses a raw JSON line into an ExecResponse.
// It uses key-presence detection: "output" means success, "error" means failure.
func ParseExecResponse(raw []byte) (ExecResponse, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return ExecResponse{}, fmt.Errorf("decode exec response: %w", err)
	}

	var resp ExecResponse

	if idRaw, ok := fields["id"]; ok {
		if err := json.Unmarshal(idRaw, &resp.ID); err != nil {
			return ExecResponse{}, fmt.Errorf("decode exec response id: %w", err)
		}
	}

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

	return resp, nil
}
