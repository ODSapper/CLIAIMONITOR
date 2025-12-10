//go:build ignore

package main

import (
	"database/sql"
	"fmt"
	_ "modernc.org/sqlite"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: set-shutdown-flag <agent-id> [reason]\n")
		os.Exit(1)
	}

	agentID := os.Args[1]
	reason := "Manual shutdown"
	if len(os.Args) > 2 {
		reason = os.Args[2]
	}

	db, err := sql.Open("sqlite", "data/memory.db?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	_, err = db.Exec(`
		UPDATE agent_control
		SET shutdown_flag = 1, shutdown_reason = ?
		WHERE agent_id = ?
	`, reason, agentID)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error setting shutdown flag: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Set shutdown flag for %s: %s\n", agentID, reason)
}
