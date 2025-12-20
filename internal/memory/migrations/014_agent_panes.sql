-- Migration 014: Add WezTerm pane tracking to agent_control
-- Enables tracking and management of WezTerm panes for agents

-- Add pane_id column to agent_control table
ALTER TABLE agent_control ADD COLUMN pane_id TEXT;

-- Create index for fast pane lookups
CREATE INDEX IF NOT EXISTS idx_agent_control_pane_id ON agent_control(pane_id);

-- Create pane_history table for tracking pane lifecycle
CREATE TABLE IF NOT EXISTS pane_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    agent_id TEXT NOT NULL,
    pane_id TEXT NOT NULL,
    action TEXT NOT NULL,  -- 'spawned', 'closed', 'crashed', 'detached', 'reattached'
    status_before TEXT,    -- Agent status before action
    status_after TEXT,     -- Agent status after action
    details TEXT,          -- Additional context (error message, reason, etc.)
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,

    FOREIGN KEY (agent_id) REFERENCES agent_control(agent_id)
);

-- Indexes for pane history queries
CREATE INDEX IF NOT EXISTS idx_pane_history_agent_id ON pane_history(agent_id);
CREATE INDEX IF NOT EXISTS idx_pane_history_pane_id ON pane_history(pane_id);
CREATE INDEX IF NOT EXISTS idx_pane_history_action ON pane_history(action);
CREATE INDEX IF NOT EXISTS idx_pane_history_timestamp ON pane_history(timestamp);

-- Update schema version
INSERT OR REPLACE INTO schema_version (version, applied_at) VALUES (15, CURRENT_TIMESTAMP);
