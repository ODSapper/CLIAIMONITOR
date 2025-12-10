-- Migration 007: Captain Session Context
-- Stores Captain's working context for resumption after restart

-- Captain context entries (key-value with metadata)
CREATE TABLE IF NOT EXISTS captain_context (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    context_key TEXT NOT NULL UNIQUE,     -- e.g., 'current_focus', 'recent_work', 'pending_tasks'
    context_value TEXT NOT NULL,          -- The actual context content
    priority INTEGER DEFAULT 5,           -- 1-10, higher = more important to preserve
    max_age_hours INTEGER DEFAULT 24,     -- Auto-expire after this many hours (0 = never)
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_captain_context_key ON captain_context(context_key);
CREATE INDEX IF NOT EXISTS idx_captain_context_priority ON captain_context(priority DESC);
CREATE INDEX IF NOT EXISTS idx_captain_context_updated ON captain_context(updated_at);

-- Captain session log (append-only log of significant events)
CREATE TABLE IF NOT EXISTS captain_session_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT NOT NULL,             -- Groups entries by session
    event_type TEXT NOT NULL,             -- 'startup', 'command', 'spawn', 'decision', 'error', 'shutdown'
    summary TEXT NOT NULL,                -- Brief description
    details TEXT,                         -- Full details (JSON or text)
    agent_id TEXT,                        -- Related agent if applicable
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_captain_log_session ON captain_session_log(session_id);
CREATE INDEX IF NOT EXISTS idx_captain_log_type ON captain_session_log(event_type);
CREATE INDEX IF NOT EXISTS idx_captain_log_time ON captain_session_log(created_at DESC);

-- Update schema version
INSERT OR REPLACE INTO schema_version (version, description) VALUES
    (8, 'Add captain_context and captain_session_log tables');
