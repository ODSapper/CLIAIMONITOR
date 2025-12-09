package agents

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/CLIAIMONITOR/internal/memory"
	"github.com/CLIAIMONITOR/internal/types"
)

// MockMemoryDB implements memory.MemoryDB for testing
type MockMemoryDB struct {
	mu               sync.Mutex
	agents           map[string]*memory.AgentControl
	shutdownFlags    map[string]string
	registerCalled   int
	updateCalled     int
	markStoppedCalls []string
}

func NewMockMemoryDB() *MockMemoryDB {
	return &MockMemoryDB{
		agents:        make(map[string]*memory.AgentControl),
		shutdownFlags: make(map[string]string),
	}
}

func (m *MockMemoryDB) RegisterAgent(agent *memory.AgentControl) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.agents[agent.AgentID] = agent
	m.registerCalled++
	return nil
}

func (m *MockMemoryDB) UpdateStatus(agentID, status, currentTask string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if agent, ok := m.agents[agentID]; ok {
		agent.Status = status
		agent.CurrentTask = currentTask
	}
	m.updateCalled++
	return nil
}

func (m *MockMemoryDB) SetShutdownFlag(agentID string, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.shutdownFlags[agentID] = reason
	return nil
}

func (m *MockMemoryDB) ClearShutdownFlag(agentID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.shutdownFlags, agentID)
	return nil
}

func (m *MockMemoryDB) MarkStopped(agentID, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if agent, ok := m.agents[agentID]; ok {
		agent.Status = "stopped"
	}
	m.markStoppedCalls = append(m.markStoppedCalls, agentID)
	return nil
}

func (m *MockMemoryDB) RemoveAgent(agentID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.agents, agentID)
	return nil
}

func (m *MockMemoryDB) GetAgent(agentID string) (*memory.AgentControl, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if agent, ok := m.agents[agentID]; ok {
		return agent, nil
	}
	return nil, nil
}

func (m *MockMemoryDB) GetAllAgents() ([]*memory.AgentControl, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*memory.AgentControl
	for _, agent := range m.agents {
		result = append(result, agent)
	}
	return result, nil
}

func (m *MockMemoryDB) GetStaleAgents(threshold time.Duration) ([]*memory.AgentControl, error) {
	return nil, nil
}

func (m *MockMemoryDB) GetAgentsByStatus(status string) ([]*memory.AgentControl, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*memory.AgentControl
	for _, agent := range m.agents {
		if agent.Status == status {
			result = append(result, agent)
		}
	}
	return result, nil
}

func (m *MockMemoryDB) CheckShutdownFlag(agentID string) (bool, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if reason, ok := m.shutdownFlags[agentID]; ok {
		return true, reason, nil
	}
	return false, "", nil
}

// Stub implementations for other MemoryDB interface methods
func (m *MockMemoryDB) DiscoverRepo(basePath string) (*memory.Repo, error)        { return nil, nil }
func (m *MockMemoryDB) GetRepo(repoID string) (*memory.Repo, error)               { return nil, nil }
func (m *MockMemoryDB) GetRepoByPath(basePath string) (*memory.Repo, error)       { return nil, nil }
func (m *MockMemoryDB) UpdateRepoScan(repoID string) error                        { return nil }
func (m *MockMemoryDB) SetRepoRescan(repoID string, needsRescan bool) error       { return nil }
func (m *MockMemoryDB) StoreRepoFile(file *memory.RepoFile) error                 { return nil }
func (m *MockMemoryDB) GetRepoFiles(repoID string, fileType string) ([]*memory.RepoFile, error) {
	return nil, nil
}
func (m *MockMemoryDB) GetRepoFile(repoID, filePath string) (*memory.RepoFile, error) { return nil, nil }
func (m *MockMemoryDB) StoreAgentLearning(learning *memory.AgentLearning) error      { return nil }
func (m *MockMemoryDB) GetAgentLearnings(filter memory.LearnFilter) ([]*memory.AgentLearning, error) {
	return nil, nil
}
func (m *MockMemoryDB) GetRecentLearnings(limit int) ([]*memory.AgentLearning, error) { return nil, nil }
func (m *MockMemoryDB) StoreContextSummary(summary *memory.ContextSummary) error     { return nil }
func (m *MockMemoryDB) GetRecentSummaries(limit int) ([]*memory.ContextSummary, error) {
	return nil, nil
}
func (m *MockMemoryDB) GetSummariesByAgent(agentID string, limit int) ([]*memory.ContextSummary, error) {
	return nil, nil
}
func (m *MockMemoryDB) GetSummariesBySession(sessionID string) ([]*memory.ContextSummary, error) {
	return nil, nil
}
func (m *MockMemoryDB) CreateTask(task *memory.WorkflowTask) error        { return nil }
func (m *MockMemoryDB) CreateTasks(tasks []*memory.WorkflowTask) error    { return nil }
func (m *MockMemoryDB) GetTask(taskID string) (*memory.WorkflowTask, error) { return nil, nil }
func (m *MockMemoryDB) GetTasks(filter memory.TaskFilter) ([]*memory.WorkflowTask, error) {
	return nil, nil
}
func (m *MockMemoryDB) UpdateTaskStatus(taskID, status, agentID string) error { return nil }
func (m *MockMemoryDB) UpdateTask(task *memory.WorkflowTask) error            { return nil }
func (m *MockMemoryDB) StoreDecision(decision *memory.HumanDecision) error    { return nil }
func (m *MockMemoryDB) GetRecentDecisions(limit int) ([]*memory.HumanDecision, error) {
	return nil, nil
}
func (m *MockMemoryDB) GetDecisionsByAgent(agentID string, limit int) ([]*memory.HumanDecision, error) {
	return nil, nil
}
func (m *MockMemoryDB) CreateDeployment(deployment *memory.Deployment) error { return nil }
func (m *MockMemoryDB) GetDeployment(deploymentID int64) (*memory.Deployment, error) {
	return nil, nil
}
func (m *MockMemoryDB) GetRecentDeployments(repoID string, limit int) ([]*memory.Deployment, error) {
	return nil, nil
}
func (m *MockMemoryDB) UpdateDeploymentStatus(deploymentID int64, status string) error { return nil }
func (m *MockMemoryDB) AsLearningDB() memory.LearningDB                               { return nil }
func (m *MockMemoryDB) SetContext(key, value string, priority int, maxAgeHours int) error {
	return nil
}
func (m *MockMemoryDB) GetContext(key string) (*memory.CaptainContext, error)         { return nil, nil }
func (m *MockMemoryDB) GetAllContext() ([]*memory.CaptainContext, error)              { return nil, nil }
func (m *MockMemoryDB) GetContextByPriority(minPriority int) ([]*memory.CaptainContext, error) {
	return nil, nil
}
func (m *MockMemoryDB) DeleteContext(key string) error               { return nil }
func (m *MockMemoryDB) CleanExpiredContext() (int, error)            { return 0, nil }
func (m *MockMemoryDB) LogSessionEvent(sessionID, eventType, summary, details, agentID string) error {
	return nil
}
func (m *MockMemoryDB) GetSessionLog(sessionID string, limit int) ([]*memory.SessionLogEntry, error) {
	return nil, nil
}
func (m *MockMemoryDB) GetRecentSessionLog(limit int) ([]*memory.SessionLogEntry, error) {
	return nil, nil
}
func (m *MockMemoryDB) RecordMetricsHistory(agentID, model string, tokensUsed int64, estimatedCost float64, taskID string) error {
	return nil
}
func (m *MockMemoryDB) GetMetricsByModel(modelFilter string) ([]*memory.ModelMetrics, error) {
	return nil, nil
}
func (m *MockMemoryDB) GetMetricsByAgentType() ([]*memory.AgentTypeMetrics, error) { return nil, nil }
func (m *MockMemoryDB) GetMetricsByAgent() ([]*memory.AgentMetricsSummary, error)  { return nil, nil }
func (m *MockMemoryDB) RecordMetricsWithType(agentID, model, agentType, parentAgent string, tokensUsed int64, estimatedCost float64, taskID string, assignmentID *int64) error {
	return nil
}
func (m *MockMemoryDB) CreateAssignment(assignment *memory.TaskAssignment) error { return nil }
func (m *MockMemoryDB) GetAssignment(id int64) (*memory.TaskAssignment, error)   { return nil, nil }
func (m *MockMemoryDB) GetAssignmentsByTask(taskID string) ([]*memory.TaskAssignment, error) {
	return nil, nil
}
func (m *MockMemoryDB) GetAssignmentsByAgent(agentID string, status string) ([]*memory.TaskAssignment, error) {
	return nil, nil
}
func (m *MockMemoryDB) GetActiveAssignment(agentID string) (*memory.TaskAssignment, error) {
	return nil, nil
}
func (m *MockMemoryDB) UpdateAssignmentStatus(id int64, status string) error { return nil }
func (m *MockMemoryDB) CompleteAssignment(id int64, status string, feedback string) error {
	return nil
}
func (m *MockMemoryDB) AddWorker(worker *memory.AssignmentWorker) error { return nil }
func (m *MockMemoryDB) UpdateWorkerStatus(id int64, status, result string, tokensUsed int64) error {
	return nil
}
func (m *MockMemoryDB) GetWorkersByAssignment(assignmentID int64) ([]*memory.AssignmentWorker, error) {
	return nil, nil
}

// Health and lifecycle methods
func (m *MockMemoryDB) Health() (*memory.HealthStatus, error) {
	return &memory.HealthStatus{Connected: true}, nil
}

func (m *MockMemoryDB) Close() error {
	return nil
}

// TestNewSpawner tests the spawner constructor
func TestNewSpawner(t *testing.T) {
	basePath := t.TempDir()
	mcpURL := "http://localhost:3000/mcp/sse"
	mockDB := NewMockMemoryDB()

	spawner := NewSpawner(basePath, mcpURL, mockDB)

	if spawner == nil {
		t.Fatal("NewSpawner returned nil")
	}

	if spawner.basePath != basePath {
		t.Errorf("Expected basePath %s, got %s", basePath, spawner.basePath)
	}

	if spawner.mcpServerURL != mcpURL {
		t.Errorf("Expected mcpServerURL %s, got %s", mcpURL, spawner.mcpServerURL)
	}

	expectedPromptsPath := filepath.Join(basePath, "configs", "prompts")
	if spawner.promptsPath != expectedPromptsPath {
		t.Errorf("Expected promptsPath %s, got %s", expectedPromptsPath, spawner.promptsPath)
	}

	expectedScriptsPath := filepath.Join(basePath, "scripts")
	if spawner.scriptsPath != expectedScriptsPath {
		t.Errorf("Expected scriptsPath %s, got %s", expectedScriptsPath, spawner.scriptsPath)
	}

	if spawner.runningAgents == nil {
		t.Error("runningAgents map not initialized")
	}

	if spawner.agentCounters == nil {
		t.Error("agentCounters map not initialized")
	}
}

// TestSetNATSURL tests setting and getting NATS URL
func TestSetNATSURL(t *testing.T) {
	spawner := NewSpawner(t.TempDir(), "http://localhost:3000/mcp/sse", nil)

	natsURL := "nats://localhost:4222"
	spawner.SetNATSURL(natsURL)

	if spawner.GetNATSURL() != natsURL {
		t.Errorf("Expected NATS URL %s, got %s", natsURL, spawner.GetNATSURL())
	}
}

// TestGenerateAgentID tests agent ID generation
func TestGenerateAgentID(t *testing.T) {
	spawner := NewSpawner(t.TempDir(), "http://localhost:3000/mcp/sse", nil)

	tests := []struct {
		agentType string
		expected  string
	}{
		{"OpusGreen", "team-opusgreen001"},
		{"OpusGreen", "team-opusgreen002"},
		{"SNTPurple", "team-sntpurple001"},
		{"OpusGreen", "team-opusgreen003"},
		{"Snake", "team-snake001"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := spawner.GenerateAgentID(tt.agentType)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

// TestGenerateAgentID_Concurrent tests thread safety of ID generation
func TestGenerateAgentID_Concurrent(t *testing.T) {
	spawner := NewSpawner(t.TempDir(), "http://localhost:3000/mcp/sse", nil)

	var wg sync.WaitGroup
	ids := make(chan string, 100)

	// Generate 100 IDs concurrently
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id := spawner.GenerateAgentID("TestAgent")
			ids <- id
		}()
	}

	wg.Wait()
	close(ids)

	// Collect all IDs and check for duplicates
	seen := make(map[string]bool)
	for id := range ids {
		if seen[id] {
			t.Errorf("Duplicate ID generated: %s", id)
		}
		seen[id] = true
	}

	if len(seen) != 100 {
		t.Errorf("Expected 100 unique IDs, got %d", len(seen))
	}
}

// TestGetNextSequence tests sequence number prediction
func TestGetNextSequence(t *testing.T) {
	spawner := NewSpawner(t.TempDir(), "http://localhost:3000/mcp/sse", nil)

	// Should return 1 for new agent type
	if seq := spawner.GetNextSequence("NewType"); seq != 1 {
		t.Errorf("Expected next sequence 1, got %d", seq)
	}

	// Generate an ID
	spawner.GenerateAgentID("NewType")

	// Should now return 2
	if seq := spawner.GetNextSequence("NewType"); seq != 2 {
		t.Errorf("Expected next sequence 2, got %d", seq)
	}
}

// TestGetRunningAgents tests the running agents retrieval
func TestGetRunningAgents(t *testing.T) {
	spawner := NewSpawner(t.TempDir(), "http://localhost:3000/mcp/sse", nil)

	// Initially empty
	agents := spawner.GetRunningAgents()
	if len(agents) != 0 {
		t.Errorf("Expected 0 running agents, got %d", len(agents))
	}

	// Manually add an agent (simulating spawn)
	spawner.mu.Lock()
	spawner.runningAgents["test-agent-001"] = 12345
	spawner.mu.Unlock()

	// Should now have one
	agents = spawner.GetRunningAgents()
	if len(agents) != 1 {
		t.Errorf("Expected 1 running agent, got %d", len(agents))
	}

	if pid, ok := agents["test-agent-001"]; !ok || pid != 12345 {
		t.Errorf("Expected PID 12345 for test-agent-001, got %d", pid)
	}

	// Verify it returns a copy, not the original map
	agents["test-agent-001"] = 99999
	originalAgents := spawner.GetRunningAgents()
	if originalAgents["test-agent-001"] != 12345 {
		t.Error("GetRunningAgents should return a copy, not the original map")
	}
}

// TestRemoveAgent tests agent removal from tracking
func TestRemoveAgent(t *testing.T) {
	spawner := NewSpawner(t.TempDir(), "http://localhost:3000/mcp/sse", nil)

	// Add an agent
	spawner.mu.Lock()
	spawner.runningAgents["test-agent-001"] = 12345
	spawner.mu.Unlock()

	// Remove it
	spawner.RemoveAgent("test-agent-001")

	// Should be gone
	agents := spawner.GetRunningAgents()
	if _, ok := agents["test-agent-001"]; ok {
		t.Error("Agent should have been removed")
	}
}

// TestGetAgentByPID tests finding agent by PID
func TestGetAgentByPID(t *testing.T) {
	spawner := NewSpawner(t.TempDir(), "http://localhost:3000/mcp/sse", nil)

	// Add agents
	spawner.mu.Lock()
	spawner.runningAgents["agent-001"] = 12345
	spawner.runningAgents["agent-002"] = 67890
	spawner.mu.Unlock()

	// Find by PID
	if id := spawner.GetAgentByPID(12345); id != "agent-001" {
		t.Errorf("Expected agent-001, got %s", id)
	}

	if id := spawner.GetAgentByPID(67890); id != "agent-002" {
		t.Errorf("Expected agent-002, got %s", id)
	}

	// Non-existent PID
	if id := spawner.GetAgentByPID(99999); id != "" {
		t.Errorf("Expected empty string for non-existent PID, got %s", id)
	}
}

// TestGetAccessRules tests access rule generation for different roles
func TestGetAccessRules(t *testing.T) {
	spawner := NewSpawner(t.TempDir(), "http://localhost:3000/mcp/sse", nil)
	projectPath := "/test/project"

	tests := []struct {
		role     types.AgentRole
		contains string
	}{
		{types.RoleCodeAuditor, "read files from any project"},
		{types.RoleSecurity, "read files from any project"},
		{types.RoleSupervisor, "should not write code files"},
		{types.RoleGoDeveloper, "ONLY read and write files within"},
		{types.RoleEngineer, "ONLY read and write files within"},
	}

	for _, tt := range tests {
		t.Run(string(tt.role), func(t *testing.T) {
			rules := spawner.getAccessRules(tt.role, projectPath)
			if rules == "" {
				t.Error("Access rules should not be empty")
			}
			if !contains(rules, tt.contains) {
				t.Errorf("Expected rules for %s to contain '%s', got: %s", tt.role, tt.contains, rules)
			}
		})
	}
}

// TestCreateMCPConfig tests MCP configuration file creation
func TestCreateMCPConfig(t *testing.T) {
	basePath := t.TempDir()
	spawner := NewSpawner(basePath, "http://localhost:3000/mcp/sse", nil)
	spawner.SetNATSURL("nats://localhost:4222")

	// Create necessary directories
	os.MkdirAll(filepath.Join(basePath, "configs", "mcp"), 0755)

	configPath, err := spawner.createMCPConfig("test-agent-001", "/test/project", types.AccessStrict)
	if err != nil {
		t.Fatalf("createMCPConfig failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatalf("Config file not created at %s", configPath)
	}

	// Read and verify content
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	content := string(data)
	if !contains(content, "test-agent-001") {
		t.Error("Config should contain agent ID")
	}
	if !contains(content, "nats://localhost:4222") {
		t.Error("Config should contain NATS URL")
	}
	if !contains(content, "/test/project") {
		t.Error("Config should contain project path")
	}
}

// TestCleanupAgentPIDFile tests PID file cleanup
func TestCleanupAgentPIDFile(t *testing.T) {
	basePath := t.TempDir()
	spawner := NewSpawner(basePath, "http://localhost:3000/mcp/sse", nil)

	// Create PID directory and file
	pidsDir := filepath.Join(basePath, "data", "pids")
	os.MkdirAll(pidsDir, 0755)

	pidFile := filepath.Join(pidsDir, "test-agent.pid")
	os.WriteFile(pidFile, []byte("12345"), 0644)

	// Verify file exists
	if _, err := os.Stat(pidFile); os.IsNotExist(err) {
		t.Fatal("PID file should exist before cleanup")
	}

	// Cleanup
	err := spawner.CleanupAgentPIDFile("test-agent")
	if err != nil {
		t.Errorf("CleanupAgentPIDFile failed: %v", err)
	}

	// Verify file is gone
	if _, err := os.Stat(pidFile); !os.IsNotExist(err) {
		t.Error("PID file should be removed after cleanup")
	}
}

// TestGetAgentPIDFromFile tests reading PID from file
func TestGetAgentPIDFromFile(t *testing.T) {
	basePath := t.TempDir()
	spawner := NewSpawner(basePath, "http://localhost:3000/mcp/sse", nil)

	// Create PID directory and file
	pidsDir := filepath.Join(basePath, "data", "pids")
	os.MkdirAll(pidsDir, 0755)

	pidFile := filepath.Join(pidsDir, "test-agent.pid")
	os.WriteFile(pidFile, []byte("12345"), 0644)

	// Read PID
	pid, err := spawner.GetAgentPIDFromFile("test-agent")
	if err != nil {
		t.Fatalf("GetAgentPIDFromFile failed: %v", err)
	}

	if pid != 12345 {
		t.Errorf("Expected PID 12345, got %d", pid)
	}

	// Test with whitespace
	os.WriteFile(pidFile, []byte("  67890\n"), 0644)
	pid, err = spawner.GetAgentPIDFromFile("test-agent")
	if err != nil {
		t.Fatalf("GetAgentPIDFromFile with whitespace failed: %v", err)
	}
	if pid != 67890 {
		t.Errorf("Expected PID 67890, got %d", pid)
	}

	// Test non-existent file
	_, err = spawner.GetAgentPIDFromFile("non-existent")
	if err == nil {
		t.Error("Expected error for non-existent PID file")
	}

	// Test invalid PID
	os.WriteFile(pidFile, []byte("not-a-number"), 0644)
	_, err = spawner.GetAgentPIDFromFile("test-agent")
	if err == nil {
		t.Error("Expected error for invalid PID content")
	}
}

// TestCleanupAgentFiles tests agent file cleanup
func TestCleanupAgentFiles(t *testing.T) {
	basePath := t.TempDir()
	spawner := NewSpawner(basePath, "http://localhost:3000/mcp/sse", nil)

	// Create all directories
	mcpDir := filepath.Join(basePath, "configs", "mcp")
	promptsDir := filepath.Join(basePath, "configs", "prompts", "active")
	pidsDir := filepath.Join(basePath, "data", "pids")
	os.MkdirAll(mcpDir, 0755)
	os.MkdirAll(promptsDir, 0755)
	os.MkdirAll(pidsDir, 0755)

	// Create files
	os.WriteFile(filepath.Join(mcpDir, "test-agent-mcp.json"), []byte("{}"), 0644)
	os.WriteFile(filepath.Join(promptsDir, "test-agent-prompt.md"), []byte("# Test"), 0644)
	os.WriteFile(filepath.Join(pidsDir, "test-agent.pid"), []byte("12345"), 0644)

	// Cleanup
	err := spawner.CleanupAgentFiles("test-agent")
	if err != nil {
		t.Errorf("CleanupAgentFiles failed: %v", err)
	}

	// Verify all files are gone
	if _, err := os.Stat(filepath.Join(mcpDir, "test-agent-mcp.json")); !os.IsNotExist(err) {
		t.Error("MCP config should be removed")
	}
	if _, err := os.Stat(filepath.Join(promptsDir, "test-agent-prompt.md")); !os.IsNotExist(err) {
		t.Error("Prompt file should be removed")
	}
	if _, err := os.Stat(filepath.Join(pidsDir, "test-agent.pid")); !os.IsNotExist(err) {
		t.Error("PID file should be removed")
	}
}

// TestStopAgentWithReason tests agent stopping with DB updates
func TestStopAgentWithReason(t *testing.T) {
	basePath := t.TempDir()
	mockDB := NewMockMemoryDB()
	spawner := NewSpawner(basePath, "http://localhost:3000/mcp/sse", mockDB)

	// Register a fake agent
	mockDB.RegisterAgent(&memory.AgentControl{
		AgentID: "test-agent-001",
		Status:  "running",
	})

	// Add to running agents
	spawner.mu.Lock()
	spawner.runningAgents["test-agent-001"] = 99999 // Fake PID
	spawner.mu.Unlock()

	// Stop it
	err := spawner.StopAgentWithReason("test-agent-001", "test stop")
	if err != nil {
		t.Errorf("StopAgentWithReason failed: %v", err)
	}

	// Verify shutdown flag was set
	if reason, ok := mockDB.shutdownFlags["test-agent-001"]; !ok || reason != "test stop" {
		t.Error("Shutdown flag should be set")
	}

	// Verify MarkStopped was called
	if len(mockDB.markStoppedCalls) == 0 || mockDB.markStoppedCalls[0] != "test-agent-001" {
		t.Error("MarkStopped should have been called")
	}

	// Verify removed from running agents
	if _, ok := spawner.GetRunningAgents()["test-agent-001"]; ok {
		t.Error("Agent should be removed from running agents")
	}
}

// TestStopAgent tests backward compatible stop method
func TestStopAgent(t *testing.T) {
	basePath := t.TempDir()
	mockDB := NewMockMemoryDB()
	spawner := NewSpawner(basePath, "http://localhost:3000/mcp/sse", mockDB)

	// Register a fake agent
	mockDB.RegisterAgent(&memory.AgentControl{
		AgentID: "test-agent-001",
		Status:  "running",
	})

	// Stop it using backward compatible method
	err := spawner.StopAgent("test-agent-001")
	if err != nil {
		t.Errorf("StopAgent failed: %v", err)
	}

	// Should have used "manual stop" as reason
	if reason, ok := mockDB.shutdownFlags["test-agent-001"]; !ok || reason != "manual stop" {
		t.Errorf("Expected reason 'manual stop', got '%s'", reason)
	}
}

// TestTrackHeartbeatPID tests heartbeat PID tracking
func TestTrackHeartbeatPID(t *testing.T) {
	spawner := NewSpawner(t.TempDir(), "http://localhost:3000/mcp/sse", nil)

	spawner.trackHeartbeatPID("agent-001", 12345)

	spawner.mu.RLock()
	pid, ok := spawner.heartbeatPIDs["agent-001"]
	spawner.mu.RUnlock()

	if !ok || pid != 12345 {
		t.Errorf("Expected heartbeat PID 12345, got %d", pid)
	}
}

// TestCleanupAllAgentFiles tests bulk file cleanup
func TestCleanupAllAgentFiles(t *testing.T) {
	basePath := t.TempDir()
	spawner := NewSpawner(basePath, "http://localhost:3000/mcp/sse", nil)

	// Create all directories
	mcpDir := filepath.Join(basePath, "configs", "mcp")
	promptsDir := filepath.Join(basePath, "configs", "prompts", "active")
	pidsDir := filepath.Join(basePath, "data", "pids")
	os.MkdirAll(mcpDir, 0755)
	os.MkdirAll(promptsDir, 0755)
	os.MkdirAll(pidsDir, 0755)

	// Create multiple agent files
	os.WriteFile(filepath.Join(mcpDir, "agent1-mcp.json"), []byte("{}"), 0644)
	os.WriteFile(filepath.Join(mcpDir, "agent2-mcp.json"), []byte("{}"), 0644)
	os.WriteFile(filepath.Join(promptsDir, "agent1-prompt.md"), []byte("# Test"), 0644)
	os.WriteFile(filepath.Join(promptsDir, "agent2-prompt.md"), []byte("# Test"), 0644)
	os.WriteFile(filepath.Join(pidsDir, "agent1.pid"), []byte("111"), 0644)
	os.WriteFile(filepath.Join(pidsDir, "agent2.pid"), []byte("222"), 0644)

	// Also create non-agent files that should NOT be deleted
	os.WriteFile(filepath.Join(mcpDir, "config.json"), []byte("{}"), 0644)

	// Cleanup all
	err := spawner.CleanupAllAgentFiles()
	if err != nil {
		t.Errorf("CleanupAllAgentFiles failed: %v", err)
	}

	// Verify agent files are gone
	entries, _ := os.ReadDir(mcpDir)
	for _, e := range entries {
		if e.Name() != "config.json" {
			t.Errorf("Agent file should be removed: %s", e.Name())
		}
	}

	// Verify non-agent file still exists
	if _, err := os.Stat(filepath.Join(mcpDir, "config.json")); os.IsNotExist(err) {
		t.Error("Non-agent config file should not be removed")
	}

	// Verify prompts directory is empty (of agent files)
	entries, _ = os.ReadDir(promptsDir)
	if len(entries) > 0 {
		t.Errorf("All prompt files should be removed, found %d", len(entries))
	}
}

// TestStopAllAgents tests stopping all running agents
func TestStopAllAgents(t *testing.T) {
	basePath := t.TempDir()
	mockDB := NewMockMemoryDB()
	spawner := NewSpawner(basePath, "http://localhost:3000/mcp/sse", mockDB)

	// Add multiple agents
	for _, id := range []string{"agent-001", "agent-002", "agent-003"} {
		mockDB.RegisterAgent(&memory.AgentControl{
			AgentID: id,
			Status:  "running",
		})
		spawner.mu.Lock()
		spawner.runningAgents[id] = 12345
		spawner.mu.Unlock()
	}

	// Stop all
	errors := spawner.StopAllAgents()
	if len(errors) > 0 {
		t.Errorf("StopAllAgents returned errors: %v", errors)
	}

	// Verify all are gone from running agents
	if len(spawner.GetRunningAgents()) > 0 {
		t.Error("All agents should be removed from running agents")
	}

	// Verify all were marked stopped
	if len(mockDB.markStoppedCalls) != 3 {
		t.Errorf("Expected 3 MarkStopped calls, got %d", len(mockDB.markStoppedCalls))
	}
}

// TestBuildProjectContext tests project context building
func TestBuildProjectContext(t *testing.T) {
	basePath := t.TempDir()
	spawner := NewSpawner(basePath, "http://localhost:3000/mcp/sse", nil)

	projectPath := basePath
	projectName := "TestProject"
	agentID := "team-test001"
	role := types.RoleGoDeveloper

	// Create CLAUDE.md in project
	claudeContent := "# Project Instructions\n\nThis is a test project."
	os.WriteFile(filepath.Join(basePath, "CLAUDE.md"), []byte(claudeContent), 0644)

	context := spawner.buildProjectContext(projectPath, projectName, role, agentID)

	// Verify key sections
	if !contains(context, "# Project Context") {
		t.Error("Missing project context header")
	}
	if !contains(context, projectName) {
		t.Error("Missing project name")
	}
	if !contains(context, projectPath) {
		t.Error("Missing project path")
	}
	if !contains(context, "## Project Instructions (from CLAUDE.md)") {
		t.Error("Missing CLAUDE.md section")
	}
	if !contains(context, claudeContent) {
		t.Error("Missing CLAUDE.md content")
	}
	if !contains(context, "## Team Context Override") {
		t.Error("Missing team context override")
	}
	if !contains(context, agentID) {
		t.Error("Missing agent ID in team context")
	}
	if !contains(context, "## Access Rules") {
		t.Error("Missing access rules section")
	}
}

// TestBuildProjectContextWithoutClaudeMD tests context when CLAUDE.md doesn't exist
func TestBuildProjectContextWithoutClaudeMD(t *testing.T) {
	basePath := t.TempDir()
	spawner := NewSpawner(basePath, "http://localhost:3000/mcp/sse", nil)

	projectPath := basePath
	projectName := "TestProject"
	agentID := "team-test001"
	role := types.RoleGoDeveloper

	context := spawner.buildProjectContext(projectPath, projectName, role, agentID)

	// Should still have basic sections
	if !contains(context, "# Project Context") {
		t.Error("Missing project context header")
	}
	if !contains(context, projectName) {
		t.Error("Missing project name")
	}
	// Should NOT have CLAUDE.md section
	if contains(context, "## Project Instructions (from CLAUDE.md)") {
		t.Error("Should not have CLAUDE.md section when file doesn't exist")
	}
}

// TestCreateSystemPrompt tests system prompt creation
func TestCreateSystemPrompt(t *testing.T) {
	basePath := t.TempDir()
	spawner := NewSpawner(basePath, "http://localhost:3000/mcp/sse", nil)

	// Create prompts directory and template
	promptsDir := filepath.Join(basePath, "configs", "prompts")
	os.MkdirAll(promptsDir, 0755)

	templateContent := `# System Prompt for {{AGENT_ID}}

Project: {{PROJECT_NAME}}
Path: {{PROJECT_PATH}}

## Project Context
{{PROJECT_CONTEXT}}

## Access Rules
{{ACCESS_RULES}}
`
	templatePath := filepath.Join(promptsDir, "go-developer.md")
	os.WriteFile(templatePath, []byte(templateContent), 0644)

	config := types.AgentConfig{
		Name:  "TestAgent",
		Model: "claude-sonnet-4-5",
		Role:  types.RoleGoDeveloper,
		Color: "#00cc66",
	}

	agentID := "team-test001"
	projectPath := basePath
	projectName := "TestProject"

	promptPath, err := spawner.createSystemPrompt(agentID, config, projectPath, projectName)
	if err != nil {
		t.Fatalf("createSystemPrompt failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(promptPath); os.IsNotExist(err) {
		t.Fatalf("Prompt file not created at %s", promptPath)
	}

	// Read and verify content
	data, err := os.ReadFile(promptPath)
	if err != nil {
		t.Fatalf("Failed to read prompt: %v", err)
	}

	content := string(data)
	// Verify all placeholders were replaced
	if contains(content, "{{AGENT_ID}}") {
		t.Error("AGENT_ID placeholder not replaced")
	}
	if contains(content, "{{PROJECT_NAME}}") {
		t.Error("PROJECT_NAME placeholder not replaced")
	}
	if contains(content, "{{PROJECT_PATH}}") {
		t.Error("PROJECT_PATH placeholder not replaced")
	}
	if contains(content, "{{PROJECT_CONTEXT}}") {
		t.Error("PROJECT_CONTEXT placeholder not replaced")
	}
	if contains(content, "{{ACCESS_RULES}}") {
		t.Error("ACCESS_RULES placeholder not replaced")
	}

	// Verify actual values are present
	if !contains(content, agentID) {
		t.Error("Agent ID not in prompt")
	}
	if !contains(content, projectName) {
		t.Error("Project name not in prompt")
	}
	if !contains(content, projectPath) {
		t.Error("Project path not in prompt")
	}
}

// TestCreateSystemPromptWithCustomPromptFile tests using custom prompt file
func TestCreateSystemPromptWithCustomPromptFile(t *testing.T) {
	basePath := t.TempDir()
	spawner := NewSpawner(basePath, "http://localhost:3000/mcp/sse", nil)

	// Create prompts directory and custom template
	promptsDir := filepath.Join(basePath, "configs", "prompts")
	os.MkdirAll(promptsDir, 0755)

	customContent := "# Custom Prompt for {{AGENT_ID}}"
	customPath := filepath.Join(promptsDir, "custom-prompt.md")
	os.WriteFile(customPath, []byte(customContent), 0644)

	config := types.AgentConfig{
		Name:       "CustomAgent",
		Model:      "claude-opus-4-5",
		Role:       types.RoleCodeAuditor,
		Color:      "#bb66ff",
		PromptFile: "custom-prompt.md", // Override default
	}

	agentID := "team-custom001"
	promptPath, err := spawner.createSystemPrompt(agentID, config, basePath, "TestProject")
	if err != nil {
		t.Fatalf("createSystemPrompt with custom file failed: %v", err)
	}

	// Read and verify it used the custom template
	data, err := os.ReadFile(promptPath)
	if err != nil {
		t.Fatalf("Failed to read prompt: %v", err)
	}

	content := string(data)
	if !contains(content, "Custom Prompt for") {
		t.Error("Should have used custom prompt template")
	}
	if !contains(content, agentID) {
		t.Error("Agent ID not replaced in custom prompt")
	}
}

// TestIsAgentRunning tests process running detection
func TestIsAgentRunning(t *testing.T) {
	spawner := NewSpawner(t.TempDir(), "http://localhost:3000/mcp/sse", nil)

	// Test with current process (should be running)
	currentPID := os.Getpid()
	if !spawner.IsAgentRunning(currentPID) {
		t.Error("Current process should be detected as running")
	}

	// Test with very high PID (unlikely to exist)
	if spawner.IsAgentRunning(99999999) {
		t.Error("Invalid PID should not be detected as running")
	}
}

// TestKillHeartbeatFromPIDFile tests heartbeat cleanup
func TestKillHeartbeatFromPIDFile(t *testing.T) {
	basePath := t.TempDir()
	spawner := NewSpawner(basePath, "http://localhost:3000/mcp/sse", nil)

	// Test with non-existent file (should not error)
	err := spawner.KillHeartbeatFromPIDFile("non-existent-agent")
	if err != nil {
		t.Errorf("Should not error on non-existent heartbeat PID file: %v", err)
	}

	// Test with invalid PID content
	pidsDir := filepath.Join(basePath, "data", "pids")
	os.MkdirAll(pidsDir, 0755)
	pidFile := filepath.Join(pidsDir, "test-agent-heartbeat.pid")
	os.WriteFile(pidFile, []byte("not-a-number"), 0644)

	err = spawner.KillHeartbeatFromPIDFile("test-agent")
	if err == nil {
		t.Error("Should error on invalid PID content")
	}

	// Test with valid but fake PID (process won't exist, but that's OK)
	os.WriteFile(pidFile, []byte("99999999"), 0644)
	err = spawner.KillHeartbeatFromPIDFile("test-agent")
	// Should not error even if kill fails - it's best effort
	if err != nil {
		t.Logf("KillHeartbeatFromPIDFile returned: %v (expected for fake PID)", err)
	}

	// Verify file was cleaned up
	if _, err := os.Stat(pidFile); !os.IsNotExist(err) {
		t.Error("Heartbeat PID file should be removed")
	}
}

// TestSpawnSupervisor tests supervisor spawning (mock test)
func TestSpawnSupervisor(t *testing.T) {
	basePath := t.TempDir()
	mockDB := NewMockMemoryDB()
	spawner := NewSpawner(basePath, "http://localhost:3000/mcp/sse", mockDB)

	// Create necessary directories
	os.MkdirAll(filepath.Join(basePath, "configs", "prompts"), 0755)
	os.MkdirAll(filepath.Join(basePath, "configs", "mcp"), 0755)
	os.MkdirAll(filepath.Join(basePath, "scripts"), 0755)

	// Create dummy prompt template
	promptPath := filepath.Join(basePath, "configs", "prompts", "supervisor.md")
	os.WriteFile(promptPath, []byte("# Supervisor {{AGENT_ID}}"), 0644)

	// Create dummy launcher script (needed for SpawnAgent)
	scriptPath := filepath.Join(basePath, "scripts", "agent-launcher.ps1")
	os.WriteFile(scriptPath, []byte("# Mock launcher"), 0644)

	config := types.AgentConfig{
		Name:  "Supervisor",
		Model: "claude-opus-4-5",
		Role:  types.RoleSupervisor,
		Color: "#ffd700",
	}

	// Note: This will fail to actually spawn because we're not on Windows with PowerShell,
	// but we can test the setup logic
	_, err := spawner.SpawnSupervisor(config)

	// We expect this to fail in test environment (no PowerShell), but config should be created
	if err == nil {
		t.Log("SpawnSupervisor succeeded (unexpected in test environment)")
	} else {
		t.Logf("SpawnSupervisor failed as expected in test: %v", err)
	}

	// Verify MCP config was created
	mcpConfig := filepath.Join(basePath, "configs", "mcp", "Supervisor-mcp.json")
	if _, err := os.Stat(mcpConfig); os.IsNotExist(err) {
		t.Error("MCP config should be created for supervisor")
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
