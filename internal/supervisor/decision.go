package supervisor

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/CLIAIMONITOR/internal/memory"
)

// OperationalMode defines how the Captain coordinates agents
type OperationalMode string

const (
	ModeDirectControl  OperationalMode = "direct"       // High risk, low familiarity - Captain spawns with specific instructions
	ModeTaskDispatch   OperationalMode = "dispatch"     // Low risk, high familiarity - Use task dispatch system
	ModeHierarchical   OperationalMode = "hierarchical" // Large scope, complex - Opus agents direct Sonnet workers
)

// Effort estimation constants (hours per finding by severity)
const (
	HoursPerCriticalFinding = 2.0  // Critical findings require more investigation
	HoursPerHighFinding     = 1.0  // High findings are significant but less complex
	HoursPerMediumFinding   = 0.5  // Medium findings are routine fixes
	HoursPerLowFinding      = 0.25 // Low findings are minor improvements
	EffortBufferFactor      = 1.2  // 20% buffer for uncertainty
)

// DecisionEngine analyzes reconnaissance reports and recommends actions
type DecisionEngine interface {
	// Analyze Snake report and recommend action
	AnalyzeReport(ctx context.Context, report *ReconReport) (*ActionPlan, error)

	// Determine operational mode based on findings
	SelectMode(findings []*ReconFinding) OperationalMode

	// Recommend which agents to spawn
	RecommendAgents(plan *ActionPlan) []*AgentRecommendation

	// Check if human escalation needed
	RequiresEscalation(findings []*ReconFinding) (bool, string)
}

// ActionPlan describes recommended actions from a reconnaissance report
type ActionPlan struct {
	ID               string              `json:"id"`
	ReportID         string              `json:"report_id"`
	Mode             OperationalMode     `json:"mode"`
	Priority         string              `json:"priority"` // critical, high, medium, low
	ImmediateActions []PlannedAction     `json:"immediate_actions"`
	ShortTermActions []PlannedAction     `json:"short_term_actions"`
	LongTermActions  []PlannedAction     `json:"long_term_actions"`
	EstimatedAgents  int                 `json:"estimated_agents"`
	EstimatedHours   float64             `json:"estimated_hours"`
	RequiresHuman    bool                `json:"requires_human"`
	EscalationReason string              `json:"escalation_reason,omitempty"`
	CreatedAt        time.Time           `json:"created_at"`
	AgentRecommendations []*AgentRecommendation `json:"agent_recommendations"`
}

// PlannedAction describes a single remediation action
type PlannedAction struct {
	ID              string   `json:"id"`
	Description     string   `json:"description"`
	FindingIDs      []string `json:"finding_ids"` // Which findings this addresses
	EstimatedHours  float64  `json:"estimated_hours"`
	RequiresOpus    bool     `json:"requires_opus"`
	RequiresSecurity bool    `json:"requires_security"`
}

// AgentRecommendation recommends which agent to spawn for a task
type AgentRecommendation struct {
	AgentType   string   `json:"agent_type"` // SNTGreen, SNTRed, OpusGreen, etc.
	Task        string   `json:"task"`
	Priority    int      `json:"priority"` // 1 = highest
	FindingIDs  []string `json:"finding_ids"`
	Rationale   string   `json:"rationale"`
}

// ReconReport represents a parsed reconnaissance report from Snake
type ReconReport struct {
	ID               string                 `json:"id"`
	AgentID          string                 `json:"agent_id"`
	Environment      string                 `json:"environment"`
	Timestamp        time.Time              `json:"timestamp"`
	Mission          string                 `json:"mission"`
	Findings         *ReconFindings         `json:"findings"`
	Summary          *ReconSummary          `json:"summary"`
	Recommendations  *ReconRecommendations  `json:"recommendations"`
}

// ReconFindings contains categorized findings
type ReconFindings struct {
	Critical []*ReconFinding `json:"critical"`
	High     []*ReconFinding `json:"high"`
	Medium   []*ReconFinding `json:"medium"`
	Low      []*ReconFinding `json:"low"`
}

// ReconFinding represents a single finding
type ReconFinding struct {
	ID             string `json:"id"`
	Type           string `json:"type"` // security, architecture, dependency, process
	Description    string `json:"description"`
	Location       string `json:"location"`
	Recommendation string `json:"recommendation"`
}

// ReconSummary contains scan statistics
type ReconSummary struct {
	TotalFilesScanned int      `json:"total_files_scanned"`
	Languages         []string `json:"languages"`
	Frameworks        []string `json:"frameworks"`
	TestCoverage      string   `json:"test_coverage"`
	SecurityScore     string   `json:"security_score"`
}

// ReconRecommendations contains prioritized recommendations
type ReconRecommendations struct {
	Immediate []string `json:"immediate"`
	ShortTerm []string `json:"short_term"`
	LongTerm  []string `json:"long_term"`
}

// StandardDecisionEngine implements the Captain's decision framework
type StandardDecisionEngine struct {
	memDB memory.MemoryDB
}

// NewDecisionEngine creates a new decision engine
func NewDecisionEngine(memDB memory.MemoryDB) DecisionEngine {
	return &StandardDecisionEngine{
		memDB: memDB,
	}
}

// AnalyzeReport analyzes a reconnaissance report and produces an action plan
func (e *StandardDecisionEngine) AnalyzeReport(ctx context.Context, report *ReconReport) (*ActionPlan, error) {
	if report == nil || report.Findings == nil {
		return nil, fmt.Errorf("invalid report: missing findings")
	}

	// Collect all findings
	allFindings := e.collectAllFindings(report.Findings)

	// Step 1: ASSESS severity
	priority := e.assessPriority(report.Findings)

	// Step 2: Determine mode
	mode := e.SelectMode(allFindings)

	// Step 3: ESTIMATE effort
	estimatedHours := e.estimateEffort(report.Findings, report.Recommendations)

	// Step 4: Check escalation needs
	requiresHuman, escalationReason := e.RequiresEscalation(allFindings)

	// Step 5: Build planned actions
	immediate, shortTerm, longTerm := e.buildPlannedActions(report)

	// Count estimated agents
	estimatedAgents := e.estimateAgentCount(estimatedHours, mode)

	// Create action plan
	plan := &ActionPlan{
		ID:               fmt.Sprintf("plan-%d", time.Now().Unix()),
		ReportID:         report.ID,
		Mode:             mode,
		Priority:         priority,
		ImmediateActions: immediate,
		ShortTermActions: shortTerm,
		LongTermActions:  longTerm,
		EstimatedAgents:  estimatedAgents,
		EstimatedHours:   estimatedHours,
		RequiresHuman:    requiresHuman,
		EscalationReason: escalationReason,
		CreatedAt:        time.Now(),
	}

	// Step 6: SELECT agents
	plan.AgentRecommendations = e.RecommendAgents(plan)

	return plan, nil
}

// SelectMode determines operational mode based on findings
func (e *StandardDecisionEngine) SelectMode(findings []*ReconFinding) OperationalMode {
	// Count critical and high severity findings
	securityCount := 0

	for _, f := range findings {
		if f.Type == "security" {
			securityCount++
		}
	}

	// If multiple security findings or complex security issues = Direct Control
	if securityCount > 3 {
		// Critical security = Direct Control
		return ModeDirectControl
	}

	// Estimate complexity based on finding count
	totalFindings := len(findings)
	if totalFindings > 20 {
		// Large scope = Hierarchical
		return ModeHierarchical
	}

	// Default to task dispatch for routine work
	return ModeTaskDispatch
}

// RecommendAgents recommends which agents to spawn
func (e *StandardDecisionEngine) RecommendAgents(plan *ActionPlan) []*AgentRecommendation {
	recommendations := make([]*AgentRecommendation, 0)
	priority := 1

	// Process immediate actions
	for _, action := range plan.ImmediateActions {
		agentType := e.selectAgentType(action)
		rec := &AgentRecommendation{
			AgentType:  agentType,
			Task:       action.Description,
			Priority:   priority,
			FindingIDs: action.FindingIDs,
			Rationale:  e.buildRationale(action, agentType),
		}
		recommendations = append(recommendations, rec)
		priority++
	}

	// Process short-term actions (lower priority)
	for _, action := range plan.ShortTermActions {
		agentType := e.selectAgentType(action)
		rec := &AgentRecommendation{
			AgentType:  agentType,
			Task:       action.Description,
			Priority:   priority,
			FindingIDs: action.FindingIDs,
			Rationale:  e.buildRationale(action, agentType),
		}
		recommendations = append(recommendations, rec)
		priority++
	}

	return recommendations
}

// RequiresEscalation checks if human intervention is needed
func (e *StandardDecisionEngine) RequiresEscalation(findings []*ReconFinding) (bool, string) {
	for _, f := range findings {
		// Critical security in production
		if f.Type == "security" && containsKeyword(f.Description, []string{"production", "live", "customer-facing"}) {
			return true, "Critical security vulnerability in production environment"
		}

		// Architectural decisions with high impact
		if f.Type == "architecture" && containsKeyword(f.Description, []string{"migration", "rewrite", "replace"}) {
			return true, "Architectural decision with potential high impact"
		}

		// Customer-facing changes
		if containsKeyword(f.Description, []string{"customer", "user-facing", "public API"}) {
			return true, "Changes affect customer-facing functionality"
		}

		// Data loss risks
		if containsKeyword(f.Description, []string{"data loss", "irreversible", "destructive"}) {
			return true, "Risk of data loss or irreversible changes"
		}
	}

	return false, ""
}

// Helper functions

func (e *StandardDecisionEngine) collectAllFindings(findings *ReconFindings) []*ReconFinding {
	all := make([]*ReconFinding, 0)
	all = append(all, findings.Critical...)
	all = append(all, findings.High...)
	all = append(all, findings.Medium...)
	all = append(all, findings.Low...)
	return all
}

func (e *StandardDecisionEngine) assessPriority(findings *ReconFindings) string {
	if len(findings.Critical) > 0 {
		return "critical"
	}
	if len(findings.High) > 3 {
		return "high"
	}
	if len(findings.High) > 0 || len(findings.Medium) > 5 {
		return "medium"
	}
	return "low"
}

func (e *StandardDecisionEngine) estimateEffort(findings *ReconFindings, recs *ReconRecommendations) float64 {
	// Estimation based on finding counts using defined constants
	hours := 0.0

	// Critical findings
	hours += float64(len(findings.Critical)) * HoursPerCriticalFinding

	// High findings
	hours += float64(len(findings.High)) * HoursPerHighFinding

	// Medium findings
	hours += float64(len(findings.Medium)) * HoursPerMediumFinding

	// Low findings
	hours += float64(len(findings.Low)) * HoursPerLowFinding

	// Add buffer for coordination
	hours *= EffortBufferFactor

	return hours
}

func (e *StandardDecisionEngine) estimateAgentCount(hours float64, mode OperationalMode) int {
	if hours < 2 {
		return 1
	}
	if hours < 8 {
		return 2
	}
	if mode == ModeHierarchical {
		// 1 Opus lead + multiple Sonnet workers
		return 1 + int(hours/4)
	}
	// Parallel workers
	return int(hours/4) + 1
}

func (e *StandardDecisionEngine) buildPlannedActions(report *ReconReport) ([]PlannedAction, []PlannedAction, []PlannedAction) {
	immediate := make([]PlannedAction, 0)
	shortTerm := make([]PlannedAction, 0)
	longTerm := make([]PlannedAction, 0)

	actionID := 1

	// Process immediate recommendations
	for _, rec := range report.Recommendations.Immediate {
		// Match recommendation to findings
		findingIDs := e.matchRecommendationToFindings(rec, report.Findings.Critical, report.Findings.High)
		action := PlannedAction{
			ID:              fmt.Sprintf("action-%d", actionID),
			Description:     rec,
			FindingIDs:      findingIDs,
			EstimatedHours:  1.5,
			RequiresSecurity: containsKeyword(rec, []string{"security", "vulnerability", "injection", "XSS"}),
			RequiresOpus:    containsKeyword(rec, []string{"architecture", "design", "refactor"}),
		}
		immediate = append(immediate, action)
		actionID++
	}

	// Process short-term recommendations
	for _, rec := range report.Recommendations.ShortTerm {
		findingIDs := e.matchRecommendationToFindings(rec, report.Findings.Medium)
		action := PlannedAction{
			ID:             fmt.Sprintf("action-%d", actionID),
			Description:    rec,
			FindingIDs:     findingIDs,
			EstimatedHours: 3.0,
			RequiresOpus:   containsKeyword(rec, []string{"architecture", "design"}),
		}
		shortTerm = append(shortTerm, action)
		actionID++
	}

	// Process long-term recommendations
	for _, rec := range report.Recommendations.LongTerm {
		findingIDs := e.matchRecommendationToFindings(rec, report.Findings.Low)
		action := PlannedAction{
			ID:             fmt.Sprintf("action-%d", actionID),
			Description:    rec,
			FindingIDs:     findingIDs,
			EstimatedHours: 8.0,
			RequiresOpus:   true, // Long-term work often needs architectural oversight
		}
		longTerm = append(longTerm, action)
		actionID++
	}

	return immediate, shortTerm, longTerm
}

func (e *StandardDecisionEngine) matchRecommendationToFindings(rec string, findingGroups ...[]*ReconFinding) []string {
	ids := make([]string, 0)

	// Simple keyword matching - in production this would be more sophisticated
	for _, group := range findingGroups {
		for _, finding := range group {
			if containsKeyword(rec, []string{finding.Type}) {
				ids = append(ids, finding.ID)
			}
		}
	}

	return ids
}

func (e *StandardDecisionEngine) selectAgentType(action PlannedAction) string {
	// Security fixes → Red agents
	if action.RequiresSecurity {
		if action.RequiresOpus {
			return "OpusRed"
		}
		return "SNTRed"
	}

	// Architecture work → Opus Green
	if action.RequiresOpus {
		return "OpusGreen"
	}

	// General coding → Sonnet Green
	return "SNTGreen"
}

func (e *StandardDecisionEngine) buildRationale(action PlannedAction, agentType string) string {
	if action.RequiresSecurity {
		return fmt.Sprintf("Security-related task requiring %s expertise", agentType)
	}
	if action.RequiresOpus {
		return fmt.Sprintf("Complex architectural work best handled by %s", agentType)
	}
	return fmt.Sprintf("Standard implementation task suitable for %s", agentType)
}

// containsKeyword checks if text contains any of the keywords (case-insensitive)
func containsKeyword(text string, keywords []string) bool {
	lowerText := strings.ToLower(text)
	for _, kw := range keywords {
		if strings.Contains(lowerText, strings.ToLower(kw)) {
			return true
		}
	}
	return false
}

