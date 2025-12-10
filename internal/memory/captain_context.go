package memory

import (
	"database/sql"
	"fmt"
	"time"
)

// SetContext stores or updates a context entry
func (m *SQLiteMemoryDB) SetContext(key, value string, priority int, maxAgeHours int) error {
	query := `
		INSERT INTO captain_context (context_key, context_value, priority, max_age_hours, updated_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(context_key) DO UPDATE SET
			context_value = excluded.context_value,
			priority = excluded.priority,
			max_age_hours = excluded.max_age_hours,
			updated_at = CURRENT_TIMESTAMP
	`
	_, err := m.db.Exec(query, key, value, priority, maxAgeHours)
	if err != nil {
		return fmt.Errorf("failed to set context %s: %w", key, err)
	}
	return nil
}

// GetContext retrieves a single context entry by key
func (m *SQLiteMemoryDB) GetContext(key string) (*CaptainContext, error) {
	query := `
		SELECT id, context_key, context_value, priority, max_age_hours, created_at, updated_at
		FROM captain_context
		WHERE context_key = ?
	`
	ctx := &CaptainContext{}
	err := m.db.QueryRow(query, key).Scan(
		&ctx.ID, &ctx.Key, &ctx.Value, &ctx.Priority, &ctx.MaxAgeHours,
		&ctx.CreatedAt, &ctx.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get context %s: %w", key, err)
	}
	return ctx, nil
}

// GetAllContext retrieves all context entries ordered by priority
func (m *SQLiteMemoryDB) GetAllContext() ([]*CaptainContext, error) {
	query := `
		SELECT id, context_key, context_value, priority, max_age_hours, created_at, updated_at
		FROM captain_context
		ORDER BY priority DESC, updated_at DESC
	`
	return m.queryContextEntries(query)
}

// GetContextByPriority retrieves context entries with priority >= minPriority
func (m *SQLiteMemoryDB) GetContextByPriority(minPriority int) ([]*CaptainContext, error) {
	query := `
		SELECT id, context_key, context_value, priority, max_age_hours, created_at, updated_at
		FROM captain_context
		WHERE priority >= ?
		ORDER BY priority DESC, updated_at DESC
	`
	return m.queryContextEntriesWithArg(query, minPriority)
}

// DeleteContext removes a context entry
func (m *SQLiteMemoryDB) DeleteContext(key string) error {
	_, err := m.db.Exec("DELETE FROM captain_context WHERE context_key = ?", key)
	if err != nil {
		return fmt.Errorf("failed to delete context %s: %w", key, err)
	}
	return nil
}

// CleanExpiredContext removes context entries that have exceeded their max age
// Returns the number of entries removed
func (m *SQLiteMemoryDB) CleanExpiredContext() (int, error) {
	query := `
		DELETE FROM captain_context
		WHERE max_age_hours > 0
		AND datetime(updated_at, '+' || max_age_hours || ' hours') < datetime('now')
	`
	result, err := m.db.Exec(query)
	if err != nil {
		return 0, fmt.Errorf("failed to clean expired context: %w", err)
	}
	count, _ := result.RowsAffected()
	return int(count), nil
}

// LogSessionEvent records a significant event in the session log
func (m *SQLiteMemoryDB) LogSessionEvent(sessionID, eventType, summary, details, agentID string) error {
	query := `
		INSERT INTO captain_session_log (session_id, event_type, summary, details, agent_id)
		VALUES (?, ?, ?, ?, ?)
	`
	_, err := m.db.Exec(query, sessionID, eventType, summary, details, agentID)
	if err != nil {
		return fmt.Errorf("failed to log session event: %w", err)
	}
	return nil
}

// GetSessionLog retrieves log entries for a specific session
func (m *SQLiteMemoryDB) GetSessionLog(sessionID string, limit int) ([]*SessionLogEntry, error) {
	query := `
		SELECT id, session_id, event_type, summary, details, agent_id, created_at
		FROM captain_session_log
		WHERE session_id = ?
		ORDER BY created_at DESC
		LIMIT ?
	`
	return m.queryLogEntries(query, sessionID, limit)
}

// GetRecentSessionLog retrieves the most recent log entries across all sessions
func (m *SQLiteMemoryDB) GetRecentSessionLog(limit int) ([]*SessionLogEntry, error) {
	query := `
		SELECT id, session_id, event_type, summary, details, agent_id, created_at
		FROM captain_session_log
		ORDER BY created_at DESC
		LIMIT ?
	`
	return m.queryLogEntriesOneArg(query, limit)
}

// Helper functions

func (m *SQLiteMemoryDB) queryContextEntries(query string) ([]*CaptainContext, error) {
	rows, err := m.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query context: %w", err)
	}
	defer rows.Close()

	return m.scanContextRows(rows)
}

func (m *SQLiteMemoryDB) queryContextEntriesWithArg(query string, arg interface{}) ([]*CaptainContext, error) {
	rows, err := m.db.Query(query, arg)
	if err != nil {
		return nil, fmt.Errorf("failed to query context: %w", err)
	}
	defer rows.Close()

	return m.scanContextRows(rows)
}

func (m *SQLiteMemoryDB) scanContextRows(rows *sql.Rows) ([]*CaptainContext, error) {
	var entries []*CaptainContext
	for rows.Next() {
		ctx := &CaptainContext{}
		err := rows.Scan(
			&ctx.ID, &ctx.Key, &ctx.Value, &ctx.Priority, &ctx.MaxAgeHours,
			&ctx.CreatedAt, &ctx.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan context row: %w", err)
		}
		entries = append(entries, ctx)
	}
	return entries, rows.Err()
}

func (m *SQLiteMemoryDB) queryLogEntries(query string, sessionID string, limit int) ([]*SessionLogEntry, error) {
	rows, err := m.db.Query(query, sessionID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query session log: %w", err)
	}
	defer rows.Close()

	return m.scanLogRows(rows)
}

func (m *SQLiteMemoryDB) queryLogEntriesOneArg(query string, limit int) ([]*SessionLogEntry, error) {
	rows, err := m.db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query session log: %w", err)
	}
	defer rows.Close()

	return m.scanLogRows(rows)
}

func (m *SQLiteMemoryDB) scanLogRows(rows *sql.Rows) ([]*SessionLogEntry, error) {
	var entries []*SessionLogEntry
	for rows.Next() {
		entry := &SessionLogEntry{}
		var agentID sql.NullString
		var details sql.NullString
		err := rows.Scan(
			&entry.ID, &entry.SessionID, &entry.EventType, &entry.Summary,
			&details, &agentID, &entry.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan log row: %w", err)
		}
		if agentID.Valid {
			entry.AgentID = agentID.String
		}
		if details.Valid {
			entry.Details = details.String
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

// GetContextSummary returns a formatted summary of all context for Captain startup
func (m *SQLiteMemoryDB) GetContextSummary() (string, error) {
	contexts, err := m.GetAllContext()
	if err != nil {
		return "", err
	}

	if len(contexts) == 0 {
		return "", nil
	}

	summary := "=== CAPTAIN CONTEXT (from memory.db) ===\n\n"
	for _, ctx := range contexts {
		ageStr := ""
		if ctx.MaxAgeHours > 0 {
			remaining := time.Until(ctx.UpdatedAt.Add(time.Duration(ctx.MaxAgeHours) * time.Hour))
			if remaining > 0 {
				ageStr = fmt.Sprintf(" (expires in %s)", remaining.Round(time.Minute))
			}
		}
		summary += fmt.Sprintf("[%s] (priority: %d)%s\n%s\n\n", ctx.Key, ctx.Priority, ageStr, ctx.Value)
	}

	return summary, nil
}
