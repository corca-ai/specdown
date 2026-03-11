package adapterprotocol

import (
	"testing"
)

func TestParseExecResponse_ValidOutput(t *testing.T) {
	resp, err := ParseExecResponse([]byte(`{"id": 1, "output": "hello"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.HasOutput {
		t.Error("expected HasOutput=true")
	}
	if resp.ID != 1 {
		t.Errorf("expected id=1, got %d", resp.ID)
	}
}

func TestParseExecResponse_ValidError(t *testing.T) {
	resp, err := ParseExecResponse([]byte(`{"id": 2, "error": "fail"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.HasOutput {
		t.Error("expected HasOutput=false")
	}
	if resp.Error != "fail" {
		t.Errorf("expected error=fail, got %q", resp.Error)
	}
}

func TestParseExecResponse_BothKeys(t *testing.T) {
	_, err := ParseExecResponse([]byte(`{"output": "x", "error": "y"}`))
	if err == nil {
		t.Error("expected error for both keys")
	}
}

func TestParseExecResponse_NeitherKey(t *testing.T) {
	_, err := ParseExecResponse([]byte(`{"id": 1}`))
	if err == nil {
		t.Error("expected error for neither key")
	}
}

func TestParseExecResponse_InvalidJSON(t *testing.T) {
	_, err := ParseExecResponse([]byte(`not json`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParseExecResponse_StructuredOutput(t *testing.T) {
	resp, err := ParseExecResponse([]byte(`{"id": 3, "output": {"key": "value"}}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.HasOutput {
		t.Error("expected HasOutput=true")
	}
	if string(resp.Output) != `{"key": "value"}` {
		t.Errorf("unexpected output: %s", resp.Output)
	}
}
