# Captain HTTP API Endpoints

This document describes the HTTP endpoints added to `internal/handlers/captain.go` for controlling the Captain orchestrator.

## Overview

The Captain Handler provides REST endpoints for submitting tasks, checking status, triggering reconnaissance, and managing escalations. All endpoints return JSON responses.

## Endpoints

### 1. POST /api/captain/task - Submit Task

Submit a new task to Captain for execution.

**Request Body:**
```json
{
  "title": "string (required)",
  "description": "string (required)",
  "project_path": "string (optional)",
  "priority": "integer (optional, default: 0)",
  "needs_recon": "boolean (optional, default: false)",
  "metadata": {
    "key": "value"
  }
}
```

**Response (201 Created):**
```json
{
  "task_id": "task-1234567890",
  "status": "submitted",
  "message": "Task submitted successfully and will be executed asynchronously"
}
```

**Task Type Inference:**
- If `needs_recon` is true, task type will be `TaskRecon`
- Otherwise inferred from title/description keywords:
  - "scan", "recon", "audit", "discover" → TaskRecon
  - "review", "analyze", "assess" → TaskAnalysis
  - "test", "coverage" → TaskTesting
  - "plan", "task", "api" → TaskPlanning
  - Default → TaskImplementation

**Example:**
```bash
curl -X POST http://localhost:8080/api/captain/task \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Security Scan",
    "description": "Scan the MAH project for vulnerabilities",
    "project_path": "C:\\Projects\\MAH",
    "priority": 1
  }'
```

---

### 2. GET /api/captain/status - Get Captain Status

Get the current status of the Captain orchestration loop.

**Response (200 OK):**
```json
{
  "running": true,
  "last_cycle": "2025-12-02T10:30:00Z",
  "pending_tasks": 3,
  "active_agents": 5,
  "escalations": 2
}
```

**Example:**
```bash
curl http://localhost:8080/api/captain/status
```

---

### 3. POST /api/captain/recon - Trigger Manual Reconnaissance

Trigger a manual reconnaissance mission on a project.

**Request Body:**
```json
{
  "project_path": "string (required)",
  "mission": "string (optional)"
}
```

If `mission` is not provided, a full reconnaissance prompt will be generated.

**Response (201 Created):**
```json
{
  "recon_id": "recon-1234567890",
  "status": "started"
}
```

**Example:**
```bash
curl -X POST http://localhost:8080/api/captain/recon \
  -H "Content-Type: application/json" \
  -d '{
    "project_path": "C:\\Projects\\MSS",
    "mission": "Focus on security vulnerabilities in firewall rules"
  }'
```

---

### 4. GET /api/captain/escalations - List Pending Escalations

Get all pending escalations requiring human intervention.

**Response (200 OK):**
```json
{
  "escalations": [
    {
      "id": "stop-123",
      "type": "stop_request",
      "agent_id": "Snake001",
      "question": "Agent requests to stop: task_complete",
      "context": "Work completed: Implemented feature X\nDetails: All tests passing",
      "created_at": "2025-12-02T10:00:00Z"
    },
    {
      "id": "human-456",
      "type": "human_input",
      "agent_id": "SNTGreen002",
      "question": "Should I use approach A or B?",
      "context": "Implementing database migration",
      "created_at": "2025-12-02T09:45:00Z"
    }
  ],
  "total": 2
}
```

**Escalation Types:**
- `stop_request` - Agent requesting permission to stop work
- `human_input` - Agent needs human decision/input

**Example:**
```bash
curl http://localhost:8080/api/captain/escalations
```

---

### 5. POST /api/captain/escalation/{id}/respond - Respond to Escalation

Respond to a pending escalation.

**Request Body:**
```json
{
  "response": "string (required)",
  "action": "string (required: approve|reject|modify)"
}
```

**Response (200 OK):**
```json
{
  "status": "responded",
  "message": "Stop request responded to successfully"
}
```

**Error (404 Not Found):**
```json
{
  "error": "Escalation not found"
}
```

**Example:**
```bash
curl -X POST http://localhost:8080/api/captain/escalation/stop-123/respond \
  -H "Content-Type: application/json" \
  -d '{
    "response": "Approved. Good work!",
    "action": "approve"
  }'
```

---

## Handler Structure

The `CaptainHandler` struct manages all Captain endpoints:

```go
type CaptainHandler struct {
    captain *captain.Captain        // Captain orchestrator
    store   *persistence.JSONStore  // State persistence
}
```

## Integration

To register these endpoints in your HTTP server:

```go
captainHandler := handlers.NewCaptainHandler(captain, store)

router.HandleFunc("/api/captain/task", captainHandler.HandleSubmitTask)
router.HandleFunc("/api/captain/status", captainHandler.HandleGetStatus)
router.HandleFunc("/api/captain/recon", captainHandler.HandleTriggerRecon)
router.HandleFunc("/api/captain/escalations", captainHandler.HandleGetEscalations)
router.HandleFunc("/api/captain/escalation/{id}/respond", captainHandler.HandleRespondToEscalation)
```

## Asynchronous Execution

All task and recon execution happens asynchronously:
- The endpoint returns immediately with a 201 status
- The actual work happens in a background goroutine
- Results are logged to the activity log in the store
- Use the activity log or agent status endpoints to track progress

## Error Handling

All endpoints use standard HTTP status codes:
- `200 OK` - Successful GET request
- `201 Created` - Successful POST creating a resource
- `400 Bad Request` - Invalid request body or missing required fields
- `404 Not Found` - Resource (escalation) not found
- `405 Method Not Allowed` - Wrong HTTP method
- `500 Internal Server Error` - Server-side error

## Activity Logging

All task execution results are logged to the store's activity log:
- Task failures: `action: "task_failed"`
- Task completions: `action: "task_completed"`
- Recon failures: `action: "recon_failed"`
- Recon completions: `action: "recon_completed"`

Access activity logs via the store's `GetState().ActivityLog` field.

## Testing

See `captain_endpoints_test.go` for comprehensive unit tests covering:
- Task submission
- Status retrieval
- Recon triggering
- Escalation listing
- Escalation response handling
- Task type inference
- String utilities

Run tests:
```bash
go test ./internal/handlers/ -v -run TestHandle
```
