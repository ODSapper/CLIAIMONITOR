// Package utils provides common utility functions for CLIAIMONITOR.
package utils

import (
	"fmt"
	"time"
)

// Hello returns a greeting message with the given name.
// If name is empty, it defaults to "World".
func Hello(name string) string {
	if name == "" {
		name = "World"
	}
	return fmt.Sprintf("Hello, %s!", name)
}

// HelloWithTime returns a greeting with the current timestamp.
func HelloWithTime(name string) string {
	if name == "" {
		name = "World"
	}
	return fmt.Sprintf("Hello, %s! The time is %s", name, time.Now().Format(time.RFC3339))
}

// IsValidAgentName checks if an agent name meets basic requirements.
// Agent names must be non-empty and not exceed 64 characters.
func IsValidAgentName(name string) bool {
	return len(name) > 0 && len(name) <= 64
}
