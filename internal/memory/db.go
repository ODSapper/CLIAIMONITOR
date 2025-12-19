package memory

import (
	"crypto/sha256"
	"database/sql"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

//go:embed migrations/001_remove_chat.sql
var migration001 string

//go:embed migrations/002_add_recon_tables.sql
var migration002 string

//go:embed migrations/003_agent_control.sql
var migration003 string

//go:embed migrations/004_learning_db.sql
var migration004 string

//go:embed migrations/005_agent_type_filter.sql
var migration005 string

//go:embed migrations/006_tasks.sql
var migration006 string

//go:embed migrations/007_captain_context.sql
var migration007 string

//go:embed migrations/008_metrics_history.sql
var migration008 string

//go:embed migrations/009_task_assignments.sql
var migration009 string

//go:embed migrations/010_agent_type_metrics.sql
var migration010 string

//go:embed migrations/011_review_board.sql
var migration011 string

//go:embed migrations/012_prompt_templates.sql
var migration012 string

//go:embed migrations/013_documents.sql
var migration013 string

// SQLiteMemoryDB is the concrete implementation of MemoryDB using SQLite
type SQLiteMemoryDB struct {
	db   *sql.DB
	path string
}

// NewMemoryDB creates a new memory database instance
// If the database doesn't exist, it will be created and initialized
func NewMemoryDB(path string) (MemoryDB, error) {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create memory db directory: %w", err)
	}

	// Open database
	db, err := sql.Open("sqlite", path+"?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("failed to open memory db: %w", err)
	}

	// Set connection pool settings
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)

	memDB := &SQLiteMemoryDB{
		db:   db,
		path: path,
	}

	// Run migrations
	if err := memDB.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to migrate memory db: %w", err)
	}

	return memDB, nil
}

// migrate runs database migrations
func (m *SQLiteMemoryDB) migrate() error {
	// Execute schema
	if _, err := m.db.Exec(schemaSQL); err != nil {
		return fmt.Errorf("failed to execute schema: %w", err)
	}

	// Check current version
	var version int
	err := m.db.QueryRow("SELECT version FROM schema_version ORDER BY version DESC LIMIT 1").Scan(&version)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("failed to check schema version: %w", err)
	}

	// Run migrations based on current version
	if version < 2 {
		fmt.Println("[MIGRATION] Running migration to v2: Remove chat_messages table")
		if _, err := m.db.Exec(migration001); err != nil {
			return fmt.Errorf("failed to run migration 001: %w", err)
		}
		fmt.Println("[MIGRATION] Successfully migrated to schema v2")
	}

	if version < 3 {
		fmt.Println("[MIGRATION] Running migration to v3: Add reconnaissance tables")
		if _, err := m.db.Exec(migration002); err != nil {
			return fmt.Errorf("failed to run migration 002: %w", err)
		}
		fmt.Println("[MIGRATION] Successfully migrated to schema v3")
	}

	if version < 4 {
		fmt.Println("[MIGRATION] Running migration to v4: Add agent_control table")
		if _, err := m.db.Exec(migration003); err != nil {
			return fmt.Errorf("failed to run migration 003: %w", err)
		}
		fmt.Println("[MIGRATION] Successfully migrated to schema v4")
	}

	if version < 5 {
		fmt.Println("[MIGRATION] Running migration to v5: Add learning database tables")
		if _, err := m.db.Exec(migration004); err != nil {
			return fmt.Errorf("failed to run migration 004: %w", err)
		}
		fmt.Println("[MIGRATION] Successfully migrated to schema v5")
	}

	if version < 6 {
		fmt.Println("[MIGRATION] Running migration to v6: Add agent_type filtering")
		if _, err := m.db.Exec(migration005); err != nil {
			return fmt.Errorf("failed to run migration 005: %w", err)
		}
		fmt.Println("[MIGRATION] Successfully migrated to schema v6")
	}

	if version < 7 {
		fmt.Println("[MIGRATION] Running migration to v7: Add task tables")
		if _, err := m.db.Exec(migration006); err != nil {
			return fmt.Errorf("failed to run migration 006: %w", err)
		}
		fmt.Println("[MIGRATION] Successfully migrated to schema v7")
	}

	if version < 8 {
		fmt.Println("[MIGRATION] Running migration to v8: Add captain_context tables")
		if _, err := m.db.Exec(migration007); err != nil {
			return fmt.Errorf("failed to run migration 007: %w", err)
		}
		fmt.Println("[MIGRATION] Successfully migrated to schema v8")
	}

	if version < 9 {
		fmt.Println("[MIGRATION] Running migration to v9: Add metrics_history table")
		if _, err := m.db.Exec(migration008); err != nil {
			return fmt.Errorf("failed to run migration 008: %w", err)
		}
		fmt.Println("[MIGRATION] Successfully migrated to schema v9")
	}

	if version < 10 {
		fmt.Println("[MIGRATION] Running migration to v10: Add task_assignments tables")
		if _, err := m.db.Exec(migration009); err != nil {
			return fmt.Errorf("failed to run migration 009: %w", err)
		}
		fmt.Println("[MIGRATION] Successfully migrated to schema v10")
	}

	if version < 11 {
		fmt.Println("[MIGRATION] Running migration to v11: Add agent_type metrics segmentation")
		// Add columns if they don't exist (ignore errors for duplicate columns)
		m.db.Exec("ALTER TABLE metrics_history ADD COLUMN agent_type TEXT DEFAULT 'spawned_window'")
		m.db.Exec("ALTER TABLE metrics_history ADD COLUMN parent_agent TEXT")
		m.db.Exec("ALTER TABLE metrics_history ADD COLUMN assignment_id INTEGER")
		// Run the rest of the migration (views and indexes)
		if _, err := m.db.Exec(migration010); err != nil {
			return fmt.Errorf("failed to run migration 010: %w", err)
		}
		fmt.Println("[MIGRATION] Successfully migrated to schema v11")
	}

	if version < 12 {
		fmt.Println("[MIGRATION] Running migration to v12: Add Fagan Review Board tables")
		if _, err := m.db.Exec(migration011); err != nil {
			return fmt.Errorf("failed to run migration 011: %w", err)
		}
		fmt.Println("[MIGRATION] Successfully migrated to schema v12")
	}

	if version < 13 {
		fmt.Println("[MIGRATION] Running migration to v13: Add prompt templates table")
		if _, err := m.db.Exec(migration012); err != nil {
			return fmt.Errorf("failed to run migration 012: %w", err)
		}
		fmt.Println("[MIGRATION] Successfully migrated to schema v13")
	}

	if version < 14 {
		fmt.Println("[MIGRATION] Running migration to v14: Add documents and config tables")
		if _, err := m.db.Exec(migration013); err != nil {
			return fmt.Errorf("failed to run migration 013: %w", err)
		}
		fmt.Println("[MIGRATION] Successfully migrated to schema v14")
	}

	return nil
}

// DB returns the underlying sql.DB connection for use with other stores
func (m *SQLiteMemoryDB) DB() *sql.DB {
	return m.db
}

// Close closes the database connection
func (m *SQLiteMemoryDB) Close() error {
	if m.db != nil {
		return m.db.Close()
	}
	return nil
}

// Health returns the health status of the memory database
func (m *SQLiteMemoryDB) Health() (*HealthStatus, error) {
	status := &HealthStatus{
		Connected: false,
		DBPath:    m.path,
	}

	// Check connection with ping
	if err := m.db.Ping(); err != nil {
		return status, fmt.Errorf("database ping failed: %w", err)
	}
	status.Connected = true

	// Get schema version
	err := m.db.QueryRow("SELECT version FROM schema_version ORDER BY version DESC LIMIT 1").Scan(&status.SchemaVersion)
	if err != nil {
		return status, fmt.Errorf("failed to get schema version: %w", err)
	}

	// Get agent count
	err = m.db.QueryRow("SELECT COUNT(*) FROM agent_control").Scan(&status.AgentCount)
	if err != nil {
		// Table may not exist yet, not fatal
		status.AgentCount = 0
	}

	// Get task count
	err = m.db.QueryRow("SELECT COUNT(*) FROM workflow_tasks").Scan(&status.TaskCount)
	if err != nil {
		status.TaskCount = 0
	}

	// Get learning count
	err = m.db.QueryRow("SELECT COUNT(*) FROM agent_learnings").Scan(&status.LearningCount)
	if err != nil {
		status.LearningCount = 0
	}

	// Get context count and last update time
	err = m.db.QueryRow("SELECT COUNT(*) FROM captain_context").Scan(&status.ContextCount)
	if err != nil {
		status.ContextCount = 0
	}

	// Get last context save time
	var lastSave sql.NullString
	err = m.db.QueryRow("SELECT MAX(updated_at) FROM captain_context").Scan(&lastSave)
	if err == nil && lastSave.Valid {
		status.LastContextSave = lastSave.String
	}

	// Get database file size
	if fileInfo, err := os.Stat(m.path); err == nil {
		status.DBSizeBytes = fileInfo.Size()
	}

	return status, nil
}

// Transaction helpers

// withTx executes a function within a transaction
func (m *SQLiteMemoryDB) withTx(fn func(*sql.Tx) error) error {
	tx, err := m.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// Utility functions

// generateRepoID generates a unique ID for a repository based on git remote or base path
func generateRepoID(gitRemote, basePath string) string {
	if gitRemote != "" {
		return hashString(gitRemote)
	}
	return hashString(basePath)
}

// hashString creates a SHA256 hash of a string
func hashString(s string) string {
	h := sha256.New()
	h.Write([]byte(s))
	return fmt.Sprintf("%x", h.Sum(nil))[:16] // Use first 16 chars for readability
}

// nullString converts an empty string to sql.NullString
func nullString(s string) sql.NullString {
	return sql.NullString{
		String: s,
		Valid:  s != "",
	}
}

// nullInt64 converts a pointer to sql.NullInt64
func nullInt64(i *int64) sql.NullInt64 {
	if i == nil {
		return sql.NullInt64{Valid: false}
	}
	return sql.NullInt64{
		Int64: *i,
		Valid: true,
	}
}
