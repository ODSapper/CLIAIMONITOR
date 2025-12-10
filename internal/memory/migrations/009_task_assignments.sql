-- Migration 009: Task assignments for SGT workflow
-- Tracks task handoffs between Captain, SGT Green, SGT Purple

CREATE TABLE IF NOT EXISTS task_assignments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id TEXT NOT NULL,
    assigned_to TEXT NOT NULL,
    assigned_by TEXT NOT NULL,
    assignment_type TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    branch_name TEXT,
    review_feedback TEXT,
    review_attempt INTEGER DEFAULT 1,
    worker_count INTEGER DEFAULT 0,
    started_at DATETIME,
    completed_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_task_assignments_task ON task_assignments(task_id);
CREATE INDEX IF NOT EXISTS idx_task_assignments_agent ON task_assignments(assigned_to);
CREATE INDEX IF NOT EXISTS idx_task_assignments_status ON task_assignments(status);

CREATE TABLE IF NOT EXISTS assignment_workers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    assignment_id INTEGER NOT NULL REFERENCES task_assignments(id),
    worker_type TEXT NOT NULL,
    worker_id TEXT,
    task_description TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    result TEXT,
    tokens_used INTEGER DEFAULT 0,
    started_at DATETIME,
    completed_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (assignment_id) REFERENCES task_assignments(id)
);

CREATE INDEX IF NOT EXISTS idx_assignment_workers_assignment ON assignment_workers(assignment_id);

INSERT INTO schema_version (version, applied_at, description)
VALUES (10, CURRENT_TIMESTAMP, 'Task assignments for SGT workflow');
