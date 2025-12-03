package bootstrap

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/CLIAIMONITOR/internal/memory"
)

func TestNewPortableState(t *testing.T) {
	state := NewPortableState("env-test", "Test Environment", "test", "captain-001")

	if state.Version != "1.0" {
		t.Errorf("Expected version 1.0, got %s", state.Version)
	}

	if state.CaptainID != "captain-001" {
		t.Errorf("Expected captain-001, got %s", state.CaptainID)
	}

	if state.Environment.ID != "env-test" {
		t.Errorf("Expected env-test, got %s", state.Environment.ID)
	}

	if state.Mode != "lightweight" {
		t.Errorf("Expected lightweight mode, got %s", state.Mode)
	}

	if state.ScaleUp.Triggered {
		t.Error("Expected scale-up not triggered")
	}
}

func TestStateManager_SaveAndLoad(t *testing.T) {
	// Create temporary directory
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	manager := NewStateManager()

	// Create test state
	original := NewPortableState("env-test", "Test Env", "test", "captain-test")
	original.ActiveAgents = []string{"snake001", "snake002"}
	original.FindingsSummary.Critical = 2
	original.FindingsSummary.High = 5

	// Save state
	err := manager.SaveState(original, statePath)
	if err != nil {
		t.Fatalf("Failed to save state: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(statePath); err != nil {
		t.Fatalf("State file not created: %v", err)
	}

	// Load state
	loaded, err := manager.LoadState(statePath)
	if err != nil {
		t.Fatalf("Failed to load state: %v", err)
	}

	// Verify fields
	if loaded.CaptainID != original.CaptainID {
		t.Errorf("Captain ID mismatch: got %s, want %s", loaded.CaptainID, original.CaptainID)
	}

	if loaded.Environment.Name != original.Environment.Name {
		t.Errorf("Environment name mismatch: got %s, want %s", loaded.Environment.Name, original.Environment.Name)
	}

	if len(loaded.ActiveAgents) != len(original.ActiveAgents) {
		t.Errorf("Active agents count mismatch: got %d, want %d", len(loaded.ActiveAgents), len(original.ActiveAgents))
	}

	if loaded.FindingsSummary.Critical != original.FindingsSummary.Critical {
		t.Errorf("Critical count mismatch: got %d, want %d", loaded.FindingsSummary.Critical, original.FindingsSummary.Critical)
	}
}

func TestStateManager_MergeFindings(t *testing.T) {
	manager := NewStateManager()
	state := NewPortableState("env-test", "Test", "test", "captain-test")

	// Create test findings
	findings := []*memory.ReconFinding{
		{Severity: "critical"},
		{Severity: "critical"},
		{Severity: "high"},
		{Severity: "high"},
		{Severity: "high"},
		{Severity: "medium"},
		{Severity: "low"},
	}

	err := manager.MergeFindings(state, findings)
	if err != nil {
		t.Fatalf("MergeFindings failed: %v", err)
	}

	if state.FindingsSummary.Critical != 2 {
		t.Errorf("Expected 2 critical, got %d", state.FindingsSummary.Critical)
	}

	if state.FindingsSummary.High != 3 {
		t.Errorf("Expected 3 high, got %d", state.FindingsSummary.High)
	}

	if state.FindingsSummary.Medium != 1 {
		t.Errorf("Expected 1 medium, got %d", state.FindingsSummary.Medium)
	}

	if state.FindingsSummary.Low != 1 {
		t.Errorf("Expected 1 low, got %d", state.FindingsSummary.Low)
	}
}

func TestStateManager_ExportForSync(t *testing.T) {
	manager := NewStateManager()
	state := NewPortableState("env-test", "Test Env", "customer", "captain-test")

	state.ActiveAgents = []string{"snake001", "worker001"}
	state.FindingsSummary.Critical = 3
	state.FindingsSummary.High = 7

	report, err := manager.ExportForSync(state)
	if err != nil {
		t.Fatalf("ExportForSync failed: %v", err)
	}

	if report.CaptainID != "captain-test" {
		t.Errorf("Captain ID mismatch: got %s", report.CaptainID)
	}

	if report.Environment != "Test Env" {
		t.Errorf("Environment mismatch: got %s", report.Environment)
	}

	if report.FindingsSummary["critical"] != 3 {
		t.Errorf("Critical count mismatch: got %d", report.FindingsSummary["critical"])
	}

	if len(report.ActiveAgents) != 2 {
		t.Errorf("Active agents count mismatch: got %d", len(report.ActiveAgents))
	}

	// Status should be "scanning" with critical/high findings
	if report.Status != "scanning" {
		t.Errorf("Expected status 'scanning', got '%s'", report.Status)
	}
}

func TestStateManager_ImportFromHQ(t *testing.T) {
	manager := NewStateManager()

	// Create test JSON
	jsonData := []byte(`{
		"version": "1.0",
		"captain_id": "captain-hq",
		"environment": {
			"id": "env-hq",
			"name": "HQ Backup",
			"type": "customer",
			"first_contact": "2025-12-01T10:00:00Z"
		},
		"mode": "connected",
		"findings_summary": {
			"critical": 1,
			"high": 2,
			"medium": 3,
			"low": 4
		},
		"active_agents": ["snake001"],
		"pending_decisions": [],
		"phone_home": {
			"enabled": true,
			"endpoint": "https://hq.example.com",
			"last_sync": null,
			"api_key_env": "API_KEY"
		},
		"scale_up": {
			"triggered": false,
			"reason": null,
			"cliaimonitor_port": null
		}
	}`)

	state, err := manager.ImportFromHQ(jsonData)
	if err != nil {
		t.Fatalf("ImportFromHQ failed: %v", err)
	}

	if state.CaptainID != "captain-hq" {
		t.Errorf("Captain ID mismatch: got %s", state.CaptainID)
	}

	if state.Mode != "connected" {
		t.Errorf("Mode mismatch: got %s", state.Mode)
	}

	if state.FindingsSummary.Critical != 1 {
		t.Errorf("Critical count mismatch: got %d", state.FindingsSummary.Critical)
	}
}

func TestStateManager_ReconstructMemory(t *testing.T) {
	// This test would require a real ReconRepository implementation
	// For now, we'll create a basic test structure

	manager := NewStateManager()
	state := NewPortableState("env-test", "Test Env", "test", "captain-test")

	// Create a mock recon repository (would use real implementation in practice)
	// For now, just verify the method doesn't panic
	ctx := context.Background()

	// Note: This will fail without a real ReconRepository implementation
	// In a real test, you'd use a test database
	err := manager.ReconstructMemory(ctx, state, nil)
	if err == nil {
		t.Error("Expected error with nil reconRepo")
	}
}

func TestLoadState_InvalidVersion(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	// Write state with invalid version
	invalidState := `{
		"version": "2.0",
		"captain_id": "test"
	}`

	err := os.WriteFile(statePath, []byte(invalidState), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	manager := NewStateManager()
	_, err = manager.LoadState(statePath)
	if err == nil {
		t.Error("Expected error for unsupported version")
	}
}

func TestExportForSync_NeedsHelp(t *testing.T) {
	manager := NewStateManager()
	state := NewPortableState("env-test", "Test", "test", "captain-test")

	// Add many pending decisions to trigger help request
	for i := 0; i < 10; i++ {
		state.PendingDecisions = append(state.PendingDecisions, "decision-"+string(rune(i)))
	}

	report, err := manager.ExportForSync(state)
	if err != nil {
		t.Fatalf("ExportForSync failed: %v", err)
	}

	if !report.NeedsHelp {
		t.Error("Expected NeedsHelp to be true with 10 pending decisions")
	}

	if report.HelpReason == "" {
		t.Error("Expected HelpReason to be set")
	}
}

func TestPhoneHomeConfig(t *testing.T) {
	state := NewPortableState("env-test", "Test", "test", "captain-test")

	if state.PhoneHome.Enabled {
		t.Error("Phone home should be disabled by default")
	}

	if state.PhoneHome.APIKeyEnv != "MAGNOLIA_API_KEY" {
		t.Errorf("Expected API key env MAGNOLIA_API_KEY, got %s", state.PhoneHome.APIKeyEnv)
	}

	if state.PhoneHome.LastSync != nil {
		t.Error("LastSync should be nil initially")
	}
}

func TestScaleUpStatus(t *testing.T) {
	state := NewPortableState("env-test", "Test", "test", "captain-test")

	if state.ScaleUp.Triggered {
		t.Error("ScaleUp should not be triggered initially")
	}

	if state.ScaleUp.Reason != nil {
		t.Error("ScaleUp reason should be nil initially")
	}

	if state.ScaleUp.CLIAIMonitorPort != nil {
		t.Error("CLIAIMonitor port should be nil initially")
	}

	// Test triggering scale-up
	state.ScaleUp.Triggered = true
	reason := "Test reason"
	state.ScaleUp.Reason = &reason
	port := 8080
	state.ScaleUp.CLIAIMonitorPort = &port

	// Save and reload to verify serialization
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	manager := NewStateManager()
	err := manager.SaveState(state, statePath)
	if err != nil {
		t.Fatalf("Failed to save state: %v", err)
	}

	loaded, err := manager.LoadState(statePath)
	if err != nil {
		t.Fatalf("Failed to load state: %v", err)
	}

	if !loaded.ScaleUp.Triggered {
		t.Error("ScaleUp triggered status not preserved")
	}

	if loaded.ScaleUp.Reason == nil || *loaded.ScaleUp.Reason != reason {
		t.Error("ScaleUp reason not preserved")
	}

	if loaded.ScaleUp.CLIAIMonitorPort == nil || *loaded.ScaleUp.CLIAIMonitorPort != port {
		t.Error("ScaleUp port not preserved")
	}
}

func TestEnvironmentFirstContact(t *testing.T) {
	before := time.Now()
	state := NewPortableState("env-test", "Test", "test", "captain-test")
	after := time.Now()

	if state.Environment.FirstContact.Before(before) || state.Environment.FirstContact.After(after) {
		t.Error("FirstContact timestamp not within expected range")
	}
}
