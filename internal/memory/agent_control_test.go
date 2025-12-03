package memory

import (
	"testing"
	"time"
)

// TestRegisterAgent tests agent registration
func TestRegisterAgent(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	agent := &AgentControl{
		AgentID:    "test-agent-001",
		ConfigName: "test-config",
		Role:       "tester",
		Status:     "starting",
		Model:      "claude-sonnet-4",
		Color:      "green",
	}

	err := db.RegisterAgent(agent)
	if err != nil {
		t.Fatalf("RegisterAgent failed: %v", err)
	}

	// Retrieve and verify
	retrieved, err := db.GetAgent("test-agent-001")
	if err != nil {
		t.Fatalf("GetAgent failed: %v", err)
	}

	if retrieved.AgentID != "test-agent-001" {
		t.Errorf("Expected AgentID test-agent-001, got %s", retrieved.AgentID)
	}

	if retrieved.ConfigName != "test-config" {
		t.Errorf("Expected ConfigName test-config, got %s", retrieved.ConfigName)
	}

	if retrieved.Role != "tester" {
		t.Errorf("Expected Role tester, got %s", retrieved.Role)
	}

	if retrieved.Status != "starting" {
		t.Errorf("Expected Status starting, got %s", retrieved.Status)
	}
}

// TestRegisterAgentWithPID tests agent registration with PID
func TestRegisterAgentWithPID(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	pid := 12345
	agent := &AgentControl{
		AgentID:    "test-agent-002",
		ConfigName: "test-config",
		PID:        &pid,
		Status:     "running",
	}

	err := db.RegisterAgent(agent)
	if err != nil {
		t.Fatalf("RegisterAgent failed: %v", err)
	}

	// Retrieve and verify PID
	retrieved, err := db.GetAgent("test-agent-002")
	if err != nil {
		t.Fatalf("GetAgent failed: %v", err)
	}

	if retrieved.PID == nil {
		t.Error("Expected PID to be set, got nil")
	} else if *retrieved.PID != 12345 {
		t.Errorf("Expected PID 12345, got %d", *retrieved.PID)
	}
}

// TestUpdateHeartbeat tests heartbeat updates
func TestUpdateHeartbeat(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Register agent
	agent := &AgentControl{
		AgentID:    "test-agent-003",
		ConfigName: "test-config",
		Status:     "running",
	}

	err := db.RegisterAgent(agent)
	if err != nil {
		t.Fatalf("RegisterAgent failed: %v", err)
	}

	// Wait a moment to ensure timestamp difference
	time.Sleep(10 * time.Millisecond)

	// Update heartbeat
	err = db.UpdateHeartbeat("test-agent-003")
	if err != nil {
		t.Fatalf("UpdateHeartbeat failed: %v", err)
	}

	// Verify heartbeat was updated
	retrieved, err := db.GetAgent("test-agent-003")
	if err != nil {
		t.Fatalf("GetAgent failed: %v", err)
	}

	if retrieved.HeartbeatAt == nil {
		t.Error("Expected HeartbeatAt to be set")
	}

	// Update again to verify it can be updated multiple times
	firstHeartbeat := *retrieved.HeartbeatAt

	// Sleep longer to ensure timestamp difference in SQLite (1-second precision)
	time.Sleep(1 * time.Second)

	err = db.UpdateHeartbeat("test-agent-003")
	if err != nil {
		t.Fatalf("Second UpdateHeartbeat failed: %v", err)
	}

	retrieved, err = db.GetAgent("test-agent-003")
	if err != nil {
		t.Fatalf("GetAgent failed: %v", err)
	}

	// With SQLite's 1-second precision, this should now be different
	if retrieved.HeartbeatAt.Before(firstHeartbeat) {
		t.Errorf("Expected heartbeat not to go backwards. First: %v, Second: %v", firstHeartbeat, *retrieved.HeartbeatAt)
	}
}

// TestUpdateStatus tests status updates
func TestUpdateStatus(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Register agent
	agent := &AgentControl{
		AgentID:    "test-agent-004",
		ConfigName: "test-config",
		Status:     "starting",
	}

	err := db.RegisterAgent(agent)
	if err != nil {
		t.Fatalf("RegisterAgent failed: %v", err)
	}

	// Update status
	err = db.UpdateStatus("test-agent-004", "running", "Processing task ABC")
	if err != nil {
		t.Fatalf("UpdateStatus failed: %v", err)
	}

	// Verify update
	retrieved, err := db.GetAgent("test-agent-004")
	if err != nil {
		t.Fatalf("GetAgent failed: %v", err)
	}

	if retrieved.Status != "running" {
		t.Errorf("Expected Status running, got %s", retrieved.Status)
	}

	if retrieved.CurrentTask != "Processing task ABC" {
		t.Errorf("Expected CurrentTask 'Processing task ABC', got '%s'", retrieved.CurrentTask)
	}

	// Verify heartbeat was also updated
	if retrieved.HeartbeatAt == nil {
		t.Error("Expected HeartbeatAt to be updated")
	}
}

// TestShutdownFlag tests shutdown flag operations
func TestShutdownFlag(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Register agent
	agent := &AgentControl{
		AgentID:    "test-agent-005",
		ConfigName: "test-config",
		Status:     "running",
	}

	err := db.RegisterAgent(agent)
	if err != nil {
		t.Fatalf("RegisterAgent failed: %v", err)
	}

	// Check initial flag (should be false)
	isSet, reason, err := db.CheckShutdownFlag("test-agent-005")
	if err != nil {
		t.Fatalf("CheckShutdownFlag failed: %v", err)
	}

	if isSet {
		t.Error("Expected shutdown flag to be false initially")
	}

	if reason != "" {
		t.Errorf("Expected empty reason, got '%s'", reason)
	}

	// Set shutdown flag
	err = db.SetShutdownFlag("test-agent-005", "cleanup requested")
	if err != nil {
		t.Fatalf("SetShutdownFlag failed: %v", err)
	}

	// Verify flag is set
	isSet, reason, err = db.CheckShutdownFlag("test-agent-005")
	if err != nil {
		t.Fatalf("CheckShutdownFlag failed: %v", err)
	}

	if !isSet {
		t.Error("Expected shutdown flag to be true")
	}

	if reason != "cleanup requested" {
		t.Errorf("Expected reason 'cleanup requested', got '%s'", reason)
	}

	// Clear shutdown flag
	err = db.ClearShutdownFlag("test-agent-005")
	if err != nil {
		t.Fatalf("ClearShutdownFlag failed: %v", err)
	}

	// Verify flag is cleared
	isSet, reason, err = db.CheckShutdownFlag("test-agent-005")
	if err != nil {
		t.Fatalf("CheckShutdownFlag failed: %v", err)
	}

	if isSet {
		t.Error("Expected shutdown flag to be false after clear")
	}

	if reason != "" {
		t.Errorf("Expected empty reason after clear, got '%s'", reason)
	}
}

// TestMarkStopped tests marking an agent as stopped
func TestMarkStopped(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Register agent
	agent := &AgentControl{
		AgentID:    "test-agent-006",
		ConfigName: "test-config",
		Status:     "running",
	}

	err := db.RegisterAgent(agent)
	if err != nil {
		t.Fatalf("RegisterAgent failed: %v", err)
	}

	// Mark as stopped
	err = db.MarkStopped("test-agent-006", "task completed")
	if err != nil {
		t.Fatalf("MarkStopped failed: %v", err)
	}

	// Verify stopped status
	retrieved, err := db.GetAgent("test-agent-006")
	if err != nil {
		t.Fatalf("GetAgent failed: %v", err)
	}

	if retrieved.Status != "stopped" {
		t.Errorf("Expected Status stopped, got %s", retrieved.Status)
	}

	if retrieved.StopReason != "task completed" {
		t.Errorf("Expected StopReason 'task completed', got '%s'", retrieved.StopReason)
	}

	if retrieved.StoppedAt == nil {
		t.Error("Expected StoppedAt to be set")
	}
}

// TestRemoveAgent tests agent removal
func TestRemoveAgent(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Register agent
	agent := &AgentControl{
		AgentID:    "test-agent-007",
		ConfigName: "test-config",
		Status:     "stopped",
	}

	err := db.RegisterAgent(agent)
	if err != nil {
		t.Fatalf("RegisterAgent failed: %v", err)
	}

	// Verify it exists
	_, err = db.GetAgent("test-agent-007")
	if err != nil {
		t.Fatalf("GetAgent failed: %v", err)
	}

	// Remove agent
	err = db.RemoveAgent("test-agent-007")
	if err != nil {
		t.Fatalf("RemoveAgent failed: %v", err)
	}

	// Verify it's gone
	_, err = db.GetAgent("test-agent-007")
	if err == nil {
		t.Error("Expected error when getting removed agent")
	}
}

// TestGetAllAgents tests retrieving all agents
func TestGetAllAgents(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Register multiple agents
	agents := []*AgentControl{
		{AgentID: "agent-001", ConfigName: "config-1", Status: "running"},
		{AgentID: "agent-002", ConfigName: "config-2", Status: "idle"},
		{AgentID: "agent-003", ConfigName: "config-3", Status: "stopped"},
	}

	for _, agent := range agents {
		if err := db.RegisterAgent(agent); err != nil {
			t.Fatalf("RegisterAgent failed: %v", err)
		}
	}

	// Retrieve all agents
	allAgents, err := db.GetAllAgents()
	if err != nil {
		t.Fatalf("GetAllAgents failed: %v", err)
	}

	if len(allAgents) != 3 {
		t.Errorf("Expected 3 agents, got %d", len(allAgents))
	}

	// Verify all agents are present (ordering may vary due to timestamp precision)
	foundAgents := make(map[string]bool)
	for _, agent := range allAgents {
		foundAgents[agent.AgentID] = true
	}

	expectedIDs := []string{"agent-001", "agent-002", "agent-003"}
	for _, id := range expectedIDs {
		if !foundAgents[id] {
			t.Errorf("Expected to find agent %s in results", id)
		}
	}
}

// TestGetAgentsByStatus tests retrieving agents by status
func TestGetAgentsByStatus(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Register multiple agents with different statuses
	agents := []*AgentControl{
		{AgentID: "agent-001", ConfigName: "config-1", Status: "running"},
		{AgentID: "agent-002", ConfigName: "config-2", Status: "running"},
		{AgentID: "agent-003", ConfigName: "config-3", Status: "stopped"},
		{AgentID: "agent-004", ConfigName: "config-4", Status: "idle"},
	}

	for _, agent := range agents {
		if err := db.RegisterAgent(agent); err != nil {
			t.Fatalf("RegisterAgent failed: %v", err)
		}
	}

	// Get running agents
	runningAgents, err := db.GetAgentsByStatus("running")
	if err != nil {
		t.Fatalf("GetAgentsByStatus failed: %v", err)
	}

	if len(runningAgents) != 2 {
		t.Errorf("Expected 2 running agents, got %d", len(runningAgents))
	}

	// Get stopped agents
	stoppedAgents, err := db.GetAgentsByStatus("stopped")
	if err != nil {
		t.Fatalf("GetAgentsByStatus failed: %v", err)
	}

	if len(stoppedAgents) != 1 {
		t.Errorf("Expected 1 stopped agent, got %d", len(stoppedAgents))
	}

	// Get idle agents
	idleAgents, err := db.GetAgentsByStatus("idle")
	if err != nil {
		t.Fatalf("GetAgentsByStatus failed: %v", err)
	}

	if len(idleAgents) != 1 {
		t.Errorf("Expected 1 idle agent, got %d", len(idleAgents))
	}
}

// TestGetStaleAgents tests retrieving stale agents
func TestGetStaleAgents(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Register agents with different heartbeat times
	now := time.Now()

	// Fresh agent (just registered)
	fresh := &AgentControl{
		AgentID:    "agent-fresh",
		ConfigName: "config-1",
		Status:     "running",
	}
	if err := db.RegisterAgent(fresh); err != nil {
		t.Fatalf("RegisterAgent failed: %v", err)
	}
	if err := db.UpdateHeartbeat("agent-fresh"); err != nil {
		t.Fatalf("UpdateHeartbeat failed: %v", err)
	}

	// Stale agent (old heartbeat)
	stale := &AgentControl{
		AgentID:    "agent-stale",
		ConfigName: "config-2",
		Status:     "running",
	}
	oldHeartbeat := now.Add(-5 * time.Minute)
	stale.HeartbeatAt = &oldHeartbeat

	if err := db.RegisterAgent(stale); err != nil {
		t.Fatalf("RegisterAgent failed: %v", err)
	}

	// Very stale agent
	veryStale := &AgentControl{
		AgentID:    "agent-very-stale",
		ConfigName: "config-3",
		Status:     "running",
	}
	veryOldHeartbeat := now.Add(-10 * time.Minute)
	veryStale.HeartbeatAt = &veryOldHeartbeat

	if err := db.RegisterAgent(veryStale); err != nil {
		t.Fatalf("RegisterAgent failed: %v", err)
	}

	// Agent with no heartbeat
	noHeartbeat := &AgentControl{
		AgentID:    "agent-no-heartbeat",
		ConfigName: "config-4",
		Status:     "running",
	}
	if err := db.RegisterAgent(noHeartbeat); err != nil {
		t.Fatalf("RegisterAgent failed: %v", err)
	}

	// Stopped agent (should not be included even if stale)
	stopped := &AgentControl{
		AgentID:    "agent-stopped",
		ConfigName: "config-5",
		Status:     "stopped",
	}
	stoppedHeartbeat := now.Add(-10 * time.Minute)
	stopped.HeartbeatAt = &stoppedHeartbeat

	if err := db.RegisterAgent(stopped); err != nil {
		t.Fatalf("RegisterAgent failed: %v", err)
	}

	// Stuck starting agent (no heartbeat, old spawn time) - should be stale
	stuckStarting := &AgentControl{
		AgentID:    "agent-stuck-starting",
		ConfigName: "config-6",
		Status:     "starting",
	}
	if err := db.RegisterAgent(stuckStarting); err != nil {
		t.Fatalf("RegisterAgent failed: %v", err)
	}
	// Manually set spawned_at to old time via direct SQL
	_, err := db.(*SQLiteMemoryDB).db.Exec(
		"UPDATE agent_control SET spawned_at = datetime('now', '-5 minutes') WHERE agent_id = ?",
		"agent-stuck-starting",
	)
	if err != nil {
		t.Fatalf("Failed to set old spawned_at: %v", err)
	}

	// Fresh starting agent (just spawned) - should NOT be stale
	freshStarting := &AgentControl{
		AgentID:    "agent-fresh-starting",
		ConfigName: "config-7",
		Status:     "starting",
	}
	if err := db.RegisterAgent(freshStarting); err != nil {
		t.Fatalf("RegisterAgent failed: %v", err)
	}

	// Get stale agents (threshold: 2 minutes)
	threshold := 2 * time.Minute
	staleAgents, err := db.GetStaleAgents(threshold)
	if err != nil {
		t.Fatalf("GetStaleAgents failed: %v", err)
	}

	// Should find 3 stale agents:
	// - agent-very-stale (old heartbeat)
	// - agent-stale (old heartbeat)
	// - agent-stuck-starting (no heartbeat, old spawn time)
	// Should NOT include:
	// - agent-fresh (recent heartbeat)
	// - agent-no-heartbeat (running status, not starting)
	// - agent-stopped (stopped status)
	// - agent-fresh-starting (just spawned)
	if len(staleAgents) != 3 {
		t.Errorf("Expected 3 stale agents, got %d", len(staleAgents))
		for _, a := range staleAgents {
			t.Logf("  Stale agent: %s (status=%s)", a.AgentID, a.Status)
		}
	}

	// Verify stuck-starting agent is included
	foundStuckStarting := false
	for _, a := range staleAgents {
		if a.AgentID == "agent-stuck-starting" {
			foundStuckStarting = true
			break
		}
	}
	if !foundStuckStarting {
		t.Error("Expected agent-stuck-starting to be in stale agents list")
	}
}

// TestUpdateNonExistentAgent tests error handling for non-existent agents
func TestUpdateNonExistentAgent(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Try to update heartbeat for non-existent agent
	err := db.UpdateHeartbeat("non-existent-agent")
	if err == nil {
		t.Error("Expected error when updating non-existent agent")
	}

	// Try to update status for non-existent agent
	err = db.UpdateStatus("non-existent-agent", "running", "task")
	if err == nil {
		t.Error("Expected error when updating status of non-existent agent")
	}

	// Try to set shutdown flag for non-existent agent
	err = db.SetShutdownFlag("non-existent-agent", "reason")
	if err == nil {
		t.Error("Expected error when setting shutdown flag for non-existent agent")
	}
}

// TestAgentControlUpsert tests that RegisterAgent can update existing agents
func TestAgentControlUpsert(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Register initial agent
	agent := &AgentControl{
		AgentID:    "test-agent-upsert",
		ConfigName: "config-v1",
		Status:     "starting",
		Role:       "initial-role",
	}

	err := db.RegisterAgent(agent)
	if err != nil {
		t.Fatalf("RegisterAgent failed: %v", err)
	}

	// Register again with updated fields (upsert)
	updatedAgent := &AgentControl{
		AgentID:    "test-agent-upsert",
		ConfigName: "config-v2",
		Status:     "running",
		Role:       "updated-role",
	}

	err = db.RegisterAgent(updatedAgent)
	if err != nil {
		t.Fatalf("RegisterAgent (upsert) failed: %v", err)
	}

	// Verify updated values
	retrieved, err := db.GetAgent("test-agent-upsert")
	if err != nil {
		t.Fatalf("GetAgent failed: %v", err)
	}

	if retrieved.ConfigName != "config-v2" {
		t.Errorf("Expected ConfigName config-v2, got %s", retrieved.ConfigName)
	}

	if retrieved.Status != "running" {
		t.Errorf("Expected Status running, got %s", retrieved.Status)
	}

	if retrieved.Role != "updated-role" {
		t.Errorf("Expected Role updated-role, got %s", retrieved.Role)
	}
}
