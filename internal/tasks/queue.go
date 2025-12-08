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
