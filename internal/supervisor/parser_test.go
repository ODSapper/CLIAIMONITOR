package supervisor

import (
	"testing"
)

func TestParseYAML(t *testing.T) {
	parser := NewReportParser()

	yamlData := `
snake_report:
  agent_id: "Snake001"
  environment: "test-env"
  timestamp: "2025-12-02T10:30:00Z"
  mission: "initial_recon"
  findings:
    critical:
      - id: "VULN-001"
        type: "security"
        description: "SQL injection vulnerability"
        location: "src/auth/login.go:45"
        recommendation: "Use parameterized queries"
    high: []
    medium: []
    low: []
  summary:
    total_files_scanned: 342
    languages: ["go", "typescript"]
    frameworks: ["chi", "react"]
    test_coverage: "23%"
    security_score: "C"
  recommendations:
    immediate:
      - "Patch SQL injection"
    short_term: []
    long_term: []
`

	report, err := parser.ParseYAML([]byte(yamlData))
	if err != nil {
		t.Fatalf("ParseYAML() error = %v", err)
	}

	// Verify basic fields
	if report.AgentID != "Snake001" {
		t.Errorf("report.AgentID = %v, want Snake001", report.AgentID)
	}

	if report.Environment != "test-env" {
		t.Errorf("report.Environment = %v, want test-env", report.Environment)
	}

	if report.Mission != "initial_recon" {
		t.Errorf("report.Mission = %v, want initial_recon", report.Mission)
	}

	// Verify findings
	if len(report.Findings.Critical) != 1 {
		t.Fatalf("len(report.Findings.Critical) = %v, want 1", len(report.Findings.Critical))
	}

	finding := report.Findings.Critical[0]
	if finding.ID != "VULN-001" {
		t.Errorf("finding.ID = %v, want VULN-001", finding.ID)
	}

	if finding.Type != "security" {
		t.Errorf("finding.Type = %v, want security", finding.Type)
	}

	// Verify summary
	if report.Summary.TotalFilesScanned != 342 {
		t.Errorf("report.Summary.TotalFilesScanned = %v, want 342", report.Summary.TotalFilesScanned)
	}

	if len(report.Summary.Languages) != 2 {
		t.Errorf("len(report.Summary.Languages) = %v, want 2", len(report.Summary.Languages))
	}

	// Verify recommendations
	if len(report.Recommendations.Immediate) != 1 {
		t.Errorf("len(report.Recommendations.Immediate) = %v, want 1", len(report.Recommendations.Immediate))
	}
}

func TestParseJSON(t *testing.T) {
	parser := NewReportParser()

	jsonData := `{
  "agent_id": "Snake002",
  "environment": "customer-acme",
  "timestamp": "2025-12-02T11:00:00Z",
  "mission": "security_audit",
  "findings": {
    "critical": [],
    "high": [
      {
        "id": "ARCH-001",
        "type": "architecture",
        "description": "No rate limiting",
        "location": "src/api/",
        "recommendation": "Add rate limiter"
      }
    ],
    "medium": [],
    "low": []
  },
  "summary": {
    "total_files_scanned": 150,
    "languages": ["go"],
    "frameworks": ["chi"],
    "test_coverage": "45%",
    "security_score": "B"
  },
  "recommendations": {
    "immediate": ["Add rate limiting"],
    "short_term": ["Improve test coverage"],
    "long_term": []
  }
}`

	report, err := parser.ParseJSON([]byte(jsonData))
	if err != nil {
		t.Fatalf("ParseJSON() error = %v", err)
	}

	if report.AgentID != "Snake002" {
		t.Errorf("report.AgentID = %v, want Snake002", report.AgentID)
	}

	if len(report.Findings.High) != 1 {
		t.Fatalf("len(report.Findings.High) = %v, want 1", len(report.Findings.High))
	}

	if report.Summary.SecurityScore != "B" {
		t.Errorf("report.Summary.SecurityScore = %v, want B", report.Summary.SecurityScore)
	}
}

func TestParseMCPReport(t *testing.T) {
	parser := NewReportParser()

	params := map[string]interface{}{
		"agent_id":    "Snake003",
		"environment": "internal-cliaimonitor",
		"mission":     "code_review",
		"timestamp":   "2025-12-02T12:00:00Z",
		"findings": map[string]interface{}{
			"critical": []interface{}{},
			"high":     []interface{}{},
			"medium": []interface{}{
				map[string]interface{}{
					"id":             "QUALITY-001",
					"type":           "code_quality",
					"description":    "Missing error handling",
					"location":       "internal/handlers/api.go",
					"recommendation": "Add proper error checks",
				},
			},
			"low": []interface{}{},
		},
		"summary": map[string]interface{}{
			"total_files_scanned": 200.0,
			"languages":           []interface{}{"go", "yaml"},
			"frameworks":          []interface{}{"gorilla/mux"},
			"test_coverage":       "65%",
			"security_score":      "A",
		},
		"recommendations": map[string]interface{}{
			"immediate":  []interface{}{},
			"short_term": []interface{}{"Add error handling"},
			"long_term":  []interface{}{"Refactor API layer"},
		},
	}

	report, err := parser.ParseMCPReport(params)
	if err != nil {
		t.Fatalf("ParseMCPReport() error = %v", err)
	}

	if report.AgentID != "Snake003" {
		t.Errorf("report.AgentID = %v, want Snake003", report.AgentID)
	}

	if len(report.Findings.Medium) != 1 {
		t.Fatalf("len(report.Findings.Medium) = %v, want 1", len(report.Findings.Medium))
	}

	if report.Summary.TotalFilesScanned != 200 {
		t.Errorf("report.Summary.TotalFilesScanned = %v, want 200", report.Summary.TotalFilesScanned)
	}

	if len(report.Recommendations.ShortTerm) != 1 {
		t.Errorf("len(report.Recommendations.ShortTerm) = %v, want 1", len(report.Recommendations.ShortTerm))
	}
}

func TestValidateReport(t *testing.T) {
	tests := []struct {
		name      string
		report    *ReconReport
		wantError bool
	}{
		{
			name:      "nil report fails validation",
			report:    nil,
			wantError: true,
		},
		{
			name: "missing agent_id fails validation",
			report: &ReconReport{
				Environment:     "test",
				Mission:         "test",
				Findings:        &ReconFindings{},
				Summary:         &ReconSummary{},
				Recommendations: &ReconRecommendations{},
			},
			wantError: true,
		},
		{
			name: "missing environment fails validation",
			report: &ReconReport{
				AgentID:         "Snake001",
				Mission:         "test",
				Findings:        &ReconFindings{},
				Summary:         &ReconSummary{},
				Recommendations: &ReconRecommendations{},
			},
			wantError: true,
		},
		{
			name: "valid report passes validation",
			report: &ReconReport{
				AgentID:     "Snake001",
				Environment: "test",
				Mission:     "test",
				Findings: &ReconFindings{
					Critical: []*ReconFinding{},
					High:     []*ReconFinding{},
					Medium:   []*ReconFinding{},
					Low:      []*ReconFinding{},
				},
				Summary: &ReconSummary{},
				Recommendations: &ReconRecommendations{
					Immediate: []string{},
					ShortTerm: []string{},
					LongTerm:  []string{},
				},
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateReport(tt.report)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateReport() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}
