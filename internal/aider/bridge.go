package aider

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
)

// Bridge connects an Aider CLI process to NATS messaging
type Bridge struct {
	agentID     string
	status      string
	currentTask string

	// Process I/O
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser

	// NATS connection
	natsConn  *nats.Conn
	connected bool
	mu        sync.RWMutex

	// Control
	stopCh chan struct{}
}

// StatusMessage represents an agent status update published to NATS
type StatusMessage struct {
	AgentID     string    `json:"agent_id"`
	Status      string    `json:"status"`
	CurrentTask string    `json:"current_task"`
	Timestamp   time.Time `json:"timestamp"`
}

// CommandMessage represents a command sent to an agent via NATS
type CommandMessage struct {
	Type    string                 `json:"type"`
	Payload map[string]interface{} `json:"payload"`
}

// NewBridge creates a new Aider-NATS bridge
func NewBridge(agentID string, nc *nats.Conn, stdin io.WriteCloser, stdout, stderr io.ReadCloser) *Bridge {
	return &Bridge{
		agentID:   agentID,
		natsConn:  nc,
		stdin:     stdin,
		stdout:    stdout,
		stderr:    stderr,
		status:    "starting",
		connected: false,
		stopCh:    make(chan struct{}),
	}
}

// Start begins bridging Aider I/O to NATS
func (b *Bridge) Start() error {
	// Subscribe to commands for this agent
	subject := fmt.Sprintf("agent.%s.command", b.agentID)
	_, err := b.natsConn.Subscribe(subject, b.handleCommand)
	if err != nil {
		return fmt.Errorf("failed to subscribe to commands: %w", err)
	}

	// Start output parsing goroutines
	go b.parseAiderOutput()
	go b.parseAiderErrors()

	// Mark as connected and publish initial status
	b.mu.Lock()
	b.connected = true
	b.status = "connected"
	b.currentTask = "Aider ready"
	b.mu.Unlock()

	b.publishStatus("connected", "Aider ready")

	log.Printf("[BRIDGE] Started for agent %s", b.agentID)
	return nil
}

// Stop terminates the bridge and cleans up resources
func (b *Bridge) Stop() {
	select {
	case <-b.stopCh:
		// Already stopped
		return
	default:
		close(b.stopCh)
	}

	b.mu.Lock()
	b.connected = false
	b.mu.Unlock()

	// Send quit command to Aider
	if b.stdin != nil {
		fmt.Fprintln(b.stdin, "/quit")
		b.stdin.Close()
	}

	// Close readers
	if b.stdout != nil {
		b.stdout.Close()
	}
	if b.stderr != nil {
		b.stderr.Close()
	}

	b.publishStatus("disconnected", "Bridge stopped")
	log.Printf("[BRIDGE] Stopped for agent %s", b.agentID)
}

// parseAiderOutput continuously reads and parses stdout from Aider
func (b *Bridge) parseAiderOutput() {
	scanner := bufio.NewScanner(b.stdout)
	for scanner.Scan() {
		select {
		case <-b.stopCh:
			return
		default:
		}

		line := scanner.Text()
		b.parseAiderLine(line)

		// Publish raw output for logging
		b.publishOutput("stdout", line)
	}

	if err := scanner.Err(); err != nil {
		log.Printf("[BRIDGE] Stdout scanner error: %v", err)
	}

	// Process ended
	b.mu.Lock()
	b.connected = false
	b.mu.Unlock()
	b.publishStatus("disconnected", "Aider process ended")
}

// parseAiderErrors continuously reads stderr from Aider
func (b *Bridge) parseAiderErrors() {
	scanner := bufio.NewScanner(b.stderr)
	for scanner.Scan() {
		select {
		case <-b.stopCh:
			return
		default:
		}

		line := scanner.Text()
		b.publishOutput("stderr", line)

		// Check for errors and update status
		if strings.Contains(strings.ToLower(line), "error") {
			b.publishStatus("error", line)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("[BRIDGE] Stderr scanner error: %v", err)
	}
}

// parseAiderLine interprets Aider output patterns to determine status
func (b *Bridge) parseAiderLine(line string) {
	lower := strings.ToLower(line)
	trimmed := strings.TrimSpace(line)

	var newStatus, newTask string

	switch {
	case strings.Contains(lower, "thinking"):
		newStatus = "working"
		newTask = "Thinking..."

	case strings.Contains(lower, "applied edit"):
		newStatus = "working"
		newTask = "Applied edit to files"

	case strings.HasPrefix(trimmed, ">"):
		// Aider prompt indicates ready for input
		newStatus = "idle"
		newTask = "Awaiting prompt"

	case strings.Contains(lower, "error"):
		newStatus = "error"
		newTask = line

	case strings.Contains(lower, "commit"):
		newStatus = "working"
		newTask = "Committing changes"

	case strings.Contains(lower, "added"):
		newStatus = "working"
		newTask = "Adding files to context"

	case strings.Contains(lower, "searching"):
		newStatus = "working"
		newTask = "Searching codebase"

	case strings.Contains(lower, "reading"):
		newStatus = "working"
		newTask = "Reading files"

	default:
		// No status change for unrecognized patterns
		return
	}

	// Update status and publish
	b.mu.Lock()
	b.status = newStatus
	b.currentTask = newTask
	b.mu.Unlock()

	b.publishStatus(newStatus, newTask)
}

// handleCommand processes incoming commands from NATS
func (b *Bridge) handleCommand(msg *nats.Msg) {
	var cmd CommandMessage

	if err := json.Unmarshal(msg.Data, &cmd); err != nil {
		log.Printf("[BRIDGE] Invalid command JSON: %v", err)
		return
	}

	log.Printf("[BRIDGE] Received command: %s for agent %s", cmd.Type, b.agentID)

	switch cmd.Type {
	case "prompt":
		// Send user prompt to Aider
		if text, ok := cmd.Payload["text"].(string); ok {
			b.mu.Lock()
			b.currentTask = text
			b.mu.Unlock()

			b.publishStatus("working", "Processing prompt")
			fmt.Fprintln(b.stdin, text)
		}

	case "stop":
		// Send /quit command to Aider
		fmt.Fprintln(b.stdin, "/quit")
		b.publishStatus("stopping", "Quitting Aider")

	case "clear":
		// Clear Aider's chat history
		fmt.Fprintln(b.stdin, "/clear")
		b.publishStatus("working", "Clearing chat history")

	case "add":
		// Add file to Aider's context
		if file, ok := cmd.Payload["file"].(string); ok {
			fmt.Fprintf(b.stdin, "/add %s\n", file)
			b.publishStatus("working", fmt.Sprintf("Adding file: %s", file))
		}

	case "drop":
		// Remove file from Aider's context
		if file, ok := cmd.Payload["file"].(string); ok {
			fmt.Fprintf(b.stdin, "/drop %s\n", file)
			b.publishStatus("working", fmt.Sprintf("Dropping file: %s", file))
		}

	default:
		log.Printf("[BRIDGE] Unknown command type: %s", cmd.Type)
	}
}

// publishStatus publishes a status update to NATS
func (b *Bridge) publishStatus(status, task string) {
	b.mu.Lock()
	b.status = status
	b.currentTask = task
	b.mu.Unlock()

	msg := StatusMessage{
		AgentID:     b.agentID,
		Status:      status,
		CurrentTask: task,
		Timestamp:   time.Now(),
	}

	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("[BRIDGE] Failed to marshal status: %v", err)
		return
	}

	subject := fmt.Sprintf("agent.%s.status", b.agentID)
	if err := b.natsConn.Publish(subject, data); err != nil {
		log.Printf("[BRIDGE] Failed to publish status: %v", err)
	}
}

// publishOutput publishes raw output to NATS for logging and monitoring
func (b *Bridge) publishOutput(stream, line string) {
	msg := map[string]interface{}{
		"agent_id":  b.agentID,
		"stream":    stream,
		"line":      line,
		"timestamp": time.Now(),
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	subject := fmt.Sprintf("agent.%s.output", b.agentID)
	b.natsConn.Publish(subject, data)
}

// GetStatus returns the current agent status (thread-safe)
func (b *Bridge) GetStatus() (string, string) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.status, b.currentTask
}

// IsConnected returns true if the bridge is active
func (b *Bridge) IsConnected() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.connected
}

// SendPrompt sends a prompt to Aider via stdin
func (b *Bridge) SendPrompt(prompt string) error {
	b.mu.Lock()
	b.currentTask = prompt
	b.mu.Unlock()

	b.publishStatus("working", "Processing prompt")

	_, err := fmt.Fprintln(b.stdin, prompt)
	return err
}

// SendCommand sends a special command to Aider (e.g., /quit, /clear)
func (b *Bridge) SendCommand(cmd string) error {
	_, err := fmt.Fprintln(b.stdin, cmd)
	return err
}
