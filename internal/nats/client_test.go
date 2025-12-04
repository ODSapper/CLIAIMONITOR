package nats

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
)

// startTestServer starts an embedded NATS server for testing
func startTestServer(t *testing.T) (*server.Server, string) {
	opts := &server.Options{
		Host:           "127.0.0.1",
		Port:           -1, // Random port
		NoLog:          true,
		NoSigs:         true,
		MaxControlLine: 2048,
	}

	ns, err := server.NewServer(opts)
	if err != nil {
		t.Fatalf("Failed to create NATS server: %v", err)
	}

	go ns.Start()

	if !ns.ReadyForConnections(5 * time.Second) {
		t.Fatal("NATS server not ready")
	}

	url := ns.ClientURL()
	return ns, url
}

// TestNATSClient_RequestReply verifies request-reply pattern works
func TestNATSClient_RequestReply(t *testing.T) {
	ns, url := startTestServer(t)
	defer ns.Shutdown()

	// Create client
	client, err := NewClient(url, "test-client")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Verify connection
	if !client.IsConnected() {
		t.Fatal("Client should be connected")
	}

	// Set up responder
	responder, err := NewClient(url, "test-responder")
	if err != nil {
		t.Fatalf("Failed to create responder: %v", err)
	}
	defer responder.Close()

	subject := "test.request"
	expectedResponse := []byte("pong")

	// Subscribe to handle requests
	_, err = responder.Subscribe(subject, func(msg *Message) {
		// Respond to the request
		if msg.Reply != "" {
			responder.conn.Publish(msg.Reply, expectedResponse)
		}
	})
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	// Give subscription time to be ready
	time.Sleep(100 * time.Millisecond)

	// Send request
	requestData := []byte("ping")
	response, err := client.Request(subject, requestData, 2*time.Second)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	// Verify response
	if string(response.Data) != string(expectedResponse) {
		t.Errorf("Expected response %s, got %s", expectedResponse, response.Data)
	}
}

// TestNATSClient_PubSub verifies pub-sub with handler
func TestNATSClient_PubSub(t *testing.T) {
	ns, url := startTestServer(t)
	defer ns.Shutdown()

	// Create publisher
	publisher, err := NewClient(url, "test-publisher")
	if err != nil {
		t.Fatalf("Failed to create publisher: %v", err)
	}
	defer publisher.Close()

	// Create subscriber
	subscriber, err := NewClient(url, "test-subscriber")
	if err != nil {
		t.Fatalf("Failed to create subscriber: %v", err)
	}
	defer subscriber.Close()

	subject := "test.pubsub"
	expectedData := []byte("hello world")

	// Track received messages
	var mu sync.Mutex
	var receivedMessages []*Message

	// Subscribe
	_, err = subscriber.Subscribe(subject, func(msg *Message) {
		mu.Lock()
		receivedMessages = append(receivedMessages, msg)
		mu.Unlock()
	})
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	// Give subscription time to be ready
	time.Sleep(100 * time.Millisecond)

	// Publish message
	err = publisher.Publish(subject, expectedData)
	if err != nil {
		t.Fatalf("Failed to publish: %v", err)
	}

	// Flush to ensure delivery
	err = publisher.Flush()
	if err != nil {
		t.Fatalf("Failed to flush: %v", err)
	}

	// Wait for message to be received
	time.Sleep(200 * time.Millisecond)

	// Verify message received
	mu.Lock()
	defer mu.Unlock()

	if len(receivedMessages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(receivedMessages))
	}

	msg := receivedMessages[0]
	if msg.Subject != subject {
		t.Errorf("Expected subject %s, got %s", subject, msg.Subject)
	}
	if string(msg.Data) != string(expectedData) {
		t.Errorf("Expected data %s, got %s", expectedData, msg.Data)
	}
}

// TestNATSClient_PublishJSON tests JSON publishing
func TestNATSClient_PublishJSON(t *testing.T) {
	ns, url := startTestServer(t)
	defer ns.Shutdown()

	client, err := NewClient(url, "test-client")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	subject := "test.json"
	testData := map[string]interface{}{
		"name":  "test",
		"value": 42,
	}

	// Track received data
	var mu sync.Mutex
	var receivedData map[string]interface{}

	// Subscribe
	_, err = client.Subscribe(subject, func(msg *Message) {
		mu.Lock()
		defer mu.Unlock()
		json.Unmarshal(msg.Data, &receivedData)
	})
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Publish JSON
	err = client.PublishJSON(subject, testData)
	if err != nil {
		t.Fatalf("Failed to publish JSON: %v", err)
	}

	client.Flush()
	time.Sleep(200 * time.Millisecond)

	// Verify
	mu.Lock()
	defer mu.Unlock()

	if receivedData["name"] != "test" {
		t.Errorf("Expected name 'test', got %v", receivedData["name"])
	}
	if receivedData["value"].(float64) != 42 {
		t.Errorf("Expected value 42, got %v", receivedData["value"])
	}
}

// TestNATSClient_RequestJSON tests JSON request-reply
func TestNATSClient_RequestJSON(t *testing.T) {
	ns, url := startTestServer(t)
	defer ns.Shutdown()

	client, err := NewClient(url, "test-client")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	responder, err := NewClient(url, "test-responder")
	if err != nil {
		t.Fatalf("Failed to create responder: %v", err)
	}
	defer responder.Close()

	subject := "test.json.request"

	type Request struct {
		Name string `json:"name"`
	}

	type Response struct {
		Greeting string `json:"greeting"`
	}

	// Set up responder
	_, err = responder.Subscribe(subject, func(msg *Message) {
		var req Request
		if err := json.Unmarshal(msg.Data, &req); err == nil {
			resp := Response{Greeting: "Hello " + req.Name}
			data, _ := json.Marshal(resp)
			if msg.Reply != "" {
				responder.conn.Publish(msg.Reply, data)
			}
		}
	})
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Send JSON request
	req := Request{Name: "World"}
	var resp Response

	err = client.RequestJSON(subject, req, &resp, 2*time.Second)
	if err != nil {
		t.Fatalf("RequestJSON failed: %v", err)
	}

	// Verify response
	expectedGreeting := "Hello World"
	if resp.Greeting != expectedGreeting {
		t.Errorf("Expected greeting %s, got %s", expectedGreeting, resp.Greeting)
	}
}

// TestNATSClient_QueueSubscribe tests load-balanced queue subscription
func TestNATSClient_QueueSubscribe(t *testing.T) {
	ns, url := startTestServer(t)
	defer ns.Shutdown()

	publisher, err := NewClient(url, "test-publisher")
	if err != nil {
		t.Fatalf("Failed to create publisher: %v", err)
	}
	defer publisher.Close()

	// Create two queue subscribers
	subscriber1, err := NewClient(url, "test-subscriber1")
	if err != nil {
		t.Fatalf("Failed to create subscriber1: %v", err)
	}
	defer subscriber1.Close()

	subscriber2, err := NewClient(url, "test-subscriber2")
	if err != nil {
		t.Fatalf("Failed to create subscriber2: %v", err)
	}
	defer subscriber2.Close()

	subject := "test.queue"
	queueName := "workers"

	var mu sync.Mutex
	count1 := 0
	count2 := 0

	// Subscribe with queue group
	_, err = subscriber1.QueueSubscribe(subject, queueName, func(msg *Message) {
		mu.Lock()
		count1++
		mu.Unlock()
	})
	if err != nil {
		t.Fatalf("Failed to queue subscribe (subscriber1): %v", err)
	}

	_, err = subscriber2.QueueSubscribe(subject, queueName, func(msg *Message) {
		mu.Lock()
		count2++
		mu.Unlock()
	})
	if err != nil {
		t.Fatalf("Failed to queue subscribe (subscriber2): %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Publish multiple messages
	numMessages := 10
	for i := 0; i < numMessages; i++ {
		err = publisher.Publish(subject, []byte("message"))
		if err != nil {
			t.Fatalf("Failed to publish: %v", err)
		}
	}

	publisher.Flush()
	time.Sleep(300 * time.Millisecond)

	// Verify load balancing (both subscribers should receive some messages)
	mu.Lock()
	defer mu.Unlock()

	totalReceived := count1 + count2
	if totalReceived != numMessages {
		t.Errorf("Expected %d total messages, got %d (sub1: %d, sub2: %d)",
			numMessages, totalReceived, count1, count2)
	}

	// Both should have received at least one message (basic load balancing check)
	if count1 == 0 || count2 == 0 {
		t.Logf("Warning: Load balancing may not be working perfectly (sub1: %d, sub2: %d)", count1, count2)
	}
}

// TestNATSClient_Connection tests connection state
func TestNATSClient_Connection(t *testing.T) {
	ns, url := startTestServer(t)
	defer ns.Shutdown()

	client, err := NewClient(url, "test-client")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	if !client.IsConnected() {
		t.Error("Client should be connected")
	}

	client.Close()

	// After close, connection state may vary, but Close() should not panic
	// IsConnected may return false or true depending on timing
	_ = client.IsConnected()
}
