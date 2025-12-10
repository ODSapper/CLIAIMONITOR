-- Migration 008: Metrics history for cost tracking
-- Records agent token usage and cost estimates for analysis

-- Metrics history table (append-only log of agent costs)
CREATE TABLE IF NOT EXISTS metrics_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    agent_id TEXT NOT NULL,
    model TEXT NOT NULL,
    task_id TEXT,
    tokens_used INTEGER DEFAULT 0,
    estimated_cost REAL DEFAULT 0,
    recorded_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_metrics_agent ON metrics_history(agent_id);
CREATE INDEX IF NOT EXISTS idx_metrics_model ON metrics_history(model);
CREATE INDEX IF NOT EXISTS idx_metrics_recorded ON metrics_history(recorded_at);

-- Aggregation view for cost analysis
CREATE VIEW IF NOT EXISTS metrics_by_model AS
SELECT
    model,
    COUNT(*) as report_count,
    SUM(tokens_used) as total_tokens,
    SUM(estimated_cost) as total_cost,
    AVG(tokens_used) as avg_tokens_per_report
FROM metrics_history
GROUP BY model;

-- Update schema version
INSERT OR REPLACE INTO schema_version (version, description) VALUES
    (9, 'Add metrics_history table and metrics_by_model view');
