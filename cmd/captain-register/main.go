package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/nats-io/nats.go"
)

type CaptainStatusMessage struct {
	Status    string    `json:"status"`
	CurrentOp string    `json:"current_op,omitempty"`
	QueueSize int       `json:"queue_size"`
	Timestamp time.Time `json:"timestamp"`
}

func main() {
	natsURL := flag.String("url", "nats://127.0.0.1:4222", "NATS server URL")
	status := flag.String("status", "idle", "Captain status (idle, busy, error)")
	currentOp := flag.String("op", "", "Current operation description")
	flag.Parse()

	nc, err := nats.Connect(*natsURL)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer nc.Close()

	msg := CaptainStatusMessage{
		Status:    *status,
		CurrentOp: *currentOp,
		QueueSize: 0,
		Timestamp: time.Now(),
	}

	data, err := json.Marshal(msg)
	if err != nil {
		log.Fatalf("Failed to marshal message: %v", err)
	}

	if err := nc.Publish("captain.status", data); err != nil {
		log.Fatalf("Failed to publish: %v", err)
	}

	nc.Flush()
	fmt.Printf("Captain registered with status: %s\n", *status)
}
