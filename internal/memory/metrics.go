package memory

import (
	"fmt"
)

// Additional AgentType constants for metrics segmentation
// Note: AgentTypeCaptain is defined in learning.go
const (
	AgentTypeSGT           = "sgt"
	AgentTypeSpawnedWindow = "spawned_window"
	AgentTypeSubagent      = "subagent"
)

// RecordMetricsHistory records agent metrics to the metrics_history table
func (m *SQLiteMemoryDB) RecordMetricsHistory(agentID, model string, tokensUsed int64, estimatedCost float64, taskID string) error {
	query := `
		INSERT INTO metrics_history (agent_id, model, task_id, tokens_used, estimated_cost)
		VALUES (?, ?, ?, ?, ?)
	`

	_, err := m.db.Exec(query, agentID, model, nullString(taskID), tokensUsed, estimatedCost)
	if err != nil {
		return fmt.Errorf("failed to record metrics history: %w", err)
	}

	return nil
}

// RecordMetricsWithType records agent metrics with agent type classification
func (m *SQLiteMemoryDB) RecordMetricsWithType(agentID, model, agentType, parentAgent string, tokensUsed int64, estimatedCost float64, taskID string, assignmentID *int64) error {
	query := `
		INSERT INTO metrics_history (agent_id, model, agent_type, parent_agent, task_id, tokens_used, estimated_cost, assignment_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	var assignID interface{}
	if assignmentID != nil {
		assignID = *assignmentID
	}

	_, err := m.db.Exec(query, agentID, model, agentType, nullString(parentAgent), nullString(taskID), tokensUsed, estimatedCost, assignID)
	if err != nil {
		return fmt.Errorf("failed to record metrics with type: %w", err)
	}

	return nil
}

// GetMetricsByModel retrieves aggregated metrics per model from the metrics_by_model view
// If modelFilter is provided and not empty, it filters to that specific model
func (m *SQLiteMemoryDB) GetMetricsByModel(modelFilter string) ([]*ModelMetrics, error) {
	var query string
	var args []interface{}

	if modelFilter != "" {
		query = `
			SELECT model, report_count, total_tokens, total_cost, avg_tokens_per_report
			FROM metrics_by_model
			WHERE model = ?
			ORDER BY total_cost DESC
		`
		args = []interface{}{modelFilter}
	} else {
		query = `
			SELECT model, report_count, total_tokens, total_cost, avg_tokens_per_report
			FROM metrics_by_model
			ORDER BY total_cost DESC
		`
	}

	rows, err := m.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query metrics by model: %w", err)
	}
	defer rows.Close()

	var metrics []*ModelMetrics
	for rows.Next() {
		metric := &ModelMetrics{}
		err := rows.Scan(
			&metric.Model,
			&metric.ReportCount,
			&metric.TotalTokens,
			&metric.TotalCost,
			&metric.AvgTokensPerReport,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan metrics row: %w", err)
		}
		metrics = append(metrics, metric)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating metrics rows: %w", err)
	}

	return metrics, nil
}

// AgentTypeMetrics represents aggregated metrics by agent type
type AgentTypeMetrics struct {
	AgentType         string  `json:"agent_type"`
	AgentCount        int     `json:"agent_count"`
	ReportCount       int     `json:"report_count"`
	TotalTokens       int64   `json:"total_tokens"`
	TotalCost         float64 `json:"total_cost"`
	AvgTokensPerReport float64 `json:"avg_tokens_per_report"`
}

// AgentMetricsSummary represents metrics for an individual agent
type AgentMetricsSummary struct {
	AgentID     string  `json:"agent_id"`
	AgentType   string  `json:"agent_type"`
	Model       string  `json:"model"`
	ParentAgent string  `json:"parent_agent,omitempty"`
	ReportCount int     `json:"report_count"`
	TotalTokens int64   `json:"total_tokens"`
	TotalCost   float64 `json:"total_cost"`
	FirstReport string  `json:"first_report"`
	LastReport  string  `json:"last_report"`
}

// GetMetricsByAgentType retrieves aggregated metrics by agent type
func (m *SQLiteMemoryDB) GetMetricsByAgentType() ([]*AgentTypeMetrics, error) {
	query := `
		SELECT agent_type, agent_count, report_count, total_tokens, total_cost, avg_tokens_per_report
		FROM metrics_by_agent_type
		ORDER BY total_cost DESC
	`

	rows, err := m.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query metrics by agent type: %w", err)
	}
	defer rows.Close()

	var metrics []*AgentTypeMetrics
	for rows.Next() {
		metric := &AgentTypeMetrics{}
		err := rows.Scan(
			&metric.AgentType,
			&metric.AgentCount,
			&metric.ReportCount,
			&metric.TotalTokens,
			&metric.TotalCost,
			&metric.AvgTokensPerReport,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan agent type metrics: %w", err)
		}
		metrics = append(metrics, metric)
	}

	return metrics, rows.Err()
}

// GetMetricsByAgent retrieves metrics rollup for all agents
func (m *SQLiteMemoryDB) GetMetricsByAgent() ([]*AgentMetricsSummary, error) {
	query := `
		SELECT agent_id, agent_type, model, parent_agent, report_count, total_tokens, total_cost, first_report, last_report
		FROM metrics_by_agent
		ORDER BY total_cost DESC
	`

	rows, err := m.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query metrics by agent: %w", err)
	}
	defer rows.Close()

	var metrics []*AgentMetricsSummary
	for rows.Next() {
		metric := &AgentMetricsSummary{}
		var parentAgent, firstReport, lastReport *string
		err := rows.Scan(
			&metric.AgentID,
			&metric.AgentType,
			&metric.Model,
			&parentAgent,
			&metric.ReportCount,
			&metric.TotalTokens,
			&metric.TotalCost,
			&firstReport,
			&lastReport,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan agent metrics: %w", err)
		}
		if parentAgent != nil {
			metric.ParentAgent = *parentAgent
		}
		if firstReport != nil {
			metric.FirstReport = *firstReport
		}
		if lastReport != nil {
			metric.LastReport = *lastReport
		}
		metrics = append(metrics, metric)
	}

	return metrics, rows.Err()
}
