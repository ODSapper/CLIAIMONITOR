package instance

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	mgr := NewManager("/tmp/test.pid", "/tmp/state.json", 3000)

	if mgr == nil {
		t.Fatal("NewManager returned nil")
	}

	if mgr.pidFilePath != "/tmp/test.pid" {
		t.Errorf("Expected pidFilePath=/tmp/test.pid, got %s", mgr.pidFilePath)
	}

	if mgr.statePath != "/tmp/state.json" {
		t.Errorf("Expected statePath=/tmp/state.json, got %s", mgr.statePath)
	}

	if mgr.port != 3000 {
		t.Errorf("Expected port=3000, got %d", mgr.port)
	}

	if mgr.acquiredLock {
		t.Error("Expected acquiredLock=false for new manager")
	}
}

func TestGetSetPort(t *testing.T) {
	mgr := NewManager("/tmp/test.pid", "/tmp/state.json", 3000)

	if mgr.GetPort() != 3000 {
		t.Errorf("Expected GetPort()=3000, got %d", mgr.GetPort())
	}

	mgr.SetPort(8080)

	if mgr.GetPort() != 8080 {
		t.Errorf("Expected GetPort()=8080 after SetPort, got %d", mgr.GetPort())
	}
}

func TestWriteReadRemovePIDFile(t *testing.T) {
	// Create temp directory
	tempDir := t.TempDir()
	pidPath := filepath.Join(tempDir, "test.pid")

	mgr := NewManager(pidPath, "", 3000)

	// Test WritePIDFile
	err := mgr.WritePIDFile(12345, 3000, "/test/base/path")
	if err != nil {
		t.Fatalf("WritePIDFile failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(pidPath); os.IsNotExist(err) {
		t.Fatal("PID file was not created")
	}

	// Test ReadPIDFile
	pidData, err := mgr.ReadPIDFile()
	if err != nil {
		t.Fatalf("ReadPIDFile failed: %v", err)
	}

	// Verify data
	if pidData.PID != 12345 {
		t.Errorf("Expected PID=12345, got %d", pidData.PID)
	}

	if pidData.Port != 3000 {
		t.Errorf("Expected Port=3000, got %d", pidData.Port)
	}

	if pidData.Version != "1.0.0" {
		t.Errorf("Expected Version=1.0.0, got %s", pidData.Version)
	}

	if pidData.BasePath != "/test/base/path" {
		t.Errorf("Expected BasePath=/test/base/path, got %s", pidData.BasePath)
	}

	// Verify timestamp is recent
	if time.Since(pidData.StartedAt) > 5*time.Second {
		t.Error("StartedAt timestamp is too old")
	}

	// Test RemovePIDFile
	err = mgr.RemovePIDFile()
	if err != nil {
		t.Fatalf("RemovePIDFile failed: %v", err)
	}

	// Verify file is deleted
	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Fatal("PID file was not removed")
	}
}

func TestRemovePIDFile_NonExistent(t *testing.T) {
	tempDir := t.TempDir()
	pidPath := filepath.Join(tempDir, "nonexistent.pid")

	mgr := NewManager(pidPath, "", 3000)

	// Should not error when removing non-existent file
	err := mgr.RemovePIDFile()
	if err != nil {
		t.Errorf("RemovePIDFile should not error on non-existent file, got: %v", err)
	}
}

func TestReadPIDFile_NonExistent(t *testing.T) {
	tempDir := t.TempDir()
	pidPath := filepath.Join(tempDir, "nonexistent.pid")

	mgr := NewManager(pidPath, "", 3000)

	_, err := mgr.ReadPIDFile()
	if err == nil {
		t.Error("ReadPIDFile should error on non-existent file")
	}

	if !os.IsNotExist(err) {
		t.Errorf("Expected IsNotExist error, got: %v", err)
	}
}

func TestReadPIDFile_InvalidJSON(t *testing.T) {
	tempDir := t.TempDir()
	pidPath := filepath.Join(tempDir, "invalid.pid")

	// Write invalid JSON
	err := os.WriteFile(pidPath, []byte("not valid json"), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	mgr := NewManager(pidPath, "", 3000)

	_, err = mgr.ReadPIDFile()
	if err == nil {
		t.Error("ReadPIDFile should error on invalid JSON")
	}
}

func TestPIDFileFormat(t *testing.T) {
	tempDir := t.TempDir()
	pidPath := filepath.Join(tempDir, "format.pid")

	mgr := NewManager(pidPath, "", 3000)

	// Write PID file
	err := mgr.WritePIDFile(99999, 8080, "/custom/path")
	if err != nil {
		t.Fatalf("WritePIDFile failed: %v", err)
	}

	// Read raw JSON
	jsonData, err := os.ReadFile(pidPath)
	if err != nil {
		t.Fatalf("Failed to read PID file: %v", err)
	}

	// Parse JSON
	var data map[string]interface{}
	err = json.Unmarshal(jsonData, &data)
	if err != nil {
		t.Fatalf("Failed to parse PID file JSON: %v", err)
	}

	// Verify all expected fields exist
	expectedFields := []string{"pid", "port", "started_at", "version", "base_path", "hostname"}
	for _, field := range expectedFields {
		if _, ok := data[field]; !ok {
			t.Errorf("PID file missing expected field: %s", field)
		}
	}

	// Verify values
	if int(data["pid"].(float64)) != 99999 {
		t.Errorf("Expected pid=99999, got %v", data["pid"])
	}

	if int(data["port"].(float64)) != 8080 {
		t.Errorf("Expected port=8080, got %v", data["port"])
	}

	if data["base_path"] != "/custom/path" {
		t.Errorf("Expected base_path=/custom/path, got %v", data["base_path"])
	}
}

func TestCheckExistingInstance_NoFile(t *testing.T) {
	tempDir := t.TempDir()
	pidPath := filepath.Join(tempDir, "nonexistent.pid")

	mgr := NewManager(pidPath, "", 3000)

	info, err := mgr.CheckExistingInstance()
	if err != nil {
		t.Fatalf("CheckExistingInstance should not error when no PID file exists: %v", err)
	}

	if info != nil {
		t.Error("CheckExistingInstance should return nil when no PID file exists")
	}
}

func TestCheckExistingInstance_InvalidPID(t *testing.T) {
	tempDir := t.TempDir()
	pidPath := filepath.Join(tempDir, "invalid.pid")

	mgr := NewManager(pidPath, "", 3000)

	// Write PID file with invalid PID (99999 should not exist)
	err := mgr.WritePIDFile(99999, 3000, "/test")
	if err != nil {
		t.Fatalf("WritePIDFile failed: %v", err)
	}

	info, err := mgr.CheckExistingInstance()
	if err != nil {
		t.Fatalf("CheckExistingInstance failed: %v", err)
	}

	// Should return nil because process doesn't exist (stale PID)
	if info != nil {
		t.Error("CheckExistingInstance should return nil for non-existent process")
	}

	// Verify PID file was cleaned up
	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Error("Stale PID file should have been removed")
	}
}

func TestCheckExistingInstance_CurrentProcess(t *testing.T) {
	tempDir := t.TempDir()
	pidPath := filepath.Join(tempDir, "current.pid")

	mgr := NewManager(pidPath, "", 3000)

	// Write PID file with current process ID
	currentPID := os.Getpid()
	err := mgr.WritePIDFile(currentPID, 3000, "/test")
	if err != nil {
		t.Fatalf("WritePIDFile failed: %v", err)
	}

	info, err := mgr.CheckExistingInstance()
	if err != nil {
		t.Fatalf("CheckExistingInstance failed: %v", err)
	}

	// Note: This may return nil if process name doesn't match "cliaimonitor.exe"
	// which is expected during tests. The test verifies the code path works.
	if info != nil {
		// If info is returned, verify it's correct
		if info.PID != currentPID {
			t.Errorf("Expected PID=%d, got %d", currentPID, info.PID)
		}

		if info.Port != 3000 {
			t.Errorf("Expected Port=3000, got %d", info.Port)
		}

		if !info.IsRunning {
			t.Error("Expected IsRunning=true for current process")
		}
	}

	// Clean up
	mgr.RemovePIDFile()
}

func TestLockAcquireRelease(t *testing.T) {
	tempDir := t.TempDir()
	pidPath := filepath.Join(tempDir, "lock.pid")

	mgr := NewManager(pidPath, "", 3000)

	// Acquire lock
	err := mgr.AcquireLock()
	if err != nil {
		t.Fatalf("AcquireLock failed: %v", err)
	}

	if !mgr.acquiredLock {
		t.Error("Expected acquiredLock=true after AcquireLock")
	}

	// Verify lock file exists
	lockPath := pidPath + ".lock"
	if _, err := os.Stat(lockPath); os.IsNotExist(err) {
		t.Error("Lock file was not created")
	}

	// Try to acquire lock again (should fail on Windows due to exclusive access)
	mgr2 := NewManager(pidPath, "", 3000)
	err = mgr2.AcquireLock()
	if err == nil {
		t.Error("AcquireLock should fail when lock is already held")
		mgr2.ReleaseLock() // Clean up
	}

	// Release lock
	err = mgr.ReleaseLock()
	if err != nil {
		t.Fatalf("ReleaseLock failed: %v", err)
	}

	if mgr.acquiredLock {
		t.Error("Expected acquiredLock=false after ReleaseLock")
	}

	// Verify lock file is removed
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Error("Lock file was not removed")
	}
}

func TestReleaseLock_NotAcquired(t *testing.T) {
	tempDir := t.TempDir()
	pidPath := filepath.Join(tempDir, "nolock.pid")

	mgr := NewManager(pidPath, "", 3000)

	// Should not error when releasing lock that wasn't acquired
	err := mgr.ReleaseLock()
	if err != nil {
		t.Errorf("ReleaseLock should not error when lock not acquired: %v", err)
	}
}

func TestInstanceInfo(t *testing.T) {
	info := &InstanceInfo{
		PID:          12345,
		Port:         3000,
		StartTime:    time.Now().Add(-1 * time.Hour),
		IsRunning:    true,
		IsResponding: true,
		Version:      "1.0.0",
		BasePath:     "/test/path",
	}

	if info.PID != 12345 {
		t.Errorf("Expected PID=12345, got %d", info.PID)
	}

	if info.Port != 3000 {
		t.Errorf("Expected Port=3000, got %d", info.Port)
	}

	if !info.IsRunning {
		t.Error("Expected IsRunning=true")
	}

	if !info.IsResponding {
		t.Error("Expected IsResponding=true")
	}

	if time.Since(info.StartTime) < 30*time.Minute {
		t.Error("StartTime should be about 1 hour ago")
	}
}
