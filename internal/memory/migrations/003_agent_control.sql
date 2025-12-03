-- Migration 003: Add agent_control table
-- This table is the source of truth for agent lifecycle

CREATE TABLE IF NOT EXISTS agent_control (
    agent_id TEXT PRIMARY KEY,
    config_name TEXT NOT NULL,
    role TEXT,
    project_path TEXT,
    pid INTEGER,

    -- Heartbeat & Status
    status TEXT DEFAULT 'starting',
    heartbeat_at DATETIME,
    current_task TEXT,

    -- Control Flags
    shutdown_flag INTEGER DEFAULT 0,
    shutdown_reason TEXT,
    priority_override INTEGER,

    -- Lifecycle
    spawned_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    stopped_at DATETIME,
    stop_reason TEXT,

    -- Metadata
    model TEXT,
    color TEXT
);

CREATE INDEX IF NOT EXISTS idx_agent_heartbeat ON agent_control(heartbeat_at);
CREATE INDEX IF NOT EXISTS idx_agent_status ON agent_control(status);

-- Update schema version
INSERT OR REPLACE INTO schema_version (version, applied_at) VALUES (4, datetime('now'));
