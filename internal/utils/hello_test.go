package utils

import (
	"strings"
	"testing"
)

func TestHello(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"with name", "Agent", "Hello, Agent!"},
		{"empty name defaults to World", "", "Hello, World!"},
		{"with special chars", "SGT-Green-001", "Hello, SGT-Green-001!"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Hello(tt.input)
			if result != tt.expected {
				t.Errorf("Hello(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestHelloWithTime(t *testing.T) {
	result := HelloWithTime("Test")
	if !strings.HasPrefix(result, "Hello, Test!") {
		t.Errorf("HelloWithTime should start with greeting, got %q", result)
	}
	if !strings.Contains(result, "The time is") {
		t.Errorf("HelloWithTime should contain timestamp, got %q", result)
	}
}

func TestHelloWithTimeEmptyName(t *testing.T) {
	result := HelloWithTime("")
	if !strings.HasPrefix(result, "Hello, World!") {
		t.Errorf("HelloWithTime('') should default to World, got %q", result)
	}
}

func TestIsValidAgentName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"valid simple name", "agent1", true},
		{"valid with dashes", "SGT-Green-001", true},
		{"empty string", "", false},
		{"max length (64 chars)", strings.Repeat("a", 64), true},
		{"too long (65 chars)", strings.Repeat("a", 65), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidAgentName(tt.input)
			if result != tt.expected {
				t.Errorf("IsValidAgentName(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}
