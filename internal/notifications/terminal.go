package notifications

import (
	"fmt"
	"os"
	"runtime"
	"sync"
)

// TerminalNotifier handles terminal title manipulation for notifications
type TerminalNotifier struct {
	originalTitle string
	mu            sync.Mutex
}

// NewTerminalNotifier creates a new terminal notifier
func NewTerminalNotifier() *TerminalNotifier {
	return &TerminalNotifier{
		originalTitle: "CLIAIMONITOR",
	}
}

// SetOriginalTitle stores the original terminal title for restoration
func (t *TerminalNotifier) SetOriginalTitle(title string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.originalTitle = title
}

// FlashTerminal changes the terminal title to show an alert
func (t *TerminalNotifier) FlashTerminal(message string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	alertTitle := fmt.Sprintf("🔔 CLIAIMONITOR - %s", message)
	return t.setTerminalTitle(alertTitle)
}

// NotifySupervisorNeedsInput changes terminal title to indicate supervisor needs input
func (t *TerminalNotifier) NotifySupervisorNeedsInput(message string) error {
	return t.FlashTerminal(fmt.Sprintf("Supervisor needs input: %s", message))
}

// RestoreTerminalTitle restores the original terminal title
func (t *TerminalNotifier) RestoreTerminalTitle() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.setTerminalTitle(t.originalTitle)
}

// ClearAlert restores the terminal title to its original state
func (t *TerminalNotifier) ClearAlert() error {
	return t.RestoreTerminalTitle()
}

// setTerminalTitle sets the terminal window title using ANSI escape sequences
func (t *TerminalNotifier) setTerminalTitle(title string) error {
	switch runtime.GOOS {
	case "windows":
		// Windows Command Prompt and PowerShell support
		// Use ANSI escape sequence for Windows Terminal, ConEmu, etc.
		fmt.Printf("\033]0;%s\007", title)

		// Also try using Windows API title change (works in cmd.exe)
		// Note: This requires the terminal to support ANSI sequences
		// Windows 10+ with VT100 enabled should work
		return nil

	case "linux", "darwin":
		// Unix-like systems (Linux, macOS)
		// Use ANSI OSC (Operating System Command) sequence
		fmt.Printf("\033]0;%s\007", title)
		return nil

	default:
		return fmt.Errorf("terminal title manipulation not supported on %s", runtime.GOOS)
	}
}

// IsSupported returns true if terminal title manipulation is supported
func (t *TerminalNotifier) IsSupported() bool {
	// Check if we're running in a terminal
	if !isTerminal() {
		return false
	}

	// Supported on Windows (with modern terminals), Linux, and macOS
	switch runtime.GOOS {
	case "windows", "linux", "darwin":
		return true
	default:
		return false
	}
}

// isTerminal checks if stdout is connected to a terminal
func isTerminal() bool {
	fileInfo, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	// Check if it's a character device (terminal)
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}

// GetCurrentTitle returns the stored original title
func (t *TerminalNotifier) GetCurrentTitle() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.originalTitle
}
