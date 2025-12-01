package instance

import (
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// IsPortAvailable checks if a TCP port is available for binding
func IsPortAvailable(port int) bool {
	address := fmt.Sprintf(":%d", port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return false
	}
	listener.Close()
	return true
}

// GetProcessUsingPort attempts to find which process is using a given port
// Returns PID of the process, or 0 if not found
func GetProcessUsingPort(port int) (int, error) {
	// Use netstat to find the process
	cmd := exec.Command("cmd", "/C", fmt.Sprintf("netstat -ano | findstr :%d | findstr LISTENING", port))
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("netstat command failed: %w", err)
	}

	outputStr := strings.TrimSpace(string(output))
	if outputStr == "" {
		return 0, fmt.Errorf("no process found listening on port %d", port)
	}

	// Parse netstat output
	// Format: "  TCP    0.0.0.0:3000    0.0.0.0:0    LISTENING       11316"
	// or:     "  TCP    [::]:3000      [::]:0       LISTENING       11316"
	lines := strings.Split(outputStr, "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}

		// The PID is the last field
		pidStr := fields[len(fields)-1]
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			continue
		}

		return pid, nil
	}

	return 0, fmt.Errorf("could not parse PID from netstat output")
}

// FindAvailablePort finds the next available port starting from startPort
// Returns the first available port found, or 0 if none available within maxAttempts
func FindAvailablePort(startPort int) int {
	maxAttempts := 20
	for i := 0; i < maxAttempts; i++ {
		port := startPort + i
		if IsPortAvailable(port) {
			return port
		}
	}
	return 0
}

// HealthCheck performs an HTTP GET request to the health endpoint
// Returns nil if the server is responding, error otherwise
func HealthCheck(port int) error {
	url := fmt.Sprintf("http://localhost:%d/api/health", port)
	client := &http.Client{
		Timeout: 2 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check returned status %d", resp.StatusCode)
	}

	return nil
}

// SendShutdownRequest sends a graceful shutdown request to a running instance
func SendShutdownRequest(port int) error {
	url := fmt.Sprintf("http://localhost:%d/api/shutdown", port)
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Post(url, "application/json", nil)
	if err != nil {
		return fmt.Errorf("shutdown request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("shutdown request returned status %d", resp.StatusCode)
	}

	return nil
}

// WaitForPortToBeAvailable polls the port until it becomes available or timeout
func WaitForPortToBeAvailable(port int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if IsPortAvailable(port) {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}
