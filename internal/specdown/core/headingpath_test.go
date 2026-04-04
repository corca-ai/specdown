package core

import "testing"

func TestSlug(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Hello World", "hello-world"},
		{"specs/my-doc.spec.md", "specs-my-doc-spec-md"},
		{"", "spec"},
		{"---", "spec"},
		{"CamelCase123", "camelcase123"},
		{"a  b", "a-b"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := Slug(tt.input); got != tt.want {
				t.Fatalf("Slug(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestHeadingPathReachable(t *testing.T) {
	tests := []struct {
		name string
		hp   HeadingPath
		from HeadingPath
		want bool
	}{
		{"ancestor", HeadingPath{"A"}, HeadingPath{"A", "B"}, true},
		{"self", HeadingPath{"A", "B"}, HeadingPath{"A", "B"}, true},
		{"sibling", HeadingPath{"A"}, HeadingPath{"B"}, true},
		{"root reachable from any", HeadingPath{}, HeadingPath{"A"}, true},
		{"nested sibling", HeadingPath{"A", "B"}, HeadingPath{"A", "C"}, true},
		{"child not reachable from parent", HeadingPath{"A", "B"}, HeadingPath{"A"}, false},
		{"unrelated deep path", HeadingPath{"A", "B"}, HeadingPath{"C", "D"}, false},
		{"sibling child", HeadingPath{"A", "B"}, HeadingPath{"A", "C", "D"}, true},
		{"top-level sibling child", HeadingPath{"A"}, HeadingPath{"B", "C"}, true},
		{"deep sibling child", HeadingPath{"R", "A", "B"}, HeadingPath{"R", "A", "C", "D"}, true},
		{"deeper child not reachable from shallower sibling", HeadingPath{"A", "B", "C"}, HeadingPath{"A", "D"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.hp.Reachable(tt.from)
			if got != tt.want {
				t.Fatalf("HeadingPath%v.Reachable(%v) = %v, want %v", []string(tt.hp), []string(tt.from), got, tt.want)
			}
		})
	}
}
