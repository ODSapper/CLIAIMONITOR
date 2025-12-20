package nats

import (
	"fmt"
	"sync"
	"time"

	"github.com/nats-io/nats-server/v2/server"
)

// EmbeddedServerConfig holds configuration for the embedded NATS server
type EmbeddedServerConfig struct {
	Port          int    // Port to listen on
	WebSocketPort int    // WebSocket port to listen on (0 to disable)
	JetStream     bool   // Enable JetStream
	DataDir       string // Data directory for JetStream storage
}

// EmbeddedServer wraps the NATS server
type EmbeddedServer struct {
	server           *server.Server
	config           EmbeddedServerConfig
	mu               sync.RWMutex
	running          bool
	connectedClients map[string]time.Time // clientID -> connected timestamp
}

// NewEmbeddedServer creates a new embedded NATS server instance
func NewEmbeddedServer(config EmbeddedServerConfig) (*EmbeddedServer, error) {
	if config.Port <= 0 {
		config.Port = 4222 // Default NATS port
	}

	if config.JetStream && config.DataDir == "" {
		return nil, fmt.Errorf("DataDir is required when JetStream is enabled")
	}

	return &EmbeddedServer{
		config:           config,
		connectedClients: make(map[string]time.Time),
	}, nil
}

// Start starts the embedded NATS server with JetStream support
func (e *EmbeddedServer) Start() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.running {
		return fmt.Errorf("server already running")
	}

	// Create server options
	opts := &server.Options{
		Host: "127.0.0.1",
		Port: e.config.Port,
		// Disable logging to stdout by default for cleaner test output
		NoLog:  false,
		NoSigs: true,
		MaxPayload: 1024 * 1024, // 1MB max payload
	}

	// Configure WebSocket if enabled
	if e.config.WebSocketPort > 0 {
		opts.Websocket = server.WebsocketOpts{
			Host:  "127.0.0.1",
			Port:  e.config.WebSocketPort,
			NoTLS: true, // localhost doesn't need TLS
		}
	}

	// Configure JetStream if enabled
	if e.config.JetStream {
		opts.JetStream = true
		opts.StoreDir = e.config.DataDir
	}

	// Create and start the server
	ns, err := server.NewServer(opts)
	if err != nil {
		return fmt.Errorf("failed to create NATS server: %w", err)
	}

	e.server = ns

	// Start server in background
	go ns.Start()

	// Wait for server to be ready
	if !ns.ReadyForConnections(10 * time.Second) {
		return fmt.Errorf("server not ready for connections")
	}

	e.running = true

	// Log WebSocket status
	if e.config.WebSocketPort > 0 {
		fmt.Printf("[NATS] WebSocket enabled on ws://127.0.0.1:%d\n", e.config.WebSocketPort)
	}

	return nil
}

// Shutdown gracefully shuts down the NATS server
func (e *EmbeddedServer) Shutdown() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.running || e.server == nil {
		return
	}

	e.server.Shutdown()

	// Wait for shutdown to complete
	e.server.WaitForShutdown()

	e.running = false
	e.server = nil
}

// URL returns the connection URL for the NATS server
func (e *EmbeddedServer) URL() string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return fmt.Sprintf("nats://127.0.0.1:%d", e.config.Port)
}

// WebSocketURL returns the WebSocket connection URL for the NATS server
// Returns empty string if WebSocket is not enabled
func (e *EmbeddedServer) WebSocketURL() string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.config.WebSocketPort <= 0 {
		return ""
	}

	return fmt.Sprintf("ws://127.0.0.1:%d", e.config.WebSocketPort)
}

// IsRunning returns whether the server is currently running
func (e *EmbeddedServer) IsRunning() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return e.running
}

// GetConnectedClients returns a list of currently connected client IDs
func (e *EmbeddedServer) GetConnectedClients() []ClientInfo {
	e.mu.RLock()
	defer e.mu.RUnlock()

	clients := make([]ClientInfo, 0, len(e.connectedClients))
	for clientID, connectedAt := range e.connectedClients {
		clients = append(clients, ClientInfo{
			ClientID:    clientID,
			ConnectedAt: connectedAt,
		})
	}
	return clients
}

// IsClientConnected checks if a specific client ID is currently connected
func (e *EmbeddedServer) IsClientConnected(clientID string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	_, exists := e.connectedClients[clientID]
	return exists
}

// trackClientConnected records a client connection
func (e *EmbeddedServer) trackClientConnected(clientID string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.connectedClients[clientID] = time.Now()
}

// trackClientDisconnected removes a client from the connected list
func (e *EmbeddedServer) trackClientDisconnected(clientID string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	delete(e.connectedClients, clientID)
}
