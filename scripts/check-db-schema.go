package main

import (
	"database/sql"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	db, err := sql.Open("sqlite3", "data/memory.db?_journal_mode=WAL")
	if err != nil {
		fmt.Printf("Error opening DB: %v\n", err)
		return
	}
	defer db.Close()

	// Check schema version
	var version int
	err = db.QueryRow("SELECT version FROM schema_version ORDER BY version DESC LIMIT 1").Scan(&version)
	if err != nil {
		fmt.Printf("Error checking version: %v\n", err)
		return
	}
	fmt.Printf("Current schema version: %d\n", version)

	// Check if agent_control table exists
	var tableName string
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='agent_control'").Scan(&tableName)
	if err == sql.ErrNoRows {
		fmt.Println("agent_control table: NOT FOUND")
	} else if err != nil {
		fmt.Printf("Error checking table: %v\n", err)
	} else {
		fmt.Printf("agent_control table: EXISTS\n")

		// Count agents
		var count int
		db.QueryRow("SELECT COUNT(*) FROM agent_control").Scan(&count)
		fmt.Printf("Agent count: %d\n", count)
	}
}
