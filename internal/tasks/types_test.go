// internal/tasks/types_test.go
package tasks

import (
	"testing"
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
		task := &Task{Title: "Test", Priority: tt.priority}
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
