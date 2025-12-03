package main

import (
	"database/sql"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: delete-agent <agent-id>\n")
		os.Exit(1)
	}

	agentID := os.Args[1]

	db, err := sql.Open("sqlite3", "data/memory.db?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	result, err := db.Exec("DELETE FROM agent_control WHERE agent_id = ?", agentID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error deleting agent: %v\n", err)
		os.Exit(1)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		fmt.Printf("Agent not found: %s\n", agentID)
	} else {
		fmt.Printf("Deleted agent: %s\n", agentID)
	}
}
