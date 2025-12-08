package external

import (
	"fmt"
	"net/smtp"
	"strings"
	"time"

	"github.com/CLIAIMONITOR/internal/events"
)

// EmailConfig holds configuration for email notifications
type EmailConfig struct {
	SMTPHost    string              `json:"smtp_host"`
	SMTPPort    int                 `json:"smtp_port"`
	Username    string              `json:"username"`
	Password    string              `json:"password"`
	From        string              `json:"from"`
	To          []string            `json:"to"`
	EventTypes  []events.EventType  `json:"event_types,omitempty"`
	MinPriority int                 `json:"min_priority,omitempty"`
}

// EmailNotifier sends notifications via email
type EmailNotifier struct {
	config EmailConfig
}

// NewEmailNotifier creates a new email notifier
func NewEmailNotifier(config EmailConfig) *EmailNotifier {
	return &EmailNotifier{
		config: config,
	}
}

// Name returns the notifier name
func (e *EmailNotifier) Name() string {
	return "email"
}

// ShouldNotify checks if the event should trigger a notification
func (e *EmailNotifier) ShouldNotify(event events.Event) bool {
	// Check minimum priority
	if e.config.MinPriority > 0 && event.Priority > e.config.MinPriority {
		return false
	}

	// Check event types filter
	if len(e.config.EventTypes) > 0 {
		found := false
		for _, et := range e.config.EventTypes {
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

// Send sends a notification via email
func (e *EmailNotifier) Send(event events.Event) error {
	if e.config.SMTPHost == "" {
		return fmt.Errorf("SMTP host not configured")
	}
	if e.config.From == "" {
		return fmt.Errorf("from address not configured")
	}
	if len(e.config.To) == 0 {
		return fmt.Errorf("no recipient addresses configured")
	}

	// Build subject with priority prefix
	subject := e.buildSubject(event)

	// Build email body
	body := e.buildBody(event)

	// Build email message
	message := e.buildMessage(subject, body)

	// Send via SMTP
	addr := fmt.Sprintf("%s:%d", e.config.SMTPHost, e.config.SMTPPort)
	var auth smtp.Auth
	if e.config.Username != "" && e.config.Password != "" {
		auth = smtp.PlainAuth("", e.config.Username, e.config.Password, e.config.SMTPHost)
	}

	err := smtp.SendMail(addr, auth, e.config.From, e.config.To, []byte(message))
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

// buildSubject creates the email subject line with priority prefix
func (e *EmailNotifier) buildSubject(event events.Event) string {
	prefix := ""
	if event.Priority == events.PriorityCritical {
		prefix = "[CRITICAL] "
	} else if event.Priority == events.PriorityHigh {
		prefix = "[HIGH] "
	}

	return fmt.Sprintf("%sCLIAIMONITOR %s Event - %s", prefix, event.Type, event.ID)
}

// buildBody creates the email body content
func (e *EmailNotifier) buildBody(event events.Event) string {
	var body strings.Builder

	body.WriteString("CLIAIMONITOR Event Notification\n")
	body.WriteString("================================\n\n")

	body.WriteString(fmt.Sprintf("Event ID: %s\n", event.ID))
	body.WriteString(fmt.Sprintf("Type: %s\n", event.Type))
	body.WriteString(fmt.Sprintf("Source: %s\n", event.Source))
	if event.Target != "" {
		body.WriteString(fmt.Sprintf("Target: %s\n", event.Target))
	}
	body.WriteString(fmt.Sprintf("Priority: %s\n", priorityString(event.Priority)))
	body.WriteString(fmt.Sprintf("Timestamp: %s\n", event.CreatedAt.Format(time.RFC3339)))

	if len(event.Payload) > 0 {
		body.WriteString("\nPayload:\n")
		body.WriteString("--------\n")
		for k, v := range event.Payload {
			body.WriteString(fmt.Sprintf("%s: %v\n", k, v))
		}
	}

	body.WriteString("\n--\n")
	body.WriteString("This is an automated notification from CLIAIMONITOR\n")

	return body.String()
}

// buildMessage creates the full email message with headers
func (e *EmailNotifier) buildMessage(subject, body string) string {
	var message strings.Builder

	message.WriteString(fmt.Sprintf("From: %s\r\n", e.config.From))
	message.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(e.config.To, ", ")))
	message.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	message.WriteString("MIME-Version: 1.0\r\n")
	message.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
	message.WriteString("\r\n")
	message.WriteString(body)

	return message.String()
}
