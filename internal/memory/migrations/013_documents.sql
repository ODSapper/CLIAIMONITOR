-- Migration 013: Add documents table for internal work products
-- Stores plans, reports, agent work, and other internal documents

CREATE TABLE IF NOT EXISTS documents (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    doc_type TEXT NOT NULL,              -- 'plan', 'report', 'review', 'test_report', 'agent_work', 'config'
    title TEXT NOT NULL,                 -- Human-readable title
    content TEXT NOT NULL,               -- Full document content (markdown or JSON)
    format TEXT NOT NULL DEFAULT 'markdown', -- 'markdown', 'json', 'yaml', 'text'

    -- Ownership and context
    author_id TEXT,                      -- Agent ID or 'human' or 'system'
    project_id TEXT,                     -- Related project (e.g., 'MAH', 'MSS', 'CLIAIMONITOR')
    task_id TEXT,                        -- Related task if applicable
    assignment_id INTEGER,               -- Related assignment if applicable

    -- Categorization
    tags TEXT,                           -- JSON array of tags for searching
    status TEXT DEFAULT 'active',        -- 'draft', 'active', 'archived', 'superseded'
    version INTEGER NOT NULL DEFAULT 1,  -- Version number for updates
    parent_id INTEGER,                   -- Reference to previous version if superseded

    -- Timestamps
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    archived_at TIMESTAMP,

    FOREIGN KEY (parent_id) REFERENCES documents(id),
    FOREIGN KEY (assignment_id) REFERENCES task_assignments(id)
);

-- Indexes for fast lookups
CREATE INDEX IF NOT EXISTS idx_documents_type ON documents(doc_type);
CREATE INDEX IF NOT EXISTS idx_documents_author ON documents(author_id);
CREATE INDEX IF NOT EXISTS idx_documents_project ON documents(project_id);
CREATE INDEX IF NOT EXISTS idx_documents_task ON documents(task_id);
CREATE INDEX IF NOT EXISTS idx_documents_status ON documents(status);
CREATE INDEX IF NOT EXISTS idx_documents_created ON documents(created_at);

-- Full-text search on title and content
CREATE VIRTUAL TABLE IF NOT EXISTS documents_fts USING fts5(
    title,
    content,
    content='documents',
    content_rowid='id'
);

-- Triggers to keep FTS in sync
CREATE TRIGGER IF NOT EXISTS documents_ai AFTER INSERT ON documents BEGIN
    INSERT INTO documents_fts(rowid, title, content) VALUES (new.id, new.title, new.content);
END;

CREATE TRIGGER IF NOT EXISTS documents_ad AFTER DELETE ON documents BEGIN
    INSERT INTO documents_fts(documents_fts, rowid, title, content) VALUES('delete', old.id, old.title, old.content);
END;

CREATE TRIGGER IF NOT EXISTS documents_au AFTER UPDATE ON documents BEGIN
    INSERT INTO documents_fts(documents_fts, rowid, title, content) VALUES('delete', old.id, old.title, old.content);
    INSERT INTO documents_fts(rowid, title, content) VALUES (new.id, new.title, new.content);
END;

-- Trigger to update updated_at timestamp
CREATE TRIGGER IF NOT EXISTS documents_update_timestamp
    AFTER UPDATE ON documents
    FOR EACH ROW
    WHEN NEW.updated_at = OLD.updated_at
BEGIN
    UPDATE documents SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

-- Config storage table (for teams.yaml, projects.yaml, etc.)
CREATE TABLE IF NOT EXISTS config_store (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    config_type TEXT NOT NULL UNIQUE,    -- 'teams', 'projects', 'notifications'
    content TEXT NOT NULL,               -- YAML or JSON content
    format TEXT NOT NULL DEFAULT 'yaml', -- 'yaml', 'json'
    version INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_config_store_type ON config_store(config_type);

-- Update schema version
INSERT OR REPLACE INTO schema_version (version, applied_at) VALUES (14, CURRENT_TIMESTAMP);
