package external

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/CLIAIMONITOR/internal/events"
)

// DiscordConfig holds configuration for Discord notifications
type DiscordConfig struct {
	WebhookURL string              `json:"webhook_url"`
	Username   string              `json:"username,omitempty"`
	AvatarURL  string              `json:"avatar_url,omitempty"`
	EventTypes []events.EventType  `json:"event_types,omitempty"`
	MinPriority int                `json:"min_priority,omitempty"`
}

// DiscordNotifier sends notifications to Discord via webhooks
type DiscordNotifier struct {
	config DiscordConfig
	client *http.Client
}

// NewDiscordNotifier creates a new Discord notifier
func NewDiscordNotifier(config DiscordConfig) *DiscordNotifier {
	return &DiscordNotifier{
		config: config,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Name returns the notifier name
func (d *DiscordNotifier) Name() string {
	return "discord"
}

// ShouldNotify checks if the event should trigger a notification
func (d *DiscordNotifier) ShouldNotify(event events.Event) bool {
	// Check minimum priority
	if d.config.MinPriority > 0 && event.Priority > d.config.MinPriority {
		return false
	}

	// Check event types filter
	if len(d.config.EventTypes) > 0 {
		found := false
		for _, et := range d.config.EventTypes {
			if event.Type == et {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

// Send sends a notification to Discord
func (d *DiscordNotifier) Send(event events.Event) error {
	if d.config.WebhookURL == "" {
		return fmt.Errorf("discord webhook URL not configured")
	}

	// Determine color based on priority
	color := 0x00FF00 // green for normal
	if event.Priority == events.PriorityCritical {
		color = 0xFF0000 // red
	} else if event.Priority == events.PriorityHigh {
		color = 0xFFA500 // orange
	}

	// Build embed fields
	fields := []map[string]interface{}{
		{
			"name":   "Type",
			"value":  string(event.Type),
			"inline": true,
		},
		{
			"name":   "Source",
			"value":  event.Source,
			"inline": true,
		},
		{
			"name":   "Priority",
			"value":  priorityString(event.Priority),
			"inline": true,
		},
	}

	if event.Target != "" {
		fields = append(fields, map[string]interface{}{
			"name":   "Target",
			"value":  event.Target,
			"inline": true,
		})
	}

	// Add payload fields
	for k, v := range event.Payload {
		fields = append(fields, map[string]interface{}{
			"name":   k,
			"value":  fmt.Sprintf("%v", v),
			"inline": false,
		})
	}

	// Build Discord embed
	embed := map[string]interface{}{
		"title":       fmt.Sprintf("%s Event", event.Type),
		"description": fmt.Sprintf("Event ID: %s", event.ID),
		"color":       color,
		"timestamp":   event.CreatedAt.Format(time.RFC3339),
		"fields":      fields,
	}

	// Build Discord message payload
	payload := map[string]interface{}{
		"embeds": []map[string]interface{}{embed},
	}

	if d.config.Username != "" {
		payload["username"] = d.config.Username
	}
	if d.config.AvatarURL != "" {
		payload["avatar_url"] = d.config.AvatarURL
	}

	// Marshal payload
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Send HTTP request
	resp, err := d.client.Post(d.config.WebhookURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to send discord notification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("discord API returned status %d", resp.StatusCode)
	}

	return nil
}
