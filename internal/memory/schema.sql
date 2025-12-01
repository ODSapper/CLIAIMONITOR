-- CLIAIMONITOR Memory Database Schema
-- Version: 1.0.0
-- Purpose: Persistent cross-session memory for adaptive supervisor

-- Schema version tracking
CREATE TABLE IF NOT EXISTS schema_version (
    version INTEGER PRIMARY KEY,
    applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    description TEXT
);

-- Repository context (auto-discovered)
CREATE TABLE IF NOT EXISTS repos (
    id TEXT PRIMARY KEY,              -- Hash of git remote or base path
    base_path TEXT NOT NULL,
    git_remote TEXT,
    claude_md_hash TEXT,              -- Detect CLAUDE.md changes
    discovered_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_scanned TIMESTAMP,
    needs_rescan INTEGER DEFAULT 1    -- Boolean flag
);

CREATE INDEX IF NOT EXISTS idx_repos_base_path ON repos(base_path);
CREATE INDEX IF NOT EXISTS idx_repos_last_scanned ON repos(last_scanned);

-- Repository files discovered
CREATE TABLE IF NOT EXISTS repo_files (
    repo_id TEXT NOT NULL,
    file_path TEXT NOT NULL,          -- Relative path
    file_type TEXT NOT NULL,          -- 'claude_md', 'workflow_yaml', 'plan_yaml'
    content_hash TEXT,
    content TEXT,                     -- Full file content for analysis
    discovered_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (repo_id, file_path),
    FOREIGN KEY (repo_id) REFERENCES repos(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_repo_files_type ON repo_files(file_type);
CREATE INDEX IF NOT EXISTS idx_repo_files_repo ON repo_files(repo_id);

-- Agent learnings (from all agents)
CREATE TABLE IF NOT EXISTS agent_learnings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    agent_id TEXT NOT NULL,
    agent_type TEXT NOT NULL,         -- 'coder', 'tester', 'reviewer', 'supervisor'
    category TEXT NOT NULL,           -- 'error_pattern', 'solution', 'best_practice', 'workflow_insight'
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    repo_id TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (repo_id) REFERENCES repos(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_agent_learnings_time ON agent_learnings(created_at);
CREATE INDEX IF NOT EXISTS idx_agent_learnings_agent ON agent_learnings(agent_id);
CREATE INDEX IF NOT EXISTS idx_agent_learnings_repo ON agent_learnings(repo_id);
CREATE INDEX IF NOT EXISTS idx_agent_learnings_type ON agent_learnings(agent_type);
CREATE INDEX IF NOT EXISTS idx_agent_learnings_category ON agent_learnings(category);

-- Context summaries (written before compaction)
CREATE TABLE IF NOT EXISTS context_summaries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT NOT NULL,
    agent_id TEXT NOT NULL,
    summary TEXT NOT NULL,
    full_context TEXT,                -- Optional: full context before compaction
    repo_id TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (repo_id) REFERENCES repos(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_summaries_time ON context_summaries(created_at);
CREATE INDEX IF NOT EXISTS idx_summaries_session ON context_summaries(session_id);
CREATE INDEX IF NOT EXISTS idx_summaries_agent ON context_summaries(agent_id);

-- Workflow tasks (parsed from plans)
CREATE TABLE IF NOT EXISTS workflow_tasks (
    id TEXT PRIMARY KEY,              -- e.g., 'MAH-123', 'MSS-AI-045'
    repo_id TEXT NOT NULL,
    source_file TEXT NOT NULL,        -- Which workflow file it came from
    title TEXT NOT NULL,
    description TEXT,
    priority TEXT DEFAULT 'medium',   -- 'low', 'medium', 'high', 'critical'
    status TEXT DEFAULT 'pending',    -- 'pending', 'assigned', 'in_progress', 'completed', 'blocked'
    assigned_agent_id TEXT,
    parent_task_id TEXT,              -- For task dependencies
    estimated_effort TEXT,            -- 'small', 'medium', 'large'
    tags TEXT,                        -- JSON array of tags
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP,
    FOREIGN KEY (repo_id) REFERENCES repos(id) ON DELETE CASCADE,
    FOREIGN KEY (parent_task_id) REFERENCES workflow_tasks(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_tasks_status ON workflow_tasks(status);
CREATE INDEX IF NOT EXISTS idx_tasks_repo ON workflow_tasks(repo_id);
CREATE INDEX IF NOT EXISTS idx_tasks_assigned ON workflow_tasks(assigned_agent_id);
CREATE INDEX IF NOT EXISTS idx_tasks_priority ON workflow_tasks(priority);
CREATE INDEX IF NOT EXISTS idx_tasks_parent ON workflow_tasks(parent_task_id);

-- Human decisions (all approvals/guidance)
CREATE TABLE IF NOT EXISTS human_decisions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    context TEXT NOT NULL,            -- What was being decided
    question TEXT NOT NULL,           -- What supervisor asked
    answer TEXT NOT NULL,             -- Human's response
    decision_type TEXT,               -- 'approval', 'guidance', 'clarification', 'rejection'
    agent_id TEXT,                    -- Which agent triggered question
    related_task_id TEXT,             -- If related to a task
    repo_id TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (repo_id) REFERENCES repos(id) ON DELETE SET NULL,
    FOREIGN KEY (related_task_id) REFERENCES workflow_tasks(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_decisions_time ON human_decisions(created_at);
CREATE INDEX IF NOT EXISTS idx_decisions_agent ON human_decisions(agent_id);
CREATE INDEX IF NOT EXISTS idx_decisions_type ON human_decisions(decision_type);
CREATE INDEX IF NOT EXISTS idx_decisions_repo ON human_decisions(repo_id);

-- Deployment history (track agent spawning)
CREATE TABLE IF NOT EXISTS deployments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    repo_id TEXT NOT NULL,
    deployment_plan TEXT NOT NULL,    -- JSON of the deployment strategy
    proposed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    approved_at TIMESTAMP,
    executed_at TIMESTAMP,
    status TEXT DEFAULT 'proposed',   -- 'proposed', 'approved', 'executing', 'completed', 'failed'
    agent_configs TEXT,               -- JSON array of spawned agent configs
    result TEXT,                      -- Success/failure summary
    FOREIGN KEY (repo_id) REFERENCES repos(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_deployments_status ON deployments(status);
CREATE INDEX IF NOT EXISTS idx_deployments_repo ON deployments(repo_id);

-- Insert schema version
INSERT OR IGNORE INTO schema_version (version, description) VALUES
    (1, 'Initial schema with repos, agent_learnings, context_summaries, workflow_tasks, human_decisions, deployments');
