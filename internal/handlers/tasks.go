// internal/handlers/tasks.go
package handlers

import (
	"encoding/json"
	"net/http"
	"time"

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

	// Limit request size to prevent DoS
	limitRequestSize(r, MaxPayloadSize)

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

	task := tasks.NewTask(req.Title, req.Description, req.Priority)
	if req.Repo != "" {
		task.Repo = req.Repo
	}

	if err := task.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.queue.Add(task)

	if h.store != nil {
		if err := h.store.Save(task); err != nil {
			// Log error but don't fail the request
			// The task is already in memory queue
		}
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

	// Limit request size to prevent DoS
	limitRequestSize(r, MaxPayloadSize)

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

	task.UpdatedAt = time.Now()
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
