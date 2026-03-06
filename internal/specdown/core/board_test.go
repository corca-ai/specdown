package core

import "testing"

func TestBoardRuntimeVerifySupportsExistenceChecks(t *testing.T) {
	runtime := newBoardRuntime()

	if err := runtime.Run("create-board \"demo\""); err != nil {
		t.Fatalf("run create-board: %v", err)
	}
	if err := runtime.Verify("board \"demo\" should exist"); err != nil {
		t.Fatalf("verify existing board: %v", err)
	}
	if err := runtime.Verify("board \"archive\" should not exist"); err != nil {
		t.Fatalf("verify missing board: %v", err)
	}
}

func TestBoardRuntimeVerifyReturnsExpectedActualMessage(t *testing.T) {
	runtime := newBoardRuntime()

	if err := runtime.Run("create-board \"demo\""); err != nil {
		t.Fatalf("run create-board: %v", err)
	}
	err := runtime.Verify("board \"archive\" should exist")
	if err == nil {
		t.Fatal("expected verification failure")
	}
	if got, want := err.Error(), "expected board \"archive\" to exist; actual boards: [\"demo\"]"; got != want {
		t.Fatalf("unexpected error %q", got)
	}
}

func TestParseBoardAssertionParsesQuotedAndNegativeForms(t *testing.T) {
	assertion, err := parseBoardAssertion("board \"demo\" should exist")
	if err != nil {
		t.Fatalf("parse positive assertion: %v", err)
	}
	if assertion.Name != "demo" || !assertion.ShouldExist {
		t.Fatalf("unexpected positive assertion %#v", assertion)
	}

	assertion, err = parseBoardAssertion("board archive should not exist")
	if err != nil {
		t.Fatalf("parse negative assertion: %v", err)
	}
	if assertion.Name != "archive" || assertion.ShouldExist {
		t.Fatalf("unexpected negative assertion %#v", assertion)
	}
}
