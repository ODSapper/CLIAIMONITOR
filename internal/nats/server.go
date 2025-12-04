package nats

import (
	"fmt"
	"sync"
	"time"

	"github.com/nats-io/nats-server/v2/server"
)

// EmbeddedServerConfig holds configuration for the embedded NATS server
type EmbeddedServerConfig struct {
	Port       int    // Port to listen on
	JetStream  bool   // Enable JetStream
	DataDir    string // Data directory for JetStream storage
}

// EmbeddedServer wraps the NATS server
type EmbeddedServer struct {
	server *server.Server
	config EmbeddedServerConfig
	mu     sync.RWMutex
	running bool
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
		config: config,
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

// IsRunning returns whether the server is currently running
func (e *EmbeddedServer) IsRunning() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return e.running
}
