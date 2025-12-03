package bootstrap

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// PhoneHomeClient manages communication with Magnolia HQ
type PhoneHomeClient interface {
	// Send findings report to Magnolia HQ
	SendReport(ctx context.Context, report *PhoneHomeReport) error

	// Get pending tasks/instructions from HQ
	GetInstructions(ctx context.Context) (*HQInstructions, error)

	// Heartbeat - let HQ know we're alive
	Heartbeat(ctx context.Context) error

	// Upload full state for backup
	SyncState(ctx context.Context, state *PortableState) error
}

// PhoneHomeReport is sent to Magnolia HQ
type PhoneHomeReport struct {
	CaptainID       string         `json:"captain_id"`
	Environment     string         `json:"environment"`
	Timestamp       time.Time      `json:"timestamp"`
	FindingsSummary map[string]int `json:"findings_summary"`
	ActiveAgents    []string       `json:"active_agents"`
	Status          string         `json:"status"` // scanning, coordinating, idle
	NeedsHelp       bool           `json:"needs_help"`
	HelpReason      string         `json:"help_reason,omitempty"`
}

// HQInstructions received from Magnolia HQ
type HQInstructions struct {
	Priority      string                 `json:"priority"` // critical, high, medium, low
	Tasks         []HQTask               `json:"tasks"`
	ConfigUpdates map[string]interface{} `json:"config_updates"`
	AbortMission  bool                   `json:"abort_mission"`
	AbortReason   string                 `json:"abort_reason,omitempty"`
}

// HQTask represents a task assigned from HQ
type HQTask struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"` // recon, fix, audit, report
	Description string    `json:"description"`
	Priority    int       `json:"priority"`
	Deadline    time.Time `json:"deadline,omitempty"`
}

// HTTPPhoneHomeClient implements PhoneHomeClient using HTTP
type HTTPPhoneHomeClient struct {
	endpoint   string
	apiKey     string
	captainID  string
	httpClient *http.Client
}

// NewPhoneHomeClient creates a new phone home client
func NewPhoneHomeClient(endpoint, apiKeyEnv, captainID string) (PhoneHomeClient, error) {
	// Get API key from environment
	apiKey := os.Getenv(apiKeyEnv)
	if apiKey == "" {
		return nil, fmt.Errorf("API key not found in environment variable: %s", apiKeyEnv)
	}

	// Create HTTP client with reasonable timeouts
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
			},
			MaxIdleConns:        10,
			IdleConnTimeout:     90 * time.Second,
			DisableCompression:  false,
		},
	}

	return &HTTPPhoneHomeClient{
		endpoint:   endpoint,
		apiKey:     apiKey,
		captainID:  captainID,
		httpClient: client,
	}, nil
}

// SendReport sends a findings report to Magnolia HQ
func (c *HTTPPhoneHomeClient) SendReport(ctx context.Context, report *PhoneHomeReport) error {
	// Set captain ID if not already set
	if report.CaptainID == "" {
		report.CaptainID = c.captainID
	}

	data, err := json.Marshal(report)
	if err != nil {
		return fmt.Errorf("failed to marshal report: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.endpoint+"/reports", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("X-Captain-ID", c.captainID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send report: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HQ returned error %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// GetInstructions retrieves pending instructions from Magnolia HQ
func (c *HTTPPhoneHomeClient) GetInstructions(ctx context.Context) (*HQInstructions, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.endpoint+"/instructions", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("X-Captain-ID", c.captainID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get instructions: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HQ returned error %d: %s", resp.StatusCode, string(body))
	}

	var instructions HQInstructions
	if err := json.NewDecoder(resp.Body).Decode(&instructions); err != nil {
		return nil, fmt.Errorf("failed to decode instructions: %w", err)
	}

	return &instructions, nil
}

// Heartbeat sends a heartbeat to let HQ know Captain is alive
func (c *HTTPPhoneHomeClient) Heartbeat(ctx context.Context) error {
	heartbeat := map[string]interface{}{
		"captain_id": c.captainID,
		"timestamp":  time.Now().Format(time.RFC3339),
		"status":     "alive",
	}

	data, err := json.Marshal(heartbeat)
	if err != nil {
		return fmt.Errorf("failed to marshal heartbeat: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.endpoint+"/heartbeat", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("X-Captain-ID", c.captainID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send heartbeat: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HQ returned error %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// SyncState uploads full state to HQ for backup
func (c *HTTPPhoneHomeClient) SyncState(ctx context.Context, state *PortableState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.endpoint+"/state", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("X-Captain-ID", c.captainID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to sync state: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HQ returned error %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// MockPhoneHomeClient for testing without actual HQ connection
type MockPhoneHomeClient struct {
	ReportsSent   []*PhoneHomeReport
	StatesSynced  []*PortableState
	HeartbeatsSent int
	Instructions  *HQInstructions
	ShouldError   bool
}

// NewMockPhoneHomeClient creates a mock client for testing
func NewMockPhoneHomeClient() *MockPhoneHomeClient {
	return &MockPhoneHomeClient{
		ReportsSent:  make([]*PhoneHomeReport, 0),
		StatesSynced: make([]*PortableState, 0),
		Instructions: &HQInstructions{
			Priority: "low",
			Tasks:    []HQTask{},
		},
	}
}

func (m *MockPhoneHomeClient) SendReport(ctx context.Context, report *PhoneHomeReport) error {
	if m.ShouldError {
		return fmt.Errorf("mock error: simulated failure")
	}
	m.ReportsSent = append(m.ReportsSent, report)
	return nil
}

func (m *MockPhoneHomeClient) GetInstructions(ctx context.Context) (*HQInstructions, error) {
	if m.ShouldError {
		return nil, fmt.Errorf("mock error: simulated failure")
	}
	return m.Instructions, nil
}

func (m *MockPhoneHomeClient) Heartbeat(ctx context.Context) error {
	if m.ShouldError {
		return fmt.Errorf("mock error: simulated failure")
	}
	m.HeartbeatsSent++
	return nil
}

func (m *MockPhoneHomeClient) SyncState(ctx context.Context, state *PortableState) error {
	if m.ShouldError {
		return fmt.Errorf("mock error: simulated failure")
	}
	m.StatesSynced = append(m.StatesSynced, state)
	return nil
}
