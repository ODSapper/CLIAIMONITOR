package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestSecurityHeadersMiddleware verifies that version headers are removed/masked
func TestSecurityHeadersMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		handlerHeaders map[string]string
		expectServer   string
		expectNoHeaders []string
	}{
		{
			name: "removes default Server header and sets MAH",
			handlerHeaders: map[string]string{
				"Server": "Go/1.21 chi/v5",
			},
			expectServer:    "MAH",
			expectNoHeaders: []string{},
		},
		{
			name: "removes X-Powered-By header",
			handlerHeaders: map[string]string{
				"X-Powered-By": "Go",
			},
			expectServer:    "MAH",
			expectNoHeaders: []string{"X-Powered-By"},
		},
		{
			name: "sets MAH when no Server header present",
			handlerHeaders: map[string]string{
				"Content-Type": "application/json",
			},
			expectServer:    "MAH",
			expectNoHeaders: []string{},
		},
		{
			name: "removes multiple version headers",
			handlerHeaders: map[string]string{
				"Server":       "Go/1.21",
				"X-Powered-By": "chi",
			},
			expectServer:    "MAH",
			expectNoHeaders: []string{"X-Powered-By"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test handler that sets specific headers
			innerHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				for k, v := range tt.handlerHeaders {
					w.Header().Set(k, v)
				}
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("OK"))
			})

			// Wrap with security middleware
			handler := SecurityHeadersMiddleware(innerHandler)

			// Make request
			recorder := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "http://localhost/test", nil)
			handler.ServeHTTP(recorder, req)

			// Check Server header
			if got := recorder.Header().Get("Server"); got != tt.expectServer {
				t.Errorf("Server header: got %q, want %q", got, tt.expectServer)
			}

			// Check that sensitive headers are removed
			for _, header := range tt.expectNoHeaders {
				if got := recorder.Header().Get(header); got != "" {
					t.Errorf("%s header should be removed but got %q", header, got)
				}
			}

			// Verify response status is correct
			if got := recorder.Code; got != http.StatusOK {
				t.Errorf("status code: got %d, want %d", got, http.StatusOK)
			}

			// Verify body is still intact
			if got := recorder.Body.String(); got != "OK" {
				t.Errorf("body: got %q, want %q", got, "OK")
			}
		})
	}
}

// TestSecurityHeadersMiddlewareWithoutWriteHeader verifies headers are set even
// if WriteHeader is not explicitly called
func TestSecurityHeadersMiddlewareWithoutWriteHeader(t *testing.T) {
	innerHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Directly write to body without calling WriteHeader
		w.Write([]byte("test"))
	})

	handler := SecurityHeadersMiddleware(innerHandler)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "http://localhost/test", nil)
	handler.ServeHTTP(recorder, req)

	// Check Server header is still set even without explicit WriteHeader call
	if got := recorder.Header().Get("Server"); got != "MAH" {
		t.Errorf("Server header: got %q, want %q", got, "MAH")
	}
}

// BenchmarkSecurityHeadersMiddleware measures middleware overhead
func BenchmarkSecurityHeadersMiddleware(b *testing.B) {
	innerHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", "Go/1.21")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	handler := SecurityHeadersMiddleware(innerHandler)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "http://localhost/test", nil)
		handler.ServeHTTP(recorder, req)
	}
}
