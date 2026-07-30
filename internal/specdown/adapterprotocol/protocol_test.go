package adapterprotocol

import (
	"strings"
	"testing"
)

type execResponseCase struct {
	name          string
	raw           string
	wantErr       string
	wantHasOutput bool
	wantOutput    string
	wantError     string
	wantExitCode  *int
	wantStderr    string
}

func TestParseExecResponseCompatibility(t *testing.T) {
	tests := []execResponseCase{
		{
			name:          "output",
			raw:           `{"id":1,"output":"hello"}`,
			wantHasOutput: true,
			wantOutput:    `"hello"`,
		},
		{
			name:          "structured output and unknown field",
			raw:           `{"id":2,"output":{"key":"value"},"extension":true}`,
			wantHasOutput: true,
			wantOutput:    `{"key":"value"}`,
		},
		{
			name:          "null output is present",
			raw:           `{"id":3,"output":null}`,
			wantHasOutput: true,
			wantOutput:    "null",
		},
		{
			name:         "error with optional diagnostics",
			raw:          `{"id":4,"error":"failed","exitCode":7,"stderr":"detail"}`,
			wantError:    "failed",
			wantExitCode: intPointer(7),
			wantStderr:   "detail",
		},
		{
			name:      "error without optional diagnostics",
			raw:       `{"id":5,"error":"failed"}`,
			wantError: "failed",
		},
		{
			name:      "malformed optional diagnostics keep compatible defaults",
			raw:       `{"id":6,"error":"failed","exitCode":"seven","stderr":7}`,
			wantError: "failed",
		},
		{name: "malformed JSON", raw: `not json`, wantErr: "decode exec response"},
		{name: "non-object JSON", raw: `null`, wantErr: "must be a JSON object"},
		{name: "missing id", raw: `{"output":"x"}`, wantErr: `must include "id"`},
		{name: "zero id", raw: `{"id":0,"output":"x"}`, wantErr: "positive integer"},
		{name: "negative id", raw: `{"id":-1,"output":"x"}`, wantErr: "positive integer"},
		{name: "string id", raw: `{"id":"1","output":"x"}`, wantErr: "decode exec response id"},
		{name: "fractional id", raw: `{"id":1.5,"output":"x"}`, wantErr: "decode exec response id"},
		{name: "both result keys", raw: `{"id":1,"output":"x","error":"y"}`, wantErr: "exactly one"},
		{name: "neither result key", raw: `{"id":1}`, wantErr: "exactly one"},
		{name: "non-string error", raw: `{"id":1,"error":7}`, wantErr: "decode exec response error"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response, err := ParseExecResponse([]byte(test.raw))
			assertProtocolResult(t, err, test.wantErr)
			if err == nil {
				assertExecResponse(t, response, test)
			}
		})
	}
}

func assertExecResponse(
	t *testing.T,
	response ExecResponse,
	want execResponseCase,
) {
	t.Helper()
	if response.HasOutput != want.wantHasOutput || string(response.Output) != want.wantOutput ||
		response.Error != want.wantError || response.Stderr != want.wantStderr {
		t.Fatalf("response = %+v, want output=%q error=%q stderr=%q", response, want.wantOutput, want.wantError, want.wantStderr)
	}
	if want.wantExitCode == nil {
		if response.ExitCode != nil {
			t.Fatalf("response exit code = %d, want none", *response.ExitCode)
		}
		return
	}
	if response.ExitCode == nil || *response.ExitCode != *want.wantExitCode {
		t.Fatalf("response exit code = %v, want %d", response.ExitCode, *want.wantExitCode)
	}
}

func intPointer(value int) *int {
	return &value
}

func TestParseAssertResponseCompatibility(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantErr string
		check   func(*testing.T, AssertResponse)
	}{
		{
			name: "passed with extension",
			raw:  `{"id":1,"type":"passed","label":"row 1","extension":{"value":true}}`,
			check: func(t *testing.T, response AssertResponse) {
				t.Helper()
				if response.ID != 1 || response.Type != AssertResponsePassed || response.Label != "row 1" {
					t.Fatalf("response = %+v, want passed response", response)
				}
			},
		},
		{
			name: "failed with diagnostics",
			raw:  `{"id":2,"type":"failed","message":"mismatch","expected":"yes","actual":"no"}`,
			check: func(t *testing.T, response AssertResponse) {
				t.Helper()
				if response.Type != AssertResponseFailed || response.Message != "mismatch" ||
					response.Expected != "yes" || response.Actual != "no" {
					t.Fatalf("response = %+v, want failed diagnostics", response)
				}
			},
		},
		{
			name: "wrong-case optional field is an ignored extension",
			raw:  `{"id":3,"type":"passed","Message":7}`,
			check: func(t *testing.T, response AssertResponse) {
				t.Helper()
				if response.Message != "" {
					t.Fatalf("response = %+v, want wrong-case extension ignored", response)
				}
			},
		},
		{name: "malformed JSON", raw: `{`, wantErr: "decode assert response"},
		{name: "non-object JSON", raw: `null`, wantErr: "must be a JSON object"},
		{name: "missing id", raw: `{"type":"passed"}`, wantErr: `must include "id"`},
		{name: "zero id", raw: `{"id":0,"type":"passed"}`, wantErr: "positive integer"},
		{name: "negative id", raw: `{"id":-1,"type":"passed"}`, wantErr: "positive integer"},
		{name: "string id", raw: `{"id":"1","type":"passed"}`, wantErr: "decode assert response id"},
		{name: "fractional id", raw: `{"id":1.5,"type":"passed"}`, wantErr: "decode assert response id"},
		{name: "missing type", raw: `{"id":1}`, wantErr: `must include "type"`},
		{name: "wrong-case type key", raw: `{"id":1,"Type":"passed"}`, wantErr: `must include "type"`},
		{name: "unknown type", raw: `{"id":1,"type":"maybe"}`, wantErr: "must be \"passed\" or \"failed\""},
		{name: "wrong-case alias cannot override unknown type", raw: `{"id":1,"type":"maybe","TYPE":"passed"}`, wantErr: "must be \"passed\" or \"failed\""},
		{name: "non-string type", raw: `{"id":1,"type":7}`, wantErr: "decode assert response type"},
		{name: "malformed optional field", raw: `{"id":1,"type":"failed","message":7}`, wantErr: "decode assert response message"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response, err := ParseAssertResponse([]byte(test.raw))
			assertProtocolResult(t, err, test.wantErr)
			if err == nil && test.check != nil {
				test.check(t, response)
			}
		})
	}
}

func assertProtocolResult(t *testing.T, err error, wantErr string) {
	t.Helper()
	if wantErr == "" {
		if err != nil {
			t.Fatalf("parse response: %v", err)
		}
		return
	}
	if err == nil || !strings.Contains(err.Error(), wantErr) {
		t.Fatalf("error = %v, want it to contain %q", err, wantErr)
	}
}
