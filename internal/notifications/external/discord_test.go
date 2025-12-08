package external

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/CLIAIMONITOR/internal/events"
)

func TestDiscordNotifier_Name(t *testing.T) {
	notifier := NewDiscordNotifier(DiscordConfig{})
	if notifier.Name() != "discord" {
		t.Errorf("expected name 'discord', got '%s'", notifier.Name())
	}
}

func TestDiscordNotifier_ShouldNotify(t *testing.T) {
	tests := []struct {
		name     string
		config   DiscordConfig
		event    events.Event
		expected bool
	}{
		{
			name:   "no filters - should notify",
			config: DiscordConfig{},
			event: events.Event{
				Type:     events.EventAlert,
				Priority: events.PriorityNormal,
			},
			expected: true,
		},
		{
			name: "priority filter - event too low",
			config: DiscordConfig{
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
			config: DiscordConfig{
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
			config: DiscordConfig{
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
			config: DiscordConfig{
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
			config: DiscordConfig{
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
			config: DiscordConfig{
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
			config: DiscordConfig{
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
			notifier := NewDiscordNotifier(tt.config)
			result := notifier.ShouldNotify(tt.event)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestDiscordNotifier_Send(t *testing.T) {
	tests := []struct {
		name          string
		config        DiscordConfig
		event         events.Event
		expectError   bool
		validatePayload func(t *testing.T, payload map[string]interface{})
	}{
		{
			name: "basic notification",
			config: DiscordConfig{
				Username:  "CLIAIMONITOR",
				AvatarURL: "https://example.com/avatar.png",
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
				if payload["username"] != "CLIAIMONITOR" {
					t.Errorf("expected username 'CLIAIMONITOR', got '%v'", payload["username"])
				}
				if payload["avatar_url"] != "https://example.com/avatar.png" {
					t.Errorf("expected avatar_url, got '%v'", payload["avatar_url"])
				}
				embeds, ok := payload["embeds"].([]interface{})
				if !ok || len(embeds) == 0 {
					t.Fatal("expected embeds array")
				}
				embed := embeds[0].(map[string]interface{})
				if embed["color"].(float64) != 0x00FF00 {
					t.Errorf("expected color 0x00FF00 (green), got %v", embed["color"])
				}
			},
		},
		{
			name: "critical priority",
			config: DiscordConfig{},
			event: events.Event{
				ID:       "crit-456",
				Type:     events.EventAlert,
				Source:   "agent-1",
				Priority: events.PriorityCritical,
				Payload:  map[string]interface{}{},
			},
			expectError: false,
			validatePayload: func(t *testing.T, payload map[string]interface{}) {
				embeds := payload["embeds"].([]interface{})
				embed := embeds[0].(map[string]interface{})
				if embed["color"].(float64) != 0xFF0000 {
					t.Errorf("expected color 0xFF0000 (red) for critical, got %v", embed["color"])
				}
			},
		},
		{
			name: "high priority",
			config: DiscordConfig{},
			event: events.Event{
				ID:       "high-789",
				Type:     events.EventTask,
				Source:   "agent-2",
				Priority: events.PriorityHigh,
				Payload:  map[string]interface{}{},
			},
			expectError: false,
			validatePayload: func(t *testing.T, payload map[string]interface{}) {
				embeds := payload["embeds"].([]interface{})
				embed := embeds[0].(map[string]interface{})
				if embed["color"].(float64) != 0xFFA500 {
					t.Errorf("expected color 0xFFA500 (orange) for high, got %v", embed["color"])
				}
			},
		},
		{
			name: "with target field",
			config: DiscordConfig{},
			event: events.Event{
				ID:       "target-123",
				Type:     events.EventMessage,
				Source:   "captain",
				Target:   "agent-3",
				Priority: events.PriorityNormal,
				Payload:  map[string]interface{}{},
			},
			expectError: false,
			validatePayload: func(t *testing.T, payload map[string]interface{}) {
				embeds := payload["embeds"].([]interface{})
				embed := embeds[0].(map[string]interface{})
				fields := embed["fields"].([]interface{})

				// Look for target field
				foundTarget := false
				for _, f := range fields {
					field := f.(map[string]interface{})
					if field["name"] == "Target" {
						foundTarget = true
						if field["value"] != "agent-3" {
							t.Errorf("expected target 'agent-3', got '%v'", field["value"])
						}
						break
					}
				}
				if !foundTarget {
					t.Error("expected target field in embed")
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
				w.WriteHeader(http.StatusNoContent)
			}))
			defer server.Close()

			// Update config with test server URL
			tt.config.WebhookURL = server.URL

			// Create notifier and send
			notifier := NewDiscordNotifier(tt.config)
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

func TestDiscordNotifier_Send_NoWebhook(t *testing.T) {
	notifier := NewDiscordNotifier(DiscordConfig{})
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

func TestDiscordNotifier_Send_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	notifier := NewDiscordNotifier(DiscordConfig{
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
