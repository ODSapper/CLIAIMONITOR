// internal/tasks/store.go
package tasks

import (
	"database/sql"
	"encoding/json"
	"time"
)

// Store persists tasks to SQLite
type Store struct {
	db *sql.DB
}

// NewStore creates a new task store
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// Init creates the tasks table
func (s *Store) Init() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS tasks (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			description TEXT,
			priority INTEGER NOT NULL DEFAULT 5,
			status TEXT NOT NULL DEFAULT 'pending',
			source TEXT NOT NULL DEFAULT 'captain',
			repo TEXT,
			assigned_to TEXT,
			branch TEXT,
			pr_url TEXT,
			requirements TEXT,
			metadata TEXT,
			created_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL,
			started_at TIMESTAMP,
			completed_at TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}

	// Add requirements column if it doesn't exist (migration for existing DBs)
	s.db.Exec(`ALTER TABLE tasks ADD COLUMN requirements TEXT`)
	return nil
}

// Save creates or updates a task
func (s *Store) Save(task *Task) error {
	metadata, _ := json.Marshal(task.Metadata)
	requirements, _ := json.Marshal(task.Requirements)

	_, err := s.db.Exec(`
		INSERT INTO tasks (id, title, description, priority, status, source, repo, assigned_to, branch, pr_url, requirements, metadata, created_at, updated_at, started_at, completed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			title=excluded.title,
			description=excluded.description,
			priority=excluded.priority,
			status=excluded.status,
			assigned_to=excluded.assigned_to,
			branch=excluded.branch,
			pr_url=excluded.pr_url,
			requirements=excluded.requirements,
			metadata=excluded.metadata,
			updated_at=excluded.updated_at,
			started_at=excluded.started_at,
			completed_at=excluded.completed_at
	`,
		task.ID, task.Title, task.Description, task.Priority,
		task.Status, task.Source, task.Repo, task.AssignedTo,
		task.Branch, task.PRUrl, string(requirements), string(metadata),
		task.CreatedAt, task.UpdatedAt, task.StartedAt, task.CompletedAt,
	)
	return err
}

// GetByID retrieves a task by ID
func (s *Store) GetByID(id string) (*Task, error) {
	row := s.db.QueryRow(`
		SELECT id, title, description, priority, status, source, repo, assigned_to, branch, pr_url, requirements, metadata, created_at, updated_at, started_at, completed_at
		FROM tasks WHERE id = ?
	`, id)

	return s.scanTask(row)
}

// GetByStatus retrieves all tasks with a given status
func (s *Store) GetByStatus(status TaskStatus) ([]*Task, error) {
	rows, err := s.db.Query(`
		SELECT id, title, description, priority, status, source, repo, assigned_to, branch, pr_url, requirements, metadata, created_at, updated_at, started_at, completed_at
		FROM tasks WHERE status = ? ORDER BY priority, created_at
	`, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return s.scanTasks(rows)
}

// GetAll retrieves all tasks
func (s *Store) GetAll() ([]*Task, error) {
	rows, err := s.db.Query(`
		SELECT id, title, description, priority, status, source, repo, assigned_to, branch, pr_url, requirements, metadata, created_at, updated_at, started_at, completed_at
		FROM tasks ORDER BY priority, created_at
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return s.scanTasks(rows)
}

// Delete removes a task
func (s *Store) Delete(id string) error {
	_, err := s.db.Exec(`DELETE FROM tasks WHERE id = ?`, id)
	return err
}

func (s *Store) scanTask(row *sql.Row) (*Task, error) {
	var task Task
	var requirements, metadata sql.NullString
	var startedAt, completedAt sql.NullTime
	var repo, assignedTo, branch, prUrl sql.NullString

	err := row.Scan(
		&task.ID, &task.Title, &task.Description, &task.Priority,
		&task.Status, &task.Source, &repo, &assignedTo,
		&branch, &prUrl, &requirements, &metadata,
		&task.CreatedAt, &task.UpdatedAt, &startedAt, &completedAt,
	)
	if err != nil {
		return nil, err
	}

	if repo.Valid {
		task.Repo = repo.String
	}
	if assignedTo.Valid {
		task.AssignedTo = assignedTo.String
	}
	if branch.Valid {
		task.Branch = branch.String
	}
	if prUrl.Valid {
		task.PRUrl = prUrl.String
	}
	if startedAt.Valid {
		task.StartedAt = &startedAt.Time
	}
	if completedAt.Valid {
		task.CompletedAt = &completedAt.Time
	}
	if requirements.Valid && requirements.String != "" {
		if err := json.Unmarshal([]byte(requirements.String), &task.Requirements); err != nil {
			// Log but continue - don't fail on bad JSON
			task.Requirements = nil
		}
	}
	if metadata.Valid && metadata.String != "" {
		if err := json.Unmarshal([]byte(metadata.String), &task.Metadata); err != nil {
			// Log but continue - don't fail on bad JSON
			task.Metadata = make(map[string]string)
		}
	}

	return &task, nil
}

func (s *Store) scanTasks(rows *sql.Rows) ([]*Task, error) {
	var tasks []*Task
	for rows.Next() {
		var task Task
		var requirements, metadata sql.NullString
		var startedAt, completedAt sql.NullTime
		var repo, assignedTo, branch, prUrl sql.NullString

		err := rows.Scan(
			&task.ID, &task.Title, &task.Description, &task.Priority,
			&task.Status, &task.Source, &repo, &assignedTo,
			&branch, &prUrl, &requirements, &metadata,
			&task.CreatedAt, &task.UpdatedAt, &startedAt, &completedAt,
		)
		if err != nil {
			return nil, err
		}

		if repo.Valid {
			task.Repo = repo.String
		}
		if assignedTo.Valid {
			task.AssignedTo = assignedTo.String
		}
		if branch.Valid {
			task.Branch = branch.String
		}
		if prUrl.Valid {
			task.PRUrl = prUrl.String
		}
		if startedAt.Valid {
			task.StartedAt = &startedAt.Time
		}
		if completedAt.Valid {
			task.CompletedAt = &completedAt.Time
		}
		if requirements.Valid && requirements.String != "" {
			if err := json.Unmarshal([]byte(requirements.String), &task.Requirements); err != nil {
				task.Requirements = nil
			}
		}
		if metadata.Valid && metadata.String != "" {
			if err := json.Unmarshal([]byte(metadata.String), &task.Metadata); err != nil {
				task.Metadata = make(map[string]string)
			}
		}

		tasks = append(tasks, &task)
	}
	return tasks, nil
}

// RecordHistory saves a status transition
func (s *Store) RecordHistory(taskID, fromStatus, toStatus, changedBy, reason string) error {
	_, err := s.db.Exec(`
		INSERT INTO task_history (task_id, from_status, to_status, changed_by, reason, changed_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, taskID, fromStatus, toStatus, changedBy, reason, time.Now())
	return err
}
