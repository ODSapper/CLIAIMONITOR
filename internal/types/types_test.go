package types

import (
	"encoding/json"
	"testing"
)

func TestAgentStatusConstants(t *testing.T) {
	statuses := []AgentStatus{
		StatusStarting,
		StatusConnected,
		StatusWorking,
		StatusIdle,
		StatusBlocked,
		StatusDisconnected,
	}

	expected := []string{
		"starting",
		"connected",
		"working",
		"idle",
		"blocked",
		"disconnected",
	}

	for i, status := range statuses {
		if string(status) != expected[i] {
			t.Errorf("status[%d] = %q, want %q", i, status, expected[i])
		}
	}
}

func TestAgentRoleConstants(t *testing.T) {
	roles := []AgentRole{
		RoleGoDeveloper,
		RoleCodeAuditor,
		RoleEngineer,
		RoleSecurity,
		RoleSupervisor,
	}

	expected := []string{
		"Go Developer",
		"Code Auditor",
		"Engineer",
		"Security",
		"Supervisor",
	}

	for i, role := range roles {
		if string(role) != expected[i] {
			t.Errorf("role[%d] = %q, want %q", i, role, expected[i])
		}
	}
}

func TestDefaultThresholds(t *testing.T) {
	thresholds := DefaultThresholds()

	if thresholds.FailedTestsMax != 5 {
		t.Errorf("FailedTestsMax = %d, want 5", thresholds.FailedTestsMax)
	}
	if thresholds.IdleTimeMaxSeconds != 600 {
		t.Errorf("IdleTimeMaxSeconds = %d, want 600", thresholds.IdleTimeMaxSeconds)
	}
	if thresholds.EscalationQueueMax != 10 {
		t.Errorf("EscalationQueueMax = %d, want 10", thresholds.EscalationQueueMax)
	}
	if thresholds.TokenUsageMax != 100000 {
		t.Errorf("TokenUsageMax = %d, want 100000", thresholds.TokenUsageMax)
	}
	if thresholds.ConsecutiveRejectsMax != 3 {
		t.Errorf("ConsecutiveRejectsMax = %d, want 3", thresholds.ConsecutiveRejectsMax)
	}
}

func TestNewDashboardState(t *testing.T) {
	state := NewDashboardState()

	if state == nil {
		t.Fatal("NewDashboardState returned nil")
	}
	if state.Agents == nil {
		t.Error("Agents map should be initialized")
	}
	if state.Metrics == nil {
		t.Error("Metrics map should be initialized")
	}
	if state.HumanRequests == nil {
		t.Error("HumanRequests map should be initialized")
	}
	if state.AgentCounters == nil {
		t.Error("AgentCounters map should be initialized")
	}
	if state.Alerts == nil {
		t.Error("Alerts slice should be initialized")
	}
	if state.ActivityLog == nil {
		t.Error("ActivityLog slice should be initialized")
	}
	if state.Judgments == nil {
		t.Error("Judgments slice should be initialized")
	}
	if state.Thresholds.FailedTestsMax != 5 {
		t.Error("Thresholds should have default values")
	}
}

func TestAgentJSONSerialization(t *testing.T) {
	agent := &Agent{
		ID:          "TestAgent001",
		ConfigName:  "SNTGreen",
		Role:        RoleGoDeveloper,
		Model:       "claude-sonnet-4-5-20250929",
		Color:       "#00cc66",
		Status:      StatusWorking,
		PID:         12345,
		ProjectPath: "/home/user/project",
		CurrentTask: "Implementing feature",
	}

	data, err := json.Marshal(agent)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var decoded Agent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	if decoded.ID != agent.ID {
		t.Errorf("ID = %q, want %q", decoded.ID, agent.ID)
	}
	if decoded.Role != agent.Role {
		t.Errorf("Role = %v, want %v", decoded.Role, agent.Role)
	}
	if decoded.Status != agent.Status {
		t.Errorf("Status = %v, want %v", decoded.Status, agent.Status)
	}
}

func TestAlertThresholdsJSONSerialization(t *testing.T) {
	thresholds := AlertThresholds{
		FailedTestsMax:        10,
		IdleTimeMaxSeconds:    300,
		EscalationQueueMax:    5,
		TokenUsageMax:         50000,
		ConsecutiveRejectsMax: 2,
	}

	data, err := json.Marshal(thresholds)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var decoded AlertThresholds
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	if decoded.FailedTestsMax != 10 {
		t.Errorf("FailedTestsMax = %d, want 10", decoded.FailedTestsMax)
	}
	if decoded.TokenUsageMax != 50000 {
		t.Errorf("TokenUsageMax = %d, want 50000", decoded.TokenUsageMax)
	}
}

func TestMCPRequestJSONSerialization(t *testing.T) {
	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name": "test_tool",
			"arguments": map[string]interface{}{
				"param1": "value1",
			},
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var decoded MCPRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	if decoded.JSONRPC != "2.0" {
		t.Errorf("JSONRPC = %q, want %q", decoded.JSONRPC, "2.0")
	}
	if decoded.Method != "tools/call" {
		t.Errorf("Method = %q, want %q", decoded.Method, "tools/call")
	}
}

func TestMCPResponseJSONSerialization(t *testing.T) {
	// Test success response
	successResp := MCPResponse{
		JSONRPC: "2.0",
		ID:      1,
		Result: map[string]interface{}{
			"status": "ok",
		},
	}

	data, err := json.Marshal(successResp)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var decoded MCPResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	if decoded.Error != nil {
		t.Error("expected no error in success response")
	}

	// Test error response
	errorResp := MCPResponse{
		JSONRPC: "2.0",
		ID:      2,
		Error: &MCPError{
			Code:    -32600,
			Message: "Invalid request",
		},
	}

	data, err = json.Marshal(errorResp)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var decodedErr MCPResponse
	if err := json.Unmarshal(data, &decodedErr); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	if decodedErr.Error == nil {
		t.Fatal("expected error in error response")
	}
	if decodedErr.Error.Code != -32600 {
		t.Errorf("Error.Code = %d, want -32600", decodedErr.Error.Code)
	}
}

func TestWSMessageTypes(t *testing.T) {
	if WSTypeStateUpdate != "state_update" {
		t.Errorf("WSTypeStateUpdate = %q, want %q", WSTypeStateUpdate, "state_update")
	}
	if WSTypeAlert != "alert" {
		t.Errorf("WSTypeAlert = %q, want %q", WSTypeAlert, "alert")
	}
	if WSTypeActivity != "activity" {
		t.Errorf("WSTypeActivity = %q, want %q", WSTypeActivity, "activity")
	}
	if WSTypeSupervisor != "supervisor_status" {
		t.Errorf("WSTypeSupervisor = %q, want %q", WSTypeSupervisor, "supervisor_status")
	}
}

func TestWSMessageJSONSerialization(t *testing.T) {
	msg := WSMessage{
		Type: WSTypeAlert,
		Data: map[string]interface{}{
			"id":      "alert-001",
			"message": "Test alert",
		},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var decoded WSMessage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	if decoded.Type != WSTypeAlert {
		t.Errorf("Type = %q, want %q", decoded.Type, WSTypeAlert)
	}
}

func TestHumanInputRequestJSONSerialization(t *testing.T) {
	req := &HumanInputRequest{
		ID:       "req-001",
		AgentID:  "TestAgent",
		Question: "Should I proceed?",
		Context:  "Working on feature X",
		Answered: false,
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var decoded HumanInputRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	if decoded.ID != "req-001" {
		t.Errorf("ID = %q, want %q", decoded.ID, "req-001")
	}
	if decoded.Answered {
		t.Error("expected Answered = false")
	}
}

func TestAlertJSONSerialization(t *testing.T) {
	alert := &Alert{
		ID:           "alert-001",
		Type:         "failed_tests",
		AgentID:      "TestAgent",
		Message:      "Too many failures",
		Severity:     "warning",
		Acknowledged: false,
	}

	data, err := json.Marshal(alert)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var decoded Alert
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	if decoded.Severity != "warning" {
		t.Errorf("Severity = %q, want %q", decoded.Severity, "warning")
	}
}

func TestSupervisorJudgmentJSONSerialization(t *testing.T) {
	judgment := &SupervisorJudgment{
		ID:        "jdg-001",
		AgentID:   "TestAgent",
		Issue:     "Agent idle",
		Decision:  "Restart",
		Reasoning: "No activity for 10 minutes",
		Action:    "restart",
	}

	data, err := json.Marshal(judgment)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var decoded SupervisorJudgment
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	if decoded.Action != "restart" {
		t.Errorf("Action = %q, want %q", decoded.Action, "restart")
	}
}
