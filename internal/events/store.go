package events

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// SQLiteStore implements EventStore using SQLite
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore creates a new SQLite event store and initializes the schema
func NewSQLiteStore(db *sql.DB) (*SQLiteStore, error) {
	store := &SQLiteStore{db: db}

	if err := store.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return store, nil
}

// initSchema creates the events table and indexes
func (s *SQLiteStore) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS events (
		id TEXT PRIMARY KEY,
		type TEXT NOT NULL,
		source TEXT NOT NULL,
		target TEXT NOT NULL,
		priority INTEGER NOT NULL,
		payload TEXT NOT NULL,
		created_at TIMESTAMP NOT NULL,
		delivered_at TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_events_target ON events(target, delivered_at);
	CREATE INDEX IF NOT EXISTS idx_events_type ON events(type);
	`

	_, err := s.db.Exec(schema)
	if err != nil {
		return fmt.Errorf("failed to execute schema: %w", err)
	}

	return nil
}

// Save persists an event to the database
func (s *SQLiteStore) Save(event *Event) error {
	// Marshal payload to JSON
	payloadJSON, err := json.Marshal(event.Payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	query := `
		INSERT INTO events (id, type, source, target, priority, payload, created_at, delivered_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, NULL)
	`

	_, err = s.db.Exec(query,
		event.ID,
		event.Type,
		event.Source,
		event.Target,
		event.Priority,
		string(payloadJSON),
		event.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to insert event: %w", err)
	}

	return nil
}

// GetPending retrieves undelivered events for a specific target.
// If target is "all", returns only events explicitly targeted to "all".
// Otherwise, returns events for the specific target OR targeted to "all".
// If types is nil or empty, returns all event types.
func (s *SQLiteStore) GetPending(target string, types []EventType) ([]*Event, error) {
	var query string
	var args []interface{}

	// Build the query based on type filtering
	if len(types) == 0 {
		// No type filter
		if target == "all" {
			query = `
				SELECT id, type, source, target, priority, payload, created_at
				FROM events
				WHERE delivered_at IS NULL AND target = ?
				ORDER BY priority ASC, created_at ASC
			`
			args = []interface{}{target}
		} else {
			query = `
				SELECT id, type, source, target, priority, payload, created_at
				FROM events
				WHERE delivered_at IS NULL AND (target = ? OR target = 'all')
				ORDER BY priority ASC, created_at ASC
			`
			args = []interface{}{target}
		}
	} else {
		// With type filter
		placeholders := make([]interface{}, 0, len(types)+1)
		typeQuery := ""

		for i, eventType := range types {
			if i > 0 {
				typeQuery += ", "
			}
			typeQuery += "?"
			placeholders = append(placeholders, string(eventType))
		}

		if target == "all" {
			query = fmt.Sprintf(`
				SELECT id, type, source, target, priority, payload, created_at
				FROM events
				WHERE delivered_at IS NULL AND target = ? AND type IN (%s)
				ORDER BY priority ASC, created_at ASC
			`, typeQuery)
			args = append([]interface{}{target}, placeholders...)
		} else {
			query = fmt.Sprintf(`
				SELECT id, type, source, target, priority, payload, created_at
				FROM events
				WHERE delivered_at IS NULL AND (target = ? OR target = 'all') AND type IN (%s)
				ORDER BY priority ASC, created_at ASC
			`, typeQuery)
			args = append([]interface{}{target}, placeholders...)
		}
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query events: %w", err)
	}
	defer rows.Close()

	var events []*Event

	for rows.Next() {
		var event Event
		var payloadJSON string

		err := rows.Scan(
			&event.ID,
			&event.Type,
			&event.Source,
			&event.Target,
			&event.Priority,
			&payloadJSON,
			&event.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan event row: %w", err)
		}

		// Unmarshal payload
		if err := json.Unmarshal([]byte(payloadJSON), &event.Payload); err != nil {
			return nil, fmt.Errorf("failed to unmarshal payload: %w", err)
		}

		events = append(events, &event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return events, nil
}

// MarkDelivered marks an event as delivered by setting its delivered_at timestamp
func (s *SQLiteStore) MarkDelivered(eventID string) error {
	query := `UPDATE events SET delivered_at = ? WHERE id = ?`

	result, err := s.db.Exec(query, time.Now(), eventID)
	if err != nil {
		return fmt.Errorf("failed to mark event as delivered: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("event not found: %s", eventID)
	}

	return nil
}

// Cleanup deletes delivered events older than the specified duration
func (s *SQLiteStore) Cleanup(olderThan time.Duration) error {
	cutoffTime := time.Now().Add(-olderThan)

	query := `DELETE FROM events WHERE delivered_at IS NOT NULL AND created_at < ?`

	_, err := s.db.Exec(query, cutoffTime)
	if err != nil {
		return fmt.Errorf("failed to cleanup old events: %w", err)
	}

	return nil
}
