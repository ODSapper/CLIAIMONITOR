-- Migration 010: Add agent_type for metrics segmentation
-- Allows tracking costs by captain, sgt, spawned_window, and subagent

-- SQLite doesn't support IF NOT EXISTS for ALTER TABLE, so we use a workaround
-- by creating a new table if columns don't exist. But since this is a fresh migration,
-- we'll just skip if columns already exist by checking pragma.

-- Drop old views first (safe to do multiple times)
DROP VIEW IF EXISTS metrics_by_model;
DROP VIEW IF EXISTS metrics_by_agent_type;
DROP VIEW IF EXISTS metrics_by_agent;

-- Recreate metrics_by_model view
CREATE VIEW metrics_by_model AS
SELECT
    model,
    COUNT(*) as report_count,
    SUM(tokens_used) as total_tokens,
    SUM(estimated_cost) as total_cost,
    AVG(tokens_used) as avg_tokens_per_report
FROM metrics_history
GROUP BY model;

-- New view: Metrics by agent type for cost breakdown
CREATE VIEW metrics_by_agent_type AS
SELECT
    COALESCE(agent_type, 'spawned_window') as agent_type,
    COUNT(DISTINCT agent_id) as agent_count,
    COUNT(*) as report_count,
    SUM(tokens_used) as total_tokens,
    SUM(estimated_cost) as total_cost,
    AVG(tokens_used) as avg_tokens_per_report
FROM metrics_history
GROUP BY agent_type;

-- New view: Metrics by agent (individual agent rollup)
CREATE VIEW metrics_by_agent AS
SELECT
    agent_id,
    COALESCE(agent_type, 'spawned_window') as agent_type,
    model,
    parent_agent,
    COUNT(*) as report_count,
    SUM(tokens_used) as total_tokens,
    SUM(estimated_cost) as total_cost,
    MIN(recorded_at) as first_report,
    MAX(recorded_at) as last_report
FROM metrics_history
GROUP BY agent_id, agent_type, model, parent_agent;

-- Create indexes (IF NOT EXISTS works for indexes)
CREATE INDEX IF NOT EXISTS idx_metrics_agent_type ON metrics_history(agent_type);
CREATE INDEX IF NOT EXISTS idx_metrics_parent ON metrics_history(parent_agent);

-- Update schema version
INSERT OR REPLACE INTO schema_version (version, description) VALUES
    (11, 'Add agent_type metrics segmentation views');
