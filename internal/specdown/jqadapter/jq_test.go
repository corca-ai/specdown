package jqadapter

import (
	"testing"

	"github.com/corca-ai/specdown/internal/specdown/adapterprotocol"
)

func makeReq(params map[string]string, columns, cells []string) *adapterprotocol.AssertRequest {
	return &adapterprotocol.AssertRequest{
		Check:       "jq",
		CheckParams: params,
		Columns:     columns,
		Cells:       cells,
	}
}

func TestAssertStringField(t *testing.T) {
	resp := Assert(1, makeReq(
		map[string]string{"input": `{"name":"Alice"}`},
		[]string{"expr", "expected"},
		[]string{".name", "Alice"},
	))
	if resp.Type != "passed" {
		t.Fatalf("expected passed, got %s: %s", resp.Type, resp.Message)
	}
}

func TestAssertNumberField(t *testing.T) {
	resp := Assert(1, makeReq(
		map[string]string{"input": `{"age":30}`},
		[]string{"expr", "expected"},
		[]string{".age", "30"},
	))
	if resp.Type != "passed" {
		t.Fatalf("expected passed, got %s: %s", resp.Type, resp.Message)
	}
}

func TestAssertBooleanField(t *testing.T) {
	resp := Assert(1, makeReq(
		map[string]string{"input": `{"active":true}`},
		[]string{"expr", "expected"},
		[]string{".active", "true"},
	))
	if resp.Type != "passed" {
		t.Fatalf("expected passed, got %s: %s", resp.Type, resp.Message)
	}
}

func TestAssertNullField(t *testing.T) {
	resp := Assert(1, makeReq(
		map[string]string{"input": `{"x":null}`},
		[]string{"expr", "expected"},
		[]string{".x", "null"},
	))
	if resp.Type != "passed" {
		t.Fatalf("expected passed, got %s: %s", resp.Type, resp.Message)
	}
}

func TestAssertMissingFieldIsNull(t *testing.T) {
	resp := Assert(1, makeReq(
		map[string]string{"input": `{}`},
		[]string{"expr", "expected"},
		[]string{".missing", "null"},
	))
	if resp.Type != "passed" {
		t.Fatalf("expected passed, got %s: %s", resp.Type, resp.Message)
	}
}

func TestAssertArrayNormalized(t *testing.T) {
	resp := Assert(1, makeReq(
		map[string]string{"input": `{"items":[1,2,3]}`},
		[]string{"expr", "expected"},
		[]string{".items", "[1, 2, 3]"},
	))
	if resp.Type != "passed" {
		t.Fatalf("expected passed, got %s (actual=%q expected=%q)", resp.Type, resp.Actual, resp.Expected)
	}
}

func TestAssertObjectKeyOrderInsensitive(t *testing.T) {
	resp := Assert(1, makeReq(
		map[string]string{"input": `{"meta":{"b":2,"a":1}}`},
		[]string{"expr", "expected"},
		[]string{".meta", `{"a":1,"b":2}`},
	))
	if resp.Type != "passed" {
		t.Fatalf("expected passed, got %s (actual=%q)", resp.Type, resp.Actual)
	}
}

func TestAssertNestedPath(t *testing.T) {
	resp := Assert(1, makeReq(
		map[string]string{"input": `{"users":[{"name":"Alice"},{"name":"Bob"}]}`},
		[]string{"expr", "expected"},
		[]string{".users[1].name", "Bob"},
	))
	if resp.Type != "passed" {
		t.Fatalf("expected passed, got %s: %s", resp.Type, resp.Message)
	}
}

func TestAssertJQExpression(t *testing.T) {
	resp := Assert(1, makeReq(
		map[string]string{"input": `{"items":[1,2,3]}`},
		[]string{"expr", "expected"},
		[]string{".items | length", "3"},
	))
	if resp.Type != "passed" {
		t.Fatalf("expected passed, got %s: %s", resp.Type, resp.Message)
	}
}

func TestAssertBooleanExpression(t *testing.T) {
	resp := Assert(1, makeReq(
		map[string]string{"input": `{"items":[1,2,3]}`},
		[]string{"expr", "expected"},
		[]string{".items | length > 0", "true"},
	))
	if resp.Type != "passed" {
		t.Fatalf("expected passed, got %s: %s", resp.Type, resp.Message)
	}
}

func TestAssertFailure(t *testing.T) {
	resp := Assert(1, makeReq(
		map[string]string{"input": `{"name":"Alice"}`},
		[]string{"expr", "expected"},
		[]string{".name", "Bob"},
	))
	if resp.Type != "failed" {
		t.Fatalf("expected failed, got %s", resp.Type)
	}
	if resp.Expected != "Bob" {
		t.Fatalf("expected Expected=Bob, got %q", resp.Expected)
	}
	if resp.Actual != "Alice" {
		t.Fatalf("expected Actual=Alice, got %q", resp.Actual)
	}
}

func TestAssertInvalidJSON(t *testing.T) {
	resp := Assert(1, makeReq(
		map[string]string{"input": "not json"},
		[]string{"expr", "expected"},
		[]string{".name", "Alice"},
	))
	if resp.Type != "failed" {
		t.Fatalf("expected failed, got %s", resp.Type)
	}
	if resp.Message == "" {
		t.Fatal("expected error message")
	}
}

func TestAssertInvalidExpr(t *testing.T) {
	resp := Assert(1, makeReq(
		map[string]string{"input": `{"a":1}`},
		[]string{"expr", "expected"},
		[]string{".[invalid", "1"},
	))
	if resp.Type != "failed" {
		t.Fatalf("expected failed, got %s", resp.Type)
	}
}

func TestAssertMissingInput(t *testing.T) {
	resp := Assert(1, makeReq(
		nil,
		[]string{"expr", "expected"},
		[]string{".name", "Alice"},
	))
	if resp.Type != "failed" {
		t.Fatalf("expected failed, got %s", resp.Type)
	}
}

func TestAssertMissingExpr(t *testing.T) {
	resp := Assert(1, makeReq(
		map[string]string{"input": `{"a":1}`},
		[]string{"expected"},
		[]string{"1"},
	))
	if resp.Type != "failed" {
		t.Fatalf("expected failed, got %s", resp.Type)
	}
}

func TestAssertInputFromColumn(t *testing.T) {
	resp := Assert(1, makeReq(
		nil,
		[]string{"input", "expr", "expected"},
		[]string{`{"name":"Alice"}`, ".name", "Alice"},
	))
	if resp.Type != "passed" {
		t.Fatalf("expected passed, got %s: %s", resp.Type, resp.Message)
	}
}

func TestAssertMultipleResults(t *testing.T) {
	resp := Assert(1, makeReq(
		map[string]string{"input": `{"a":1,"b":2}`},
		[]string{"expr", "expected"},
		[]string{".a, .b", "1\n2"},
	))
	if resp.Type != "passed" {
		t.Fatalf("expected passed, got %s (actual=%q)", resp.Type, resp.Actual)
	}
}

func TestAssertColumnOverridesParam(t *testing.T) {
	resp := Assert(1, makeReq(
		map[string]string{"input": `{"name":"FromParam"}`},
		[]string{"input", "expr", "expected"},
		[]string{`{"name":"FromColumn"}`, ".name", "FromColumn"},
	))
	if resp.Type != "passed" {
		t.Fatalf("expected passed, got %s: %s", resp.Type, resp.Message)
	}
}
