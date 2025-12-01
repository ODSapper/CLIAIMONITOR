package memory

import (
	"database/sql"
	"fmt"
)

// CreateTask creates a single workflow task
func (m *SQLiteMemoryDB) CreateTask(task *WorkflowTask) error {
	return m.CreateTasks([]*WorkflowTask{task})
}

// CreateTasks creates multiple workflow tasks in a transaction
func (m *SQLiteMemoryDB) CreateTasks(tasks []*WorkflowTask) error {
	return m.withTx(func(tx *sql.Tx) error {
		stmt, err := tx.Prepare(`
			INSERT INTO workflow_tasks
			(id, repo_id, source_file, title, description, priority, status, assigned_agent_id, parent_task_id, estimated_effort, tags)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET
				title = excluded.title,
				description = excluded.description,
				priority = excluded.priority,
				updated_at = CURRENT_TIMESTAMP`)
		if err != nil {
			return fmt.Errorf("failed to prepare task insert: %w", err)
		}
		defer stmt.Close()

		for _, task := range tasks {
			_, err := stmt.Exec(
				task.ID,
				task.RepoID,
				task.SourceFile,
				task.Title,
				nullString(task.Description),
				task.Priority,
				task.Status,
				nullString(task.AssignedAgentID),
				nullString(task.ParentTaskID),
				nullString(task.EstimatedEffort),
				nullString(task.Tags),
			)
			if err != nil {
				return fmt.Errorf("failed to insert task %s: %w", task.ID, err)
			}
		}

		return nil
	})
}

// GetTask retrieves a single task by ID
func (m *SQLiteMemoryDB) GetTask(taskID string) (*WorkflowTask, error) {
	var task WorkflowTask
	var description, assignedAgentID, parentTaskID, estimatedEffort, tags sql.NullString
	var completedAt sql.NullTime

	err := m.db.QueryRow(`
		SELECT id, repo_id, source_file, title, description, priority, status,
		       assigned_agent_id, parent_task_id, estimated_effort, tags,
		       created_at, updated_at, completed_at
		FROM workflow_tasks
		WHERE id = ?`,
		taskID,
	).Scan(
		&task.ID,
		&task.RepoID,
		&task.SourceFile,
		&task.Title,
		&description,
		&task.Priority,
		&task.Status,
		&assignedAgentID,
		&parentTaskID,
		&estimatedEffort,
		&tags,
		&task.CreatedAt,
		&task.UpdatedAt,
		&completedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("task not found: %s", taskID)
		}
		return nil, fmt.Errorf("failed to get task: %w", err)
	}

	task.Description = description.String
	task.AssignedAgentID = assignedAgentID.String
	task.ParentTaskID = parentTaskID.String
	task.EstimatedEffort = estimatedEffort.String
	task.Tags = tags.String
	if completedAt.Valid {
		task.CompletedAt = &completedAt.Time
	}

	return &task, nil
}

// GetTasks retrieves tasks with filters
func (m *SQLiteMemoryDB) GetTasks(filter TaskFilter) ([]*WorkflowTask, error) {
	query := `
		SELECT id, repo_id, source_file, title, description, priority, status,
		       assigned_agent_id, parent_task_id, estimated_effort, tags,
		       created_at, updated_at, completed_at
		FROM workflow_tasks
		WHERE 1=1`
	var args []interface{}

	if filter.RepoID != "" {
		query += " AND repo_id = ?"
		args = append(args, filter.RepoID)
	}
	if filter.Status != "" {
		query += " AND status = ?"
		args = append(args, filter.Status)
	}
	if filter.AssignedAgentID != "" {
		query += " AND assigned_agent_id = ?"
		args = append(args, filter.AssignedAgentID)
	}
	if filter.Priority != "" {
		query += " AND priority = ?"
		args = append(args, filter.Priority)
	}
	if filter.ParentTaskID != "" {
		query += " AND parent_task_id = ?"
		args = append(args, filter.ParentTaskID)
	}

	query += " ORDER BY created_at DESC"

	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
	}

	rows, err := m.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query tasks: %w", err)
	}
	defer rows.Close()

	return scanWorkflowTasks(rows)
}

// UpdateTaskStatus updates the status and optionally assigns an agent
func (m *SQLiteMemoryDB) UpdateTaskStatus(taskID, status, agentID string) error {
	query := `
		UPDATE workflow_tasks
		SET status = ?, updated_at = CURRENT_TIMESTAMP`
	args := []interface{}{status}

	if agentID != "" {
		query += ", assigned_agent_id = ?"
		args = append(args, agentID)
	}

	if status == "completed" {
		query += ", completed_at = CURRENT_TIMESTAMP"
	}

	query += " WHERE id = ?"
	args = append(args, taskID)

	result, err := m.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("failed to update task status: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check affected rows: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("task not found: %s", taskID)
	}

	return nil
}

// UpdateTask updates a full task
func (m *SQLiteMemoryDB) UpdateTask(task *WorkflowTask) error {
	result, err := m.db.Exec(`
		UPDATE workflow_tasks
		SET title = ?, description = ?, priority = ?, status = ?,
		    assigned_agent_id = ?, parent_task_id = ?, estimated_effort = ?,
		    tags = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`,
		task.Title,
		nullString(task.Description),
		task.Priority,
		task.Status,
		nullString(task.AssignedAgentID),
		nullString(task.ParentTaskID),
		nullString(task.EstimatedEffort),
		nullString(task.Tags),
		task.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update task: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check affected rows: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("task not found: %s", task.ID)
	}

	return nil
}

// Helper scanning functions

func scanWorkflowTasks(rows *sql.Rows) ([]*WorkflowTask, error) {
	var tasks []*WorkflowTask
	for rows.Next() {
		var task WorkflowTask
		var description, assignedAgentID, parentTaskID, estimatedEffort, tags sql.NullString
		var completedAt sql.NullTime

		err := rows.Scan(
			&task.ID,
			&task.RepoID,
			&task.SourceFile,
			&task.Title,
			&description,
			&task.Priority,
			&task.Status,
			&assignedAgentID,
			&parentTaskID,
			&estimatedEffort,
			&tags,
			&task.CreatedAt,
			&task.UpdatedAt,
			&completedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan workflow task: %w", err)
		}

		task.Description = description.String
		task.AssignedAgentID = assignedAgentID.String
		task.ParentTaskID = parentTaskID.String
		task.EstimatedEffort = estimatedEffort.String
		task.Tags = tags.String
		if completedAt.Valid {
			task.CompletedAt = &completedAt.Time
		}

		tasks = append(tasks, &task)
	}

	return tasks, rows.Err()
}
