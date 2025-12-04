package memory

import (
	"crypto/sha256"
	"database/sql"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
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
	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on")
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

	return nil
}

// Close closes the database connection
func (m *SQLiteMemoryDB) Close() error {
	if m.db != nil {
		return m.db.Close()
	}
	return nil
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
