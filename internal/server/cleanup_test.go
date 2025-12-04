package server

import (
	"context"
	"testing"
	"time"

	"github.com/CLIAIMONITOR/internal/memory"
	"github.com/CLIAIMONITOR/internal/types"
)

// mockMemoryDB is a mock implementation of memory.MemoryDB for testing
type mockMemoryDB struct {
	staleAgents     []*memory.AgentControl
	staleAgentsErr  error
	updateStatusErr error
	statusUpdates   []statusUpdate
}

type statusUpdate struct {
	agentID     string
	status      string
	currentTask string
}

func (m *mockMemoryDB) GetStaleAgents(threshold time.Duration) ([]*memory.AgentControl, error) {
	return m.staleAgents, m.staleAgentsErr
}

func (m *mockMemoryDB) UpdateStatus(agentID, status, currentTask string) error {
	m.statusUpdates = append(m.statusUpdates, statusUpdate{
		agentID:     agentID,
		status:      status,
		currentTask: currentTask,
	})
	return m.updateStatusErr
}

// Stub implementations for other MemoryDB methods
func (m *mockMemoryDB) DiscoverRepo(basePath string) (*memory.Repo, error)  { return nil, nil }
func (m *mockMemoryDB) GetRepo(repoID string) (*memory.Repo, error)         { return nil, nil }
func (m *mockMemoryDB) GetRepoByPath(basePath string) (*memory.Repo, error) { return nil, nil }
func (m *mockMemoryDB) UpdateRepoScan(repoID string) error                  { return nil }
func (m *mockMemoryDB) SetRepoRescan(repoID string, needsRescan bool) error { return nil }
func (m *mockMemoryDB) StoreRepoFile(file *memory.RepoFile) error           { return nil }
func (m *mockMemoryDB) GetRepoFiles(repoID string, fileType string) ([]*memory.RepoFile, error) {
	return nil, nil
}
func (m *mockMemoryDB) GetRepoFile(repoID, filePath string) (*memory.RepoFile, error) {
	return nil, nil
}
func (m *mockMemoryDB) StoreAgentLearning(learning *memory.AgentLearning) error { return nil }
func (m *mockMemoryDB) GetAgentLearnings(filter memory.LearnFilter) ([]*memory.AgentLearning, error) {
	return nil, nil
}
func (m *mockMemoryDB) GetRecentLearnings(limit int) ([]*memory.AgentLearning, error) {
	return nil, nil
}
func (m *mockMemoryDB) StoreContextSummary(summary *memory.ContextSummary) error { return nil }
func (m *mockMemoryDB) GetRecentSummaries(limit int) ([]*memory.ContextSummary, error) {
	return nil, nil
}
func (m *mockMemoryDB) GetSummariesByAgent(agentID string, limit int) ([]*memory.ContextSummary, error) {
	return nil, nil
}
func (m *mockMemoryDB) GetSummariesBySession(sessionID string) ([]*memory.ContextSummary, error) {
	return nil, nil
}
func (m *mockMemoryDB) CreateTask(task *memory.WorkflowTask) error          { return nil }
func (m *mockMemoryDB) CreateTasks(tasks []*memory.WorkflowTask) error      { return nil }
func (m *mockMemoryDB) GetTask(taskID string) (*memory.WorkflowTask, error) { return nil, nil }
func (m *mockMemoryDB) GetTasks(filter memory.TaskFilter) ([]*memory.WorkflowTask, error) {
	return nil, nil
}
func (m *mockMemoryDB) UpdateTaskStatus(taskID, status, agentID string) error { return nil }
func (m *mockMemoryDB) UpdateTask(task *memory.WorkflowTask) error            { return nil }
func (m *mockMemoryDB) StoreDecision(decision *memory.HumanDecision) error    { return nil }
func (m *mockMemoryDB) GetRecentDecisions(limit int) ([]*memory.HumanDecision, error) {
	return nil, nil
}
func (m *mockMemoryDB) GetDecisionsByAgent(agentID string, limit int) ([]*memory.HumanDecision, error) {
	return nil, nil
}
func (m *mockMemoryDB) CreateDeployment(deployment *memory.Deployment) error         { return nil }
func (m *mockMemoryDB) GetDeployment(deploymentID int64) (*memory.Deployment, error) { return nil, nil }
func (m *mockMemoryDB) GetRecentDeployments(repoID string, limit int) ([]*memory.Deployment, error) {
	return nil, nil
}
func (m *mockMemoryDB) UpdateDeploymentStatus(deploymentID int64, status string) error { return nil }
func (m *mockMemoryDB) RegisterAgent(agent *memory.AgentControl) error                 { return nil }
func (m *mockMemoryDB) SetShutdownFlag(agentID string, reason string) error            { return nil }
func (m *mockMemoryDB) ClearShutdownFlag(agentID string) error                         { return nil }
func (m *mockMemoryDB) MarkStopped(agentID, reason string) error                       { return nil }
func (m *mockMemoryDB) RemoveAgent(agentID string) error                               { return nil }
func (m *mockMemoryDB) GetAgent(agentID string) (*memory.AgentControl, error)          { return nil, nil }
func (m *mockMemoryDB) GetAllAgents() ([]*memory.AgentControl, error)                  { return nil, nil }
func (m *mockMemoryDB) GetAgentsByStatus(status string) ([]*memory.AgentControl, error) {
	return nil, nil
}
func (m *mockMemoryDB) CheckShutdownFlag(agentID string) (bool, string, error) { return false, "", nil }
func (m *mockMemoryDB) AsLearningDB() memory.LearningDB                        { return nil }
func (m *mockMemoryDB) Close() error                                           { return nil }

// mockStore is a simple mock implementation of persistence.Store for testing
type mockStore struct {
	state         *types.DashboardState
	removedAgents []string
}

func newMockStore() *mockStore {
	return &mockStore{
		state:         types.NewDashboardState(),
		removedAgents: []string{},
	}
}

func (m *mockStore) Load() (*types.DashboardState, error)                   { return m.state, nil }
func (m *mockStore) Save() error                                            { return nil }
func (m *mockStore) GetState() *types.DashboardState                        { return m.state }
func (m *mockStore) ResetMetricsHistory() error                             { return nil }
func (m *mockStore) AddAgent(agent *types.Agent)                            { m.state.Agents[agent.ID] = agent }
func (m *mockStore) UpdateAgent(agentID string, updater func(*types.Agent)) {}
func (m *mockStore) RemoveAgent(agentID string) {
	m.removedAgents = append(m.removedAgents, agentID)
	delete(m.state.Agents, agentID)
}
func (m *mockStore) GetAgent(agentID string) *types.Agent                       { return m.state.Agents[agentID] }
func (m *mockStore) RequestAgentShutdown(agentID string, requestTime time.Time) {}
func (m *mockStore) UpdateMetrics(agentID string, metrics *types.AgentMetrics)  {}
func (m *mockStore) GetMetrics(agentID string) *types.AgentMetrics              { return nil }
func (m *mockStore) TakeMetricsSnapshot()                                       {}
func (m *mockStore) AddHumanRequest(req *types.HumanInputRequest)               {}
func (m *mockStore) AnswerHumanRequest(id string, answer string)                {}
func (m *mockStore) GetPendingRequests() []*types.HumanInputRequest             { return nil }
func (m *mockStore) AddStopRequest(req *types.StopApprovalRequest)              {}
func (m *mockStore) RespondStopRequest(id string, approved bool, response string, reviewedBy string) {
}
func (m *mockStore) GetPendingStopRequests() []*types.StopApprovalRequest    { return nil }
func (m *mockStore) GetStopRequestByID(id string) *types.StopApprovalRequest { return nil }
func (m *mockStore) AddAlert(alert *types.Alert)                             {}
func (m *mockStore) AcknowledgeAlert(id string)                              {}
func (m *mockStore) ClearAllAlerts()                                         {}
func (m *mockStore) GetActiveAlerts() []*types.Alert                         { return nil }
func (m *mockStore) AddActivity(activity *types.ActivityLog)                 {}
func (m *mockStore) AddJudgment(judgment *types.SupervisorJudgment)          {}
func (m *mockStore) GetNextAgentNumber(configName string) int                { return 1 }
func (m *mockStore) RecordHumanCheckin()                                     {}
func (m *mockStore) GetLastHumanCheckin() time.Time                          { return time.Time{} }
func (m *mockStore) SetThresholds(thresholds types.AlertThresholds)          {}
func (m *mockStore) GetThresholds() types.AlertThresholds                    { return types.AlertThresholds{} }
func (m *mockStore) CleanupStaleAgents() int                                 { return 0 }

func TestNewCleanupService(t *testing.T) {
	mockDB := &mockMemoryDB{}
	mockStore := newMockStore()
	hub := NewHub()

	service := NewCleanupService(mockDB, mockStore, hub)

	if service == nil {
		t.Fatal("NewCleanupService returned nil")
	}

	if service.memDB != mockDB {
		t.Error("memDB not set correctly")
	}

	if service.store != mockStore {
		t.Error("store not set correctly")
	}

	if service.hub != hub {
		t.Error("hub not set correctly")
	}

	// Verify default intervals
	if service.checkInterval != 30*time.Second {
		t.Errorf("expected checkInterval 30s, got %v", service.checkInterval)
	}

	if service.staleThreshold != 120*time.Second {
		t.Errorf("expected staleThreshold 120s, got %v", service.staleThreshold)
	}
}

func TestCleanupStaleAgents(t *testing.T) {
	now := time.Now()
	oldHeartbeat := now.Add(-180 * time.Second)
	pid1 := 12345
	pid2 := 67890

	// Create mock stale agents
	staleAgents := []*memory.AgentControl{
		{
			AgentID:     "agent-001",
			ConfigName:  "test-config-1",
			PID:         &pid1,
			Status:      "running",
			HeartbeatAt: &oldHeartbeat,
		},
		{
			AgentID:     "agent-002",
			ConfigName:  "test-config-2",
			PID:         &pid2,
			Status:      "running",
			HeartbeatAt: &oldHeartbeat,
		},
	}

	mockDB := &mockMemoryDB{
		staleAgents: staleAgents,
	}
	mockStore := newMockStore()
	hub := NewHub()

	// Add agents to store first
	mockStore.AddAgent(&types.Agent{ID: "agent-001", Status: types.StatusConnected})
	mockStore.AddAgent(&types.Agent{ID: "agent-002", Status: types.StatusConnected})

	service := NewCleanupService(mockDB, mockStore, hub)

	// Run cleanup once
	removed := service.RunOnce()

	if removed != 2 {
		t.Errorf("expected 2 agents removed, got %d", removed)
	}

	// Verify agents were removed from store
	if len(mockStore.removedAgents) != 2 {
		t.Errorf("expected 2 agents removed from store, got %d", len(mockStore.removedAgents))
	}

	// Verify status was updated to dead in DB
	if len(mockDB.statusUpdates) != 2 {
		t.Errorf("expected 2 status updates, got %d", len(mockDB.statusUpdates))
	}

	for _, update := range mockDB.statusUpdates {
		if update.status != "dead" {
			t.Errorf("expected status 'dead', got '%s'", update.status)
		}
	}
}

func TestNoStaleAgents(t *testing.T) {
	mockDB := &mockMemoryDB{
		staleAgents: []*memory.AgentControl{}, // Empty list
	}
	mockStore := newMockStore()
	hub := NewHub()

	service := NewCleanupService(mockDB, mockStore, hub)

	// Run cleanup once
	removed := service.RunOnce()

	if removed != 0 {
		t.Errorf("expected 0 agents removed, got %d", removed)
	}

	// Verify no agents were removed from store
	if len(mockStore.removedAgents) != 0 {
		t.Errorf("expected 0 agents removed from store, got %d", len(mockStore.removedAgents))
	}

	// Verify no status updates
	if len(mockDB.statusUpdates) != 0 {
		t.Errorf("expected 0 status updates, got %d", len(mockDB.statusUpdates))
	}
}

func TestCleanupWithNullPID(t *testing.T) {
	now := time.Now()
	oldHeartbeat := now.Add(-180 * time.Second)

	// Create stale agent with nil PID
	staleAgents := []*memory.AgentControl{
		{
			AgentID:     "agent-no-pid",
			ConfigName:  "test-config",
			PID:         nil, // No PID
			Status:      "running",
			HeartbeatAt: &oldHeartbeat,
		},
	}

	mockDB := &mockMemoryDB{
		staleAgents: staleAgents,
	}
	mockStore := newMockStore()
	hub := NewHub()

	// Add agent to store
	mockStore.AddAgent(&types.Agent{ID: "agent-no-pid", Status: types.StatusConnected})

	service := NewCleanupService(mockDB, mockStore, hub)

	// Should not panic even without PID
	removed := service.RunOnce()

	if removed != 1 {
		t.Errorf("expected 1 agent removed, got %d", removed)
	}

	// Verify agent was still removed from store
	if len(mockStore.removedAgents) != 1 {
		t.Errorf("expected 1 agent removed from store, got %d", len(mockStore.removedAgents))
	}

	if mockStore.removedAgents[0] != "agent-no-pid" {
		t.Errorf("expected 'agent-no-pid' removed, got '%s'", mockStore.removedAgents[0])
	}
}

func TestSetIntervals(t *testing.T) {
	mockDB := &mockMemoryDB{}
	mockStore := newMockStore()
	hub := NewHub()

	service := NewCleanupService(mockDB, mockStore, hub)

	// Change intervals
	newCheckInterval := 10 * time.Second
	newStaleThreshold := 60 * time.Second

	service.SetIntervals(newCheckInterval, newStaleThreshold)

	if service.checkInterval != newCheckInterval {
		t.Errorf("expected checkInterval %v, got %v", newCheckInterval, service.checkInterval)
	}

	if service.staleThreshold != newStaleThreshold {
		t.Errorf("expected staleThreshold %v, got %v", newStaleThreshold, service.staleThreshold)
	}
}

func TestCleanupServiceStart(t *testing.T) {
	mockDB := &mockMemoryDB{
		staleAgents: []*memory.AgentControl{},
	}
	mockStore := newMockStore()
	hub := NewHub()

	service := NewCleanupService(mockDB, mockStore, hub)

	// Set very short check interval for testing
	service.SetIntervals(50*time.Millisecond, 120*time.Second)

	// Start service with context
	ctx, cancel := context.WithCancel(context.Background())

	// Run in background
	go service.Start(ctx)

	// Let it run a couple cycles
	time.Sleep(150 * time.Millisecond)

	// Cancel context
	cancel()

	// Give it time to stop
	time.Sleep(50 * time.Millisecond)

	// Test passes if no panic occurred
}

func TestCleanupDBError(t *testing.T) {
	mockDB := &mockMemoryDB{
		staleAgentsErr: context.DeadlineExceeded, // Simulate error
	}
	mockStore := newMockStore()
	hub := NewHub()

	service := NewCleanupService(mockDB, mockStore, hub)

	// Should handle error gracefully
	removed := service.RunOnce()

	if removed != 0 {
		t.Errorf("expected 0 agents removed on error, got %d", removed)
	}
}

func TestCleanupUpdateStatusError(t *testing.T) {
	now := time.Now()
	oldHeartbeat := now.Add(-180 * time.Second)
	pid := 12345

	staleAgents := []*memory.AgentControl{
		{
			AgentID:     "agent-001",
			ConfigName:  "test-config",
			PID:         &pid,
			Status:      "running",
			HeartbeatAt: &oldHeartbeat,
		},
	}

	mockDB := &mockMemoryDB{
		staleAgents:     staleAgents,
		updateStatusErr: context.DeadlineExceeded, // Simulate error on update
	}
	mockStore := newMockStore()
	hub := NewHub()

	mockStore.AddAgent(&types.Agent{ID: "agent-001", Status: types.StatusConnected})

	service := NewCleanupService(mockDB, mockStore, hub)

	// Should still remove from store even if DB update fails
	removed := service.RunOnce()

	if removed != 1 {
		t.Errorf("expected 1 agent removed, got %d", removed)
	}

	// Verify agent was still removed from store
	if len(mockStore.removedAgents) != 1 {
		t.Errorf("expected 1 agent removed from store, got %d", len(mockStore.removedAgents))
	}
}
