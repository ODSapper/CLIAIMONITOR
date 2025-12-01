package mcp

import (
	"sync"
	"testing"
)

func TestNewConnectionManager(t *testing.T) {
	m := NewConnectionManager()
	if m == nil {
		t.Fatal("NewConnectionManager returned nil")
	}
	if m.connections == nil {
		t.Error("connections map should be initialized")
	}
}

func TestSetCallbacks(t *testing.T) {
	m := NewConnectionManager()

	m.SetCallbacks(
		func(agentID string) {},
		func(agentID string) {},
	)

	if m.onConnect == nil {
		t.Error("onConnect should be set")
	}
	if m.onDisconnect == nil {
		t.Error("onDisconnect should be set")
	}
}

func TestAddAndGet(t *testing.T) {
	m := NewConnectionManager()

	// Create a mock connection (we can't use http.ResponseWriter easily in tests)
	conn := &SSEConnection{
		AgentID: "Agent1",
		Done:    make(chan struct{}),
	}

	m.Add("Agent1", conn)

	retrieved := m.Get("Agent1")
	if retrieved == nil {
		t.Fatal("Get returned nil for added connection")
	}
	if retrieved.AgentID != "Agent1" {
		t.Errorf("AgentID = %q, want %q", retrieved.AgentID, "Agent1")
	}
}

func TestConnectionManagerGetNotFound(t *testing.T) {
	m := NewConnectionManager()

	retrieved := m.Get("NonExistent")
	if retrieved != nil {
		t.Error("Get should return nil for nonexistent connection")
	}
}

func TestRemove(t *testing.T) {
	m := NewConnectionManager()

	conn := &SSEConnection{
		AgentID: "Agent1",
		Done:    make(chan struct{}),
	}
	m.Add("Agent1", conn)

	m.Remove("Agent1")

	if m.Get("Agent1") != nil {
		t.Error("connection should be removed")
	}
}

func TestRemoveClosesConnection(t *testing.T) {
	m := NewConnectionManager()

	conn := &SSEConnection{
		AgentID: "Agent1",
		Done:    make(chan struct{}),
	}
	m.Add("Agent1", conn)

	m.Remove("Agent1")

	// Check if Done channel is closed
	select {
	case <-conn.Done:
		// Expected - channel is closed
	default:
		t.Error("connection Done channel should be closed on Remove")
	}
}

func TestAddReplacesExisting(t *testing.T) {
	m := NewConnectionManager()

	conn1 := &SSEConnection{
		AgentID: "Agent1",
		Done:    make(chan struct{}),
	}
	conn2 := &SSEConnection{
		AgentID: "Agent1",
		Done:    make(chan struct{}),
	}

	m.Add("Agent1", conn1)
	m.Add("Agent1", conn2)

	// First connection should be closed
	select {
	case <-conn1.Done:
		// Expected
	default:
		t.Error("first connection should be closed when replaced")
	}

	// Second connection should be active
	retrieved := m.Get("Agent1")
	if retrieved != conn2 {
		t.Error("should return new connection")
	}
}

func TestGetAll(t *testing.T) {
	m := NewConnectionManager()

	conn1 := &SSEConnection{AgentID: "Agent1", Done: make(chan struct{})}
	conn2 := &SSEConnection{AgentID: "Agent2", Done: make(chan struct{})}
	conn3 := &SSEConnection{AgentID: "Agent3", Done: make(chan struct{})}

	m.Add("Agent1", conn1)
	m.Add("Agent2", conn2)
	m.Add("Agent3", conn3)

	all := m.GetAll()
	if len(all) != 3 {
		t.Errorf("expected 3 connections, got %d", len(all))
	}

	// Verify it's a copy
	delete(all, "Agent1")
	if m.Get("Agent1") == nil {
		t.Error("GetAll should return a copy, not the original map")
	}
}

func TestGetConnectedAgentIDs(t *testing.T) {
	m := NewConnectionManager()

	m.Add("Agent1", &SSEConnection{AgentID: "Agent1", Done: make(chan struct{})})
	m.Add("Agent2", &SSEConnection{AgentID: "Agent2", Done: make(chan struct{})})

	ids := m.GetConnectedAgentIDs()
	if len(ids) != 2 {
		t.Errorf("expected 2 IDs, got %d", len(ids))
	}

	// Check both IDs are present
	hasAgent1 := false
	hasAgent2 := false
	for _, id := range ids {
		if id == "Agent1" {
			hasAgent1 = true
		}
		if id == "Agent2" {
			hasAgent2 = true
		}
	}

	if !hasAgent1 || !hasAgent2 {
		t.Error("missing expected agent IDs")
	}
}

func TestCallbacksOnAdd(t *testing.T) {
	m := NewConnectionManager()

	var connectedAgent string
	m.SetCallbacks(
		func(agentID string) { connectedAgent = agentID },
		nil,
	)

	m.Add("Agent1", &SSEConnection{AgentID: "Agent1", Done: make(chan struct{})})

	if connectedAgent != "Agent1" {
		t.Errorf("onConnect should be called with Agent1, got %q", connectedAgent)
	}
}

func TestCallbacksOnRemove(t *testing.T) {
	m := NewConnectionManager()

	var disconnectedAgent string
	m.SetCallbacks(
		nil,
		func(agentID string) { disconnectedAgent = agentID },
	)

	m.Add("Agent1", &SSEConnection{AgentID: "Agent1", Done: make(chan struct{})})
	m.Remove("Agent1")

	if disconnectedAgent != "Agent1" {
		t.Errorf("onDisconnect should be called with Agent1, got %q", disconnectedAgent)
	}
}

func TestRemoveNonexistent(t *testing.T) {
	m := NewConnectionManager()

	// Should not panic
	m.Remove("NonExistent")
}

func TestConcurrentAccess(t *testing.T) {
	m := NewConnectionManager()
	var wg sync.WaitGroup

	// Concurrent adds
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				conn := &SSEConnection{
					AgentID: "Agent",
					Done:    make(chan struct{}),
				}
				m.Add("Agent", conn)
				m.Get("Agent")
				m.GetAll()
				m.GetConnectedAgentIDs()
			}
		}(i)
	}

	wg.Wait()
}

func TestSSEConnectionClose(t *testing.T) {
	conn := &SSEConnection{
		AgentID: "Agent1",
		Done:    make(chan struct{}),
	}

	conn.Close()

	// Verify channel is closed
	select {
	case <-conn.Done:
		// Expected
	default:
		t.Error("Done channel should be closed")
	}

	// Close again should not panic
	conn.Close()
}
