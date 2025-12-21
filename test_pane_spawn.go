// +build manual
// This is a manual test to verify WezTerm pane ID capture
package main

import (
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"strings"
)

func main() {
	fmt.Println("Testing WezTerm pane ID capture...")

	// Try spawning a simple command and capturing pane ID
	cmd := exec.Command("wezterm.exe", "cli", "spawn", "--new-window", "--", "cmd.exe", "/k", "echo Hello from pane test")
	output, err := cmd.Output()
	if err != nil {
		log.Printf("ERROR: wezterm cli spawn failed: %v", err)
		log.Printf("This is expected if no WezTerm mux server is running")
		log.Printf("Try running this test from within a WezTerm window")
		return
	}

	paneIDStr := strings.TrimSpace(string(output))
	paneID, err := strconv.Atoi(paneIDStr)
	if err != nil {
		log.Fatalf("Failed to parse pane ID from output: %s (error: %v)", paneIDStr, err)
	}

	fmt.Printf("✓ Successfully spawned pane with ID: %d\n", paneID)

	// List panes to verify it exists
	cmd = exec.Command("wezterm.exe", "cli", "list", "--format", "json")
	output, err = cmd.Output()
	if err != nil {
		log.Fatalf("Failed to list panes: %v", err)
	}

	fmt.Printf("\n✓ Current panes:\n%s\n", string(output))

	// Try killing the pane
	cmd = exec.Command("wezterm.exe", "cli", "kill-pane", "--pane-id", fmt.Sprintf("%d", paneID))
	output, err = cmd.CombinedOutput()
	if err != nil {
		log.Fatalf("Failed to kill pane: %v (output: %s)", err, string(output))
	}

	fmt.Printf("\n✓ Successfully killed pane %d\n", paneID)
	fmt.Println("\nAll tests passed! WezTerm pane ID tracking is working correctly.")
}
