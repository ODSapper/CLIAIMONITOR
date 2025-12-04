package nats

import (
	"encoding/json"
	"sync"
	"testing"
	"time"
)

// TestNATSIntegration_HeartbeatFlow tests the complete heartbeat flow via NATS
func TestNATSIntegration_HeartbeatFlow(t *testing.T) {
	// Start embedded server
	config := EmbeddedServerConfig{
		Port: 14300,
	}
	server, err := NewEmbeddedServer(config)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Shutdown()

	// Create monitor client (simulates CLIAIMONITOR server)
	monitor, err := NewClient(server.URL())
	if err != nil {
		t.Fatalf("Failed to create monitor client: %v", err)
	}
	defer monitor.Close()

	// Create agent client (simulates Claude agent)
	agent, err := NewClient(server.URL())
	if err != nil {
		t.Fatalf("Failed to create agent client: %v", err)
	}
	defer agent.Close()

	// Track received heartbeats
	var receivedHeartbeats []HeartbeatMessage
	var mu sync.Mutex

	// Monitor subscribes to all heartbeats
	_, err = monitor.Subscribe(SubjectAllHeartbeats, func(msg *Message) {
		var hb HeartbeatMessage
		if err := json.Unmarshal(msg.Data, &hb); err != nil {
			t.Errorf("Failed to unmarshal heartbeat: %v", err)
			return
		}
		mu.Lock()
		receivedHeartbeats = append(receivedHeartbeats, hb)
		mu.Unlock()
	})
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	// Agent sends heartbeats
	for i := 0; i < 3; i++ {
		hb := HeartbeatMessage{
			AgentID:     "test-agent-001",
			ConfigName:  "go-developer",
			ProjectPath: "C:\\test\\project",
			Status:      "working",
			CurrentTask: "Running tests",
			Timestamp:   time.Now(),
		}

		subject := "agent.test-agent-001.heartbeat"
		if err := agent.PublishJSON(subject, hb); err != nil {
			t.Errorf("Failed to publish heartbeat: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Wait for messages to be received
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	count := len(receivedHeartbeats)
	mu.Unlock()

	if count != 3 {
		t.Errorf("Expected 3 heartbeats, got %d", count)
	}
}

// TestNATSIntegration_ToolCallRequestReply tests tool call request-reply pattern
func TestNATSIntegration_ToolCallRequestReply(t *testing.T) {
	config := EmbeddedServerConfig{
		Port: 14301,
	}
	server, err := NewEmbeddedServer(config)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Shutdown()

	// Server client
	serverClient, err := NewClient(server.URL())
	if err != nil {
		t.Fatalf("Failed to create server client: %v", err)
	}
	defer serverClient.Close()

	// Agent client
	agentClient, err := NewClient(server.URL())
	if err != nil {
		t.Fatalf("Failed to create agent client: %v", err)
	}
	defer agentClient.Close()

	// Server handles tool calls
	_, err = serverClient.Subscribe(SubjectToolCall, func(msg *Message) {
		var req ToolCallRequest
		if err := json.Unmarshal(msg.Data, &req); err != nil {
			return
		}

		// Simulate tool execution
		resp := ToolCallResponse{
			RequestID: req.RequestID,
			Success:   true,
			Result: map[string]interface{}{
				"status":  "ok",
				"message": "Tool executed successfully",
			},
		}

		if msg.Reply != "" {
			serverClient.PublishJSON(msg.Reply, resp)
		}
	})
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	// Agent makes tool call request
	req := ToolCallRequest{
		RequestID: "req-001",
		AgentID:   "test-agent",
		Tool:      "report_status",
		Arguments: map[string]interface{}{
			"status": "working",
			"task":   "Testing NATS",
		},
	}

	var resp ToolCallResponse
	err = agentClient.RequestJSON(SubjectToolCall, req, &resp, 2*time.Second)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if !resp.Success {
		t.Errorf("Expected success, got failure: %s", resp.Error)
	}
	if resp.RequestID != "req-001" {
		t.Errorf("Request ID mismatch: got %s", resp.RequestID)
	}
}

// TestNATSIntegration_MultipleAgents tests multiple agents sending messages concurrently
func TestNATSIntegration_MultipleAgents(t *testing.T) {
	config := EmbeddedServerConfig{
		Port: 14302,
	}
	server, err := NewEmbeddedServer(config)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Shutdown()

	// Monitor client
	monitor, err := NewClient(server.URL())
	if err != nil {
		t.Fatalf("Failed to create monitor client: %v", err)
	}
	defer monitor.Close()

	// Track messages by agent
	agentMessages := make(map[string]int)
	var mu sync.Mutex

	_, err = monitor.Subscribe(SubjectAllHeartbeats, func(msg *Message) {
		var hb HeartbeatMessage
		if err := json.Unmarshal(msg.Data, &hb); err != nil {
			return
		}
		mu.Lock()
		agentMessages[hb.AgentID]++
		mu.Unlock()
	})
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	// Spawn multiple agent clients concurrently
	var wg sync.WaitGroup
	agentCount := 5
	messagesPerAgent := 10

	for i := 0; i < agentCount; i++ {
		wg.Add(1)
		go func(agentNum int) {
			defer wg.Done()

			client, err := NewClient(server.URL())
			if err != nil {
				t.Errorf("Failed to create agent %d client: %v", agentNum, err)
				return
			}
			defer client.Close()

			agentID := "agent-" + string(rune('A'+agentNum))
			subject := "agent." + agentID + ".heartbeat"

			for j := 0; j < messagesPerAgent; j++ {
				hb := HeartbeatMessage{
					AgentID:   agentID,
					Status:    "working",
					Timestamp: time.Now(),
				}
				client.PublishJSON(subject, hb)
				time.Sleep(10 * time.Millisecond)
			}
		}(i)
	}

	wg.Wait()
	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	totalMessages := 0
	for _, count := range agentMessages {
		totalMessages += count
	}
	agentsSeen := len(agentMessages)
	mu.Unlock()

	expectedTotal := agentCount * messagesPerAgent
	if totalMessages != expectedTotal {
		t.Errorf("Expected %d total messages, got %d", expectedTotal, totalMessages)
	}
	if agentsSeen != agentCount {
		t.Errorf("Expected %d agents, saw %d", agentCount, agentsSeen)
	}
}
