package notifications

import (
	"runtime"
	"testing"
)

func TestNewTerminalNotifier(t *testing.T) {
	terminal := NewTerminalNotifier()
	if terminal == nil {
		t.Fatal("NewTerminalNotifier returned nil")
	}

	if terminal.GetCurrentTitle() != "CLIAIMONITOR" {
		t.Errorf("Expected default title 'CLIAIMONITOR', got '%s'", terminal.GetCurrentTitle())
	}
}

func TestTerminalSetOriginalTitle(t *testing.T) {
	terminal := NewTerminalNotifier()

	testTitle := "Custom Title"
	terminal.SetOriginalTitle(testTitle)

	if terminal.GetCurrentTitle() != testTitle {
		t.Errorf("Expected title '%s', got '%s'", testTitle, terminal.GetCurrentTitle())
	}
}

func TestTerminalFlashTerminal(t *testing.T) {
	terminal := NewTerminalNotifier()

	// This won't actually change the terminal title in tests,
	// but it should not error
	err := terminal.FlashTerminal("Test alert")
	if err != nil {
		t.Errorf("FlashTerminal returned error: %v", err)
	}
}

func TestTerminalNotifySupervisorNeedsInput(t *testing.T) {
	terminal := NewTerminalNotifier()

	err := terminal.NotifySupervisorNeedsInput("Supervisor alert")
	if err != nil {
		t.Errorf("NotifySupervisorNeedsInput returned error: %v", err)
	}
}

func TestTerminalRestoreTitle(t *testing.T) {
	terminal := NewTerminalNotifier()

	originalTitle := "My Application"
	terminal.SetOriginalTitle(originalTitle)

	// Flash terminal
	terminal.FlashTerminal("Alert")

	// Restore
	err := terminal.RestoreTerminalTitle()
	if err != nil {
		t.Errorf("RestoreTerminalTitle returned error: %v", err)
	}
}

func TestTerminalClearAlert(t *testing.T) {
	terminal := NewTerminalNotifier()

	err := terminal.ClearAlert()
	if err != nil {
		t.Errorf("ClearAlert returned error: %v", err)
	}
}

func TestTerminalIsSupported(t *testing.T) {
	terminal := NewTerminalNotifier()

	supported := terminal.IsSupported()

	// Terminal title manipulation should be supported on Windows, Linux, and macOS
	switch runtime.GOOS {
	case "windows", "linux", "darwin":
		// We can't reliably test if we're in a terminal during tests,
		// so we just verify the method doesn't panic
		_ = supported
	default:
		if supported {
			t.Error("Expected terminal manipulation to be unsupported on this platform")
		}
	}
}

func TestTerminalThreadSafety(t *testing.T) {
	terminal := NewTerminalNotifier()

	done := make(chan bool)

	// Writer goroutines
	for i := 0; i < 10; i++ {
		go func(n int) {
			for j := 0; j < 100; j++ {
				if n%2 == 0 {
					terminal.FlashTerminal("Alert")
				} else {
					terminal.RestoreTerminalTitle()
				}
			}
			done <- true
		}(i)
	}

	// Reader goroutines
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				terminal.GetCurrentTitle()
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 20; i++ {
		<-done
	}
}

func TestTerminalSetTitleConcurrent(t *testing.T) {
	terminal := NewTerminalNotifier()

	done := make(chan bool)

	// Multiple goroutines setting title
	for i := 0; i < 10; i++ {
		go func(n int) {
			for j := 0; j < 50; j++ {
				terminal.SetOriginalTitle("Title from goroutine")
				terminal.GetCurrentTitle()
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}
