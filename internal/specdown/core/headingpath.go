package core

import "strings"

// HeadingPath represents the path from document root to a heading.
type HeadingPath []string

// IsPrefix returns true if hp is a prefix of other (ancestor or self).
func (hp HeadingPath) IsPrefix(other HeadingPath) bool {
	if len(hp) > len(other) {
		return false
	}
	for i := range hp {
		if hp[i] != other[i] {
			return false
		}
	}
	return true
}

// IsSiblingOf returns true if hp and other share the same parent and depth.
func (hp HeadingPath) IsSiblingOf(other HeadingPath) bool {
	if len(hp) == 0 || len(other) == 0 || len(hp) != len(other) {
		return false
	}
	return hp[:len(hp)-1].IsPrefix(other[:len(other)-1])
}

// Reachable returns true if hp is reachable from other.
// A path is reachable if it is an ancestor (prefix) or a sibling.
func (hp HeadingPath) Reachable(from HeadingPath) bool {
	return hp.IsPrefix(from) || hp.IsSiblingOf(from)
}

// Key returns a null-byte-joined string suitable as a map key.
func (hp HeadingPath) Key() string {
	return strings.Join(hp, "\x00")
}

// Join returns the path elements joined by the given separator.
func (hp HeadingPath) Join(sep string) string {
	return strings.Join(hp, sep)
}
