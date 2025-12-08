package external

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/CLIAIMONITOR/internal/events"
)

func TestSlackNotifier_Name(t *testing.T) {
	notifier := NewSlackNotifier(SlackConfig{})
	if notifier.Name() != "slack" {
		t.Errorf("expected name 'slack', got '%s'", notifier.Name())
	}
}

func TestSlackNotifier_ShouldNotify(t *testing.T) {
	tests := []struct {
		name     string
		config   SlackConfig
		event    events.Event
		expected bool
	}{
		{
			name:   "no filters - should notify",
			config: SlackConfig{},
			event: events.Event{
				Type:     events.EventAlert,
				Priority: events.PriorityNormal,
			},
			expected: true,
		},
		{
			name: "priority filter - event too low",
			config: SlackConfig{
				MinPriority: events.PriorityHigh,
			},
			event: events.Event{
				Type:     events.EventAlert,
				Priority: events.PriorityNormal,
			},
			expected: false,
		},
		{
			name: "priority filter - event matches",
			config: SlackConfig{
				MinPriority: events.PriorityHigh,
			},
			event: events.Event{
				Type:     events.EventAlert,
				Priority: events.PriorityHigh,
			},
			expected: true,
		},
		{
			name: "priority filter - event higher priority",
			config: SlackConfig{
				MinPriority: events.PriorityHigh,
			},
			event: events.Event{
				Type:     events.EventAlert,
				Priority: events.PriorityCritical,
			},
			expected: true,
		},
		{
			name: "event type filter - matches",
			config: SlackConfig{
				EventTypes: []events.EventType{events.EventAlert, events.EventTask},
			},
			event: events.Event{
				Type:     events.EventAlert,
				Priority: events.PriorityNormal,
			},
			expected: true,
		},
		{
			name: "event type filter - no match",
			config: SlackConfig{
				EventTypes: []events.EventType{events.EventTask},
			},
			event: events.Event{
				Type:     events.EventAlert,
				Priority: events.PriorityNormal,
			},
			expected: false,
		},
		{
			name: "both filters - both match",
			config: SlackConfig{
				MinPriority: events.PriorityHigh,
				EventTypes:  []events.EventType{events.EventAlert},
			},
			event: events.Event{
				Type:     events.EventAlert,
				Priority: events.PriorityCritical,
			},
			expected: true,
		},
		{
			name: "both filters - priority fails",
			config: SlackConfig{
				MinPriority: events.PriorityHigh,
				EventTypes:  []events.EventType{events.EventAlert},
			},
			event: events.Event{
				Type:     events.EventAlert,
				Priority: events.PriorityNormal,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notifier := NewSlackNotifier(tt.config)
			result := notifier.ShouldNotify(tt.event)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestSlackNotifier_Send(t *testing.T) {
	tests := []struct {
		name          string
		config        SlackConfig
		event         events.Event
		expectError   bool
		validatePayload func(t *testing.T, payload map[string]interface{})
	}{
		{
			name: "basic notification",
			config: SlackConfig{
				Channel:   "#alerts",
				Username:  "CLIAIMONITOR",
				IconEmoji: ":robot_face:",
			},
			event: events.Event{
				ID:       "test-123",
				Type:     events.EventAlert,
				Source:   "captain",
				Target:   "system",
				Priority: events.PriorityNormal,
				Payload: map[string]interface{}{
					"message": "Test alert",
				},
			},
			expectError: false,
			validatePayload: func(t *testing.T, payload map[string]interface{}) {
				if payload["channel"] != "#alerts" {
					t.Errorf("expected channel '#alerts', got '%v'", payload["channel"])
				}
				if payload["username"] != "CLIAIMONITOR" {
					t.Errorf("expected username 'CLIAIMONITOR', got '%v'", payload["username"])
				}
				if payload["icon_emoji"] != ":robot_face:" {
					t.Errorf("expected icon_emoji ':robot_face:', got '%v'", payload["icon_emoji"])
				}
				attachments, ok := payload["attachments"].([]interface{})
				if !ok || len(attachments) == 0 {
					t.Fatal("expected attachments array")
				}
				attachment := attachments[0].(map[string]interface{})
				if attachment["color"] != "good" {
					t.Errorf("expected color 'good', got '%v'", attachment["color"])
				}
			},
		},
		{
			name: "critical priority",
			config: SlackConfig{},
			event: events.Event{
				ID:       "crit-456",
				Type:     events.EventAlert,
				Source:   "agent-1",
				Priority: events.PriorityCritical,
				Payload:  map[string]interface{}{},
			},
			expectError: false,
			validatePayload: func(t *testing.T, payload map[string]interface{}) {
				attachments := payload["attachments"].([]interface{})
				attachment := attachments[0].(map[string]interface{})
				if attachment["color"] != "danger" {
					t.Errorf("expected color 'danger' for critical, got '%v'", attachment["color"])
				}
			},
		},
		{
			name: "high priority",
			config: SlackConfig{},
			event: events.Event{
				ID:       "high-789",
				Type:     events.EventTask,
				Source:   "agent-2",
				Priority: events.PriorityHigh,
				Payload:  map[string]interface{}{},
			},
			expectError: false,
			validatePayload: func(t *testing.T, payload map[string]interface{}) {
				attachments := payload["attachments"].([]interface{})
				attachment := attachments[0].(map[string]interface{})
				if attachment["color"] != "warning" {
					t.Errorf("expected color 'warning' for high, got '%v'", attachment["color"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			var receivedPayload map[string]interface{}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Fatalf("failed to read request body: %v", err)
				}
				if err := json.Unmarshal(body, &receivedPayload); err != nil {
					t.Fatalf("failed to unmarshal payload: %v", err)
				}
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			// Update config with test server URL
			tt.config.WebhookURL = server.URL

			// Create notifier and send
			notifier := NewSlackNotifier(tt.config)
			err := notifier.Send(tt.event)

			if tt.expectError && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			// Validate payload if test succeeded
			if !tt.expectError && tt.validatePayload != nil {
				tt.validatePayload(t, receivedPayload)
			}
		})
	}
}

func TestSlackNotifier_Send_NoWebhook(t *testing.T) {
	notifier := NewSlackNotifier(SlackConfig{})
	event := events.Event{
		ID:       "test-1",
		Type:     events.EventAlert,
		Source:   "test",
		Priority: events.PriorityNormal,
	}

	err := notifier.Send(event)
	if err == nil {
		t.Error("expected error for missing webhook URL")
	}
}

func TestSlackNotifier_Send_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	notifier := NewSlackNotifier(SlackConfig{
		WebhookURL: server.URL,
	})
	event := events.Event{
		ID:       "test-2",
		Type:     events.EventAlert,
		Source:   "test",
		Priority: events.PriorityNormal,
	}

	err := notifier.Send(event)
	if err == nil {
		t.Error("expected error for server error response")
	}
}
