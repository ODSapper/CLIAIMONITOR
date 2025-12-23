package agents

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/CLIAIMONITOR/internal/memory"
)

// newTestDB creates a real in-memory SQLite database for testing
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

// TestNewSpawner tests the spawner constructor
func TestNewSpawner(t *testing.T) {
	basePath := t.TempDir()
	mcpURL := "http://localhost:3000/mcp/sse"
	db := newTestDB(t)

	spawner := NewSpawner(basePath, mcpURL, db)

	if spawner == nil {
		t.Fatal("NewSpawner returned nil")
	}

	if spawner.basePath != basePath {
		t.Errorf("Expected basePath %s, got %s", basePath, spawner.basePath)
	}

	if spawner.mcpServerURL != mcpURL {
		t.Errorf("Expected mcpServerURL %s, got %s", mcpURL, spawner.mcpServerURL)
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

	if spawner.agentPanes == nil {
		t.Error("agentPanes map not initialized")
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

	// Add an agent with both PID and pane ID
	spawner.mu.Lock()
	spawner.runningAgents["test-agent-001"] = 12345
	spawner.agentPanes["test-agent-001"] = 42
	spawner.mu.Unlock()

	// Remove it
	spawner.RemoveAgent("test-agent-001")

	// Should be gone from running agents
	agents := spawner.GetRunningAgents()
	if _, ok := agents["test-agent-001"]; ok {
		t.Error("Agent should have been removed from running agents")
	}

	// Should be gone from panes
	if _, ok := spawner.GetAgentPaneID("test-agent-001"); ok {
		t.Error("Agent pane ID should have been removed")
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

// TestCreateMCPConfig is disabled - createMCPConfig method was removed
func TestCreateMCPConfig(t *testing.T) {
	t.Skip("createMCPConfig method has been removed")
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
	os.WriteFile(filepath.Join(pidsDir, "test-agent.pid"), []byte("12345"), 0644)

	// Cleanup
	err := spawner.CleanupAgentFiles("test-agent")
	if err != nil {
		t.Errorf("CleanupAgentFiles failed: %v", err)
	}

	// Verify MCP config and PID files are gone (prompt files are not cleaned up by this function)
	if _, err := os.Stat(filepath.Join(mcpDir, "test-agent-mcp.json")); !os.IsNotExist(err) {
		t.Error("MCP config should be removed")
	}
	if _, err := os.Stat(filepath.Join(pidsDir, "test-agent.pid")); !os.IsNotExist(err) {
		t.Error("PID file should be removed")
	}
}

// TestStopAgentWithReason tests agent stopping
func TestStopAgentWithReason(t *testing.T) {
	basePath := t.TempDir()
	db := newTestDB(t)
	spawner := NewSpawner(basePath, "http://localhost:3000/mcp/sse", db)

	// Add to running agents
	spawner.mu.Lock()
	spawner.runningAgents["test-agent-001"] = 99999 // Fake PID
	spawner.mu.Unlock()

	// Stop it
	err := spawner.StopAgentWithReason("test-agent-001", "test stop")
	if err != nil {
		t.Errorf("StopAgentWithReason failed: %v", err)
	}

	// Verify removed from running agents
	if _, ok := spawner.GetRunningAgents()["test-agent-001"]; ok {
		t.Error("Agent should be removed from running agents")
	}
}

// TestStopAgent tests backward compatible stop method
func TestStopAgent(t *testing.T) {
	basePath := t.TempDir()
	db := newTestDB(t)
	spawner := NewSpawner(basePath, "http://localhost:3000/mcp/sse", db)

	// Add to running agents
	spawner.mu.Lock()
	spawner.runningAgents["test-agent-001"] = 99999 // Fake PID
	spawner.mu.Unlock()

	// Stop it using backward compatible method
	err := spawner.StopAgent("test-agent-001")
	if err != nil {
		t.Errorf("StopAgent failed: %v", err)
	}

	// Verify removed from running agents
	if _, ok := spawner.GetRunningAgents()["test-agent-001"]; ok {
		t.Error("Agent should be removed from running agents")
	}
}

// TestGetAgentPaneID tests retrieving WezTerm pane IDs
func TestGetAgentPaneID(t *testing.T) {
	spawner := NewSpawner(t.TempDir(), "http://localhost:3000/mcp/sse", nil)

	// Test non-existent pane ID
	_, ok := spawner.GetAgentPaneID("non-existent")
	if ok {
		t.Error("Expected pane ID lookup to fail for non-existent agent")
	}

	// Set a pane ID
	spawner.SetAgentPaneID("agent-001", 42)

	// Retrieve it
	paneID, ok := spawner.GetAgentPaneID("agent-001")
	if !ok {
		t.Error("Expected pane ID lookup to succeed")
	}
	if paneID != 42 {
		t.Errorf("Expected pane ID 42, got %d", paneID)
	}
}

// TestSetAgentPaneID tests storing WezTerm pane IDs
func TestSetAgentPaneID(t *testing.T) {
	spawner := NewSpawner(t.TempDir(), "http://localhost:3000/mcp/sse", nil)

	// Set multiple pane IDs
	spawner.SetAgentPaneID("agent-001", 10)
	spawner.SetAgentPaneID("agent-002", 20)
	spawner.SetAgentPaneID("agent-003", 30)

	// Verify all are stored
	tests := []struct {
		agentID  string
		expected int
	}{
		{"agent-001", 10},
		{"agent-002", 20},
		{"agent-003", 30},
	}

	for _, tt := range tests {
		paneID, ok := spawner.GetAgentPaneID(tt.agentID)
		if !ok {
			t.Errorf("Expected pane ID for %s to exist", tt.agentID)
		}
		if paneID != tt.expected {
			t.Errorf("Expected pane ID %d for %s, got %d", tt.expected, tt.agentID, paneID)
		}
	}

	// Update an existing pane ID
	spawner.SetAgentPaneID("agent-001", 99)
	paneID, _ := spawner.GetAgentPaneID("agent-001")
	if paneID != 99 {
		t.Errorf("Expected updated pane ID 99, got %d", paneID)
	}
}

// TestCleanupAllAgentFiles tests bulk file cleanup
func TestCleanupAllAgentFiles(t *testing.T) {
	basePath := t.TempDir()
	spawner := NewSpawner(basePath, "http://localhost:3000/mcp/sse", nil)

	// Create all directories
	mcpDir := filepath.Join(basePath, "configs", "mcp")
	pidsDir := filepath.Join(basePath, "data", "pids")
	os.MkdirAll(mcpDir, 0755)
	os.MkdirAll(pidsDir, 0755)

	// Create multiple agent files (MCP configs and PIDs only - prompt files are not cleaned)
	os.WriteFile(filepath.Join(mcpDir, "agent1-mcp.json"), []byte("{}"), 0644)
	os.WriteFile(filepath.Join(mcpDir, "agent2-mcp.json"), []byte("{}"), 0644)
	os.WriteFile(filepath.Join(pidsDir, "agent1.pid"), []byte("111"), 0644)
	os.WriteFile(filepath.Join(pidsDir, "agent2.pid"), []byte("222"), 0644)

	// Also create non-agent files that should NOT be deleted
	os.WriteFile(filepath.Join(mcpDir, "config.json"), []byte("{}"), 0644)

	// Cleanup all
	err := spawner.CleanupAllAgentFiles()
	if err != nil {
		t.Errorf("CleanupAllAgentFiles failed: %v", err)
	}

	// Verify agent MCP config files are gone
	entries, _ := os.ReadDir(mcpDir)
	for _, e := range entries {
		if e.Name() != "config.json" {
			t.Errorf("Agent MCP config file should be removed: %s", e.Name())
		}
	}

	// Verify non-agent file still exists
	if _, err := os.Stat(filepath.Join(mcpDir, "config.json")); os.IsNotExist(err) {
		t.Error("Non-agent config file should not be removed")
	}

	// Verify PID files are gone
	entries, _ = os.ReadDir(pidsDir)
	if len(entries) > 0 {
		t.Errorf("All PID files should be removed, found %d", len(entries))
	}
}

// TestStopAllAgents tests stopping all running agents
func TestStopAllAgents(t *testing.T) {
	basePath := t.TempDir()
	db := newTestDB(t)
	spawner := NewSpawner(basePath, "http://localhost:3000/mcp/sse", db)

	// Add multiple agents to spawner
	agentIDs := []string{"agent-001", "agent-002", "agent-003"}
	for _, id := range agentIDs {
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
