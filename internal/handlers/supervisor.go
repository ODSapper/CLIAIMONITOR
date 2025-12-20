package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/CLIAIMONITOR/internal/agents"
	"github.com/CLIAIMONITOR/internal/memory"
	"github.com/CLIAIMONITOR/internal/supervisor"
	"github.com/CLIAIMONITOR/internal/types"
	"github.com/gorilla/mux"
)

const (
	DefaultLimit = 100
	MaxLimit     = 1000
)

// Package-level valid statuses maps (created once for performance)
var validTaskStatuses = map[string]bool{
	"pending":     true,
	"assigned":    true,
	"in_progress": true,
	"completed":   true,
	"blocked":     true,
	"cancelled":   true,
}

var validDeploymentStatuses = map[string]bool{
	"proposed":  true,
	"approved":  true,
	"executing": true,
	"completed": true,
	"failed":    true,
	"cancelled": true,
}

// SupervisorHandler handles supervisor-related HTTP endpoints
type SupervisorHandler struct {
	memDB    memory.MemoryDB
	scanner  *supervisor.Scanner
	planner  *supervisor.Planner
	executor *supervisor.Executor
}

// NewSupervisorHandler creates a new supervisor handler
func NewSupervisorHandler(memDB memory.MemoryDB) *SupervisorHandler {
	return &SupervisorHandler{
		memDB:   memDB,
		scanner: supervisor.NewScanner(memDB),
		planner: supervisor.NewPlanner(memDB),
	}
}

// NewSupervisorHandlerWithExecutor creates a handler with executor capabilities
func NewSupervisorHandlerWithExecutor(memDB memory.MemoryDB, spawner *agents.ProcessSpawner, configs map[string]types.AgentConfig) *SupervisorHandler {
	return &SupervisorHandler{
		memDB:    memDB,
		scanner:  supervisor.NewScanner(memDB),
		planner:  supervisor.NewPlanner(memDB),
		executor: supervisor.NewExecutor(memDB, spawner, configs),
	}
}

// RegisterRoutes registers supervisor API routes on the given router
func (h *SupervisorHandler) RegisterRoutes(r *mux.Router) {
	// Supervisor API routes
	r.HandleFunc("/supervisor/repos", h.handleDiscoverRepo).Methods("POST")
	r.HandleFunc("/supervisor/repos/{id}", h.handleGetRepo).Methods("GET")
	r.HandleFunc("/supervisor/repos/{id}/scan", h.handleScanRepo).Methods("POST")
	r.HandleFunc("/supervisor/repos/{id}/plan", h.handleCreatePlan).Methods("POST")
	r.HandleFunc("/supervisor/tasks", h.handleGetTasks).Methods("GET")
	r.HandleFunc("/supervisor/tasks/{id}", h.handleGetTask).Methods("GET")
	r.HandleFunc("/supervisor/tasks/{id}/status", h.handleUpdateTaskStatus).Methods("PUT")
	r.HandleFunc("/supervisor/deployments", h.handleGetDeployments).Methods("GET")
	r.HandleFunc("/supervisor/deployments/{id}", h.handleGetDeployment).Methods("GET")
	r.HandleFunc("/supervisor/deployments/{id}/status", h.handleUpdateDeploymentStatus).Methods("PUT")
	r.HandleFunc("/supervisor/deployments/{id}/execute", h.handleExecuteDeployment).Methods("POST")
}

// handleDiscoverRepo discovers a new repository
func (h *SupervisorHandler) handleDiscoverRepo(w http.ResponseWriter, r *http.Request) {
	// Limit request size to prevent DoS
	limitRequestSize(r, MaxPayloadSize)

	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Path == "" {
		respondError(w, http.StatusBadRequest, "Path is required")
		return
	}

	repo, err := h.memDB.DiscoverRepo(req.Path)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, repo)
}

// handleGetRepo retrieves a repository by ID
func (h *SupervisorHandler) handleGetRepo(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	repoID := vars["id"]

	repo, err := h.memDB.GetRepo(repoID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Repository not found")
		return
	}

	respondJSON(w, repo)
}

// handleScanRepo scans a repository for workflow files
func (h *SupervisorHandler) handleScanRepo(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	repoID := vars["id"]

	// Verify repo exists
	_, err := h.memDB.GetRepo(repoID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Repository not found")
		return
	}

	// Perform scan
	result, err := h.scanner.ScanForWorkflows(repoID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Build response
	response := map[string]interface{}{
		"repo_id":          result.RepoID,
		"claude_md_found":  result.CLAUDEmd != nil,
		"workflow_files":   len(result.WorkflowFiles),
		"plan_files":       len(result.PlanFiles),
		"tasks_discovered": len(result.DiscoveredTasks),
	}

	if result.CLAUDEmd != nil {
		response["claude_md_sections"] = len(result.CLAUDEmd.KeySections)
	}

	respondJSON(w, response)
}

// handleCreatePlan generates a deployment plan for a repository
func (h *SupervisorHandler) handleCreatePlan(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	repoID := vars["id"]

	// Verify repo exists
	_, err := h.memDB.GetRepo(repoID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Repository not found")
		return
	}

	// Create deployment plan
	plan, err := h.planner.CreateDeploymentPlan(repoID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Store the plan
	deploymentID, err := h.planner.StoreDeploymentPlan(plan)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := map[string]interface{}{
		"deployment_id": deploymentID,
		"plan":          plan,
	}

	respondJSON(w, response)
}

// handleGetTasks retrieves workflow tasks with optional filters
func (h *SupervisorHandler) handleGetTasks(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters for filtering
	query := r.URL.Query()

	limit := DefaultLimit
	if l := query.Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= MaxLimit {
			limit = parsed
		}
	}

	offset := 0
	if o := query.Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	filter := memory.TaskFilter{
		RepoID:          query.Get("repo_id"),
		Status:          query.Get("status"),
		AssignedAgentID: query.Get("agent_id"),
		Priority:        query.Get("priority"),
		ParentTaskID:    query.Get("parent_id"),
		Limit:           limit,
		Offset:          offset,
	}

	tasks, err := h.memDB.GetTasks(filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, map[string]interface{}{
		"tasks":  tasks,
		"count":  len(tasks),
		"limit":  limit,
		"offset": offset,
	})
}

// handleGetTask retrieves a single task by ID
func (h *SupervisorHandler) handleGetTask(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	taskID := vars["id"]

	task, err := h.memDB.GetTask(taskID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Task not found")
		return
	}

	respondJSON(w, task)
}

// handleUpdateTaskStatus updates the status of a task
func (h *SupervisorHandler) handleUpdateTaskStatus(w http.ResponseWriter, r *http.Request) {
	// Limit request size to prevent DoS
	limitRequestSize(r, MaxPayloadSize)

	vars := mux.Vars(r)
	taskID := vars["id"]

	var req struct {
		Status  string `json:"status"`
		AgentID string `json:"agent_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Status == "" {
		respondError(w, http.StatusBadRequest, "Status is required")
		return
	}

	// Validate status
	if !validTaskStatuses[req.Status] {
		respondError(w, http.StatusBadRequest, "Invalid status value")
		return
	}

	if err := h.memDB.UpdateTaskStatus(taskID, req.Status, req.AgentID); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, map[string]string{
		"status": "updated",
	})
}

// handleGetDeployments retrieves deployment plans
func (h *SupervisorHandler) handleGetDeployments(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	repoID := query.Get("repo_id")

	// Get deployments with limit
	limit := 50 // Default limit
	deployments, err := h.memDB.GetRecentDeployments(repoID, limit)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Filter by status if provided
	status := query.Get("status")
	if status != "" {
		filtered := make([]*memory.Deployment, 0)
		for _, d := range deployments {
			if d.Status == status {
				filtered = append(filtered, d)
			}
		}
		deployments = filtered
	}

	respondJSON(w, map[string]interface{}{
		"deployments": deployments,
		"count":       len(deployments),
	})
}

// handleGetDeployment retrieves a single deployment by ID
func (h *SupervisorHandler) handleGetDeployment(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid deployment ID")
		return
	}

	deployment, err := h.memDB.GetDeployment(id)
	if err != nil {
		respondError(w, http.StatusNotFound, "Deployment not found")
		return
	}

	respondJSON(w, deployment)
}

// handleUpdateDeploymentStatus updates the status of a deployment
func (h *SupervisorHandler) handleUpdateDeploymentStatus(w http.ResponseWriter, r *http.Request) {
	// Limit request size to prevent DoS
	limitRequestSize(r, MaxPayloadSize)

	vars := mux.Vars(r)
	idStr := vars["id"]

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid deployment ID")
		return
	}

	var req struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Status == "" {
		respondError(w, http.StatusBadRequest, "Status is required")
		return
	}

	// Validate status
	if !validDeploymentStatuses[req.Status] {
		respondError(w, http.StatusBadRequest, "Invalid status value")
		return
	}

	if err := h.memDB.UpdateDeploymentStatus(id, req.Status); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, map[string]string{
		"status": "updated",
	})
}

// handleExecuteDeployment executes a deployment plan by spawning agents
func (h *SupervisorHandler) handleExecuteDeployment(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid deployment ID")
		return
	}

	// Check if executor is available
	if h.executor == nil {
		respondError(w, http.StatusServiceUnavailable, "Executor not configured - handler created without spawner")
		return
	}

	result, err := h.executor.ExecutePlan(id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, result)
}

// Helper functions

func respondJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
