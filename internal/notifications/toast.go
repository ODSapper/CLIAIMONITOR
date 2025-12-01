package notifications

import (
	"fmt"
	"runtime"

	"github.com/go-toast/toast"
)

// ToastNotifier handles Windows toast notifications
type ToastNotifier struct {
	appID string
}

// NewToastNotifier creates a new toast notifier
func NewToastNotifier(appID string) *ToastNotifier {
	if appID == "" {
		appID = "CLIAIMONITOR"
	}
	return &ToastNotifier{
		appID: appID,
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
				Arguments: "http://localhost:8080", // Will be configurable
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
				Arguments: "http://localhost:8080",
			},
		},
	}

	return notification.Push()
}

// IsSupported returns true if toast notifications are supported on this platform
func (t *ToastNotifier) IsSupported() bool {
	return runtime.GOOS == "windows"
}
