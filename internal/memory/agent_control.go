package memory

import (
	"database/sql"
	"fmt"
	"time"
)

// AgentControlRepository provides methods for managing agent lifecycle
type AgentControlRepository interface {
	// Write operations
	RegisterAgent(agent *AgentControl) error
	UpdateStatus(agentID, status, currentTask string) error
	SetShutdownFlag(agentID string, reason string) error
	ClearShutdownFlag(agentID string) error
	MarkStopped(agentID, reason string) error
	RemoveAgent(agentID string) error

	// Read operations
	GetAgent(agentID string) (*AgentControl, error)
	GetAllAgents() ([]*AgentControl, error)
	GetStaleAgents(threshold time.Duration) ([]*AgentControl, error)
	GetAgentsByStatus(status string) ([]*AgentControl, error)
	CheckShutdownFlag(agentID string) (bool, string, error)

	// Pane tracking operations
	UpdateAgentPaneID(agentID, paneID string) error
	GetAgentByPaneID(paneID string) (*AgentControl, error)
	LogPaneEvent(agentID, paneID, action, statusBefore, statusAfter, details string) error
	GetPaneHistory(agentID string, limit int) ([]*PaneHistoryEntry, error)
}

// AgentControl represents an agent's lifecycle state
type AgentControl struct {
	AgentID         string
	ConfigName      string
	Role            string
	ProjectPath     string
	PID             *int

	// Heartbeat & Status
	Status      string
	HeartbeatAt *time.Time
	CurrentTask string

	// Control Flags
	ShutdownFlag     bool
	ShutdownReason   string
	PriorityOverride *int

	// Lifecycle
	SpawnedAt time.Time
	StoppedAt *time.Time
	StopReason string

	// Metadata
	Model  string
	Color  string
	PaneID string // WezTerm pane ID
}

// PaneHistoryEntry represents a pane lifecycle event
type PaneHistoryEntry struct {
	ID           int64
	AgentID      string
	PaneID       string
	Action       string
	StatusBefore string
	StatusAfter  string
	Details      string
	Timestamp    time.Time
}

// RegisterAgent registers a new agent in the control table
func (m *SQLiteMemoryDB) RegisterAgent(agent *AgentControl) error {
	query := `
		INSERT INTO agent_control (
			agent_id, config_name, role, project_path, pid,
			status, heartbeat_at, current_task,
			shutdown_flag, shutdown_reason, priority_override,
			model, color, pane_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(agent_id) DO UPDATE SET
			config_name = excluded.config_name,
			role = excluded.role,
			project_path = excluded.project_path,
			pid = excluded.pid,
			status = excluded.status,
			heartbeat_at = excluded.heartbeat_at,
			current_task = excluded.current_task,
			model = excluded.model,
			color = excluded.color,
			pane_id = excluded.pane_id
	`

	_, err := m.db.Exec(query,
		agent.AgentID, agent.ConfigName, nullString(agent.Role),
		nullString(agent.ProjectPath), nullInt64Ptr(agent.PID),
		agent.Status, nullTimePtr(agent.HeartbeatAt), nullString(agent.CurrentTask),
		boolToInt(agent.ShutdownFlag), nullString(agent.ShutdownReason),
		nullInt64Ptr(agent.PriorityOverride),
		nullString(agent.Model), nullString(agent.Color), nullString(agent.PaneID),
	)

	if err != nil {
		return fmt.Errorf("failed to register agent: %w", err)
	}

	return nil
}

// UpdateStatus updates the status and current task for an agent
func (m *SQLiteMemoryDB) UpdateStatus(agentID, status, currentTask string) error {
	query := `
		UPDATE agent_control
		SET status = ?, current_task = ?, heartbeat_at = CURRENT_TIMESTAMP
		WHERE agent_id = ?
	`

	result, err := m.db.Exec(query, status, nullString(currentTask), agentID)
	if err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check affected rows: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("agent not found: %s", agentID)
	}

	return nil
}

// SetShutdownFlag sets the shutdown flag for an agent
func (m *SQLiteMemoryDB) SetShutdownFlag(agentID string, reason string) error {
	query := `
		UPDATE agent_control
		SET shutdown_flag = 1, shutdown_reason = ?
		WHERE agent_id = ?
	`

	result, err := m.db.Exec(query, reason, agentID)
	if err != nil {
		return fmt.Errorf("failed to set shutdown flag: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check affected rows: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("agent not found: %s", agentID)
	}

	return nil
}

// ClearShutdownFlag clears the shutdown flag for an agent
func (m *SQLiteMemoryDB) ClearShutdownFlag(agentID string) error {
	query := `
		UPDATE agent_control
		SET shutdown_flag = 0, shutdown_reason = NULL
		WHERE agent_id = ?
	`

	result, err := m.db.Exec(query, agentID)
	if err != nil {
		return fmt.Errorf("failed to clear shutdown flag: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check affected rows: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("agent not found: %s", agentID)
	}

	return nil
}

// MarkStopped marks an agent as stopped
func (m *SQLiteMemoryDB) MarkStopped(agentID, reason string) error {
	query := `
		UPDATE agent_control
		SET status = 'stopped',
		    stopped_at = CURRENT_TIMESTAMP,
		    stop_reason = ?
		WHERE agent_id = ?
	`

	result, err := m.db.Exec(query, reason, agentID)
	if err != nil {
		return fmt.Errorf("failed to mark agent stopped: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check affected rows: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("agent not found: %s", agentID)
	}

	return nil
}

// RemoveAgent removes an agent from the control table
func (m *SQLiteMemoryDB) RemoveAgent(agentID string) error {
	query := `DELETE FROM agent_control WHERE agent_id = ?`

	result, err := m.db.Exec(query, agentID)
	if err != nil {
		return fmt.Errorf("failed to remove agent: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check affected rows: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("agent not found: %s", agentID)
	}

	return nil
}

// GetAgent retrieves a single agent by ID
func (m *SQLiteMemoryDB) GetAgent(agentID string) (*AgentControl, error) {
	query := `
		SELECT agent_id, config_name, role, project_path, pid,
		       status, heartbeat_at, current_task,
		       shutdown_flag, shutdown_reason, priority_override,
		       spawned_at, stopped_at, stop_reason,
		       model, color, pane_id
		FROM agent_control
		WHERE agent_id = ?
	`

	var agent AgentControl
	var role, projectPath, currentTask, shutdownReason, stopReason, model, color, paneID sql.NullString
	var pid, priorityOverride sql.NullInt64
	var heartbeatAt, stoppedAt sql.NullTime
	var shutdownFlag int

	err := m.db.QueryRow(query, agentID).Scan(
		&agent.AgentID, &agent.ConfigName, &role, &projectPath, &pid,
		&agent.Status, &heartbeatAt, &currentTask,
		&shutdownFlag, &shutdownReason, &priorityOverride,
		&agent.SpawnedAt, &stoppedAt, &stopReason,
		&model, &color, &paneID,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("agent not found: %s", agentID)
		}
		return nil, fmt.Errorf("failed to get agent: %w", err)
	}

	// Convert nullable fields
	agent.Role = role.String
	agent.ProjectPath = projectPath.String
	agent.CurrentTask = currentTask.String
	agent.ShutdownReason = shutdownReason.String
	agent.StopReason = stopReason.String
	agent.Model = model.String
	agent.Color = color.String
	agent.PaneID = paneID.String
	agent.ShutdownFlag = shutdownFlag != 0

	if pid.Valid {
		pidInt := int(pid.Int64)
		agent.PID = &pidInt
	}

	if priorityOverride.Valid {
		priorityInt := int(priorityOverride.Int64)
		agent.PriorityOverride = &priorityInt
	}

	if heartbeatAt.Valid {
		agent.HeartbeatAt = &heartbeatAt.Time
	}

	if stoppedAt.Valid {
		agent.StoppedAt = &stoppedAt.Time
	}

	return &agent, nil
}

// GetAllAgents retrieves all agents
func (m *SQLiteMemoryDB) GetAllAgents() ([]*AgentControl, error) {
	query := `
		SELECT agent_id, config_name, role, project_path, pid,
		       status, heartbeat_at, current_task,
		       shutdown_flag, shutdown_reason, priority_override,
		       spawned_at, stopped_at, stop_reason,
		       model, color, pane_id
		FROM agent_control
		ORDER BY spawned_at DESC
	`

	rows, err := m.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query agents: %w", err)
	}
	defer rows.Close()

	var agents []*AgentControl
	for rows.Next() {
		agent, err := scanAgentControl(rows)
		if err != nil {
			return nil, err
		}
		agents = append(agents, agent)
	}

	return agents, rows.Err()
}

// GetStaleAgents retrieves agents that haven't sent a heartbeat within the threshold,
// OR agents stuck in "starting" status that never sent a heartbeat
func (m *SQLiteMemoryDB) GetStaleAgents(threshold time.Duration) ([]*AgentControl, error) {
	query := `
		SELECT agent_id, config_name, role, project_path, pid,
		       status, heartbeat_at, current_task,
		       shutdown_flag, shutdown_reason, priority_override,
		       spawned_at, stopped_at, stop_reason,
		       model, color, pane_id
		FROM agent_control
		WHERE status NOT IN ('stopped', 'dead')
		  AND (
		    -- Case 1: Has heartbeat but it's stale
		    (heartbeat_at IS NOT NULL AND heartbeat_at < datetime('now', ?))
		    OR
		    -- Case 2: Never sent heartbeat and stuck in starting status too long
		    (heartbeat_at IS NULL AND status = 'starting' AND spawned_at < datetime('now', ?))
		  )
		ORDER BY COALESCE(heartbeat_at, spawned_at) ASC
	`

	// Convert threshold to SQLite format (e.g., "-120 seconds")
	thresholdStr := fmt.Sprintf("-%d seconds", int(threshold.Seconds()))

	rows, err := m.db.Query(query, thresholdStr, thresholdStr)
	if err != nil {
		return nil, fmt.Errorf("failed to query stale agents: %w", err)
	}
	defer rows.Close()

	var agents []*AgentControl
	for rows.Next() {
		agent, err := scanAgentControl(rows)
		if err != nil {
			return nil, err
		}
		agents = append(agents, agent)
	}

	return agents, rows.Err()
}

// GetAgentsByStatus retrieves all agents with a specific status
func (m *SQLiteMemoryDB) GetAgentsByStatus(status string) ([]*AgentControl, error) {
	query := `
		SELECT agent_id, config_name, role, project_path, pid,
		       status, heartbeat_at, current_task,
		       shutdown_flag, shutdown_reason, priority_override,
		       spawned_at, stopped_at, stop_reason,
		       model, color, pane_id
		FROM agent_control
		WHERE status = ?
		ORDER BY spawned_at DESC
	`

	rows, err := m.db.Query(query, status)
	if err != nil {
		return nil, fmt.Errorf("failed to query agents by status: %w", err)
	}
	defer rows.Close()

	var agents []*AgentControl
	for rows.Next() {
		agent, err := scanAgentControl(rows)
		if err != nil {
			return nil, err
		}
		agents = append(agents, agent)
	}

	return agents, rows.Err()
}

// CheckShutdownFlag checks if the shutdown flag is set for an agent
func (m *SQLiteMemoryDB) CheckShutdownFlag(agentID string) (bool, string, error) {
	query := `SELECT shutdown_flag, shutdown_reason FROM agent_control WHERE agent_id = ?`

	var shutdownFlag int
	var shutdownReason sql.NullString

	err := m.db.QueryRow(query, agentID).Scan(&shutdownFlag, &shutdownReason)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, "", fmt.Errorf("agent not found: %s", agentID)
		}
		return false, "", fmt.Errorf("failed to check shutdown flag: %w", err)
	}

	return shutdownFlag != 0, shutdownReason.String, nil
}

// Helper functions

// scanAgentControl scans a row into an AgentControl struct
func scanAgentControl(scanner interface {
	Scan(...interface{}) error
}) (*AgentControl, error) {
	var agent AgentControl
	var role, projectPath, currentTask, shutdownReason, stopReason, model, color, paneID sql.NullString
	var pid, priorityOverride sql.NullInt64
	var heartbeatAt, stoppedAt sql.NullTime
	var shutdownFlag int

	err := scanner.Scan(
		&agent.AgentID, &agent.ConfigName, &role, &projectPath, &pid,
		&agent.Status, &heartbeatAt, &currentTask,
		&shutdownFlag, &shutdownReason, &priorityOverride,
		&agent.SpawnedAt, &stoppedAt, &stopReason,
		&model, &color, &paneID,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to scan agent control: %w", err)
	}

	// Convert nullable fields
	agent.Role = role.String
	agent.ProjectPath = projectPath.String
	agent.CurrentTask = currentTask.String
	agent.ShutdownReason = shutdownReason.String
	agent.StopReason = stopReason.String
	agent.Model = model.String
	agent.Color = color.String
	agent.PaneID = paneID.String
	agent.ShutdownFlag = shutdownFlag != 0

	if pid.Valid {
		pidInt := int(pid.Int64)
		agent.PID = &pidInt
	}

	if priorityOverride.Valid {
		priorityInt := int(priorityOverride.Int64)
		agent.PriorityOverride = &priorityInt
	}

	if heartbeatAt.Valid {
		agent.HeartbeatAt = &heartbeatAt.Time
	}

	if stoppedAt.Valid {
		agent.StoppedAt = &stoppedAt.Time
	}

	return &agent, nil
}

// nullTimePtr converts a *time.Time to sql.NullTime
func nullTimePtr(t *time.Time) sql.NullTime {
	if t == nil {
		return sql.NullTime{Valid: false}
	}
	return sql.NullTime{Time: *t, Valid: true}
}

// nullInt64Ptr converts an *int to sql.NullInt64
func nullInt64Ptr(i *int) sql.NullInt64 {
	if i == nil {
		return sql.NullInt64{Valid: false}
	}
	return sql.NullInt64{Int64: int64(*i), Valid: true}
}

// boolToInt converts a bool to int (0 or 1)
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// UpdateAgentPaneID updates the pane_id for an agent
func (m *SQLiteMemoryDB) UpdateAgentPaneID(agentID, paneID string) error {
	query := `
		UPDATE agent_control
		SET pane_id = ?
		WHERE agent_id = ?
	`

	result, err := m.db.Exec(query, nullString(paneID), agentID)
	if err != nil {
		return fmt.Errorf("failed to update pane ID: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check affected rows: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("agent not found: %s", agentID)
	}

	return nil
}

// GetAgentByPaneID retrieves an agent by its WezTerm pane ID
func (m *SQLiteMemoryDB) GetAgentByPaneID(paneID string) (*AgentControl, error) {
	query := `
		SELECT agent_id, config_name, role, project_path, pid,
		       status, heartbeat_at, current_task,
		       shutdown_flag, shutdown_reason, priority_override,
		       spawned_at, stopped_at, stop_reason,
		       model, color, pane_id
		FROM agent_control
		WHERE pane_id = ?
	`

	var agent AgentControl
	var role, projectPath, currentTask, shutdownReason, stopReason, model, color, paneIDNullable sql.NullString
	var pid, priorityOverride sql.NullInt64
	var heartbeatAt, stoppedAt sql.NullTime
	var shutdownFlag int

	err := m.db.QueryRow(query, paneID).Scan(
		&agent.AgentID, &agent.ConfigName, &role, &projectPath, &pid,
		&agent.Status, &heartbeatAt, &currentTask,
		&shutdownFlag, &shutdownReason, &priorityOverride,
		&agent.SpawnedAt, &stoppedAt, &stopReason,
		&model, &color, &paneIDNullable,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("agent not found for pane: %s", paneID)
		}
		return nil, fmt.Errorf("failed to get agent by pane ID: %w", err)
	}

	// Convert nullable fields
	agent.Role = role.String
	agent.ProjectPath = projectPath.String
	agent.CurrentTask = currentTask.String
	agent.ShutdownReason = shutdownReason.String
	agent.StopReason = stopReason.String
	agent.Model = model.String
	agent.Color = color.String
	agent.PaneID = paneIDNullable.String
	agent.ShutdownFlag = shutdownFlag != 0

	if pid.Valid {
		pidInt := int(pid.Int64)
		agent.PID = &pidInt
	}

	if priorityOverride.Valid {
		priorityInt := int(priorityOverride.Int64)
		agent.PriorityOverride = &priorityInt
	}

	if heartbeatAt.Valid {
		agent.HeartbeatAt = &heartbeatAt.Time
	}

	if stoppedAt.Valid {
		agent.StoppedAt = &stoppedAt.Time
	}

	return &agent, nil
}

// LogPaneEvent records a pane lifecycle event in pane_history
func (m *SQLiteMemoryDB) LogPaneEvent(agentID, paneID, action, statusBefore, statusAfter, details string) error {
	query := `
		INSERT INTO pane_history (agent_id, pane_id, action, status_before, status_after, details)
		VALUES (?, ?, ?, ?, ?, ?)
	`

	_, err := m.db.Exec(query,
		agentID,
		paneID,
		action,
		nullString(statusBefore),
		nullString(statusAfter),
		nullString(details),
	)

	if err != nil {
		return fmt.Errorf("failed to log pane event: %w", err)
	}

	return nil
}

// GetPaneHistory retrieves pane lifecycle events for an agent
func (m *SQLiteMemoryDB) GetPaneHistory(agentID string, limit int) ([]*PaneHistoryEntry, error) {
	query := `
		SELECT id, agent_id, pane_id, action, status_before, status_after, details, timestamp
		FROM pane_history
		WHERE agent_id = ?
		ORDER BY timestamp DESC, id DESC
		LIMIT ?
	`

	rows, err := m.db.Query(query, agentID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query pane history: %w", err)
	}
	defer rows.Close()

	var history []*PaneHistoryEntry
	for rows.Next() {
		var entry PaneHistoryEntry
		var statusBefore, statusAfter, details sql.NullString

		err := rows.Scan(
			&entry.ID,
			&entry.AgentID,
			&entry.PaneID,
			&entry.Action,
			&statusBefore,
			&statusAfter,
			&details,
			&entry.Timestamp,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan pane history entry: %w", err)
		}

		entry.StatusBefore = statusBefore.String
		entry.StatusAfter = statusAfter.String
		entry.Details = details.String

		history = append(history, &entry)
	}

	return history, rows.Err()
}
