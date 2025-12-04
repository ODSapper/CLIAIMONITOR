-- Migration 005: Add agent_type filtering to knowledge
-- Allows each agent type (captain, developer, recon, etc.) to have isolated knowledge

-- Add agent_type column to knowledge table
ALTER TABLE knowledge ADD COLUMN agent_type TEXT DEFAULT 'captain';

-- Add index for efficient filtering by agent_type
CREATE INDEX IF NOT EXISTS idx_knowledge_agent_type ON knowledge(agent_type);

-- Add agent_type to episodes as well for filtering
ALTER TABLE episodes ADD COLUMN agent_type TEXT DEFAULT 'captain';

CREATE INDEX IF NOT EXISTS idx_episodes_agent_type ON episodes(agent_type);

-- Update schema version
INSERT OR REPLACE INTO schema_version (version, applied_at) VALUES (6, datetime('now'));
