package instance

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"golang.org/x/sys/windows"
)

// InstanceManager handles lifecycle management for CLIAIMONITOR instances
type InstanceManager struct {
	pidFilePath  string
	statePath    string
	port         int
	lockHandle   windows.Handle
	acquiredLock bool
}

// InstanceInfo contains information about a running instance
type InstanceInfo struct {
	PID          int
	Port         int
	StartTime    time.Time
	IsRunning    bool
	IsResponding bool
	Version      string
	BasePath     string
}

// PIDFileData represents the JSON structure of the PID file
type PIDFileData struct {
	PID       int       `json:"pid"`
	Port      int       `json:"port"`
	StartedAt time.Time `json:"started_at"`
	Version   string    `json:"version"`
	BasePath  string    `json:"base_path"`
	Hostname  string    `json:"hostname"`
}

// NewManager creates a new instance manager
func NewManager(pidFilePath, statePath string, port int) *InstanceManager {
	return &InstanceManager{
		pidFilePath:  pidFilePath,
		statePath:    statePath,
		port:         port,
		acquiredLock: false,
	}
}

// CheckExistingInstance checks if a CLIAIMONITOR instance is already running
func (m *InstanceManager) CheckExistingInstance() (*InstanceInfo, error) {
	// Try to read PID file
	pidData, err := m.ReadPIDFile()
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No existing instance
		}
		return nil, fmt.Errorf("failed to read PID file: %w", err)
	}

	// Check if process is actually running
	running, err := IsProcessRunning(pidData.PID)
	if err != nil {
		return nil, fmt.Errorf("failed to check process: %w", err)
	}

	if !running {
		// Stale PID file - remove it
		fmt.Printf("Detected stale PID file (process %d not running)\n", pidData.PID)
		m.RemovePIDFile()
		return nil, nil
	}

	// Verify process name matches cliaimonitor.exe
	name, err := GetProcessName(pidData.PID)
	if err != nil {
		fmt.Printf("Warning: Failed to get process name for PID %d: %v\n", pidData.PID, err)
	} else if name != "cliaimonitor.exe" {
		// PID reused by different process
		fmt.Printf("Detected PID reuse (process %d is %s, not cliaimonitor.exe)\n", pidData.PID, name)
		m.RemovePIDFile()
		return nil, nil
	}

	// Check if responding via health endpoint
	responding := HealthCheck(pidData.Port) == nil

	return &InstanceInfo{
		PID:          pidData.PID,
		Port:         pidData.Port,
		StartTime:    pidData.StartedAt,
		IsRunning:    true,
		IsResponding: responding,
		Version:      pidData.Version,
		BasePath:     pidData.BasePath,
	}, nil
}

// WritePIDFile creates a PID file with instance information
func (m *InstanceManager) WritePIDFile(pid, port int, basePath string) error {
	hostname, _ := os.Hostname()

	data := PIDFileData{
		PID:       pid,
		Port:      port,
		StartedAt: time.Now(),
		Version:   "1.0.0",
		BasePath:  basePath,
		Hostname:  hostname,
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal PID data: %w", err)
	}

	if err := os.WriteFile(m.pidFilePath, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}

	return nil
}

// ReadPIDFile reads and parses the PID file
func (m *InstanceManager) ReadPIDFile() (*PIDFileData, error) {
	jsonData, err := os.ReadFile(m.pidFilePath)
	if err != nil {
		return nil, err
	}

	var data PIDFileData
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return nil, fmt.Errorf("failed to parse PID file: %w", err)
	}

	return &data, nil
}

// RemovePIDFile deletes the PID file
func (m *InstanceManager) RemovePIDFile() error {
	if err := os.Remove(m.pidFilePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove PID file: %w", err)
	}
	return nil
}

// GetPort returns the port the instance manager is configured for
func (m *InstanceManager) GetPort() int {
	return m.port
}

// SetPort updates the port (used when resolver chooses different port)
func (m *InstanceManager) SetPort(port int) {
	m.port = port
}
