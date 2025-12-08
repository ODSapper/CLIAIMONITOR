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
