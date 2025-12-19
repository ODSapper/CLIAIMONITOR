-- Migration 012: Add prompt templates table
-- Stores agent system prompts in the database instead of files

CREATE TABLE IF NOT EXISTS prompt_templates (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,           -- e.g., 'sgt-green', 'engineer', 'security'
    role TEXT NOT NULL,                  -- e.g., 'supervisor', 'engineer', 'security'
    content TEXT NOT NULL,               -- The full prompt template with {{PLACEHOLDERS}}
    description TEXT,                    -- Human-readable description
    version INTEGER NOT NULL DEFAULT 1,  -- For tracking changes
    is_active BOOLEAN NOT NULL DEFAULT 1,-- Whether this template is currently in use
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Index for fast lookups by name and role
CREATE INDEX IF NOT EXISTS idx_prompt_templates_name ON prompt_templates(name);
CREATE INDEX IF NOT EXISTS idx_prompt_templates_role ON prompt_templates(role);
CREATE INDEX IF NOT EXISTS idx_prompt_templates_active ON prompt_templates(is_active);

-- Trigger to update updated_at timestamp
CREATE TRIGGER IF NOT EXISTS prompt_templates_update_timestamp
    AFTER UPDATE ON prompt_templates
    FOR EACH ROW
BEGIN
    UPDATE prompt_templates SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

-- Update schema version
INSERT OR REPLACE INTO schema_version (version, applied_at) VALUES (13, CURRENT_TIMESTAMP);
