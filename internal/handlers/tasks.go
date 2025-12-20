// internal/handlers/tasks.go
package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
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

// HandleList returns all tasks with pagination support
func (h *TasksHandler) HandleList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse pagination parameters
	query := r.URL.Query()
	limit := 100 // default
	offset := 0

	if l := query.Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 1000 {
			limit = parsed
		}
	}
	if o := query.Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	// Filter by status if provided
	status := query.Get("status")
	var taskList []*tasks.Task

	if status != "" {
		taskList = h.queue.GetByStatus(tasks.TaskStatus(status))
	} else {
		taskList = h.queue.All()
	}

	// Apply pagination
	total := len(taskList)
	if offset < len(taskList) {
		end := offset + limit
		if end > len(taskList) {
			end = len(taskList)
		}
		taskList = taskList[offset:end]
	} else {
		taskList = []*tasks.Task{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"tasks":  taskList,
		"total":  total,
		"limit":  limit,
		"offset": offset,
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
			log.Printf("[TASKS] Failed to persist task %s: %v", task.ID, err)
			// Task is already in memory queue, continue
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

	// Validate task ID
	if id == "" || len(id) > 100 {
		http.Error(w, "Invalid task ID", http.StatusBadRequest)
		return
	}

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

	// Validate task ID
	if id == "" || len(id) > 100 {
		http.Error(w, "Invalid task ID", http.StatusBadRequest)
		return
	}

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
		if err := h.store.Save(task); err != nil {
			log.Printf("[TASKS] Failed to persist updated task %s: %v", task.ID, err)
			// Task is already in memory queue, continue
		}
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

	// Validate task ID
	if id == "" || len(id) > 100 {
		http.Error(w, "Invalid task ID", http.StatusBadRequest)
		return
	}

	if !h.queue.Remove(id) {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	if h.store != nil {
		if err := h.store.Delete(id); err != nil {
			log.Printf("[TASKS] Failed to delete task %s from store: %v", id, err)
			// Task already removed from memory queue, continue
		}
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

	// Validate agent ID
	if agentID == "" || len(agentID) > 100 {
		http.Error(w, "Invalid agent ID", http.StatusBadRequest)
		return
	}

	taskList := h.queue.GetByAgent(agentID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"agent_id": agentID,
		"tasks":    taskList,
		"total":    len(taskList),
	})
}
