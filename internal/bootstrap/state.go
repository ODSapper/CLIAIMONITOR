package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/CLIAIMONITOR/internal/memory"
)

// StateManager manages portable bootstrap state
type StateManager interface {
	// Load state from file
	LoadState(path string) (*PortableState, error)

	// Save state to file
	SaveState(state *PortableState, path string) error

	// Merge findings into state
	MergeFindings(state *PortableState, findings []*memory.ReconFinding) error

	// Export state for phone home
	ExportForSync(state *PortableState) (*PhoneHomeReport, error)

	// Import state from HQ backup
	ImportFromHQ(data []byte) (*PortableState, error)

	// Reconstruct memory DB from portable state
	ReconstructMemory(ctx context.Context, state *PortableState, reconRepo memory.ReconRepository) error
}

// PortableState is the minimal JSON state for Captain in infrastructure-poor environments
type PortableState struct {
	Version     string              `json:"version"`
	CaptainID   string              `json:"captain_id"`
	Environment EnvironmentInfo     `json:"environment"`
	Mode        string              `json:"mode"` // lightweight, local, connected, full
	FindingsSummary FindingsSummary `json:"findings_summary"`
	ActiveAgents    []string        `json:"active_agents"`
	PendingDecisions []string       `json:"pending_decisions"`
	PhoneHome    PhoneHomeConfig     `json:"phone_home"`
	ScaleUp      ScaleUpStatus       `json:"scale_up"`
}

// EnvironmentInfo describes the environment being monitored
type EnvironmentInfo struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Type         string    `json:"type"` // internal, customer, test
	FirstContact time.Time `json:"first_contact"`
}

// FindingsSummary contains aggregate counts by severity
type FindingsSummary struct {
	Critical int `json:"critical"`
	High     int `json:"high"`
	Medium   int `json:"medium"`
	Low      int `json:"low"`
}

// PhoneHomeConfig configures connection to Magnolia HQ
type PhoneHomeConfig struct {
	Enabled    bool       `json:"enabled"`
	Endpoint   string     `json:"endpoint"`
	LastSync   *time.Time `json:"last_sync"`
	APIKeyEnv  string     `json:"api_key_env"` // Environment variable name containing API key
}

// ScaleUpStatus tracks infrastructure scale-up state
type ScaleUpStatus struct {
	Triggered          bool    `json:"triggered"`
	Reason             *string `json:"reason"`
	CLIAIMonitorPort   *int    `json:"cliaimonitor_port"`
}

// FileStateManager implements StateManager using filesystem
type FileStateManager struct {
}

// NewStateManager creates a new state manager
func NewStateManager() StateManager {
	return &FileStateManager{}
}

// LoadState loads portable state from JSON file
func (m *FileStateManager) LoadState(path string) (*PortableState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	var state PortableState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse state JSON: %w", err)
	}

	// Validate version
	if state.Version != "1.0" {
		return nil, fmt.Errorf("unsupported state version: %s", state.Version)
	}

	return &state, nil
}

// SaveState saves portable state to JSON file
func (m *FileStateManager) SaveState(state *PortableState, path string) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	return nil
}

// MergeFindings updates state with new findings
func (m *FileStateManager) MergeFindings(state *PortableState, findings []*memory.ReconFinding) error {
	// Reset counts
	state.FindingsSummary = FindingsSummary{}

	// Count by severity
	for _, f := range findings {
		switch f.Severity {
		case "critical":
			state.FindingsSummary.Critical++
		case "high":
			state.FindingsSummary.High++
		case "medium":
			state.FindingsSummary.Medium++
		case "low":
			state.FindingsSummary.Low++
		}
	}

	return nil
}

// ExportForSync converts state to phone home report format
func (m *FileStateManager) ExportForSync(state *PortableState) (*PhoneHomeReport, error) {
	status := "idle"
	if len(state.ActiveAgents) > 0 {
		status = "coordinating"
	}
	if state.FindingsSummary.Critical > 0 || state.FindingsSummary.High > 0 {
		status = "scanning"
	}

	needsHelp := false
	helpReason := ""
	if len(state.PendingDecisions) > 5 {
		needsHelp = true
		helpReason = fmt.Sprintf("%d pending decisions require human guidance", len(state.PendingDecisions))
	}

	report := &PhoneHomeReport{
		CaptainID:   state.CaptainID,
		Environment: state.Environment.Name,
		Timestamp:   time.Now(),
		FindingsSummary: map[string]int{
			"critical": state.FindingsSummary.Critical,
			"high":     state.FindingsSummary.High,
			"medium":   state.FindingsSummary.Medium,
			"low":      state.FindingsSummary.Low,
		},
		ActiveAgents: state.ActiveAgents,
		Status:       status,
		NeedsHelp:    needsHelp,
		HelpReason:   helpReason,
	}

	return report, nil
}

// ImportFromHQ restores state from HQ backup
func (m *FileStateManager) ImportFromHQ(data []byte) (*PortableState, error) {
	var state PortableState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse HQ backup: %w", err)
	}

	return &state, nil
}

// ReconstructMemory rebuilds memory DB from portable state
func (m *FileStateManager) ReconstructMemory(ctx context.Context, state *PortableState, reconRepo memory.ReconRepository) error {
	if reconRepo == nil {
		return fmt.Errorf("reconRepo cannot be nil")
	}

	// Register environment if not exists
	env := &memory.Environment{
		ID:          state.Environment.ID,
		Name:        state.Environment.Name,
		EnvType:     state.Environment.Type,
		RegisteredAt: state.Environment.FirstContact,
	}

	if err := reconRepo.RegisterEnvironment(ctx, env); err != nil {
		// Ignore if already exists
		if existing, getErr := reconRepo.GetEnvironment(ctx, state.Environment.ID); getErr == nil {
			env = existing
		} else {
			return fmt.Errorf("failed to register environment: %w", err)
		}
	}

	// Note: Detailed findings are not stored in portable state
	// They would need to be rescanned or synced from HQ
	// The portable state only contains summaries for lightweight operation

	return nil
}

// NewPortableState creates a new portable state for an environment
func NewPortableState(envID, envName, envType, captainID string) *PortableState {
	return &PortableState{
		Version:   "1.0",
		CaptainID: captainID,
		Environment: EnvironmentInfo{
			ID:           envID,
			Name:         envName,
			Type:         envType,
			FirstContact: time.Now(),
		},
		Mode: "lightweight",
		FindingsSummary: FindingsSummary{},
		ActiveAgents:    []string{},
		PendingDecisions: []string{},
		PhoneHome: PhoneHomeConfig{
			Enabled:   false,
			Endpoint:  "https://magnolia-hq.example.com/api/v1/reports",
			APIKeyEnv: "MAGNOLIA_API_KEY",
		},
		ScaleUp: ScaleUpStatus{
			Triggered: false,
		},
	}
}
