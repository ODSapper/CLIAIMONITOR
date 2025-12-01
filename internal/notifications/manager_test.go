package notifications

import (
	"log"
	"os"
	"testing"
)

func TestNewManager(t *testing.T) {
	config := Config{
		AppID:          "TestApp",
		DashboardURL:   "http://localhost:8080",
		EnableToast:    true,
		EnableTerminal: true,
		EnableBanner:   true,
		Logger:         log.New(os.Stdout, "", 0),
	}

	manager := NewManager(config)
	if manager == nil {
		t.Fatal("NewManager returned nil")
	}

	if !manager.IsEnabled() {
		t.Error("Expected manager to be enabled")
	}
}

func TestNewDefaultManager(t *testing.T) {
	manager := NewDefaultManager()
	if manager == nil {
		t.Fatal("NewDefaultManager returned nil")
	}

	if !manager.IsEnabled() {
		t.Error("Expected default manager to be enabled")
	}
}

func TestManagerEnableDisable(t *testing.T) {
	manager := NewDefaultManager()

	// Initially enabled
	if !manager.IsEnabled() {
		t.Error("Expected manager to be enabled initially")
	}

	// Disable
	manager.Disable()
	if manager.IsEnabled() {
		t.Error("Expected manager to be disabled after Disable()")
	}

	// Enable
	manager.Enable()
	if !manager.IsEnabled() {
		t.Error("Expected manager to be enabled after Enable()")
	}
}

func TestManagerShowToast(t *testing.T) {
	manager := NewDefaultManager()

	err := manager.ShowToast("Test Title", "Test Message")

	// Error behavior depends on platform
	// We mainly test that it doesn't panic
	_ = err
}

func TestManagerFlashTerminal(t *testing.T) {
	manager := NewDefaultManager()

	err := manager.FlashTerminal("Test Alert")

	// Should not panic
	_ = err
}

func TestManagerShowDashboardBanner(t *testing.T) {
	manager := NewDefaultManager()

	err := manager.ShowDashboardBanner("Test Message")
	if err != nil {
		t.Errorf("ShowDashboardBanner returned error: %v", err)
	}

	// Verify banner state
	state := manager.GetBannerState()
	if !state.Visible {
		t.Error("Expected banner to be visible")
	}
	if state.Message != "Test Message" {
		t.Errorf("Expected message 'Test Message', got '%s'", state.Message)
	}
}

func TestManagerNotifySupervisorNeedsInput(t *testing.T) {
	manager := NewDefaultManager()

	err := manager.NotifySupervisorNeedsInput("Supervisor needs input")

	// Should attempt all notification methods
	// Error behavior depends on platform
	_ = err

	// Verify banner state (should always work)
	state := manager.GetBannerState()
	if !state.Visible {
		t.Error("Expected banner to be visible after supervisor notification")
	}
}

func TestManagerClearAlert(t *testing.T) {
	manager := NewDefaultManager()

	// Show banner first
	manager.ShowDashboardBanner("Test Message")

	// Clear all alerts
	err := manager.ClearAlert()
	if err != nil {
		t.Errorf("ClearAlert returned error: %v", err)
	}

	// Verify banner is cleared
	state := manager.GetBannerState()
	if state.Visible {
		t.Error("Expected banner to be hidden after ClearAlert")
	}
}

func TestManagerGetBannerState(t *testing.T) {
	manager := NewDefaultManager()

	// Initially hidden
	state := manager.GetBannerState()
	if state.Visible {
		t.Error("Expected banner to be hidden initially")
	}

	// Show banner
	manager.ShowDashboardBanner("Test")
	state = manager.GetBannerState()
	if !state.Visible {
		t.Error("Expected banner to be visible")
	}
	if state.Message != "Test" {
		t.Errorf("Expected message 'Test', got '%s'", state.Message)
	}
}

func TestManagerSetTerminalTitle(t *testing.T) {
	manager := NewDefaultManager()

	// Should not panic
	manager.SetTerminalTitle("Custom Title")

	// Verify terminal title was set
	if manager.terminal.GetCurrentTitle() != "Custom Title" {
		t.Error("Terminal title was not set correctly")
	}
}

func TestManagerDisabledNotifications(t *testing.T) {
	manager := NewDefaultManager()
	manager.Disable()

	// All notification methods should return error when disabled
	err := manager.ShowToast("Test", "Test")
	if err == nil {
		t.Error("Expected error when notifications disabled")
	}

	err = manager.FlashTerminal("Test")
	if err == nil {
		t.Error("Expected error when notifications disabled")
	}

	err = manager.ShowDashboardBanner("Test")
	if err == nil {
		t.Error("Expected error when notifications disabled")
	}

	err = manager.NotifySupervisorNeedsInput("Test")
	if err == nil {
		t.Error("Expected error when notifications disabled")
	}
}

func TestManagerConcurrentAccess(t *testing.T) {
	manager := NewDefaultManager()

	done := make(chan bool)

	// Writer goroutines
	for i := 0; i < 10; i++ {
		go func(n int) {
			for j := 0; j < 50; j++ {
				switch n % 4 {
				case 0:
					manager.ShowDashboardBanner("Test")
				case 1:
					manager.FlashTerminal("Test")
				case 2:
					manager.NotifySupervisorNeedsInput("Test")
				case 3:
					manager.ClearAlert()
				}
			}
			done <- true
		}(i)
	}

	// Reader goroutines
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 50; j++ {
				manager.GetBannerState()
				manager.IsEnabled()
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 20; i++ {
		<-done
	}
}

func TestManagerNilLogger(t *testing.T) {
	config := Config{
		AppID:          "TestApp",
		EnableToast:    true,
		EnableTerminal: true,
		EnableBanner:   true,
		Logger:         nil, // Nil logger should use default
	}

	manager := NewManager(config)
	if manager == nil {
		t.Fatal("NewManager with nil logger returned nil")
	}

	// Should not panic with nil logger
	manager.ShowDashboardBanner("Test")
}

func TestManagerPartialConfig(t *testing.T) {
	// Test with only some notification types enabled
	config := Config{
		AppID:          "TestApp",
		EnableToast:    false,
		EnableTerminal: true,
		EnableBanner:   true,
	}

	manager := NewManager(config)
	if !manager.IsEnabled() {
		t.Error("Expected manager to be enabled when some notification types are enabled")
	}

	// Test with all disabled
	config = Config{
		AppID:          "TestApp",
		EnableToast:    false,
		EnableTerminal: false,
		EnableBanner:   false,
	}

	manager = NewManager(config)
	if manager.IsEnabled() {
		t.Error("Expected manager to be disabled when all notification types are disabled")
	}
}
