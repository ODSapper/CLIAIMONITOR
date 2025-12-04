package nats

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	nc "github.com/nats-io/nats.go"
)

// TestEmbeddedNATSServer_StartStop verifies the server starts and accepts connections
func TestEmbeddedNATSServer_StartStop(t *testing.T) {
	// Create temp directory for JetStream data
	tempDir, err := os.MkdirTemp("", "nats-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create server config using non-default port to avoid conflicts
	config := EmbeddedServerConfig{
		Port:      14222,
		JetStream: true,
		DataDir:   filepath.Join(tempDir, "jetstream"),
	}

	// Create server
	server, err := NewEmbeddedServer(config)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Verify server is not running yet
	if server.IsRunning() {
		t.Error("Server should not be running before Start() is called")
	}

	// Start server
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Shutdown()

	// Verify server is running
	if !server.IsRunning() {
		t.Error("Server should be running after Start() is called")
	}

	// Verify URL is correct
	expectedURL := "nats://127.0.0.1:14222"
	if server.URL() != expectedURL {
		t.Errorf("Expected URL %s, got %s", expectedURL, server.URL())
	}

	// Try to connect to verify server is accepting connections
	conn, err := nc.Connect(server.URL())
	if err != nil {
		t.Fatalf("Failed to connect to NATS server: %v", err)
	}
	defer conn.Close()

	// Verify connection is established
	if !conn.IsConnected() {
		t.Error("Connection should be established")
	}

	// Shutdown server
	server.Shutdown()

	// Verify server is no longer running
	if server.IsRunning() {
		t.Error("Server should not be running after Shutdown() is called")
	}

	// Verify connection is closed
	time.Sleep(100 * time.Millisecond) // Brief delay for shutdown propagation
	if conn.IsConnected() {
		t.Error("Connection should be closed after server shutdown")
	}
}

// TestEmbeddedNATSServer_PubSub verifies pub/sub functionality works
func TestEmbeddedNATSServer_PubSub(t *testing.T) {
	// Create temp directory for JetStream data
	tempDir, err := os.MkdirTemp("", "nats-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create server config using non-default port
	config := EmbeddedServerConfig{
		Port:      14223,
		JetStream: true,
		DataDir:   filepath.Join(tempDir, "jetstream"),
	}

	// Create and start server
	server, err := NewEmbeddedServer(config)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Shutdown()

	// Connect to server
	conn, err := nc.Connect(server.URL())
	if err != nil {
		t.Fatalf("Failed to connect to NATS server: %v", err)
	}
	defer conn.Close()

	// Test subject and message
	subject := "test.subject"
	testMessage := "Hello NATS!"
	received := make(chan string, 1)

	// Subscribe to subject
	sub, err := conn.Subscribe(subject, func(msg *nc.Msg) {
		received <- string(msg.Data)
	})
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}
	defer sub.Unsubscribe()

	// Ensure subscription is active
	if err := conn.Flush(); err != nil {
		t.Fatalf("Failed to flush: %v", err)
	}

	// Publish message
	if err := conn.Publish(subject, []byte(testMessage)); err != nil {
		t.Fatalf("Failed to publish: %v", err)
	}

	// Wait for message with timeout
	select {
	case msg := <-received:
		if msg != testMessage {
			t.Errorf("Expected message %q, got %q", testMessage, msg)
		}
	case <-time.After(2 * time.Second):
		t.Error("Timeout waiting for message")
	}

	// Test request/reply pattern
	replySubject := "test.reply"
	expectedReply := "PONG"

	// Create responder
	_, err = conn.Subscribe(replySubject, func(msg *nc.Msg) {
		if err := msg.Respond([]byte(expectedReply)); err != nil {
			t.Logf("Failed to respond: %v", err)
		}
	})
	if err != nil {
		t.Fatalf("Failed to create responder: %v", err)
	}

	if err := conn.Flush(); err != nil {
		t.Fatalf("Failed to flush: %v", err)
	}

	// Send request
	msg, err := conn.Request(replySubject, []byte("PING"), 2*time.Second)
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}

	if string(msg.Data) != expectedReply {
		t.Errorf("Expected reply %q, got %q", expectedReply, string(msg.Data))
	}
}

// TestEmbeddedNATSServer_MultipleServers verifies multiple servers can run on different ports
func TestEmbeddedNATSServer_MultipleServers(t *testing.T) {
	// Create temp directories for JetStream data
	tempDir1, err := os.MkdirTemp("", "nats-test-1-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir 1: %v", err)
	}
	defer os.RemoveAll(tempDir1)

	tempDir2, err := os.MkdirTemp("", "nats-test-2-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir 2: %v", err)
	}
	defer os.RemoveAll(tempDir2)

	// Create two servers on different ports
	config1 := EmbeddedServerConfig{
		Port:      14224,
		JetStream: true,
		DataDir:   filepath.Join(tempDir1, "jetstream"),
	}

	config2 := EmbeddedServerConfig{
		Port:      14225,
		JetStream: true,
		DataDir:   filepath.Join(tempDir2, "jetstream"),
	}

	server1, err := NewEmbeddedServer(config1)
	if err != nil {
		t.Fatalf("Failed to create server 1: %v", err)
	}

	server2, err := NewEmbeddedServer(config2)
	if err != nil {
		t.Fatalf("Failed to create server 2: %v", err)
	}

	// Start both servers
	if err := server1.Start(); err != nil {
		t.Fatalf("Failed to start server 1: %v", err)
	}
	defer server1.Shutdown()

	if err := server2.Start(); err != nil {
		t.Fatalf("Failed to start server 2: %v", err)
	}
	defer server2.Shutdown()

	// Connect to both servers
	conn1, err := nc.Connect(server1.URL())
	if err != nil {
		t.Fatalf("Failed to connect to server 1: %v", err)
	}
	defer conn1.Close()

	conn2, err := nc.Connect(server2.URL())
	if err != nil {
		t.Fatalf("Failed to connect to server 2: %v", err)
	}
	defer conn2.Close()

	// Verify both connections are independent
	subject := "test.independent"
	received1 := make(chan bool, 1)
	received2 := make(chan bool, 1)

	// Subscribe on server 1
	_, err = conn1.Subscribe(subject, func(msg *nc.Msg) {
		received1 <- true
	})
	if err != nil {
		t.Fatalf("Failed to subscribe on server 1: %v", err)
	}

	// Subscribe on server 2
	_, err = conn2.Subscribe(subject, func(msg *nc.Msg) {
		received2 <- true
	})
	if err != nil {
		t.Fatalf("Failed to subscribe on server 2: %v", err)
	}

	// Flush to ensure subscriptions are active
	conn1.Flush()
	conn2.Flush()

	// Publish on server 1 - should only be received on server 1
	if err := conn1.Publish(subject, []byte("test")); err != nil {
		t.Fatalf("Failed to publish on server 1: %v", err)
	}

	select {
	case <-received1:
		// Expected - message received on server 1
	case <-time.After(1 * time.Second):
		t.Error("Timeout waiting for message on server 1")
	}

	// Server 2 should NOT receive the message (different server instance)
	select {
	case <-received2:
		t.Error("Server 2 should not receive message published on server 1")
	case <-time.After(500 * time.Millisecond):
		// Expected - no message on server 2
	}
}

// TestEmbeddedNATSServer_ConfigValidation tests configuration validation
func TestEmbeddedNATSServer_ConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      EmbeddedServerConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "Valid config with JetStream",
			config: EmbeddedServerConfig{
				Port:      14222,
				JetStream: true,
				DataDir:   "/tmp/test",
			},
			expectError: false,
		},
		{
			name: "Valid config without JetStream",
			config: EmbeddedServerConfig{
				Port:      14222,
				JetStream: false,
			},
			expectError: false,
		},
		{
			name: "Invalid - JetStream enabled without DataDir",
			config: EmbeddedServerConfig{
				Port:      14222,
				JetStream: true,
				DataDir:   "",
			},
			expectError: true,
			errorMsg:    "DataDir is required when JetStream is enabled",
		},
		{
			name: "Default port when not specified",
			config: EmbeddedServerConfig{
				Port:      0,
				JetStream: false,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, err := NewEmbeddedServer(tt.config)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error containing %q, got nil", tt.errorMsg)
				} else if err.Error() != tt.errorMsg {
					t.Errorf("Expected error %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
				if server == nil {
					t.Error("Expected server to be created")
				}
				// Verify default port is set
				if tt.config.Port == 0 && server.config.Port != 4222 {
					t.Errorf("Expected default port 4222, got %d", server.config.Port)
				}
			}
		})
	}
}

// TestEmbeddedNATSServer_DoubleStart verifies starting an already running server returns error
func TestEmbeddedNATSServer_DoubleStart(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "nats-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := EmbeddedServerConfig{
		Port:      14222,
		JetStream: true,
		DataDir:   filepath.Join(tempDir, "jetstream"),
	}

	server, err := NewEmbeddedServer(config)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Start server
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Shutdown()

	// Try to start again
	err = server.Start()
	if err == nil {
		t.Error("Expected error when starting already running server")
	} else if err.Error() != "server already running" {
		t.Errorf("Expected 'server already running' error, got: %v", err)
	}
}

// Benchmark pub/sub performance
func BenchmarkEmbeddedNATSServer_PubSub(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "nats-bench-*")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := EmbeddedServerConfig{
		Port:      14222,
		JetStream: true,
		DataDir:   filepath.Join(tempDir, "jetstream"),
	}

	server, err := NewEmbeddedServer(config)
	if err != nil {
		b.Fatalf("Failed to create server: %v", err)
	}

	if err := server.Start(); err != nil {
		b.Fatalf("Failed to start server: %v", err)
	}
	defer server.Shutdown()

	conn, err := nc.Connect(server.URL())
	if err != nil {
		b.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	subject := "bench.test"
	message := []byte("benchmark message")

	// Subscribe
	received := 0
	_, err = conn.Subscribe(subject, func(msg *nc.Msg) {
		received++
	})
	if err != nil {
		b.Fatalf("Failed to subscribe: %v", err)
	}
	conn.Flush()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := conn.Publish(subject, message); err != nil {
			b.Fatalf("Failed to publish: %v", err)
		}
	}
	conn.Flush()

	b.StopTimer()
	b.Logf("Published %d messages, received %d", b.N, received)
}
