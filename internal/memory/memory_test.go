package memory

import (
	"path/filepath"
	"testing"
	"time"
)

// setupTestDB creates a temporary test database
func setupTestDB(t *testing.T) (MemoryDB, func()) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	db, err := NewMemoryDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to create test DB: %v", err)
	}

	cleanup := func() {
		db.Close()
	}

	return db, cleanup
}

// Test Repository Operations

func TestDiscoverRepo(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Discover a new repo
	repo, err := db.DiscoverRepo(".")
	if err != nil {
		t.Fatalf("DiscoverRepo failed: %v", err)
	}

	if repo == nil {
		t.Fatal("Expected repo, got nil")
	}

	if repo.ID == "" {
		t.Error("Expected repo ID to be set")
	}

	if repo.BasePath == "" {
		t.Error("Expected base path to be set")
	}

	// Discover same repo again - should return existing
	repo2, err := db.DiscoverRepo(".")
	if err != nil {
		t.Fatalf("Second DiscoverRepo failed: %v", err)
	}

	if repo.ID != repo2.ID {
		t.Errorf("Expected same repo ID, got %s and %s", repo.ID, repo2.ID)
	}
}

func TestGetRepo(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Create a repo
	repo1, err := db.DiscoverRepo(".")
	if err != nil {
		t.Fatalf("DiscoverRepo failed: %v", err)
	}

	// Get repo by ID
	repo2, err := db.GetRepo(repo1.ID)
	if err != nil {
		t.Fatalf("GetRepo failed: %v", err)
	}

	if repo1.ID != repo2.ID {
		t.Errorf("Expected ID %s, got %s", repo1.ID, repo2.ID)
	}

	// Get non-existent repo
	_, err = db.GetRepo("nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent repo")
	}
}

func TestUpdateRepoScan(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, err := db.DiscoverRepo(".")
	if err != nil {
		t.Fatalf("DiscoverRepo failed: %v", err)
	}

	// Mark as scanned
	err = db.UpdateRepoScan(repo.ID)
	if err != nil {
		t.Fatalf("UpdateRepoScan failed: %v", err)
	}

	// Verify scan time updated
	updated, err := db.GetRepo(repo.ID)
	if err != nil {
		t.Fatalf("GetRepo failed: %v", err)
	}

	if updated.LastScanned.IsZero() {
		t.Error("Expected last_scanned to be set")
	}

	if updated.NeedsRescan {
		t.Error("Expected needs_rescan to be false")
	}
}

// Test Repository Files

func TestStoreRepoFile(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, err := db.DiscoverRepo(".")
	if err != nil {
		t.Fatalf("DiscoverRepo failed: %v", err)
	}

	file := &RepoFile{
		RepoID:      repo.ID,
		FilePath:    "CLAUDE.md",
		FileType:    "claude_md",
		ContentHash: "abc123",
		Content:     "# Test content",
	}

	err = db.StoreRepoFile(file)
	if err != nil {
		t.Fatalf("StoreRepoFile failed: %v", err)
	}

	// Retrieve file
	retrieved, err := db.GetRepoFile(repo.ID, "CLAUDE.md")
	if err != nil {
		t.Fatalf("GetRepoFile failed: %v", err)
	}

	if retrieved.ContentHash != "abc123" {
		t.Errorf("Expected hash abc123, got %s", retrieved.ContentHash)
	}
}

func TestGetRepoFiles(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, err := db.DiscoverRepo(".")
	if err != nil {
		t.Fatalf("DiscoverRepo failed: %v", err)
	}

	// Store multiple files
	files := []*RepoFile{
		{RepoID: repo.ID, FilePath: "CLAUDE.md", FileType: "claude_md", ContentHash: "hash1"},
		{RepoID: repo.ID, FilePath: "workflow.yaml", FileType: "workflow_yaml", ContentHash: "hash2"},
		{RepoID: repo.ID, FilePath: "plan.yaml", FileType: "plan_yaml", ContentHash: "hash3"},
	}

	for _, file := range files {
		if err := db.StoreRepoFile(file); err != nil {
			t.Fatalf("StoreRepoFile failed: %v", err)
		}
	}

	// Get all files
	allFiles, err := db.GetRepoFiles(repo.ID, "")
	if err != nil {
		t.Fatalf("GetRepoFiles failed: %v", err)
	}

	if len(allFiles) != 3 {
		t.Errorf("Expected 3 files, got %d", len(allFiles))
	}

	// Get files by type
	claudeFiles, err := db.GetRepoFiles(repo.ID, "claude_md")
	if err != nil {
		t.Fatalf("GetRepoFiles with type failed: %v", err)
	}

	if len(claudeFiles) != 1 {
		t.Errorf("Expected 1 claude_md file, got %d", len(claudeFiles))
	}
}

// Test Agent Learnings

func TestStoreAgentLearning(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Create a repo first for foreign key constraint
	repo, err := db.DiscoverRepo(".")
	if err != nil {
		t.Fatalf("DiscoverRepo failed: %v", err)
	}

	learning := &AgentLearning{
		AgentID:   "coder001",
		AgentType: "coder",
		Category:  "solution",
		Title:     "How to handle port conflicts",
		Content:   "Use instance management with PID files",
		RepoID:    repo.ID,
	}

	err = db.StoreAgentLearning(learning)
	if err != nil {
		t.Fatalf("StoreAgentLearning failed: %v", err)
	}

	if learning.ID == 0 {
		t.Error("Expected learning ID to be set")
	}
}

func TestGetAgentLearnings(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Store multiple learnings
	learnings := []*AgentLearning{
		{AgentID: "coder001", AgentType: "coder", Category: "solution", Title: "Learning 1", Content: "Content 1"},
		{AgentID: "coder001", AgentType: "coder", Category: "error_pattern", Title: "Learning 2", Content: "Content 2"},
		{AgentID: "tester001", AgentType: "tester", Category: "solution", Title: "Learning 3", Content: "Content 3"},
	}

	for _, learning := range learnings {
		if err := db.StoreAgentLearning(learning); err != nil {
			t.Fatalf("StoreAgentLearning failed: %v", err)
		}
	}

	// Get all learnings
	all, err := db.GetAgentLearnings(LearnFilter{})
	if err != nil {
		t.Fatalf("GetAgentLearnings failed: %v", err)
	}

	if len(all) != 3 {
		t.Errorf("Expected 3 learnings, got %d", len(all))
	}

	// Filter by agent
	coderLearnings, err := db.GetAgentLearnings(LearnFilter{AgentID: "coder001"})
	if err != nil {
		t.Fatalf("GetAgentLearnings with filter failed: %v", err)
	}

	if len(coderLearnings) != 2 {
		t.Errorf("Expected 2 coder learnings, got %d", len(coderLearnings))
	}

	// Filter by category
	solutions, err := db.GetAgentLearnings(LearnFilter{Category: "solution"})
	if err != nil {
		t.Fatalf("GetAgentLearnings with category filter failed: %v", err)
	}

	if len(solutions) != 2 {
		t.Errorf("Expected 2 solutions, got %d", len(solutions))
	}
}

// Test Context Summaries

func TestStoreContextSummary(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Create a repo first for foreign key constraint
	repo, err := db.DiscoverRepo(".")
	if err != nil {
		t.Fatalf("DiscoverRepo failed: %v", err)
	}

	summary := &ContextSummary{
		SessionID: "session-123",
		AgentID:   "coder001",
		Summary:   "Implemented port conflict resolution",
		RepoID:    repo.ID,
	}

	err = db.StoreContextSummary(summary)
	if err != nil {
		t.Fatalf("StoreContextSummary failed: %v", err)
	}

	if summary.ID == 0 {
		t.Error("Expected summary ID to be set")
	}
}

func TestGetRecentSummaries(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Store multiple summaries
	for i := 0; i < 5; i++ {
		summary := &ContextSummary{
			SessionID: "session-123",
			AgentID:   "coder001",
			Summary:   "Summary " + string(rune('A'+i)),
		}
		if err := db.StoreContextSummary(summary); err != nil {
			t.Fatalf("StoreContextSummary failed: %v", err)
		}
		time.Sleep(10 * time.Millisecond) // Ensure different timestamps
	}

	// Get recent summaries
	summaries, err := db.GetRecentSummaries(3)
	if err != nil {
		t.Fatalf("GetRecentSummaries failed: %v", err)
	}

	if len(summaries) != 3 {
		t.Errorf("Expected 3 summaries, got %d", len(summaries))
	}
}

// Test Workflow Tasks

func TestCreateTask(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Create a repo first for foreign key constraint
	repo, err := db.DiscoverRepo(".")
	if err != nil {
		t.Fatalf("DiscoverRepo failed: %v", err)
	}

	task := &WorkflowTask{
		ID:         "MAH-123",
		RepoID:     repo.ID,
		SourceFile: "workflow.yaml",
		Title:      "Implement feature X",
		Priority:   "high",
		Status:     "pending",
	}

	err = db.CreateTask(task)
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	// Retrieve task
	retrieved, err := db.GetTask("MAH-123")
	if err != nil {
		t.Fatalf("GetTask failed: %v", err)
	}

	if retrieved.Title != "Implement feature X" {
		t.Errorf("Expected title 'Implement feature X', got '%s'", retrieved.Title)
	}
}

func TestUpdateTaskStatus(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Create a repo first for foreign key constraint
	repo, err := db.DiscoverRepo(".")
	if err != nil {
		t.Fatalf("DiscoverRepo failed: %v", err)
	}

	task := &WorkflowTask{
		ID:         "MAH-124",
		RepoID:     repo.ID,
		SourceFile: "workflow.yaml",
		Title:      "Test task",
		Status:     "pending",
	}

	if err := db.CreateTask(task); err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	// Update status
	err = db.UpdateTaskStatus("MAH-124", "in_progress", "coder001")
	if err != nil {
		t.Fatalf("UpdateTaskStatus failed: %v", err)
	}

	// Verify update
	updated, err := db.GetTask("MAH-124")
	if err != nil {
		t.Fatalf("GetTask failed: %v", err)
	}

	if updated.Status != "in_progress" {
		t.Errorf("Expected status 'in_progress', got '%s'", updated.Status)
	}

	if updated.AssignedAgentID != "coder001" {
		t.Errorf("Expected agent 'coder001', got '%s'", updated.AssignedAgentID)
	}
}

func TestGetTasks(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Create a repo first for foreign key constraint
	repo, err := db.DiscoverRepo(".")
	if err != nil {
		t.Fatalf("DiscoverRepo failed: %v", err)
	}

	// Create multiple tasks (all using the same repo for simplicity)
	tasks := []*WorkflowTask{
		{ID: "MAH-125", RepoID: repo.ID, SourceFile: "w1.yaml", Title: "Task 1", Status: "pending"},
		{ID: "MAH-126", RepoID: repo.ID, SourceFile: "w1.yaml", Title: "Task 2", Status: "in_progress", AssignedAgentID: "coder001"},
		{ID: "MAH-127", RepoID: repo.ID, SourceFile: "w2.yaml", Title: "Task 3", Status: "completed"},
	}

	for _, task := range tasks {
		if err := db.CreateTask(task); err != nil {
			t.Fatalf("CreateTask failed: %v", err)
		}
	}

	// Get all tasks
	all, err := db.GetTasks(TaskFilter{})
	if err != nil {
		t.Fatalf("GetTasks failed: %v", err)
	}

	if len(all) != 3 {
		t.Errorf("Expected 3 tasks, got %d", len(all))
	}

	// Filter by repo
	repoTasks, err := db.GetTasks(TaskFilter{RepoID: repo.ID})
	if err != nil {
		t.Fatalf("GetTasks with repo filter failed: %v", err)
	}

	if len(repoTasks) != 3 {
		t.Errorf("Expected 3 repo tasks, got %d", len(repoTasks))
	}

	// Filter by status
	pendingTasks, err := db.GetTasks(TaskFilter{Status: "pending"})
	if err != nil {
		t.Fatalf("GetTasks with status filter failed: %v", err)
	}

	if len(pendingTasks) != 1 {
		t.Errorf("Expected 1 pending task, got %d", len(pendingTasks))
	}
}

// Test Human Decisions

func TestStoreDecision(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	decision := &HumanDecision{
		Context:      "Agent proposed deployment plan",
		Question:     "Should I deploy 3 agents for these tasks?",
		Answer:       "Yes, proceed with deployment",
		DecisionType: "approval",
		AgentID:      "supervisor",
	}

	err := db.StoreDecision(decision)
	if err != nil {
		t.Fatalf("StoreDecision failed: %v", err)
	}

	if decision.ID == 0 {
		t.Error("Expected decision ID to be set")
	}
}

func TestGetRecentDecisions(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Store multiple decisions
	for i := 0; i < 3; i++ {
		decision := &HumanDecision{
			Context:  "Test context",
			Question: "Question?",
			Answer:   "Answer",
			AgentID:  "supervisor",
		}
		if err := db.StoreDecision(decision); err != nil {
			t.Fatalf("StoreDecision failed: %v", err)
		}
	}

	decisions, err := db.GetRecentDecisions(10)
	if err != nil {
		t.Fatalf("GetRecentDecisions failed: %v", err)
	}

	if len(decisions) != 3 {
		t.Errorf("Expected 3 decisions, got %d", len(decisions))
	}
}

// Test Deployments

func TestCreateDeployment(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Create a repo first for foreign key constraint
	repo, err := db.DiscoverRepo(".")
	if err != nil {
		t.Fatalf("DiscoverRepo failed: %v", err)
	}

	deployment := &Deployment{
		RepoID:         repo.ID,
		DeploymentPlan: `{"agents": ["coder", "tester"]}`,
		Status:         "proposed",
	}

	err = db.CreateDeployment(deployment)
	if err != nil {
		t.Fatalf("CreateDeployment failed: %v", err)
	}

	if deployment.ID == 0 {
		t.Error("Expected deployment ID to be set")
	}

	// Retrieve deployment
	retrieved, err := db.GetDeployment(deployment.ID)
	if err != nil {
		t.Fatalf("GetDeployment failed: %v", err)
	}

	if retrieved.Status != "proposed" {
		t.Errorf("Expected status 'proposed', got '%s'", retrieved.Status)
	}
}

func TestUpdateDeploymentStatus(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Create a repo first for foreign key constraint
	repo, err := db.DiscoverRepo(".")
	if err != nil {
		t.Fatalf("DiscoverRepo failed: %v", err)
	}

	deployment := &Deployment{
		RepoID:         repo.ID,
		DeploymentPlan: "{}",
		Status:         "proposed",
	}

	if err := db.CreateDeployment(deployment); err != nil {
		t.Fatalf("CreateDeployment failed: %v", err)
	}

	// Update to approved
	err = db.UpdateDeploymentStatus(deployment.ID, "approved")
	if err != nil {
		t.Fatalf("UpdateDeploymentStatus failed: %v", err)
	}

	// Verify update
	updated, err := db.GetDeployment(deployment.ID)
	if err != nil {
		t.Fatalf("GetDeployment failed: %v", err)
	}

	if updated.Status != "approved" {
		t.Errorf("Expected status 'approved', got '%s'", updated.Status)
	}

	if updated.ApprovedAt == nil {
		t.Error("Expected approved_at to be set")
	}
}
