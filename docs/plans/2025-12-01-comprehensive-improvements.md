# Comprehensive CLIAIMONITOR Improvements

**Date**: 2025-12-01
**Status**: Ready for implementation

## Overview

This plan covers 6 improvement areas identified from the Tasks API review:
1. HTTP API quick fixes (filters, pagination, status validation)
2. Planner-to-Spawner executor bridge
3. MCP task tools for agents
4. Clean agent shutdown (existing plan)
5. Dashboard task display (existing plan)

---

## Task 1: HTTP API Quick Fixes

**Files**: `internal/handlers/supervisor.go`, `internal/memory/interface.go`, `internal/memory/tasks.go`
**Effort**: ~1 hour

### 1.1 Expose All TaskFilter Fields

Update `handleGetTasks` in `supervisor.go`:

```go
func (h *SupervisorHandler) handleGetTasks(w http.ResponseWriter, r *http.Request) {
    query := r.URL.Query()

    limit := 100
    if l := query.Get("limit"); l != "" {
        if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 1000 {
            limit = parsed
        }
    }

    offset := 0
    if o := query.Get("offset"); o != "" {
        if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
            offset = parsed
        }
    }

    filter := memory.TaskFilter{
        RepoID:          query.Get("repo_id"),
        Status:          query.Get("status"),
        AssignedAgentID: query.Get("agent_id"),
        Priority:        query.Get("priority"),
        ParentTaskID:    query.Get("parent_id"),
        Limit:           limit,
        Offset:          offset,
    }

    tasks, err := h.memDB.GetTasks(filter)
    if err != nil {
        respondError(w, http.StatusInternalServerError, err.Error())
        return
    }

    respondJSON(w, map[string]interface{}{
        "tasks":  tasks,
        "count":  len(tasks),
        "limit":  limit,
        "offset": offset,
    })
}
```

### 1.2 Add Offset to TaskFilter

Update `interface.go`:

```go
type TaskFilter struct {
    RepoID          string
    Status          string
    AssignedAgentID string
    Priority        string
    ParentTaskID    string
    Limit           int
    Offset          int  // NEW
}
```

### 1.3 Add Offset to SQL Query

Update `tasks.go` GetTasks method:

```go
query += " ORDER BY created_at DESC"

if filter.Limit > 0 {
    query += " LIMIT ?"
    args = append(args, filter.Limit)
}

if filter.Offset > 0 {
    query += " OFFSET ?"
    args = append(args, filter.Offset)
}
```

### 1.4 Fix Status Validation

Update valid statuses in `handleUpdateTaskStatus`:

```go
validStatuses := map[string]bool{
    "pending":     true,
    "assigned":    true,  // ADD - was missing
    "in_progress": true,
    "completed":   true,
    "blocked":     true,
    "cancelled":   true,
}
```

### Verification

```bash
go build -o cliaimonitor.exe ./cmd/cliaimonitor/main.go
curl "http://localhost:3000/supervisor/tasks?status=pending&priority=high&limit=10&offset=0"
```

---

## Task 2: Planner-to-Spawner Executor

**Files**: New `internal/supervisor/executor.go`, update `internal/handlers/supervisor.go`
**Effort**: ~2 hours

### 2.1 Create Executor

Create `internal/supervisor/executor.go`:

```go
package supervisor

import (
    "encoding/json"
    "fmt"
    "sort"

    "github.com/CLIAIMONITOR/internal/agents"
    "github.com/CLIAIMONITOR/internal/memory"
)

// Executor bridges deployment plans to agent spawning
type Executor struct {
    memDB   memory.MemoryDB
    spawner *agents.Spawner
}

// NewExecutor creates a new deployment executor
func NewExecutor(memDB memory.MemoryDB, spawner *agents.Spawner) *Executor {
    return &Executor{memDB: memDB, spawner: spawner}
}

// ExecutionResult contains results of plan execution
type ExecutionResult struct {
    DeploymentID  int64    `json:"deployment_id"`
    SpawnedAgents []string `json:"spawned_agents"`
    FailedAgents  []string `json:"failed_agents"`
    TasksAssigned int      `json:"tasks_assigned"`
}

// ExecutePlan spawns agents based on a deployment plan
func (e *Executor) ExecutePlan(deploymentID int64) (*ExecutionResult, error) {
    // Get deployment from DB
    deployment, err := e.memDB.GetDeployment(deploymentID)
    if err != nil {
        return nil, fmt.Errorf("failed to get deployment: %w", err)
    }

    if deployment.Status != "proposed" && deployment.Status != "approved" {
        return nil, fmt.Errorf("deployment status must be 'proposed' or 'approved', got '%s'", deployment.Status)
    }

    // Parse agent proposals
    var proposals []AgentProposal
    if err := json.Unmarshal([]byte(deployment.AgentConfigs), &proposals); err != nil {
        return nil, fmt.Errorf("failed to parse agent configs: %w", err)
    }

    // Sort by priority (highest first)
    sort.Slice(proposals, func(i, j int) bool {
        return proposals[i].Priority > proposals[j].Priority
    })

    result := &ExecutionResult{
        DeploymentID:  deploymentID,
        SpawnedAgents: []string{},
        FailedAgents:  []string{},
    }

    // Update deployment status to executing
    if err := e.memDB.UpdateDeploymentStatus(deploymentID, "executing"); err != nil {
        return nil, fmt.Errorf("failed to update deployment status: %w", err)
    }

    // Spawn agents
    for _, proposal := range proposals {
        agentID, err := e.spawnFromProposal(proposal)
        if err != nil {
            result.FailedAgents = append(result.FailedAgents, proposal.ConfigName)
            continue
        }
        result.SpawnedAgents = append(result.SpawnedAgents, agentID)

        // Assign tasks to agent
        for _, taskID := range proposal.TaskIDs {
            if err := e.memDB.UpdateTaskStatus(taskID, "assigned", agentID); err == nil {
                result.TasksAssigned++
            }
        }
    }

    // Update final status
    finalStatus := "completed"
    if len(result.FailedAgents) > 0 && len(result.SpawnedAgents) == 0 {
        finalStatus = "failed"
    }
    e.memDB.UpdateDeploymentStatus(deploymentID, finalStatus)

    return result, nil
}

// spawnFromProposal converts an AgentProposal to a spawn request
func (e *Executor) spawnFromProposal(proposal AgentProposal) (string, error) {
    // Map proposal to spawn config
    config := agents.SpawnConfig{
        ConfigName: proposal.ConfigName,
        Role:       proposal.Role,
        Task:       fmt.Sprintf("Work on assigned tasks: %v", proposal.TaskIDs),
    }

    return e.spawner.SpawnAgent(config)
}
```

### 2.2 Add HTTP Endpoint

Add to `supervisor.go` RegisterRoutes:

```go
r.HandleFunc("/supervisor/deployments/{id}/execute", h.handleExecuteDeployment).Methods("POST")
```

Add handler:

```go
func (h *SupervisorHandler) handleExecuteDeployment(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    idStr := vars["id"]

    id, err := strconv.ParseInt(idStr, 10, 64)
    if err != nil {
        respondError(w, http.StatusBadRequest, "Invalid deployment ID")
        return
    }

    result, err := h.executor.ExecutePlan(id)
    if err != nil {
        respondError(w, http.StatusInternalServerError, err.Error())
        return
    }

    respondJSON(w, result)
}
```

### Verification

```bash
# Create a plan
curl -X POST http://localhost:3000/supervisor/repos/REPO_ID/plan

# Execute the plan
curl -X POST http://localhost:3000/supervisor/deployments/1/execute
```

---

## Task 3: MCP Task Tools

**Files**: `internal/mcp/tools.go`
**Effort**: ~2 hours

### 3.1 Add Task Tools

Add to the tools registry in `tools.go`:

```go
// get_my_tasks - List tasks assigned to the calling agent
{
    Name:        "get_my_tasks",
    Description: "Get workflow tasks assigned to you",
    InputSchema: map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "status": {
                "type":        "string",
                "description": "Filter by status (pending, assigned, in_progress, completed, blocked)",
            },
        },
    },
},

// claim_task - Claim an unassigned pending task
{
    Name:        "claim_task",
    Description: "Claim a pending task to work on. Only works for unassigned tasks.",
    InputSchema: map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "task_id": {
                "type":        "string",
                "description": "The ID of the task to claim",
            },
        },
        "required": []string{"task_id"},
    },
},

// update_task_progress - Update status of your assigned task
{
    Name:        "update_task_progress",
    Description: "Update progress on a task you're working on",
    InputSchema: map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "task_id": {
                "type":        "string",
                "description": "The task ID",
            },
            "status": {
                "type":        "string",
                "enum":        []string{"in_progress", "blocked"},
                "description": "New status",
            },
            "note": {
                "type":        "string",
                "description": "Optional progress note",
            },
        },
        "required": []string{"task_id", "status"},
    },
},

// complete_task - Mark task as completed with summary
{
    Name:        "complete_task",
    Description: "Mark a task as completed with a summary of what was done",
    InputSchema: map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "task_id": {
                "type":        "string",
                "description": "The task ID",
            },
            "summary": {
                "type":        "string",
                "description": "Summary of work completed",
            },
        },
        "required": []string{"task_id", "summary"},
    },
},
```

### 3.2 Add Tool Handlers

```go
case "get_my_tasks":
    status, _ := args["status"].(string)
    tasks, err := s.memDB.GetTasks(memory.TaskFilter{
        AssignedAgentID: agentID,
        Status:          status,
        Limit:           50,
    })
    if err != nil {
        return errorResponse(req.ID, -32000, err.Error())
    }
    return successResponse(req.ID, map[string]interface{}{
        "tasks": tasks,
        "count": len(tasks),
    })

case "claim_task":
    taskID := args["task_id"].(string)
    task, err := s.memDB.GetTask(taskID)
    if err != nil {
        return errorResponse(req.ID, -32000, "Task not found")
    }
    if task.AssignedAgentID != "" {
        return errorResponse(req.ID, -32000, "Task already assigned")
    }
    if task.Status != "pending" {
        return errorResponse(req.ID, -32000, "Task is not pending")
    }
    err = s.memDB.UpdateTaskStatus(taskID, "assigned", agentID)
    if err != nil {
        return errorResponse(req.ID, -32000, err.Error())
    }
    return successResponse(req.ID, map[string]interface{}{
        "success": true,
        "task_id": taskID,
        "message": "Task claimed successfully",
    })

case "update_task_progress":
    taskID := args["task_id"].(string)
    status := args["status"].(string)
    err := s.memDB.UpdateTaskStatus(taskID, status, agentID)
    if err != nil {
        return errorResponse(req.ID, -32000, err.Error())
    }
    return successResponse(req.ID, map[string]interface{}{
        "success": true,
    })

case "complete_task":
    taskID := args["task_id"].(string)
    summary := args["summary"].(string)
    err := s.memDB.UpdateTaskStatus(taskID, "completed", agentID)
    if err != nil {
        return errorResponse(req.ID, -32000, err.Error())
    }
    // Store summary as agent learning
    s.memDB.StoreAgentLearning(&memory.AgentLearning{
        AgentID:   agentID,
        Category:  "task_completion",
        Title:     fmt.Sprintf("Completed task %s", taskID),
        Content:   summary,
    })
    return successResponse(req.ID, map[string]interface{}{
        "success": true,
        "message": "Task completed",
    })
```

### 3.3 Update Agent System Prompt

Add to agent prompts:

```markdown
## Task Workflow
You may have tasks assigned to you. Use these MCP tools:
1. `get_my_tasks` - See your assigned tasks
2. `update_task_progress` - Set status to "in_progress" when starting
3. `complete_task` - Mark done with summary when finished

If no tasks assigned, you can:
- `claim_task` - Claim an unassigned pending task
```

---

## Task 4: Clean Agent Shutdown

See existing plan: `docs/plans/2025-12-01-clean-agent-shutdown.md`

Key implementation points:
- Add `ShutdownRequested` and `ShutdownRequestedAt` to Agent type
- Add `RequestAgentShutdown()` to store
- Add `handleGracefulStopAgent()` handler with 60s timeout goroutine
- Include `_shutdown_requested` flag in MCP responses
- Update dashboard with Stop/Kill buttons and countdown

---

## Task 5: Dashboard Task Display

See existing plan: `docs/plans/2025-12-01-dashboard-task-display.md`

Key implementation points:
- Update `renderAgents()` to show current_task prominently
- Change status from "disconnected" to "working" when task is active
- Add `.agent-current-task` CSS styles
- Show relative time since last update

---

## Implementation Order

Execute in this order to minimize conflicts:

1. **Quick fixes** (no dependencies)
2. **Dashboard task display** (no dependencies)
3. **Clean shutdown** (no dependencies)
4. **MCP task tools** (depends on quick fixes for status validation)
5. **Executor bridge** (depends on spawner, can test independently)

## Testing Checklist

- [ ] HTTP filters work: `?agent_id=X&priority=high`
- [ ] Pagination works: `?limit=10&offset=20`
- [ ] Status "assigned" is valid
- [ ] Executor spawns agents from plan
- [ ] MCP tools: get_my_tasks, claim_task, complete_task
- [ ] Graceful stop shows countdown
- [ ] Force stop after 60s timeout
- [ ] Dashboard shows task text prominently
