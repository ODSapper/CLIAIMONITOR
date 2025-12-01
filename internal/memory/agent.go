package memory

import (
	"database/sql"
	"fmt"
)

// StoreAgentLearning stores a new agent learning
func (m *SQLiteMemoryDB) StoreAgentLearning(learning *AgentLearning) error {
	result, err := m.db.Exec(`
		INSERT INTO agent_learnings (agent_id, agent_type, category, title, content, repo_id)
		VALUES (?, ?, ?, ?, ?, ?)`,
		learning.AgentID,
		learning.AgentType,
		learning.Category,
		learning.Title,
		learning.Content,
		nullString(learning.RepoID),
	)
	if err != nil {
		return fmt.Errorf("failed to store agent learning: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get learning ID: %w", err)
	}
	learning.ID = id

	return nil
}

// GetAgentLearnings retrieves agent learnings with filters
func (m *SQLiteMemoryDB) GetAgentLearnings(filter LearnFilter) ([]*AgentLearning, error) {
	query := `
		SELECT id, agent_id, agent_type, category, title, content, repo_id, created_at
		FROM agent_learnings
		WHERE 1=1`
	var args []interface{}

	if filter.AgentID != "" {
		query += " AND agent_id = ?"
		args = append(args, filter.AgentID)
	}
	if filter.AgentType != "" {
		query += " AND agent_type = ?"
		args = append(args, filter.AgentType)
	}
	if filter.Category != "" {
		query += " AND category = ?"
		args = append(args, filter.Category)
	}
	if filter.RepoID != "" {
		query += " AND repo_id = ?"
		args = append(args, filter.RepoID)
	}
	if !filter.Since.IsZero() {
		query += " AND created_at >= ?"
		args = append(args, filter.Since)
	}

	query += " ORDER BY created_at DESC"

	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
	}

	rows, err := m.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query agent learnings: %w", err)
	}
	defer rows.Close()

	return scanAgentLearnings(rows)
}

// GetRecentLearnings retrieves the most recent agent learnings
func (m *SQLiteMemoryDB) GetRecentLearnings(limit int) ([]*AgentLearning, error) {
	rows, err := m.db.Query(`
		SELECT id, agent_id, agent_type, category, title, content, repo_id, created_at
		FROM agent_learnings
		ORDER BY created_at DESC
		LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query recent learnings: %w", err)
	}
	defer rows.Close()

	return scanAgentLearnings(rows)
}

// StoreContextSummary stores a context summary
func (m *SQLiteMemoryDB) StoreContextSummary(summary *ContextSummary) error {
	result, err := m.db.Exec(`
		INSERT INTO context_summaries (session_id, agent_id, summary, full_context, repo_id)
		VALUES (?, ?, ?, ?, ?)`,
		summary.SessionID,
		summary.AgentID,
		summary.Summary,
		nullString(summary.FullContext),
		nullString(summary.RepoID),
	)
	if err != nil {
		return fmt.Errorf("failed to store context summary: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get summary ID: %w", err)
	}
	summary.ID = id

	return nil
}

// GetRecentSummaries retrieves the most recent context summaries
func (m *SQLiteMemoryDB) GetRecentSummaries(limit int) ([]*ContextSummary, error) {
	rows, err := m.db.Query(`
		SELECT id, session_id, agent_id, summary, full_context, repo_id, created_at
		FROM context_summaries
		ORDER BY created_at DESC
		LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query recent summaries: %w", err)
	}
	defer rows.Close()

	return scanContextSummaries(rows)
}

// GetSummariesByAgent retrieves summaries for a specific agent
func (m *SQLiteMemoryDB) GetSummariesByAgent(agentID string, limit int) ([]*ContextSummary, error) {
	rows, err := m.db.Query(`
		SELECT id, session_id, agent_id, summary, full_context, repo_id, created_at
		FROM context_summaries
		WHERE agent_id = ?
		ORDER BY created_at DESC
		LIMIT ?`,
		agentID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query agent summaries: %w", err)
	}
	defer rows.Close()

	return scanContextSummaries(rows)
}

// GetSummariesBySession retrieves all summaries for a session
func (m *SQLiteMemoryDB) GetSummariesBySession(sessionID string) ([]*ContextSummary, error) {
	rows, err := m.db.Query(`
		SELECT id, session_id, agent_id, summary, full_context, repo_id, created_at
		FROM context_summaries
		WHERE session_id = ?
		ORDER BY created_at ASC`,
		sessionID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query session summaries: %w", err)
	}
	defer rows.Close()

	return scanContextSummaries(rows)
}

// Helper scanning functions

func scanAgentLearnings(rows *sql.Rows) ([]*AgentLearning, error) {
	var learnings []*AgentLearning
	for rows.Next() {
		var learning AgentLearning
		var repoID sql.NullString

		err := rows.Scan(
			&learning.ID,
			&learning.AgentID,
			&learning.AgentType,
			&learning.Category,
			&learning.Title,
			&learning.Content,
			&repoID,
			&learning.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan agent learning: %w", err)
		}

		if repoID.Valid {
			learning.RepoID = repoID.String
		}

		learnings = append(learnings, &learning)
	}

	return learnings, rows.Err()
}

func scanContextSummaries(rows *sql.Rows) ([]*ContextSummary, error) {
	var summaries []*ContextSummary
	for rows.Next() {
		var summary ContextSummary
		var fullContext, repoID sql.NullString

		err := rows.Scan(
			&summary.ID,
			&summary.SessionID,
			&summary.AgentID,
			&summary.Summary,
			&fullContext,
			&repoID,
			&summary.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan context summary: %w", err)
		}

		if fullContext.Valid {
			summary.FullContext = fullContext.String
		}
		if repoID.Valid {
			summary.RepoID = repoID.String
		}

		summaries = append(summaries, &summary)
	}

	return summaries, rows.Err()
}
