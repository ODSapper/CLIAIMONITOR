package supervisor

import (
	"context"
	"testing"
	"time"
)

func TestSelectMode(t *testing.T) {
	engine := &StandardDecisionEngine{}

	tests := []struct {
		name     string
		findings []*ReconFinding
		expected OperationalMode
	}{
		{
			name: "many security findings trigger direct control",
			findings: []*ReconFinding{
				{Type: "security", Description: "SQL injection vulnerability"},
				{Type: "security", Description: "Authentication bypass"},
				{Type: "security", Description: "XSS vulnerability"},
				{Type: "security", Description: "CSRF vulnerability"},
			},
			expected: ModeDirectControl,
		},
		{
			name: "many findings trigger hierarchical mode",
			findings: generateFindings(25),
			expected: ModeHierarchical,
		},
		{
			name: "few findings use task dispatch",
			findings: []*ReconFinding{
				{Type: "code_quality", Description: "Missing tests"},
				{Type: "documentation", Description: "Missing README"},
			},
			expected: ModeTaskDispatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mode := engine.SelectMode(tt.findings)
			if mode != tt.expected {
				t.Errorf("SelectMode() = %v, want %v", mode, tt.expected)
			}
		})
	}
}

func TestRequiresEscalation(t *testing.T) {
	engine := &StandardDecisionEngine{}

	tests := []struct {
		name           string
		findings       []*ReconFinding
		shouldEscalate bool
		reasonContains string
	}{
		{
			name: "production security issue requires escalation",
			findings: []*ReconFinding{
				{Type: "security", Description: "Critical vulnerability in production database"},
			},
			shouldEscalate: true,
			reasonContains: "production",
		},
		{
			name: "architectural migration requires escalation",
			findings: []*ReconFinding{
				{Type: "architecture", Description: "Recommend migration to new framework"},
			},
			shouldEscalate: true,
			reasonContains: "Architectural",
		},
		{
			name: "customer-facing changes require escalation",
			findings: []*ReconFinding{
				{Type: "feature", Description: "Update customer-facing API endpoint"},
			},
			shouldEscalate: true,
			reasonContains: "customer",
		},
		{
			name: "routine fixes don't require escalation",
			findings: []*ReconFinding{
				{Type: "code_quality", Description: "Add unit tests"},
				{Type: "documentation", Description: "Update comments"},
			},
			shouldEscalate: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			escalate, reason := engine.RequiresEscalation(tt.findings)
			if escalate != tt.shouldEscalate {
				t.Errorf("RequiresEscalation() escalate = %v, want %v", escalate, tt.shouldEscalate)
			}
			if tt.shouldEscalate && !contains(reason, tt.reasonContains) {
				t.Errorf("RequiresEscalation() reason = %v, should contain %v", reason, tt.reasonContains)
			}
		})
	}
}

func TestAnalyzeReport(t *testing.T) {
	engine := &StandardDecisionEngine{}

	report := &ReconReport{
		ID:          "test-report-1",
		AgentID:     "Snake001",
		Environment: "test-env",
		Timestamp:   time.Now(),
		Mission:     "initial_recon",
		Findings: &ReconFindings{
			Critical: []*ReconFinding{
				{
					ID:             "VULN-001",
					Type:           "security",
					Description:    "SQL injection in login endpoint",
					Location:       "src/auth/login.go:45",
					Recommendation: "Use parameterized queries",
				},
			},
			High: []*ReconFinding{
				{
					ID:             "ARCH-001",
					Type:           "architecture",
					Description:    "No rate limiting on API endpoints",
					Recommendation: "Implement middleware rate limiter",
				},
			},
			Medium: []*ReconFinding{},
			Low:    []*ReconFinding{},
		},
		Summary: &ReconSummary{
			TotalFilesScanned: 342,
			Languages:         []string{"go", "typescript"},
			Frameworks:        []string{"chi", "react"},
			TestCoverage:      "23%",
			SecurityScore:     "C",
		},
		Recommendations: &ReconRecommendations{
			Immediate: []string{
				"Patch SQL injection (VULN-001)",
				"Add rate limiting (ARCH-001)",
			},
			ShortTerm: []string{
				"Increase test coverage to 60%",
			},
			LongTerm: []string{
				"Migrate to structured logging",
			},
		},
	}

	ctx := context.Background()
	plan, err := engine.AnalyzeReport(ctx, report)

	if err != nil {
		t.Fatalf("AnalyzeReport() error = %v", err)
	}

	// Verify plan basics
	if plan.ReportID != report.ID {
		t.Errorf("plan.ReportID = %v, want %v", plan.ReportID, report.ID)
	}

	if plan.Priority != "critical" {
		t.Errorf("plan.Priority = %v, want critical", plan.Priority)
	}

	if len(plan.ImmediateActions) != 2 {
		t.Errorf("len(plan.ImmediateActions) = %v, want 2", len(plan.ImmediateActions))
	}

	if len(plan.AgentRecommendations) == 0 {
		t.Error("plan.AgentRecommendations is empty")
	}

	// Verify security finding triggers OpusRed or SNTRed
	hasSecurityAgent := false
	for _, rec := range plan.AgentRecommendations {
		if rec.AgentType == "OpusRed" || rec.AgentType == "SNTRed" {
			hasSecurityAgent = true
			break
		}
	}
	if !hasSecurityAgent {
		t.Error("No security agent (OpusRed/SNTRed) recommended for security finding")
	}
}

func TestRecommendAgents(t *testing.T) {
	engine := &StandardDecisionEngine{}

	plan := &ActionPlan{
		ID:       "plan-test",
		ReportID: "report-test",
		Mode:     ModeDirectControl,
		Priority: "high",
		ImmediateActions: []PlannedAction{
			{
				ID:               "action-1",
				Description:      "Fix SQL injection vulnerability",
				FindingIDs:       []string{"VULN-001"},
				EstimatedHours:   2.0,
				RequiresSecurity: true,
			},
			{
				ID:             "action-2",
				Description:    "Refactor authentication module",
				FindingIDs:     []string{"ARCH-001"},
				EstimatedHours: 4.0,
				RequiresOpus:   true,
			},
		},
	}

	recs := engine.RecommendAgents(plan)

	if len(recs) != 2 {
		t.Fatalf("len(recommendations) = %v, want 2", len(recs))
	}

	// First action should get security agent
	if recs[0].AgentType != "SNTRed" && recs[0].AgentType != "OpusRed" {
		t.Errorf("First recommendation agent type = %v, want SNTRed or OpusRed", recs[0].AgentType)
	}

	// Second action should get Opus agent for architecture
	if recs[1].AgentType != "OpusGreen" {
		t.Errorf("Second recommendation agent type = %v, want OpusGreen", recs[1].AgentType)
	}

	// Verify priorities are assigned
	if recs[0].Priority != 1 {
		t.Errorf("First recommendation priority = %v, want 1", recs[0].Priority)
	}
	if recs[1].Priority != 2 {
		t.Errorf("Second recommendation priority = %v, want 2", recs[1].Priority)
	}
}

func TestEstimateEffort(t *testing.T) {
	engine := &StandardDecisionEngine{}

	tests := []struct {
		name     string
		findings *ReconFindings
		recs     *ReconRecommendations
		minHours float64
		maxHours float64
	}{
		{
			name: "critical findings estimate 2 hours each",
			findings: &ReconFindings{
				Critical: []*ReconFinding{{}, {}},
				High:     []*ReconFinding{},
				Medium:   []*ReconFinding{},
				Low:      []*ReconFinding{},
			},
			recs:     &ReconRecommendations{},
			minHours: 4.0,  // 2 critical * 2 hours * 1.2 buffer
			maxHours: 5.0,
		},
		{
			name: "mixed severity findings",
			findings: &ReconFindings{
				Critical: []*ReconFinding{{}},
				High:     []*ReconFinding{{}, {}},
				Medium:   []*ReconFinding{{}, {}, {}},
				Low:      []*ReconFinding{{}},
			},
			recs:     &ReconRecommendations{},
			minHours: 5.0,
			maxHours: 7.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hours := engine.estimateEffort(tt.findings, tt.recs)
			if hours < tt.minHours || hours > tt.maxHours {
				t.Errorf("estimateEffort() = %v, want between %v and %v", hours, tt.minHours, tt.maxHours)
			}
		})
	}
}

// Helper functions

func generateFindings(count int) []*ReconFinding {
	findings := make([]*ReconFinding, count)
	for i := 0; i < count; i++ {
		findings[i] = &ReconFinding{
			ID:          string(rune('A' + i)),
			Type:        "code_quality",
			Description: "Test finding",
		}
	}
	return findings
}
