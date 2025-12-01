package notifications

import (
	"testing"
	"time"
)

func TestNewBannerNotifier(t *testing.T) {
	banner := NewBannerNotifier()
	if banner == nil {
		t.Fatal("NewBannerNotifier returned nil")
	}

	state := banner.GetState()
	if state.Visible {
		t.Error("Expected new banner to be hidden")
	}
}

func TestBannerShow(t *testing.T) {
	banner := NewBannerNotifier()

	// Test showing info banner
	err := banner.Show("Test message", "info")
	if err != nil {
		t.Fatalf("Show failed: %v", err)
	}

	state := banner.GetState()
	if !state.Visible {
		t.Error("Expected banner to be visible after Show")
	}
	if state.Message != "Test message" {
		t.Errorf("Expected message 'Test message', got '%s'", state.Message)
	}
	if state.Type != BannerTypeInfo {
		t.Errorf("Expected type 'info', got '%s'", state.Type)
	}
}

func TestBannerSupervisorAlert(t *testing.T) {
	banner := NewBannerNotifier()

	err := banner.ShowSupervisorAlert("Supervisor needs input")
	if err != nil {
		t.Fatalf("ShowSupervisorAlert failed: %v", err)
	}

	state := banner.GetState()
	if !state.Visible {
		t.Error("Expected banner to be visible")
	}
	if state.Type != BannerTypeSupervisor {
		t.Errorf("Expected type 'supervisor', got '%s'", state.Type)
	}
	if state.Message != "Supervisor needs input" {
		t.Errorf("Expected message 'Supervisor needs input', got '%s'", state.Message)
	}
}

func TestBannerClear(t *testing.T) {
	banner := NewBannerNotifier()

	// Show banner
	banner.Show("Test message", "info")
	if !banner.IsVisible() {
		t.Error("Expected banner to be visible")
	}

	// Clear banner
	err := banner.Clear()
	if err != nil {
		t.Fatalf("Clear failed: %v", err)
	}

	if banner.IsVisible() {
		t.Error("Expected banner to be hidden after Clear")
	}
}

func TestBannerThreadSafety(t *testing.T) {
	banner := NewBannerNotifier()

	// Test concurrent access
	done := make(chan bool)

	// Writer goroutines
	for i := 0; i < 10; i++ {
		go func(n int) {
			for j := 0; j < 100; j++ {
				if n%2 == 0 {
					banner.Show("Test", "info")
				} else {
					banner.Clear()
				}
			}
			done <- true
		}(i)
	}

	// Reader goroutines
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				banner.GetState()
				banner.IsVisible()
				banner.GetMessage()
				banner.GetType()
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 20; i++ {
		<-done
	}
}

func TestBannerGetters(t *testing.T) {
	banner := NewBannerNotifier()

	// Initially hidden
	if banner.IsVisible() {
		t.Error("Expected new banner to be hidden")
	}
	if banner.GetMessage() != "" {
		t.Error("Expected empty message for new banner")
	}

	// Show banner
	testMessage := "Test notification"
	banner.Show(testMessage, "warning")

	if !banner.IsVisible() {
		t.Error("Expected banner to be visible")
	}
	if banner.GetMessage() != testMessage {
		t.Errorf("Expected message '%s', got '%s'", testMessage, banner.GetMessage())
	}
	if banner.GetType() != BannerTypeWarning {
		t.Errorf("Expected type 'warning', got '%s'", banner.GetType())
	}
}

func TestBannerTimestamp(t *testing.T) {
	banner := NewBannerNotifier()

	before := time.Now()
	banner.Show("Test", "info")
	after := time.Now()

	state := banner.GetState()
	if state.Timestamp.Before(before) || state.Timestamp.After(after) {
		t.Error("Timestamp not set correctly")
	}
}

func TestBannerTypes(t *testing.T) {
	banner := NewBannerNotifier()

	tests := []struct {
		bannerType string
		expected   BannerType
	}{
		{"info", BannerTypeInfo},
		{"warning", BannerTypeWarning},
		{"error", BannerTypeError},
		{"supervisor", BannerTypeSupervisor},
	}

	for _, tt := range tests {
		t.Run(string(tt.expected), func(t *testing.T) {
			banner.Show("Test", tt.bannerType)
			state := banner.GetState()
			if state.Type != tt.expected {
				t.Errorf("Expected type %s, got %s", tt.expected, state.Type)
			}
		})
	}
}
