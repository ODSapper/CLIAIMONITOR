package server

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/CLIAIMONITOR/internal/memory"
	"github.com/CLIAIMONITOR/internal/types"
)

// newTestDB creates a real SQLite database for testing
func newTestDB(t *testing.T) memory.MemoryDB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := memory.NewMemoryDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// mockStore is a simple mock implementation of persistence.Store for testing
// We keep this mock because the Store interface manages in-memory dashboard state,
// not database persistence. Using a real store would require more infrastructure.
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
func (m *mockStore) AddCaptainMessage(msg *types.CaptainMessage)             {}
func (m *mockStore) GetUnreadCaptainMessages() []*types.CaptainMessage       { return nil }
func (m *mockStore) MarkCaptainMessagesRead(ids []string)                    {}
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
func (m *mockStore) SetNATSConnected(connected bool)                         {}
func (m *mockStore) SetCaptainConnected(connected bool)                      {}
func (m *mockStore) SetCaptainStatus(status string)                          {}

func TestNewCleanupService(t *testing.T) {
	db := newTestDB(t)
	store := newMockStore()
	hub := NewHub()

	service := NewCleanupService(db, store, hub)

	if service == nil {
		t.Fatal("NewCleanupService returned nil")
	}

	if service.memDB != db {
		t.Error("memDB not set correctly")
	}

	if service.store != store {
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
	db := newTestDB(t)
	store := newMockStore()
	hub := NewHub()

	// Use UTC to match SQLite's datetime('now') which is UTC
	now := time.Now().UTC()
	oldHeartbeat := now.Add(-180 * time.Second) // 3 minutes ago, beyond 120s threshold
	pid1 := 12345
	pid2 := 67890

	// Register stale agents in the database
	agent1 := &memory.AgentControl{
		AgentID:     "agent-001",
		ConfigName:  "test-config-1",
		PID:         &pid1,
		Status:      "running",
		HeartbeatAt: &oldHeartbeat,
	}
	agent2 := &memory.AgentControl{
		AgentID:     "agent-002",
		ConfigName:  "test-config-2",
		PID:         &pid2,
		Status:      "running",
		HeartbeatAt: &oldHeartbeat,
	}

	if err := db.RegisterAgent(agent1); err != nil {
		t.Fatalf("Failed to register agent1: %v", err)
	}
	if err := db.RegisterAgent(agent2); err != nil {
		t.Fatalf("Failed to register agent2: %v", err)
	}

	// Add agents to store
	store.AddAgent(&types.Agent{ID: "agent-001", Status: types.StatusConnected})
	store.AddAgent(&types.Agent{ID: "agent-002", Status: types.StatusConnected})

	service := NewCleanupService(db, store, hub)

	// Run cleanup once
	removed := service.RunOnce()

	if removed != 2 {
		t.Errorf("expected 2 agents removed, got %d", removed)
	}

	// Verify agents were removed from store
	if len(store.removedAgents) != 2 {
		t.Errorf("expected 2 agents removed from store, got %d", len(store.removedAgents))
	}

	// Verify status was updated to dead in DB
	dbAgent1, err := db.GetAgent("agent-001")
	if err != nil {
		t.Fatalf("Failed to get agent1: %v", err)
	}
	if dbAgent1 != nil && dbAgent1.Status != "dead" {
		t.Errorf("expected agent1 status 'dead', got '%s'", dbAgent1.Status)
	}

	dbAgent2, err := db.GetAgent("agent-002")
	if err != nil {
		t.Fatalf("Failed to get agent2: %v", err)
	}
	if dbAgent2 != nil && dbAgent2.Status != "dead" {
		t.Errorf("expected agent2 status 'dead', got '%s'", dbAgent2.Status)
	}
}

func TestNoStaleAgents(t *testing.T) {
	db := newTestDB(t)
	store := newMockStore()
	hub := NewHub()

	// Register an agent with a recent heartbeat (not stale)
	// Use UTC to match SQLite's datetime('now') which is UTC
	now := time.Now().UTC()
	recentHeartbeat := now // Use current time to ensure it's not stale
	pid := 12345

	agent := &memory.AgentControl{
		AgentID:     "agent-fresh",
		ConfigName:  "test-config",
		PID:         &pid,
		Status:      "running",
		HeartbeatAt: &recentHeartbeat,
	}

	if err := db.RegisterAgent(agent); err != nil {
		t.Fatalf("Failed to register agent: %v", err)
	}

	service := NewCleanupService(db, store, hub)

	// Run cleanup once
	removed := service.RunOnce()

	if removed != 0 {
		t.Errorf("expected 0 agents removed, got %d", removed)
	}

	// Verify no agents were removed from store
	if len(store.removedAgents) != 0 {
		t.Errorf("expected 0 agents removed from store, got %d", len(store.removedAgents))
	}

	// Verify agent still has running status
	dbAgent, err := db.GetAgent("agent-fresh")
	if err != nil {
		t.Fatalf("Failed to get agent: %v", err)
	}
	if dbAgent.Status != "running" {
		t.Errorf("expected status 'running', got '%s'", dbAgent.Status)
	}
}

func TestCleanupWithNullPID(t *testing.T) {
	db := newTestDB(t)
	store := newMockStore()
	hub := NewHub()

	// Use UTC to match SQLite's datetime('now') which is UTC
	now := time.Now().UTC()
	oldHeartbeat := now.Add(-180 * time.Second)

	// Create stale agent with nil PID
	agent := &memory.AgentControl{
		AgentID:     "agent-no-pid",
		ConfigName:  "test-config",
		PID:         nil, // No PID
		Status:      "running",
		HeartbeatAt: &oldHeartbeat,
	}

	if err := db.RegisterAgent(agent); err != nil {
		t.Fatalf("Failed to register agent: %v", err)
	}

	// Add agent to store
	store.AddAgent(&types.Agent{ID: "agent-no-pid", Status: types.StatusConnected})

	service := NewCleanupService(db, store, hub)

	// Should not panic even without PID
	removed := service.RunOnce()

	if removed != 1 {
		t.Errorf("expected 1 agent removed, got %d", removed)
	}

	// Verify agent was removed from store
	if len(store.removedAgents) != 1 {
		t.Errorf("expected 1 agent removed from store, got %d", len(store.removedAgents))
	}

	if store.removedAgents[0] != "agent-no-pid" {
		t.Errorf("expected 'agent-no-pid' removed, got '%s'", store.removedAgents[0])
	}
}

func TestSetIntervals(t *testing.T) {
	db := newTestDB(t)
	store := newMockStore()
	hub := NewHub()

	service := NewCleanupService(db, store, hub)

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
	db := newTestDB(t)
	store := newMockStore()
	hub := NewHub()

	service := NewCleanupService(db, store, hub)

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

func TestCleanupWithMixedAgents(t *testing.T) {
	db := newTestDB(t)
	store := newMockStore()
	hub := NewHub()

	// Use UTC to match SQLite's datetime('now') which is UTC
	now := time.Now().UTC()
	oldHeartbeat := now.Add(-180 * time.Second) // Stale
	recentHeartbeat := now                       // Fresh - use current time
	pid1 := 12345
	pid2 := 67890

	// Register one stale and one fresh agent
	staleAgent := &memory.AgentControl{
		AgentID:     "agent-stale",
		ConfigName:  "test-config",
		PID:         &pid1,
		Status:      "running",
		HeartbeatAt: &oldHeartbeat,
	}
	freshAgent := &memory.AgentControl{
		AgentID:     "agent-fresh",
		ConfigName:  "test-config",
		PID:         &pid2,
		Status:      "running",
		HeartbeatAt: &recentHeartbeat,
	}

	if err := db.RegisterAgent(staleAgent); err != nil {
		t.Fatalf("Failed to register stale agent: %v", err)
	}
	if err := db.RegisterAgent(freshAgent); err != nil {
		t.Fatalf("Failed to register fresh agent: %v", err)
	}

	// Add stale agent to store
	store.AddAgent(&types.Agent{ID: "agent-stale", Status: types.StatusConnected})
	store.AddAgent(&types.Agent{ID: "agent-fresh", Status: types.StatusConnected})

	service := NewCleanupService(db, store, hub)
	removed := service.RunOnce()

	if removed != 1 {
		t.Errorf("expected 1 agent removed (only stale), got %d", removed)
	}

	// Verify only stale agent was removed
	if len(store.removedAgents) != 1 || store.removedAgents[0] != "agent-stale" {
		t.Errorf("expected only 'agent-stale' removed, got %v", store.removedAgents)
	}

	// Verify fresh agent still running
	freshDB, _ := db.GetAgent("agent-fresh")
	if freshDB.Status != "running" {
		t.Errorf("expected fresh agent status 'running', got '%s'", freshDB.Status)
	}
}
