package main

import (
	"database/sql"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"os"
)

func main() {
	agentID := "test-verify"
	if len(os.Args) > 1 {
		agentID = os.Args[1]
	}

	db, err := sql.Open("sqlite3", "data/memory.db?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	_, err = db.Exec(`
		INSERT INTO agent_control (agent_id, config_name, role, status, spawned_at)
		VALUES (?, 'test-config', 'tester', 'starting', datetime('now'))
	`, agentID)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating agent: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Created test agent: %s\n", agentID)
}
