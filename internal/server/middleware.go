package server

import (
	"net/http"
)

// SecurityHeadersMiddleware removes or masks version headers from HTTP responses
// for security hardening. It prevents information disclosure about the server,
// Go version, and framework information.
//
// This middleware:
// - Removes the default Server header that includes Go version
// - Removes X-Powered-By header if present
// - Sets a generic Server header to "MAH" without version info
// - Should be applied early in the middleware chain
func SecurityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Create a response writer wrapper that intercepts header writes
		wrapper := &headerRemovalWriter{ResponseWriter: w}

		// Call the next handler with the wrapped writer
		next.ServeHTTP(wrapper, r)

		// Ensure headers are set (in case the handler didn't write anything)
		// The wrapper will have already handled the Server header
		if wrapper.headerWritten {
			// Headers already written by handler, nothing more to do
			return
		}

		// If headers weren't written, set them now
		wrapper.writeSecurityHeaders()
	})
}

// headerRemovalWriter wraps http.ResponseWriter to intercept and modify headers
type headerRemovalWriter struct {
	http.ResponseWriter
	headerWritten bool
}

// Header intercepts the Header() call to remove sensitive headers
func (w *headerRemovalWriter) Header() http.Header {
	return w.ResponseWriter.Header()
}

// WriteHeader intercepts WriteHeader to apply security headers
func (w *headerRemovalWriter) WriteHeader(statusCode int) {
	w.writeSecurityHeaders()
	w.ResponseWriter.WriteHeader(statusCode)
}

// Write ensures security headers are applied before writing body
func (w *headerRemovalWriter) Write(b []byte) (int, error) {
	if !w.headerWritten {
		w.writeSecurityHeaders()
	}
	return w.ResponseWriter.Write(b)
}

// writeSecurityHeaders applies the security header changes
func (w *headerRemovalWriter) writeSecurityHeaders() {
	if w.headerWritten {
		return
	}
	w.headerWritten = true

	h := w.ResponseWriter.Header()

	// Remove version-exposing headers
	h.Del("Server")
	h.Del("X-Powered-By")

	// Set generic Server header without version information
	h.Set("Server", "MAH")
}

// Flush implements http.Flusher to support SSE streaming
// This is critical for MCP SSE connections to work properly
func (w *headerRemovalWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}
