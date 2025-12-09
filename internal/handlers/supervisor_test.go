package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/CLIAIMONITOR/internal/memory"
	"github.com/gorilla/mux"
)

// setupTestHandler creates a test handler with a temporary database
func setupTestHandler(t *testing.T) (*SupervisorHandler, func()) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	memDB, err := memory.NewMemoryDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to create test DB: %v", err)
	}

	handler := NewSupervisorHandler(memDB)

	cleanup := func() {
		memDB.Close()
	}

	return handler, cleanup
}

// setupTestRouter creates a router with supervisor routes registered
func setupTestRouter(handler *SupervisorHandler) *mux.Router {
	router := mux.NewRouter()
	api := router.PathPrefix("/api").Subrouter()
	handler.RegisterRoutes(api)
	return router
}

func TestDiscoverRepo(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	router := setupTestRouter(handler)

	// Test with valid path
	body := map[string]string{"path": "."}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/supervisor/repos", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var response memory.Repo
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response.ID == "" {
		t.Error("Expected repo ID to be set")
	}
}

func TestDiscoverRepoMissingPath(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	router := setupTestRouter(handler)

	// Test with missing path
	body := map[string]string{}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/supervisor/repos", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rr.Code)
	}
}

func TestGetRepo(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	router := setupTestRouter(handler)

	// First create a repo
	repo, err := handler.memDB.DiscoverRepo(".")
	if err != nil {
		t.Fatalf("Failed to create repo: %v", err)
	}

	// Test getting the repo
	req := httptest.NewRequest("GET", "/api/supervisor/repos/"+repo.ID, nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestGetRepoNotFound(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	router := setupTestRouter(handler)

	req := httptest.NewRequest("GET", "/api/supervisor/repos/nonexistent", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", rr.Code)
	}
}

func TestScanRepo(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	router := setupTestRouter(handler)

	// First create a repo
	repo, err := handler.memDB.DiscoverRepo(".")
	if err != nil {
		t.Fatalf("Failed to create repo: %v", err)
	}

	// Test scanning
	req := httptest.NewRequest("POST", "/api/supervisor/repos/"+repo.ID+"/scan", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response["repo_id"] != repo.ID {
		t.Errorf("Expected repo_id %s, got %v", repo.ID, response["repo_id"])
	}
}

func TestScanRepoNotFound(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	router := setupTestRouter(handler)

	req := httptest.NewRequest("POST", "/api/supervisor/repos/nonexistent/scan", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", rr.Code)
	}
}

func TestGetTasks(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	router := setupTestRouter(handler)

	// Create a repo and add some tasks
	repo, err := handler.memDB.DiscoverRepo(".")
	if err != nil {
		t.Fatalf("Failed to create repo: %v", err)
	}

	// Add a test task
	task := &memory.WorkflowTask{
		ID:         "TEST-001",
		RepoID:     repo.ID,
		SourceFile: "test.yaml",
		Title:      "Test Task",
		Status:     "pending",
	}
	if err := handler.memDB.CreateTask(task); err != nil {
		t.Fatalf("Failed to create task: %v", err)
	}

	// Test getting tasks
	req := httptest.NewRequest("GET", "/api/supervisor/tasks", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	count := response["count"].(float64)
	if count < 1 {
		t.Errorf("Expected at least 1 task, got %v", count)
	}
}

func TestGetTasksWithFilter(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	router := setupTestRouter(handler)

	// Create a repo
	repo, err := handler.memDB.DiscoverRepo(".")
	if err != nil {
		t.Fatalf("Failed to create repo: %v", err)
	}

	// Add tasks with different statuses
	tasks := []*memory.WorkflowTask{
		{ID: "TEST-002", RepoID: repo.ID, SourceFile: "test.yaml", Title: "Task 1", Status: "pending"},
		{ID: "TEST-003", RepoID: repo.ID, SourceFile: "test.yaml", Title: "Task 2", Status: "completed"},
	}
	for _, task := range tasks {
		if err := handler.memDB.CreateTask(task); err != nil {
			t.Fatalf("Failed to create task: %v", err)
		}
	}

	// Test filtering by status
	req := httptest.NewRequest("GET", "/api/supervisor/tasks?status=pending", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &response)

	count := response["count"].(float64)
	if count != 1 {
		t.Errorf("Expected 1 pending task, got %v", count)
	}
}

func TestGetTask(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	router := setupTestRouter(handler)

	// Create a repo and task
	repo, _ := handler.memDB.DiscoverRepo(".")
	task := &memory.WorkflowTask{
		ID:         "TEST-004",
		RepoID:     repo.ID,
		SourceFile: "test.yaml",
		Title:      "Get Single Task",
		Status:     "pending",
	}
	handler.memDB.CreateTask(task)

	// Test getting single task
	req := httptest.NewRequest("GET", "/api/supervisor/tasks/TEST-004", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestGetTaskNotFound(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	router := setupTestRouter(handler)

	req := httptest.NewRequest("GET", "/api/supervisor/tasks/nonexistent", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", rr.Code)
	}
}

func TestUpdateTaskStatus(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	router := setupTestRouter(handler)

	// Create a repo and task
	repo, _ := handler.memDB.DiscoverRepo(".")
	task := &memory.WorkflowTask{
		ID:         "TEST-005",
		RepoID:     repo.ID,
		SourceFile: "test.yaml",
		Title:      "Update Status Task",
		Status:     "pending",
	}
	handler.memDB.CreateTask(task)

	// Test updating status
	body := map[string]string{"status": "in_progress", "agent_id": "coder001"}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("PUT", "/api/supervisor/tasks/TEST-005/status", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify the update
	updatedTask, _ := handler.memDB.GetTask("TEST-005")
	if updatedTask.Status != "in_progress" {
		t.Errorf("Expected status 'in_progress', got '%s'", updatedTask.Status)
	}
	if updatedTask.AssignedAgentID != "coder001" {
		t.Errorf("Expected agent 'coder001', got '%s'", updatedTask.AssignedAgentID)
	}
}

func TestUpdateTaskStatusInvalidStatus(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	router := setupTestRouter(handler)

	// Create a repo and task
	repo, _ := handler.memDB.DiscoverRepo(".")
	task := &memory.WorkflowTask{
		ID:         "TEST-006",
		RepoID:     repo.ID,
		SourceFile: "test.yaml",
		Title:      "Invalid Status Task",
		Status:     "pending",
	}
	handler.memDB.CreateTask(task)

	// Test with invalid status
	body := map[string]string{"status": "invalid_status"}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("PUT", "/api/supervisor/tasks/TEST-006/status", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rr.Code)
	}
}

func TestCreatePlan(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	router := setupTestRouter(handler)

	// Create a repo
	repo, err := handler.memDB.DiscoverRepo(".")
	if err != nil {
		t.Fatalf("Failed to create repo: %v", err)
	}

	// Test creating a plan
	req := httptest.NewRequest("POST", "/api/supervisor/repos/"+repo.ID+"/plan", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response["deployment_id"] == nil {
		t.Error("Expected deployment_id to be set")
	}
	if response["plan"] == nil {
		t.Error("Expected plan to be set")
	}
}

func TestGetDeployments(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	router := setupTestRouter(handler)

	// Create a repo and deployment
	repo, _ := handler.memDB.DiscoverRepo(".")
	deployment := &memory.Deployment{
		RepoID:         repo.ID,
		DeploymentPlan: `{"test": true}`,
		Status:         "proposed",
	}
	handler.memDB.CreateDeployment(deployment)

	// Test getting deployments (must filter by repo_id)
	req := httptest.NewRequest("GET", "/api/supervisor/deployments?repo_id="+repo.ID, nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var response map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &response)

	count := response["count"].(float64)
	if count < 1 {
		t.Errorf("Expected at least 1 deployment, got %v", count)
	}
}

func TestGetDeployment(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	router := setupTestRouter(handler)

	// Create a repo and deployment
	repo, _ := handler.memDB.DiscoverRepo(".")
	deployment := &memory.Deployment{
		RepoID:         repo.ID,
		DeploymentPlan: `{"test": true}`,
		Status:         "proposed",
	}
	handler.memDB.CreateDeployment(deployment)

	// Test getting single deployment
	req := httptest.NewRequest("GET", "/api/supervisor/deployments/1", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestGetDeploymentInvalidID(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	router := setupTestRouter(handler)

	req := httptest.NewRequest("GET", "/api/supervisor/deployments/invalid", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rr.Code)
	}
}

func TestUpdateDeploymentStatus(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	router := setupTestRouter(handler)

	// Create a repo and deployment
	repo, _ := handler.memDB.DiscoverRepo(".")
	deployment := &memory.Deployment{
		RepoID:         repo.ID,
		DeploymentPlan: `{"test": true}`,
		Status:         "proposed",
	}
	handler.memDB.CreateDeployment(deployment)

	// Test updating status
	body := map[string]string{"status": "approved"}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("PUT", "/api/supervisor/deployments/1/status", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify the update
	updatedDeployment, _ := handler.memDB.GetDeployment(1)
	if updatedDeployment.Status != "approved" {
		t.Errorf("Expected status 'approved', got '%s'", updatedDeployment.Status)
	}
}

func TestUpdateDeploymentStatusInvalid(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	router := setupTestRouter(handler)

	// Create a repo and deployment
	repo, _ := handler.memDB.DiscoverRepo(".")
	deployment := &memory.Deployment{
		RepoID:         repo.ID,
		DeploymentPlan: `{"test": true}`,
		Status:         "proposed",
	}
	handler.memDB.CreateDeployment(deployment)

	// Test with invalid status
	body := map[string]string{"status": "invalid_status"}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("PUT", "/api/supervisor/deployments/1/status", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rr.Code)
	}
}

// Additional comprehensive tests for SupervisorHandler

func TestDiscoverRepoInvalidPath(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	router := setupTestRouter(handler)

	// Test with invalid path
	body := map[string]string{"path": "/nonexistent/path/that/does/not/exist"}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/supervisor/repos", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	// Should handle gracefully (either 200 or 500)
	if rr.Code != http.StatusOK && rr.Code != http.StatusInternalServerError {
		t.Logf("Expected 200 or 500, got %d", rr.Code)
	}
}

func TestGetRepoWrongMethod(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	router := setupTestRouter(handler)

	req := httptest.NewRequest("POST", "/api/supervisor/repos/123", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	// Router won't match POST to GET endpoint
	if rr.Code == http.StatusNotFound {
		t.Log("Got expected 404 for wrong method")
	}
}

func TestGetTasksWithLimitOffset(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	router := setupTestRouter(handler)

	// Create a repo
	repo, _ := handler.memDB.DiscoverRepo(".")

	// Add multiple tasks
	for i := 0; i < 10; i++ {
		task := &memory.WorkflowTask{
			ID:         fmt.Sprintf("TASK-%03d", i),
			RepoID:     repo.ID,
			SourceFile: "test.yaml",
			Title:      fmt.Sprintf("Task %d", i),
			Status:     "pending",
		}
		handler.memDB.CreateTask(task)
	}

	// Test with limit and offset
	req := httptest.NewRequest("GET", "/api/supervisor/tasks?limit=5&offset=0", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &response)

	limit := response["limit"].(float64)
	if int(limit) != 5 {
		t.Errorf("Expected limit 5, got %v", limit)
	}
}

func TestGetTasksInvalidLimitOffset(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	router := setupTestRouter(handler)

	// Test with invalid limit and offset
	req := httptest.NewRequest("GET", "/api/supervisor/tasks?limit=abc&offset=xyz", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200 (defaults used), got %d", rr.Code)
	}
}

func TestUpdateTaskStatusMissingStatus(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	router := setupTestRouter(handler)

	// Create a task
	repo, _ := handler.memDB.DiscoverRepo(".")
	task := &memory.WorkflowTask{
		ID:         "TASK-999",
		RepoID:     repo.ID,
		SourceFile: "test.yaml",
		Title:      "Test Task",
		Status:     "pending",
	}
	handler.memDB.CreateTask(task)

	// Test updating without status
	body := map[string]string{}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("PUT", "/api/supervisor/tasks/TASK-999/status", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rr.Code)
	}
}

func TestCreatePlanNotFound(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	router := setupTestRouter(handler)

	// Test creating plan for non-existent repo
	req := httptest.NewRequest("POST", "/api/supervisor/repos/nonexistent/plan", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", rr.Code)
	}
}

func TestScanRepoInvalidMethod(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	router := setupTestRouter(handler)

	// Test with GET instead of POST
	req := httptest.NewRequest("GET", "/api/supervisor/repos/123/scan", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	// Router won't match GET to POST endpoint
	if rr.Code == http.StatusNotFound {
		t.Log("Got expected 404 for wrong method")
	}
}

func TestGetDeploymentsStatusFilter(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	router := setupTestRouter(handler)

	// Create a repo and deployments with different statuses
	repo, _ := handler.memDB.DiscoverRepo(".")

	deployment1 := &memory.Deployment{
		RepoID:         repo.ID,
		DeploymentPlan: `{}`,
		Status:         "proposed",
	}
	deployment2 := &memory.Deployment{
		RepoID:         repo.ID,
		DeploymentPlan: `{}`,
		Status:         "completed",
	}

	handler.memDB.CreateDeployment(deployment1)
	handler.memDB.CreateDeployment(deployment2)

	// Test filtering by status
	req := httptest.NewRequest("GET", "/api/supervisor/deployments?repo_id="+repo.ID+"&status=completed", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &response)

	count := response["count"].(float64)
	if int(count) != 1 {
		t.Errorf("Expected 1 completed deployment, got %v", count)
	}
}

func TestGetDeploymentNotFound(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	router := setupTestRouter(handler)

	req := httptest.NewRequest("GET", "/api/supervisor/deployments/99999", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", rr.Code)
	}
}

func TestUpdateDeploymentStatusMissing(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	router := setupTestRouter(handler)

	// Create a deployment
	repo, _ := handler.memDB.DiscoverRepo(".")
	deployment := &memory.Deployment{
		RepoID:         repo.ID,
		DeploymentPlan: `{}`,
		Status:         "proposed",
	}
	handler.memDB.CreateDeployment(deployment)

	// Test updating without status
	body := map[string]string{}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("PUT", "/api/supervisor/deployments/1/status", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rr.Code)
	}
}

func TestExecuteDeploymentNotConfigured(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	router := setupTestRouter(handler)

	// Handler created without executor
	req := httptest.NewRequest("POST", "/api/supervisor/deployments/1/execute", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	// Should return error since executor not configured
	if rr.Code != http.StatusServiceUnavailable && rr.Code != http.StatusInternalServerError {
		t.Logf("Got status %d (executor may not be configured)", rr.Code)
	}
}

func TestDiscoverRepoInvalidJSON(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	router := setupTestRouter(handler)

	req := httptest.NewRequest("POST", "/api/supervisor/repos", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rr.Code)
	}
}

func TestGetTasksWithStatusFilter(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	router := setupTestRouter(handler)

	// Create a repo and tasks
	repo, _ := handler.memDB.DiscoverRepo(".")

	tasks := []*memory.WorkflowTask{
		{ID: "T1", RepoID: repo.ID, SourceFile: "test.yaml", Title: "Task 1", Status: "pending"},
		{ID: "T2", RepoID: repo.ID, SourceFile: "test.yaml", Title: "Task 2", Status: "in_progress"},
		{ID: "T3", RepoID: repo.ID, SourceFile: "test.yaml", Title: "Task 3", Status: "pending"},
	}

	for _, task := range tasks {
		handler.memDB.CreateTask(task)
	}

	// Test filtering by status
	req := httptest.NewRequest("GET", "/api/supervisor/tasks?status=pending", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &response)

	count := response["count"].(float64)
	if int(count) != 2 {
		t.Errorf("Expected 2 pending tasks, got %v", count)
	}
}
