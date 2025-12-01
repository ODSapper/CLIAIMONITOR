package notifications

import (
	"runtime"
	"testing"
)

func TestNewToastNotifier(t *testing.T) {
	toast := NewToastNotifier("")
	if toast == nil {
		t.Fatal("NewToastNotifier returned nil")
	}

	if toast.appID != "CLIAIMONITOR" {
		t.Errorf("Expected default appID 'CLIAIMONITOR', got '%s'", toast.appID)
	}
}

func TestNewToastNotifierWithAppID(t *testing.T) {
	customAppID := "MyCustomApp"
	toast := NewToastNotifier(customAppID)

	if toast.appID != customAppID {
		t.Errorf("Expected appID '%s', got '%s'", customAppID, toast.appID)
	}
}

func TestToastIsSupported(t *testing.T) {
	toast := NewToastNotifier("")

	supported := toast.IsSupported()

	if runtime.GOOS == "windows" {
		if !supported {
			t.Error("Expected toast to be supported on Windows")
		}
	} else {
		if supported {
			t.Error("Expected toast to be unsupported on non-Windows platforms")
		}
	}
}

func TestToastShowToast(t *testing.T) {
	toast := NewToastNotifier("")

	err := toast.ShowToast("Test Title", "Test Message")

	// On Windows, this might fail if we don't have proper permissions
	// or if the notification system is not available
	// On other platforms, it should return an error
	if runtime.GOOS != "windows" {
		if err == nil {
			t.Error("Expected error on non-Windows platform")
		}
	}
	// On Windows, we can't reliably test if it succeeds without user interaction
	// so we just verify it doesn't panic
}

func TestToastNotifySupervisorNeedsInput(t *testing.T) {
	toast := NewToastNotifier("")

	err := toast.NotifySupervisorNeedsInput("Supervisor needs your input")

	// Similar to ShowToast, behavior depends on platform
	if runtime.GOOS != "windows" {
		if err == nil {
			t.Error("Expected error on non-Windows platform")
		}
	}
}

func TestToastMultipleNotifications(t *testing.T) {
	toast := NewToastNotifier("")

	// Send multiple notifications rapidly
	// This tests that we don't panic or cause issues
	for i := 0; i < 5; i++ {
		err := toast.ShowToast("Test", "Message")
		// On non-Windows, all should error
		if runtime.GOOS != "windows" && err == nil {
			t.Error("Expected error on non-Windows platform")
		}
	}
}

func TestToastEmptyMessages(t *testing.T) {
	toast := NewToastNotifier("")

	// Test with empty messages - should not panic
	err := toast.ShowToast("", "")
	if runtime.GOOS != "windows" && err == nil {
		t.Error("Expected error on non-Windows platform")
	}

	err = toast.NotifySupervisorNeedsInput("")
	if runtime.GOOS != "windows" && err == nil {
		t.Error("Expected error on non-Windows platform")
	}
}

func TestToastConcurrentAccess(t *testing.T) {
	toast := NewToastNotifier("")

	done := make(chan bool)

	// Multiple goroutines sending toasts
	for i := 0; i < 5; i++ {
		go func(n int) {
			for j := 0; j < 20; j++ {
				toast.ShowToast("Test", "Message")
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 5; i++ {
		<-done
	}
}
