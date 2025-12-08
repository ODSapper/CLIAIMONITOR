package external

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/CLIAIMONITOR/internal/events"
)

func TestEmailNotifier_Name(t *testing.T) {
	notifier := NewEmailNotifier(EmailConfig{})
	if notifier.Name() != "email" {
		t.Errorf("expected name 'email', got '%s'", notifier.Name())
	}
}

func TestEmailNotifier_ShouldNotify(t *testing.T) {
	tests := []struct {
		name     string
		config   EmailConfig
		event    events.Event
		expected bool
	}{
		{
			name:   "no filters - should notify",
			config: EmailConfig{},
			event: events.Event{
				Type:     events.EventAlert,
				Priority: events.PriorityNormal,
			},
			expected: true,
		},
		{
			name: "priority filter - event too low",
			config: EmailConfig{
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
			config: EmailConfig{
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
			config: EmailConfig{
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
			config: EmailConfig{
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
			config: EmailConfig{
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
			config: EmailConfig{
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
			config: EmailConfig{
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
			notifier := NewEmailNotifier(tt.config)
			result := notifier.ShouldNotify(tt.event)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestEmailNotifier_buildSubject(t *testing.T) {
	tests := []struct {
		name     string
		event    events.Event
		expected string
	}{
		{
			name: "critical priority",
			event: events.Event{
				ID:       "crit-123",
				Type:     events.EventAlert,
				Priority: events.PriorityCritical,
			},
			expected: "[CRITICAL] CLIAIMONITOR alert Event - crit-123",
		},
		{
			name: "high priority",
			event: events.Event{
				ID:       "high-456",
				Type:     events.EventTask,
				Priority: events.PriorityHigh,
			},
			expected: "[HIGH] CLIAIMONITOR task Event - high-456",
		},
		{
			name: "normal priority",
			event: events.Event{
				ID:       "norm-789",
				Type:     events.EventMessage,
				Priority: events.PriorityNormal,
			},
			expected: "CLIAIMONITOR message Event - norm-789",
		},
		{
			name: "low priority",
			event: events.Event{
				ID:       "low-999",
				Type:     events.EventRecon,
				Priority: events.PriorityLow,
			},
			expected: "CLIAIMONITOR recon Event - low-999",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notifier := NewEmailNotifier(EmailConfig{})
			subject := notifier.buildSubject(tt.event)
			if subject != tt.expected {
				t.Errorf("expected subject '%s', got '%s'", tt.expected, subject)
			}
		})
	}
}

func TestEmailNotifier_buildBody(t *testing.T) {
	event := events.Event{
		ID:       "test-123",
		Type:     events.EventAlert,
		Source:   "captain",
		Target:   "agent-1",
		Priority: events.PriorityCritical,
		Payload: map[string]interface{}{
			"message": "Test message",
			"count":   42,
		},
		CreatedAt: time.Date(2025, 12, 8, 12, 0, 0, 0, time.UTC),
	}

	notifier := NewEmailNotifier(EmailConfig{})
	body := notifier.buildBody(event)

	// Check for required content
	requiredStrings := []string{
		"CLIAIMONITOR Event Notification",
		"Event ID: test-123",
		"Type: alert",
		"Source: captain",
		"Target: agent-1",
		"Priority: Critical",
		"Payload:",
		"automated notification",
	}

	for _, required := range requiredStrings {
		if !strings.Contains(body, required) {
			t.Errorf("body missing required string: %s", required)
		}
	}

	// Check payload fields present
	if !strings.Contains(body, "message:") && !strings.Contains(body, "count:") {
		t.Error("body missing payload fields")
	}
}

func TestEmailNotifier_buildMessage(t *testing.T) {
	notifier := NewEmailNotifier(EmailConfig{
		From: "sender@example.com",
		To:   []string{"recipient1@example.com", "recipient2@example.com"},
	})

	subject := "Test Subject"
	body := "Test Body"

	message := notifier.buildMessage(subject, body)

	// Check headers
	requiredHeaders := []string{
		"From: sender@example.com",
		"To: recipient1@example.com, recipient2@example.com",
		"Subject: Test Subject",
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=utf-8",
	}

	for _, header := range requiredHeaders {
		if !strings.Contains(message, header) {
			t.Errorf("message missing required header: %s", header)
		}
	}

	// Check body is present
	if !strings.Contains(message, "Test Body") {
		t.Error("message missing body content")
	}
}

func TestEmailNotifier_Send_MissingConfig(t *testing.T) {
	tests := []struct {
		name   string
		config EmailConfig
	}{
		{
			name:   "missing SMTP host",
			config: EmailConfig{
				From: "test@example.com",
				To:   []string{"recipient@example.com"},
			},
		},
		{
			name:   "missing from address",
			config: EmailConfig{
				SMTPHost: "smtp.example.com",
				SMTPPort: 25,
				To:       []string{"recipient@example.com"},
			},
		},
		{
			name:   "missing recipients",
			config: EmailConfig{
				SMTPHost: "smtp.example.com",
				SMTPPort: 25,
				From:     "test@example.com",
				To:       []string{},
			},
		},
	}

	event := events.Event{
		ID:       "test-1",
		Type:     events.EventAlert,
		Source:   "test",
		Priority: events.PriorityNormal,
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notifier := NewEmailNotifier(tt.config)
			err := notifier.Send(event)
			if err == nil {
				t.Error("expected error for missing config")
			}
		})
	}
}

func TestEmailNotifier_Send(t *testing.T) {
	// Create a mock SMTP server
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start mock SMTP server: %v", err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port

	// Channel to signal when message received
	messageChan := make(chan string, 1)

	// Start mock SMTP server
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		reader := bufio.NewReader(conn)
		writer := bufio.NewWriter(conn)

		// SMTP conversation
		writer.WriteString("220 localhost SMTP Mock\r\n")
		writer.Flush()

		var messageData strings.Builder
		inData := false

		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				break
			}

			if inData {
				if strings.TrimSpace(line) == "." {
					messageChan <- messageData.String()
					writer.WriteString("250 OK\r\n")
					writer.Flush()
					inData = false
				} else {
					messageData.WriteString(line)
				}
				continue
			}

			if strings.HasPrefix(line, "HELO") || strings.HasPrefix(line, "EHLO") {
				writer.WriteString("250 Hello\r\n")
			} else if strings.HasPrefix(line, "MAIL FROM:") {
				writer.WriteString("250 OK\r\n")
			} else if strings.HasPrefix(line, "RCPT TO:") {
				writer.WriteString("250 OK\r\n")
			} else if strings.HasPrefix(line, "DATA") {
				writer.WriteString("354 Start mail input\r\n")
				inData = true
			} else if strings.HasPrefix(line, "QUIT") {
				writer.WriteString("221 Bye\r\n")
				writer.Flush()
				break
			}
			writer.Flush()
		}
	}()

	// Create notifier
	notifier := NewEmailNotifier(EmailConfig{
		SMTPHost: "127.0.0.1",
		SMTPPort: port,
		From:     "sender@example.com",
		To:       []string{"recipient@example.com"},
	})

	// Create and send event
	event := events.Event{
		ID:       "test-123",
		Type:     events.EventAlert,
		Source:   "captain",
		Priority: events.PriorityCritical,
		Payload: map[string]interface{}{
			"message": "Test alert",
		},
		CreatedAt: time.Now(),
	}

	err = notifier.Send(event)
	if err != nil {
		t.Fatalf("failed to send email: %v", err)
	}

	// Wait for message
	select {
	case message := <-messageChan:
		// Verify message contains expected content
		if !strings.Contains(message, "From: sender@example.com") {
			t.Error("message missing From header")
		}
		if !strings.Contains(message, "To: recipient@example.com") {
			t.Error("message missing To header")
		}
		if !strings.Contains(message, "[CRITICAL]") {
			t.Error("message missing CRITICAL prefix in subject")
		}
		if !strings.Contains(message, "test-123") {
			t.Error("message missing event ID")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for email")
	}
}

func TestEmailNotifier_Send_WithAuth(t *testing.T) {
	// This test verifies that auth credentials are used when configured
	// We can't easily test actual SMTP auth without a real server,
	// but we can verify the config is accepted
	config := EmailConfig{
		SMTPHost: "smtp.example.com",
		SMTPPort: 587,
		Username: "testuser",
		Password: "testpass",
		From:     "sender@example.com",
		To:       []string{"recipient@example.com"},
	}

	notifier := NewEmailNotifier(config)
	if notifier.config.Username != "testuser" {
		t.Error("username not stored correctly")
	}
	if notifier.config.Password != "testpass" {
		t.Error("password not stored correctly")
	}
}

func TestEmailNotifier_Send_Integration(t *testing.T) {
	// Integration test with different event types and priorities
	tests := []struct {
		name            string
		event           events.Event
		expectedPrefix  string
	}{
		{
			name: "critical alert",
			event: events.Event{
				ID:       "crit-1",
				Type:     events.EventAlert,
				Priority: events.PriorityCritical,
			},
			expectedPrefix: "[CRITICAL]",
		},
		{
			name: "high priority task",
			event: events.Event{
				ID:       "high-2",
				Type:     events.EventTask,
				Priority: events.PriorityHigh,
			},
			expectedPrefix: "[HIGH]",
		},
		{
			name: "normal message",
			event: events.Event{
				ID:       "norm-3",
				Type:     events.EventMessage,
				Priority: events.PriorityNormal,
			},
			expectedPrefix: "CLIAIMONITOR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notifier := NewEmailNotifier(EmailConfig{
				From: "test@example.com",
				To:   []string{"recipient@example.com"},
			})

			tt.event.CreatedAt = time.Now()
			subject := notifier.buildSubject(tt.event)

			if !strings.HasPrefix(subject, tt.expectedPrefix) {
				t.Errorf("expected subject to start with '%s', got '%s'", tt.expectedPrefix, subject)
			}
		})
	}
}

// Helper to test priority string formatting
func TestPriorityString(t *testing.T) {
	tests := []struct {
		priority int
		expected string
	}{
		{events.PriorityCritical, "Critical"},
		{events.PriorityHigh, "High"},
		{events.PriorityNormal, "Normal"},
		{events.PriorityLow, "Low"},
		{999, fmt.Sprintf("Unknown (%d)", 999)},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := priorityString(tt.priority)
			if result != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}
