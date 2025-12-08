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
