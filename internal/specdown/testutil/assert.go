// Package testutil provides shared test assertion helpers.
package testutil

import (
	"strings"
	"testing"
)

// Equal fails if got != want.
func Equal[T comparable](t *testing.T, got, want T) {
	t.Helper()
	if got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// NotEqual fails if got == want.
func NotEqual[T comparable](t *testing.T, got, want T) {
	t.Helper()
	if got == want {
		t.Fatalf("got %v, should differ from %v", got, want)
	}
}

// NilErr fails if err is not nil.
func NilErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// WantErr fails if err is nil.
func WantErr(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ErrContains fails if err is nil or doesn't contain substr.
func ErrContains(t *testing.T, err error, substr string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error containing %q, got nil", substr)
	}
	if !strings.Contains(err.Error(), substr) {
		t.Fatalf("error %q does not contain %q", err.Error(), substr)
	}
}

// Contains fails if s does not contain substr.
func Contains(t *testing.T, s, substr string) {
	t.Helper()
	if !strings.Contains(s, substr) {
		t.Fatalf("%q does not contain %q", s, substr)
	}
}

// NotContains fails if s contains substr.
func NotContains(t *testing.T, s, substr string) {
	t.Helper()
	if strings.Contains(s, substr) {
		t.Fatalf("%q should not contain %q", s, substr)
	}
}

// True fails if v is false.
func True(t *testing.T, v bool) {
	t.Helper()
	if !v {
		t.Fatal("expected true, got false")
	}
}

// False fails if v is true.
func False(t *testing.T, v bool) {
	t.Helper()
	if v {
		t.Fatal("expected false, got true")
	}
}

// Len fails if the slice length doesn't match.
func Len[T any](t *testing.T, slice []T, want int) {
	t.Helper()
	if len(slice) != want {
		t.Fatalf("len = %d, want %d", len(slice), want)
	}
}
