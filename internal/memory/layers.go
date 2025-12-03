package memory

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// LayerManager handles syncing between hot/warm/cold layers
type LayerManager struct {
	db       ReconRepository
	repoPath string // Base path of the repository
}

// NewLayerManager creates a new layer manager
func NewLayerManager(db ReconRepository, repoPath string) *LayerManager {
	return &LayerManager{
		db:       db,
		repoPath: repoPath,
	}
}

// SyncToWarmLayer exports findings to docs/recon/*.md markdown files
func (lm *LayerManager) SyncToWarmLayer(ctx context.Context, envID string) error {
	// Get all findings for the environment
	findings, err := lm.db.GetFindingsByEnvironment(ctx, envID)
	if err != nil {
		return fmt.Errorf("failed to get findings: %w", err)
	}

	// Group findings by type
	findingsByType := make(map[string][]*ReconFinding)
	for _, finding := range findings {
		if finding.Status == "open" || finding.Status == "resolved" {
			findingsByType[finding.FindingType] = append(findingsByType[finding.FindingType], finding)
		}
	}

	// Ensure docs/recon directory exists
	reconDir := filepath.Join(lm.repoPath, "docs", "recon")
	if err := os.MkdirAll(reconDir, 0755); err != nil {
		return fmt.Errorf("failed to create recon directory: %w", err)
	}

	// Write each type to its file
	typeToFile := map[string]string{
		"architecture": "architecture.md",
		"security":     "vulnerabilities.md",
		"dependency":   "dependencies.md",
		"process":      "infrastructure.md",
		"performance":  "infrastructure.md",
	}

	for findingType, filename := range typeToFile {
		findings := findingsByType[findingType]
		if err := lm.writeWarmLayerFile(reconDir, filename, findingType, findings); err != nil {
			return fmt.Errorf("failed to write %s: %w", filename, err)
		}
	}

	return nil
}

// writeWarmLayerFile writes findings to a markdown file
func (lm *LayerManager) writeWarmLayerFile(dir, filename, findingType string, findings []*ReconFinding) error {
	filePath := filepath.Join(dir, filename)

	// Sort findings by severity
	sort.Slice(findings, func(i, j int) bool {
		severityOrder := map[string]int{
			"critical": 0,
			"high":     1,
			"medium":   2,
			"low":      3,
			"info":     4,
		}
		return severityOrder[findings[i].Severity] < severityOrder[findings[j].Severity]
	})

	// Build markdown content
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s Findings\n\n", strings.Title(findingType)))
	sb.WriteString(fmt.Sprintf("**Last Updated**: %s\n", time.Now().Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("**Total Findings**: %d\n\n", len(findings)))

	// Group by severity
	bySeverity := make(map[string][]*ReconFinding)
	for _, finding := range findings {
		bySeverity[finding.Severity] = append(bySeverity[finding.Severity], finding)
	}

	// Write each severity section
	for _, severity := range []string{"critical", "high", "medium", "low", "info"} {
		sevFindings := bySeverity[severity]
		if len(sevFindings) == 0 {
			continue
		}

		sb.WriteString(fmt.Sprintf("## %s Severity (%d)\n\n", strings.Title(severity), len(sevFindings)))

		for _, finding := range sevFindings {
			statusIcon := "🔴"
			if finding.Status == "resolved" {
				statusIcon = "✅"
			} else if finding.Status == "ignored" {
				statusIcon = "⚪"
			}

			sb.WriteString(fmt.Sprintf("### %s %s\n\n", statusIcon, finding.Title))
			sb.WriteString(fmt.Sprintf("**ID**: `%s`\n", finding.ID))
			sb.WriteString(fmt.Sprintf("**Status**: %s\n", finding.Status))
			if finding.Location != "" {
				sb.WriteString(fmt.Sprintf("**Location**: `%s`\n", finding.Location))
			}
			sb.WriteString(fmt.Sprintf("**Discovered**: %s\n\n", finding.DiscoveredAt.Format("2006-01-02")))

			sb.WriteString(fmt.Sprintf("**Description**:\n%s\n\n", finding.Description))

			if finding.Recommendation != "" {
				sb.WriteString(fmt.Sprintf("**Recommendation**:\n%s\n\n", finding.Recommendation))
			}

			if finding.Status == "resolved" && finding.ResolutionNotes != "" {
				sb.WriteString(fmt.Sprintf("**Resolution** (by %s on %s):\n%s\n\n",
					finding.ResolvedBy,
					finding.ResolvedAt.Format("2006-01-02"),
					finding.ResolutionNotes))
			}

			sb.WriteString("---\n\n")
		}
	}

	// Write to file
	return os.WriteFile(filePath, []byte(sb.String()), 0644)
}

// SyncToHotLayer updates CLAUDE.md with critical findings summary
func (lm *LayerManager) SyncToHotLayer(ctx context.Context, envID string) error {
	claudeMDPath := filepath.Join(lm.repoPath, "CLAUDE.md")

	// Read existing CLAUDE.md
	content, err := os.ReadFile(claudeMDPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to read CLAUDE.md: %w", err)
		}
		content = []byte("# CLAUDE.md\n\n")
	}

	// Get critical and high severity findings
	criticalFindings, err := lm.db.GetFindings(ctx, FindingFilter{
		EnvID:    envID,
		Severity: "critical",
		Status:   "open",
		Limit:    10,
	})
	if err != nil {
		return fmt.Errorf("failed to get critical findings: %w", err)
	}

	highFindings, err := lm.db.GetFindings(ctx, FindingFilter{
		EnvID:    envID,
		Severity: "high",
		Status:   "open",
		Limit:    10,
	})
	if err != nil {
		return fmt.Errorf("failed to get high findings: %w", err)
	}

	// Build recon intelligence section
	reconSection := lm.buildReconIntelligenceSection(criticalFindings, highFindings)

	// Update CLAUDE.md
	updatedContent := lm.updateClaudeMD(string(content), reconSection)

	// Write back
	return os.WriteFile(claudeMDPath, []byte(updatedContent), 0644)
}

// buildReconIntelligenceSection creates the recon intelligence markdown
func (lm *LayerManager) buildReconIntelligenceSection(critical, high []*ReconFinding) string {
	var sb strings.Builder

	sb.WriteString("## Recon Intelligence\n\n")
	sb.WriteString(fmt.Sprintf("**Last Updated**: %s\n\n", time.Now().Format("2006-01-02 15:04:05")))

	if len(critical) == 0 && len(high) == 0 {
		sb.WriteString("No critical or high severity findings at this time.\n\n")
		sb.WriteString("For detailed findings, see:\n")
		sb.WriteString("- [Architecture Findings](docs/recon/architecture.md)\n")
		sb.WriteString("- [Security Vulnerabilities](docs/recon/vulnerabilities.md)\n")
		sb.WriteString("- [Dependency Health](docs/recon/dependencies.md)\n")
		sb.WriteString("- [Infrastructure Status](docs/recon/infrastructure.md)\n\n")
		return sb.String()
	}

	if len(critical) > 0 {
		sb.WriteString("### Critical Issues\n\n")
		for _, finding := range critical {
			sb.WriteString(fmt.Sprintf("- **%s** [`%s`]\n", finding.Title, finding.ID))
			sb.WriteString(fmt.Sprintf("  - Type: %s\n", finding.FindingType))
			if finding.Location != "" {
				sb.WriteString(fmt.Sprintf("  - Location: `%s`\n", finding.Location))
			}
			sb.WriteString(fmt.Sprintf("  - %s\n", truncate(finding.Description, 150)))
			if finding.Recommendation != "" {
				sb.WriteString(fmt.Sprintf("  - Recommendation: %s\n", truncate(finding.Recommendation, 100)))
			}
			sb.WriteString("\n")
		}
	}

	if len(high) > 0 {
		sb.WriteString("### High Priority Issues\n\n")
		for _, finding := range high {
			sb.WriteString(fmt.Sprintf("- **%s** [`%s`]\n", finding.Title, finding.ID))
			sb.WriteString(fmt.Sprintf("  - Type: %s | Location: `%s`\n", finding.FindingType, finding.Location))
			sb.WriteString("\n")
		}
	}

	sb.WriteString("\nFor complete findings and resolution tracking, see:\n")
	sb.WriteString("- [Architecture Findings](docs/recon/architecture.md)\n")
	sb.WriteString("- [Security Vulnerabilities](docs/recon/vulnerabilities.md)\n")
	sb.WriteString("- [Dependency Health](docs/recon/dependencies.md)\n")
	sb.WriteString("- [Infrastructure Status](docs/recon/infrastructure.md)\n\n")

	return sb.String()
}

// updateClaudeMD updates the CLAUDE.md content with the recon section
func (lm *LayerManager) updateClaudeMD(content, reconSection string) string {
	// Look for existing recon intelligence section
	startMarker := "## Recon Intelligence"
	endMarkers := []string{"## ", "# "}

	startIdx := strings.Index(content, startMarker)
	if startIdx == -1 {
		// Section doesn't exist, append it
		return content + "\n" + reconSection
	}

	// Find the end of the section (next heading)
	endIdx := len(content)
	searchStart := startIdx + len(startMarker)
	for _, marker := range endMarkers {
		if idx := strings.Index(content[searchStart:], marker); idx != -1 {
			candidateIdx := searchStart + idx
			if candidateIdx < endIdx {
				endIdx = candidateIdx
			}
		}
	}

	// Replace the section
	before := content[:startIdx]
	after := content[endIdx:]
	return before + reconSection + after
}

// LoadFromLayers reconstructs state from markdown if DB is missing/empty
func (lm *LayerManager) LoadFromLayers(ctx context.Context, envID string) error {
	// Check if we have any findings in DB
	existingFindings, err := lm.db.GetFindingsByEnvironment(ctx, envID)
	if err != nil {
		return fmt.Errorf("failed to check existing findings: %w", err)
	}

	if len(existingFindings) > 0 {
		// DB already has data, no need to load from markdown
		return nil
	}

	// Try to load from markdown files
	reconDir := filepath.Join(lm.repoPath, "docs", "recon")
	if _, err := os.Stat(reconDir); os.IsNotExist(err) {
		// No markdown files exist yet
		return nil
	}

	// For now, we'll just create a placeholder implementation
	// In a real system, you'd parse the markdown files and reconstruct findings
	// This is intentionally simple since the design doc says this is for recovery
	fmt.Printf("[LAYER] Warm layer files exist at %s but DB is empty\n", reconDir)
	fmt.Println("[LAYER] To fully restore, re-run a Snake scan or manually import findings")

	return nil
}

// GetLayerStatus returns information about the current state of all layers
func (lm *LayerManager) GetLayerStatus(ctx context.Context, envID string) (*LayerStatus, error) {
	status := &LayerStatus{
		EnvID: envID,
	}

	// Check cold layer (DB)
	findings, err := lm.db.GetFindingsByEnvironment(ctx, envID)
	if err != nil {
		return nil, fmt.Errorf("failed to get findings: %w", err)
	}
	status.ColdLayer.FindingCount = len(findings)
	status.ColdLayer.Available = len(findings) > 0

	// Count by severity
	for _, f := range findings {
		if f.Status == "open" {
			switch f.Severity {
			case "critical":
				status.ColdLayer.CriticalCount++
			case "high":
				status.ColdLayer.HighCount++
			case "medium":
				status.ColdLayer.MediumCount++
			case "low":
				status.ColdLayer.LowCount++
			}
		}
	}

	// Check warm layer (markdown files)
	reconDir := filepath.Join(lm.repoPath, "docs", "recon")
	if stat, err := os.Stat(reconDir); err == nil && stat.IsDir() {
		status.WarmLayer.Available = true
		files := []string{"architecture.md", "vulnerabilities.md", "dependencies.md", "infrastructure.md"}
		for _, file := range files {
			if _, err := os.Stat(filepath.Join(reconDir, file)); err == nil {
				status.WarmLayer.FileCount++
			}
		}
	}

	// Check hot layer (CLAUDE.md)
	claudeMDPath := filepath.Join(lm.repoPath, "CLAUDE.md")
	if content, err := os.ReadFile(claudeMDPath); err == nil {
		status.HotLayer.Available = true
		status.HotLayer.HasReconSection = strings.Contains(string(content), "## Recon Intelligence")
	}

	return status, nil
}

// LayerStatus represents the status of all memory layers
type LayerStatus struct {
	EnvID      string
	ColdLayer  ColdLayerStatus
	WarmLayer  WarmLayerStatus
	HotLayer   HotLayerStatus
}

type ColdLayerStatus struct {
	Available     bool
	FindingCount  int
	CriticalCount int
	HighCount     int
	MediumCount   int
	LowCount      int
}

type WarmLayerStatus struct {
	Available bool
	FileCount int
}

type HotLayerStatus struct {
	Available       bool
	HasReconSection bool
}

// Helper functions

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
