package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestCheckWebSocketOrigin(t *testing.T) {
	tests := []struct {
		name     string
		origin   string
		expected bool
	}{
		// Allowed: localhost variants
		{name: "localhost:3000", origin: "http://localhost:3000", expected: true},
		{name: "localhost:8080", origin: "http://localhost:8080", expected: true},
		{name: "localhost custom port", origin: "http://localhost:9999", expected: true},
		{name: "127.0.0.1:3000", origin: "http://127.0.0.1:3000", expected: true},
		{name: "127.0.0.1 custom port", origin: "http://127.0.0.1:5555", expected: true},
		{name: "IPv6 localhost", origin: "http://[::1]:3000", expected: true},

		// Allowed: no origin header (same-origin)
		{name: "empty origin", origin: "", expected: true},

		// Rejected: external origins
		{name: "evil.com", origin: "http://evil.com", expected: false},
		{name: "attacker.com:3000", origin: "http://attacker.com:3000", expected: false},
		{name: "evil.com with path", origin: "http://evil.com/path", expected: false},
		{name: "phishing site", origin: "http://localhost.evil.com", expected: false},

		// Rejected: malformed origins
		{name: "invalid URL", origin: "not-a-url", expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/ws", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}

			result := checkWebSocketOrigin(req)
			if result != tt.expected {
				t.Errorf("checkWebSocketOrigin(%q) = %v, want %v", tt.origin, result, tt.expected)
			}
		})
	}
}

func TestCheckWebSocketOrigin_EnvConfig(t *testing.T) {
	// Save original env
	original := os.Getenv("CLIAIMONITOR_ALLOWED_ORIGINS")
	defer os.Setenv("CLIAIMONITOR_ALLOWED_ORIGINS", original)

	// Set custom allowed origins
	os.Setenv("CLIAIMONITOR_ALLOWED_ORIGINS", "https://dashboard.example.com,http://internal.local:8080")

	// Reinitialize allowed origins
	allowedOrigins = initAllowedOrigins()

	tests := []struct {
		name     string
		origin   string
		expected bool
	}{
		{name: "configured HTTPS origin", origin: "https://dashboard.example.com", expected: true},
		{name: "configured HTTP origin", origin: "http://internal.local:8080", expected: true},
		{name: "localhost still works", origin: "http://localhost:3000", expected: true},
		{name: "unconfigured origin rejected", origin: "http://other.example.com", expected: false},
		{name: "wrong port rejected", origin: "http://internal.local:9999", expected: false},
		{name: "wrong scheme rejected", origin: "http://dashboard.example.com", expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/ws", nil)
			req.Header.Set("Origin", tt.origin)

			result := checkWebSocketOrigin(req)
			if result != tt.expected {
				t.Errorf("checkWebSocketOrigin(%q) = %v, want %v", tt.origin, result, tt.expected)
			}
		})
	}

	// Restore default allowed origins
	os.Setenv("CLIAIMONITOR_ALLOWED_ORIGINS", "")
	allowedOrigins = initAllowedOrigins()
}

func TestInitAllowedOrigins(t *testing.T) {
	// Save original env
	original := os.Getenv("CLIAIMONITOR_ALLOWED_ORIGINS")
	defer os.Setenv("CLIAIMONITOR_ALLOWED_ORIGINS", original)

	// Test with no env var
	os.Setenv("CLIAIMONITOR_ALLOWED_ORIGINS", "")
	origins := initAllowedOrigins()
	if len(origins) != 4 {
		t.Errorf("initAllowedOrigins() with empty env should return 4 defaults, got %d", len(origins))
	}

	// Test with env var
	os.Setenv("CLIAIMONITOR_ALLOWED_ORIGINS", "https://a.com,https://b.com")
	origins = initAllowedOrigins()
	if len(origins) != 6 {
		t.Errorf("initAllowedOrigins() with 2 custom origins should return 6 total, got %d", len(origins))
	}

	// Test with whitespace
	os.Setenv("CLIAIMONITOR_ALLOWED_ORIGINS", "  https://a.com  ,  https://b.com  ")
	origins = initAllowedOrigins()
	found := false
	for _, o := range origins {
		if o == "https://a.com" {
			found = true
			break
		}
	}
	if !found {
		t.Error("initAllowedOrigins() should trim whitespace from origins")
	}
}
