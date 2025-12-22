package mcp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/CLIAIMONITOR/internal/types"
	"github.com/google/uuid"
)

// ConnectionState represents the lifecycle state of a connection
type ConnectionState int

const (
	StateConnecting ConnectionState = iota
	StateActive
	StateClosing
	StateClosed
)

// SSEConnection represents a connected agent
type SSEConnection struct {
	AgentID   string
	SessionID string
	Writer    http.ResponseWriter
	Flusher   http.Flusher
	Done      chan struct{}
	CreatedAt time.Time
	LastPing  time.Time
	state     ConnectionState
	mu        sync.Mutex
	closeOnce sync.Once // Ensure Close is idempotent
}

// NewSSEConnection creates a new SSE connection
func NewSSEConnection(agentID string, w http.ResponseWriter) (*SSEConnection, error) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("streaming not supported")
	}

	return &SSEConnection{
		AgentID:   agentID,
		SessionID: uuid.New().String(),
		Writer:    w,
		Flusher:   flusher,
		Done:      make(chan struct{}),
		CreatedAt: time.Now(),
		LastPing:  time.Now(),
		state:     StateConnecting,
	}, nil
}

// Send writes an SSE message to the connection
func (c *SSEConnection) Send(event string, data interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	// SSE format: event: <event>\ndata: <data>\n\n
	_, err = fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", event, jsonData)
	if err != nil {
		return err
	}

	c.Flusher.Flush()
	c.LastPing = time.Now()
	return nil
}

// SendPlainData writes an SSE message with plain string data (not JSON)
func (c *SSEConnection) SendPlainData(event string, data string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// SSE format: event: <event>\ndata: <data>\n\n
	_, err := fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", event, data)
	if err != nil {
		return err
	}

	c.Flusher.Flush()
	c.LastPing = time.Now()
	return nil
}

// SendResponse sends an MCP JSON-RPC response
func (c *SSEConnection) SendResponse(resp types.MCPResponse) error {
	return c.Send("message", resp)
}

// SendNotification sends an MCP notification
func (c *SSEConnection) SendNotification(method string, params interface{}) error {
	notification := types.MCPNotification{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}
	return c.Send("message", notification)
}

// Close closes the connection safely (idempotent)
func (c *SSEConnection) Close() {
	c.closeOnce.Do(func() {
		c.mu.Lock()
		c.state = StateClosing
		c.mu.Unlock()

		close(c.Done)

		c.mu.Lock()
		c.state = StateClosed
		c.mu.Unlock()
	})
}

// IsClosed returns true if the connection is closed or closing
func (c *SSEConnection) IsClosed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.state == StateClosing || c.state == StateClosed
}

// SetActive marks the connection as active
func (c *SSEConnection) SetActive() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state == StateConnecting {
		c.state = StateActive
	}
}

// ConnectionManager manages all SSE connections
type ConnectionManager struct {
	mu           sync.RWMutex
	connections  map[string]*SSEConnection
	sessions     map[string]*SSEConnection
	onConnect    func(agentID string)
	onDisconnect func(agentID string)
	shutdownChan chan struct{}
	shutdownOnce sync.Once
}

// NewConnectionManager creates a new connection manager
func NewConnectionManager() *ConnectionManager {
	cm := &ConnectionManager{
		connections:  make(map[string]*SSEConnection),
		sessions:     make(map[string]*SSEConnection),
		shutdownChan: make(chan struct{}),
	}

	// Start background cleanup goroutine
	go cm.cleanupStaleConnections()

	return cm
}

// cleanupStaleConnections periodically checks for and removes stale connections
func (m *ConnectionManager) cleanupStaleConnections() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.shutdownChan:
			return
		case <-ticker.C:
			m.mu.Lock()
			now := time.Now()
			var staleAgents []string

			// Find connections with no activity for 5 minutes
			for agentID, conn := range m.connections {
				conn.mu.Lock()
				lastPing := conn.LastPing
				isClosed := conn.state == StateClosing || conn.state == StateClosed
				conn.mu.Unlock()

				if isClosed || now.Sub(lastPing) > 5*time.Minute {
					staleAgents = append(staleAgents, agentID)
				}
			}
			m.mu.Unlock()

			// Remove stale connections
			for _, agentID := range staleAgents {
				m.Remove(agentID)
			}
		}
	}
}

// Shutdown stops the connection manager and cleans up all connections
func (m *ConnectionManager) Shutdown() {
	m.shutdownOnce.Do(func() {
		close(m.shutdownChan)

		// Close all active connections
		m.mu.Lock()
		for agentID := range m.connections {
			if conn, ok := m.connections[agentID]; ok {
				delete(m.sessions, conn.SessionID)
				conn.Close()
			}
		}
		m.connections = make(map[string]*SSEConnection)
		m.sessions = make(map[string]*SSEConnection)
		m.mu.Unlock()
	})
}

// SetCallbacks sets connection event callbacks
func (m *ConnectionManager) SetCallbacks(onConnect, onDisconnect func(agentID string)) {
	m.onConnect = onConnect
	m.onDisconnect = onDisconnect
}

// Add registers a new connection
func (m *ConnectionManager) Add(agentID string, conn *SSEConnection) {
	m.mu.Lock()
	// Close existing connection if any
	if existing, ok := m.connections[agentID]; ok {
		delete(m.sessions, existing.SessionID)
		existing.Close()
	}
	m.connections[agentID] = conn
	m.sessions[conn.SessionID] = conn
	m.mu.Unlock()

	if m.onConnect != nil {
		m.onConnect(agentID)
	}
}

// Remove unregisters a connection
func (m *ConnectionManager) Remove(agentID string) {
	m.mu.Lock()
	if conn, ok := m.connections[agentID]; ok {
		delete(m.sessions, conn.SessionID)
		conn.Close()
		delete(m.connections, agentID)
	}
	m.mu.Unlock()

	if m.onDisconnect != nil {
		m.onDisconnect(agentID)
	}
}

// Get returns a connection by agent ID
func (m *ConnectionManager) Get(agentID string) *SSEConnection {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.connections[agentID]
}

// GetBySession looks up connection by session ID
func (m *ConnectionManager) GetBySession(sessionID string) *SSEConnection {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessions[sessionID]
}

// GetAll returns all connections
func (m *ConnectionManager) GetAll() map[string]*SSEConnection {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*SSEConnection)
	for k, v := range m.connections {
		result[k] = v
	}
	return result
}

// GetConnectedAgentIDs returns list of connected agent IDs
func (m *ConnectionManager) GetConnectedAgentIDs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]string, 0, len(m.connections))
	for id := range m.connections {
		ids = append(ids, id)
	}
	return ids
}

// Broadcast sends a notification to all connected agents
func (m *ConnectionManager) Broadcast(method string, params interface{}) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, conn := range m.connections {
		conn.SendNotification(method, params)
	}
}
