package persistence

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/CLIAIMONITOR/internal/types"
)

func TestNewJSONStore(t *testing.T) {
	store := NewJSONStore("/tmp/test-state.json")
	if store == nil {
		t.Fatal("NewJSONStore returned nil")
	}
	if store.filepath != "/tmp/test-state.json" {
		t.Errorf("filepath = %q, want %q", store.filepath, "/tmp/test-state.json")
	}
}

func TestLoadEmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "data", "state.json")

	store := NewJSONStore(storePath)
	state, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if state == nil {
		t.Fatal("Load() returned nil state")
	}
	if len(state.Agents) != 0 {
		t.Errorf("expected empty Agents map, got %d agents", len(state.Agents))
	}
	if state.Thresholds.FailedTestsMax != 5 {
		t.Errorf("expected default FailedTestsMax = 5, got %d", state.Thresholds.FailedTestsMax)
	}
}

func TestLoadExistingFile(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "state.json")

	// Write test state
	testJSON := `{
		"agents": {
			"TestAgent": {
				"id": "TestAgent",
				"role": "Engineer",
				"status": "working"
			}
		},
		"metrics": {},
		"human_requests": {},
		"thresholds": {
			"failed_tests_max": 10
		}
	}`
	if err := os.WriteFile(storePath, []byte(testJSON), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	store := NewJSONStore(storePath)
	state, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(state.Agents) != 1 {
		t.Errorf("expected 1 agent, got %d", len(state.Agents))
	}
	if state.Thresholds.FailedTestsMax != 10 {
		t.Errorf("expected FailedTestsMax = 10, got %d", state.Thresholds.FailedTestsMax)
	}
}

func TestSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "state.json")

	store := NewJSONStore(storePath)
	store.Load()

	// Add an agent
	agent := &types.Agent{
		ID:     "Agent001",
		Role:   types.RoleEngineer,
		Status: types.StatusWorking,
	}
	store.AddAgent(agent)

	// Save
	if err := store.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Load in new store instance
	store2 := NewJSONStore(storePath)
	state, err := store2.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if state.Agents["Agent001"] == nil {
		t.Error("expected Agent001 to be persisted")
	}
}

func TestAddAgent(t *testing.T) {
	store := NewJSONStore(filepath.Join(t.TempDir(), "state.json"))
	store.Load()

	agent := &types.Agent{
		ID:     "TestAgent",
		Role:   types.RoleGoDeveloper,
		Status: types.StatusConnected,
		Model:  "claude-sonnet-4-5-20250929",
	}
	store.AddAgent(agent)

	retrieved := store.GetAgent("TestAgent")
	if retrieved == nil {
		t.Fatal("GetAgent returned nil")
	}
	if retrieved.Role != types.RoleGoDeveloper {
		t.Errorf("Role = %v, want %v", retrieved.Role, types.RoleGoDeveloper)
	}
}

func TestUpdateAgent(t *testing.T) {
	store := NewJSONStore(filepath.Join(t.TempDir(), "state.json"))
	store.Load()

	agent := &types.Agent{
		ID:          "TestAgent",
		Status:      types.StatusConnected,
		CurrentTask: "Initial task",
	}
	store.AddAgent(agent)

	store.UpdateAgent("TestAgent", func(a *types.Agent) {
		a.Status = types.StatusWorking
		a.CurrentTask = "Updated task"
	})

	retrieved := store.GetAgent("TestAgent")
	if retrieved.Status != types.StatusWorking {
		t.Errorf("Status = %v, want %v", retrieved.Status, types.StatusWorking)
	}
	if retrieved.CurrentTask != "Updated task" {
		t.Errorf("CurrentTask = %q, want %q", retrieved.CurrentTask, "Updated task")
	}
}

func TestRemoveAgent(t *testing.T) {
	store := NewJSONStore(filepath.Join(t.TempDir(), "state.json"))
	store.Load()

	store.AddAgent(&types.Agent{ID: "TestAgent"})
	store.UpdateMetrics("TestAgent", &types.AgentMetrics{TokensUsed: 100})

	store.RemoveAgent("TestAgent")

	if store.GetAgent("TestAgent") != nil {
		t.Error("agent should be removed")
	}
	if store.GetMetrics("TestAgent") != nil {
		t.Error("metrics should be removed")
	}
}

func TestUpdateMetrics(t *testing.T) {
	store := NewJSONStore(filepath.Join(t.TempDir(), "state.json"))
	store.Load()

	metrics := &types.AgentMetrics{
		AgentID:     "TestAgent",
		TokensUsed:  5000,
		FailedTests: 2,
	}
	store.UpdateMetrics("TestAgent", metrics)

	retrieved := store.GetMetrics("TestAgent")
	if retrieved == nil {
		t.Fatal("GetMetrics returned nil")
	}
	if retrieved.TokensUsed != 5000 {
		t.Errorf("TokensUsed = %d, want %d", retrieved.TokensUsed, 5000)
	}
}

func TestMetricsSnapshot(t *testing.T) {
	store := NewJSONStore(filepath.Join(t.TempDir(), "state.json"))
	store.Load()

	store.UpdateMetrics("Agent1", &types.AgentMetrics{TokensUsed: 100})
	store.UpdateMetrics("Agent2", &types.AgentMetrics{TokensUsed: 200})

	store.TakeMetricsSnapshot()

	state := store.GetState()
	if len(state.MetricsHistory) != 1 {
		t.Errorf("expected 1 snapshot, got %d", len(state.MetricsHistory))
	}
	if len(state.MetricsHistory[0].Agents) != 2 {
		t.Errorf("expected 2 agents in snapshot, got %d", len(state.MetricsHistory[0].Agents))
	}
}

func TestHumanInputRequests(t *testing.T) {
	store := NewJSONStore(filepath.Join(t.TempDir(), "state.json"))
	store.Load()

	req := &types.HumanInputRequest{
		ID:       "req-001",
		AgentID:  "TestAgent",
		Question: "What should I do?",
	}
	store.AddHumanRequest(req)

	pending := store.GetPendingRequests()
	if len(pending) != 1 {
		t.Errorf("expected 1 pending request, got %d", len(pending))
	}

	store.AnswerHumanRequest("req-001", "Do this thing")

	pending = store.GetPendingRequests()
	if len(pending) != 0 {
		t.Errorf("expected 0 pending requests after answer, got %d", len(pending))
	}
}

func TestAlerts(t *testing.T) {
	store := NewJSONStore(filepath.Join(t.TempDir(), "state.json"))
	store.Load()

	alert := &types.Alert{
		ID:       "alert-001",
		Type:     "test",
		Message:  "Test alert",
		Severity: "warning",
	}
	store.AddAlert(alert)

	active := store.GetActiveAlerts()
	if len(active) != 1 {
		t.Errorf("expected 1 active alert, got %d", len(active))
	}

	store.AcknowledgeAlert("alert-001")

	active = store.GetActiveAlerts()
	if len(active) != 0 {
		t.Errorf("expected 0 active alerts after ack, got %d", len(active))
	}
}

func TestActivityLog(t *testing.T) {
	store := NewJSONStore(filepath.Join(t.TempDir(), "state.json"))
	store.Load()

	activity := &types.ActivityLog{
		ID:        "act-001",
		AgentID:   "TestAgent",
		Action:    "test",
		Details:   "Test activity",
		Timestamp: time.Now(),
	}
	store.AddActivity(activity)

	state := store.GetState()
	if len(state.ActivityLog) != 1 {
		t.Errorf("expected 1 activity, got %d", len(state.ActivityLog))
	}
}

func TestJudgments(t *testing.T) {
	store := NewJSONStore(filepath.Join(t.TempDir(), "state.json"))
	store.Load()

	judgment := &types.SupervisorJudgment{
		ID:        "jdg-001",
		AgentID:   "TestAgent",
		Issue:     "Test issue",
		Decision:  "Test decision",
		Action:    "continue",
		Timestamp: time.Now(),
	}
	store.AddJudgment(judgment)

	state := store.GetState()
	if len(state.Judgments) != 1 {
		t.Errorf("expected 1 judgment, got %d", len(state.Judgments))
	}
}

func TestHumanCheckin(t *testing.T) {
	store := NewJSONStore(filepath.Join(t.TempDir(), "state.json"))
	store.Load()

	before := store.GetLastHumanCheckin()
	time.Sleep(10 * time.Millisecond)
	store.RecordHumanCheckin()
	after := store.GetLastHumanCheckin()

	if !after.After(before) {
		t.Error("expected checkin time to be updated")
	}
}

func TestThresholds(t *testing.T) {
	store := NewJSONStore(filepath.Join(t.TempDir(), "state.json"))
	store.Load()

	newThresholds := types.AlertThresholds{
		FailedTestsMax:     10,
		IdleTimeMaxSeconds: 1200,
		TokenUsageMax:      50000,
	}
	store.SetThresholds(newThresholds)

	retrieved := store.GetThresholds()
	if retrieved.FailedTestsMax != 10 {
		t.Errorf("FailedTestsMax = %d, want %d", retrieved.FailedTestsMax, 10)
	}
}

func TestAgentCounters(t *testing.T) {
	store := NewJSONStore(filepath.Join(t.TempDir(), "state.json"))
	store.Load()

	n1 := store.GetNextAgentNumber("SNTGreen")
	n2 := store.GetNextAgentNumber("SNTGreen")
	n3 := store.GetNextAgentNumber("SNTPurple")

	if n1 != 1 {
		t.Errorf("first SNTGreen counter = %d, want 1", n1)
	}
	if n2 != 2 {
		t.Errorf("second SNTGreen counter = %d, want 2", n2)
	}
	if n3 != 1 {
		t.Errorf("first SNTPurple counter = %d, want 1", n3)
	}
}

func TestConcurrentAccess(t *testing.T) {
	store := NewJSONStore(filepath.Join(t.TempDir(), "state.json"))
	store.Load()

	done := make(chan bool)

	// Concurrent writes
	go func() {
		for i := 0; i < 100; i++ {
			store.AddAgent(&types.Agent{ID: "Agent-A"})
			store.UpdateAgent("Agent-A", func(a *types.Agent) {
				a.Status = types.StatusWorking
			})
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 100; i++ {
			store.AddAgent(&types.Agent{ID: "Agent-B"})
			store.UpdateMetrics("Agent-B", &types.AgentMetrics{TokensUsed: int64(i)})
		}
		done <- true
	}()

	// Concurrent reads
	go func() {
		for i := 0; i < 100; i++ {
			store.GetState()
			store.GetAgent("Agent-A")
		}
		done <- true
	}()

	<-done
	<-done
	<-done
}

func TestConcurrentSaveOperations(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "state.json")
	store := NewJSONStore(storePath)
	store.Load()

	const goroutines = 10
	const iterations = 50
	done := make(chan bool, goroutines)

	// Multiple goroutines performing various operations concurrently
	for g := 0; g < goroutines; g++ {
		gID := g
		go func() {
			for i := 0; i < iterations; i++ {
				agentID := filepath.Join("Agent", string(rune('A'+gID)))

				// Mix of operations
				store.AddAgent(&types.Agent{
					ID:     agentID,
					Status: types.StatusWorking,
				})
				store.UpdateMetrics(agentID, &types.AgentMetrics{
					TokensUsed: int64(i * 100),
				})
				store.GetAgent(agentID)
				store.GetMetrics(agentID)
				store.GetNextAgentNumber("TestConfig")

				if i%10 == 0 {
					store.Save()
				}
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < goroutines; i++ {
		<-done
	}

	// Final save and verify no corruption
	if err := store.Save(); err != nil {
		t.Fatalf("Save() after concurrent operations failed: %v", err)
	}

	// Load in new instance to verify persistence
	store2 := NewJSONStore(storePath)
	if _, err := store2.Load(); err != nil {
		t.Fatalf("Load() after concurrent operations failed: %v", err)
	}
}

func TestConcurrentRequestShutdown(t *testing.T) {
	store := NewJSONStore(filepath.Join(t.TempDir(), "state.json"))
	store.Load()

	// Add multiple agents
	for i := 0; i < 10; i++ {
		agentID := filepath.Join("Agent", string(rune('A'+i)))
		store.AddAgent(&types.Agent{
			ID:     agentID,
			Status: types.StatusWorking,
		})
	}

	done := make(chan bool, 10)

	// Concurrent shutdown requests
	for i := 0; i < 10; i++ {
		gID := i
		go func() {
			agentID := filepath.Join("Agent", string(rune('A'+gID)))
			for j := 0; j < 20; j++ {
				store.RequestAgentShutdown(agentID, time.Now())
				store.GetAgent(agentID)
			}
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all agents are in stopping state
	for i := 0; i < 10; i++ {
		agentID := filepath.Join("Agent", string(rune('A'+i)))
		agent := store.GetAgent(agentID)
		if agent == nil {
			t.Fatalf("Agent %s should exist", agentID)
		}
		if agent.Status != types.StatusStopping {
			t.Errorf("Agent %s status = %v, want %v", agentID, agent.Status, types.StatusStopping)
		}
	}
}
