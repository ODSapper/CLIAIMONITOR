package instance

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// ConflictResolver handles conflicts when an instance is already running
type ConflictResolver struct {
	instanceMgr *InstanceManager
	interactive bool
}

// NewConflictResolver creates a new conflict resolver
func NewConflictResolver(instanceMgr *InstanceManager, interactive bool) *ConflictResolver {
	return &ConflictResolver{
		instanceMgr: instanceMgr,
		interactive: interactive,
	}
}

// Resolve handles the conflict resolution process
// May exit the process (for connect/exit options)
// Returns error if resolution fails, nil if resolved successfully
func (r *ConflictResolver) Resolve(info *InstanceInfo) error {
	if !r.interactive {
		return r.handleNonInteractive(info)
	}
	return r.handleInteractive(info)
}

// handleInteractive presents the user with options and processes their choice
func (r *ConflictResolver) handleInteractive(info *InstanceInfo) error {
	r.displayConflictInfo(info)

	reader := bufio.NewReader(os.Stdin)

	for {
		choice, err := r.promptUser(reader)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading input: %v\n", err)
			continue
		}

		switch choice {
		case 1:
			// Connect to existing
			return r.connectToExisting(info)
		case 2:
			// Stop existing gracefully
			return r.stopExisting(info, false)
		case 3:
			// Use different port
			return r.useDifferentPort(info)
		case 4:
			// Force kill
			return r.stopExisting(info, true)
		case 5:
			// Exit
			fmt.Println("\nCanceling startup.")
			os.Exit(0)
		default:
			fmt.Println("Invalid choice. Please enter 1-5.")
		}
	}
}

// handleNonInteractive handles conflict resolution for non-interactive environments
func (r *ConflictResolver) handleNonInteractive(info *InstanceInfo) error {
	strategy := os.Getenv("CLIAIMONITOR_ON_CONFLICT")
	if strategy == "" {
		strategy = "exit" // Safe default
	}

	fmt.Printf("Port %d is in use (PID %d). Conflict strategy: %s\n", info.Port, info.PID, strategy)

	switch strategy {
	case "exit":
		fmt.Fprintf(os.Stderr, "Another instance is running on port %d (PID %d)\n", info.Port, info.PID)
		fmt.Fprintf(os.Stderr, "Set CLIAIMONITOR_ON_CONFLICT to 'kill', 'port', or 'connect' to change behavior\n")
		os.Exit(1)
		return nil
	case "kill":
		return r.stopExisting(info, true)
	case "port":
		return r.useDifferentPort(info)
	case "connect":
		return r.connectToExisting(info)
	default:
		return fmt.Errorf("unknown conflict strategy: %s", strategy)
	}
}

// displayConflictInfo shows formatted conflict information to the user
func (r *ConflictResolver) displayConflictInfo(info *InstanceInfo) {
	fmt.Println()
	fmt.Println("╔═══════════════════════════════════════════════════════════════════╗")
	fmt.Println("║  ERROR: Cannot start CLIAIMONITOR                                  ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Printf("Another instance is already running on port %d:\n\n", info.Port)
	fmt.Printf("  PID:         %d\n", info.PID)
	fmt.Printf("  Port:        %d\n", info.Port)
	fmt.Printf("  Started:     %s (%s ago)\n",
		info.StartTime.Format("2006-01-02 15:04:05"),
		time.Since(info.StartTime).Round(time.Second))

	status := "Not responding"
	if info.IsResponding {
		status = "✓ Running and responding"
	}
	fmt.Printf("  Status:      %s\n", status)
	fmt.Printf("  Dashboard:   http://localhost:%d\n", info.Port)
	fmt.Println()

	fmt.Println("┌─────────────────────────────────────────────────────────────────┐")
	fmt.Println("│  What would you like to do?                                      │")
	fmt.Println("└─────────────────────────────────────────────────────────────────┘")
	fmt.Println()
	fmt.Println("  1. Connect to existing instance")
	fmt.Printf("     → Opens http://localhost:%d in your browser\n", info.Port)
	fmt.Println()
	fmt.Println("  2. Stop existing instance and start new one")
	fmt.Println("     → Gracefully shuts down previous instance")
	fmt.Println()
	fmt.Println("  3. Start on a different port")
	fmt.Println("     → Automatically finds next available port")
	fmt.Println()
	fmt.Println("  4. Force kill existing instance")
	fmt.Println("     → Terminates process immediately (use if unresponsive)")
	fmt.Println()
	fmt.Println("  5. Exit")
	fmt.Println("     → Cancel startup")
	fmt.Println()
}

// promptUser reads and validates user input
func (r *ConflictResolver) promptUser(reader *bufio.Reader) (int, error) {
	fmt.Print("Enter choice (1-5): ")
	input, err := reader.ReadString('\n')
	if err != nil {
		return 0, err
	}

	input = strings.TrimSpace(input)
	choice, err := strconv.Atoi(input)
	if err != nil {
		return 0, fmt.Errorf("invalid input")
	}

	return choice, nil
}

// connectToExisting opens the existing instance's dashboard in the browser and exits
func (r *ConflictResolver) connectToExisting(info *InstanceInfo) error {
	url := fmt.Sprintf("http://localhost:%d", info.Port)
	fmt.Printf("\nConnecting to existing instance at %s\n", url)

	// Open browser (Windows)
	cmd := exec.Command("cmd", "/C", "start", url)
	err := cmd.Start()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to open browser: %v\n", err)
		fmt.Printf("Please open %s manually\n", url)
	}

	fmt.Println("Exiting...")
	os.Exit(0)
	return nil
}

// stopExisting attempts to stop the existing instance
func (r *ConflictResolver) stopExisting(info *InstanceInfo, force bool) error {
	if !force && info.IsResponding {
		// Try graceful shutdown first
		fmt.Println("\nSending graceful shutdown request...")
		err := SendShutdownRequest(info.Port)
		if err != nil {
			fmt.Printf("Graceful shutdown failed: %v\n", err)
			fmt.Println("Attempting force kill...")
			force = true
		} else {
			// Wait for process to exit
			fmt.Println("Waiting for graceful shutdown...")
			time.Sleep(3 * time.Second)

			// Check if process stopped
			running, _ := IsProcessRunning(info.PID)
			if !running {
				fmt.Println("Previous instance stopped successfully ✓")
				r.instanceMgr.RemovePIDFile()
				return nil
			}

			fmt.Println("Process still running after graceful shutdown request")
			fmt.Println("Attempting force kill...")
			force = true
		}
	}

	if force {
		// Force kill
		fmt.Printf("Force killing process %d...\n", info.PID)
		err := KillProcess(info.PID)
		if err != nil {
			return fmt.Errorf("failed to kill process: %w", err)
		}

		// Wait a moment for process to fully terminate
		time.Sleep(1 * time.Second)

		// Clean up PID file
		r.instanceMgr.RemovePIDFile()

		fmt.Println("Previous instance terminated ✓")
	}

	return nil
}

// useDifferentPort finds an available port and continues startup
func (r *ConflictResolver) useDifferentPort(info *InstanceInfo) error {
	currentPort := r.instanceMgr.GetPort()
	newPort := FindAvailablePort(currentPort + 1)

	if newPort == 0 {
		return fmt.Errorf("could not find an available port")
	}

	fmt.Printf("\nStarting on port %d instead...\n", newPort)
	r.instanceMgr.SetPort(newPort)

	return nil
}

// IsInteractive checks if we're running in an interactive terminal
func IsInteractive() bool {
	// Check if stdin is a terminal
	fileInfo, err := os.Stdin.Stat()
	if err != nil {
		return false
	}

	// Check if it's a character device (terminal)
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}
