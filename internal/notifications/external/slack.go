package external

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/CLIAIMONITOR/internal/events"
)

// SlackConfig holds configuration for Slack notifications
type SlackConfig struct {
	WebhookURL string              `json:"webhook_url"`
	Channel    string              `json:"channel,omitempty"`
	Username   string              `json:"username,omitempty"`
	IconEmoji  string              `json:"icon_emoji,omitempty"`
	EventTypes []events.EventType  `json:"event_types,omitempty"`
	MinPriority int                `json:"min_priority,omitempty"`
}

// SlackNotifier sends notifications to Slack via webhooks
type SlackNotifier struct {
	config SlackConfig
	client *http.Client
}

// NewSlackNotifier creates a new Slack notifier
func NewSlackNotifier(config SlackConfig) *SlackNotifier {
	return &SlackNotifier{
		config: config,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Name returns the notifier name
func (s *SlackNotifier) Name() string {
	return "slack"
}

// ShouldNotify checks if the event should trigger a notification
func (s *SlackNotifier) ShouldNotify(event events.Event) bool {
	// Check minimum priority
	if s.config.MinPriority > 0 && event.Priority > s.config.MinPriority {
		return false
	}

	// Check event types filter
	if len(s.config.EventTypes) > 0 {
		found := false
		for _, et := range s.config.EventTypes {
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

// Send sends a notification to Slack
func (s *SlackNotifier) Send(event events.Event) error {
	if s.config.WebhookURL == "" {
		return fmt.Errorf("slack webhook URL not configured")
	}

	// Determine color based on priority
	color := "good"
	if event.Priority == events.PriorityCritical {
		color = "danger"
	} else if event.Priority == events.PriorityHigh {
		color = "warning"
	}

	// Build attachment fields
	fields := []map[string]interface{}{
		{
			"title": "Type",
			"value": string(event.Type),
			"short": true,
		},
		{
			"title": "Source",
			"value": event.Source,
			"short": true,
		},
		{
			"title": "Priority",
			"value": priorityString(event.Priority),
			"short": true,
		},
	}

	if event.Target != "" {
		fields = append(fields, map[string]interface{}{
			"title": "Target",
			"value": event.Target,
			"short": true,
		})
	}

	// Add payload fields
	for k, v := range event.Payload {
		fields = append(fields, map[string]interface{}{
			"title": k,
			"value": fmt.Sprintf("%v", v),
			"short": false,
		})
	}

	// Build Slack message payload
	payload := map[string]interface{}{
		"text": fmt.Sprintf("Event: %s", event.ID),
		"attachments": []map[string]interface{}{
			{
				"color":  color,
				"title":  fmt.Sprintf("%s Event", event.Type),
				"fields": fields,
				"ts":     event.CreatedAt.Unix(),
			},
		},
	}

	if s.config.Channel != "" {
		payload["channel"] = s.config.Channel
	}
	if s.config.Username != "" {
		payload["username"] = s.config.Username
	}
	if s.config.IconEmoji != "" {
		payload["icon_emoji"] = s.config.IconEmoji
	}

	// Marshal payload
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Send HTTP request
	resp, err := s.client.Post(s.config.WebhookURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to send slack notification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack API returned status %d", resp.StatusCode)
	}

	return nil
}

// priorityString converts priority number to string
func priorityString(priority int) string {
	switch priority {
	case events.PriorityCritical:
		return "Critical"
	case events.PriorityHigh:
		return "High"
	case events.PriorityNormal:
		return "Normal"
	case events.PriorityLow:
		return "Low"
	default:
		return fmt.Sprintf("Unknown (%d)", priority)
	}
}
