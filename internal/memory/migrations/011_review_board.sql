-- Migration 011: Fagan-style Review Board
-- Multi-reviewer code inspection with defect tracking and quality scoring

-- Review Boards: Track multi-reviewer review sessions
CREATE TABLE IF NOT EXISTS review_boards (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    assignment_id INTEGER NOT NULL,
    reviewer_count INTEGER NOT NULL DEFAULT 1,
    status TEXT NOT NULL DEFAULT 'pending',  -- pending, in_progress, completed, escalated
    complexity_score INTEGER DEFAULT 0,       -- 0-100, used to determine reviewer count
    risk_level TEXT DEFAULT 'medium',         -- low, medium, high, critical
    final_verdict TEXT,                       -- approved, rejected, escalated
    aggregated_feedback TEXT,                 -- Combined feedback from all reviewers
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    started_at DATETIME,
    completed_at DATETIME,
    FOREIGN KEY (assignment_id) REFERENCES task_assignments(id)
);

CREATE INDEX IF NOT EXISTS idx_review_boards_assignment ON review_boards(assignment_id);
CREATE INDEX IF NOT EXISTS idx_review_boards_status ON review_boards(status);

-- Review Defects: Individual defect findings from reviewers
CREATE TABLE IF NOT EXISTS review_defects (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    board_id INTEGER NOT NULL,
    reviewer_id TEXT NOT NULL,

    -- Defect classification (Fagan + Modern categories)
    category TEXT NOT NULL,       -- LOGIC, DATA, INTERFACE, DOCS, SYNTAX, STANDARDS, SECURITY, PERFORMANCE, TESTING, ARCHITECTURE, STYLE
    severity TEXT NOT NULL,       -- critical, high, medium, low, info

    -- Location
    file_path TEXT,
    line_start INTEGER,
    line_end INTEGER,

    -- Details
    title TEXT NOT NULL,
    description TEXT NOT NULL,
    suggested_fix TEXT,

    -- Status tracking
    status TEXT NOT NULL DEFAULT 'open',  -- open, acknowledged, disputed, fixed, wontfix
    resolution_notes TEXT,
    resolved_by TEXT,
    resolved_at DATETIME,

    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (board_id) REFERENCES review_boards(id)
);

CREATE INDEX IF NOT EXISTS idx_defects_board ON review_defects(board_id);
CREATE INDEX IF NOT EXISTS idx_defects_reviewer ON review_defects(reviewer_id);
CREATE INDEX IF NOT EXISTS idx_defects_severity ON review_defects(severity);
CREATE INDEX IF NOT EXISTS idx_defects_category ON review_defects(category);

-- Reviewer Votes: Each reviewer's final verdict
CREATE TABLE IF NOT EXISTS reviewer_votes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    board_id INTEGER NOT NULL,
    reviewer_id TEXT NOT NULL,

    -- Vote
    approved BOOLEAN NOT NULL DEFAULT FALSE,
    confidence_score INTEGER DEFAULT 80,     -- 0-100 confidence in verdict

    -- Metrics
    defects_found INTEGER DEFAULT 0,
    review_time_seconds INTEGER,
    tokens_used INTEGER DEFAULT 0,

    -- Timestamps
    started_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    completed_at DATETIME,

    UNIQUE(board_id, reviewer_id),
    FOREIGN KEY (board_id) REFERENCES review_boards(id)
);

CREATE INDEX IF NOT EXISTS idx_votes_board ON reviewer_votes(board_id);
CREATE INDEX IF NOT EXISTS idx_votes_reviewer ON reviewer_votes(reviewer_id);

-- Agent Quality Scores: Aggregate performance metrics for leaderboard
CREATE TABLE IF NOT EXISTS agent_quality_scores (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    agent_id TEXT NOT NULL UNIQUE,
    role TEXT NOT NULL,                       -- author, reviewer

    -- Author metrics
    total_submissions INTEGER DEFAULT 0,
    approved_first_try INTEGER DEFAULT 0,
    total_approvals INTEGER DEFAULT 0,
    total_review_cycles INTEGER DEFAULT 0,
    total_defects_received INTEGER DEFAULT 0,
    critical_defects_received INTEGER DEFAULT 0,

    -- Reviewer metrics
    total_reviews INTEGER DEFAULT 0,
    defects_found INTEGER DEFAULT 0,
    true_positives INTEGER DEFAULT 0,         -- Defects confirmed by peers/author
    false_positives INTEGER DEFAULT 0,        -- Defects disputed and rejected
    critical_finds INTEGER DEFAULT 0,         -- Critical issues caught

    -- Cost efficiency
    total_tokens_used INTEGER DEFAULT 0,
    total_cost REAL DEFAULT 0,
    value_delivered REAL DEFAULT 0,           -- Calculated quality contribution

    -- Computed scores (updated on each review completion)
    approval_rate REAL DEFAULT 0,             -- Author: approvals/submissions
    first_pass_rate REAL DEFAULT 0,           -- Author: first try approvals/total
    avg_review_cycles REAL DEFAULT 0,         -- Author: cycles per task
    defect_density REAL DEFAULT 0,            -- Author: defects per submission

    detection_accuracy REAL DEFAULT 0,        -- Reviewer: true_pos/(true_pos+false_pos)
    defect_find_rate REAL DEFAULT 0,          -- Reviewer: defects per review
    cost_efficiency REAL DEFAULT 0,           -- value_delivered / cost

    quality_score REAL DEFAULT 50,            -- Overall 0-100 score

    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_quality_agent ON agent_quality_scores(agent_id);
CREATE INDEX IF NOT EXISTS idx_quality_role ON agent_quality_scores(role);
CREATE INDEX IF NOT EXISTS idx_quality_score ON agent_quality_scores(quality_score DESC);

-- Defect Categories reference (for validation/display)
CREATE TABLE IF NOT EXISTS defect_categories (
    code TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    category_type TEXT NOT NULL,              -- fagan, modern
    description TEXT,
    default_severity TEXT DEFAULT 'medium'
);

-- Insert Fagan classic categories
INSERT OR IGNORE INTO defect_categories (code, name, category_type, description, default_severity) VALUES
    ('LOGIC', 'Logic Error', 'fagan', 'Incorrect algorithm or business logic', 'high'),
    ('DATA', 'Data Handling', 'fagan', 'Incorrect data manipulation, type errors', 'high'),
    ('INTERFACE', 'Interface Error', 'fagan', 'API contract violations, parameter errors', 'high'),
    ('DOCS', 'Documentation', 'fagan', 'Missing or incorrect documentation', 'low'),
    ('SYNTAX', 'Syntax Error', 'fagan', 'Language syntax issues (should be caught by compiler)', 'medium'),
    ('STANDARDS', 'Coding Standards', 'fagan', 'Style guide and convention violations', 'low');

-- Insert Modern categories
INSERT OR IGNORE INTO defect_categories (code, name, category_type, description, default_severity) VALUES
    ('SECURITY', 'Security Vulnerability', 'modern', 'Security flaws, injection risks, auth issues', 'critical'),
    ('PERFORMANCE', 'Performance Issue', 'modern', 'Inefficient algorithms, resource leaks, N+1 queries', 'medium'),
    ('TESTING', 'Test Coverage', 'modern', 'Missing tests, weak assertions, untested paths', 'medium'),
    ('ARCHITECTURE', 'Architecture Violation', 'modern', 'Layering violations, coupling issues, SOLID violations', 'high'),
    ('STYLE', 'Style/Formatting', 'modern', 'Code formatting, naming conventions', 'info');

-- View: Active review boards with metrics
CREATE VIEW IF NOT EXISTS active_review_boards AS
SELECT
    rb.id,
    rb.assignment_id,
    ta.task_id,
    ta.assigned_to as author_id,
    rb.reviewer_count,
    rb.status,
    rb.complexity_score,
    rb.risk_level,
    COUNT(DISTINCT rv.reviewer_id) as votes_received,
    COUNT(DISTINCT rd.id) as total_defects,
    SUM(CASE WHEN rd.severity = 'critical' THEN 1 ELSE 0 END) as critical_defects,
    SUM(CASE WHEN rd.severity = 'high' THEN 1 ELSE 0 END) as high_defects,
    rb.created_at,
    rb.started_at
FROM review_boards rb
JOIN task_assignments ta ON rb.assignment_id = ta.id
LEFT JOIN reviewer_votes rv ON rb.id = rv.board_id AND rv.completed_at IS NOT NULL
LEFT JOIN review_defects rd ON rb.id = rd.board_id
WHERE rb.status IN ('pending', 'in_progress')
GROUP BY rb.id;

-- View: Agent leaderboard
CREATE VIEW IF NOT EXISTS agent_leaderboard AS
SELECT
    agent_id,
    role,
    quality_score,
    CASE
        WHEN role = 'author' THEN approval_rate
        ELSE detection_accuracy
    END as primary_metric,
    cost_efficiency,
    total_tokens_used,
    total_cost,
    CASE
        WHEN role = 'author' THEN total_submissions
        ELSE total_reviews
    END as task_count,
    updated_at
FROM agent_quality_scores
ORDER BY quality_score DESC;

-- Update schema version
INSERT OR REPLACE INTO schema_version (version, description) VALUES
    (12, 'Fagan-style Review Board with multi-reviewer support and quality scoring');
