package memory

import (
	"testing"
	"time"
)

func TestPaneTracking(t *testing.T) {
	db, err := NewMemoryDB(":memory:")
	if err != nil {
		t.Fatalf("failed to create memory db: %v", err)
	}
	defer db.Close()

	// Register an agent
	agent := &AgentControl{
		AgentID:    "test-agent-1",
		ConfigName: "SNTGreen",
		Role:       "tester",
		Status:     "starting",
		Model:      "claude-sonnet-4.5",
		Color:      "green",
	}

	err = db.RegisterAgent(agent)
	if err != nil {
		t.Fatalf("failed to register agent: %v", err)
	}

	// Update with pane ID
	paneID := "wezterm-pane-12345"
	err = db.UpdateAgentPaneID(agent.AgentID, paneID)
	if err != nil {
		t.Fatalf("failed to update pane ID: %v", err)
	}

	// Retrieve agent and verify pane ID
	retrieved, err := db.GetAgent(agent.AgentID)
	if err != nil {
		t.Fatalf("failed to get agent: %v", err)
	}

	if retrieved.PaneID != paneID {
		t.Errorf("expected pane ID %s, got %s", paneID, retrieved.PaneID)
	}

	// Test GetAgentByPaneID
	agentByPane, err := db.GetAgentByPaneID(paneID)
	if err != nil {
		t.Fatalf("failed to get agent by pane ID: %v", err)
	}

	if agentByPane.AgentID != agent.AgentID {
		t.Errorf("expected agent ID %s, got %s", agent.AgentID, agentByPane.AgentID)
	}

	// Log pane events
	err = db.LogPaneEvent(agent.AgentID, paneID, "spawned", "", "starting", "Agent spawned successfully")
	if err != nil {
		t.Fatalf("failed to log pane event: %v", err)
	}

	// Sleep to ensure different timestamps
	time.Sleep(10 * time.Millisecond)

	err = db.LogPaneEvent(agent.AgentID, paneID, "closed", "idle", "stopped", "User closed pane")
	if err != nil {
		t.Fatalf("failed to log pane event: %v", err)
	}

	// Retrieve pane history
	history, err := db.GetPaneHistory(agent.AgentID, 10)
	if err != nil {
		t.Fatalf("failed to get pane history: %v", err)
	}

	if len(history) != 2 {
		t.Errorf("expected 2 history entries, got %d", len(history))
	}

	// Verify latest event is first (DESC order)
	if history[0].Action != "closed" {
		t.Errorf("expected first event to be 'closed', got '%s'", history[0].Action)
	}

	if history[1].Action != "spawned" {
		t.Errorf("expected second event to be 'spawned', got '%s'", history[1].Action)
	}

	// Verify event details
	if history[0].StatusBefore != "idle" {
		t.Errorf("expected status_before 'idle', got '%s'", history[0].StatusBefore)
	}

	if history[0].StatusAfter != "stopped" {
		t.Errorf("expected status_after 'stopped', got '%s'", history[0].StatusAfter)
	}

	if history[0].Details != "User closed pane" {
		t.Errorf("expected details 'User closed pane', got '%s'", history[0].Details)
	}

	t.Log("Pane tracking test completed successfully")
}

func TestPaneTrackingNotFound(t *testing.T) {
	db, err := NewMemoryDB(":memory:")
	if err != nil {
		t.Fatalf("failed to create memory db: %v", err)
	}
	defer db.Close()

	// Try to get agent by non-existent pane ID
	_, err = db.GetAgentByPaneID("non-existent-pane")
	if err == nil {
		t.Error("expected error when getting agent by non-existent pane ID")
	}

	// Try to update pane ID for non-existent agent
	err = db.UpdateAgentPaneID("non-existent-agent", "some-pane")
	if err == nil {
		t.Error("expected error when updating pane ID for non-existent agent")
	}

	t.Log("Pane tracking not-found test completed successfully")
}

func TestPaneHistoryEmpty(t *testing.T) {
	db, err := NewMemoryDB(":memory:")
	if err != nil {
		t.Fatalf("failed to create memory db: %v", err)
	}
	defer db.Close()

	// Register an agent
	agent := &AgentControl{
		AgentID:    "test-agent-no-history",
		ConfigName: "SNTGreen",
		Status:     "starting",
	}

	err = db.RegisterAgent(agent)
	if err != nil {
		t.Fatalf("failed to register agent: %v", err)
	}

	// Get history for agent with no events
	history, err := db.GetPaneHistory(agent.AgentID, 10)
	if err != nil {
		t.Fatalf("failed to get pane history: %v", err)
	}

	if len(history) != 0 {
		t.Errorf("expected empty history, got %d entries", len(history))
	}

	t.Log("Pane history empty test completed successfully")
}
