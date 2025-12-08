# Orchestrator Improvements Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Transform CLIAIMONITOR into a self-sufficient orchestration system with agent-centric dashboard, task queue, full git automation, and comprehensive metrics.

**Architecture:** Captain becomes team lead, breaking down goals into tasks, assigning to agents, managing git lifecycle (branch → PR → merge). Dashboard shows agent lanes with task queues. All task sources feed into unified priority queue.

**Tech Stack:** Go 1.25+, SQLite (existing), gorilla/mux (existing), vanilla JS dashboard, git/gh CLI for automation.

---

## Phase 1: Task System Foundation (Parallel: 1A, 1B, 1C)

### Task 1A: Task Types and Schema

**Files:**
- Create: `internal/tasks/types.go`
- Create: `internal/tasks/types_test.go`

**Step 1: Write the failing test**

```go
// internal/tasks/types_test.go
package tasks

import (
	"testing"
	"time"
)

func TestTaskStatusTransitions(t *testing.T) {
	task := &Task{
		ID:       "TASK-001",
		Title:    "Test task",
		Status:   StatusPending,
		Priority: 3,
	}

	// Pending -> Assigned is valid
	if err := task.TransitionTo(StatusAssigned); err != nil {
		t.Errorf("expected valid transition, got: %v", err)
	}

	// Assigned -> Merged is invalid (must go through review)
	task.Status = StatusAssigned
	if err := task.TransitionTo(StatusMerged); err == nil {
		t.Error("expected invalid transition error")
	}
}

func TestTaskPriorityValidation(t *testing.T) {
	tests := []struct {
		priority int
		valid    bool
	}{
		{0, false},
		{1, true},
		{7, true},
		{8, false},
	}

	for _, tt := range tests {
		task := &Task{Priority: tt.priority}
		err := task.Validate()
		if tt.valid && err != nil {
			t.Errorf("priority %d should be valid, got: %v", tt.priority, err)
		}
		if !tt.valid && err == nil {
			t.Errorf("priority %d should be invalid", tt.priority)
		}
	}
}

func TestNewTask(t *testing.T) {
	task := NewTask("Test title", "Test description", 2)

	if task.ID == "" {
		t.Error("expected auto-generated ID")
	}
	if task.Status != StatusPending {
		t.Errorf("expected pending status, got: %s", task.Status)
	}
	if task.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/tasks/... -v`
Expected: FAIL with "package tasks is not in std"

**Step 3: Write minimal implementation**

```go
// internal/tasks/types.go
package tasks

import (
	"fmt"
	"time"
)

// TaskStatus represents the current state of a task
type TaskStatus string

const (
	StatusPending          TaskStatus = "pending"
	StatusAssigned         TaskStatus = "assigned"
	StatusInProgress       TaskStatus = "in_progress"
	StatusReview           TaskStatus = "review"
	StatusChangesRequested TaskStatus = "changes_requested"
	StatusApproved         TaskStatus = "approved"
	StatusMerged           TaskStatus = "merged"
	StatusBlocked          TaskStatus = "blocked"
)

// TaskSource identifies where the task originated
type TaskSource string

const (
	SourceCaptain   TaskSource = "captain"
	SourceDashboard TaskSource = "dashboard"
	SourceCLI       TaskSource = "cli"
	SourceFile      TaskSource = "file"
)

// Task represents a unit of work in the system
type Task struct {
	ID          string            `json:"id"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Priority    int               `json:"priority"` // 1-7, 1=critical
	Status      TaskStatus        `json:"status"`
	Source      TaskSource        `json:"source"`
	Repo        string            `json:"repo,omitempty"`
	AssignedTo  string            `json:"assigned_to,omitempty"`
	Branch      string            `json:"branch,omitempty"`
	PRUrl       string            `json:"pr_url,omitempty"`
	Requirements []Requirement    `json:"requirements,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	StartedAt   *time.Time        `json:"started_at,omitempty"`
	CompletedAt *time.Time        `json:"completed_at,omitempty"`
}

// Requirement is an acceptance criterion for a task
type Requirement struct {
	Text     string `json:"text"`
	Required bool   `json:"required"`
	Met      bool   `json:"met"`
}

// validTransitions defines allowed status transitions
var validTransitions = map[TaskStatus][]TaskStatus{
	StatusPending:          {StatusAssigned, StatusBlocked},
	StatusAssigned:         {StatusInProgress, StatusPending, StatusBlocked},
	StatusInProgress:       {StatusReview, StatusBlocked, StatusAssigned},
	StatusReview:           {StatusApproved, StatusChangesRequested},
	StatusChangesRequested: {StatusInProgress, StatusBlocked},
	StatusApproved:         {StatusMerged},
	StatusBlocked:          {StatusPending, StatusAssigned, StatusInProgress},
}

// NewTask creates a new task with auto-generated ID
func NewTask(title, description string, priority int) *Task {
	now := time.Now()
	return &Task{
		ID:          fmt.Sprintf("TASK-%d", now.UnixNano()),
		Title:       title,
		Description: description,
		Priority:    priority,
		Status:      StatusPending,
		Source:      SourceCaptain,
		Metadata:    make(map[string]string),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// Validate checks that the task has valid field values
func (t *Task) Validate() error {
	if t.Priority < 1 || t.Priority > 7 {
		return fmt.Errorf("priority must be between 1 and 7")
	}
	if t.Title == "" {
		return fmt.Errorf("title is required")
	}
	return nil
}

// TransitionTo attempts to move the task to a new status
func (t *Task) TransitionTo(newStatus TaskStatus) error {
	allowed, ok := validTransitions[t.Status]
	if !ok {
		return fmt.Errorf("unknown current status: %s", t.Status)
	}

	for _, s := range allowed {
		if s == newStatus {
			t.Status = newStatus
			t.UpdatedAt = time.Now()
			return nil
		}
	}

	return fmt.Errorf("invalid transition from %s to %s", t.Status, newStatus)
}

// IsTerminal returns true if the task is in a final state
func (t *Task) IsTerminal() bool {
	return t.Status == StatusMerged
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/tasks/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/tasks/
git commit -m "feat(tasks): add task types and status transitions"
```

---

### Task 1B: Task Queue Implementation

**Files:**
- Create: `internal/tasks/queue.go`
- Create: `internal/tasks/queue_test.go`

**Step 1: Write the failing test**

```go
// internal/tasks/queue_test.go
package tasks

import (
	"testing"
)

func TestQueuePriorityOrdering(t *testing.T) {
	q := NewQueue()

	// Add tasks with different priorities
	q.Add(NewTask("Low priority", "", 7))
	q.Add(NewTask("Critical", "", 1))
	q.Add(NewTask("Medium", "", 4))

	// Peek should return highest priority (lowest number)
	task := q.Peek()
	if task.Priority != 1 {
		t.Errorf("expected priority 1, got %d", task.Priority)
	}
}

func TestQueuePopRemovesTask(t *testing.T) {
	q := NewQueue()
	q.Add(NewTask("Task 1", "", 3))
	q.Add(NewTask("Task 2", "", 3))

	if q.Len() != 2 {
		t.Errorf("expected 2 tasks, got %d", q.Len())
	}

	q.Pop()

	if q.Len() != 1 {
		t.Errorf("expected 1 task after pop, got %d", q.Len())
	}
}

func TestQueueGetByID(t *testing.T) {
	q := NewQueue()
	task := NewTask("Find me", "", 3)
	q.Add(task)

	found := q.GetByID(task.ID)
	if found == nil {
		t.Error("expected to find task by ID")
	}
	if found.Title != "Find me" {
		t.Errorf("wrong task returned")
	}
}

func TestQueueGetByStatus(t *testing.T) {
	q := NewQueue()
	t1 := NewTask("Pending 1", "", 3)
	t2 := NewTask("Pending 2", "", 3)
	t3 := NewTask("Assigned", "", 3)
	t3.Status = StatusAssigned

	q.Add(t1)
	q.Add(t2)
	q.Add(t3)

	pending := q.GetByStatus(StatusPending)
	if len(pending) != 2 {
		t.Errorf("expected 2 pending tasks, got %d", len(pending))
	}
}

func TestQueueGetByAgent(t *testing.T) {
	q := NewQueue()
	t1 := NewTask("Agent 1 task", "", 3)
	t1.AssignedTo = "SNTGreen"
	t2 := NewTask("Agent 2 task", "", 3)
	t2.AssignedTo = "SNTPurple"

	q.Add(t1)
	q.Add(t2)

	agentTasks := q.GetByAgent("SNTGreen")
	if len(agentTasks) != 1 {
		t.Errorf("expected 1 task for agent, got %d", len(agentTasks))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/tasks/... -v -run Queue`
Expected: FAIL with "undefined: NewQueue"

**Step 3: Write minimal implementation**

```go
// internal/tasks/queue.go
package tasks

import (
	"sort"
	"sync"
)

// Queue is a thread-safe priority queue for tasks
type Queue struct {
	mu    sync.RWMutex
	tasks []*Task
	index map[string]*Task // ID -> Task for fast lookup
}

// NewQueue creates a new task queue
func NewQueue() *Queue {
	return &Queue{
		tasks: make([]*Task, 0),
		index: make(map[string]*Task),
	}
}

// Add inserts a task into the queue, maintaining priority order
func (q *Queue) Add(task *Task) {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.tasks = append(q.tasks, task)
	q.index[task.ID] = task
	q.sortLocked()
}

// Peek returns the highest priority task without removing it
func (q *Queue) Peek() *Task {
	q.mu.RLock()
	defer q.mu.RUnlock()

	if len(q.tasks) == 0 {
		return nil
	}
	return q.tasks[0]
}

// Pop removes and returns the highest priority task
func (q *Queue) Pop() *Task {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.tasks) == 0 {
		return nil
	}

	task := q.tasks[0]
	q.tasks = q.tasks[1:]
	delete(q.index, task.ID)
	return task
}

// Remove removes a task by ID
func (q *Queue) Remove(id string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	task, exists := q.index[id]
	if !exists {
		return false
	}

	delete(q.index, id)
	for i, t := range q.tasks {
		if t.ID == id {
			q.tasks = append(q.tasks[:i], q.tasks[i+1:]...)
			break
		}
	}
	_ = task // silence unused
	return true
}

// GetByID returns a task by its ID
func (q *Queue) GetByID(id string) *Task {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.index[id]
}

// GetByStatus returns all tasks with the given status
func (q *Queue) GetByStatus(status TaskStatus) []*Task {
	q.mu.RLock()
	defer q.mu.RUnlock()

	var result []*Task
	for _, t := range q.tasks {
		if t.Status == status {
			result = append(result, t)
		}
	}
	return result
}

// GetByAgent returns all tasks assigned to an agent
func (q *Queue) GetByAgent(agentID string) []*Task {
	q.mu.RLock()
	defer q.mu.RUnlock()

	var result []*Task
	for _, t := range q.tasks {
		if t.AssignedTo == agentID {
			result = append(result, t)
		}
	}
	return result
}

// Len returns the number of tasks in the queue
func (q *Queue) Len() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return len(q.tasks)
}

// All returns all tasks (for dashboard display)
func (q *Queue) All() []*Task {
	q.mu.RLock()
	defer q.mu.RUnlock()

	result := make([]*Task, len(q.tasks))
	copy(result, q.tasks)
	return result
}

// Update modifies a task in the queue
func (q *Queue) Update(task *Task) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	if _, exists := q.index[task.ID]; !exists {
		return false
	}

	q.index[task.ID] = task
	for i, t := range q.tasks {
		if t.ID == task.ID {
			q.tasks[i] = task
			break
		}
	}
	q.sortLocked()
	return true
}

// sortLocked sorts tasks by priority (must hold lock)
func (q *Queue) sortLocked() {
	sort.Slice(q.tasks, func(i, j int) bool {
		// Lower priority number = higher priority
		if q.tasks[i].Priority != q.tasks[j].Priority {
			return q.tasks[i].Priority < q.tasks[j].Priority
		}
		// Same priority: older tasks first (FIFO)
		return q.tasks[i].CreatedAt.Before(q.tasks[j].CreatedAt)
	})
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/tasks/... -v -run Queue`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/tasks/
git commit -m "feat(tasks): add priority queue implementation"
```

---

### Task 1C: Database Migration for Tasks

**Files:**
- Create: `internal/memory/migrations/006_tasks.sql`
- Modify: `internal/memory/db.go` (add migration)

**Step 1: Create migration file**

```sql
-- internal/memory/migrations/006_tasks.sql
-- Task queue persistence

CREATE TABLE IF NOT EXISTS tasks (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    description TEXT,
    priority INTEGER NOT NULL DEFAULT 5,
    status TEXT NOT NULL DEFAULT 'pending',
    source TEXT NOT NULL DEFAULT 'captain',
    repo TEXT,
    assigned_to TEXT,
    branch TEXT,
    pr_url TEXT,
    metadata TEXT, -- JSON
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    started_at TIMESTAMP,
    completed_at TIMESTAMP
);

CREATE TABLE IF NOT EXISTS task_requirements (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    text TEXT NOT NULL,
    required BOOLEAN NOT NULL DEFAULT 1,
    met BOOLEAN NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS task_metrics (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    agent_id TEXT NOT NULL,
    tokens_used INTEGER DEFAULT 0,
    time_spent_seconds INTEGER DEFAULT 0,
    commits INTEGER DEFAULT 0,
    lines_added INTEGER DEFAULT 0,
    lines_removed INTEGER DEFAULT 0,
    recorded_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS task_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    from_status TEXT NOT NULL,
    to_status TEXT NOT NULL,
    changed_by TEXT, -- agent_id or 'captain' or 'human'
    reason TEXT,
    changed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
CREATE INDEX IF NOT EXISTS idx_tasks_priority ON tasks(priority);
CREATE INDEX IF NOT EXISTS idx_tasks_assigned ON tasks(assigned_to);
CREATE INDEX IF NOT EXISTS idx_task_history_task ON task_history(task_id);

-- Update schema version
INSERT OR REPLACE INTO schema_version (version, applied_at) VALUES (7, datetime('now'));
```

**Step 2: Add migration embed and run**

Modify `internal/memory/db.go`:

```go
// Add to imports (near other embeds around line 25)
//go:embed migrations/006_tasks.sql
var migration006 string

// Add to migrate() function after version < 6 block (around line 122)
if version < 7 {
    fmt.Println("[MIGRATION] Running migration to v7: Add task tables")
    if _, err := m.db.Exec(migration006); err != nil {
        return fmt.Errorf("failed to run migration 006: %w", err)
    }
    fmt.Println("[MIGRATION] Successfully migrated to schema v7")
}
```

**Step 3: Test migration runs**

Run: `go build ./cmd/cliaimonitor && rm -f data/memory.db && ./cliaimonitor.exe -h`
Expected: Migration messages showing v7 applied

**Step 4: Commit**

```bash
git add internal/memory/
git commit -m "feat(db): add task tables migration"
```

---

## Phase 2: Git Automation (Parallel: 2A, 2B)

### Task 2A: Git Operations Module

**Files:**
- Create: `internal/git/git.go`
- Create: `internal/git/git_test.go`

**Step 1: Write the failing test**

```go
// internal/git/git_test.go
package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBranchNameSanitization(t *testing.T) {
	tests := []struct {
		taskID   string
		title    string
		expected string
	}{
		{"TASK-001", "Fix auth bug", "task/TASK-001-fix-auth-bug"},
		{"TASK-002", "Add rate limiting!", "task/TASK-002-add-rate-limiting"},
		{"TASK-003", "This is a very long title that should be truncated", "task/TASK-003-this-is-a-very-long-title-that"},
	}

	for _, tt := range tests {
		result := BranchName(tt.taskID, tt.title)
		if result != tt.expected {
			t.Errorf("BranchName(%q, %q) = %q, want %q", tt.taskID, tt.title, result, tt.expected)
		}
	}
}

func TestGitOperationsInTempRepo(t *testing.T) {
	// Skip if git not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "git-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	// Configure git
	exec.Command("git", "-C", tmpDir, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", tmpDir, "config", "user.name", "Test").Run()

	// Create initial commit
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("initial"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "initial").Run()

	// Test Git operations
	g := New(tmpDir)

	// Test CreateBranch
	branch := "task/TASK-001-test"
	if err := g.CreateBranch(branch); err != nil {
		t.Errorf("CreateBranch failed: %v", err)
	}

	// Verify we're on the new branch
	current, err := g.CurrentBranch()
	if err != nil {
		t.Errorf("CurrentBranch failed: %v", err)
	}
	if current != branch {
		t.Errorf("expected branch %q, got %q", branch, current)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/git/... -v`
Expected: FAIL with "package git is not in std"

**Step 3: Write minimal implementation**

```go
// internal/git/git.go
package git

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// Git provides git operations for a repository
type Git struct {
	repoPath string
}

// New creates a Git instance for the given repository path
func New(repoPath string) *Git {
	return &Git{repoPath: repoPath}
}

// BranchName creates a sanitized branch name from task ID and title
func BranchName(taskID, title string) string {
	// Lowercase and replace spaces with hyphens
	slug := strings.ToLower(title)
	slug = strings.ReplaceAll(slug, " ", "-")

	// Remove non-alphanumeric characters except hyphens
	reg := regexp.MustCompile(`[^a-z0-9-]`)
	slug = reg.ReplaceAllString(slug, "")

	// Remove consecutive hyphens
	reg = regexp.MustCompile(`-+`)
	slug = reg.ReplaceAllString(slug, "-")

	// Trim hyphens from ends
	slug = strings.Trim(slug, "-")

	// Truncate to reasonable length (40 chars for slug)
	if len(slug) > 40 {
		slug = slug[:40]
		// Don't end on a hyphen
		slug = strings.TrimRight(slug, "-")
	}

	return fmt.Sprintf("task/%s-%s", taskID, slug)
}

// run executes a git command and returns output
func (g *Git) run(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = g.repoPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, output)
	}
	return strings.TrimSpace(string(output)), nil
}

// CurrentBranch returns the current branch name
func (g *Git) CurrentBranch() (string, error) {
	return g.run("rev-parse", "--abbrev-ref", "HEAD")
}

// CreateBranch creates and checks out a new branch
func (g *Git) CreateBranch(name string) error {
	_, err := g.run("checkout", "-b", name)
	return err
}

// SwitchBranch switches to an existing branch
func (g *Git) SwitchBranch(name string) error {
	_, err := g.run("checkout", name)
	return err
}

// HasUncommittedChanges returns true if there are uncommitted changes
func (g *Git) HasUncommittedChanges() (bool, error) {
	output, err := g.run("status", "--porcelain")
	if err != nil {
		return false, err
	}
	return output != "", nil
}

// Add stages files for commit
func (g *Git) Add(paths ...string) error {
	args := append([]string{"add"}, paths...)
	_, err := g.run(args...)
	return err
}

// Commit creates a commit with the given message
func (g *Git) Commit(message string) error {
	_, err := g.run("commit", "-m", message)
	return err
}

// Push pushes the current branch to origin
func (g *Git) Push() error {
	branch, err := g.CurrentBranch()
	if err != nil {
		return err
	}
	_, err = g.run("push", "-u", "origin", branch)
	return err
}

// GetDiff returns the diff for staged changes
func (g *Git) GetDiff() (string, error) {
	return g.run("diff", "--staged")
}

// GetLog returns recent commit messages
func (g *Git) GetLog(count int) (string, error) {
	return g.run("log", fmt.Sprintf("-%d", count), "--oneline")
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/git/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/git/
git commit -m "feat(git): add git operations module"
```

---

### Task 2B: GitHub PR Operations

**Files:**
- Modify: `internal/git/git.go` (add PR functions)
- Modify: `internal/git/git_test.go` (add PR tests)

**Step 1: Write the failing test**

Add to `internal/git/git_test.go`:

```go
func TestPRBodyGeneration(t *testing.T) {
	pr := PRInfo{
		Title:   "Fix authentication bypass",
		Summary: "Added input validation to prevent auth bypass",
		TaskIDs: []string{"TASK-001"},
		Agents:  []string{"SNTGreen", "SNTPurple"},
		Metrics: PRMetrics{
			TokensUsed: 23450,
			TimeMinutes: 45,
		},
	}

	body := pr.GenerateBody()

	if !strings.Contains(body, "## Summary") {
		t.Error("body should contain Summary section")
	}
	if !strings.Contains(body, "TASK-001") {
		t.Error("body should contain task ID")
	}
	if !strings.Contains(body, "SNTGreen") {
		t.Error("body should contain agent name")
	}
	if !strings.Contains(body, "23,450") {
		t.Error("body should contain token count")
	}
	if !strings.Contains(body, "team-coop") {
		t.Error("body should contain team identifier")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/git/... -v -run PRBody`
Expected: FAIL with "undefined: PRInfo"

**Step 3: Add implementation**

Add to `internal/git/git.go`:

```go
// PRInfo contains information for creating a pull request
type PRInfo struct {
	Title   string
	Summary string
	TaskIDs []string
	Agents  []string
	Metrics PRMetrics
}

// PRMetrics tracks work done for a PR
type PRMetrics struct {
	TokensUsed  int64
	TimeMinutes int
}

// GenerateBody creates the PR body markdown
func (p *PRInfo) GenerateBody() string {
	var sb strings.Builder

	sb.WriteString("## Summary\n")
	sb.WriteString(p.Summary)
	sb.WriteString("\n\n")

	sb.WriteString("## Tasks Completed\n")
	for _, id := range p.TaskIDs {
		sb.WriteString(fmt.Sprintf("- [x] %s\n", id))
	}
	sb.WriteString("\n")

	sb.WriteString("## Agents Involved\n")
	for _, agent := range p.Agents {
		sb.WriteString(fmt.Sprintf("- %s\n", agent))
	}
	sb.WriteString("\n")

	sb.WriteString("## Metrics\n")
	sb.WriteString(fmt.Sprintf("- Tokens used: %s\n", formatNumber(p.Metrics.TokensUsed)))
	sb.WriteString(fmt.Sprintf("- Time: %dm\n", p.Metrics.TimeMinutes))
	sb.WriteString("\n")

	sb.WriteString("---\n")
	sb.WriteString("Generated by CLIAIMONITOR team-coop\n")

	return sb.String()
}

// formatNumber adds thousand separators
func formatNumber(n int64) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}

	var result strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result.WriteRune(',')
		}
		result.WriteRune(c)
	}
	return result.String()
}

// CreatePR creates a pull request using gh CLI
func (g *Git) CreatePR(pr PRInfo) (string, error) {
	body := pr.GenerateBody()

	output, err := g.run("gh", "pr", "create",
		"--title", pr.Title,
		"--body", body)
	if err != nil {
		return "", fmt.Errorf("failed to create PR: %w", err)
	}

	// Output should be the PR URL
	return output, nil
}

// MergePR merges a pull request using squash
func (g *Git) MergePR(prURL string) error {
	// Extract PR number from URL
	parts := strings.Split(prURL, "/")
	prNum := parts[len(parts)-1]

	_, err := g.run("gh", "pr", "merge", prNum, "--squash", "--delete-branch")
	return err
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/git/... -v -run PRBody`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/git/
git commit -m "feat(git): add PR creation and body generation"
```

---

## Phase 3: Metrics System (Parallel: 3A, 3B)

### Task 3A: Extended Metrics Types

**Files:**
- Create: `internal/metrics/extended.go`
- Create: `internal/metrics/extended_test.go`

**Step 1: Write the failing test**

```go
// internal/metrics/extended_test.go
package metrics

import (
	"testing"
	"time"
)

func TestAgentMetricsEfficiency(t *testing.T) {
	m := &ExtendedAgentMetrics{
		TasksCompleted: 5,
		TotalTokens:    50000,
		TotalTimeSeconds: 3600,
	}

	tokensPerTask := m.TokensPerTask()
	if tokensPerTask != 10000 {
		t.Errorf("expected 10000 tokens/task, got %d", tokensPerTask)
	}
}

func TestAgentMetricsHealthStatus(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		metrics  *ExtendedAgentMetrics
		expected HealthStatus
	}{
		{
			name: "healthy",
			metrics: &ExtendedAgentMetrics{
				LastActivity:       now,
				ConsecutiveFailures: 0,
			},
			expected: HealthHealthy,
		},
		{
			name: "idle",
			metrics: &ExtendedAgentMetrics{
				LastActivity:       now.Add(-15 * time.Minute),
				ConsecutiveFailures: 0,
			},
			expected: HealthIdle,
		},
		{
			name: "stuck",
			metrics: &ExtendedAgentMetrics{
				LastActivity:       now.Add(-35 * time.Minute),
				ConsecutiveFailures: 0,
			},
			expected: HealthStuck,
		},
		{
			name: "failing",
			metrics: &ExtendedAgentMetrics{
				LastActivity:       now,
				ConsecutiveFailures: 3,
			},
			expected: HealthFailing,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := tt.metrics.HealthStatus()
			if status != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, status)
			}
		})
	}
}

func TestTeamMetricsAggregation(t *testing.T) {
	team := NewTeamMetrics("team-coop")

	team.AddAgentMetrics("SNTGreen", &ExtendedAgentMetrics{
		TasksCompleted:   3,
		TotalTokens:      30000,
		TotalTimeSeconds: 1800,
	})
	team.AddAgentMetrics("SNTPurple", &ExtendedAgentMetrics{
		TasksCompleted:   2,
		TotalTokens:      20000,
		TotalTimeSeconds: 1200,
	})

	if team.TotalTasks() != 5 {
		t.Errorf("expected 5 total tasks, got %d", team.TotalTasks())
	}
	if team.TotalTokens() != 50000 {
		t.Errorf("expected 50000 total tokens, got %d", team.TotalTokens())
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/metrics/... -v -run Extended`
Expected: FAIL with "undefined: ExtendedAgentMetrics"

**Step 3: Write minimal implementation**

```go
// internal/metrics/extended.go
package metrics

import (
	"sync"
	"time"
)

// HealthStatus represents agent health
type HealthStatus string

const (
	HealthHealthy HealthStatus = "healthy"
	HealthIdle    HealthStatus = "idle"
	HealthStuck   HealthStatus = "stuck"
	HealthFailing HealthStatus = "failing"
	HealthError   HealthStatus = "error"
)

// ExtendedAgentMetrics provides comprehensive agent metrics
type ExtendedAgentMetrics struct {
	AgentID     string `json:"agent_id"`
	AgentType   string `json:"agent_type"`

	// Efficiency metrics
	TasksCompleted   int   `json:"tasks_completed"`
	TotalTokens      int64 `json:"total_tokens"`
	TotalTimeSeconds int64 `json:"total_time_seconds"`

	// Progress metrics
	CurrentTaskID    string `json:"current_task_id,omitempty"`
	QueueDepth       int    `json:"queue_depth"`

	// Health metrics
	LastActivity        time.Time `json:"last_activity"`
	ConsecutiveFailures int       `json:"consecutive_failures"`
	FailedTests         int       `json:"failed_tests"`
	ReviewRejections    int       `json:"review_rejections"`
}

// TokensPerTask returns average tokens per completed task
func (m *ExtendedAgentMetrics) TokensPerTask() int64 {
	if m.TasksCompleted == 0 {
		return 0
	}
	return m.TotalTokens / int64(m.TasksCompleted)
}

// AvgTaskTimeSeconds returns average time per task in seconds
func (m *ExtendedAgentMetrics) AvgTaskTimeSeconds() int64 {
	if m.TasksCompleted == 0 {
		return 0
	}
	return m.TotalTimeSeconds / int64(m.TasksCompleted)
}

// HealthStatus returns the agent's health status
func (m *ExtendedAgentMetrics) HealthStatus() HealthStatus {
	if m.ConsecutiveFailures >= 3 {
		return HealthFailing
	}

	idleTime := time.Since(m.LastActivity)

	if idleTime > 30*time.Minute {
		return HealthStuck
	}
	if idleTime > 10*time.Minute {
		return HealthIdle
	}

	return HealthHealthy
}

// TeamMetrics aggregates metrics across all agents
type TeamMetrics struct {
	mu       sync.RWMutex
	TeamID   string                          `json:"team_id"`
	Agents   map[string]*ExtendedAgentMetrics `json:"agents"`
}

// NewTeamMetrics creates a new team metrics tracker
func NewTeamMetrics(teamID string) *TeamMetrics {
	return &TeamMetrics{
		TeamID: teamID,
		Agents: make(map[string]*ExtendedAgentMetrics),
	}
}

// AddAgentMetrics adds or updates metrics for an agent
func (t *TeamMetrics) AddAgentMetrics(agentID string, m *ExtendedAgentMetrics) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Agents[agentID] = m
}

// TotalTasks returns total tasks completed across all agents
func (t *TeamMetrics) TotalTasks() int {
	t.mu.RLock()
	defer t.mu.RUnlock()

	total := 0
	for _, m := range t.Agents {
		total += m.TasksCompleted
	}
	return total
}

// TotalTokens returns total tokens used across all agents
func (t *TeamMetrics) TotalTokens() int64 {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var total int64
	for _, m := range t.Agents {
		total += m.TotalTokens
	}
	return total
}

// ActiveAgents returns count of agents with healthy/idle status
func (t *TeamMetrics) ActiveAgents() int {
	t.mu.RLock()
	defer t.mu.RUnlock()

	count := 0
	for _, m := range t.Agents {
		status := m.HealthStatus()
		if status == HealthHealthy || status == HealthIdle {
			count++
		}
	}
	return count
}

// EstimatedCost calculates total cost based on model pricing
func (t *TeamMetrics) EstimatedCost() float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var cost float64
	for _, m := range t.Agents {
		// Opus: $15/M input, Sonnet: $3/M input (simplified)
		rate := 0.003 // Default Sonnet rate
		if m.AgentType == "OpusGreen" || m.AgentType == "OpusPurple" || m.AgentType == "OpusRed" {
			rate = 0.015
		}
		cost += float64(m.TotalTokens) * rate / 1000000
	}
	return cost
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/metrics/... -v -run Extended`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/metrics/
git commit -m "feat(metrics): add extended agent and team metrics"
```

---

### Task 3B: Metrics Persistence

**Files:**
- Create: `internal/tasks/store.go`
- Create: `internal/tasks/store_test.go`

**Step 1: Write the failing test**

```go
// internal/tasks/store_test.go
package tasks

import (
	"database/sql"
	"os"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func setupTestDB(t *testing.T) (*Store, func()) {
	f, err := os.CreateTemp("", "tasks-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	db, err := sql.Open("sqlite3", f.Name())
	if err != nil {
		t.Fatal(err)
	}

	store := NewStore(db)
	if err := store.Init(); err != nil {
		t.Fatal(err)
	}

	cleanup := func() {
		db.Close()
		os.Remove(f.Name())
	}

	return store, cleanup
}

func TestStoreSaveAndLoad(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	task := NewTask("Test task", "Description", 3)

	// Save
	if err := store.Save(task); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Load
	loaded, err := store.GetByID(task.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if loaded.Title != task.Title {
		t.Errorf("title mismatch: %q != %q", loaded.Title, task.Title)
	}
	if loaded.Priority != task.Priority {
		t.Errorf("priority mismatch: %d != %d", loaded.Priority, task.Priority)
	}
}

func TestStoreGetByStatus(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	t1 := NewTask("Task 1", "", 3)
	t2 := NewTask("Task 2", "", 3)
	t2.Status = StatusAssigned

	store.Save(t1)
	store.Save(t2)

	pending, err := store.GetByStatus(StatusPending)
	if err != nil {
		t.Fatal(err)
	}

	if len(pending) != 1 {
		t.Errorf("expected 1 pending task, got %d", len(pending))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/tasks/... -v -run Store`
Expected: FAIL with "undefined: Store"

**Step 3: Write minimal implementation**

```go
// internal/tasks/store.go
package tasks

import (
	"database/sql"
	"encoding/json"
	"time"
)

// Store persists tasks to SQLite
type Store struct {
	db *sql.DB
}

// NewStore creates a new task store
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// Init creates the tasks table
func (s *Store) Init() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS tasks (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			description TEXT,
			priority INTEGER NOT NULL DEFAULT 5,
			status TEXT NOT NULL DEFAULT 'pending',
			source TEXT NOT NULL DEFAULT 'captain',
			repo TEXT,
			assigned_to TEXT,
			branch TEXT,
			pr_url TEXT,
			metadata TEXT,
			created_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL,
			started_at TIMESTAMP,
			completed_at TIMESTAMP
		)
	`)
	return err
}

// Save creates or updates a task
func (s *Store) Save(task *Task) error {
	metadata, _ := json.Marshal(task.Metadata)

	_, err := s.db.Exec(`
		INSERT INTO tasks (id, title, description, priority, status, source, repo, assigned_to, branch, pr_url, metadata, created_at, updated_at, started_at, completed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			title=excluded.title,
			description=excluded.description,
			priority=excluded.priority,
			status=excluded.status,
			assigned_to=excluded.assigned_to,
			branch=excluded.branch,
			pr_url=excluded.pr_url,
			metadata=excluded.metadata,
			updated_at=excluded.updated_at,
			started_at=excluded.started_at,
			completed_at=excluded.completed_at
	`,
		task.ID, task.Title, task.Description, task.Priority,
		task.Status, task.Source, task.Repo, task.AssignedTo,
		task.Branch, task.PRUrl, string(metadata),
		task.CreatedAt, task.UpdatedAt, task.StartedAt, task.CompletedAt,
	)
	return err
}

// GetByID retrieves a task by ID
func (s *Store) GetByID(id string) (*Task, error) {
	row := s.db.QueryRow(`
		SELECT id, title, description, priority, status, source, repo, assigned_to, branch, pr_url, metadata, created_at, updated_at, started_at, completed_at
		FROM tasks WHERE id = ?
	`, id)

	return s.scanTask(row)
}

// GetByStatus retrieves all tasks with a given status
func (s *Store) GetByStatus(status TaskStatus) ([]*Task, error) {
	rows, err := s.db.Query(`
		SELECT id, title, description, priority, status, source, repo, assigned_to, branch, pr_url, metadata, created_at, updated_at, started_at, completed_at
		FROM tasks WHERE status = ? ORDER BY priority, created_at
	`, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return s.scanTasks(rows)
}

// GetAll retrieves all tasks
func (s *Store) GetAll() ([]*Task, error) {
	rows, err := s.db.Query(`
		SELECT id, title, description, priority, status, source, repo, assigned_to, branch, pr_url, metadata, created_at, updated_at, started_at, completed_at
		FROM tasks ORDER BY priority, created_at
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return s.scanTasks(rows)
}

// Delete removes a task
func (s *Store) Delete(id string) error {
	_, err := s.db.Exec(`DELETE FROM tasks WHERE id = ?`, id)
	return err
}

func (s *Store) scanTask(row *sql.Row) (*Task, error) {
	var task Task
	var metadata string
	var startedAt, completedAt sql.NullTime
	var repo, assignedTo, branch, prUrl sql.NullString

	err := row.Scan(
		&task.ID, &task.Title, &task.Description, &task.Priority,
		&task.Status, &task.Source, &repo, &assignedTo,
		&branch, &prUrl, &metadata,
		&task.CreatedAt, &task.UpdatedAt, &startedAt, &completedAt,
	)
	if err != nil {
		return nil, err
	}

	if repo.Valid {
		task.Repo = repo.String
	}
	if assignedTo.Valid {
		task.AssignedTo = assignedTo.String
	}
	if branch.Valid {
		task.Branch = branch.String
	}
	if prUrl.Valid {
		task.PRUrl = prUrl.String
	}
	if startedAt.Valid {
		task.StartedAt = &startedAt.Time
	}
	if completedAt.Valid {
		task.CompletedAt = &completedAt.Time
	}
	if metadata != "" {
		json.Unmarshal([]byte(metadata), &task.Metadata)
	}

	return &task, nil
}

func (s *Store) scanTasks(rows *sql.Rows) ([]*Task, error) {
	var tasks []*Task
	for rows.Next() {
		var task Task
		var metadata string
		var startedAt, completedAt sql.NullTime
		var repo, assignedTo, branch, prUrl sql.NullString

		err := rows.Scan(
			&task.ID, &task.Title, &task.Description, &task.Priority,
			&task.Status, &task.Source, &repo, &assignedTo,
			&branch, &prUrl, &metadata,
			&task.CreatedAt, &task.UpdatedAt, &startedAt, &completedAt,
		)
		if err != nil {
			return nil, err
		}

		if repo.Valid {
			task.Repo = repo.String
		}
		if assignedTo.Valid {
			task.AssignedTo = assignedTo.String
		}
		if branch.Valid {
			task.Branch = branch.String
		}
		if prUrl.Valid {
			task.PRUrl = prUrl.String
		}
		if startedAt.Valid {
			task.StartedAt = &startedAt.Time
		}
		if completedAt.Valid {
			task.CompletedAt = &completedAt.Time
		}
		if metadata != "" {
			json.Unmarshal([]byte(metadata), &task.Metadata)
		}

		tasks = append(tasks, &task)
	}
	return tasks, nil
}

// RecordHistory saves a status transition
func (s *Store) RecordHistory(taskID, fromStatus, toStatus, changedBy, reason string) error {
	_, err := s.db.Exec(`
		INSERT INTO task_history (task_id, from_status, to_status, changed_by, reason, changed_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, taskID, fromStatus, toStatus, changedBy, reason, time.Now())
	return err
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/tasks/... -v -run Store`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/tasks/
git commit -m "feat(tasks): add SQLite persistence store"
```

---

## Phase 4: Dashboard UI (Sequential after Phase 1-3)

### Task 4A: API Handlers for Tasks

**Files:**
- Create: `internal/handlers/tasks.go`
- Create: `internal/handlers/tasks_test.go`

**Step 1: Write the failing test**

```go
// internal/handlers/tasks_test.go
package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/CLIAIMONITOR/internal/tasks"
)

func TestTasksListHandler(t *testing.T) {
	queue := tasks.NewQueue()
	queue.Add(tasks.NewTask("Task 1", "Desc", 3))
	queue.Add(tasks.NewTask("Task 2", "Desc", 1))

	handler := NewTasksHandler(queue, nil)

	req := httptest.NewRequest("GET", "/api/tasks", nil)
	w := httptest.NewRecorder()

	handler.HandleList(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var response struct {
		Tasks []*tasks.Task `json:"tasks"`
		Total int           `json:"total"`
	}
	json.NewDecoder(w.Body).Decode(&response)

	if response.Total != 2 {
		t.Errorf("expected 2 tasks, got %d", response.Total)
	}

	// Should be priority-sorted (1 before 3)
	if response.Tasks[0].Priority != 1 {
		t.Error("tasks should be priority sorted")
	}
}

func TestTasksCreateHandler(t *testing.T) {
	queue := tasks.NewQueue()
	handler := NewTasksHandler(queue, nil)

	body := bytes.NewBufferString(`{"title":"New task","description":"Test","priority":2}`)
	req := httptest.NewRequest("POST", "/api/tasks", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.HandleCreate(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	if queue.Len() != 1 {
		t.Error("task should be added to queue")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/handlers/... -v -run Tasks`
Expected: FAIL with "undefined: NewTasksHandler"

**Step 3: Write minimal implementation**

```go
// internal/handlers/tasks.go
package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/CLIAIMONITOR/internal/tasks"
	"github.com/gorilla/mux"
)

// TasksHandler handles task-related HTTP endpoints
type TasksHandler struct {
	queue *tasks.Queue
	store *tasks.Store
}

// NewTasksHandler creates a new tasks handler
func NewTasksHandler(queue *tasks.Queue, store *tasks.Store) *TasksHandler {
	return &TasksHandler{
		queue: queue,
		store: store,
	}
}

// HandleList returns all tasks
func (h *TasksHandler) HandleList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Filter by status if provided
	status := r.URL.Query().Get("status")
	var taskList []*tasks.Task

	if status != "" {
		taskList = h.queue.GetByStatus(tasks.TaskStatus(status))
	} else {
		taskList = h.queue.All()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"tasks": taskList,
		"total": len(taskList),
	})
}

// HandleCreate creates a new task
func (h *TasksHandler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Priority    int    `json:"priority"`
		Repo        string `json:"repo,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Title == "" {
		http.Error(w, "Title is required", http.StatusBadRequest)
		return
	}

	if req.Priority < 1 || req.Priority > 7 {
		req.Priority = 5 // Default
	}

	task := tasks.NewTask(req.Title, req.Description, req.Priority)
	task.Source = tasks.SourceDashboard
	task.Repo = req.Repo

	h.queue.Add(task)

	// Persist if store available
	if h.store != nil {
		h.store.Save(task)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(task)
}

// HandleGet returns a single task by ID
func (h *TasksHandler) HandleGet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	vars := mux.Vars(r)
	id := vars["id"]

	task := h.queue.GetByID(id)
	if task == nil {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(task)
}

// HandleUpdate updates a task
func (h *TasksHandler) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch && r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	vars := mux.Vars(r)
	id := vars["id"]

	task := h.queue.GetByID(id)
	if task == nil {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	var updates struct {
		Priority *int    `json:"priority,omitempty"`
		Status   *string `json:"status,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if updates.Priority != nil {
		task.Priority = *updates.Priority
	}
	if updates.Status != nil {
		if err := task.TransitionTo(tasks.TaskStatus(*updates.Status)); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	h.queue.Update(task)

	if h.store != nil {
		h.store.Save(task)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(task)
}

// HandleDelete removes a task
func (h *TasksHandler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	vars := mux.Vars(r)
	id := vars["id"]

	if !h.queue.Remove(id) {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	if h.store != nil {
		h.store.Delete(id)
	}

	w.WriteHeader(http.StatusNoContent)
}

// HandleAgentTasks returns tasks for a specific agent
func (h *TasksHandler) HandleAgentTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	vars := mux.Vars(r)
	agentID := vars["agent_id"]

	taskList := h.queue.GetByAgent(agentID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"agent_id": agentID,
		"tasks":    taskList,
		"total":    len(taskList),
	})
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/handlers/... -v -run Tasks`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/handlers/
git commit -m "feat(api): add task management endpoints"
```

---

### Task 4B: Agent-Centric Dashboard HTML

**Files:**
- Modify: `web/index.html`

**Step 1: Update dashboard HTML**

Replace `web/index.html` with agent-centric layout:

```html
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>CLIAIMONITOR - team-coop</title>
    <link rel="stylesheet" href="/style.css">
</head>
<body>
    <div id="notification-banner" class="notification-banner" style="display: none;">
        <div class="notification-content">
            <span class="notification-icon"></span>
            <span id="notification-message" class="notification-message"></span>
            <button id="notification-dismiss" class="notification-dismiss">&times;</button>
        </div>
    </div>

    <div id="app">
        <header class="header">
            <h1>CLIAIMONITOR</h1>
            <div class="header-controls">
                <div class="status-bar">
                    <span class="status-indicator" id="nats-status">NATS: <span class="dot"></span></span>
                    <span class="status-indicator" id="captain-status">Captain: <span class="dot"></span> <span class="status-text">--</span></span>
                </div>
                <button id="new-task-btn" class="btn btn-primary">+ New Task</button>
                <button id="mute-toggle" class="btn btn-icon" title="Toggle Sound">🔔</button>
            </div>
        </header>

        <!-- Summary Bar -->
        <section class="summary-bar">
            <div class="summary-item">
                <span class="summary-label">Active</span>
                <span id="summary-active" class="summary-value">0</span>
            </div>
            <div class="summary-item">
                <span class="summary-label">Pending</span>
                <span id="summary-pending" class="summary-value">0</span>
            </div>
            <div class="summary-item">
                <span class="summary-label">In Review</span>
                <span id="summary-review" class="summary-value">0</span>
            </div>
            <div class="summary-item">
                <span class="summary-label">Tokens Today</span>
                <span id="summary-tokens" class="summary-value">0</span>
            </div>
            <div class="summary-item">
                <span class="summary-label">Est. Cost</span>
                <span id="summary-cost" class="summary-value">$0.00</span>
            </div>
        </section>

        <!-- Agent Cards -->
        <section class="agent-lanes">
            <div id="agent-cards" class="agent-cards-grid">
                <!-- Agent cards populated by JS -->
            </div>
        </section>

        <!-- Pending Queue -->
        <section class="panel pending-queue-panel">
            <h2>Pending Queue <span id="pending-count" class="badge">0</span></h2>
            <table class="task-table">
                <thead>
                    <tr>
                        <th>Priority</th>
                        <th>Title</th>
                        <th>Repo</th>
                        <th>Status</th>
                        <th>Age</th>
                        <th>Actions</th>
                    </tr>
                </thead>
                <tbody id="pending-queue-body">
                    <!-- Tasks populated by JS -->
                </tbody>
            </table>
        </section>

        <!-- Bottom Row: Alerts & Metrics -->
        <div class="bottom-row">
            <section class="panel alerts-panel">
                <h2>Alerts <span id="alert-count" class="badge">0</span></h2>
                <div id="alerts-list" class="alerts-list"></div>
            </section>

            <section class="panel metrics-panel">
                <h2>Team Metrics</h2>
                <div class="metrics-grid">
                    <div class="metric-card">
                        <span class="metric-label">Tasks Completed</span>
                        <span id="metric-tasks" class="metric-value">0</span>
                    </div>
                    <div class="metric-card">
                        <span class="metric-label">Avg Tokens/Task</span>
                        <span id="metric-tokens-per-task" class="metric-value">0</span>
                    </div>
                    <div class="metric-card">
                        <span class="metric-label">Avg Time/Task</span>
                        <span id="metric-time-per-task" class="metric-value">--</span>
                    </div>
                    <div class="metric-card">
                        <span class="metric-label">Review Rejections</span>
                        <span id="metric-rejections" class="metric-value">0</span>
                    </div>
                </div>
            </section>
        </div>
    </div>

    <!-- New Task Modal -->
    <div id="task-modal" class="modal" style="display: none;">
        <div class="modal-content">
            <h2>New Task</h2>
            <form id="task-form">
                <div class="form-group">
                    <label for="task-title">Title</label>
                    <input type="text" id="task-title" required>
                </div>
                <div class="form-group">
                    <label for="task-description">Description</label>
                    <textarea id="task-description" rows="3"></textarea>
                </div>
                <div class="form-row">
                    <div class="form-group">
                        <label for="task-priority">Priority</label>
                        <select id="task-priority">
                            <option value="1">P1 - Critical</option>
                            <option value="2">P2 - High</option>
                            <option value="3" selected>P3 - Standard</option>
                            <option value="4">P4 - Enhancement</option>
                            <option value="5">P5 - Normal</option>
                            <option value="6">P6 - Low</option>
                            <option value="7">P7 - Background</option>
                        </select>
                    </div>
                    <div class="form-group">
                        <label for="task-repo">Repository</label>
                        <select id="task-repo">
                            <option value="">Local</option>
                            <option value="MAH">MAH</option>
                            <option value="MSS">MSS</option>
                            <option value="mss-ai">mss-ai</option>
                        </select>
                    </div>
                </div>
                <div class="form-actions">
                    <button type="button" class="btn" onclick="closeTaskModal()">Cancel</button>
                    <button type="submit" class="btn btn-primary">Create Task</button>
                </div>
            </form>
        </div>
    </div>

    <script src="/app.js"></script>
</body>
</html>
```

**Step 2: Commit**

```bash
git add web/index.html
git commit -m "feat(ui): agent-centric dashboard layout"
```

---

### Task 4C: Dashboard JavaScript Updates

**Files:**
- Modify: `web/app.js`

**Step 1: Update JavaScript**

Add/modify functions in `web/app.js`:

```javascript
// Add to Dashboard class

// Render agent cards
renderAgentCards() {
    const container = document.getElementById('agent-cards');
    if (!this.state || !this.state.agents) {
        container.innerHTML = '<p class="empty-state">No agents connected</p>';
        return;
    }

    const agents = Object.values(this.state.agents);
    if (agents.length === 0) {
        container.innerHTML = '<p class="empty-state">No agents connected</p>';
        return;
    }

    container.innerHTML = agents.map(agent => {
        const agentTasks = this.getAgentTasks(agent.id);
        const currentTask = agentTasks.find(t => t.status === 'in_progress');
        const queuedTasks = agentTasks.filter(t => t.status === 'assigned');

        return `
            <div class="agent-card ${agent.status}" style="border-color: ${agent.color}">
                <div class="agent-header">
                    <span class="agent-status-dot" style="background: ${this.getStatusColor(agent.status)}"></span>
                    <span class="agent-name">${this.escapeHtml(agent.config_name || agent.id)}</span>
                    <span class="agent-role">${this.escapeHtml(agent.role || '')}</span>
                </div>
                <div class="agent-current-task">
                    ${currentTask ? `
                        <div class="current-task">
                            <span class="task-indicator">▶</span>
                            <span class="task-id">${this.escapeHtml(currentTask.id)}</span>
                            <span class="task-title">${this.escapeHtml(currentTask.title)}</span>
                            <span class="task-time">${this.formatDuration(currentTask.started_at)}</span>
                        </div>
                    ` : `<span class="idle-state">idle</span>`}
                </div>
                <div class="agent-queue">
                    ${queuedTasks.slice(0, 3).map(t => `
                        <div class="queued-task">
                            <span class="queue-indicator">◦</span>
                            <span class="task-id">${this.escapeHtml(t.id)}</span>
                        </div>
                    `).join('')}
                    ${queuedTasks.length > 3 ? `<span class="more-tasks">+${queuedTasks.length - 3} more</span>` : ''}
                </div>
            </div>
        `;
    }).join('');
}

getAgentTasks(agentId) {
    if (!this.tasks) return [];
    return this.tasks.filter(t => t.assigned_to === agentId);
}

getStatusColor(status) {
    const colors = {
        'connected': '#00cc66',
        'working': '#00cc66',
        'idle': '#999',
        'blocked': '#ff9900',
        'disconnected': '#cc3333',
        'error': '#cc3333'
    };
    return colors[status] || '#999';
}

formatDuration(startTime) {
    if (!startTime) return '';
    const start = new Date(startTime);
    const now = new Date();
    const diff = Math.floor((now - start) / 1000);

    if (diff < 60) return `${diff}s`;
    if (diff < 3600) return `${Math.floor(diff / 60)}m`;
    return `${Math.floor(diff / 3600)}h ${Math.floor((diff % 3600) / 60)}m`;
}

// Load and render tasks
async loadTasks() {
    try {
        const response = await fetch('/api/tasks');
        const data = await response.json();
        this.tasks = data.tasks || [];
        this.renderPendingQueue();
        this.renderAgentCards();
        this.updateSummary();
    } catch (error) {
        console.error('Failed to load tasks:', error);
    }
}

renderPendingQueue() {
    const tbody = document.getElementById('pending-queue-body');
    const pending = (this.tasks || []).filter(t => t.status === 'pending');

    document.getElementById('pending-count').textContent = pending.length;

    if (pending.length === 0) {
        tbody.innerHTML = '<tr><td colspan="6" class="empty-state">No pending tasks</td></tr>';
        return;
    }

    tbody.innerHTML = pending.map(task => `
        <tr>
            <td><span class="priority-badge p${task.priority}">P${task.priority}</span></td>
            <td>${this.escapeHtml(task.title)}</td>
            <td>${this.escapeHtml(task.repo || 'local')}</td>
            <td>${task.status}</td>
            <td>${this.formatAge(task.created_at)}</td>
            <td>
                <button class="btn btn-small" onclick="dashboard.assignTask('${task.id}')">Assign</button>
            </td>
        </tr>
    `).join('');
}

formatAge(timestamp) {
    const created = new Date(timestamp);
    const now = new Date();
    const diff = Math.floor((now - created) / 1000);

    if (diff < 60) return 'just now';
    if (diff < 3600) return `${Math.floor(diff / 60)}m ago`;
    if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`;
    return `${Math.floor(diff / 86400)}d ago`;
}

updateSummary() {
    const tasks = this.tasks || [];
    const agents = this.state?.agents ? Object.values(this.state.agents) : [];

    document.getElementById('summary-active').textContent = agents.filter(a => a.status === 'working').length;
    document.getElementById('summary-pending').textContent = tasks.filter(t => t.status === 'pending').length;
    document.getElementById('summary-review').textContent = tasks.filter(t => t.status === 'review').length;

    // Token/cost from session stats
    const stats = this.state?.session_stats || {};
    document.getElementById('summary-tokens').textContent = this.formatNumber(stats.total_tokens_used || 0);
    document.getElementById('summary-cost').textContent = `$${(stats.total_estimated_cost || 0).toFixed(2)}`;
}

formatNumber(n) {
    return n.toString().replace(/\B(?=(\d{3})+(?!\d))/g, ',');
}

// Task modal
openTaskModal() {
    document.getElementById('task-modal').style.display = 'flex';
}

closeTaskModal() {
    document.getElementById('task-modal').style.display = 'none';
    document.getElementById('task-form').reset();
}

async createTask(e) {
    e.preventDefault();

    const task = {
        title: document.getElementById('task-title').value,
        description: document.getElementById('task-description').value,
        priority: parseInt(document.getElementById('task-priority').value),
        repo: document.getElementById('task-repo').value
    };

    try {
        const response = await fetch('/api/tasks', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(task)
        });

        if (response.ok) {
            this.closeTaskModal();
            this.loadTasks();
        } else {
            alert('Failed to create task');
        }
    } catch (error) {
        console.error('Create task error:', error);
        alert('Failed to create task');
    }
}

// Initialize in constructor
constructor() {
    // ... existing code ...
    this.tasks = [];
}

init() {
    // ... existing code ...
    this.loadTasks();
    setInterval(() => this.loadTasks(), 10000); // Refresh every 10s

    // Task modal bindings
    document.getElementById('new-task-btn')?.addEventListener('click', () => this.openTaskModal());
    document.getElementById('task-form')?.addEventListener('submit', (e) => this.createTask(e));
}

// Make closeTaskModal global for onclick
window.closeTaskModal = () => dashboard.closeTaskModal();
```

**Step 2: Commit**

```bash
git add web/app.js
git commit -m "feat(ui): add agent cards and task queue rendering"
```

---

### Task 4D: Dashboard CSS Updates

**Files:**
- Modify: `web/style.css`

**Step 1: Add agent-centric styles**

Add to `web/style.css`:

```css
/* Summary Bar */
.summary-bar {
    display: flex;
    gap: 2rem;
    padding: 1rem 2rem;
    background: var(--panel-bg);
    border-bottom: 1px solid var(--border-color);
}

.summary-item {
    display: flex;
    flex-direction: column;
    align-items: center;
}

.summary-label {
    font-size: 0.75rem;
    color: var(--text-muted);
    text-transform: uppercase;
}

.summary-value {
    font-size: 1.5rem;
    font-weight: bold;
    color: var(--text-primary);
}

/* Agent Cards Grid */
.agent-lanes {
    padding: 1rem 2rem;
}

.agent-cards-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(250px, 1fr));
    gap: 1rem;
}

.agent-card {
    background: var(--panel-bg);
    border: 2px solid var(--border-color);
    border-radius: 8px;
    padding: 1rem;
    border-left-width: 4px;
}

.agent-header {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    margin-bottom: 0.75rem;
}

.agent-status-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
}

.agent-name {
    font-weight: bold;
    flex: 1;
}

.agent-role {
    font-size: 0.75rem;
    color: var(--text-muted);
}

.agent-current-task {
    background: var(--bg-darker);
    padding: 0.5rem;
    border-radius: 4px;
    margin-bottom: 0.5rem;
}

.current-task {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    font-size: 0.875rem;
}

.task-indicator {
    color: var(--accent-green);
}

.task-id {
    color: var(--text-muted);
    font-family: monospace;
    font-size: 0.75rem;
}

.task-time {
    margin-left: auto;
    color: var(--text-muted);
    font-size: 0.75rem;
}

.idle-state {
    color: var(--text-muted);
    font-style: italic;
}

.agent-queue {
    font-size: 0.875rem;
}

.queued-task {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.25rem 0;
    color: var(--text-muted);
}

.queue-indicator {
    color: var(--text-muted);
}

.more-tasks {
    color: var(--text-muted);
    font-size: 0.75rem;
    font-style: italic;
}

/* Pending Queue Table */
.pending-queue-panel {
    margin: 1rem 2rem;
}

.task-table {
    width: 100%;
    border-collapse: collapse;
}

.task-table th,
.task-table td {
    padding: 0.75rem;
    text-align: left;
    border-bottom: 1px solid var(--border-color);
}

.task-table th {
    background: var(--bg-darker);
    font-weight: 600;
    font-size: 0.75rem;
    text-transform: uppercase;
    color: var(--text-muted);
}

.priority-badge {
    padding: 0.25rem 0.5rem;
    border-radius: 4px;
    font-size: 0.75rem;
    font-weight: bold;
}

.priority-badge.p1 { background: #cc3333; color: white; }
.priority-badge.p2 { background: #ff6600; color: white; }
.priority-badge.p3 { background: #ffcc00; color: black; }
.priority-badge.p4 { background: #66cc66; color: white; }
.priority-badge.p5 { background: #6699cc; color: white; }
.priority-badge.p6 { background: #999; color: white; }
.priority-badge.p7 { background: #666; color: white; }

/* Bottom Row */
.bottom-row {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 1rem;
    padding: 1rem 2rem;
}

/* Metrics Grid */
.metrics-grid {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 1rem;
}

.metric-card {
    background: var(--bg-darker);
    padding: 1rem;
    border-radius: 4px;
    text-align: center;
}

.metric-label {
    display: block;
    font-size: 0.75rem;
    color: var(--text-muted);
    margin-bottom: 0.5rem;
}

.metric-value {
    font-size: 1.5rem;
    font-weight: bold;
}

/* Modal */
.modal {
    position: fixed;
    top: 0;
    left: 0;
    width: 100%;
    height: 100%;
    background: rgba(0, 0, 0, 0.7);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 1000;
}

.modal-content {
    background: var(--panel-bg);
    padding: 2rem;
    border-radius: 8px;
    width: 500px;
    max-width: 90%;
}

.modal-content h2 {
    margin-top: 0;
    margin-bottom: 1.5rem;
}

.form-group {
    margin-bottom: 1rem;
}

.form-group label {
    display: block;
    margin-bottom: 0.5rem;
    font-weight: 500;
}

.form-group input,
.form-group textarea,
.form-group select {
    width: 100%;
    padding: 0.5rem;
    border: 1px solid var(--border-color);
    border-radius: 4px;
    background: var(--bg-darker);
    color: var(--text-primary);
}

.form-row {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 1rem;
}

.form-actions {
    display: flex;
    justify-content: flex-end;
    gap: 1rem;
    margin-top: 1.5rem;
}

/* Empty state */
.empty-state {
    color: var(--text-muted);
    text-align: center;
    padding: 2rem;
    font-style: italic;
}
```

**Step 2: Commit**

```bash
git add web/style.css
git commit -m "feat(ui): add agent-centric dashboard styles"
```

---

## Phase 5: Captain Integration (Sequential)

### Task 5A: Captain Task Assignment

**Files:**
- Modify: `internal/captain/captain.go`

**Step 1: Add task assignment methods**

Add to `internal/captain/captain.go`:

```go
// TaskManager interface for Captain to interact with task queue
type TaskManager interface {
	GetNextTask() *tasks.Task
	AssignTask(taskID, agentID string) error
	CompleteTask(taskID string) error
	FailTask(taskID, reason string) error
}

// AssignNextTask picks the highest priority pending task and assigns to best agent
func (c *Captain) AssignNextTask(tm TaskManager) error {
	task := tm.GetNextTask()
	if task == nil {
		return nil // No tasks pending
	}

	// Select best agent for this task
	agentType := c.selectAgentForTask(task)
	agentID := c.spawner.GenerateAgentID(agentType)

	// Create branch for the task
	branchName := git.BranchName(task.ID, task.Title)
	g := git.New(c.resolveProjectPath(task.Repo))

	if err := g.CreateBranch(branchName); err != nil {
		return fmt.Errorf("failed to create branch: %w", err)
	}

	// Update task
	task.Branch = branchName
	if err := tm.AssignTask(task.ID, agentID); err != nil {
		return fmt.Errorf("failed to assign task: %w", err)
	}

	// Spawn agent with task context
	mission := Mission{
		ID:          task.ID,
		Title:       task.Title,
		Description: task.Description,
		TaskType:    TaskImplementation,
		ProjectPath: c.resolveProjectPath(task.Repo),
		Priority:    task.Priority,
		Metadata: map[string]string{
			"branch":  branchName,
			"task_id": task.ID,
		},
	}

	_, err := c.ExecuteMission(context.Background(), mission)
	return err
}

// selectAgentForTask picks the best agent type based on task characteristics
func (c *Captain) selectAgentForTask(task *tasks.Task) string {
	title := strings.ToLower(task.Title)

	// Security tasks -> OpusRed
	if strings.Contains(title, "security") || strings.Contains(title, "vulnerability") {
		return "OpusRed"
	}

	// Review/audit tasks -> SNTPurple
	if strings.Contains(title, "review") || strings.Contains(title, "audit") {
		return "SNTPurple"
	}

	// High priority -> Opus
	if task.Priority <= 2 {
		return "OpusGreen"
	}

	// Default -> Sonnet
	return "SNTGreen"
}

// OnTaskComplete handles task completion, creates PR
func (c *Captain) OnTaskComplete(tm TaskManager, task *tasks.Task, metrics PRMetrics) error {
	g := git.New(c.resolveProjectPath(task.Repo))

	// Check if there are changes to commit
	hasChanges, err := g.HasUncommittedChanges()
	if err != nil {
		return fmt.Errorf("failed to check changes: %w", err)
	}

	if hasChanges {
		// Add all changes
		if err := g.Add("."); err != nil {
			return fmt.Errorf("failed to stage changes: %w", err)
		}

		// Commit
		commitMsg := fmt.Sprintf("feat: %s\n\nTask-ID: %s\nGenerated by team-coop", task.Title, task.ID)
		if err := g.Commit(commitMsg); err != nil {
			return fmt.Errorf("failed to commit: %w", err)
		}
	}

	// Push branch
	if err := g.Push(); err != nil {
		return fmt.Errorf("failed to push: %w", err)
	}

	// Create PR
	prInfo := git.PRInfo{
		Title:   task.Title,
		Summary: task.Description,
		TaskIDs: []string{task.ID},
		Agents:  []string{task.AssignedTo},
		Metrics: metrics,
	}

	prURL, err := g.CreatePR(prInfo)
	if err != nil {
		return fmt.Errorf("failed to create PR: %w", err)
	}

	task.PRUrl = prURL
	return tm.CompleteTask(task.ID)
}
```

**Step 2: Commit**

```bash
git add internal/captain/
git commit -m "feat(captain): add task assignment and PR creation"
```

---

### Task 5B: MCP Tools for Agents

**Files:**
- Modify: `internal/mcp/tools.go`

**Step 1: Add task-related MCP tools**

Add to `internal/mcp/tools.go`:

```go
// GetAssignedTaskTool returns the agent's currently assigned task
var GetAssignedTaskTool = Tool{
	Name:        "get_assigned_task",
	Description: "Get the task currently assigned to this agent",
	InputSchema: map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"agent_id": map[string]interface{}{
				"type":        "string",
				"description": "The agent's ID",
			},
		},
		"required": []string{"agent_id"},
	},
}

// SignalTaskDoneTool signals that the agent has completed their task
var SignalTaskDoneTool = Tool{
	Name:        "signal_task_done",
	Description: "Signal that you have completed the assigned task",
	InputSchema: map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"agent_id": map[string]interface{}{
				"type":        "string",
				"description": "The agent's ID",
			},
			"task_id": map[string]interface{}{
				"type":        "string",
				"description": "The task ID that was completed",
			},
			"summary": map[string]interface{}{
				"type":        "string",
				"description": "Brief summary of work completed",
			},
			"tokens_used": map[string]interface{}{
				"type":        "integer",
				"description": "Approximate tokens used for this task",
			},
		},
		"required": []string{"agent_id", "task_id", "summary"},
	},
}

// RequestTaskChangeTool flags a blocker or requests changes to task
var RequestTaskChangeTool = Tool{
	Name:        "request_task_change",
	Description: "Flag a blocker or request clarification on the task",
	InputSchema: map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"agent_id": map[string]interface{}{
				"type":        "string",
				"description": "The agent's ID",
			},
			"task_id": map[string]interface{}{
				"type":        "string",
				"description": "The task ID",
			},
			"issue_type": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"blocker", "clarification", "dependency", "scope_change"},
				"description": "Type of issue",
			},
			"description": map[string]interface{}{
				"type":        "string",
				"description": "Description of the issue",
			},
		},
		"required": []string{"agent_id", "task_id", "issue_type", "description"},
	},
}

// Add to AllTools slice
func init() {
	AllTools = append(AllTools,
		GetAssignedTaskTool,
		SignalTaskDoneTool,
		RequestTaskChangeTool,
	)
}
```

**Step 2: Add handlers**

Add to MCP handlers:

```go
// HandleGetAssignedTask returns the agent's current task
func (h *ToolHandler) HandleGetAssignedTask(args map[string]interface{}) (interface{}, error) {
	agentID, _ := args["agent_id"].(string)
	if agentID == "" {
		return nil, fmt.Errorf("agent_id required")
	}

	tasks := h.taskQueue.GetByAgent(agentID)
	for _, t := range tasks {
		if t.Status == tasks.StatusInProgress || t.Status == tasks.StatusAssigned {
			return t, nil
		}
	}

	return map[string]string{"status": "no_task_assigned"}, nil
}

// HandleSignalTaskDone processes task completion
func (h *ToolHandler) HandleSignalTaskDone(args map[string]interface{}) (interface{}, error) {
	agentID, _ := args["agent_id"].(string)
	taskID, _ := args["task_id"].(string)
	summary, _ := args["summary"].(string)
	tokensUsed, _ := args["tokens_used"].(float64)

	task := h.taskQueue.GetByID(taskID)
	if task == nil {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}

	if task.AssignedTo != agentID {
		return nil, fmt.Errorf("task not assigned to this agent")
	}

	// Transition to review
	task.TransitionTo(tasks.StatusReview)
	h.taskQueue.Update(task)

	// Record metrics
	h.metrics.RecordTaskCompletion(taskID, agentID, int64(tokensUsed))

	return map[string]interface{}{
		"status":  "task_moved_to_review",
		"task_id": taskID,
		"summary": summary,
	}, nil
}

// HandleRequestTaskChange flags an issue
func (h *ToolHandler) HandleRequestTaskChange(args map[string]interface{}) (interface{}, error) {
	agentID, _ := args["agent_id"].(string)
	taskID, _ := args["task_id"].(string)
	issueType, _ := args["issue_type"].(string)
	description, _ := args["description"].(string)

	task := h.taskQueue.GetByID(taskID)
	if task == nil {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}

	// Transition to blocked if blocker
	if issueType == "blocker" || issueType == "dependency" {
		task.TransitionTo(tasks.StatusBlocked)
	}

	// Create escalation
	h.escalations.Add(Escalation{
		TaskID:      taskID,
		AgentID:     agentID,
		Type:        issueType,
		Description: description,
	})

	h.taskQueue.Update(task)

	return map[string]interface{}{
		"status":      "issue_flagged",
		"task_status": task.Status,
	}, nil
}
```

**Step 3: Commit**

```bash
git add internal/mcp/
git commit -m "feat(mcp): add task management tools for agents"
```

---

## Phase 6: Wire Everything Together (Sequential)

### Task 6A: Update Main and Router Registration

**Files:**
- Modify: `cmd/cliaimonitor/main.go`
- Modify: Server route registration

**Step 1: Update main.go**

Add task queue initialization and route registration:

```go
// In main.go, add:

// Initialize task queue
taskQueue := tasks.NewQueue()

// Initialize task store (uses memory DB connection)
taskStore := tasks.NewStore(memDB.DB())
taskStore.Init()

// Load persisted tasks into queue
savedTasks, _ := taskStore.GetAll()
for _, t := range savedTasks {
	taskQueue.Add(t)
}

// Create handlers
taskHandler := handlers.NewTasksHandler(taskQueue, taskStore)

// Register routes
router.HandleFunc("/api/tasks", taskHandler.HandleList).Methods("GET")
router.HandleFunc("/api/tasks", taskHandler.HandleCreate).Methods("POST")
router.HandleFunc("/api/tasks/{id}", taskHandler.HandleGet).Methods("GET")
router.HandleFunc("/api/tasks/{id}", taskHandler.HandleUpdate).Methods("PATCH", "PUT")
router.HandleFunc("/api/tasks/{id}", taskHandler.HandleDelete).Methods("DELETE")
router.HandleFunc("/api/agents/{agent_id}/tasks", taskHandler.HandleAgentTasks).Methods("GET")
```

**Step 2: Commit**

```bash
git add cmd/cliaimonitor/
git commit -m "feat: wire up task queue and API routes"
```

---

### Task 6B: Final Integration Test

**Step 1: Run full test suite**

```bash
go test ./... -v
```

Expected: All tests pass

**Step 2: Manual verification**

```bash
# Build and run
go build -o cliaimonitor.exe ./cmd/cliaimonitor
./cliaimonitor.exe

# In another terminal:
# Create a task
curl -X POST http://localhost:3000/api/tasks \
  -H "Content-Type: application/json" \
  -d '{"title":"Test task","description":"Testing the system","priority":3}'

# List tasks
curl http://localhost:3000/api/tasks

# Check dashboard
# Open http://localhost:3000 in browser
```

**Step 3: Final commit**

```bash
git add .
git commit -m "feat: complete orchestrator improvements implementation"
```

---

## Parallel Execution Guide

Tasks that can run in parallel:
- **Phase 1**: 1A, 1B, 1C (all independent)
- **Phase 2**: 2A, 2B (both git-related but independent)
- **Phase 3**: 3A, 3B (both metrics-related but independent)
- **Phase 4**: 4A must complete first, then 4B/4C/4D can run in parallel
- **Phase 5**: 5A, 5B (independent captain and MCP changes)
- **Phase 6**: Sequential (wiring depends on all previous phases)

For maximum parallelism, spawn 3 agents:
1. Agent 1: Phase 1A → 2A → 3A → 4B → 5A
2. Agent 2: Phase 1B → 2B → 3B → 4C → 5B
3. Agent 3: Phase 1C → 4A → 4D → 6A → 6B

---

## Verification Checklist

- [ ] All tests pass: `go test ./...`
- [ ] Build succeeds: `go build ./cmd/cliaimonitor`
- [ ] Dashboard loads at http://localhost:3000
- [ ] Agent cards display correctly
- [ ] Can create tasks via UI
- [ ] Tasks appear in pending queue
- [ ] Metrics display in dashboard
- [ ] Captain can assign tasks (check logs)
