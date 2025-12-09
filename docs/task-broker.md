# Task Broker - Captain's Multi-Source Task System

## Overview

The Task Broker allows Captain to pull tasks from multiple sources, including:
- **Local Task Queue**: Tasks created via the dashboard or API
- **External APIs**: Tasks from external boards like Magnolia Planner

This enables Captain to coordinate work across different systems and act as a central orchestrator for the entire Magnolia ecosystem.

## Architecture

```
┌─────────────┐
│   Captain   │
└──────┬──────┘
       │
       v
┌──────────────────┐
│   Task Broker    │
└──────────────────┘
       │
       ├─────────────────┬─────────────────┐
       v                 v                 v
┌─────────────┐   ┌─────────────┐   ┌─────────────┐
│Local Queue  │   │  Magnolia   │   │  Future     │
│             │   │  Planner    │   │  Sources    │
└─────────────┘   └─────────────┘   └─────────────┘
```

## Core Components

### TaskSourceInterface

All task sources implement this interface:

```go
type TaskSourceInterface interface {
    FetchPendingTasks() ([]*Task, error)
    ClaimTask(taskID string, agentID string) error
    CompleteTask(taskID string, result TaskResult) error
    GetName() string
}
```

### LocalTaskSource

Wraps the existing in-memory task queue:

```go
localSource := NewLocalTaskSource(queue, store)
tasks, err := localSource.FetchPendingTasks()
```

### ExternalTaskSource

Connects to external APIs like Magnolia Planner:

```go
plannerSource := NewExternalTaskSource(
    "Magnolia Planner",
    "https://plannerprojectmss.vercel.app",
    "team-captain",  // API key
    "team-captain",  // Team ID
)

tasks, err := plannerSource.FetchPendingTasks()
```

### TaskBroker

Coordinates multiple task sources:

```go
broker := NewTaskBroker(localSource, plannerSource)

// Fetch from all sources
allTasks, err := broker.FetchAllPendingTasks()
// Returns: map[string][]*Task
// Example: {"Local Queue": [...], "Magnolia Planner": [...]}
```

## Usage Examples

### Example 1: Simple Broker Setup

```go
// Initialize task queue and store
queue := tasks.NewQueue()
store := tasks.NewStore(db)

// Create local source
localSource := tasks.NewLocalTaskSource(queue, store)

// Create external source
plannerSource := tasks.NewExternalTaskSource(
    "Magnolia Planner",
    "https://plannerprojectmss.vercel.app",
    "team-captain",
    "team-captain",
)

// Create broker
broker := tasks.NewTaskBroker(localSource, plannerSource)
```

### Example 2: Fetch and Prioritize Tasks

```go
// Fetch all pending tasks
allTasks, err := broker.FetchAllPendingTasks()
if err != nil {
    log.Printf("Error fetching tasks: %v", err)
}

// Flatten and prioritize
var taskList []*tasks.Task
for sourceName, tasks := range allTasks {
    log.Printf("Found %d tasks from %s", len(tasks), sourceName)
    taskList = append(taskList, tasks...)
}

// Sort by priority (1=highest)
sort.Slice(taskList, func(i, j int) bool {
    return taskList[i].Priority < taskList[j].Priority
})

// Work on highest priority task
if len(taskList) > 0 {
    highestPriority := taskList[0]
    log.Printf("Working on: %s (Priority %d)",
        highestPriority.Title, highestPriority.Priority)
}
```

### Example 3: Claim and Complete Task

```go
// Get the external source
plannerSource := broker.GetSource("Magnolia Planner")

// Claim a task
err := plannerSource.ClaimTask("MAH-P3-042", "agent-snt-001")
if err != nil {
    log.Printf("Failed to claim: %v", err)
}

// ... agent works on task ...

// Complete the task
result := tasks.TaskResult{
    Branch:      "task/MAH-P3-042-feature",
    PRUrl:       "https://github.com/org/MAH/pull/42",
    TokensUsed:  25000,
    Success:     true,
    CompletedBy: "agent-snt-001",
}

err = plannerSource.CompleteTask("MAH-P3-042", result)
if err != nil {
    log.Printf("Failed to complete: %v", err)
}
```

### Example 4: Dynamic Source Management

```go
broker := tasks.NewTaskBroker()

// Start with just local source
localSource := tasks.NewLocalTaskSource(queue, store)
broker.AddSource(localSource)

// Add external source when needed
if config.EnableExternalTasks {
    plannerSource := tasks.NewExternalTaskSource(
        "Magnolia Planner",
        config.PlannerURL,
        config.APIKey,
        config.TeamID,
    )
    broker.AddSource(plannerSource)
}

// List active sources
fmt.Printf("Active sources: %v\n", broker.ListSources())
// Output: [Local Queue Magnolia Planner]

// Remove a source
broker.RemoveSource("Magnolia Planner")
```

### Example 5: Sync External Tasks to Local Queue

```go
// Fetch from external source
externalTasks, err := plannerSource.FetchPendingTasks()
if err != nil {
    log.Printf("Error: %v", err)
    return
}

// Import into local queue
imported := 0
for _, task := range externalTasks {
    // Check for duplicates
    existing := queue.GetByID(task.ID)
    if existing != nil {
        continue
    }

    // Add to local queue
    queue.Add(task)
    store.Save(task)
    imported++
}

log.Printf("Imported %d new tasks", imported)
```

## Task Result Structure

When completing a task, provide these details:

```go
type TaskResult struct {
    Branch      string  // Git branch name
    PRUrl       string  // Pull request URL
    TokensUsed  int64   // AI tokens consumed
    Success     bool    // true if completed successfully
    ErrorMsg    string  // Error details if failed
    CompletedBy string  // Agent ID that completed it
}
```

## Magnolia Planner Integration

### API Endpoints Used

- `GET /api/v1/tasks?status=pending` - Fetch pending tasks
- `POST /api/v1/tasks/{id}/claim` - Claim a task
- `POST /api/v1/tasks/{id}/implemented` - Mark as complete

### Authentication

Uses `X-API-Key` header with team ID:

```go
req.Header.Set("X-API-Key", "team-captain")
```

### Task Format Mapping

External tasks are converted to internal format:

| Planner Field | Internal Field | Notes |
|--------------|----------------|-------|
| `id` | `ID` | Task identifier |
| `title` | `Title` | Brief description |
| `description` | `Description` | Full details |
| `priority` | `Priority` | 1-7 scale |
| `repo` | `Repo` | Target repository |
| `requirements[]` | `Requirements[]` | Acceptance criteria |
| `status` | `Status` | Always "pending" on fetch |

## Integration with Captain

Captain can use the TaskBroker to:

1. **Fetch work from multiple boards**
2. **Prioritize across all sources**
3. **Assign tasks to Snake Force agents**
4. **Track completion across systems**
5. **Report metrics back to external systems**

### Recommended Captain Workflow

```go
// 1. Fetch all available work
allTasks, _ := broker.FetchAllPendingTasks()

// 2. Prioritize and select task
selectedTask := selectHighestPriority(allTasks)

// 3. Claim task from appropriate source
source := broker.GetSource(selectedTask.Source)
source.ClaimTask(selectedTask.ID, agentID)

// 4. Assign to agent via NATS
nats.Publish("agent."+agentID+".task", selectedTask)

// 5. Monitor agent progress
// ... agent works ...

// 6. Complete task on original source
result := buildTaskResult(agent)
source.CompleteTask(selectedTask.ID, result)
```

## Configuration

Example configuration for external sources:

```yaml
task_sources:
  magnolia_planner:
    enabled: true
    base_url: "https://plannerprojectmss.vercel.app"
    api_key: "team-captain"
    team_id: "team-captain"
    poll_interval: 300  # seconds
```

## Error Handling

The TaskBroker continues fetching from other sources if one fails:

```go
allTasks, err := broker.FetchAllPendingTasks()
// err is only returned if ALL sources fail
// Individual source errors are logged
```

Check logs for source-specific errors:

```
[TaskBroker] Error fetching from Magnolia Planner: connection timeout
```

## Testing

Run the comprehensive test suite:

```bash
go test ./internal/tasks -v
```

Tests cover:
- Local source operations
- Task claiming and completion
- Broker coordination
- Success and failure paths
- Full workflow integration

## Future Sources

The architecture supports additional sources:

- **GitHub Issues**: Pull tasks from GitHub project boards
- **Jira**: Enterprise task tracking integration
- **Custom APIs**: Team-specific task sources
- **File-based**: CSV/JSON task files

Add new sources by implementing `TaskSourceInterface`:

```go
type CustomTaskSource struct {
    // your fields
}

func (c *CustomTaskSource) FetchPendingTasks() ([]*Task, error) {
    // your implementation
}

// ... implement other methods ...
```

## Related Files

- `internal/tasks/sources.go` - Main implementation
- `internal/tasks/sources_test.go` - Comprehensive tests
- `internal/tasks/sources_example.go` - Usage examples
- `internal/tasks/types.go` - Task type definitions
- `internal/tasks/queue.go` - In-memory queue
- `internal/tasks/store.go` - SQLite persistence

## See Also

- [Captain Context System](./context/2025-12-08-captain-card-session.md)
- [Task Management API](../internal/handlers/tasks.go)
- [Magnolia Planner CLAUDE.md](../../planner/CLAUDE.md)
