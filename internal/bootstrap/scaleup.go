package bootstrap

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

// InfraLevel represents the current infrastructure deployment level
type InfraLevel string

const (
	InfraLightweight InfraLevel = "lightweight" // Just state.json
	InfraLocal       InfraLevel = "local"       // CLIAIMONITOR running locally
	InfraConnected   InfraLevel = "connected"   // Phone home to Magnolia HQ
	InfraFull        InfraLevel = "full"        // Full infrastructure
)

// ScaleUpDetector determines when to deploy additional infrastructure
type ScaleUpDetector interface {
	// Check if scale-up conditions are met
	ShouldScaleUp(state *PortableState) (bool, string)

	// Perform scale-up (start CLIAIMONITOR)
	ScaleUp(ctx context.Context) error

	// Get current infrastructure level
	GetInfraLevel() InfraLevel

	// Check if CLIAIMONITOR is available
	IsCLIAIMonitorAvailable() bool
}

// StandardScaleUpDetector implements scale-up logic
type StandardScaleUpDetector struct {
	infraLevel       InfraLevel
	cliaimonitorPath string
	dataDir          string
	port             int
}

// NewScaleUpDetector creates a new scale-up detector
func NewScaleUpDetector(cliaimonitorPath, dataDir string, port int) ScaleUpDetector {
	return &StandardScaleUpDetector{
		infraLevel:       InfraLightweight,
		cliaimonitorPath: cliaimonitorPath,
		dataDir:          dataDir,
		port:             port,
	}
}

// ShouldScaleUp checks if scale-up conditions are met
func (d *StandardScaleUpDetector) ShouldScaleUp(state *PortableState) (bool, string) {
	// Already scaled up
	if state.ScaleUp.Triggered {
		return false, ""
	}

	// Trigger 1: More than 3 agents needed simultaneously
	if len(state.ActiveAgents) > 3 {
		return true, "More than 3 agents active - need full coordination infrastructure"
	}

	// Trigger 2: Multi-day engagement detected (check first contact time)
	daysSinceFirstContact := time.Since(state.Environment.FirstContact).Hours() / 24
	if daysSinceFirstContact > 1 {
		return true, fmt.Sprintf("Multi-day engagement (%.1f days) - deploying persistent infrastructure", daysSinceFirstContact)
	}

	// Trigger 3: Critical findings require immediate coordination
	if state.FindingsSummary.Critical > 0 {
		return true, fmt.Sprintf("%d critical findings require coordinated response", state.FindingsSummary.Critical)
	}

	// Trigger 4: High volume of findings suggests extended engagement
	totalFindings := state.FindingsSummary.Critical + state.FindingsSummary.High +
	                 state.FindingsSummary.Medium + state.FindingsSummary.Low
	if totalFindings > 50 {
		return true, fmt.Sprintf("%d total findings suggest extended engagement", totalFindings)
	}

	// Trigger 5: Many pending decisions suggest need for better coordination
	if len(state.PendingDecisions) > 10 {
		return true, fmt.Sprintf("%d pending decisions require coordination infrastructure", len(state.PendingDecisions))
	}

	return false, ""
}

// ScaleUp starts CLIAIMONITOR locally
func (d *StandardScaleUpDetector) ScaleUp(ctx context.Context) error {
	if !d.IsCLIAIMonitorAvailable() {
		return fmt.Errorf("CLIAIMONITOR binary not found at: %s", d.cliaimonitorPath)
	}

	// Ensure data directory exists
	if err := os.MkdirAll(d.dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	// Build command based on OS
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		// Windows: Start in background using PowerShell
		scriptCmd := fmt.Sprintf(`Start-Process -FilePath "%s" -ArgumentList "serve --port %d --data-dir %s" -WindowStyle Hidden`,
			d.cliaimonitorPath, d.port, d.dataDir)
		cmd = exec.CommandContext(ctx, "powershell.exe", "-Command", scriptCmd)
	} else {
		// Unix: Start in background using nohup
		cmd = exec.CommandContext(ctx, "nohup", d.cliaimonitorPath, "serve",
			"--port", fmt.Sprintf("%d", d.port),
			"--data-dir", d.dataDir,
			"&")
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start CLIAIMONITOR: %w", err)
	}

	// Don't wait for process - let it run in background
	go cmd.Wait()

	// Update infrastructure level
	d.infraLevel = InfraLocal

	// Wait a moment for server to start
	time.Sleep(2 * time.Second)

	// Verify server is responding by checking health endpoint
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	maxRetries := 5
	for i := 0; i < maxRetries; i++ {
		select {
		case <-ctx.Done():
			return fmt.Errorf("health check timeout while starting CLIAIMONITOR")
		default:
		}

		// Check if server is responding
		healthErr := healthCheck(d.port)
		if healthErr == nil {
			// Server is ready
			return nil
		}

		if i < maxRetries-1 {
			time.Sleep(1 * time.Second)
		}
	}

	return fmt.Errorf("CLIAIMONITOR did not become healthy within timeout")
}

// healthCheck verifies the server is responding
func healthCheck(port int) error {
	url := fmt.Sprintf("http://localhost:%d/api/health", port)
	client := &http.Client{
		Timeout: 2 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check returned status %d", resp.StatusCode)
	}

	return nil
}

// GetInfraLevel returns the current infrastructure level
func (d *StandardScaleUpDetector) GetInfraLevel() InfraLevel {
	return d.infraLevel
}

// IsCLIAIMonitorAvailable checks if CLIAIMONITOR binary exists
func (d *StandardScaleUpDetector) IsCLIAIMonitorAvailable() bool {
	// Check if path exists
	if _, err := os.Stat(d.cliaimonitorPath); err != nil {
		return false
	}

	// Try to find in PATH if not absolute path
	if !filepath.IsAbs(d.cliaimonitorPath) {
		if _, err := exec.LookPath(d.cliaimonitorPath); err != nil {
			return false
		}
	}

	return true
}

// ScaleUpConfig contains configuration for scale-up operations
type ScaleUpConfig struct {
	CLIAIMonitorPath string `json:"cliaimonitor_path"`
	DataDir          string `json:"data_dir"`
	Port             int    `json:"port"`
	AutoScaleUp      bool   `json:"auto_scale_up"` // Automatically scale up when conditions met
}

// DefaultScaleUpConfig returns default configuration
func DefaultScaleUpConfig() *ScaleUpConfig {
	// Try to find cliaimonitor binary
	cliaimonitorPath := "cliaimonitor"
	if runtime.GOOS == "windows" {
		cliaimonitorPath = "cliaimonitor.exe"
	}

	// Check current directory first
	if _, err := os.Stat(cliaimonitorPath); err != nil {
		// Try to find in PATH
		if path, err := exec.LookPath(cliaimonitorPath); err == nil {
			cliaimonitorPath = path
		}
	}

	return &ScaleUpConfig{
		CLIAIMonitorPath: cliaimonitorPath,
		DataDir:          "./data",
		Port:             8080,
		AutoScaleUp:      true,
	}
}

// MockScaleUpDetector for testing
type MockScaleUpDetector struct {
	ShouldScale     bool
	ScaleReason     string
	InfraLevel      InfraLevel
	IsAvailable     bool
	ScaleUpCalled   bool
	ScaleUpError    error
}

func NewMockScaleUpDetector() *MockScaleUpDetector {
	return &MockScaleUpDetector{
		ShouldScale: false,
		InfraLevel:  InfraLightweight,
		IsAvailable: true,
	}
}

func (m *MockScaleUpDetector) ShouldScaleUp(state *PortableState) (bool, string) {
	return m.ShouldScale, m.ScaleReason
}

func (m *MockScaleUpDetector) ScaleUp(ctx context.Context) error {
	m.ScaleUpCalled = true
	if m.ScaleUpError != nil {
		return m.ScaleUpError
	}
	m.InfraLevel = InfraLocal
	return nil
}

func (m *MockScaleUpDetector) GetInfraLevel() InfraLevel {
	return m.InfraLevel
}

func (m *MockScaleUpDetector) IsCLIAIMonitorAvailable() bool {
	return m.IsAvailable
}
