package supervisor

import (
	"encoding/json"
	"fmt"
	"time"

	"gopkg.in/yaml.v3"
)

// ReportParser parses Snake reconnaissance reports from various formats
type ReportParser interface {
	// Parse YAML report into structured format
	ParseYAML(data []byte) (*ReconReport, error)

	// Parse JSON report into structured format
	ParseJSON(data []byte) (*ReconReport, error)

	// Parse from MCP tool call parameters
	ParseMCPReport(params map[string]interface{}) (*ReconReport, error)
}

// StandardReportParser implements report parsing
type StandardReportParser struct{}

// NewReportParser creates a new report parser
func NewReportParser() ReportParser {
	return &StandardReportParser{}
}

// ParseYAML parses a YAML reconnaissance report
func (p *StandardReportParser) ParseYAML(data []byte) (*ReconReport, error) {
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Extract snake_report section
	snakeReport, ok := raw["snake_report"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("missing snake_report section")
	}

	return p.parseReportMap(snakeReport)
}

// ParseJSON parses a JSON reconnaissance report
func (p *StandardReportParser) ParseJSON(data []byte) (*ReconReport, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return p.parseReportMap(raw)
}

// ParseMCPReport parses report from MCP tool call parameters
func (p *StandardReportParser) ParseMCPReport(params map[string]interface{}) (*ReconReport, error) {
	return p.parseReportMap(params)
}

// parseReportMap converts a map to ReconReport struct
func (p *StandardReportParser) parseReportMap(data map[string]interface{}) (*ReconReport, error) {
	report := &ReconReport{}

	// Extract basic fields
	if v, ok := data["agent_id"].(string); ok {
		report.AgentID = v
	}

	if v, ok := data["environment"].(string); ok {
		report.Environment = v
	}

	if v, ok := data["mission"].(string); ok {
		report.Mission = v
	}

	// Parse timestamp
	if v, ok := data["timestamp"].(string); ok {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			// Try other formats
			t, err = time.Parse("2006-01-02T15:04:05Z", v)
			if err != nil {
				return nil, fmt.Errorf("failed to parse timestamp: %w", err)
			}
		}
		report.Timestamp = t
	} else {
		report.Timestamp = time.Now()
	}

	// Generate ID if not provided
	report.ID = fmt.Sprintf("recon-%d", report.Timestamp.Unix())

	// Parse findings
	if findingsData, ok := data["findings"].(map[string]interface{}); ok {
		findings, err := p.parseFindings(findingsData)
		if err != nil {
			return nil, fmt.Errorf("failed to parse findings: %w", err)
		}
		report.Findings = findings
	} else {
		report.Findings = &ReconFindings{
			Critical: make([]*ReconFinding, 0),
			High:     make([]*ReconFinding, 0),
			Medium:   make([]*ReconFinding, 0),
			Low:      make([]*ReconFinding, 0),
		}
	}

	// Parse summary
	if summaryData, ok := data["summary"].(map[string]interface{}); ok {
		summary, err := p.parseSummary(summaryData)
		if err != nil {
			return nil, fmt.Errorf("failed to parse summary: %w", err)
		}
		report.Summary = summary
	} else {
		report.Summary = &ReconSummary{}
	}

	// Parse recommendations
	if recsData, ok := data["recommendations"].(map[string]interface{}); ok {
		recs, err := p.parseRecommendations(recsData)
		if err != nil {
			return nil, fmt.Errorf("failed to parse recommendations: %w", err)
		}
		report.Recommendations = recs
	} else {
		report.Recommendations = &ReconRecommendations{
			Immediate: make([]string, 0),
			ShortTerm: make([]string, 0),
			LongTerm:  make([]string, 0),
		}
	}

	return report, nil
}

// parseFindings extracts findings from map
func (p *StandardReportParser) parseFindings(data map[string]interface{}) (*ReconFindings, error) {
	findings := &ReconFindings{
		Critical: make([]*ReconFinding, 0),
		High:     make([]*ReconFinding, 0),
		Medium:   make([]*ReconFinding, 0),
		Low:      make([]*ReconFinding, 0),
	}

	// Parse each severity level
	if critical, ok := data["critical"].([]interface{}); ok {
		for _, item := range critical {
			if finding, err := p.parseFinding(item); err == nil {
				findings.Critical = append(findings.Critical, finding)
			}
		}
	}

	if high, ok := data["high"].([]interface{}); ok {
		for _, item := range high {
			if finding, err := p.parseFinding(item); err == nil {
				findings.High = append(findings.High, finding)
			}
		}
	}

	if medium, ok := data["medium"].([]interface{}); ok {
		for _, item := range medium {
			if finding, err := p.parseFinding(item); err == nil {
				findings.Medium = append(findings.Medium, finding)
			}
		}
	}

	if low, ok := data["low"].([]interface{}); ok {
		for _, item := range low {
			if finding, err := p.parseFinding(item); err == nil {
				findings.Low = append(findings.Low, finding)
			}
		}
	}

	return findings, nil
}

// parseFinding converts a map to ReconFinding
func (p *StandardReportParser) parseFinding(item interface{}) (*ReconFinding, error) {
	findingMap, ok := item.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid finding format")
	}

	finding := &ReconFinding{}

	if v, ok := findingMap["id"].(string); ok {
		finding.ID = v
	} else {
		// Generate ID if not provided
		finding.ID = fmt.Sprintf("finding-%d", time.Now().UnixNano())
	}

	if v, ok := findingMap["type"].(string); ok {
		finding.Type = v
	}

	if v, ok := findingMap["description"].(string); ok {
		finding.Description = v
	}

	if v, ok := findingMap["location"].(string); ok {
		finding.Location = v
	}

	if v, ok := findingMap["recommendation"].(string); ok {
		finding.Recommendation = v
	}

	return finding, nil
}

// parseSummary extracts summary information
func (p *StandardReportParser) parseSummary(data map[string]interface{}) (*ReconSummary, error) {
	summary := &ReconSummary{}

	if v, ok := data["total_files_scanned"].(float64); ok {
		summary.TotalFilesScanned = int(v)
	} else if v, ok := data["total_files_scanned"].(int); ok {
		summary.TotalFilesScanned = v
	}

	if languages, ok := data["languages"].([]interface{}); ok {
		summary.Languages = make([]string, 0, len(languages))
		for _, lang := range languages {
			if langStr, ok := lang.(string); ok {
				summary.Languages = append(summary.Languages, langStr)
			}
		}
	}

	if frameworks, ok := data["frameworks"].([]interface{}); ok {
		summary.Frameworks = make([]string, 0, len(frameworks))
		for _, fw := range frameworks {
			if fwStr, ok := fw.(string); ok {
				summary.Frameworks = append(summary.Frameworks, fwStr)
			}
		}
	}

	if v, ok := data["test_coverage"].(string); ok {
		summary.TestCoverage = v
	}

	if v, ok := data["security_score"].(string); ok {
		summary.SecurityScore = v
	}

	return summary, nil
}

// parseRecommendations extracts recommendations
func (p *StandardReportParser) parseRecommendations(data map[string]interface{}) (*ReconRecommendations, error) {
	recs := &ReconRecommendations{
		Immediate: make([]string, 0),
		ShortTerm: make([]string, 0),
		LongTerm:  make([]string, 0),
	}

	if immediate, ok := data["immediate"].([]interface{}); ok {
		for _, rec := range immediate {
			if recStr, ok := rec.(string); ok {
				recs.Immediate = append(recs.Immediate, recStr)
			}
		}
	}

	if shortTerm, ok := data["short_term"].([]interface{}); ok {
		for _, rec := range shortTerm {
			if recStr, ok := rec.(string); ok {
				recs.ShortTerm = append(recs.ShortTerm, recStr)
			}
		}
	}

	if longTerm, ok := data["long_term"].([]interface{}); ok {
		for _, rec := range longTerm {
			if recStr, ok := rec.(string); ok {
				recs.LongTerm = append(recs.LongTerm, recStr)
			}
		}
	}

	return recs, nil
}

// ValidateReport checks if a report has required fields
func ValidateReport(report *ReconReport) error {
	if report == nil {
		return fmt.Errorf("report is nil")
	}

	if report.AgentID == "" {
		return fmt.Errorf("missing agent_id")
	}

	if report.Environment == "" {
		return fmt.Errorf("missing environment")
	}

	if report.Mission == "" {
		return fmt.Errorf("missing mission")
	}

	if report.Findings == nil {
		return fmt.Errorf("missing findings")
	}

	if report.Summary == nil {
		return fmt.Errorf("missing summary")
	}

	if report.Recommendations == nil {
		return fmt.Errorf("missing recommendations")
	}

	return nil
}
