// Package stringutils provides utility functions for string manipulation.
package stringutils

import (
	"strings"
	"unicode"
)

// TrimAll removes all whitespace characters from a string,
// including spaces, tabs, newlines, and other Unicode whitespace.
func TrimAll(s string) string {
	var result strings.Builder
	result.Grow(len(s))
	for _, r := range s {
		if !unicode.IsSpace(r) {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// IsEmpty returns true if the string is empty or contains only whitespace.
func IsEmpty(s string) bool {
	return strings.TrimSpace(s) == ""
}
