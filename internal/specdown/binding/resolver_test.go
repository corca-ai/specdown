package binding

import (
	"errors"
	"strings"
	"testing"

	"github.com/corca-ai/specdown/internal/specdown/core"
)

func testResolver() Resolver {
	return New([]core.Binding{
		{Name: "text", Value: "hello"},
		{Name: "number", Value: float64(42)},
		{Name: "enabled", Value: true},
		{Name: "nothing", Value: nil},
		{Name: "user", Value: map[string]any{
			"name": "alice",
			"profile": map[string]any{
				"admin": true,
			},
		}},
	})
}

func TestResolverFormatsValues(t *testing.T) {
	resolver := testResolver()

	tests := []struct {
		name      string
		reference string
		want      string
	}{
		{name: "string", reference: "text", want: "hello"},
		{name: "number", reference: "number", want: "42"},
		{name: "boolean", reference: "enabled", want: "true"},
		{name: "null", reference: "nothing", want: ""},
		{name: "object", reference: "user", want: `{"name":"alice","profile":{"admin":true}}`},
		{name: "nested string", reference: "user.name", want: "alice"},
		{name: "nested boolean", reference: "user.profile.admin", want: "true"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value, err := resolver.Resolve(test.reference)
			if err != nil {
				t.Fatalf("Resolve(%q): %v", test.reference, err)
			}
			if got := Format(value); got != test.want {
				t.Errorf("Format(Resolve(%q)) = %q, want %q", test.reference, got, test.want)
			}
		})
	}
}

func TestResolverErrors(t *testing.T) {
	resolver := New([]core.Binding{
		{Name: "text", Value: "hello"},
		{Name: "nothing", Value: nil},
		{Name: "user", Value: map[string]any{"name": "alice"}},
	})

	tests := []struct {
		name        string
		reference   string
		wantError   string
		isUndefined bool
	}{
		{name: "missing root", reference: "missing", wantError: `binding "missing" is not defined`, isUndefined: true},
		{name: "missing key", reference: "user.missing", wantError: `key "missing" not found`},
		{name: "invalid traversal", reference: "text.name", wantError: `cannot access "name" on non-object value`},
		{name: "null traversal", reference: "nothing.name", wantError: `cannot access "name" on non-object value`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := resolver.Resolve(test.reference)
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("Resolve(%q) error = %v, want %q", test.reference, err, test.wantError)
			}
			var undefined *UndefinedError
			if errors.As(err, &undefined) != test.isUndefined {
				t.Fatalf("Resolve(%q) undefined error = %v, want %v", test.reference, errors.As(err, &undefined), test.isUndefined)
			}
		})
	}
}

func TestReplaceReferencesHandlesEscapingAndMixedValues(t *testing.T) {
	resolver := New([]core.Binding{
		{Name: "text", Value: "hello"},
		{Name: "user", Value: map[string]any{"name": "alice"}},
	})

	rendered, err := ReplaceReferences(
		`\${text} ${text} ${user.name}`,
		func(reference string) (string, error) {
			value, resolveErr := resolver.Resolve(reference)
			if resolveErr != nil {
				return "", resolveErr
			}
			return Format(value), nil
		},
	)
	if err != nil {
		t.Fatalf("replace references: %v", err)
	}
	if rendered != `${text} hello alice` {
		t.Fatalf("rendered = %q, want %q", rendered, `${text} hello alice`)
	}
}

func TestProtectEscapedReferencesSurvivesIntermediateRendering(t *testing.T) {
	resolver := New([]core.Binding{{Name: "text", Value: "hello"}})
	protectedInput, protected := ProtectEscapedReferences(`\${text} ${text}`)

	rendered, err := ReplaceReferences(protectedInput, func(reference string) (string, error) {
		value, resolveErr := resolver.Resolve(reference)
		if resolveErr != nil {
			return "", resolveErr
		}
		return Format(value), nil
	})
	if err != nil {
		t.Fatalf("replace references: %v", err)
	}
	if got := protected.Restore(rendered); got != `${text} hello` {
		t.Fatalf("restored = %q, want %q", got, `${text} hello`)
	}
}

func TestUnescapeReferencesLeavesUnescapedReferencesUntouched(t *testing.T) {
	input := `\${escaped} ${plain} \${nested.value}`
	if got := UnescapeReferences(input); got != `${escaped} ${plain} ${nested.value}` {
		t.Fatalf("UnescapeReferences(%q) = %q", input, got)
	}
}

func TestResolverUsesLatestBindingAndSortedNames(t *testing.T) {
	resolver := New([]core.Binding{
		{Name: "zeta", Value: "old"},
		{Name: "alpha", Value: "first"},
		{Name: "zeta", Value: "new"},
	})

	value, err := resolver.Resolve("zeta")
	if err != nil {
		t.Fatalf("resolve latest binding: %v", err)
	}
	if got := Format(value); got != "new" {
		t.Fatalf("latest binding = %q, want new", got)
	}
	if got := strings.Join(resolver.Names(), ","); got != "alpha,zeta" {
		t.Fatalf("Names() = %q, want alpha,zeta", got)
	}
}
