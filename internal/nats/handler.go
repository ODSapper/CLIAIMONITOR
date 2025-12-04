package nats

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
)

// HandlerCallbacks defines callbacks the handler uses to communicate with the server
type HandlerCallbacks struct {
	OnHeartbeat      func(agentID, status, task, configName, projectPath string) error
	OnStatusUpdate   func(agentID, status, message string) error
	OnToolCall       func(agentID, tool string, args map[string]interface{}) (interface{}, error)
	OnStopApproval   func(agentID, reason, context string, workCompleted bool) (bool, string, error)
	OnShutdownNotify func(agentID, reason string, approved, force bool) error
}

// Handler processes NATS messages and delegates to callbacks
type Handler struct {
	client    *Client
	callbacks HandlerCallbacks

	// Track subscriptions for cleanup
	subs   []*nats.Subscription
	subsMu sync.Mutex

	// Running state
	running bool
	stopCh  chan struct{}
}

// NewHandler creates a new NATS message handler
func NewHandler(client *Client, callbacks HandlerCallbacks) *Handler {
	return &Handler{
		client:    client,
		callbacks: callbacks,
		subs:      make([]*nats.Subscription, 0),
		stopCh:    make(chan struct{}),
	}
}

// Start begins processing NATS messages
func (h *Handler) Start() error {
	if h.running {
		return fmt.Errorf("handler already running")
	}

	h.running = true

	// Subscribe to heartbeat messages from all agents
	sub, err := h.client.Subscribe(SubjectAllHeartbeats, h.handleHeartbeat)
	if err != nil {
		return fmt.Errorf("failed to subscribe to heartbeats: %w", err)
	}
	h.addSub(sub)

	// Subscribe to status updates from all agents
	sub, err = h.client.Subscribe(SubjectAllStatus, h.handleStatus)
	if err != nil {
		return fmt.Errorf("failed to subscribe to status: %w", err)
	}
	h.addSub(sub)

	// Subscribe to tool calls (use queue group for load balancing)
	sub, err = h.client.QueueSubscribe(SubjectToolCall, "tool-workers", h.handleToolCall)
	if err != nil {
		return fmt.Errorf("failed to subscribe to tool calls: %w", err)
	}
	h.addSub(sub)

	log.Printf("[NATS-HANDLER] Started, subscribed to %d subjects", len(h.subs))
	return nil
}

// Stop terminates message processing
func (h *Handler) Stop() {
	if !h.running {
		return
	}

	close(h.stopCh)

	h.subsMu.Lock()
	for _, sub := range h.subs {
		sub.Unsubscribe()
	}
	h.subs = nil
	h.subsMu.Unlock()

	h.running = false
	log.Printf("[NATS-HANDLER] Stopped")
}

func (h *Handler) addSub(sub *nats.Subscription) {
	h.subsMu.Lock()
	h.subs = append(h.subs, sub)
	h.subsMu.Unlock()
}

// handleHeartbeat processes agent heartbeat messages
func (h *Handler) handleHeartbeat(msg *Message) {
	var hb HeartbeatMessage
	if err := json.Unmarshal(msg.Data, &hb); err != nil {
		log.Printf("[NATS-HANDLER] Invalid heartbeat message: %v", err)
		return
	}

	if h.callbacks.OnHeartbeat != nil {
		if err := h.callbacks.OnHeartbeat(hb.AgentID, hb.Status, hb.CurrentTask, hb.ConfigName, hb.ProjectPath); err != nil {
			log.Printf("[NATS-HANDLER] Heartbeat callback error: %v", err)
		}
	}
}

// handleStatus processes agent status updates
func (h *Handler) handleStatus(msg *Message) {
	var status StatusMessage
	if err := json.Unmarshal(msg.Data, &status); err != nil {
		log.Printf("[NATS-HANDLER] Invalid status message: %v", err)
		return
	}

	if h.callbacks.OnStatusUpdate != nil {
		if err := h.callbacks.OnStatusUpdate(status.AgentID, status.Status, status.Message); err != nil {
			log.Printf("[NATS-HANDLER] Status callback error: %v", err)
		}
	}
}

// handleToolCall processes tool call requests with reply
func (h *Handler) handleToolCall(msg *Message) {
	var req ToolCallRequest
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		h.replyError(msg.Reply, "invalid request format")
		return
	}

	if h.callbacks.OnToolCall == nil {
		h.replyError(msg.Reply, "no tool handler configured")
		return
	}

	result, err := h.callbacks.OnToolCall(req.AgentID, req.Tool, req.Arguments)

	resp := ToolCallResponse{
		RequestID: req.RequestID,
		Success:   err == nil,
		Result:    result,
	}

	if err != nil {
		resp.Error = err.Error()
	}

	h.reply(msg.Reply, resp)
}

// reply sends a JSON response to a reply subject
func (h *Handler) reply(subject string, data interface{}) {
	if subject == "" {
		return
	}
	if err := h.client.PublishJSON(subject, data); err != nil {
		log.Printf("[NATS-HANDLER] Failed to send reply: %v", err)
	}
}

// replyError sends an error response
func (h *Handler) replyError(subject string, errMsg string) {
	if subject == "" {
		return
	}
	resp := map[string]interface{}{
		"error":     errMsg,
		"timestamp": time.Now(),
	}
	h.client.PublishJSON(subject, resp)
}
