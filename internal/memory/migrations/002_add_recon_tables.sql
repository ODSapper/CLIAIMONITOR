-- Migration 002: Add reconnaissance tables for Snake Agent Force
-- Purpose: Track security findings, scans, and environments

-- Environments being monitored
CREATE TABLE IF NOT EXISTS environments (
    id TEXT PRIMARY KEY,              -- Unique environment identifier (e.g., 'magnolia-mah', 'customer-acme')
    name TEXT NOT NULL,
    description TEXT,
    env_type TEXT NOT NULL,           -- 'internal', 'customer', 'test'
    base_path TEXT,                   -- Local path if applicable
    git_remote TEXT,                  -- Git URL if applicable
    metadata TEXT,                    -- JSON for additional info (API endpoints, access details, etc.)
    registered_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_scanned TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_environments_type ON environments(env_type);
CREATE INDEX IF NOT EXISTS idx_environments_last_scanned ON environments(last_scanned);

-- Reconnaissance scans
CREATE TABLE IF NOT EXISTS recon_scans (
    id TEXT PRIMARY KEY,              -- Unique scan ID (e.g., 'SCAN-20251202-001')
    env_id TEXT NOT NULL,
    agent_id TEXT NOT NULL,           -- Snake agent that performed scan (e.g., 'Snake001')
    scan_type TEXT NOT NULL,          -- 'initial', 'incremental', 'targeted'
    mission TEXT,                     -- Mission description
    started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP,
    status TEXT DEFAULT 'running',    -- 'running', 'completed', 'failed'
    summary TEXT,                     -- JSON summary of scan results
    total_files_scanned INTEGER DEFAULT 0,
    languages_detected TEXT,          -- JSON array
    frameworks_detected TEXT,         -- JSON array
    test_coverage_percent INTEGER,
    security_score TEXT,              -- 'A', 'B', 'C', 'D', 'F'
    FOREIGN KEY (env_id) REFERENCES environments(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_recon_scans_env ON recon_scans(env_id);
CREATE INDEX IF NOT EXISTS idx_recon_scans_agent ON recon_scans(agent_id);
CREATE INDEX IF NOT EXISTS idx_recon_scans_time ON recon_scans(started_at);
CREATE INDEX IF NOT EXISTS idx_recon_scans_status ON recon_scans(status);

-- Reconnaissance findings
CREATE TABLE IF NOT EXISTS recon_findings (
    id TEXT PRIMARY KEY,              -- Unique finding ID (e.g., 'VULN-001', 'ARCH-042')
    scan_id TEXT NOT NULL,
    env_id TEXT NOT NULL,
    finding_type TEXT NOT NULL,       -- 'security', 'architecture', 'dependency', 'process', 'performance'
    severity TEXT NOT NULL,           -- 'critical', 'high', 'medium', 'low', 'info'
    title TEXT NOT NULL,
    description TEXT NOT NULL,
    location TEXT,                    -- File path and line (e.g., 'src/auth/login.go:45')
    recommendation TEXT,
    status TEXT DEFAULT 'open',       -- 'open', 'resolved', 'ignored', 'false_positive'
    resolved_at TIMESTAMP,
    resolved_by TEXT,                 -- Agent or human who resolved
    resolution_notes TEXT,
    metadata TEXT,                    -- JSON for additional context
    discovered_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (scan_id) REFERENCES recon_scans(id) ON DELETE CASCADE,
    FOREIGN KEY (env_id) REFERENCES environments(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_recon_findings_scan ON recon_findings(scan_id);
CREATE INDEX IF NOT EXISTS idx_recon_findings_env ON recon_findings(env_id);
CREATE INDEX IF NOT EXISTS idx_recon_findings_type ON recon_findings(finding_type);
CREATE INDEX IF NOT EXISTS idx_recon_findings_severity ON recon_findings(severity);
CREATE INDEX IF NOT EXISTS idx_recon_findings_status ON recon_findings(status);
CREATE INDEX IF NOT EXISTS idx_recon_findings_discovered ON recon_findings(discovered_at);

-- Finding history (track changes to findings)
CREATE TABLE IF NOT EXISTS recon_finding_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    finding_id TEXT NOT NULL,
    changed_by TEXT NOT NULL,         -- Agent or human identifier
    change_type TEXT NOT NULL,        -- 'status_change', 'severity_change', 'update'
    old_value TEXT,
    new_value TEXT,
    notes TEXT,
    changed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (finding_id) REFERENCES recon_findings(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_finding_history_finding ON recon_finding_history(finding_id);
CREATE INDEX IF NOT EXISTS idx_finding_history_time ON recon_finding_history(changed_at);

-- Update schema version
INSERT INTO schema_version (version, description) VALUES
    (3, 'Add reconnaissance tables for Snake Agent Force: environments, recon_scans, recon_findings, recon_finding_history');
