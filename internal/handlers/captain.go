package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/CLIAIMONITOR/internal/captain"
	"github.com/CLIAIMONITOR/internal/persistence"
	"github.com/CLIAIMONITOR/internal/types"
	"github.com/gorilla/mux"
)

// MaxPayloadSize defines the maximum size for request payloads (1MB)
// This prevents DoS attacks via large payloads
const MaxPayloadSize = 1 * 1024 * 1024 // 1MB

// Mission execution timeout constants
const (
	// MissionExecutionTimeout is the timeout for executing a single mission
	MissionExecutionTimeout = 10 * time.Minute
	// ParallelMissionsTimeout is the timeout for executing multiple missions in parallel
	ParallelMissionsTimeout = 30 * time.Minute
	// ReconExecutionTimeout is the timeout for reconnaissance missions
	ReconExecutionTimeout = 15 * time.Minute
)

// limitRequestSize limits the request body size to prevent DoS via large payloads
func limitRequestSize(r *http.Request, maxSize int64) {
	r.Body = http.MaxBytesReader(nil, r.Body, maxSize)
}

// CaptainHandler handles Captain orchestration endpoints
type CaptainHandler struct {
	captain *captain.Captain
	store   *persistence.JSONStore
}

// NewCaptainHandler creates a new captain handler
func NewCaptainHandler(cap *captain.Captain, store *persistence.JSONStore) *CaptainHandler {
	return &CaptainHandler{
		captain: cap,
		store:   store,
	}
}

// HandleDecideMode returns the recommended mode for a mission
func (h *CaptainHandler) HandleDecideMode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Limit request size to prevent DoS
	limitRequestSize(r, MaxPayloadSize)

	var mission captain.Mission
	if err := json.NewDecoder(r.Body).Decode(&mission); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	decision := h.captain.DecideMode(mission)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(decision)
}

// HandleExecuteMission executes a single mission
func (h *CaptainHandler) HandleExecuteMission(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Limit request size to prevent DoS
	limitRequestSize(r, MaxPayloadSize)

	var mission captain.Mission
	if err := json.NewDecoder(r.Body).Decode(&mission); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Use context with timeout for subagent execution
	ctx, cancel := context.WithTimeout(r.Context(), MissionExecutionTimeout)
	defer cancel()

	result, err := h.captain.ExecuteMission(ctx, mission)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// HandleExecuteParallel executes multiple missions in parallel
func (h *CaptainHandler) HandleExecuteParallel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Limit request size to prevent DoS
	limitRequestSize(r, MaxPayloadSize)

	var request struct {
		Missions []captain.Mission `json:"missions"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if len(request.Missions) == 0 {
		http.Error(w, "No missions provided", http.StatusBadRequest)
		return
	}

	// Use context with timeout
	ctx, cancel := context.WithTimeout(r.Context(), ParallelMissionsTimeout)
	defer cancel()

	results := h.captain.ExecuteMissionsParallel(ctx, request.Missions)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"results": results,
		"total":   len(results),
	})
}

// HandleImportTasks imports pending tasks and converts to missions
func (h *CaptainHandler) HandleImportTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	missions, err := h.captain.ImportPendingTasks()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Return missions with mode decisions
	type MissionWithDecision struct {
		Mission  captain.Mission      `json:"mission"`
		Decision captain.ModeDecision `json:"decision"`
	}

	var response []MissionWithDecision
	for _, m := range missions {
		response = append(response, MissionWithDecision{
			Mission:  m,
			Decision: h.captain.DecideMode(m),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"missions": response,
		"total":    len(response),
	})
}

// HandleActiveSubagents returns currently running subagents
func (h *CaptainHandler) HandleActiveSubagents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	active := h.captain.GetActiveSubagents()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"active": active,
		"count":  len(active),
	})
}

// HandleSetAPIKey sets the Planner API key
func (h *CaptainHandler) HandleSetAPIKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Limit request size to prevent DoS
	limitRequestSize(r, MaxPayloadSize)

	var request struct {
		APIKey string `json:"api_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if request.APIKey == "" {
		http.Error(w, "API key required", http.StatusBadRequest)
		return
	}

	h.captain.SetPlannerAPIKey(request.APIKey)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "API key set successfully",
	})
}

// HandleRecon creates and executes a reconnaissance mission
func (h *CaptainHandler) HandleRecon(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Limit request size to prevent DoS
	limitRequestSize(r, MaxPayloadSize)

	var request struct {
		ProjectPath string `json:"project_path"`
		Title       string `json:"title"`
		Focus       string `json:"focus"` // security, architecture, dependencies, etc.
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	mission := captain.Mission{
		ID:          "recon-" + time.Now().Format("20060102-150405"),
		Title:       request.Title,
		Description: buildReconDescription(request.Focus),
		TaskType:    captain.TaskRecon,
		ProjectPath: request.ProjectPath,
		Priority:    1,
		Metadata: map[string]string{
			"focus": request.Focus,
		},
	}

	ctx, cancel := context.WithTimeout(r.Context(), ReconExecutionTimeout)
	defer cancel()

	result, err := h.captain.ExecuteMission(ctx, mission)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// reconFocusDescriptions maps recon focus areas to detailed descriptions
var reconFocusDescriptions = map[string]string{
	"security":     "Focus on security vulnerabilities: OWASP Top 10, command injection, SQL injection, XSS, authentication/authorization issues, secrets in code, input validation.",
	"architecture": "Focus on architecture: project structure, design patterns, dependencies between packages, potential refactoring opportunities, code organization.",
	"dependencies": "Focus on dependency health: outdated packages, known vulnerabilities, unnecessary dependencies, version conflicts.",
	"testing":      "Focus on test coverage: identify untested code paths, missing test cases, test quality assessment.",
	"full":         "Perform full reconnaissance covering security, architecture, dependencies, code quality, and process evaluation. Provide comprehensive findings.",
}

// buildReconDescription creates a detailed recon prompt based on focus
func buildReconDescription(focus string) string {
	base := "Conduct reconnaissance on this codebase. "

	if desc, ok := reconFocusDescriptions[focus]; ok {
		return base + desc
	}
	return base + "Identify any issues, vulnerabilities, or improvement opportunities. Report findings by severity."
}

// SubmitTaskRequest is the payload for submitting a task to Captain
type SubmitTaskRequest struct {
	Title       string            `json:"title"`
	Description string            `json:"description"`
	ProjectPath string            `json:"project_path,omitempty"`
	Priority    int               `json:"priority,omitempty"`
	NeedsRecon  bool              `json:"needs_recon,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// SubmitTaskResponse is the response after submitting a task
type SubmitTaskResponse struct {
	TaskID  string `json:"task_id"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

// HandleSubmitTask submits a task to Captain
func (h *CaptainHandler) HandleSubmitTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Limit request size to prevent DoS
	limitRequestSize(r, MaxPayloadSize)

	var req SubmitTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.Title == "" {
		http.Error(w, "Title is required", http.StatusBadRequest)
		return
	}
	if req.Description == "" {
		http.Error(w, "Description is required", http.StatusBadRequest)
		return
	}

	// Infer task type based on title/description
	taskType := inferTaskTypeFromRequest(req.Title, req.Description, req.NeedsRecon)

	// Create mission
	mission := captain.Mission{
		ID:          fmt.Sprintf("task-%d", time.Now().UnixNano()),
		Title:       req.Title,
		Description: req.Description,
		TaskType:    taskType,
		ProjectPath: req.ProjectPath,
		Priority:    req.Priority,
		Metadata:    req.Metadata,
	}

	// Execute mission asynchronously
	ctx, cancel := context.WithTimeout(context.Background(), ParallelMissionsTimeout)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				// Log panic to activity log
				h.store.AddActivity(&types.ActivityLog{
					ID:        fmt.Sprintf("activity-%d", time.Now().UnixNano()),
					AgentID:   "Captain",
					Action:    "task_panic",
					Details:   fmt.Sprintf("Task %s panicked: %v", mission.ID, r),
					Timestamp: time.Now(),
				})
			}
		}()
		defer cancel() // Cancel when goroutine completes

		result, err := h.captain.ExecuteMission(ctx, mission)
		if err != nil {
			// Log error to activity log
			h.store.AddActivity(&types.ActivityLog{
				ID:        fmt.Sprintf("activity-%d", time.Now().UnixNano()),
				AgentID:   "Captain",
				Action:    "task_failed",
				Details:   fmt.Sprintf("Task %s failed: %v", mission.ID, err),
				Timestamp: time.Now(),
			})
			return
		}

		// Log successful completion
		h.store.AddActivity(&types.ActivityLog{
			ID:        fmt.Sprintf("activity-%d", time.Now().UnixNano()),
			AgentID:   result.AgentID,
			Action:    "task_completed",
			Details:   fmt.Sprintf("Task %s completed: %s", mission.ID, result.Status),
			Timestamp: time.Now(),
		})
	}()

	response := SubmitTaskResponse{
		TaskID:  mission.ID,
		Status:  "submitted",
		Message: "Task submitted successfully and will be executed asynchronously",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// CaptainStatusResponse represents Captain's current status
type CaptainStatusResponse struct {
	Running      bool      `json:"running"`
	LastCycle    time.Time `json:"last_cycle"`
	PendingTasks int       `json:"pending_tasks"`
	ActiveAgents int       `json:"active_agents"`
	Escalations  int       `json:"escalations"`
}

// HandleGetStatus returns Captain's current status
func (h *CaptainHandler) HandleGetStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get all agents from store
	state := h.store.GetState()

	// Count escalations (pending stop requests and human requests)
	escalations := len(h.store.GetPendingStopRequests()) + len(h.store.GetPendingRequests())

	response := CaptainStatusResponse{
		Running:      true,       // Captain is running if we can respond
		LastCycle:    time.Now(), // Would need to track this in Captain
		PendingTasks: 0,          // Would need Captain to expose this
		ActiveAgents: len(state.Agents),
		Escalations:  escalations,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ReconRequest is the payload for triggering manual reconnaissance
type ReconRequest struct {
	ProjectPath string `json:"project_path"`
	Mission     string `json:"mission,omitempty"`
}

// ReconResponse is the response after triggering recon
type ReconResponse struct {
	ReconID string `json:"recon_id"`
	Status  string `json:"status"`
}

// HandleTriggerRecon triggers manual reconnaissance
func (h *CaptainHandler) HandleTriggerRecon(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Limit request size to prevent DoS
	limitRequestSize(r, MaxPayloadSize)

	var req ReconRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.ProjectPath == "" {
		http.Error(w, "project_path is required", http.StatusBadRequest)
		return
	}

	// Use provided mission or default
	missionDesc := req.Mission
	if missionDesc == "" {
		missionDesc = buildReconDescription("full")
	}

	// Create recon mission
	reconID := fmt.Sprintf("recon-%d", time.Now().UnixNano())
	mission := captain.Mission{
		ID:          reconID,
		Title:       "Manual Reconnaissance",
		Description: missionDesc,
		TaskType:    captain.TaskRecon,
		ProjectPath: req.ProjectPath,
		Priority:    1,
		Metadata: map[string]string{
			"trigger": "manual",
		},
	}

	// Execute asynchronously
	ctx, cancel := context.WithTimeout(context.Background(), ReconExecutionTimeout)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				// Log panic to activity log
				h.store.AddActivity(&types.ActivityLog{
					ID:        fmt.Sprintf("activity-%d", time.Now().UnixNano()),
					AgentID:   "Captain",
					Action:    "recon_panic",
					Details:   fmt.Sprintf("Recon %s panicked: %v", reconID, r),
					Timestamp: time.Now(),
				})
			}
		}()
		defer cancel() // Cancel when goroutine completes

		result, err := h.captain.ExecuteMission(ctx, mission)
		if err != nil {
			h.store.AddActivity(&types.ActivityLog{
				ID:        fmt.Sprintf("activity-%d", time.Now().UnixNano()),
				AgentID:   "Captain",
				Action:    "recon_failed",
				Details:   fmt.Sprintf("Recon %s failed: %v", reconID, err),
				Timestamp: time.Now(),
			})
			return
		}

		h.store.AddActivity(&types.ActivityLog{
			ID:        fmt.Sprintf("activity-%d", time.Now().UnixNano()),
			AgentID:   result.AgentID,
			Action:    "recon_completed",
			Details:   fmt.Sprintf("Recon %s completed", reconID),
			Timestamp: time.Now(),
		})
	}()

	response := ReconResponse{
		ReconID: reconID,
		Status:  "started",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// Escalation represents a pending escalation
type Escalation struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"` // "stop_request" or "human_input"
	AgentID   string    `json:"agent_id"`
	Question  string    `json:"question"`
	Context   string    `json:"context"`
	CreatedAt time.Time `json:"created_at"`
}

// HandleGetEscalations lists pending escalations
func (h *CaptainHandler) HandleGetEscalations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var escalations []Escalation

	// Get stop requests
	stopRequests := h.store.GetPendingStopRequests()
	for _, sr := range stopRequests {
		escalations = append(escalations, Escalation{
			ID:        sr.ID,
			Type:      "stop_request",
			AgentID:   sr.AgentID,
			Question:  fmt.Sprintf("Agent requests to stop: %s", sr.Reason),
			Context:   fmt.Sprintf("Work completed: %s\nDetails: %s", sr.WorkCompleted, sr.Context),
			CreatedAt: sr.CreatedAt,
		})
	}

	// Get human input requests
	humanRequests := h.store.GetPendingRequests()
	for _, hr := range humanRequests {
		escalations = append(escalations, Escalation{
			ID:        hr.ID,
			Type:      "human_input",
			AgentID:   hr.AgentID,
			Question:  hr.Question,
			Context:   hr.Context,
			CreatedAt: hr.CreatedAt,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"escalations": escalations,
		"total":       len(escalations),
	})
}

// EscalationResponseRequest is the payload for responding to an escalation
type EscalationResponseRequest struct {
	Response string `json:"response"`
	Action   string `json:"action"` // "approve", "reject", "modify"
}

// HandleRespondToEscalation responds to a pending escalation
func (h *CaptainHandler) HandleRespondToEscalation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Limit request size to prevent DoS
	limitRequestSize(r, MaxPayloadSize)

	vars := mux.Vars(r)
	escalationID := vars["id"]
	if escalationID == "" {
		http.Error(w, "Escalation ID is required", http.StatusBadRequest)
		return
	}

	var req EscalationResponseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Response == "" {
		http.Error(w, "Response is required", http.StatusBadRequest)
		return
	}

	if req.Action != "approve" && req.Action != "reject" && req.Action != "modify" {
		http.Error(w, "Action must be 'approve', 'reject', or 'modify'", http.StatusBadRequest)
		return
	}

	// Try to find as stop request first
	stopRequests := h.store.GetPendingStopRequests()
	for _, sr := range stopRequests {
		if sr.ID == escalationID {
			approved := req.Action == "approve"
			h.store.RespondStopRequest(escalationID, approved, req.Response, "human")

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{
				"status":  "responded",
				"message": "Stop request responded to successfully",
			})
			return
		}
	}

	// Try human input request
	humanRequests := h.store.GetPendingRequests()
	for _, hr := range humanRequests {
		if hr.ID == escalationID {
			h.store.AnswerHumanRequest(escalationID, req.Response)

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{
				"status":  "responded",
				"message": "Human input request answered successfully",
			})
			return
		}
	}

	http.Error(w, "Escalation not found", http.StatusNotFound)
}

// inferTaskTypeFromRequest determines task type from request parameters
func inferTaskTypeFromRequest(title, description string, needsRecon bool) captain.TaskType {
	if needsRecon {
		return captain.TaskRecon
	}

	combined := strings.ToLower(title + " " + description)

	// Task type inference uses case-insensitive substring matching for best-effort categorization
	// While fragile, this method provides a flexible way to classify tasks dynamically
	// Current method uses keyword-based classification with a broad match strategy
	// Known limitations:
	// - Potential false positives due to keyword matching
	// - No ability to handle highly context-specific or nuanced task descriptions
	// Future improvements could include:
	// 1. Explicit task type field in requests
	// 2. More sophisticated NLP-based classification
	// 3. Machine learning based task type inference

	// Use similar logic to captain.inferTaskType
	if containsAny(combined, []string{"scan", "recon", "audit", "discover"}) {
		return captain.TaskRecon
	}
	if containsAny(combined, []string{"review", "analyze", "assess"}) {
		return captain.TaskAnalysis
	}
	if containsAny(combined, []string{"test", "coverage"}) {
		return captain.TaskTesting
	}
	if containsAny(combined, []string{"plan", "task", "api"}) {
		return captain.TaskPlanning
	}

	return captain.TaskImplementation
}

// containsAny checks if str contains any of the substrings (case-insensitive)
func containsAny(str string, substrings []string) bool {
	lowerStr := strings.ToLower(str)
	for _, sub := range substrings {
		if strings.Contains(lowerStr, strings.ToLower(sub)) {
			return true
		}
	}
	return false
}
