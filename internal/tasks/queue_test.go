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
