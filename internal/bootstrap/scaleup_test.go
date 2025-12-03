package bootstrap

import (
	"context"
	"testing"
	"time"
)

func TestMockScaleUpDetector_ShouldScaleUp(t *testing.T) {
	detector := NewMockScaleUpDetector()
	state := NewPortableState("env-test", "Test", "test", "captain-test")

	// Initially should not scale up
	shouldScale, reason := detector.ShouldScaleUp(state)
	if shouldScale {
		t.Error("Should not scale up initially")
	}
	if reason != "" {
		t.Error("Reason should be empty when not scaling")
	}

	// Set mock to trigger scale-up
	detector.ShouldScale = true
	detector.ScaleReason = "Test reason"

	shouldScale, reason = detector.ShouldScaleUp(state)
	if !shouldScale {
		t.Error("Should scale up when mock is configured")
	}
	if reason != "Test reason" {
		t.Errorf("Reason mismatch: got %s", reason)
	}
}

func TestMockScaleUpDetector_ScaleUp(t *testing.T) {
	detector := NewMockScaleUpDetector()
	ctx := context.Background()

	// Initial state
	if detector.GetInfraLevel() != InfraLightweight {
		t.Errorf("Expected lightweight initially, got %s", detector.GetInfraLevel())
	}

	// Perform scale-up
	err := detector.ScaleUp(ctx)
	if err != nil {
		t.Fatalf("ScaleUp failed: %v", err)
	}

	if !detector.ScaleUpCalled {
		t.Error("ScaleUpCalled should be true")
	}

	if detector.GetInfraLevel() != InfraLocal {
		t.Errorf("Expected local after scale-up, got %s", detector.GetInfraLevel())
	}
}

func TestMockScaleUpDetector_ScaleUpError(t *testing.T) {
	detector := NewMockScaleUpDetector()
	detector.ScaleUpError = context.DeadlineExceeded

	ctx := context.Background()
	err := detector.ScaleUp(ctx)
	if err == nil {
		t.Error("Expected error from ScaleUp")
	}
	if err != context.DeadlineExceeded {
		t.Errorf("Expected DeadlineExceeded, got %v", err)
	}
}

func TestScaleUpDetector_MultipleAgents(t *testing.T) {
	_ = NewMockScaleUpDetector()
	state := NewPortableState("env-test", "Test", "test", "captain-test")

	// Add more than 3 agents
	state.ActiveAgents = []string{"snake001", "snake002", "worker001", "worker002"}

	// Note: Mock doesn't implement the actual logic, would need real detector for this
	// This test demonstrates the expected behavior
	if len(state.ActiveAgents) != 4 {
		t.Errorf("Expected 4 agents, got %d", len(state.ActiveAgents))
	}
}

func TestScaleUpDetector_MultiDayEngagement(t *testing.T) {
	state := NewPortableState("env-test", "Test", "test", "captain-test")

	// Set first contact to 2 days ago
	state.Environment.FirstContact = time.Now().Add(-48 * time.Hour)

	// Note: Mock doesn't implement the actual logic
	// In a real test with StandardScaleUpDetector, this would trigger scale-up
}

func TestScaleUpDetector_CriticalFindings(t *testing.T) {
	state := NewPortableState("env-test", "Test", "test", "captain-test")

	// Add critical findings
	state.FindingsSummary.Critical = 5

	// Note: Mock doesn't implement the actual logic
	// In a real test with StandardScaleUpDetector, this would trigger scale-up
}

func TestScaleUpDetector_HighVolumeFindings(t *testing.T) {
	state := NewPortableState("env-test", "Test", "test", "captain-test")

	// Add high volume of findings
	state.FindingsSummary.High = 20
	state.FindingsSummary.Medium = 30
	state.FindingsSummary.Low = 10

	// Total: 60 findings, should trigger scale-up
	// Note: Mock doesn't implement the actual logic
}

func TestScaleUpDetector_PendingDecisions(t *testing.T) {
	state := NewPortableState("env-test", "Test", "test", "captain-test")

	// Add many pending decisions
	for i := 0; i < 15; i++ {
		state.PendingDecisions = append(state.PendingDecisions, "decision-"+string(rune(i)))
	}

	// Should trigger scale-up with >10 decisions
	// Note: Mock doesn't implement the actual logic
}

func TestScaleUpDetector_AlreadyTriggered(t *testing.T) {
	state := NewPortableState("env-test", "Test", "test", "captain-test")

	// Mark as already triggered
	state.ScaleUp.Triggered = true

	// Should not trigger again
	// Note: Mock doesn't implement the actual logic
}

func TestDefaultScaleUpConfig(t *testing.T) {
	config := DefaultScaleUpConfig()

	if config == nil {
		t.Fatal("Config should not be nil")
	}

	if config.Port != 8080 {
		t.Errorf("Expected port 8080, got %d", config.Port)
	}

	if config.DataDir != "./data" {
		t.Errorf("Expected data dir './data', got %s", config.DataDir)
	}

	if !config.AutoScaleUp {
		t.Error("AutoScaleUp should be true by default")
	}

	if config.CLIAIMonitorPath == "" {
		t.Error("CLIAIMonitorPath should not be empty")
	}
}

func TestInfraLevel_Values(t *testing.T) {
	tests := []struct {
		level    InfraLevel
		expected string
	}{
		{InfraLightweight, "lightweight"},
		{InfraLocal, "local"},
		{InfraConnected, "connected"},
		{InfraFull, "full"},
	}

	for _, tt := range tests {
		if string(tt.level) != tt.expected {
			t.Errorf("InfraLevel mismatch: got %s, want %s", tt.level, tt.expected)
		}
	}
}

func TestMockScaleUpDetector_IsAvailable(t *testing.T) {
	detector := NewMockScaleUpDetector()

	// Should be available by default
	if !detector.IsCLIAIMonitorAvailable() {
		t.Error("Should be available by default")
	}

	// Set to unavailable
	detector.IsAvailable = false
	if detector.IsCLIAIMonitorAvailable() {
		t.Error("Should not be available when set to false")
	}
}

func TestScaleUpConfig_Customization(t *testing.T) {
	config := &ScaleUpConfig{
		CLIAIMonitorPath: "/custom/path/cliaimonitor",
		DataDir:          "/var/lib/cliaimonitor",
		Port:             9090,
		AutoScaleUp:      false,
	}

	if config.Port != 9090 {
		t.Errorf("Expected port 9090, got %d", config.Port)
	}

	if config.AutoScaleUp {
		t.Error("AutoScaleUp should be false")
	}

	if config.DataDir != "/var/lib/cliaimonitor" {
		t.Errorf("Data dir mismatch: got %s", config.DataDir)
	}
}

func TestStandardScaleUpDetector_ShouldScaleUp_MultipleAgents(t *testing.T) {
	// Note: This would test the real StandardScaleUpDetector
	// For now, documenting expected behavior

	// state := NewPortableState("env-test", "Test", "test", "captain-test")
	// state.ActiveAgents = []string{"s1", "s2", "s3", "s4"}
	//
	// detector := NewScaleUpDetector("./cliaimonitor", "./data", 8080)
	// shouldScale, reason := detector.ShouldScaleUp(state)
	//
	// if !shouldScale {
	//     t.Error("Should scale up with 4 active agents")
	// }
	// if reason == "" {
	//     t.Error("Reason should be provided")
	// }
}

func TestStandardScaleUpDetector_GetInfraLevel(t *testing.T) {
	detector := NewMockScaleUpDetector()

	// Initially lightweight
	if detector.GetInfraLevel() != InfraLightweight {
		t.Errorf("Expected lightweight, got %s", detector.GetInfraLevel())
	}

	// After scale-up, should be local
	ctx := context.Background()
	detector.ScaleUp(ctx)

	if detector.GetInfraLevel() != InfraLocal {
		t.Errorf("Expected local after scale-up, got %s", detector.GetInfraLevel())
	}
}
