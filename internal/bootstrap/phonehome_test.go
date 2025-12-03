package bootstrap

import (
	"context"
	"testing"
	"time"
)

func TestMockPhoneHomeClient_SendReport(t *testing.T) {
	client := NewMockPhoneHomeClient()

	report := &PhoneHomeReport{
		CaptainID:   "captain-test",
		Environment: "Test Env",
		Timestamp:   time.Now(),
		FindingsSummary: map[string]int{
			"critical": 2,
			"high":     5,
		},
		Status: "scanning",
	}

	ctx := context.Background()
	err := client.SendReport(ctx, report)
	if err != nil {
		t.Fatalf("SendReport failed: %v", err)
	}

	if len(client.ReportsSent) != 1 {
		t.Errorf("Expected 1 report sent, got %d", len(client.ReportsSent))
	}

	if client.ReportsSent[0].CaptainID != "captain-test" {
		t.Errorf("Captain ID mismatch: got %s", client.ReportsSent[0].CaptainID)
	}
}

func TestMockPhoneHomeClient_Heartbeat(t *testing.T) {
	client := NewMockPhoneHomeClient()
	ctx := context.Background()

	// Send multiple heartbeats
	for i := 0; i < 5; i++ {
		err := client.Heartbeat(ctx)
		if err != nil {
			t.Fatalf("Heartbeat %d failed: %v", i, err)
		}
	}

	if client.HeartbeatsSent != 5 {
		t.Errorf("Expected 5 heartbeats, got %d", client.HeartbeatsSent)
	}
}

func TestMockPhoneHomeClient_GetInstructions(t *testing.T) {
	client := NewMockPhoneHomeClient()

	// Set test instructions
	client.Instructions = &HQInstructions{
		Priority: "high",
		Tasks: []HQTask{
			{
				ID:          "task-001",
				Type:        "recon",
				Description: "Scan customer network",
				Priority:    1,
			},
		},
		AbortMission: false,
	}

	ctx := context.Background()
	instructions, err := client.GetInstructions(ctx)
	if err != nil {
		t.Fatalf("GetInstructions failed: %v", err)
	}

	if instructions.Priority != "high" {
		t.Errorf("Priority mismatch: got %s", instructions.Priority)
	}

	if len(instructions.Tasks) != 1 {
		t.Errorf("Expected 1 task, got %d", len(instructions.Tasks))
	}

	if instructions.Tasks[0].ID != "task-001" {
		t.Errorf("Task ID mismatch: got %s", instructions.Tasks[0].ID)
	}
}

func TestMockPhoneHomeClient_SyncState(t *testing.T) {
	client := NewMockPhoneHomeClient()
	state := NewPortableState("env-test", "Test", "test", "captain-test")

	ctx := context.Background()
	err := client.SyncState(ctx, state)
	if err != nil {
		t.Fatalf("SyncState failed: %v", err)
	}

	if len(client.StatesSynced) != 1 {
		t.Errorf("Expected 1 state synced, got %d", len(client.StatesSynced))
	}

	if client.StatesSynced[0].CaptainID != "captain-test" {
		t.Errorf("Captain ID mismatch: got %s", client.StatesSynced[0].CaptainID)
	}
}

func TestMockPhoneHomeClient_Errors(t *testing.T) {
	client := NewMockPhoneHomeClient()
	client.ShouldError = true

	ctx := context.Background()

	// Test SendReport error
	err := client.SendReport(ctx, &PhoneHomeReport{})
	if err == nil {
		t.Error("Expected error from SendReport")
	}

	// Test Heartbeat error
	err = client.Heartbeat(ctx)
	if err == nil {
		t.Error("Expected error from Heartbeat")
	}

	// Test GetInstructions error
	_, err = client.GetInstructions(ctx)
	if err == nil {
		t.Error("Expected error from GetInstructions")
	}

	// Test SyncState error
	err = client.SyncState(ctx, NewPortableState("test", "test", "test", "test"))
	if err == nil {
		t.Error("Expected error from SyncState")
	}
}

func TestPhoneHomeReport_Structure(t *testing.T) {
	report := &PhoneHomeReport{
		CaptainID:   "captain-001",
		Environment: "Test Environment",
		Timestamp:   time.Now(),
		FindingsSummary: map[string]int{
			"critical": 1,
			"high":     2,
			"medium":   3,
			"low":      4,
		},
		ActiveAgents: []string{"snake001", "worker001"},
		Status:       "coordinating",
		NeedsHelp:    true,
		HelpReason:   "Too many findings",
	}

	if report.CaptainID != "captain-001" {
		t.Errorf("Captain ID mismatch: got %s", report.CaptainID)
	}

	if report.FindingsSummary["critical"] != 1 {
		t.Errorf("Critical count mismatch: got %d", report.FindingsSummary["critical"])
	}

	if len(report.ActiveAgents) != 2 {
		t.Errorf("Active agents count mismatch: got %d", len(report.ActiveAgents))
	}

	if !report.NeedsHelp {
		t.Error("NeedsHelp should be true")
	}
}

func TestHQInstructions_Structure(t *testing.T) {
	instructions := &HQInstructions{
		Priority: "critical",
		Tasks: []HQTask{
			{
				ID:          "task-001",
				Type:        "fix",
				Description: "Patch vulnerability",
				Priority:    1,
				Deadline:    time.Now().Add(24 * time.Hour),
			},
		},
		ConfigUpdates: map[string]interface{}{
			"phone_home_interval": 300,
		},
		AbortMission: false,
	}

	if instructions.Priority != "critical" {
		t.Errorf("Priority mismatch: got %s", instructions.Priority)
	}

	if len(instructions.Tasks) != 1 {
		t.Errorf("Expected 1 task, got %d", len(instructions.Tasks))
	}

	if instructions.AbortMission {
		t.Error("AbortMission should be false")
	}

	if instructions.ConfigUpdates["phone_home_interval"] != 300 {
		t.Error("Config update not preserved")
	}
}

func TestHQInstructions_AbortMission(t *testing.T) {
	instructions := &HQInstructions{
		Priority:     "critical",
		Tasks:        []HQTask{},
		AbortMission: true,
		AbortReason:  "Customer requested stop",
	}

	if !instructions.AbortMission {
		t.Error("AbortMission should be true")
	}

	if instructions.AbortReason != "Customer requested stop" {
		t.Errorf("Abort reason mismatch: got %s", instructions.AbortReason)
	}
}

func TestNewPhoneHomeClient_MissingAPIKey(t *testing.T) {
	// Ensure API key env var is not set
	originalKey := ""
	if val, exists := lookupEnv("TEST_MISSING_API_KEY"); exists {
		originalKey = val
		unsetEnv("TEST_MISSING_API_KEY")
	}
	defer func() {
		if originalKey != "" {
			setEnv("TEST_MISSING_API_KEY", originalKey)
		}
	}()

	// Try to create client without API key
	_, err := NewPhoneHomeClient("https://example.com", "TEST_MISSING_API_KEY", "captain-test")
	if err == nil {
		t.Error("Expected error when API key is missing")
	}
}

// Helper functions for environment variable testing
func lookupEnv(key string) (string, bool) {
	// Simplified - in real code would use os.LookupEnv
	return "", false
}

func unsetEnv(key string) {
	// Simplified - in real code would use os.Unsetenv
}

func setEnv(key, value string) {
	// Simplified - in real code would use os.Setenv
}
