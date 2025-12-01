//go:build windows
// +build windows

package instance

import (
	"fmt"
	"os"
	"syscall"

	"golang.org/x/sys/windows"
)

// AcquireLock acquires an exclusive lock to prevent multiple instances from starting
func (m *InstanceManager) AcquireLock() error {
	lockPath := m.pidFilePath + ".lock"

	// Convert path to UTF-16 for Windows API
	lockPathPtr, err := syscall.UTF16PtrFromString(lockPath)
	if err != nil {
		return fmt.Errorf("failed to convert lock path: %w", err)
	}

	// Create file with exclusive access (no sharing)
	// This prevents any other process from opening the same file
	handle, err := windows.CreateFile(
		lockPathPtr,
		windows.GENERIC_READ|windows.GENERIC_WRITE,
		0, // dwShareMode = 0 means exclusive access
		nil,
		windows.CREATE_ALWAYS,
		windows.FILE_ATTRIBUTE_NORMAL,
		0,
	)

	if err != nil {
		return fmt.Errorf("failed to acquire lock (another instance may be starting): %w", err)
	}

	m.lockHandle = handle
	m.acquiredLock = true

	// Write current PID to lock file for debugging
	pidStr := fmt.Sprintf("%d", os.Getpid())
	pidBytes := []byte(pidStr)
	var bytesWritten uint32
	err = windows.WriteFile(handle, pidBytes, &bytesWritten, nil)
	if err != nil {
		// Non-fatal - lock is still acquired
		fmt.Printf("Warning: Failed to write PID to lock file: %v\n", err)
	}

	return nil
}

// ReleaseLock releases the exclusive lock
func (m *InstanceManager) ReleaseLock() error {
	if !m.acquiredLock {
		return nil
	}

	// Close the handle
	if m.lockHandle != 0 {
		err := windows.CloseHandle(m.lockHandle)
		if err != nil {
			fmt.Printf("Warning: Failed to close lock handle: %v\n", err)
		}
		m.lockHandle = 0
	}

	// Remove the lock file
	lockPath := m.pidFilePath + ".lock"
	if err := os.Remove(lockPath); err != nil && !os.IsNotExist(err) {
		fmt.Printf("Warning: Failed to remove lock file: %v\n", err)
	}

	m.acquiredLock = false
	return nil
}
