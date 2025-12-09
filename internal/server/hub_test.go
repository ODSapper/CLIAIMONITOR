package server

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/CLIAIMONITOR/internal/types"
)

func TestNewHub(t *testing.T) {
	hub := NewHub()
	if hub == nil {
		t.Fatal("NewHub returned nil")
	}
	if hub.clients == nil {
		t.Error("clients map should be initialized")
	}
	if hub.register == nil {
		t.Error("register channel should be initialized")
	}
	if hub.unregister == nil {
		t.Error("unregister channel should be initialized")
	}
	if hub.broadcast == nil {
		t.Error("broadcast channel should be initialized")
	}
}

func TestHubClientCount(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	if hub.ClientCount() != 0 {
		t.Errorf("expected 0 clients initially, got %d", hub.ClientCount())
	}

	// Create mock clients
	client1 := &Client{
		hub:  hub,
		conn: nil, // We don't need a real connection for this test
		send: make(chan []byte, WebSocketBufferSize),
	}
	client2 := &Client{
		hub:  hub,
		conn: nil,
		send: make(chan []byte, WebSocketBufferSize),
	}

	// Register clients
	hub.Register(client1)
	time.Sleep(10 * time.Millisecond) // Allow goroutine to process

	if hub.ClientCount() != 1 {
		t.Errorf("expected 1 client after first register, got %d", hub.ClientCount())
	}

	hub.Register(client2)
	time.Sleep(10 * time.Millisecond)

	if hub.ClientCount() != 2 {
		t.Errorf("expected 2 clients after second register, got %d", hub.ClientCount())
	}

	// Unregister client
	hub.Unregister(client1)
	time.Sleep(10 * time.Millisecond)

	if hub.ClientCount() != 1 {
		t.Errorf("expected 1 client after unregister, got %d", hub.ClientCount())
	}
}

func TestHubBroadcast(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	client := &Client{
		hub:  hub,
		conn: nil,
		send: make(chan []byte, WebSocketBufferSize),
	}

	hub.Register(client)
	time.Sleep(10 * time.Millisecond)

	// Test BroadcastJSON
	testMsg := map[string]string{"test": "message"}
	hub.BroadcastJSON(testMsg)

	// Wait for message
	select {
	case received := <-client.send:
		var decoded map[string]string
		if err := json.Unmarshal(received, &decoded); err != nil {
			t.Fatalf("Failed to decode message: %v", err)
		}
		if decoded["test"] != "message" {
			t.Errorf("expected 'message', got '%s'", decoded["test"])
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("did not receive broadcast message")
	}
}

func TestHubBroadcastState(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	client := &Client{
		hub:  hub,
		conn: nil,
		send: make(chan []byte, WebSocketBufferSize),
	}

	hub.Register(client)
	time.Sleep(10 * time.Millisecond)

	state := types.NewDashboardState()

	hub.BroadcastState(state)

	select {
	case received := <-client.send:
		var msg types.WSMessage
		if err := json.Unmarshal(received, &msg); err != nil {
			t.Fatalf("Failed to decode message: %v", err)
		}
		if msg.Type != types.WSTypeStateUpdate {
			t.Errorf("expected type '%s', got '%s'", types.WSTypeStateUpdate, msg.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("did not receive state broadcast")
	}
}

func TestHubBroadcastAlert(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	client := &Client{
		hub:  hub,
		conn: nil,
		send: make(chan []byte, WebSocketBufferSize),
	}

	hub.Register(client)
	time.Sleep(10 * time.Millisecond)

	alert := &types.Alert{
		ID:       "alert-001",
		Type:     "test",
		Message:  "Test alert",
		Severity: "warning",
	}

	hub.BroadcastAlert(alert)

	select {
	case received := <-client.send:
		var msg types.WSMessage
		if err := json.Unmarshal(received, &msg); err != nil {
			t.Fatalf("Failed to decode message: %v", err)
		}
		if msg.Type != types.WSTypeAlert {
			t.Errorf("expected type '%s', got '%s'", types.WSTypeAlert, msg.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("did not receive alert broadcast")
	}
}

func TestHubBroadcastActivity(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	client := &Client{
		hub:  hub,
		conn: nil,
		send: make(chan []byte, WebSocketBufferSize),
	}

	hub.Register(client)
	time.Sleep(10 * time.Millisecond)

	activity := &types.ActivityLog{
		ID:      "act-001",
		AgentID: "TestAgent",
		Action:  "commit",
		Details: "Test commit",
	}

	hub.BroadcastActivity(activity)

	select {
	case received := <-client.send:
		var msg types.WSMessage
		if err := json.Unmarshal(received, &msg); err != nil {
			t.Fatalf("Failed to decode message: %v", err)
		}
		if msg.Type != types.WSTypeActivity {
			t.Errorf("expected type '%s', got '%s'", types.WSTypeActivity, msg.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("did not receive activity broadcast")
	}
}

func TestHubMultipleClients(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	// Create 3 clients
	clients := make([]*Client, 3)
	for i := 0; i < 3; i++ {
		clients[i] = &Client{
			hub:  hub,
			conn: nil,
			send: make(chan []byte, WebSocketBufferSize),
		}
		hub.Register(clients[i])
	}

	time.Sleep(20 * time.Millisecond)

	if hub.ClientCount() != 3 {
		t.Errorf("expected 3 clients, got %d", hub.ClientCount())
	}

	// Broadcast message
	testMsg := map[string]string{"test": "broadcast"}
	hub.BroadcastJSON(testMsg)

	// All clients should receive the message
	for i, client := range clients {
		select {
		case <-client.send:
			// Message received
		case <-time.After(100 * time.Millisecond):
			t.Errorf("client %d did not receive broadcast", i)
		}
	}
}

func TestHubUnregisterNonexistent(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	// Try to unregister a client that was never registered
	client := &Client{
		hub:  hub,
		conn: nil,
		send: make(chan []byte, WebSocketBufferSize),
	}

	// This should not panic
	hub.Unregister(client)
	time.Sleep(10 * time.Millisecond)

	if hub.ClientCount() != 0 {
		t.Errorf("expected 0 clients, got %d", hub.ClientCount())
	}
}

func TestHubBroadcastToEmptyHub(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	// Broadcasting to empty hub should not panic
	hub.BroadcastJSON(map[string]string{"test": "empty"})

	// Give it time to process
	time.Sleep(10 * time.Millisecond)

	if hub.ClientCount() != 0 {
		t.Errorf("expected 0 clients, got %d", hub.ClientCount())
	}
}

func TestFormatAgentNumber(t *testing.T) {
	tests := []struct {
		input    int
		expected string
	}{
		{1, "001"},
		{10, "010"},
		{100, "100"},
		{999, "999"},
		{0, "000"},
	}

	for _, tt := range tests {
		result := formatAgentNumber(tt.input)
		if result != tt.expected {
			t.Errorf("formatAgentNumber(%d) = %s, want %s", tt.input, result, tt.expected)
		}
	}
}
