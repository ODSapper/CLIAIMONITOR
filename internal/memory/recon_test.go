package memory

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestReconRepository(t *testing.T) {
	// Create temp database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_recon.db")

	db, err := NewMemoryDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	t.Run("Environment operations", func(t *testing.T) {
		testEnvironmentOperations(t, db, ctx)
	})

	t.Run("Scan operations", func(t *testing.T) {
		testScanOperations(t, db, ctx)
	})

	t.Run("Finding operations", func(t *testing.T) {
		testFindingOperations(t, db, ctx)
	})

	t.Run("Finding history", func(t *testing.T) {
		testFindingHistory(t, db, ctx)
	})
}

func testEnvironmentOperations(t *testing.T, db MemoryDB, ctx context.Context) {
	// Register environment
	env := &Environment{
		ID:          "test-env-001",
		Name:        "Test Environment",
		Description: "Test environment for unit tests",
		EnvType:     "test",
		BasePath:    "/test/path",
		GitRemote:   "https://github.com/test/repo.git",
		Metadata: map[string]interface{}{
			"owner": "test-team",
			"tier":  "development",
		},
	}

	if err := db.(*SQLiteMemoryDB).RegisterEnvironment(ctx, env); err != nil {
		t.Fatalf("Failed to register environment: %v", err)
	}

	// Get environment
	retrieved, err := db.(*SQLiteMemoryDB).GetEnvironment(ctx, "test-env-001")
	if err != nil {
		t.Fatalf("Failed to get environment: %v", err)
	}

	if retrieved.Name != env.Name {
		t.Errorf("Expected name %s, got %s", env.Name, retrieved.Name)
	}

	if retrieved.EnvType != env.EnvType {
		t.Errorf("Expected type %s, got %s", env.EnvType, retrieved.EnvType)
	}

	// List environments
	envs, err := db.(*SQLiteMemoryDB).ListEnvironments(ctx)
	if err != nil {
		t.Fatalf("Failed to list environments: %v", err)
	}

	if len(envs) == 0 {
		t.Error("Expected at least one environment")
	}

	// Update last scan
	if err := db.(*SQLiteMemoryDB).UpdateEnvironmentLastScan(ctx, "test-env-001"); err != nil {
		t.Fatalf("Failed to update last scan: %v", err)
	}

	// Verify last scan was updated
	updated, err := db.(*SQLiteMemoryDB).GetEnvironment(ctx, "test-env-001")
	if err != nil {
		t.Fatalf("Failed to get updated environment: %v", err)
	}

	if updated.LastScanned == nil {
		t.Error("Expected last_scanned to be set")
	}
}

func testScanOperations(t *testing.T, db MemoryDB, ctx context.Context) {
	// Ensure environment exists
	env := &Environment{
		ID:      "test-env-scan",
		Name:    "Scan Test Env",
		EnvType: "test",
	}
	db.(*SQLiteMemoryDB).RegisterEnvironment(ctx, env)

	// Record scan
	scan := &ReconScan{
		ID:                "SCAN-001",
		EnvID:             "test-env-scan",
		AgentID:           "Snake001",
		ScanType:          "initial",
		Mission:           "Full codebase reconnaissance",
		Status:            "running",
		TotalFilesScanned: 150,
		LanguagesDetected: []string{"go", "python", "javascript"},
		FrameworksDetected: []string{"chi", "flask", "react"},
		SecurityScore:     "B",
	}

	if err := db.(*SQLiteMemoryDB).RecordScan(ctx, scan); err != nil {
		t.Fatalf("Failed to record scan: %v", err)
	}

	// Get scan
	retrieved, err := db.(*SQLiteMemoryDB).GetScan(ctx, "SCAN-001")
	if err != nil {
		t.Fatalf("Failed to get scan: %v", err)
	}

	if retrieved.AgentID != "Snake001" {
		t.Errorf("Expected agent Snake001, got %s", retrieved.AgentID)
	}

	// Update status
	if err := db.(*SQLiteMemoryDB).UpdateScanStatus(ctx, "SCAN-001", "completed"); err != nil {
		t.Fatalf("Failed to update scan status: %v", err)
	}

	// Complete scan with summary
	summary := &ScanSummary{
		TotalFiles:    150,
		Languages:     []string{"go", "python"},
		SecurityScore: "B",
		CriticalCount: 2,
		HighCount:     5,
		MediumCount:   10,
		LowCount:      3,
	}

	if err := db.(*SQLiteMemoryDB).CompleteScan(ctx, "SCAN-001", summary); err != nil {
		t.Fatalf("Failed to complete scan: %v", err)
	}

	// Get latest scan
	latest, err := db.(*SQLiteMemoryDB).GetLatestScan(ctx, "test-env-scan")
	if err != nil {
		t.Fatalf("Failed to get latest scan: %v", err)
	}

	if latest.Status != "completed" {
		t.Errorf("Expected status completed, got %s", latest.Status)
	}

	// Get scans with filter
	scans, err := db.(*SQLiteMemoryDB).GetScans(ctx, ScanFilter{
		EnvID:  "test-env-scan",
		Status: "completed",
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("Failed to get scans: %v", err)
	}

	if len(scans) == 0 {
		t.Error("Expected at least one scan")
	}
}

func testFindingOperations(t *testing.T, db MemoryDB, ctx context.Context) {
	// Ensure environment and scan exist
	env := &Environment{
		ID:      "test-env-findings",
		Name:    "Findings Test Env",
		EnvType: "test",
	}
	db.(*SQLiteMemoryDB).RegisterEnvironment(ctx, env)

	scan := &ReconScan{
		ID:       "SCAN-002",
		EnvID:    "test-env-findings",
		AgentID:  "Snake002",
		ScanType: "targeted",
		Status:   "completed",
	}
	db.(*SQLiteMemoryDB).RecordScan(ctx, scan)

	// Save finding
	finding := &ReconFinding{
		ID:             "VULN-001",
		ScanID:         "SCAN-002",
		EnvID:          "test-env-findings",
		FindingType:    "security",
		Severity:       "critical",
		Title:          "SQL Injection in Login",
		Description:    "User input is directly concatenated into SQL query",
		Location:       "src/auth/login.go:45",
		Recommendation: "Use parameterized queries with prepared statements",
		Status:         "open",
		Metadata: map[string]interface{}{
			"cwe": "CWE-89",
		},
	}

	if err := db.(*SQLiteMemoryDB).SaveFinding(ctx, finding); err != nil {
		t.Fatalf("Failed to save finding: %v", err)
	}

	// Get finding
	retrieved, err := db.(*SQLiteMemoryDB).GetFinding(ctx, "VULN-001")
	if err != nil {
		t.Fatalf("Failed to get finding: %v", err)
	}

	if retrieved.Title != finding.Title {
		t.Errorf("Expected title %s, got %s", finding.Title, retrieved.Title)
	}

	// Save multiple findings
	findings := []*ReconFinding{
		{
			ID:          "ARCH-001",
			ScanID:      "SCAN-002",
			EnvID:       "test-env-findings",
			FindingType: "architecture",
			Severity:    "high",
			Title:       "No rate limiting",
			Description: "API endpoints lack rate limiting",
			Status:      "open",
		},
		{
			ID:          "DEP-001",
			ScanID:      "SCAN-002",
			EnvID:       "test-env-findings",
			FindingType: "dependency",
			Severity:    "medium",
			Title:       "Outdated dependency",
			Description: "Library X is 3 versions behind",
			Status:      "open",
		},
	}

	if err := db.(*SQLiteMemoryDB).SaveFindings(ctx, findings); err != nil {
		t.Fatalf("Failed to save findings: %v", err)
	}

	// Get findings by environment
	envFindings, err := db.(*SQLiteMemoryDB).GetFindingsByEnvironment(ctx, "test-env-findings")
	if err != nil {
		t.Fatalf("Failed to get findings by environment: %v", err)
	}

	if len(envFindings) < 3 {
		t.Errorf("Expected at least 3 findings, got %d", len(envFindings))
	}

	// Get findings by severity
	criticalFindings, err := db.(*SQLiteMemoryDB).GetFindingsBySeverity(ctx, "critical")
	if err != nil {
		t.Fatalf("Failed to get findings by severity: %v", err)
	}

	if len(criticalFindings) == 0 {
		t.Error("Expected at least one critical finding")
	}

	// Get findings with filter
	filtered, err := db.(*SQLiteMemoryDB).GetFindings(ctx, FindingFilter{
		EnvID:       "test-env-findings",
		FindingType: "security",
		Severity:    "critical",
		Status:      "open",
	})
	if err != nil {
		t.Fatalf("Failed to get filtered findings: %v", err)
	}

	if len(filtered) == 0 {
		t.Error("Expected at least one filtered finding")
	}

	// Update finding status
	if err := db.(*SQLiteMemoryDB).UpdateFindingStatus(ctx, "VULN-001", "resolved", "Snake002", "Applied fix"); err != nil {
		t.Fatalf("Failed to update finding status: %v", err)
	}

	// Verify status update
	updated, err := db.(*SQLiteMemoryDB).GetFinding(ctx, "VULN-001")
	if err != nil {
		t.Fatalf("Failed to get updated finding: %v", err)
	}

	if updated.Status != "resolved" {
		t.Errorf("Expected status resolved, got %s", updated.Status)
	}

	if updated.ResolvedBy != "Snake002" {
		t.Errorf("Expected resolved by Snake002, got %s", updated.ResolvedBy)
	}
}

func testFindingHistory(t *testing.T, db MemoryDB, ctx context.Context) {
	// Ensure finding exists
	env := &Environment{
		ID:      "test-env-history",
		Name:    "History Test Env",
		EnvType: "test",
	}
	db.(*SQLiteMemoryDB).RegisterEnvironment(ctx, env)

	scan := &ReconScan{
		ID:       "SCAN-003",
		EnvID:    "test-env-history",
		AgentID:  "Snake003",
		ScanType: "initial",
		Status:   "completed",
	}
	db.(*SQLiteMemoryDB).RecordScan(ctx, scan)

	finding := &ReconFinding{
		ID:          "TEST-001",
		ScanID:      "SCAN-003",
		EnvID:       "test-env-history",
		FindingType: "security",
		Severity:    "high",
		Title:       "Test Finding for History",
		Description: "Test description",
		Status:      "open",
	}
	db.(*SQLiteMemoryDB).SaveFinding(ctx, finding)

	// Record history entry
	entry := &FindingHistoryEntry{
		FindingID:  "TEST-001",
		ChangedBy:  "Snake003",
		ChangeType: "status_change",
		OldValue:   "open",
		NewValue:   "resolved",
		Notes:      "Fixed in commit abc123",
	}

	if err := db.(*SQLiteMemoryDB).RecordFindingChange(ctx, entry); err != nil {
		t.Fatalf("Failed to record finding change: %v", err)
	}

	// Get history
	history, err := db.(*SQLiteMemoryDB).GetFindingHistory(ctx, "TEST-001")
	if err != nil {
		t.Fatalf("Failed to get finding history: %v", err)
	}

	if len(history) == 0 {
		t.Error("Expected at least one history entry")
	}

	if history[0].ChangeType != "status_change" {
		t.Errorf("Expected change type status_change, got %s", history[0].ChangeType)
	}
}

func TestLayerManager(t *testing.T) {
	// Create temp directory and database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_layers.db")

	db, err := NewMemoryDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Setup test environment and findings
	env := &Environment{
		ID:      "layer-test-env",
		Name:    "Layer Test",
		EnvType: "test",
	}
	db.(*SQLiteMemoryDB).RegisterEnvironment(ctx, env)

	scan := &ReconScan{
		ID:       "SCAN-LAYER-001",
		EnvID:    "layer-test-env",
		AgentID:  "Snake999",
		ScanType: "initial",
		Status:   "completed",
	}
	db.(*SQLiteMemoryDB).RecordScan(ctx, scan)

	findings := []*ReconFinding{
		{
			ID:             "CRIT-001",
			ScanID:         "SCAN-LAYER-001",
			EnvID:          "layer-test-env",
			FindingType:    "security",
			Severity:       "critical",
			Title:          "Critical SQL Injection",
			Description:    "Severe SQL injection vulnerability",
			Location:       "src/db/query.go:100",
			Recommendation: "Use prepared statements",
			Status:         "open",
		},
		{
			ID:          "HIGH-001",
			ScanID:      "SCAN-LAYER-001",
			EnvID:       "layer-test-env",
			FindingType: "architecture",
			Severity:    "high",
			Title:       "Missing authentication",
			Description: "Endpoint lacks authentication",
			Location:    "src/api/handler.go:50",
			Status:      "open",
		},
		{
			ID:          "MED-001",
			ScanID:      "SCAN-LAYER-001",
			EnvID:       "layer-test-env",
			FindingType: "dependency",
			Severity:    "medium",
			Title:       "Outdated library",
			Description: "Library needs update",
			Status:      "open",
		},
	}
	db.(*SQLiteMemoryDB).SaveFindings(ctx, findings)

	// Test layer manager
	lm := NewLayerManager(db.(*SQLiteMemoryDB), tmpDir)

	t.Run("SyncToWarmLayer", func(t *testing.T) {
		if err := lm.SyncToWarmLayer(ctx, "layer-test-env"); err != nil {
			t.Fatalf("Failed to sync to warm layer: %v", err)
		}

		// Verify files were created
		reconDir := filepath.Join(tmpDir, "docs", "recon")
		files := []string{"vulnerabilities.md", "architecture.md", "dependencies.md", "infrastructure.md"}

		for _, file := range files {
			path := filepath.Join(reconDir, file)
			if _, err := os.Stat(path); os.IsNotExist(err) {
				t.Errorf("Expected file %s to exist", file)
			}
		}

		// Verify content of vulnerabilities.md
		vulnPath := filepath.Join(reconDir, "vulnerabilities.md")
		content, err := os.ReadFile(vulnPath)
		if err != nil {
			t.Fatalf("Failed to read vulnerabilities.md: %v", err)
		}

		contentStr := string(content)
		if !contains(contentStr, "CRIT-001") {
			t.Error("Expected vulnerabilities.md to contain CRIT-001")
		}
		if !contains(contentStr, "Critical SQL Injection") {
			t.Error("Expected vulnerabilities.md to contain finding title")
		}
	})

	t.Run("SyncToHotLayer", func(t *testing.T) {
		// Create initial CLAUDE.md
		claudePath := filepath.Join(tmpDir, "CLAUDE.md")
		initialContent := "# CLAUDE.md\n\nProject context.\n\n## Other Section\n\nSome content.\n"
		os.WriteFile(claudePath, []byte(initialContent), 0644)

		if err := lm.SyncToHotLayer(ctx, "layer-test-env"); err != nil {
			t.Fatalf("Failed to sync to hot layer: %v", err)
		}

		// Verify CLAUDE.md was updated
		content, err := os.ReadFile(claudePath)
		if err != nil {
			t.Fatalf("Failed to read CLAUDE.md: %v", err)
		}

		contentStr := string(content)
		if !contains(contentStr, "## Recon Intelligence") {
			t.Error("Expected CLAUDE.md to contain Recon Intelligence section")
		}
		if !contains(contentStr, "CRIT-001") {
			t.Error("Expected CLAUDE.md to contain critical finding")
		}
		if !contains(contentStr, "## Other Section") {
			t.Error("Expected CLAUDE.md to preserve existing content")
		}
	})

	t.Run("GetLayerStatus", func(t *testing.T) {
		status, err := lm.GetLayerStatus(ctx, "layer-test-env")
		if err != nil {
			t.Fatalf("Failed to get layer status: %v", err)
		}

		if !status.ColdLayer.Available {
			t.Error("Expected cold layer to be available")
		}

		if status.ColdLayer.CriticalCount != 1 {
			t.Errorf("Expected 1 critical finding, got %d", status.ColdLayer.CriticalCount)
		}

		if !status.WarmLayer.Available {
			t.Error("Expected warm layer to be available")
		}

		if !status.HotLayer.Available {
			t.Error("Expected hot layer to be available")
		}

		if !status.HotLayer.HasReconSection {
			t.Error("Expected hot layer to have recon section")
		}
	})
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && len(s) >= len(substr) &&
		(s == substr || len(s) > len(substr) && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
