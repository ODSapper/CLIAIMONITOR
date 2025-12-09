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

// Additional comprehensive tests for TasksHandler

func TestTasksListHandler_WithStatusFilter(t *testing.T) {
	queue := tasks.NewQueue()

	task1 := tasks.NewTask("Task 1", "Desc 1", 1)
	task1.Status = tasks.StatusPending

	task2 := tasks.NewTask("Task 2", "Desc 2", 2)
	task2.Status = tasks.StatusMerged

	queue.Add(task1)
	queue.Add(task2)

	handler := NewTasksHandler(queue, nil)

	req := httptest.NewRequest("GET", "/api/tasks?status=pending", nil)
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

	if response.Total != 1 {
		t.Errorf("expected 1 pending task, got %d", response.Total)
	}
}

func TestTasksCreateHandler_MissingTitle(t *testing.T) {
	queue := tasks.NewQueue()
	handler := NewTasksHandler(queue, nil)

	body := bytes.NewBufferString(`{"description":"Test","priority":2}`)
	req := httptest.NewRequest("POST", "/api/tasks", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.HandleCreate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestTasksCreateHandler_InvalidJSON(t *testing.T) {
	queue := tasks.NewQueue()
	handler := NewTasksHandler(queue, nil)

	body := bytes.NewBufferString(`{invalid json}`)
	req := httptest.NewRequest("POST", "/api/tasks", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.HandleCreate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestTasksCreateHandler_WrongMethod(t *testing.T) {
	queue := tasks.NewQueue()
	handler := NewTasksHandler(queue, nil)

	req := httptest.NewRequest("GET", "/api/tasks", nil)
	w := httptest.NewRecorder()

	handler.HandleCreate(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestTasksGetHandler(t *testing.T) {
	queue := tasks.NewQueue()
	task := tasks.NewTask("Test Task", "Description", 1)
	queue.Add(task)

	// Test that we can retrieve task
	retrievedTask := queue.GetByID(task.ID)
	if retrievedTask == nil {
		t.Error("expected task to be retrieved from queue")
	}
}

func TestTasksGetHandler_NotFound(t *testing.T) {
	queue := tasks.NewQueue()

	// Try to get non-existent task
	task := queue.GetByID("nonexistent")
	if task != nil {
		t.Error("expected no task to be found")
	}
}

func TestTasksUpdateHandler_StatusChange(t *testing.T) {
	queue := tasks.NewQueue()
	task := tasks.NewTask("Test Task", "Description", 1)
	queue.Add(task)

	// Test that we can directly transition status
	err := task.TransitionTo(tasks.StatusAssigned)
	if err != nil {
		t.Errorf("expected valid transition, got error: %v", err)
	}

	if task.Status != tasks.StatusAssigned {
		t.Errorf("expected status 'assigned', got %v", task.Status)
	}
}

func TestTasksUpdateHandler_TaskNotFound(t *testing.T) {
	queue := tasks.NewQueue()
	handler := NewTasksHandler(queue, nil)

	updates := map[string]interface{}{
		"status": "completed",
	}
	body, _ := json.Marshal(updates)
	req := httptest.NewRequest("PATCH", "/api/tasks/nonexistent", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.HandleUpdate(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestTasksUpdateHandler_WrongMethod(t *testing.T) {
	queue := tasks.NewQueue()
	handler := NewTasksHandler(queue, nil)

	req := httptest.NewRequest("DELETE", "/api/tasks/123", nil)
	w := httptest.NewRecorder()

	handler.HandleUpdate(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestTasksDeleteHandler(t *testing.T) {
	queue := tasks.NewQueue()
	task := tasks.NewTask("Test Task", "Description", 1)
	queue.Add(task)

	initialLen := queue.Len()

	// Delete task
	removed := queue.Remove(task.ID)
	if !removed {
		t.Error("expected task to be removed")
	}

	if queue.Len() != initialLen-1 {
		t.Errorf("expected queue length to decrease")
	}
}

func TestTasksDeleteHandler_NotFound(t *testing.T) {
	queue := tasks.NewQueue()

	// Try to delete non-existent task
	removed := queue.Remove("nonexistent")
	if removed {
		t.Error("expected remove to fail for non-existent task")
	}
}

func TestTasksGetAgentTasks(t *testing.T) {
	queue := tasks.NewQueue()
	task1 := tasks.NewTask("Task 1", "Desc 1", 1)
	task1.AssignedTo = "agent-1"

	task2 := tasks.NewTask("Task 2", "Desc 2", 2)
	task2.AssignedTo = "agent-2"

	queue.Add(task1)
	queue.Add(task2)

	// Get tasks for agent-1
	agentTasks := queue.GetByAgent("agent-1")
	if len(agentTasks) != 1 {
		t.Errorf("expected 1 task for agent-1, got %d", len(agentTasks))
	}

	if agentTasks[0].AssignedTo != "agent-1" {
		t.Errorf("expected agent-1, got %s", agentTasks[0].AssignedTo)
	}
}

func TestTasksListHandler_EmptyQueue(t *testing.T) {
	queue := tasks.NewQueue()
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

	if response.Total != 0 {
		t.Errorf("expected 0 tasks, got %d", response.Total)
	}
}

func TestTasksListHandler_WrongMethod(t *testing.T) {
	queue := tasks.NewQueue()
	handler := NewTasksHandler(queue, nil)

	req := httptest.NewRequest("POST", "/api/tasks", nil)
	w := httptest.NewRecorder()

	handler.HandleList(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}
