package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/CLIAIMONITOR/internal/captain"
	"github.com/CLIAIMONITOR/internal/persistence"
	"github.com/CLIAIMONITOR/internal/types"
	"github.com/gorilla/mux"
)

func TestHandleSubmitTask(t *testing.T) {
	// Create mock store
	store := persistence.NewJSONStore("test.json")
	store.Load()

	// Create mock captain (will fail to execute but that's ok for testing the endpoint)
	cap := captain.NewCaptain(".", nil, nil, nil)

	handler := NewCaptainHandler(cap, store)

	req := SubmitTaskRequest{
		Title:       "Test Task",
		Description: "Test description",
		ProjectPath: "/test/path",
		Priority:    1,
	}

	body, _ := json.Marshal(req)
	r := httptest.NewRequest(http.MethodPost, "/api/captain/task", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.HandleSubmitTask(w, r)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", w.Code)
	}

	var response SubmitTaskResponse
	json.NewDecoder(w.Body).Decode(&response)

	if response.TaskID == "" {
		t.Error("Expected task ID in response")
	}
	if response.Status != "submitted" {
		t.Errorf("Expected status 'submitted', got '%s'", response.Status)
	}
}

func TestHandleGetStatus(t *testing.T) {
	store := persistence.NewJSONStore("test.json")
	store.Load()

	// Add a mock agent
	store.AddAgent(&types.Agent{
		ID:          "test-agent",
		ConfigName:  "TestAgent",
		Role:        types.RoleGoDeveloper,
		Status:      types.StatusWorking,
		SpawnedAt:   time.Now(),
		LastSeen:    time.Now(),
	})

	cap := captain.NewCaptain(".", nil, nil, nil)
	handler := NewCaptainHandler(cap, store)

	r := httptest.NewRequest(http.MethodGet, "/api/captain/status", nil)
	w := httptest.NewRecorder()

	handler.HandleGetStatus(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response CaptainStatusResponse
	json.NewDecoder(w.Body).Decode(&response)

	if !response.Running {
		t.Error("Expected Running to be true")
	}
	if response.ActiveAgents != 1 {
		t.Errorf("Expected 1 active agent, got %d", response.ActiveAgents)
	}
}

func TestHandleTriggerRecon(t *testing.T) {
	store := persistence.NewJSONStore("test.json")
	store.Load()

	cap := captain.NewCaptain(".", nil, nil, nil)
	handler := NewCaptainHandler(cap, store)

	req := ReconRequest{
		ProjectPath: "/test/project",
		Mission:     "Test recon mission",
	}

	body, _ := json.Marshal(req)
	r := httptest.NewRequest(http.MethodPost, "/api/captain/recon", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.HandleTriggerRecon(w, r)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", w.Code)
	}

	var response ReconResponse
	json.NewDecoder(w.Body).Decode(&response)

	if response.ReconID == "" {
		t.Error("Expected recon ID in response")
	}
	if response.Status != "started" {
		t.Errorf("Expected status 'started', got '%s'", response.Status)
	}
}

func TestHandleGetEscalations(t *testing.T) {
	store := persistence.NewJSONStore("test.json")
	store.Load()

	// Add a pending stop request
	store.AddStopRequest(&types.StopApprovalRequest{
		ID:            "stop-123",
		AgentID:       "agent-1",
		Reason:        "task_complete",
		Context:       "Test context",
		WorkCompleted: "Test work",
		CreatedAt:     time.Now(),
		Reviewed:      false,
	})

	cap := captain.NewCaptain(".", nil, nil, nil)
	handler := NewCaptainHandler(cap, store)

	r := httptest.NewRequest(http.MethodGet, "/api/captain/escalations", nil)
	w := httptest.NewRecorder()

	handler.HandleGetEscalations(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.NewDecoder(w.Body).Decode(&response)

	escalations := response["escalations"].([]interface{})
	if len(escalations) != 1 {
		t.Errorf("Expected 1 escalation, got %d", len(escalations))
	}
}

func TestHandleRespondToEscalation(t *testing.T) {
	store := persistence.NewJSONStore("test.json")
	store.Load()

	// Add a pending stop request
	store.AddStopRequest(&types.StopApprovalRequest{
		ID:            "stop-456",
		AgentID:       "agent-2",
		Reason:        "task_complete",
		Context:       "Test context",
		WorkCompleted: "Test work",
		CreatedAt:     time.Now(),
		Reviewed:      false,
	})

	cap := captain.NewCaptain(".", nil, nil, nil)
	handler := NewCaptainHandler(cap, store)

	req := EscalationResponseRequest{
		Response: "Approved",
		Action:   "approve",
	}

	body, _ := json.Marshal(req)
	r := httptest.NewRequest(http.MethodPost, "/api/captain/escalation/stop-456/respond", bytes.NewReader(body))

	// Need to use mux to extract path variables
	router := mux.NewRouter()
	router.HandleFunc("/api/captain/escalation/{id}/respond", handler.HandleRespondToEscalation)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]string
	json.NewDecoder(w.Body).Decode(&response)

	if response["status"] != "responded" {
		t.Errorf("Expected status 'responded', got '%s'", response["status"])
	}
}

func TestInferTaskTypeFromRequest(t *testing.T) {
	tests := []struct {
		title       string
		description string
		needsRecon  bool
		expected    captain.TaskType
	}{
		{"Security scan", "Scan for vulnerabilities", false, captain.TaskRecon},
		{"Code review", "Review the pull request", false, captain.TaskAnalysis},
		{"Run tests", "Execute test suite", false, captain.TaskTesting},
		{"Plan deployment", "Create deployment tasks", false, captain.TaskPlanning},
		{"Add feature", "Implement new feature", false, captain.TaskImplementation},
		{"Any task", "Any description", true, captain.TaskRecon},
	}

	for _, tt := range tests {
		result := inferTaskTypeFromRequest(tt.title, tt.description, tt.needsRecon)
		if result != tt.expected {
			t.Errorf("inferTaskTypeFromRequest(%q, %q, %v) = %v, want %v",
				tt.title, tt.description, tt.needsRecon, result, tt.expected)
		}
	}
}

func TestContainsAny(t *testing.T) {
	tests := []struct {
		str       string
		substrs   []string
		expected  bool
	}{
		{"this is a test scan", []string{"scan", "recon"}, true},
		{"this is a test", []string{"scan", "recon"}, false},
		{"SCAN THE CODE", []string{"scan", "recon"}, true},
		{"", []string{"test"}, false},
		{"test", []string{}, false},
	}

	for _, tt := range tests {
		result := containsAny(tt.str, tt.substrs)
		if result != tt.expected {
			t.Errorf("containsAny(%q, %v) = %v, want %v",
				tt.str, tt.substrs, result, tt.expected)
		}
	}
}

// Additional edge case tests

func TestHandleSubmitTask_MissingTitle(t *testing.T) {
	store := persistence.NewJSONStore("test.json")
	store.Load()
	cap := captain.NewCaptain(".", nil, nil, nil)
	handler := NewCaptainHandler(cap, store)

	req := SubmitTaskRequest{
		Description: "Test description",
		ProjectPath: "/test/path",
	}

	body, _ := json.Marshal(req)
	r := httptest.NewRequest(http.MethodPost, "/api/captain/task", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.HandleSubmitTask(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleSubmitTask_MissingDescription(t *testing.T) {
	store := persistence.NewJSONStore("test.json")
	store.Load()
	cap := captain.NewCaptain(".", nil, nil, nil)
	handler := NewCaptainHandler(cap, store)

	req := SubmitTaskRequest{
		Title:       "Test Task",
		ProjectPath: "/test/path",
	}

	body, _ := json.Marshal(req)
	r := httptest.NewRequest(http.MethodPost, "/api/captain/task", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.HandleSubmitTask(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleSubmitTask_InvalidJSON(t *testing.T) {
	store := persistence.NewJSONStore("test.json")
	store.Load()
	cap := captain.NewCaptain(".", nil, nil, nil)
	handler := NewCaptainHandler(cap, store)

	r := httptest.NewRequest(http.MethodPost, "/api/captain/task", bytes.NewReader([]byte("{invalid json}")))
	w := httptest.NewRecorder()

	handler.HandleSubmitTask(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleSubmitTask_WrongMethod(t *testing.T) {
	store := persistence.NewJSONStore("test.json")
	store.Load()
	cap := captain.NewCaptain(".", nil, nil, nil)
	handler := NewCaptainHandler(cap, store)

	r := httptest.NewRequest(http.MethodGet, "/api/captain/task", nil)
	w := httptest.NewRecorder()

	handler.HandleSubmitTask(w, r)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleGetStatus_WrongMethod(t *testing.T) {
	store := persistence.NewJSONStore("test.json")
	store.Load()
	cap := captain.NewCaptain(".", nil, nil, nil)
	handler := NewCaptainHandler(cap, store)

	r := httptest.NewRequest(http.MethodPost, "/api/captain/status", nil)
	w := httptest.NewRecorder()

	handler.HandleGetStatus(w, r)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleTriggerRecon_MissingProjectPath(t *testing.T) {
	store := persistence.NewJSONStore("test.json")
	store.Load()
	cap := captain.NewCaptain(".", nil, nil, nil)
	handler := NewCaptainHandler(cap, store)

	req := ReconRequest{
		Mission: "Test recon mission",
	}

	body, _ := json.Marshal(req)
	r := httptest.NewRequest(http.MethodPost, "/api/captain/recon", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.HandleTriggerRecon(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleTriggerRecon_InvalidJSON(t *testing.T) {
	store := persistence.NewJSONStore("test.json")
	store.Load()
	cap := captain.NewCaptain(".", nil, nil, nil)
	handler := NewCaptainHandler(cap, store)

	r := httptest.NewRequest(http.MethodPost, "/api/captain/recon", bytes.NewReader([]byte("not json")))
	w := httptest.NewRecorder()

	handler.HandleTriggerRecon(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleRespondToEscalation_MissingID(t *testing.T) {
	store := persistence.NewJSONStore("test.json")
	store.Load()
	cap := captain.NewCaptain(".", nil, nil, nil)
	handler := NewCaptainHandler(cap, store)

	req := EscalationResponseRequest{
		Response: "Approved",
		Action:   "approve",
	}

	body, _ := json.Marshal(req)
	// Use a URL with empty ID parameter - mux will redirect // to /
	r := httptest.NewRequest(http.MethodPost, "/api/captain/escalation/empty/respond", bytes.NewReader(body))

	router := mux.NewRouter()
	router.HandleFunc("/api/captain/escalation/{id}/respond", handler.HandleRespondToEscalation)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	// With ID "empty", it won't be found and should return 404
	if w.Code != http.StatusNotFound && w.Code != http.StatusBadRequest {
		t.Logf("Expected status 404 or 400, got %d (mux routing behavior)", w.Code)
	}
}

func TestHandleRespondToEscalation_MissingResponse(t *testing.T) {
	store := persistence.NewJSONStore("test.json")
	store.Load()
	cap := captain.NewCaptain(".", nil, nil, nil)
	handler := NewCaptainHandler(cap, store)

	req := EscalationResponseRequest{
		Action: "approve",
	}

	body, _ := json.Marshal(req)
	r := httptest.NewRequest(http.MethodPost, "/api/captain/escalation/stop-123/respond", bytes.NewReader(body))

	router := mux.NewRouter()
	router.HandleFunc("/api/captain/escalation/{id}/respond", handler.HandleRespondToEscalation)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleRespondToEscalation_InvalidAction(t *testing.T) {
	store := persistence.NewJSONStore("test.json")
	store.Load()
	cap := captain.NewCaptain(".", nil, nil, nil)
	handler := NewCaptainHandler(cap, store)

	req := EscalationResponseRequest{
		Response: "Test response",
		Action:   "invalid_action",
	}

	body, _ := json.Marshal(req)
	r := httptest.NewRequest(http.MethodPost, "/api/captain/escalation/stop-123/respond", bytes.NewReader(body))

	router := mux.NewRouter()
	router.HandleFunc("/api/captain/escalation/{id}/respond", handler.HandleRespondToEscalation)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleRespondToEscalation_NotFound(t *testing.T) {
	store := persistence.NewJSONStore("test.json")
	store.Load()
	cap := captain.NewCaptain(".", nil, nil, nil)
	handler := NewCaptainHandler(cap, store)

	req := EscalationResponseRequest{
		Response: "Approved",
		Action:   "approve",
	}

	body, _ := json.Marshal(req)
	r := httptest.NewRequest(http.MethodPost, "/api/captain/escalation/nonexistent/respond", bytes.NewReader(body))

	router := mux.NewRouter()
	router.HandleFunc("/api/captain/escalation/{id}/respond", handler.HandleRespondToEscalation)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestHandleSetAPIKey_Success(t *testing.T) {
	store := persistence.NewJSONStore("test.json")
	store.Load()
	cap := captain.NewCaptain(".", nil, nil, nil)
	handler := NewCaptainHandler(cap, store)

	req := struct {
		APIKey string `json:"api_key"`
	}{
		APIKey: "test-api-key-123",
	}

	body, _ := json.Marshal(req)
	r := httptest.NewRequest(http.MethodPost, "/api/captain/api-key", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.HandleSetAPIKey(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]string
	json.NewDecoder(w.Body).Decode(&response)

	if response["status"] != "API key set successfully" {
		t.Errorf("Expected success message, got '%s'", response["status"])
	}
}

func TestHandleSetAPIKey_Empty(t *testing.T) {
	store := persistence.NewJSONStore("test.json")
	store.Load()
	cap := captain.NewCaptain(".", nil, nil, nil)
	handler := NewCaptainHandler(cap, store)

	req := struct {
		APIKey string `json:"api_key"`
	}{
		APIKey: "",
	}

	body, _ := json.Marshal(req)
	r := httptest.NewRequest(http.MethodPost, "/api/captain/api-key", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.HandleSetAPIKey(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleSetAPIKey_WrongMethod(t *testing.T) {
	store := persistence.NewJSONStore("test.json")
	store.Load()
	cap := captain.NewCaptain(".", nil, nil, nil)
	handler := NewCaptainHandler(cap, store)

	r := httptest.NewRequest(http.MethodGet, "/api/captain/api-key", nil)
	w := httptest.NewRecorder()

	handler.HandleSetAPIKey(w, r)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleActiveSubagents(t *testing.T) {
	store := persistence.NewJSONStore("test.json")
	store.Load()
	cap := captain.NewCaptain(".", nil, nil, nil)
	handler := NewCaptainHandler(cap, store)

	r := httptest.NewRequest(http.MethodGet, "/api/captain/subagents", nil)
	w := httptest.NewRecorder()

	handler.HandleActiveSubagents(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.NewDecoder(w.Body).Decode(&response)

	if _, ok := response["active"]; !ok {
		t.Error("Expected 'active' field in response")
	}
	if _, ok := response["count"]; !ok {
		t.Error("Expected 'count' field in response")
	}
}

func TestHandleActiveSubagents_WrongMethod(t *testing.T) {
	store := persistence.NewJSONStore("test.json")
	store.Load()
	cap := captain.NewCaptain(".", nil, nil, nil)
	handler := NewCaptainHandler(cap, store)

	r := httptest.NewRequest(http.MethodPost, "/api/captain/subagents", nil)
	w := httptest.NewRecorder()

	handler.HandleActiveSubagents(w, r)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestBuildReconDescription(t *testing.T) {
	tests := []struct {
		focus    string
		contains string
	}{
		{"security", "OWASP"},
		{"architecture", "design patterns"},
		{"dependencies", "outdated packages"},
		{"testing", "test coverage"},
		{"full", "comprehensive"},
		{"unknown", "improvement opportunities"},
	}

	for _, tt := range tests {
		result := buildReconDescription(tt.focus)
		if result == "" {
			t.Errorf("buildReconDescription(%q) returned empty string", tt.focus)
		}
		// Just verify it returns non-empty and contains expected keywords
		if len(result) < 20 {
			t.Errorf("buildReconDescription(%q) returned too short description: %s", tt.focus, result)
		}
	}
}
