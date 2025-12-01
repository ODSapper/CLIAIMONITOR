//go:build windows
// +build windows

package instance

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
)

// IsProcessRunning checks if a process with the given PID is running
// and verifies it's actually cliaimonitor.exe (not a PID reuse)
func IsProcessRunning(pid int) (bool, error) {
	// Try to open the process with limited query rights
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		// Process doesn't exist or we don't have permission
		// Try fallback method
		return checkViaTasklist(pid)
	}
	defer windows.CloseHandle(handle)

	// Process exists - now verify it's cliaimonitor.exe
	name, err := GetProcessName(pid)
	if err != nil {
		// Can't get name, assume it's running if we could open it
		return true, nil
	}

	// Check if it's cliaimonitor.exe
	return strings.EqualFold(name, "cliaimonitor.exe"), nil
}

// GetProcessName retrieves the executable name for a given PID
func GetProcessName(pid int) (string, error) {
	// Try Windows API first
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		// Fallback to tasklist
		return getProcessNameViaTasklist(pid)
	}
	defer windows.CloseHandle(handle)

	// Get process image name
	var exeNameBuf [windows.MAX_PATH]uint16
	exeNameLen := uint32(len(exeNameBuf))

	// QueryFullProcessImageName to get executable path
	err = windows.QueryFullProcessImageName(handle, 0, &exeNameBuf[0], &exeNameLen)
	if err != nil {
		return getProcessNameViaTasklist(pid)
	}

	// Convert to string and extract just the filename
	exePath := syscall.UTF16ToString(exeNameBuf[:exeNameLen])
	return filepath.Base(exePath), nil
}

// GetProcessStartTime retrieves the creation time of a process
func GetProcessStartTime(pid int) (time.Time, error) {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to open process: %w", err)
	}
	defer windows.CloseHandle(handle)

	var creationTime, exitTime, kernelTime, userTime windows.Filetime
	err = windows.GetProcessTimes(handle, &creationTime, &exitTime, &kernelTime, &userTime)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to get process times: %w", err)
	}

	// Convert FILETIME to time.Time
	return time.Unix(0, creationTime.Nanoseconds()), nil
}

// checkViaTasklist is a fallback method using tasklist command
func checkViaTasklist(pid int) (bool, error) {
	cmd := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/NH", "/FO", "CSV")
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("tasklist command failed: %w", err)
	}

	outputStr := string(output)
	// If output contains the PID and cliaimonitor.exe, process is running
	return strings.Contains(outputStr, fmt.Sprintf("%d", pid)) &&
		strings.Contains(strings.ToLower(outputStr), "cliaimonitor.exe"), nil
}

// getProcessNameViaTasklist gets process name using tasklist command
func getProcessNameViaTasklist(pid int) (string, error) {
	cmd := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/NH", "/FO", "CSV")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("tasklist command failed: %w", err)
	}

	outputStr := strings.TrimSpace(string(output))
	if outputStr == "" || strings.Contains(outputStr, "INFO: No tasks") {
		return "", fmt.Errorf("process not found")
	}

	// Parse CSV output: "imagename","pid","sessionname","session#","mem usage"
	// Example: "cliaimonitor.exe","12345","Console","1","25,000 K"
	parts := strings.Split(outputStr, ",")
	if len(parts) < 2 {
		return "", fmt.Errorf("unexpected tasklist output format")
	}

	// Remove quotes from image name
	imageName := strings.Trim(parts[0], "\"")
	return imageName, nil
}

// KillProcess forcefully terminates a process
func KillProcess(pid int) error {
	cmd := exec.Command("taskkill", "/F", "/PID", fmt.Sprintf("%d", pid))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to kill process %d: %w (output: %s)", pid, err, string(output))
	}
	return nil
}

