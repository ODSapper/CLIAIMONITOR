package stringutils

import "testing"

func TestTrimAll(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "no whitespace",
			input:    "hello",
			expected: "hello",
		},
		{
			name:     "leading and trailing spaces",
			input:    "  hello  ",
			expected: "hello",
		},
		{
			name:     "spaces between words",
			input:    "hello world",
			expected: "helloworld",
		},
		{
			name:     "tabs and newlines",
			input:    "hello\t\nworld",
			expected: "helloworld",
		},
		{
			name:     "only whitespace",
			input:    "   \t\n  ",
			expected: "",
		},
		{
			name:     "mixed whitespace",
			input:    "  a b\tc\nd  ",
			expected: "abcd",
		},
		{
			name:     "unicode whitespace",
			input:    "hello\u00A0world", // non-breaking space
			expected: "helloworld",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TrimAll(tt.input)
			if result != tt.expected {
				t.Errorf("TrimAll(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsEmpty(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "empty string",
			input:    "",
			expected: true,
		},
		{
			name:     "single space",
			input:    " ",
			expected: true,
		},
		{
			name:     "multiple spaces",
			input:    "   ",
			expected: true,
		},
		{
			name:     "tabs and newlines",
			input:    "\t\n",
			expected: true,
		},
		{
			name:     "single character",
			input:    "a",
			expected: false,
		},
		{
			name:     "text with whitespace",
			input:    "  hello  ",
			expected: false,
		},
		{
			name:     "whitespace with character in middle",
			input:    "  x  ",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsEmpty(tt.input)
			if result != tt.expected {
				t.Errorf("IsEmpty(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func BenchmarkTrimAll(b *testing.B) {
	input := "  hello world  this is a test  "
	for i := 0; i < b.N; i++ {
		TrimAll(input)
	}
}

func BenchmarkIsEmpty(b *testing.B) {
	inputs := []string{"", "   ", "hello", "  hello  "}
	for i := 0; i < b.N; i++ {
		IsEmpty(inputs[i%len(inputs)])
	}
}
