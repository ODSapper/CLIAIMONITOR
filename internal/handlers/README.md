# Handlers Package

This package contains HTTP request handlers for CLIAIMONITOR API endpoints.

## Files

- **supervisor.go** - Supervisor API endpoints for chat message management
  - GET /api/supervisor/messages
  - POST /api/supervisor/send
  - GET /api/supervisor/pending
  - POST /api/supervisor/answer/:id

- **supervisor_test.go** - Unit tests for supervisor handlers

## Usage

```go
import "github.com/CLIAIMONITOR/internal/handlers"

// Create handler
handler := handlers.NewSupervisorHandler(memDB)

// Use in router
router.HandleFunc("/api/supervisor/messages", handler.GetMessages).Methods("GET")
```
