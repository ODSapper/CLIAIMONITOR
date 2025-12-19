package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// ReconRepository provides methods for managing reconnaissance data
type ReconRepository interface {
	// Environment operations
	RegisterEnvironment(ctx context.Context, env *Environment) error
	GetEnvironment(ctx context.Context, id string) (*Environment, error)
	ListEnvironments(ctx context.Context) ([]*Environment, error)
	UpdateEnvironmentLastScan(ctx context.Context, envID string) error

	// Scan operations
	RecordScan(ctx context.Context, scan *ReconScan) error
	UpdateScanStatus(ctx context.Context, scanID, status string) error
	CompleteScan(ctx context.Context, scanID string, summary *ScanSummary) error
	GetLatestScan(ctx context.Context, envID string) (*ReconScan, error)
	GetScan(ctx context.Context, scanID string) (*ReconScan, error)
	GetScans(ctx context.Context, filter ScanFilter) ([]*ReconScan, error)

	// Finding operations
	SaveFinding(ctx context.Context, finding *ReconFinding) error
	SaveFindings(ctx context.Context, findings []*ReconFinding) error
	GetFinding(ctx context.Context, id string) (*ReconFinding, error)
	GetFindingsByEnvironment(ctx context.Context, envID string) ([]*ReconFinding, error)
	GetFindingsBySeverity(ctx context.Context, severity string) ([]*ReconFinding, error)
	GetFindingsByScan(ctx context.Context, scanID string) ([]*ReconFinding, error)
	GetFindings(ctx context.Context, filter FindingFilter) ([]*ReconFinding, error)
	UpdateFindingStatus(ctx context.Context, id, status, resolvedBy, notes string) error

	// Finding history
	RecordFindingChange(ctx context.Context, change *FindingHistoryEntry) error
	GetFindingHistory(ctx context.Context, findingID string) ([]*FindingHistoryEntry, error)
}

// Environment represents a monitored environment
type Environment struct {
	ID           string
	Name         string
	Description  string
	EnvType      string // 'internal', 'customer', 'test'
	BasePath     string
	GitRemote    string
	Metadata     map[string]interface{} // Additional info as JSON
	RegisteredAt time.Time
	LastScanned  *time.Time
}

// ReconScan represents a reconnaissance scan operation
type ReconScan struct {
	ID               string
	EnvID            string
	AgentID          string
	ScanType         string // 'initial', 'incremental', 'targeted'
	Mission          string
	StartedAt        time.Time
	CompletedAt      *time.Time
	Status           string // 'running', 'completed', 'failed'
	Summary          *ScanSummary
	TotalFilesScanned int
	LanguagesDetected []string
	FrameworksDetected []string
	TestCoveragePercent *int
	SecurityScore      string // 'A', 'B', 'C', 'D', 'F'
}

// ScanSummary contains summary information from a scan
type ScanSummary struct {
	TotalFiles     int      `json:"total_files_scanned"`
	Languages      []string `json:"languages"`
	Frameworks     []string `json:"frameworks"`
	TestCoverage   string   `json:"test_coverage"`
	SecurityScore  string   `json:"security_score"`
	CriticalCount  int      `json:"critical_count"`
	HighCount      int      `json:"high_count"`
	MediumCount    int      `json:"medium_count"`
	LowCount       int      `json:"low_count"`
}

// ReconFinding represents a single finding from reconnaissance
type ReconFinding struct {
	ID              string
	ScanID          string
	EnvID           string
	FindingType     string // 'security', 'architecture', 'dependency', 'process', 'performance'
	Severity        string // 'critical', 'high', 'medium', 'low', 'info'
	Title           string
	Description     string
	Location        string // File path and line
	Recommendation  string
	Status          string // 'open', 'resolved', 'ignored', 'false_positive'
	ResolvedAt      *time.Time
	ResolvedBy      string
	ResolutionNotes string
	Metadata        map[string]interface{}
	DiscoveredAt    time.Time
	UpdatedAt       time.Time
}

// FindingHistoryEntry tracks changes to a finding
type FindingHistoryEntry struct {
	ID         int64
	FindingID  string
	ChangedBy  string
	ChangeType string // 'status_change', 'severity_change', 'update'
	OldValue   string
	NewValue   string
	Notes      string
	ChangedAt  time.Time
}

// ScanFilter filters reconnaissance scans
type ScanFilter struct {
	EnvID    string
	AgentID  string
	ScanType string
	Status   string
	Limit    int
	Offset   int
}

// FindingFilter filters reconnaissance findings
type FindingFilter struct {
	EnvID       string
	ScanID      string
	FindingType string
	Severity    string
	Status      string
	Limit       int
	Offset      int
}

// Environment operations

func (m *SQLiteMemoryDB) RegisterEnvironment(ctx context.Context, env *Environment) error {
	metadataJSON, err := json.Marshal(env.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	_, err = m.db.ExecContext(ctx, `
		INSERT INTO environments (id, name, description, env_type, base_path, git_remote, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			description = excluded.description,
			env_type = excluded.env_type,
			base_path = excluded.base_path,
			git_remote = excluded.git_remote,
			metadata = excluded.metadata`,
		env.ID, env.Name, env.Description, env.EnvType,
		nullString(env.BasePath), nullString(env.GitRemote), string(metadataJSON),
	)
	return err
}

func (m *SQLiteMemoryDB) GetEnvironment(ctx context.Context, id string) (*Environment, error) {
	var env Environment
	var basePath, gitRemote, metadataJSON sql.NullString
	var lastScanned sql.NullTime

	err := m.db.QueryRowContext(ctx, `
		SELECT id, name, description, env_type, base_path, git_remote, metadata, registered_at, last_scanned
		FROM environments
		WHERE id = ?`,
		id,
	).Scan(
		&env.ID, &env.Name, &env.Description, &env.EnvType,
		&basePath, &gitRemote, &metadataJSON, &env.RegisteredAt, &lastScanned,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("environment not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get environment: %w", err)
	}

	env.BasePath = basePath.String
	env.GitRemote = gitRemote.String
	if lastScanned.Valid {
		env.LastScanned = &lastScanned.Time
	}

	if metadataJSON.Valid && metadataJSON.String != "" {
		if err := json.Unmarshal([]byte(metadataJSON.String), &env.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	return &env, nil
}

func (m *SQLiteMemoryDB) ListEnvironments(ctx context.Context) ([]*Environment, error) {
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, name, description, env_type, base_path, git_remote, metadata, registered_at, last_scanned
		FROM environments
		ORDER BY registered_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("failed to list environments: %w", err)
	}
	defer rows.Close()

	var environments []*Environment
	for rows.Next() {
		var env Environment
		var basePath, gitRemote, metadataJSON sql.NullString
		var lastScanned sql.NullTime

		err := rows.Scan(
			&env.ID, &env.Name, &env.Description, &env.EnvType,
			&basePath, &gitRemote, &metadataJSON, &env.RegisteredAt, &lastScanned,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan environment: %w", err)
		}

		env.BasePath = basePath.String
		env.GitRemote = gitRemote.String
		if lastScanned.Valid {
			env.LastScanned = &lastScanned.Time
		}

		if metadataJSON.Valid && metadataJSON.String != "" {
			if err := json.Unmarshal([]byte(metadataJSON.String), &env.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		}

		environments = append(environments, &env)
	}

	return environments, rows.Err()
}

func (m *SQLiteMemoryDB) UpdateEnvironmentLastScan(ctx context.Context, envID string) error {
	_, err := m.db.ExecContext(ctx, `
		UPDATE environments
		SET last_scanned = CURRENT_TIMESTAMP
		WHERE id = ?`,
		envID,
	)
	return err
}

// Scan operations

func (m *SQLiteMemoryDB) RecordScan(ctx context.Context, scan *ReconScan) error {
	var summaryJSON []byte
	var err error
	if scan.Summary != nil {
		summaryJSON, err = json.Marshal(scan.Summary)
		if err != nil {
			return fmt.Errorf("failed to marshal summary: %w", err)
		}
	}

	languagesJSON, _ := json.Marshal(scan.LanguagesDetected)
	frameworksJSON, _ := json.Marshal(scan.FrameworksDetected)

	_, err = m.db.ExecContext(ctx, `
		INSERT INTO recon_scans
		(id, env_id, agent_id, scan_type, mission, status, summary, total_files_scanned,
		 languages_detected, frameworks_detected, test_coverage_percent, security_score)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			status = excluded.status,
			summary = excluded.summary,
			total_files_scanned = excluded.total_files_scanned,
			languages_detected = excluded.languages_detected,
			frameworks_detected = excluded.frameworks_detected,
			test_coverage_percent = excluded.test_coverage_percent,
			security_score = excluded.security_score`,
		scan.ID, scan.EnvID, scan.AgentID, scan.ScanType, scan.Mission, scan.Status,
		nullString(string(summaryJSON)), scan.TotalFilesScanned,
		string(languagesJSON), string(frameworksJSON),
		nullInt64(intPtr(scan.TestCoveragePercent)), scan.SecurityScore,
	)
	return err
}

func (m *SQLiteMemoryDB) UpdateScanStatus(ctx context.Context, scanID, status string) error {
	query := `UPDATE recon_scans SET status = ?`
	args := []interface{}{status}

	if status == "completed" || status == "failed" {
		query += ", completed_at = CURRENT_TIMESTAMP"
	}

	query += " WHERE id = ?"
	args = append(args, scanID)

	_, err := m.db.ExecContext(ctx, query, args...)
	return err
}

func (m *SQLiteMemoryDB) CompleteScan(ctx context.Context, scanID string, summary *ScanSummary) error {
	summaryJSON, err := json.Marshal(summary)
	if err != nil {
		return fmt.Errorf("failed to marshal summary: %w", err)
	}

	_, err = m.db.ExecContext(ctx, `
		UPDATE recon_scans
		SET status = 'completed',
		    completed_at = CURRENT_TIMESTAMP,
		    summary = ?,
		    total_files_scanned = ?,
		    security_score = ?
		WHERE id = ?`,
		string(summaryJSON), summary.TotalFiles, summary.SecurityScore, scanID,
	)
	return err
}

func (m *SQLiteMemoryDB) GetLatestScan(ctx context.Context, envID string) (*ReconScan, error) {
	var scan ReconScan
	var mission, summaryJSON, languagesJSON, frameworksJSON sql.NullString
	var completedAt sql.NullTime
	var testCoverage sql.NullInt64

	err := m.db.QueryRowContext(ctx, `
		SELECT id, env_id, agent_id, scan_type, mission, started_at, completed_at, status,
		       summary, total_files_scanned, languages_detected, frameworks_detected,
		       test_coverage_percent, security_score
		FROM recon_scans
		WHERE env_id = ?
		ORDER BY started_at DESC
		LIMIT 1`,
		envID,
	).Scan(
		&scan.ID, &scan.EnvID, &scan.AgentID, &scan.ScanType, &mission,
		&scan.StartedAt, &completedAt, &scan.Status, &summaryJSON,
		&scan.TotalFilesScanned, &languagesJSON, &frameworksJSON,
		&testCoverage, &scan.SecurityScore,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("no scans found for environment: %s", envID)
		}
		return nil, fmt.Errorf("failed to get latest scan: %w", err)
	}

	return scanFromDB(&scan, mission, summaryJSON, languagesJSON, frameworksJSON, completedAt, testCoverage)
}

func (m *SQLiteMemoryDB) GetScan(ctx context.Context, scanID string) (*ReconScan, error) {
	var scan ReconScan
	var mission, summaryJSON, languagesJSON, frameworksJSON sql.NullString
	var completedAt sql.NullTime
	var testCoverage sql.NullInt64

	err := m.db.QueryRowContext(ctx, `
		SELECT id, env_id, agent_id, scan_type, mission, started_at, completed_at, status,
		       summary, total_files_scanned, languages_detected, frameworks_detected,
		       test_coverage_percent, security_score
		FROM recon_scans
		WHERE id = ?`,
		scanID,
	).Scan(
		&scan.ID, &scan.EnvID, &scan.AgentID, &scan.ScanType, &mission,
		&scan.StartedAt, &completedAt, &scan.Status, &summaryJSON,
		&scan.TotalFilesScanned, &languagesJSON, &frameworksJSON,
		&testCoverage, &scan.SecurityScore,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("scan not found: %s", scanID)
		}
		return nil, fmt.Errorf("failed to get scan: %w", err)
	}

	return scanFromDB(&scan, mission, summaryJSON, languagesJSON, frameworksJSON, completedAt, testCoverage)
}

func (m *SQLiteMemoryDB) GetScans(ctx context.Context, filter ScanFilter) ([]*ReconScan, error) {
	query := `
		SELECT id, env_id, agent_id, scan_type, mission, started_at, completed_at, status,
		       summary, total_files_scanned, languages_detected, frameworks_detected,
		       test_coverage_percent, security_score
		FROM recon_scans
		WHERE 1=1`
	var args []interface{}

	if filter.EnvID != "" {
		query += " AND env_id = ?"
		args = append(args, filter.EnvID)
	}
	if filter.AgentID != "" {
		query += " AND agent_id = ?"
		args = append(args, filter.AgentID)
	}
	if filter.ScanType != "" {
		query += " AND scan_type = ?"
		args = append(args, filter.ScanType)
	}
	if filter.Status != "" {
		query += " AND status = ?"
		args = append(args, filter.Status)
	}

	query += " ORDER BY started_at DESC"

	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
	}
	if filter.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, filter.Offset)
	}

	rows, err := m.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query scans: %w", err)
	}
	defer rows.Close()

	var scans []*ReconScan
	for rows.Next() {
		var scan ReconScan
		var mission, summaryJSON, languagesJSON, frameworksJSON sql.NullString
		var completedAt sql.NullTime
		var testCoverage sql.NullInt64

		err := rows.Scan(
			&scan.ID, &scan.EnvID, &scan.AgentID, &scan.ScanType, &mission,
			&scan.StartedAt, &completedAt, &scan.Status, &summaryJSON,
			&scan.TotalFilesScanned, &languagesJSON, &frameworksJSON,
			&testCoverage, &scan.SecurityScore,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		scan.Mission = mission.String
		if completedAt.Valid {
			scan.CompletedAt = &completedAt.Time
		}
		if testCoverage.Valid {
			val := int(testCoverage.Int64)
			scan.TestCoveragePercent = &val
		}

		if summaryJSON.Valid && summaryJSON.String != "" {
			var summary ScanSummary
			if err := json.Unmarshal([]byte(summaryJSON.String), &summary); err == nil {
				scan.Summary = &summary
			}
		}

		if languagesJSON.Valid && languagesJSON.String != "" {
			if err := json.Unmarshal([]byte(languagesJSON.String), &scan.LanguagesDetected); err != nil {
				scan.LanguagesDetected = nil
			}
		}

		if frameworksJSON.Valid && frameworksJSON.String != "" {
			if err := json.Unmarshal([]byte(frameworksJSON.String), &scan.FrameworksDetected); err != nil {
				scan.FrameworksDetected = nil
			}
		}

		scans = append(scans, &scan)
	}

	return scans, rows.Err()
}

// Finding operations

func (m *SQLiteMemoryDB) SaveFinding(ctx context.Context, finding *ReconFinding) error {
	return m.SaveFindings(ctx, []*ReconFinding{finding})
}

func (m *SQLiteMemoryDB) SaveFindings(ctx context.Context, findings []*ReconFinding) error {
	return m.withTx(func(tx *sql.Tx) error {
		stmt, err := tx.PrepareContext(ctx, `
			INSERT INTO recon_findings
			(id, scan_id, env_id, finding_type, severity, title, description, location,
			 recommendation, status, metadata)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET
				title = excluded.title,
				description = excluded.description,
				recommendation = excluded.recommendation,
				updated_at = CURRENT_TIMESTAMP`)
		if err != nil {
			return fmt.Errorf("failed to prepare finding insert: %w", err)
		}
		defer stmt.Close()

		for _, finding := range findings {
			metadataJSON, _ := json.Marshal(finding.Metadata)
			_, err := stmt.ExecContext(ctx,
				finding.ID, finding.ScanID, finding.EnvID, finding.FindingType,
				finding.Severity, finding.Title, finding.Description,
				nullString(finding.Location), nullString(finding.Recommendation),
				finding.Status, nullString(string(metadataJSON)),
			)
			if err != nil {
				return fmt.Errorf("failed to insert finding %s: %w", finding.ID, err)
			}
		}

		return nil
	})
}

func (m *SQLiteMemoryDB) GetFinding(ctx context.Context, id string) (*ReconFinding, error) {
	var finding ReconFinding
	var location, recommendation, resolvedBy, resolutionNotes, metadataJSON sql.NullString
	var resolvedAt sql.NullTime

	err := m.db.QueryRowContext(ctx, `
		SELECT id, scan_id, env_id, finding_type, severity, title, description, location,
		       recommendation, status, resolved_at, resolved_by, resolution_notes,
		       metadata, discovered_at, updated_at
		FROM recon_findings
		WHERE id = ?`,
		id,
	).Scan(
		&finding.ID, &finding.ScanID, &finding.EnvID, &finding.FindingType,
		&finding.Severity, &finding.Title, &finding.Description, &location,
		&recommendation, &finding.Status, &resolvedAt, &resolvedBy,
		&resolutionNotes, &metadataJSON, &finding.DiscoveredAt, &finding.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("finding not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get finding: %w", err)
	}

	finding.Location = location.String
	finding.Recommendation = recommendation.String
	finding.ResolvedBy = resolvedBy.String
	finding.ResolutionNotes = resolutionNotes.String
	if resolvedAt.Valid {
		finding.ResolvedAt = &resolvedAt.Time
	}

	if metadataJSON.Valid && metadataJSON.String != "" {
		if err := json.Unmarshal([]byte(metadataJSON.String), &finding.Metadata); err != nil {
			finding.Metadata = nil
		}
	}

	return &finding, nil
}

func (m *SQLiteMemoryDB) GetFindingsByEnvironment(ctx context.Context, envID string) ([]*ReconFinding, error) {
	return m.GetFindings(ctx, FindingFilter{EnvID: envID})
}

func (m *SQLiteMemoryDB) GetFindingsBySeverity(ctx context.Context, severity string) ([]*ReconFinding, error) {
	return m.GetFindings(ctx, FindingFilter{Severity: severity})
}

func (m *SQLiteMemoryDB) GetFindingsByScan(ctx context.Context, scanID string) ([]*ReconFinding, error) {
	return m.GetFindings(ctx, FindingFilter{ScanID: scanID})
}

func (m *SQLiteMemoryDB) GetFindings(ctx context.Context, filter FindingFilter) ([]*ReconFinding, error) {
	query := `
		SELECT id, scan_id, env_id, finding_type, severity, title, description, location,
		       recommendation, status, resolved_at, resolved_by, resolution_notes,
		       metadata, discovered_at, updated_at
		FROM recon_findings
		WHERE 1=1`
	var args []interface{}

	if filter.EnvID != "" {
		query += " AND env_id = ?"
		args = append(args, filter.EnvID)
	}
	if filter.ScanID != "" {
		query += " AND scan_id = ?"
		args = append(args, filter.ScanID)
	}
	if filter.FindingType != "" {
		query += " AND finding_type = ?"
		args = append(args, filter.FindingType)
	}
	if filter.Severity != "" {
		query += " AND severity = ?"
		args = append(args, filter.Severity)
	}
	if filter.Status != "" {
		query += " AND status = ?"
		args = append(args, filter.Status)
	}

	query += " ORDER BY discovered_at DESC"

	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
	}
	if filter.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, filter.Offset)
	}

	rows, err := m.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query findings: %w", err)
	}
	defer rows.Close()

	var findings []*ReconFinding
	for rows.Next() {
		var finding ReconFinding
		var location, recommendation, resolvedBy, resolutionNotes, metadataJSON sql.NullString
		var resolvedAt sql.NullTime

		err := rows.Scan(
			&finding.ID, &finding.ScanID, &finding.EnvID, &finding.FindingType,
			&finding.Severity, &finding.Title, &finding.Description, &location,
			&recommendation, &finding.Status, &resolvedAt, &resolvedBy,
			&resolutionNotes, &metadataJSON, &finding.DiscoveredAt, &finding.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan finding: %w", err)
		}

		finding.Location = location.String
		finding.Recommendation = recommendation.String
		finding.ResolvedBy = resolvedBy.String
		finding.ResolutionNotes = resolutionNotes.String
		if resolvedAt.Valid {
			finding.ResolvedAt = &resolvedAt.Time
		}

		if metadataJSON.Valid && metadataJSON.String != "" {
			if err := json.Unmarshal([]byte(metadataJSON.String), &finding.Metadata); err != nil {
				finding.Metadata = nil
			}
		}

		findings = append(findings, &finding)
	}

	return findings, rows.Err()
}

func (m *SQLiteMemoryDB) UpdateFindingStatus(ctx context.Context, id, status, resolvedBy, notes string) error {
	query := `UPDATE recon_findings SET status = ?, updated_at = CURRENT_TIMESTAMP`
	args := []interface{}{status}

	if status == "resolved" {
		query += ", resolved_at = CURRENT_TIMESTAMP, resolved_by = ?, resolution_notes = ?"
		args = append(args, resolvedBy, notes)
	}

	query += " WHERE id = ?"
	args = append(args, id)

	result, err := m.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to update finding status: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check affected rows: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("finding not found: %s", id)
	}

	return nil
}

// Finding history operations

func (m *SQLiteMemoryDB) RecordFindingChange(ctx context.Context, change *FindingHistoryEntry) error {
	_, err := m.db.ExecContext(ctx, `
		INSERT INTO recon_finding_history
		(finding_id, changed_by, change_type, old_value, new_value, notes)
		VALUES (?, ?, ?, ?, ?, ?)`,
		change.FindingID, change.ChangedBy, change.ChangeType,
		nullString(change.OldValue), nullString(change.NewValue), nullString(change.Notes),
	)
	return err
}

func (m *SQLiteMemoryDB) GetFindingHistory(ctx context.Context, findingID string) ([]*FindingHistoryEntry, error) {
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, finding_id, changed_by, change_type, old_value, new_value, notes, changed_at
		FROM recon_finding_history
		WHERE finding_id = ?
		ORDER BY changed_at DESC`,
		findingID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query finding history: %w", err)
	}
	defer rows.Close()

	var history []*FindingHistoryEntry
	for rows.Next() {
		var entry FindingHistoryEntry
		var oldValue, newValue, notes sql.NullString

		err := rows.Scan(
			&entry.ID, &entry.FindingID, &entry.ChangedBy, &entry.ChangeType,
			&oldValue, &newValue, &notes, &entry.ChangedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan history entry: %w", err)
		}

		entry.OldValue = oldValue.String
		entry.NewValue = newValue.String
		entry.Notes = notes.String

		history = append(history, &entry)
	}

	return history, rows.Err()
}

// Helper functions

func scanFromDB(scan *ReconScan, mission, summaryJSON, languagesJSON, frameworksJSON sql.NullString, completedAt sql.NullTime, testCoverage sql.NullInt64) (*ReconScan, error) {
	scan.Mission = mission.String
	if completedAt.Valid {
		scan.CompletedAt = &completedAt.Time
	}
	if testCoverage.Valid {
		val := int(testCoverage.Int64)
		scan.TestCoveragePercent = &val
	}

	if summaryJSON.Valid && summaryJSON.String != "" {
		var summary ScanSummary
		if err := json.Unmarshal([]byte(summaryJSON.String), &summary); err != nil {
			return nil, fmt.Errorf("failed to unmarshal summary: %w", err)
		}
		scan.Summary = &summary
	}

	if languagesJSON.Valid && languagesJSON.String != "" {
		if err := json.Unmarshal([]byte(languagesJSON.String), &scan.LanguagesDetected); err != nil {
			scan.LanguagesDetected = nil
		}
	}

	if frameworksJSON.Valid && frameworksJSON.String != "" {
		if err := json.Unmarshal([]byte(frameworksJSON.String), &scan.FrameworksDetected); err != nil {
			scan.FrameworksDetected = nil
		}
	}

	return scan, nil
}

func intPtr(i *int) *int64 {
	if i == nil {
		return nil
	}
	val := int64(*i)
	return &val
}
