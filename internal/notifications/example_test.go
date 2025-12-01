package notifications_test

import (
	"fmt"
	"log"
	"time"

	"github.com/CLIAIMONITOR/internal/notifications"
)

// Example: Basic usage with default manager
func ExampleNewDefaultManager() {
	// Create a notification manager with default settings
	manager := notifications.NewDefaultManager()

	// Send a supervisor alert (triggers all notification channels)
	err := manager.NotifySupervisorNeedsInput("Agent needs approval to proceed")
	if err != nil {
		log.Printf("Notification error: %v", err)
	}

	// Clear the alert when done
	manager.ClearAlert()
}

// Example: Custom configuration
func ExampleNewManager() {
	// Create custom configuration
	config := notifications.Config{
		AppID:          "MyApp",
		DashboardURL:   "http://localhost:8080",
		EnableToast:    true,
		EnableTerminal: true,
		EnableBanner:   true,
		Logger:         log.Default(),
	}

	manager := notifications.NewManager(config)

	// Use the manager
	manager.ShowDashboardBanner("Application started")
}

// Example: Individual notification channels
func ExampleManager_ShowToast() {
	manager := notifications.NewDefaultManager()

	// Show a Windows toast notification
	err := manager.ShowToast("Deployment Complete", "Application deployed successfully")
	if err != nil {
		log.Printf("Toast notification failed: %v", err)
	}
}

// Example: Terminal title flash
func ExampleManager_FlashTerminal() {
	manager := notifications.NewDefaultManager()

	// Set the original title
	manager.SetTerminalTitle("CLIAIMONITOR")

	// Flash the terminal with an alert
	manager.FlashTerminal("Build failed - attention needed")

	// Restore after some time
	time.Sleep(5 * time.Second)
	manager.ClearAlert()
}

// Example: Dashboard banner
func ExampleManager_ShowDashboardBanner() {
	manager := notifications.NewDefaultManager()

	// Show info banner
	manager.ShowDashboardBanner("System update available")

	// Get banner state (for HTTP handler)
	state := manager.GetBannerState()
	fmt.Printf("Banner visible: %v, Message: %s\n", state.Visible, state.Message)

	// Clear banner
	manager.ClearAlert()
}

// Example: Enable/Disable notifications
func ExampleManager_Disable() {
	manager := notifications.NewDefaultManager()

	// Disable all notifications during maintenance
	manager.Disable()

	// This will return an error
	err := manager.ShowToast("Test", "This won't show")
	if err != nil {
		fmt.Println("Notifications are disabled")
	}

	// Re-enable
	manager.Enable()

	// This will work
	manager.ShowDashboardBanner("Maintenance complete")
}

// Example: Supervisor alert workflow
func ExampleManager_NotifySupervisorNeedsInput() {
	manager := notifications.NewDefaultManager()

	// When supervisor approval is needed
	err := manager.NotifySupervisorNeedsInput("Agent requests permission to delete files")
	if err != nil {
		log.Printf("Failed to notify supervisor: %v", err)
	}

	// This triggers:
	// 1. Windows toast notification (if on Windows)
	// 2. Terminal title change
	// 3. Dashboard banner (red, supervisor type)

	// Wait for user response...
	// (In real code, this would be async with WebSocket notification)

	// Clear the alert after supervisor responds
	manager.ClearAlert()
}

// Example: Thread-safe concurrent usage
func ExampleManager_concurrent() {
	manager := notifications.NewDefaultManager()

	// Multiple goroutines can safely use the manager
	done := make(chan bool, 3)

	go func() {
		manager.ShowDashboardBanner("Worker 1 started")
		done <- true
	}()

	go func() {
		manager.FlashTerminal("Worker 2 processing")
		done <- true
	}()

	go func() {
		manager.NotifySupervisorNeedsInput("Worker 3 needs input")
		done <- true
	}()

	// Wait for all goroutines
	for i := 0; i < 3; i++ {
		<-done
	}
}

// Example: Banner state for HTTP API
func ExampleBannerNotifier_GetState() {
	banner := notifications.NewBannerNotifier()

	// Show a banner
	banner.Show("Database backup in progress", "info")

	// Get state (typically called from HTTP handler)
	state := banner.GetState()

	// Return as JSON in HTTP response
	fmt.Printf(`{"visible": %v, "message": "%s", "type": "%s"}`,
		state.Visible, state.Message, state.Type)
}

// Example: Platform-specific behavior
func ExampleToastNotifier_IsSupported() {
	toast := notifications.NewToastNotifier("CLIAIMONITOR")

	if toast.IsSupported() {
		// On Windows
		toast.ShowToast("Alert", "This is a Windows toast")
	} else {
		// On Linux/macOS - use alternative notification
		fmt.Println("Toast not supported on this platform")
	}
}

// Example: Custom terminal title
func ExampleTerminalNotifier_SetOriginalTitle() {
	terminal := notifications.NewTerminalNotifier()

	// Set custom original title
	terminal.SetOriginalTitle("My Application v1.0")

	// Flash with alert
	terminal.FlashTerminal("Error detected")

	// Later, restore to original
	terminal.RestoreTerminalTitle()
	// Title is now: "My Application v1.0"
}

// Example: Banner types
func ExampleBannerNotifier_Show() {
	banner := notifications.NewBannerNotifier()

	// Info banner (blue)
	banner.Show("System ready", "info")

	// Warning banner (yellow)
	banner.Show("High memory usage", "warning")

	// Error banner (red)
	banner.Show("Connection failed", "error")

	// Supervisor banner (red with special styling)
	banner.Show("Approval required", "supervisor")

	// Clear banner
	banner.Clear()
}
