package metrics

import (
	"testing"
	"time"

	"github.com/CLIAIMONITOR/internal/types"
)

func TestNewAlertEngine(t *testing.T) {
	thresholds := types.DefaultThresholds()
	engine := NewAlertEngine(thresholds)

	if engine == nil {
		t.Fatal("NewAlertEngine returned nil")
	}
	if engine.thresholds.FailedTestsMax != 5 {
		t.Errorf("FailedTestsMax = %d, want 5", engine.thresholds.FailedTestsMax)
	}
}

func TestSetGetThresholds(t *testing.T) {
	engine := NewAlertEngine(types.DefaultThresholds())

	newThresholds := types.AlertThresholds{
		FailedTestsMax:     10,
		IdleTimeMaxSeconds: 1200,
	}
	engine.SetThresholds(newThresholds)

	retrieved := engine.GetThresholds()
	if retrieved.FailedTestsMax != 10 {
		t.Errorf("FailedTestsMax = %d, want 10", retrieved.FailedTestsMax)
	}
}

func TestCheckMetricsFailedTests(t *testing.T) {
	thresholds := types.AlertThresholds{
		FailedTestsMax: 5,
	}
	engine := NewAlertEngine(thresholds)

	metrics := map[string]*types.AgentMetrics{
		"Agent1": {AgentID: "Agent1", FailedTests: 3}, // Below threshold
		"Agent2": {AgentID: "Agent2", FailedTests: 5}, // At threshold
		"Agent3": {AgentID: "Agent3", FailedTests: 8}, // Above threshold
	}

	alerts := engine.CheckMetrics(metrics)

	// Should have 2 alerts (at and above threshold)
	failedTestAlerts := 0
	for _, alert := range alerts {
		if alert.Type == "failed_tests" {
			failedTestAlerts++
		}
	}
	if failedTestAlerts != 2 {
		t.Errorf("expected 2 failed_tests alerts, got %d", failedTestAlerts)
	}
}

func TestCheckMetricsIdleTimeout(t *testing.T) {
	thresholds := types.AlertThresholds{
		IdleTimeMaxSeconds: 1, // 1 second for testing
	}
	engine := NewAlertEngine(thresholds)

	metrics := map[string]*types.AgentMetrics{
		"Agent1": {AgentID: "Agent1", IdleSince: time.Now().Add(-2 * time.Second)}, // Idle too long
		"Agent2": {AgentID: "Agent2", IdleSince: time.Time{}},                       // Not idle
	}

	alerts := engine.CheckMetrics(metrics)

	idleAlerts := 0
	for _, alert := range alerts {
		if alert.Type == "idle_timeout" {
			idleAlerts++
		}
	}
	if idleAlerts != 1 {
		t.Errorf("expected 1 idle_timeout alert, got %d", idleAlerts)
	}
}

func TestCheckMetricsTokenUsage(t *testing.T) {
	thresholds := types.AlertThresholds{
		TokenUsageMax: 100000,
	}
	engine := NewAlertEngine(thresholds)

	metrics := map[string]*types.AgentMetrics{
		"Agent1": {AgentID: "Agent1", TokensUsed: 50000},  // Below threshold
		"Agent2": {AgentID: "Agent2", TokensUsed: 100000}, // At threshold
	}

	alerts := engine.CheckMetrics(metrics)

	tokenAlerts := 0
	for _, alert := range alerts {
		if alert.Type == "token_usage" {
			tokenAlerts++
		}
	}
	if tokenAlerts != 1 {
		t.Errorf("expected 1 token_usage alert, got %d", tokenAlerts)
	}
}

func TestCheckMetricsConsecutiveRejects(t *testing.T) {
	thresholds := types.AlertThresholds{
		ConsecutiveRejectsMax: 3,
	}
	engine := NewAlertEngine(thresholds)

	metrics := map[string]*types.AgentMetrics{
		"Agent1": {AgentID: "Agent1", ConsecutiveRejects: 2}, // Below threshold
		"Agent2": {AgentID: "Agent2", ConsecutiveRejects: 3}, // At threshold
	}

	alerts := engine.CheckMetrics(metrics)

	rejectAlerts := 0
	for _, alert := range alerts {
		if alert.Type == "consecutive_rejects" {
			rejectAlerts++
			if alert.Severity != "critical" {
				t.Error("consecutive_rejects alert should be critical")
			}
		}
	}
	if rejectAlerts != 1 {
		t.Errorf("expected 1 consecutive_rejects alert, got %d", rejectAlerts)
	}
}

func TestCheckMetricsNoAlertForZeroThreshold(t *testing.T) {
	thresholds := types.AlertThresholds{
		FailedTestsMax: 0, // Disabled
	}
	engine := NewAlertEngine(thresholds)

	metrics := map[string]*types.AgentMetrics{
		"Agent1": {AgentID: "Agent1", FailedTests: 100},
	}

	alerts := engine.CheckMetrics(metrics)

	for _, alert := range alerts {
		if alert.Type == "failed_tests" {
			t.Error("should not alert when threshold is 0")
		}
	}
}

func TestCheckAgentStatusDisconnected(t *testing.T) {
	engine := NewAlertEngine(types.DefaultThresholds())

	agents := map[string]*types.Agent{
		"Agent1": {ID: "Agent1", Status: types.StatusWorking},
		"Agent2": {ID: "Agent2", Status: types.StatusDisconnected},
	}

	alerts := engine.CheckAgentStatus(agents)

	disconnectedAlerts := 0
	for _, alert := range alerts {
		if alert.Type == "agent_disconnected" {
			disconnectedAlerts++
			if alert.Severity != "critical" {
				t.Error("agent_disconnected should be critical")
			}
		}
	}
	if disconnectedAlerts != 1 {
		t.Errorf("expected 1 agent_disconnected alert, got %d", disconnectedAlerts)
	}
}

func TestCheckAgentStatusBlocked(t *testing.T) {
	engine := NewAlertEngine(types.DefaultThresholds())

	agents := map[string]*types.Agent{
		"Agent1": {ID: "Agent1", Status: types.StatusBlocked, CurrentTask: "Waiting for input"},
	}

	alerts := engine.CheckAgentStatus(agents)

	blockedAlerts := 0
	for _, alert := range alerts {
		if alert.Type == "agent_blocked" {
			blockedAlerts++
			if alert.Severity != "warning" {
				t.Error("agent_blocked should be warning")
			}
		}
	}
	if blockedAlerts != 1 {
		t.Errorf("expected 1 agent_blocked alert, got %d", blockedAlerts)
	}
}

func TestCheckHumanCheckin(t *testing.T) {
	thresholds := types.AlertThresholds{
		HumanCheckinSeconds: 1, // 1 second for testing
	}
	engine := NewAlertEngine(thresholds)

	// Last checkin was 2 seconds ago
	lastCheckin := time.Now().Add(-2 * time.Second)
	alert := engine.CheckHumanCheckin(lastCheckin)

	if alert == nil {
		t.Fatal("expected human_checkin alert")
	}
	if alert.Type != "human_checkin" {
		t.Errorf("alert.Type = %q, want %q", alert.Type, "human_checkin")
	}
}

func TestCheckHumanCheckinNoAlertWhenRecent(t *testing.T) {
	thresholds := types.AlertThresholds{
		HumanCheckinSeconds: 3600, // 1 hour
	}
	engine := NewAlertEngine(thresholds)

	// Last checkin was just now
	lastCheckin := time.Now()
	alert := engine.CheckHumanCheckin(lastCheckin)

	if alert != nil {
		t.Error("should not alert when checkin is recent")
	}
}

func TestCheckHumanCheckinDisabled(t *testing.T) {
	thresholds := types.AlertThresholds{
		HumanCheckinSeconds: 0, // Disabled
	}
	engine := NewAlertEngine(thresholds)

	lastCheckin := time.Now().Add(-24 * time.Hour)
	alert := engine.CheckHumanCheckin(lastCheckin)

	if alert != nil {
		t.Error("should not alert when threshold is 0")
	}
}

func TestCheckEscalationQueue(t *testing.T) {
	thresholds := types.AlertThresholds{
		EscalationQueueMax: 5,
	}
	engine := NewAlertEngine(thresholds)

	// Below threshold
	alert := engine.CheckEscalationQueue(3)
	if alert != nil {
		t.Error("should not alert below threshold")
	}

	// At threshold
	alert = engine.CheckEscalationQueue(5)
	if alert == nil {
		t.Fatal("expected escalation_queue alert")
	}
	if alert.Type != "escalation_queue" {
		t.Errorf("alert.Type = %q, want %q", alert.Type, "escalation_queue")
	}
	if alert.Severity != "critical" {
		t.Error("escalation_queue should be critical")
	}
}

func TestCheckEscalationQueueDisabled(t *testing.T) {
	thresholds := types.AlertThresholds{
		EscalationQueueMax: 0, // Disabled
	}
	engine := NewAlertEngine(thresholds)

	alert := engine.CheckEscalationQueue(100)
	if alert != nil {
		t.Error("should not alert when threshold is 0")
	}
}

func TestAlertDeduplication(t *testing.T) {
	thresholds := types.AlertThresholds{
		FailedTestsMax: 5,
	}
	engine := NewAlertEngine(thresholds)

	metrics := map[string]*types.AgentMetrics{
		"Agent1": {AgentID: "Agent1", FailedTests: 10},
	}

	// First check should produce alert
	alerts1 := engine.CheckMetrics(metrics)
	if len(alerts1) == 0 {
		t.Fatal("expected alert on first check")
	}

	// Second immediate check should not produce duplicate
	alerts2 := engine.CheckMetrics(metrics)
	if len(alerts2) != 0 {
		t.Error("should not produce duplicate alert within 5 minutes")
	}
}

func TestAlertHasUniqueID(t *testing.T) {
	thresholds := types.AlertThresholds{
		FailedTestsMax: 5,
	}
	engine := NewAlertEngine(thresholds)

	// Need to wait for dedup to clear or use different agents
	agents := map[string]*types.Agent{
		"Agent1": {ID: "Agent1", Status: types.StatusDisconnected},
		"Agent2": {ID: "Agent2", Status: types.StatusDisconnected},
	}

	alerts := engine.CheckAgentStatus(agents)

	if len(alerts) < 2 {
		t.Skip("not enough alerts to test uniqueness")
	}

	ids := make(map[string]bool)
	for _, alert := range alerts {
		if ids[alert.ID] {
			t.Error("alert IDs should be unique")
		}
		ids[alert.ID] = true
	}
}
