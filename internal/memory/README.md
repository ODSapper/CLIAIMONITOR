# Memory Package

Cross-session memory system for CLIAIMONITOR's adaptive supervisor.

## Overview

The memory package provides persistent storage for:
- Repository context and discovered files
- Agent learnings and insights
- Context summaries before session compaction
- Workflow tasks parsed from plans
- Human decisions and approvals
- Agent deployment history
- Supervisor chat messages

## Database Schema

SQLite database with 8 core tables:
- `repos` - Discovered repositories with auto-detection
- `repo_files` - Files discovered in repos (CLAUDE.md, workflows, plans)
- `agent_learnings` - Knowledge accumulated by agents
- `context_summaries` - Session summaries for continuity
- `workflow_tasks` - Tasks parsed from workflow files
- `human_decisions` - All human guidance and approvals
- `deployments` - Agent spawning history
- `chat_messages` - Supervisor<->Human conversation

## Requirements

### CGO and C Compiler

**IMPORTANT**: This package requires CGO to be enabled because it uses `github.com/mattn/go-sqlite3`.

**On Windows**:
- Install a C compiler (MinGW-w64, TDM-GCC, or use MSYS2)
- Set `CGO_ENABLED=1` environment variable
- Example:
  ```powershell
  $env:CGO_ENABLED=1
  go test ./internal/memory/...
  ```

**On Linux/Mac**:
- CGO is usually enabled by default
- GCC/Clang is typically pre-installed
- Just run: `go test ./internal/memory/...`

## Usage

```go
import "github.com/CLIAIMONITOR/internal/memory"

// Create memory database
memDB, err := memory.NewMemoryDB("data/memory.db")
if err != nil {
    log.Fatal(err)
}
defer memDB.Close()

// Discover repository
repo, err := memDB.DiscoverRepo("/path/to/repo")
if err != nil {
    log.Fatal(err)
}

// Store agent learning
learning := &memory.AgentLearning{
    AgentID:   "coder001",
    AgentType: "coder",
    Category:  "solution",
    Title:     "Port conflict resolution",
    Content:   "Use instance management with PID files",
    RepoID:    repo.ID,
}
memDB.StoreAgentLearning(learning)

// Get recent learnings
learnings, err := memDB.GetRecentLearnings(10)

// Create workflow task
task := &memory.WorkflowTask{
    ID:         "MAH-123",
    RepoID:     repo.ID,
    SourceFile: "workflow.yaml",
    Title:      "Implement feature X",
    Priority:   "high",
    Status:     "pending",
}
memDB.CreateTask(task)

// Store human decision
decision := &memory.HumanDecision{
    Context:      "Agent proposed deployment",
    Question:     "Deploy 3 agents?",
    Answer:       "Yes, proceed",
    DecisionType: "approval",
    AgentID:      "supervisor",
}
memDB.StoreDecision(decision)
```

## Interface Contract

Other work streams can depend on the `MemoryDB` interface defined in `interface.go` without waiting for the implementation to be tested:

```go
type MemoryDB interface {
    // Repository operations
    DiscoverRepo(basePath string) (*Repo, error)
    GetRepo(repoID string) (*Repo, error)
    UpdateRepoScan(repoID string) error

    // Agent learnings
    StoreAgentLearning(learning *AgentLearning) error
    GetAgentLearnings(filter LearnFilter) ([]*AgentLearning, error)

    // Context summaries
    StoreContextSummary(summary *ContextSummary) error
    GetRecentSummaries(limit int) ([]*ContextSummary, error)

    // Workflow tasks
    CreateTask(task *WorkflowTask) error
    GetTasks(filter TaskFilter) ([]*WorkflowTask, error)
    UpdateTaskStatus(taskID, status, agentID string) error

    // Human decisions
    StoreDecision(decision *HumanDecision) error
    GetRecentDecisions(limit int) ([]*HumanDecision, error)

    // Chat messages
    StoreChatMessage(msg *ChatMessage) error
    GetPendingQuestions() ([]*ChatMessage, error)

    // Deployments
    CreateDeployment(deployment *Deployment) error
    GetRecentDeployments(repoID string, limit int) ([]*Deployment, error)

    Close() error
}
```

## Testing

To run tests, ensure CGO is enabled:

```bash
# Windows (PowerShell)
$env:CGO_ENABLED=1
go test ./internal/memory/... -v

# Linux/Mac
CGO_ENABLED=1 go test ./internal/memory/... -v
```

## Architecture

- **SQLite with WAL mode**: Write-Ahead Logging for better concurrency
- **Foreign key constraints**: Maintain referential integrity
- **Temporal indexing**: Fast time-range queries on all tables
- **Embedded schema**: Schema SQL embedded in binary via `//go:embed`
- **Automatic migrations**: Schema version tracking for future updates

## Files

- `interface.go` - Public interface and type definitions
- `db.go` - Database connection and migration
- `repo.go` - Repository discovery and file management
- `agent.go` - Agent learnings and context summaries
- `tasks.go` - Workflow task management
- `decisions.go` - Human decisions and deployments
- `schema.sql` - Database schema DDL
- `memory_test.go` - Comprehensive test suite
