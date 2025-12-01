package memory

import (
	"database/sql"
	"fmt"
)

// StoreDecision stores a human decision
func (m *SQLiteMemoryDB) StoreDecision(decision *HumanDecision) error {
	result, err := m.db.Exec(`
		INSERT INTO human_decisions (context, question, answer, decision_type, agent_id, related_task_id, repo_id)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		decision.Context,
		decision.Question,
		decision.Answer,
		nullString(decision.DecisionType),
		nullString(decision.AgentID),
		nullString(decision.RelatedTaskID),
		nullString(decision.RepoID),
	)
	if err != nil {
		return fmt.Errorf("failed to store decision: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get decision ID: %w", err)
	}
	decision.ID = id

	return nil
}

// GetRecentDecisions retrieves the most recent human decisions
func (m *SQLiteMemoryDB) GetRecentDecisions(limit int) ([]*HumanDecision, error) {
	rows, err := m.db.Query(`
		SELECT id, context, question, answer, decision_type, agent_id, related_task_id, repo_id, created_at
		FROM human_decisions
		ORDER BY created_at DESC
		LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query recent decisions: %w", err)
	}
	defer rows.Close()

	return scanHumanDecisions(rows)
}

// GetDecisionsByAgent retrieves decisions related to a specific agent
func (m *SQLiteMemoryDB) GetDecisionsByAgent(agentID string, limit int) ([]*HumanDecision, error) {
	rows, err := m.db.Query(`
		SELECT id, context, question, answer, decision_type, agent_id, related_task_id, repo_id, created_at
		FROM human_decisions
		WHERE agent_id = ?
		ORDER BY created_at DESC
		LIMIT ?`,
		agentID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query agent decisions: %w", err)
	}
	defer rows.Close()

	return scanHumanDecisions(rows)
}

// CreateDeployment creates a new deployment record
func (m *SQLiteMemoryDB) CreateDeployment(deployment *Deployment) error {
	result, err := m.db.Exec(`
		INSERT INTO deployments (repo_id, deployment_plan, status, agent_configs)
		VALUES (?, ?, ?, ?)`,
		deployment.RepoID,
		deployment.DeploymentPlan,
		deployment.Status,
		nullString(deployment.AgentConfigs),
	)
	if err != nil {
		return fmt.Errorf("failed to create deployment: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get deployment ID: %w", err)
	}
	deployment.ID = id

	return nil
}

// GetDeployment retrieves a deployment by ID
func (m *SQLiteMemoryDB) GetDeployment(deploymentID int64) (*Deployment, error) {
	var deployment Deployment
	var approvedAt, executedAt sql.NullTime
	var agentConfigs, result sql.NullString

	err := m.db.QueryRow(`
		SELECT id, repo_id, deployment_plan, proposed_at, approved_at, executed_at,
		       status, agent_configs, result
		FROM deployments
		WHERE id = ?`,
		deploymentID,
	).Scan(
		&deployment.ID,
		&deployment.RepoID,
		&deployment.DeploymentPlan,
		&deployment.ProposedAt,
		&approvedAt,
		&executedAt,
		&deployment.Status,
		&agentConfigs,
		&result,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("deployment not found: %d", deploymentID)
		}
		return nil, fmt.Errorf("failed to get deployment: %w", err)
	}

	if approvedAt.Valid {
		deployment.ApprovedAt = &approvedAt.Time
	}
	if executedAt.Valid {
		deployment.ExecutedAt = &executedAt.Time
	}
	deployment.AgentConfigs = agentConfigs.String
	deployment.Result = result.String

	return &deployment, nil
}

// GetRecentDeployments retrieves recent deployments for a repo
func (m *SQLiteMemoryDB) GetRecentDeployments(repoID string, limit int) ([]*Deployment, error) {
	rows, err := m.db.Query(`
		SELECT id, repo_id, deployment_plan, proposed_at, approved_at, executed_at,
		       status, agent_configs, result
		FROM deployments
		WHERE repo_id = ?
		ORDER BY proposed_at DESC
		LIMIT ?`,
		repoID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query recent deployments: %w", err)
	}
	defer rows.Close()

	return scanDeployments(rows)
}

// UpdateDeploymentStatus updates the status of a deployment
func (m *SQLiteMemoryDB) UpdateDeploymentStatus(deploymentID int64, status string) error {
	query := `
		UPDATE deployments
		SET status = ?`

	args := []interface{}{status}

	// Set timestamps based on status
	switch status {
	case "approved":
		query += ", approved_at = CURRENT_TIMESTAMP"
	case "executing":
		query += ", executed_at = CURRENT_TIMESTAMP"
	}

	query += " WHERE id = ?"
	args = append(args, deploymentID)

	result, err := m.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("failed to update deployment status: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check affected rows: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("deployment not found: %d", deploymentID)
	}

	return nil
}

// Helper scanning functions

func scanHumanDecisions(rows *sql.Rows) ([]*HumanDecision, error) {
	var decisions []*HumanDecision
	for rows.Next() {
		var decision HumanDecision
		var decisionType, agentID, relatedTaskID, repoID sql.NullString

		err := rows.Scan(
			&decision.ID,
			&decision.Context,
			&decision.Question,
			&decision.Answer,
			&decisionType,
			&agentID,
			&relatedTaskID,
			&repoID,
			&decision.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan human decision: %w", err)
		}

		decision.DecisionType = decisionType.String
		decision.AgentID = agentID.String
		decision.RelatedTaskID = relatedTaskID.String
		decision.RepoID = repoID.String

		decisions = append(decisions, &decision)
	}

	return decisions, rows.Err()
}

func scanDeployments(rows *sql.Rows) ([]*Deployment, error) {
	var deployments []*Deployment
	for rows.Next() {
		var deployment Deployment
		var approvedAt, executedAt sql.NullTime
		var agentConfigs, result sql.NullString

		err := rows.Scan(
			&deployment.ID,
			&deployment.RepoID,
			&deployment.DeploymentPlan,
			&deployment.ProposedAt,
			&approvedAt,
			&executedAt,
			&deployment.Status,
			&agentConfigs,
			&result,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan deployment: %w", err)
		}

		if approvedAt.Valid {
			deployment.ApprovedAt = &approvedAt.Time
		}
		if executedAt.Valid {
			deployment.ExecutedAt = &executedAt.Time
		}
		deployment.AgentConfigs = agentConfigs.String
		deployment.Result = result.String

		deployments = append(deployments, &deployment)
	}

	return deployments, rows.Err()
}
