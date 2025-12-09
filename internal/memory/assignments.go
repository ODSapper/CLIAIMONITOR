package memory

import (
	"database/sql"
	"fmt"
	"time"
)

// CreateAssignment creates a new task assignment
func (m *SQLiteMemoryDB) CreateAssignment(assignment *TaskAssignment) error {
	query := `
		INSERT INTO task_assignments (
			task_id, assigned_to, assigned_by, assignment_type, status,
			branch_name, review_feedback, review_attempt, worker_count,
			started_at, completed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := m.db.Exec(
		query,
		assignment.TaskID,
		assignment.AssignedTo,
		assignment.AssignedBy,
		assignment.AssignmentType,
		assignment.Status,
		nullString(assignment.BranchName),
		nullString(assignment.ReviewFeedback),
		assignment.ReviewAttempt,
		assignment.WorkerCount,
		nullTime(assignment.StartedAt),
		nullTime(assignment.CompletedAt),
	)
	if err != nil {
		return fmt.Errorf("failed to create assignment: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get assignment ID: %w", err)
	}

	assignment.ID = id
	return nil
}

// GetAssignment retrieves a task assignment by ID
func (m *SQLiteMemoryDB) GetAssignment(id int64) (*TaskAssignment, error) {
	query := `
		SELECT id, task_id, assigned_to, assigned_by, assignment_type, status,
		       branch_name, review_feedback, review_attempt, worker_count,
		       started_at, completed_at, created_at
		FROM task_assignments
		WHERE id = ?
	`

	var a TaskAssignment
	var branchName, reviewFeedback sql.NullString
	var startedAt, completedAt sql.NullTime

	err := m.db.QueryRow(query, id).Scan(
		&a.ID, &a.TaskID, &a.AssignedTo, &a.AssignedBy, &a.AssignmentType, &a.Status,
		&branchName, &reviewFeedback, &a.ReviewAttempt, &a.WorkerCount,
		&startedAt, &completedAt, &a.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get assignment: %w", err)
	}

	if branchName.Valid {
		a.BranchName = branchName.String
	}
	if reviewFeedback.Valid {
		a.ReviewFeedback = reviewFeedback.String
	}
	if startedAt.Valid {
		t := startedAt.Time
		a.StartedAt = &t
	}
	if completedAt.Valid {
		t := completedAt.Time
		a.CompletedAt = &t
	}

	return &a, nil
}

// GetAssignmentsByTask retrieves all assignments for a task
func (m *SQLiteMemoryDB) GetAssignmentsByTask(taskID string) ([]*TaskAssignment, error) {
	query := `
		SELECT id, task_id, assigned_to, assigned_by, assignment_type, status,
		       branch_name, review_feedback, review_attempt, worker_count,
		       started_at, completed_at, created_at
		FROM task_assignments
		WHERE task_id = ?
		ORDER BY created_at DESC
	`

	rows, err := m.db.Query(query, taskID)
	if err != nil {
		return nil, fmt.Errorf("failed to query assignments by task: %w", err)
	}
	defer rows.Close()

	var assignments []*TaskAssignment
	for rows.Next() {
		var a TaskAssignment
		var branchName, reviewFeedback sql.NullString
		var startedAt, completedAt sql.NullTime

		if err := rows.Scan(
			&a.ID, &a.TaskID, &a.AssignedTo, &a.AssignedBy, &a.AssignmentType, &a.Status,
			&branchName, &reviewFeedback, &a.ReviewAttempt, &a.WorkerCount,
			&startedAt, &completedAt, &a.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan assignment: %w", err)
		}

		if branchName.Valid {
			a.BranchName = branchName.String
		}
		if reviewFeedback.Valid {
			a.ReviewFeedback = reviewFeedback.String
		}
		if startedAt.Valid {
			t := startedAt.Time
			a.StartedAt = &t
		}
		if completedAt.Valid {
			t := completedAt.Time
			a.CompletedAt = &t
		}

		assignments = append(assignments, &a)
	}

	return assignments, rows.Err()
}

// GetAssignmentsByAgent retrieves assignments for an agent, optionally filtered by status
func (m *SQLiteMemoryDB) GetAssignmentsByAgent(agentID string, status string) ([]*TaskAssignment, error) {
	var query string
	var args []interface{}

	if status != "" {
		query = `
			SELECT id, task_id, assigned_to, assigned_by, assignment_type, status,
			       branch_name, review_feedback, review_attempt, worker_count,
			       started_at, completed_at, created_at
			FROM task_assignments
			WHERE assigned_to = ? AND status = ?
			ORDER BY created_at DESC
		`
		args = []interface{}{agentID, status}
	} else {
		query = `
			SELECT id, task_id, assigned_to, assigned_by, assignment_type, status,
			       branch_name, review_feedback, review_attempt, worker_count,
			       started_at, completed_at, created_at
			FROM task_assignments
			WHERE assigned_to = ?
			ORDER BY created_at DESC
		`
		args = []interface{}{agentID}
	}

	rows, err := m.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query assignments by agent: %w", err)
	}
	defer rows.Close()

	var assignments []*TaskAssignment
	for rows.Next() {
		var a TaskAssignment
		var branchName, reviewFeedback sql.NullString
		var startedAt, completedAt sql.NullTime

		if err := rows.Scan(
			&a.ID, &a.TaskID, &a.AssignedTo, &a.AssignedBy, &a.AssignmentType, &a.Status,
			&branchName, &reviewFeedback, &a.ReviewAttempt, &a.WorkerCount,
			&startedAt, &completedAt, &a.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan assignment: %w", err)
		}

		if branchName.Valid {
			a.BranchName = branchName.String
		}
		if reviewFeedback.Valid {
			a.ReviewFeedback = reviewFeedback.String
		}
		if startedAt.Valid {
			t := startedAt.Time
			a.StartedAt = &t
		}
		if completedAt.Valid {
			t := completedAt.Time
			a.CompletedAt = &t
		}

		assignments = append(assignments, &a)
	}

	return assignments, rows.Err()
}

// GetActiveAssignment retrieves the currently active (in_progress) assignment for an agent
func (m *SQLiteMemoryDB) GetActiveAssignment(agentID string) (*TaskAssignment, error) {
	query := `
		SELECT id, task_id, assigned_to, assigned_by, assignment_type, status,
		       branch_name, review_feedback, review_attempt, worker_count,
		       started_at, completed_at, created_at
		FROM task_assignments
		WHERE assigned_to = ? AND status = 'in_progress'
		ORDER BY created_at DESC
		LIMIT 1
	`

	var a TaskAssignment
	var branchName, reviewFeedback sql.NullString
	var startedAt, completedAt sql.NullTime

	err := m.db.QueryRow(query, agentID).Scan(
		&a.ID, &a.TaskID, &a.AssignedTo, &a.AssignedBy, &a.AssignmentType, &a.Status,
		&branchName, &reviewFeedback, &a.ReviewAttempt, &a.WorkerCount,
		&startedAt, &completedAt, &a.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get active assignment: %w", err)
	}

	if branchName.Valid {
		a.BranchName = branchName.String
	}
	if reviewFeedback.Valid {
		a.ReviewFeedback = reviewFeedback.String
	}
	if startedAt.Valid {
		t := startedAt.Time
		a.StartedAt = &t
	}
	if completedAt.Valid {
		t := completedAt.Time
		a.CompletedAt = &t
	}

	return &a, nil
}

// UpdateAssignmentStatus updates the status of an assignment
func (m *SQLiteMemoryDB) UpdateAssignmentStatus(id int64, status string) error {
	var query string
	var args []interface{}

	if status == "in_progress" {
		query = `
			UPDATE task_assignments
			SET status = ?, started_at = ?
			WHERE id = ?
		`
		now := time.Now()
		args = []interface{}{status, now, id}
	} else {
		query = `
			UPDATE task_assignments
			SET status = ?
			WHERE id = ?
		`
		args = []interface{}{status, id}
	}

	result, err := m.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("failed to update assignment status: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check affected rows: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("assignment %d not found", id)
	}

	return nil
}

// CompleteAssignment marks an assignment as complete with optional feedback
func (m *SQLiteMemoryDB) CompleteAssignment(id int64, status string, feedback string) error {
	query := `
		UPDATE task_assignments
		SET status = ?, review_feedback = ?, completed_at = ?
		WHERE id = ?
	`

	now := time.Now()
	result, err := m.db.Exec(query, status, nullString(feedback), now, id)
	if err != nil {
		return fmt.Errorf("failed to complete assignment: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check affected rows: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("assignment %d not found", id)
	}

	return nil
}

// RequestRework increments review_attempt and sets status to "rework" for the coder to fix
func (m *SQLiteMemoryDB) RequestRework(id int64, feedback string) error {
	query := `
		UPDATE task_assignments
		SET status = 'rework',
		    review_feedback = ?,
		    review_attempt = review_attempt + 1,
		    completed_at = NULL
		WHERE id = ?
	`

	result, err := m.db.Exec(query, nullString(feedback), id)
	if err != nil {
		return fmt.Errorf("failed to request rework: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check affected rows: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("assignment %d not found", id)
	}

	return nil
}

// AddWorker adds a worker to an assignment
func (m *SQLiteMemoryDB) AddWorker(worker *AssignmentWorker) error {
	query := `
		INSERT INTO assignment_workers (
			assignment_id, worker_type, worker_id, task_description, status,
			result, tokens_used, started_at, completed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := m.db.Exec(
		query,
		worker.AssignmentID,
		worker.WorkerType,
		nullString(worker.WorkerID),
		worker.TaskDescription,
		worker.Status,
		nullString(worker.Result),
		worker.TokensUsed,
		nullTime(worker.StartedAt),
		nullTime(worker.CompletedAt),
	)
	if err != nil {
		return fmt.Errorf("failed to add worker: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get worker ID: %w", err)
	}

	worker.ID = id

	// Increment worker count on assignment
	_, err = m.db.Exec(`
		UPDATE task_assignments
		SET worker_count = worker_count + 1
		WHERE id = ?
	`, worker.AssignmentID)
	if err != nil {
		return fmt.Errorf("failed to increment worker count: %w", err)
	}

	return nil
}

// UpdateWorkerStatus updates a worker's status and result
func (m *SQLiteMemoryDB) UpdateWorkerStatus(id int64, status, result string, tokensUsed int64) error {
	var query string
	var args []interface{}

	if status == "in_progress" {
		query = `
			UPDATE assignment_workers
			SET status = ?, started_at = ?
			WHERE id = ?
		`
		now := time.Now()
		args = []interface{}{status, now, id}
	} else if status == "completed" || status == "failed" {
		query = `
			UPDATE assignment_workers
			SET status = ?, result = ?, tokens_used = ?, completed_at = ?
			WHERE id = ?
		`
		now := time.Now()
		args = []interface{}{status, nullString(result), tokensUsed, now, id}
	} else {
		query = `
			UPDATE assignment_workers
			SET status = ?
			WHERE id = ?
		`
		args = []interface{}{status, id}
	}

	result2, err := m.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("failed to update worker status: %w", err)
	}

	rows, err := result2.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check affected rows: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("worker %d not found", id)
	}

	return nil
}

// GetWorkersByAssignment retrieves all workers for an assignment
func (m *SQLiteMemoryDB) GetWorkersByAssignment(assignmentID int64) ([]*AssignmentWorker, error) {
	query := `
		SELECT id, assignment_id, worker_type, worker_id, task_description, status,
		       result, tokens_used, started_at, completed_at, created_at
		FROM assignment_workers
		WHERE assignment_id = ?
		ORDER BY created_at ASC
	`

	rows, err := m.db.Query(query, assignmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to query workers: %w", err)
	}
	defer rows.Close()

	var workers []*AssignmentWorker
	for rows.Next() {
		var w AssignmentWorker
		var workerID, result sql.NullString
		var startedAt, completedAt sql.NullTime

		if err := rows.Scan(
			&w.ID, &w.AssignmentID, &w.WorkerType, &workerID, &w.TaskDescription, &w.Status,
			&result, &w.TokensUsed, &startedAt, &completedAt, &w.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan worker: %w", err)
		}

		if workerID.Valid {
			w.WorkerID = workerID.String
		}
		if result.Valid {
			w.Result = result.String
		}
		if startedAt.Valid {
			t := startedAt.Time
			w.StartedAt = &t
		}
		if completedAt.Valid {
			t := completedAt.Time
			w.CompletedAt = &t
		}

		workers = append(workers, &w)
	}

	return workers, rows.Err()
}

// nullTime converts a time pointer to sql.NullTime
func nullTime(t *time.Time) sql.NullTime {
	if t == nil {
		return sql.NullTime{Valid: false}
	}
	return sql.NullTime{
		Time:  *t,
		Valid: true,
	}
}
