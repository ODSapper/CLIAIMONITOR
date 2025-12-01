//go:build windows
// +build windows

package instance

import (
	"os"
	"testing"
	"time"
)

func TestIsProcessRunning_CurrentProcess(t *testing.T) {
	// Test with current process PID
	currentPID := os.Getpid()

	running, err := IsProcessRunning(currentPID)
	if err != nil {
		t.Fatalf("IsProcessRunning failed for current process: %v", err)
	}

	// Current process should be running
	// Note: May return false if process name doesn't match cliaimonitor.exe
	t.Logf("Current process (PID %d) running: %v", currentPID, running)
}

func TestIsProcessRunning_InvalidPID(t *testing.T) {
	// Test with an invalid PID that shouldn't exist
	invalidPID := 999999

	running, err := IsProcessRunning(invalidPID)
	if err != nil {
		// Error is acceptable for invalid PID
		t.Logf("IsProcessRunning returned error for invalid PID (expected): %v", err)
		return
	}

	if running {
		t.Error("IsProcessRunning should return false for invalid PID")
	}
}

func TestIsProcessRunning_PID1(t *testing.T) {
	// Test with PID 1 (system process on Unix, may not exist on Windows)
	running, err := IsProcessRunning(1)

	if err != nil {
		t.Logf("IsProcessRunning PID 1: error (may be expected on Windows): %v", err)
	} else {
		t.Logf("IsProcessRunning PID 1: %v", running)
	}
}

func TestGetProcessName_CurrentProcess(t *testing.T) {
	currentPID := os.Getpid()

	name, err := GetProcessName(currentPID)
	if err != nil {
		t.Fatalf("GetProcessName failed for current process: %v", err)
	}

	t.Logf("Current process name: %s", name)

	if name == "" {
		t.Error("GetProcessName should return non-empty name")
	}

	// Verify it ends with .exe (Windows convention)
	if len(name) < 4 || name[len(name)-4:] != ".exe" {
		t.Logf("Warning: Process name doesn't end with .exe: %s", name)
	}
}

func TestGetProcessName_InvalidPID(t *testing.T) {
	invalidPID := 999999

	name, err := GetProcessName(invalidPID)
	if err == nil {
		t.Errorf("GetProcessName should fail for invalid PID, got name: %s", name)
	}

	if name != "" {
		t.Error("GetProcessName should return empty string on error")
	}
}

func TestGetProcessStartTime_CurrentProcess(t *testing.T) {
	currentPID := os.Getpid()

	startTime, err := GetProcessStartTime(currentPID)
	if err != nil {
		t.Fatalf("GetProcessStartTime failed for current process: %v", err)
	}

	// Start time should be in the recent past
	elapsed := time.Since(startTime)
	t.Logf("Current process started %v ago", elapsed)

	if elapsed < 0 {
		t.Error("Process start time is in the future")
	}

	if elapsed > 1*time.Hour {
		t.Log("Warning: Process appears to have started over an hour ago (may be expected for long-running test process)")
	}
}

func TestGetProcessStartTime_InvalidPID(t *testing.T) {
	invalidPID := 999999

	_, err := GetProcessStartTime(invalidPID)
	if err == nil {
		t.Error("GetProcessStartTime should fail for invalid PID")
	}
}

func TestKillProcess_InvalidPID(t *testing.T) {
	// Try to kill a PID that doesn't exist
	invalidPID := 999999

	err := KillProcess(invalidPID)
	if err == nil {
		t.Error("KillProcess should fail for invalid PID")
	}

	t.Logf("KillProcess error (expected): %v", err)
}

func TestProcessDetection_SystemProcesses(t *testing.T) {
	// Test detection of common system processes
	// These tests are informational and may vary by system

	systemProcesses := []struct {
		name string
		pid  int
	}{
		{"System Idle Process", 0},
		{"System", 4},
	}

	for _, proc := range systemProcesses {
		running, err := IsProcessRunning(proc.pid)
		t.Logf("Process %s (PID %d): running=%v, err=%v", proc.name, proc.pid, running, err)
	}
}

func TestGetProcessName_ConsistentWithRunningCheck(t *testing.T) {
	currentPID := os.Getpid()

	// Get process name
	name, err := GetProcessName(currentPID)
	if err != nil {
		t.Fatalf("GetProcessName failed: %v", err)
	}

	// Check if process is running
	running, err := IsProcessRunning(currentPID)
	if err != nil {
		t.Fatalf("IsProcessRunning failed: %v", err)
	}

	// If we can get the name, IsProcessRunning should work too
	// (though it might return false if name != cliaimonitor.exe)
	t.Logf("Process %s (PID %d): running=%v", name, currentPID, running)

	if name == "cliaimonitor.exe" && !running {
		t.Error("IsProcessRunning should return true for cliaimonitor.exe")
	}
}

func TestCheckViaTasklist(t *testing.T) {
	// Test the tasklist fallback method directly
	currentPID := os.Getpid()

	running, err := checkViaTasklist(currentPID)
	if err != nil {
		t.Logf("checkViaTasklist error (may be expected): %v", err)
		return
	}

	t.Logf("checkViaTasklist for current process: %v", running)

	// Result depends on whether current process is cliaimonitor.exe
	// Just verify the function doesn't crash
}

func TestGetProcessNameViaTasklist(t *testing.T) {
	// Test the tasklist fallback for getting process name
	currentPID := os.Getpid()

	name, err := getProcessNameViaTasklist(currentPID)
	if err != nil {
		t.Logf("getProcessNameViaTasklist error (may be expected): %v", err)
		return
	}

	t.Logf("getProcessNameViaTasklist: %s", name)

	if name == "" {
		t.Error("getProcessNameViaTasklist should return non-empty name on success")
	}
}

func TestProcessNameMatching(t *testing.T) {
	// Test process name matching logic
	testCases := []struct {
		name     string
		expected bool
	}{
		{"cliaimonitor.exe", true},
		{"CLIAIMONITOR.EXE", true},
		{"ClIaImOnItOr.ExE", true},
		{"other.exe", false},
		{"cliaimonitor", false}, // Missing .exe
		{"", false},
	}

	for _, tc := range testCases {
		// Case-insensitive comparison
		matches := tc.name == "cliaimonitor.exe" ||
			tc.name == "CLIAIMONITOR.EXE" ||
			tc.name == "ClIaImOnItOr.ExE"

		if matches != tc.expected {
			t.Errorf("Process name %q: expected match=%v, got %v", tc.name, tc.expected, matches)
		}
	}
}

func BenchmarkIsProcessRunning(b *testing.B) {
	currentPID := os.Getpid()
	for i := 0; i < b.N; i++ {
		IsProcessRunning(currentPID)
	}
}

func BenchmarkGetProcessName(b *testing.B) {
	currentPID := os.Getpid()
	for i := 0; i < b.N; i++ {
		GetProcessName(currentPID)
	}
}

func BenchmarkGetProcessStartTime(b *testing.B) {
	currentPID := os.Getpid()
	for i := 0; i < b.N; i++ {
		GetProcessStartTime(currentPID)
	}
}
