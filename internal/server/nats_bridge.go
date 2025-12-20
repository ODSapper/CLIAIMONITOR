package server

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/CLIAIMONITOR/internal/events"
	natslib "github.com/CLIAIMONITOR/internal/nats"
	"github.com/CLIAIMONITOR/internal/types"
)

// NATSBridge connects NATS messaging to server state management
type NATSBridge struct {
	server  *Server
	handler *natslib.Handler
}

// NewNATSBridge creates a bridge between NATS and server state
func NewNATSBridge(s *Server, client *natslib.Client) *NATSBridge {
	bridge := &NATSBridge{
		server: s,
	}

	callbacks := natslib.HandlerCallbacks{
		OnHeartbeat:         bridge.handleHeartbeat,
		OnStatusUpdate:      bridge.handleStatusUpdate,
		OnToolCall:          bridge.handleToolCall,
		OnStopApproval:      bridge.handleStopApproval,
		OnShutdownNotify:    bridge.handleShutdownNotify,
		OnCaptainStatus:     bridge.handleCaptainStatus,
		OnCaptainCommand:    bridge.handleCaptainCommand,
		OnEscalationForward: bridge.handleEscalationForward,
		OnSystemBroadcast:   bridge.handleSystemBroadcast,
	}

	bridge.handler = natslib.NewHandler(client, callbacks)
	return bridge
}

// Start begins processing NATS messages
func (b *NATSBridge) Start() error {
	// Start standard NATS message handlers
	if err := b.handler.Start(); err != nil {
		return err
	}

	// Setup chat bridge for NATS chat messaging
	return b.setupChatBridge()
}

// Stop terminates message processing
func (b *NATSBridge) Stop() {
	b.handler.Stop()
}

// handleHeartbeat processes agent heartbeats via NATS
func (b *NATSBridge) handleHeartbeat(agentID, status, task, configName, projectPath string) error {
	log.Printf("[NATS-BRIDGE] Heartbeat from %s: status=%s task=%s", agentID, status, task)

	// Use atomic update to keep stores in sync
	if err := b.server.atomicAgentUpdate(agentID, status, task); err != nil {
		log.Printf("[NATS-BRIDGE] ERROR: Failed to update agent %s: %v", agentID, err)
		// Don't fail heartbeat processing, just log the error
	}

	// Update additional fields that aren't in atomicAgentUpdate
	if configName != "" || projectPath != "" {
		b.server.store.UpdateAgent(agentID, func(a *types.Agent) {
			if configName != "" {
				a.ConfigName = configName
			}
			if projectPath != "" {
				a.ProjectPath = projectPath
			}
		})
	}

	// Publish agent signals to event bus
	if b.server.eventBus != nil && (status == "blocked" || status == "error") {
		event := events.NewEvent(
			events.EventAgentSignal,
			agentID,
			"Captain",
			events.PriorityHigh,
			map[string]interface{}{
				"signal": status,
				"task":   task,
			},
		)
		b.server.eventBus.Publish(event)
		log.Printf("[NATS-BRIDGE] Published agent signal to bus: %s status=%s", agentID, status)
	}

	b.server.broadcastState()
	return nil
}

// handleStatusUpdate processes status changes via NATS
func (b *NATSBridge) handleStatusUpdate(agentID, status, message string) error {
	log.Printf("[NATS-BRIDGE] Status update from %s: %s - %s", agentID, status, message)

	// Use atomic update to keep stores in sync
	if err := b.server.atomicAgentUpdate(agentID, status, message); err != nil {
		log.Printf("[NATS-BRIDGE] ERROR: Failed to update agent %s status: %v", agentID, err)
		// Don't fail status update processing, just log the error
	}

	// Update metrics idle tracking
	if status == string(types.StatusIdle) {
		b.server.metrics.SetAgentIdle(agentID)
	} else {
		b.server.metrics.SetAgentActive(agentID)
	}

	b.server.broadcastState()
	return nil
}

// handleToolCall processes tool calls via NATS request-reply
func (b *NATSBridge) handleToolCall(agentID, tool string, args map[string]interface{}) (interface{}, error) {
	log.Printf("[NATS-BRIDGE] Tool call from %s: %s", agentID, tool)

	// Delegate to MCP tool registry (placeholder - will be wired to actual tools)
	// For now, return error indicating tool not found via NATS
	return nil, fmt.Errorf("tool %s not yet available via NATS (use MCP SSE for now)", tool)
}

// handleStopApproval processes stop approval requests via NATS
func (b *NATSBridge) handleStopApproval(agentID, reason, context string, workCompleted bool) (bool, string, error) {
	log.Printf("[NATS-BRIDGE] Stop approval request from %s: %s (work_completed=%v)", agentID, reason, workCompleted)

	// Check if agent already has approved shutdown
	state := b.server.store.GetState()
	if agent, ok := state.Agents[agentID]; ok {
		if agent.ShutdownRequested {
			return true, "shutdown already approved", nil
		}
	}

	// Create stop request for human approval
	req := &types.StopApprovalRequest{
		ID:        fmt.Sprintf("nats-stop-%d", time.Now().UnixNano()),
		AgentID:   agentID,
		Reason:    fmt.Sprintf("%s (context: %s, work_completed: %v)", reason, context, workCompleted),
		CreatedAt: time.Now(),
		Reviewed:  false,
	}
	b.server.store.AddStopRequest(req)

	// Alert via WebSocket
	b.server.hub.BroadcastAlert(&types.Alert{
		ID:        req.ID,
		Type:      "stop_approval_needed",
		AgentID:   agentID,
		Message:   fmt.Sprintf("[NATS] Agent %s wants to stop: %s", agentID, reason),
		Severity:  "warning",
		CreatedAt: time.Now(),
	})

	b.server.broadcastState()

	// Return pending - agent should poll or wait for notification
	return false, "pending_approval", nil
}

// handleShutdownNotify processes shutdown notifications
func (b *NATSBridge) handleShutdownNotify(agentID, reason string, approved, force bool) error {
	log.Printf("[NATS-BRIDGE] Shutdown notification from %s: reason=%s approved=%v force=%v", agentID, reason, approved, force)

	// Use atomic update to keep stores in sync
	if err := b.server.atomicAgentUpdate(agentID, "disconnected", fmt.Sprintf("shutdown: %s", reason)); err != nil {
		log.Printf("[NATS-BRIDGE] ERROR: Failed to update agent %s shutdown status: %v", agentID, err)
	}

	b.server.broadcastState()
	return nil
}

// handleCaptainStatus processes Captain status updates
func (b *NATSBridge) handleCaptainStatus(status, currentOp string, queueSize int) error {
	log.Printf("[NATS-BRIDGE] Captain status update: status=%s currentOp=%s queueSize=%d", status, currentOp, queueSize)

	// Update Captain status in store
	b.server.store.SetCaptainStatus(status)

	// Mark Captain as connected when we receive status updates
	b.server.store.SetCaptainConnected(true)

	// Broadcast updated state to dashboard
	b.server.broadcastState()
	return nil
}

// handleCaptainCommand processes commands from dashboard to Captain
func (b *NATSBridge) handleCaptainCommand(cmdType string, payload map[string]interface{}, from string) error {
	log.Printf("[NATS-BRIDGE] Captain command received: type=%s from=%s", cmdType, from)

	// Extract text from payload for message types
	text := ""
	if payload != nil {
		if t, ok := payload["text"].(string); ok {
			text = t
		}
	}

	// Create CaptainMessage and store it
	msg := &types.CaptainMessage{
		ID:        fmt.Sprintf("capmsg-%d", time.Now().UnixNano()),
		Type:      cmdType,
		Text:      text,
		Payload:   payload,
		From:      from,
		CreatedAt: time.Now(),
		Read:      false,
	}

	b.server.store.AddCaptainMessage(msg)
	log.Printf("[NATS-BRIDGE] Stored captain message: id=%s type=%s", msg.ID, msg.Type)

	// Publish to event bus for real-time delivery
	if b.server.eventBus != nil {
		event := events.NewEvent(
			events.EventMessage,
			from,
			"Captain",
			events.PriorityNormal,
			map[string]interface{}{
				"message_id": msg.ID,
				"type":       cmdType,
				"text":       text,
				"payload":    payload,
			},
		)
		b.server.eventBus.Publish(event)
		log.Printf("[NATS-BRIDGE] Published event to bus: %s", event.ID)
	}

	// Push notify Captain via MCP SSE
	if b.server.mcp != nil {
		notification := map[string]interface{}{
			"id":         msg.ID,
			"type":       msg.Type,
			"text":       msg.Text,
			"payload":    msg.Payload,
			"from":       msg.From,
			"created_at": msg.CreatedAt,
		}
		if err := b.server.mcp.NotifyAgent("Captain", "captain/message", notification); err != nil {
			log.Printf("[NATS-BRIDGE] Failed to notify Captain via MCP: %v", err)
		} else {
			log.Printf("[NATS-BRIDGE] Notified Captain via MCP: id=%s", msg.ID)
		}
	}

	// Broadcast updated state to dashboard
	b.server.broadcastState()
	return nil
}

// handleEscalationForward processes escalations forwarded by Captain to human
func (b *NATSBridge) handleEscalationForward(id, agentID, question, captainContext, captainRecommends string) error {
	log.Printf("[NATS-BRIDGE] Escalation forwarded from Captain: id=%s agent=%s", id, agentID)

	// Create alert for dashboard
	alert := &types.Alert{
		ID:        id,
		Type:      "escalation",
		AgentID:   agentID,
		Message:   fmt.Sprintf("Question from agent %s: %s", agentID, question),
		Severity:  "info",
		CreatedAt: time.Now(),
	}

	// If captain provided context or recommendations, add to message
	if captainContext != "" {
		alert.Message += fmt.Sprintf("\n\nCaptain Context: %s", captainContext)
	}
	if captainRecommends != "" {
		alert.Message += fmt.Sprintf("\n\nCaptain Recommends: %s", captainRecommends)
	}

	// Broadcast alert to dashboard via WebSocket
	b.server.hub.BroadcastAlert(alert)

	// Also broadcast state update
	b.server.broadcastState()
	return nil
}

// handleSystemBroadcast processes system-wide broadcast messages
func (b *NATSBridge) handleSystemBroadcast(msgType, message string, data map[string]interface{}) error {
	log.Printf("[NATS-BRIDGE] System broadcast: type=%s message=%s", msgType, message)

	// Create alert for dashboard based on broadcast type
	severity := "info"
	if msgType == "shutdown" || msgType == "agent_killed" {
		severity = "warning"
	}

	alert := &types.Alert{
		ID:        fmt.Sprintf("broadcast-%d", time.Now().UnixNano()),
		Type:      msgType,
		Message:   message,
		Severity:  severity,
		CreatedAt: time.Now(),
	}

	// Extract agent ID if present in data
	if agentID, ok := data["agent_id"].(string); ok {
		alert.AgentID = agentID
	}

	// Broadcast alert to dashboard
	b.server.hub.BroadcastAlert(alert)

	return nil
}

// setupChatBridge subscribes to NATS chat topics and bridges them to WebSocket
func (b *NATSBridge) setupChatBridge() error {
	client := b.handler.GetClient()
	if client == nil {
		return fmt.Errorf("NATS client not available")
	}

	// Subscribe to chat.dashboard - messages intended for dashboard
	_, err := client.Subscribe("chat.dashboard", func(msg *natslib.Message) {
		var chatMsg types.ChatMessage
		if err := json.Unmarshal(msg.Data, &chatMsg); err != nil {
			log.Printf("[CHAT-BRIDGE] Failed to unmarshal chat.dashboard message: %v", err)
			return
		}

		log.Printf("[CHAT-BRIDGE] Received chat.dashboard message: from=%s to=%s type=%s",
			chatMsg.From, chatMsg.To, chatMsg.Type)

		// Forward to WebSocket hub
		b.server.hub.BroadcastChat(&chatMsg)
	})
	if err != nil {
		return fmt.Errorf("failed to subscribe to chat.dashboard: %w", err)
	}
	log.Printf("[CHAT-BRIDGE] Subscribed to chat.dashboard")

	// Subscribe to chat.broadcast - messages for all connected clients
	_, err = client.Subscribe("chat.broadcast", func(msg *natslib.Message) {
		var chatMsg types.ChatMessage
		if err := json.Unmarshal(msg.Data, &chatMsg); err != nil {
			log.Printf("[CHAT-BRIDGE] Failed to unmarshal chat.broadcast message: %v", err)
			return
		}

		log.Printf("[CHAT-BRIDGE] Received chat.broadcast message: from=%s type=%s text=%s",
			chatMsg.From, chatMsg.Type, chatMsg.Text)

		// Forward to all WebSocket clients
		b.server.hub.BroadcastChat(&chatMsg)
	})
	if err != nil {
		return fmt.Errorf("failed to subscribe to chat.broadcast: %w", err)
	}
	log.Printf("[CHAT-BRIDGE] Subscribed to chat.broadcast")

	// Subscribe to presence.> - trigger state broadcast on presence changes
	_, err = client.Subscribe("presence.>", func(msg *natslib.Message) {
		log.Printf("[CHAT-BRIDGE] Received presence update on %s", msg.Subject)

		// Broadcast updated state to dashboard when presence changes
		b.server.broadcastState()
	})
	if err != nil {
		return fmt.Errorf("failed to subscribe to presence.>: %w", err)
	}
	log.Printf("[CHAT-BRIDGE] Subscribed to presence.>")

	log.Printf("[CHAT-BRIDGE] Chat bridge setup complete")
	return nil
}
