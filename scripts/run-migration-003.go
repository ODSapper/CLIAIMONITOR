package main

import (
	"database/sql"
	"fmt"
	"os"
	_ "github.com/mattn/go-sqlite3"
)

var migration003 = `-- Migration 003: Add agent_control table
-- This table is the source of truth for agent lifecycle

CREATE TABLE IF NOT EXISTS agent_control (
    agent_id TEXT PRIMARY KEY,
    config_name TEXT NOT NULL,
    role TEXT,
    project_path TEXT,
    pid INTEGER,

    -- Heartbeat & Status
    status TEXT DEFAULT 'starting',
    heartbeat_at DATETIME,
    current_task TEXT,

    -- Control Flags
    shutdown_flag INTEGER DEFAULT 0,
    shutdown_reason TEXT,
    priority_override INTEGER,

    -- Lifecycle
    spawned_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    stopped_at DATETIME,
    stop_reason TEXT,

    -- Metadata
    model TEXT,
    color TEXT
);

CREATE INDEX IF NOT EXISTS idx_agent_heartbeat ON agent_control(heartbeat_at);
CREATE INDEX IF NOT EXISTS idx_agent_status ON agent_control(status);

-- Update schema version
INSERT OR REPLACE INTO schema_version (version, applied_at) VALUES (4, datetime('now'));
`

func main() {
	dbPath := "data/memory.db"
	if len(os.Args) > 1 {
		dbPath = os.Args[1]
	}

	db, err := sql.Open("sqlite3", fmt.Sprintf("%s?_journal_mode=WAL&_busy_timeout=5000", dbPath))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open DB: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// Check current version
	var version int
	err = db.QueryRow("SELECT version FROM schema_version ORDER BY version DESC LIMIT 1").Scan(&version)
	if err != nil && err != sql.ErrNoRows {
		fmt.Fprintf(os.Stderr, "Failed to check version: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Current schema version: %d\n", version)

	if version >= 4 {
		fmt.Println("Migration 003 already applied (schema version 4)")
		return
	}

	fmt.Println("Applying migration 003: agent_control table...")

	if _, err := db.Exec(migration003); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to run migration: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Migration 003 applied successfully!")

	// Verify
	var newVersion int
	db.QueryRow("SELECT version FROM schema_version ORDER BY version DESC LIMIT 1").Scan(&newVersion)
	fmt.Printf("New schema version: %d\n", newVersion)

	var tableName string
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='agent_control'").Scan(&tableName)
	if err == nil {
		fmt.Println("agent_control table created successfully!")
	} else {
		fmt.Println("Warning: Could not verify table creation")
	}
}
