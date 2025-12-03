package mcp

import (
	"testing"
)

func TestSnakeTools_Registration(t *testing.T) {
	s := NewServer()

	// Mock callbacks
	callbacks := ToolCallbacks{
		OnSubmitReconReport: func(agentID string, report map[string]interface{}) (interface{}, error) {
			return map[string]interface{}{"status": "received"}, nil
		},
		OnRequestGuidance: func(agentID string, guidance map[string]interface{}) (interface{}, error) {
			return map[string]interface{}{"status": "queued"}, nil
		},
		OnReportProgress: func(agentID string, progress map[string]interface{}) (interface{}, error) {
			return map[string]interface{}{"status": "recorded"}, nil
		},
	}

	registerSnakeTools(s, callbacks)

	// Test that tools are registered
	tools := []string{
		"submit_recon_report",
		"request_guidance",
		"report_progress",
	}

	for _, toolName := range tools {
		if _, ok := s.tools.Get(toolName); !ok {
			t.Errorf("Tool %s not registered", toolName)
		}
	}
}

func TestSnakeTools_SubmitReconReport(t *testing.T) {
	s := NewServer()

	var capturedAgentID string
	var capturedReport map[string]interface{}

	callbacks := ToolCallbacks{
		OnSubmitReconReport: func(agentID string, report map[string]interface{}) (interface{}, error) {
			capturedAgentID = agentID
			capturedReport = report
			return map[string]interface{}{"status": "received"}, nil
		},
	}

	registerSnakeTools(s, callbacks)

	// Execute tool
	params := map[string]interface{}{
		"environment": "test-env",
		"mission":     "initial_recon",
		"findings": map[string]interface{}{
			"critical": []interface{}{},
			"high":     []interface{}{},
			"medium":   []interface{}{},
			"low":      []interface{}{},
		},
		"summary": map[string]interface{}{
			"total_files_scanned": 100,
		},
		"recommendations": map[string]interface{}{
			"immediate":  []string{},
			"short_term": []string{},
			"long_term":  []string{},
		},
	}

	result, err := s.tools.Execute("submit_recon_report", "Snake001", params)

	if err != nil {
		t.Fatalf("Error executing submit_recon_report: %v", err)
	}

	if capturedAgentID != "Snake001" {
		t.Errorf("Expected agentID 'Snake001', got '%s'", capturedAgentID)
	}

	if capturedReport["environment"] != "test-env" {
		t.Errorf("Expected environment 'test-env', got '%v'", capturedReport["environment"])
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected result to be map[string]interface{}")
	}

	if resultMap["status"] != "received" {
		t.Errorf("Expected status 'received', got '%v'", resultMap["status"])
	}
}

func TestSnakeTools_RequestGuidance(t *testing.T) {
	s := NewServer()

	var capturedAgentID string
	var capturedGuidance map[string]interface{}

	callbacks := ToolCallbacks{
		OnRequestGuidance: func(agentID string, guidance map[string]interface{}) (interface{}, error) {
			capturedAgentID = agentID
			capturedGuidance = guidance
			return map[string]interface{}{"status": "queued"}, nil
		},
	}

	registerSnakeTools(s, callbacks)

	params := map[string]interface{}{
		"situation":      "Unclear authentication pattern",
		"options":        []interface{}{"Flag as high", "Request audit"},
		"recommendation": "Request human audit",
	}

	result, err := s.tools.Execute("request_guidance", "Snake001", params)

	if err != nil {
		t.Fatalf("Error executing request_guidance: %v", err)
	}

	if capturedAgentID != "Snake001" {
		t.Errorf("Expected agentID 'Snake001', got '%s'", capturedAgentID)
	}

	if capturedGuidance["situation"] != "Unclear authentication pattern" {
		t.Errorf("Expected situation 'Unclear authentication pattern', got '%v'", capturedGuidance["situation"])
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected result to be map[string]interface{}")
	}

	if resultMap["status"] != "queued" {
		t.Errorf("Expected status 'queued', got '%v'", resultMap["status"])
	}
}

func TestSnakeTools_ReportProgress(t *testing.T) {
	s := NewServer()

	var capturedAgentID string
	var capturedProgress map[string]interface{}

	callbacks := ToolCallbacks{
		OnReportProgress: func(agentID string, progress map[string]interface{}) (interface{}, error) {
			capturedAgentID = agentID
			capturedProgress = progress
			return map[string]interface{}{"status": "recorded"}, nil
		},
	}

	registerSnakeTools(s, callbacks)

	params := map[string]interface{}{
		"phase":            "security",
		"percent_complete": float64(30),
		"files_scanned":    float64(150),
		"findings_so_far":  float64(5),
	}

	result, err := s.tools.Execute("report_progress", "Snake001", params)

	if err != nil {
		t.Fatalf("Error executing report_progress: %v", err)
	}

	if capturedAgentID != "Snake001" {
		t.Errorf("Expected agentID 'Snake001', got '%s'", capturedAgentID)
	}

	if capturedProgress["phase"] != "security" {
		t.Errorf("Expected phase 'security', got '%v'", capturedProgress["phase"])
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected result to be map[string]interface{}")
	}

	if resultMap["status"] != "recorded" {
		t.Errorf("Expected status 'recorded', got '%v'", resultMap["status"])
	}
}
