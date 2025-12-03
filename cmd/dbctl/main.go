package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	// Define flags
	dbPath := flag.String("db", "data/memory.db", "Path to SQLite database")
	action := flag.String("action", "", "Action to perform: heartbeat, check-shutdown, get-agent")
	agentID := flag.String("agent", "", "Agent ID")
	jsonOutput := flag.Bool("json", false, "Output as JSON")

	flag.Parse()

	if *action == "" || *agentID == "" {
		fmt.Fprintf(os.Stderr, "Usage: dbctl -db <path> -action <action> -agent <id> [-json]\n")
		fmt.Fprintf(os.Stderr, "Actions: heartbeat, check-shutdown, get-agent\n")
		os.Exit(1)
	}

	// Open database
	db, err := sql.Open("sqlite3", fmt.Sprintf("%s?_journal_mode=WAL&_busy_timeout=5000", *dbPath))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// Perform action
	switch *action {
	case "heartbeat":
		if err := updateHeartbeat(db, *agentID); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to update heartbeat: %v\n", err)
			os.Exit(1)
		}
		if !*jsonOutput {
			fmt.Printf("Heartbeat updated for %s\n", *agentID)
		} else {
			json.NewEncoder(os.Stdout).Encode(map[string]interface{}{
				"success": true,
				"agent_id": *agentID,
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			})
		}

	case "check-shutdown":
		shutdown, reason, err := checkShutdown(db, *agentID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to check shutdown: %v\n", err)
			os.Exit(1)
		}

		if *jsonOutput {
			json.NewEncoder(os.Stdout).Encode(map[string]interface{}{
				"shutdown": shutdown,
				"reason": reason,
			})
		} else {
			if shutdown {
				fmt.Printf("1\n%s\n", reason)
			} else {
				fmt.Printf("0\n")
			}
		}

	case "get-agent":
		agent, err := getAgent(db, *agentID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to get agent: %v\n", err)
			os.Exit(1)
		}
		json.NewEncoder(os.Stdout).Encode(agent)

	default:
		fmt.Fprintf(os.Stderr, "Unknown action: %s\n", *action)
		os.Exit(1)
	}
}

func updateHeartbeat(db *sql.DB, agentID string) error {
	query := `UPDATE agent_control SET heartbeat_at = datetime('now'), status = 'active' WHERE agent_id = ?`
	result, err := db.Exec(query, agentID)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return fmt.Errorf("agent not found: %s", agentID)
	}

	return nil
}

func checkShutdown(db *sql.DB, agentID string) (bool, string, error) {
	var shutdownFlag int
	var shutdownReason sql.NullString

	query := `SELECT shutdown_flag, shutdown_reason FROM agent_control WHERE agent_id = ?`
	err := db.QueryRow(query, agentID).Scan(&shutdownFlag, &shutdownReason)
	if err != nil {
		return false, "", err
	}

	reason := ""
	if shutdownReason.Valid {
		reason = shutdownReason.String
	}

	return shutdownFlag == 1, reason, nil
}

type AgentInfo struct {
	AgentID      string    `json:"agent_id"`
	ConfigName   string    `json:"config_name"`
	Role         string    `json:"role"`
	Status       string    `json:"status"`
	HeartbeatAt  *string   `json:"heartbeat_at"`
	CurrentTask  *string   `json:"current_task"`
	ShutdownFlag int       `json:"shutdown_flag"`
	SpawnedAt    time.Time `json:"spawned_at"`
}

func getAgent(db *sql.DB, agentID string) (*AgentInfo, error) {
	var agent AgentInfo
	var heartbeatAt, currentTask sql.NullString

	query := `SELECT agent_id, config_name, role, status, heartbeat_at, current_task, shutdown_flag, spawned_at
	          FROM agent_control WHERE agent_id = ?`
	err := db.QueryRow(query, agentID).Scan(
		&agent.AgentID,
		&agent.ConfigName,
		&agent.Role,
		&agent.Status,
		&heartbeatAt,
		&currentTask,
		&agent.ShutdownFlag,
		&agent.SpawnedAt,
	)
	if err != nil {
		return nil, err
	}

	if heartbeatAt.Valid {
		agent.HeartbeatAt = &heartbeatAt.String
	}
	if currentTask.Valid {
		agent.CurrentTask = &currentTask.String
	}

	return &agent, nil
}
