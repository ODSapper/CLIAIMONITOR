package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// CLI provides bootstrap command-line operations
type CLI struct {
	stateManager StateManager
	phoneHome    PhoneHomeClient
	scaleUp      ScaleUpDetector
	statePath    string
}

// NewCLI creates a new bootstrap CLI
func NewCLI(statePath string) (*CLI, error) {
	return &CLI{
		stateManager: NewStateManager(),
		statePath:    statePath,
	}, nil
}

// InitCommand initializes a new bootstrap environment
func (c *CLI) InitCommand(envName, envType, envID, captainID string, enablePhoneHome bool, phoneHomeURL string) error {
	// Generate IDs if not provided
	if envID == "" {
		envID = fmt.Sprintf("env-%d", time.Now().Unix())
	}
	if captainID == "" {
		captainID = fmt.Sprintf("captain-%d", time.Now().Unix())
	}

	// Create new state
	state := NewPortableState(envID, envName, envType, captainID)

	if enablePhoneHome {
		state.PhoneHome.Enabled = true
		if phoneHomeURL != "" {
			state.PhoneHome.Endpoint = phoneHomeURL
		}
	}

	// Save state
	if err := c.stateManager.SaveState(state, c.statePath); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	fmt.Printf("Bootstrap initialized successfully\n")
	fmt.Printf("  Environment: %s (%s)\n", envName, envType)
	fmt.Printf("  Environment ID: %s\n", envID)
	fmt.Printf("  Captain ID: %s\n", captainID)
	fmt.Printf("  State file: %s\n", c.statePath)
	fmt.Printf("  Mode: %s\n", state.Mode)
	if enablePhoneHome {
		fmt.Printf("  Phone Home: ENABLED (%s)\n", state.PhoneHome.Endpoint)
	} else {
		fmt.Printf("  Phone Home: DISABLED\n")
	}

	return nil
}

// StatusCommand displays current bootstrap status
func (c *CLI) StatusCommand() error {
	state, err := c.stateManager.LoadState(c.statePath)
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	fmt.Printf("CLIAIMONITOR Bootstrap Status\n")
	fmt.Printf("================================\n\n")

	fmt.Printf("Environment:\n")
	fmt.Printf("  ID:           %s\n", state.Environment.ID)
	fmt.Printf("  Name:         %s\n", state.Environment.Name)
	fmt.Printf("  Type:         %s\n", state.Environment.Type)
	fmt.Printf("  First Contact: %s\n", state.Environment.FirstContact.Format(time.RFC3339))
	fmt.Printf("\n")

	fmt.Printf("Captain:\n")
	fmt.Printf("  ID:           %s\n", state.CaptainID)
	fmt.Printf("  Mode:         %s\n", state.Mode)
	fmt.Printf("\n")

	fmt.Printf("Findings Summary:\n")
	fmt.Printf("  Critical:     %d\n", state.FindingsSummary.Critical)
	fmt.Printf("  High:         %d\n", state.FindingsSummary.High)
	fmt.Printf("  Medium:       %d\n", state.FindingsSummary.Medium)
	fmt.Printf("  Low:          %d\n", state.FindingsSummary.Low)
	total := state.FindingsSummary.Critical + state.FindingsSummary.High +
		state.FindingsSummary.Medium + state.FindingsSummary.Low
	fmt.Printf("  Total:        %d\n", total)
	fmt.Printf("\n")

	fmt.Printf("Active Agents:  %d\n", len(state.ActiveAgents))
	if len(state.ActiveAgents) > 0 {
		for _, agent := range state.ActiveAgents {
			fmt.Printf("  - %s\n", agent)
		}
	}
	fmt.Printf("\n")

	fmt.Printf("Pending Decisions: %d\n", len(state.PendingDecisions))
	fmt.Printf("\n")

	fmt.Printf("Phone Home:\n")
	fmt.Printf("  Enabled:      %t\n", state.PhoneHome.Enabled)
	if state.PhoneHome.Enabled {
		fmt.Printf("  Endpoint:     %s\n", state.PhoneHome.Endpoint)
		if state.PhoneHome.LastSync != nil {
			fmt.Printf("  Last Sync:    %s\n", state.PhoneHome.LastSync.Format(time.RFC3339))
		} else {
			fmt.Printf("  Last Sync:    Never\n")
		}
	}
	fmt.Printf("\n")

	fmt.Printf("Scale-Up:\n")
	fmt.Printf("  Triggered:    %t\n", state.ScaleUp.Triggered)
	if state.ScaleUp.Triggered {
		if state.ScaleUp.Reason != nil {
			fmt.Printf("  Reason:       %s\n", *state.ScaleUp.Reason)
		}
		if state.ScaleUp.CLIAIMonitorPort != nil {
			fmt.Printf("  Port:         %d\n", *state.ScaleUp.CLIAIMonitorPort)
		}
	}

	return nil
}

// PhoneHomeCommand sends a report to Magnolia HQ
func (c *CLI) PhoneHomeCommand(apiKeyEnv string) error {
	// Load state
	state, err := c.stateManager.LoadState(c.statePath)
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	if !state.PhoneHome.Enabled {
		return fmt.Errorf("phone home is not enabled in state file")
	}

	// Create phone home client
	if apiKeyEnv == "" {
		apiKeyEnv = state.PhoneHome.APIKeyEnv
	}

	client, err := NewPhoneHomeClient(state.PhoneHome.Endpoint, apiKeyEnv, state.CaptainID)
	if err != nil {
		return fmt.Errorf("failed to create phone home client: %w", err)
	}

	// Export state to report format
	report, err := c.stateManager.ExportForSync(state)
	if err != nil {
		return fmt.Errorf("failed to export state: %w", err)
	}

	// Send report
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fmt.Printf("Sending report to Magnolia HQ...\n")
	if err := client.SendReport(ctx, report); err != nil {
		return fmt.Errorf("failed to send report: %w", err)
	}

	fmt.Printf("Report sent successfully\n")

	// Update last sync time
	now := time.Now()
	state.PhoneHome.LastSync = &now
	if err := c.stateManager.SaveState(state, c.statePath); err != nil {
		fmt.Printf("Warning: Failed to update last sync time: %v\n", err)
	}

	// Get instructions
	fmt.Printf("\nChecking for instructions from HQ...\n")
	instructions, err := client.GetInstructions(ctx)
	if err != nil {
		fmt.Printf("Warning: Failed to get instructions: %v\n", err)
		return nil
	}

	if instructions.AbortMission {
		fmt.Printf("\n*** ABORT MISSION ***\n")
		fmt.Printf("Reason: %s\n", instructions.AbortReason)
		return nil
	}

	if len(instructions.Tasks) > 0 {
		fmt.Printf("\nNew tasks from HQ:\n")
		for _, task := range instructions.Tasks {
			fmt.Printf("  [%s] %s - %s\n", task.Type, task.ID, task.Description)
		}
	} else {
		fmt.Printf("No new tasks from HQ\n")
	}

	return nil
}

// ScaleUpCommand triggers infrastructure scale-up
func (c *CLI) ScaleUpCommand(cliaimonitorPath, dataDir string, port int, force bool) error {
	// Load state
	state, err := c.stateManager.LoadState(c.statePath)
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	if state.ScaleUp.Triggered && !force {
		return fmt.Errorf("scale-up already triggered (use --force to override)")
	}

	// Create scale-up detector
	detector := NewScaleUpDetector(cliaimonitorPath, dataDir, port)

	// Check if CLIAIMONITOR is available
	if !detector.IsCLIAIMonitorAvailable() {
		return fmt.Errorf("CLIAIMONITOR binary not found at: %s", cliaimonitorPath)
	}

	// Check if scale-up is needed (unless forced)
	if !force {
		shouldScale, reason := detector.ShouldScaleUp(state)
		if !shouldScale {
			fmt.Printf("Scale-up not currently needed\n")
			fmt.Printf("Current state:\n")
			fmt.Printf("  Active agents: %d\n", len(state.ActiveAgents))
			fmt.Printf("  Days since first contact: %.1f\n", time.Since(state.Environment.FirstContact).Hours()/24)
			fmt.Printf("  Critical findings: %d\n", state.FindingsSummary.Critical)
			return nil
		}
		fmt.Printf("Scale-up triggered: %s\n", reason)
	} else {
		fmt.Printf("Forcing scale-up...\n")
	}

	// Perform scale-up
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := detector.ScaleUp(ctx); err != nil {
		return fmt.Errorf("scale-up failed: %w", err)
	}

	// Update state
	state.ScaleUp.Triggered = true
	reason := "Manual scale-up"
	state.ScaleUp.Reason = &reason
	state.ScaleUp.CLIAIMonitorPort = &port
	state.Mode = "local"

	if err := c.stateManager.SaveState(state, c.statePath); err != nil {
		fmt.Printf("Warning: Failed to save state: %v\n", err)
	}

	fmt.Printf("\nScale-up successful!\n")
	fmt.Printf("  CLIAIMONITOR started on port %d\n", port)
	fmt.Printf("  Mode changed to: %s\n", state.Mode)
	fmt.Printf("\nAccess dashboard at: http://localhost:%d\n", port)

	return nil
}

// ExportCommand exports state as JSON to stdout or file
func (c *CLI) ExportCommand(outputPath string) error {
	state, err := c.stateManager.LoadState(c.statePath)
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	if outputPath == "" || outputPath == "-" {
		// Output to stdout
		fmt.Println(string(data))
	} else {
		// Output to file
		if err := os.WriteFile(outputPath, data, 0644); err != nil {
			return fmt.Errorf("failed to write file: %w", err)
		}
		fmt.Printf("State exported to: %s\n", outputPath)
	}

	return nil
}

// ImportCommand imports state from JSON file
func (c *CLI) ImportCommand(inputPath string) error {
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	state, err := c.stateManager.ImportFromHQ(data)
	if err != nil {
		return fmt.Errorf("failed to import state: %w", err)
	}

	if err := c.stateManager.SaveState(state, c.statePath); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	fmt.Printf("State imported successfully from: %s\n", inputPath)
	fmt.Printf("  Environment: %s\n", state.Environment.Name)
	fmt.Printf("  Captain ID: %s\n", state.CaptainID)

	return nil
}
