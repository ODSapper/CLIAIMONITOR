-- Migration 004: Add learning database tables for RAG memory
-- Episodes: What happened (timestamped events for context)
-- Knowledge: What was learned (searchable solutions/patterns)

-- Episodes table
CREATE TABLE IF NOT EXISTS episodes (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    agent_id TEXT NOT NULL,
    event_type TEXT NOT NULL,  -- 'action', 'error', 'decision', 'outcome'
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    project TEXT,
    importance REAL DEFAULT 0.5,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_episodes_session ON episodes(session_id);
CREATE INDEX IF NOT EXISTS idx_episodes_agent ON episodes(agent_id);
CREATE INDEX IF NOT EXISTS idx_episodes_project ON episodes(project);
CREATE INDEX IF NOT EXISTS idx_episodes_created ON episodes(created_at);

-- Knowledge table
CREATE TABLE IF NOT EXISTS knowledge (
    id TEXT PRIMARY KEY,
    category TEXT NOT NULL,    -- 'error_solution', 'pattern', 'best_practice', 'gotcha'
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    tags TEXT,                 -- JSON array of tags
    source TEXT,               -- Where this came from (session_id, manual, etc)
    use_count INTEGER DEFAULT 0,
    last_used DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_knowledge_category ON knowledge(category);
CREATE INDEX IF NOT EXISTS idx_knowledge_use_count ON knowledge(use_count DESC);

-- TF-IDF index table for search
CREATE TABLE IF NOT EXISTS knowledge_terms (
    knowledge_id TEXT NOT NULL,
    term TEXT NOT NULL,
    tf REAL NOT NULL,          -- Term frequency
    PRIMARY KEY (knowledge_id, term),
    FOREIGN KEY (knowledge_id) REFERENCES knowledge(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_terms_term ON knowledge_terms(term);

-- Document frequency for IDF calculation
CREATE TABLE IF NOT EXISTS term_stats (
    term TEXT PRIMARY KEY,
    doc_count INTEGER DEFAULT 1
);

-- Update schema version
INSERT OR REPLACE INTO schema_version (version, applied_at) VALUES (5, datetime('now'));
