package notifications

import (
	"fmt"
	"runtime"

	"github.com/go-toast/toast"
)

// ToastNotifier handles Windows toast notifications
type ToastNotifier struct {
	appID       string
	dashboardURL string
}

// NewToastNotifier creates a new toast notifier
func NewToastNotifier(appID string) *ToastNotifier {
	if appID == "" {
		appID = "CLIAIMONITOR"
	}
	return &ToastNotifier{
		appID:       appID,
		dashboardURL: "http://localhost:8080",
	}
}

// NewToastNotifierWithURL creates a new toast notifier with a custom dashboard URL
func NewToastNotifierWithURL(appID, dashboardURL string) *ToastNotifier {
	if appID == "" {
		appID = "CLIAIMONITOR"
	}
	if dashboardURL == "" {
		dashboardURL = "http://localhost:8080"
	}
	return &ToastNotifier{
		appID:       appID,
		dashboardURL: dashboardURL,
	}
}

// ShowToast displays a Windows toast notification with sound
func (t *ToastNotifier) ShowToast(title, message string) error {
	// Only works on Windows
	if runtime.GOOS != "windows" {
		return fmt.Errorf("toast notifications only supported on Windows")
	}

	notification := toast.Notification{
		AppID:   t.appID,
		Title:   title,
		Message: message,
		Audio:   toast.Default,
		// Add action to focus dashboard when clicked
		Actions: []toast.Action{
			{
				Type:      "protocol",
				Label:     "Open Dashboard",
				Arguments: t.dashboardURL,
			},
		},
	}

	return notification.Push()
}

// NotifySupervisorNeedsInput sends a high-priority toast notification for supervisor alerts
func (t *ToastNotifier) NotifySupervisorNeedsInput(message string) error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf("toast notifications only supported on Windows")
	}

	notification := toast.Notification{
		AppID:   t.appID,
		Title:   "Supervisor Needs Input",
		Message: message,
		Audio:   toast.IM, // Instant message sound
		Icon:    "", // Could add custom icon path
		Actions: []toast.Action{
			{
				Type:      "protocol",
				Label:     "View Now",
				Arguments: t.dashboardURL,
			},
		},
	}

	return notification.Push()
}

// IsSupported returns true if toast notifications are supported on this platform
func (t *ToastNotifier) IsSupported() bool {
	return runtime.GOOS == "windows"
}
