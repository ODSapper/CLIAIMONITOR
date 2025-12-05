package main

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"
)

// Subjects originated by Captain (only forward Captain -> Sergeant)
var captainSubjects = []string{
	"captain.status",
	"captain.commands",
	"captain.decision",
	"escalation.response.*",
}

// Subjects originated by Sergeant/Agents (only forward Sergeant -> Captain)
var sergeantSubjects = []string{
	"agent.*.heartbeat",
	"agent.*.status",
	"escalation.create",
	"sergeant.status",
	"sergeant.report",
}

// Bidirectional subjects (need dedup)
var bidirectionalSubjects = []string{
	"system.broadcast",
}

// RecentMessages tracks recently seen messages to prevent loops
type RecentMessages struct {
	mu    sync.Mutex
	seen  map[string]time.Time
	ttl   time.Duration
}

func NewRecentMessages(ttl time.Duration) *RecentMessages {
	rm := &RecentMessages{
		seen: make(map[string]time.Time),
		ttl:  ttl,
	}
	// Cleanup goroutine
	go func() {
		for {
			time.Sleep(ttl)
			rm.cleanup()
		}
	}()
	return rm
}

func (rm *RecentMessages) hash(subject string, data []byte) string {
	h := sha256.New()
	h.Write([]byte(subject))
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))[:16]
}

func (rm *RecentMessages) IsSeen(subject string, data []byte) bool {
	hash := rm.hash(subject, data)
	rm.mu.Lock()
	defer rm.mu.Unlock()
	_, exists := rm.seen[hash]
	return exists
}

func (rm *RecentMessages) Mark(subject string, data []byte) {
	hash := rm.hash(subject, data)
	rm.mu.Lock()
	rm.seen[hash] = time.Now()
	rm.mu.Unlock()
}

func (rm *RecentMessages) cleanup() {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	now := time.Now()
	for hash, ts := range rm.seen {
		if now.Sub(ts) > rm.ttl {
			delete(rm.seen, hash)
		}
	}
}

func main() {
	captainURL := flag.String("captain", "nats://localhost:4222", "Captain's NATS URL")
	sergeantURL := flag.String("sergeant", "nats://localhost:4223", "Sergeant's NATS URL")
	flag.Parse()

	log.Println("===============================================")
	log.Println("  NATS Bridge - Captain <-> Sergeant")
	log.Println("===============================================")
	log.Printf("Captain NATS:  %s", *captainURL)
	log.Printf("Sergeant NATS: %s", *sergeantURL)

	// Connect to Captain's NATS
	captainConn, err := nats.Connect(*captainURL, nats.Name("bridge-to-captain"))
	if err != nil {
		log.Fatalf("Failed to connect to Captain NATS: %v", err)
	}
	defer captainConn.Close()
	log.Println("[BRIDGE] Connected to Captain NATS")

	// Connect to Sergeant's NATS
	sergeantConn, err := nats.Connect(*sergeantURL, nats.Name("bridge-to-sergeant"))
	if err != nil {
		log.Fatalf("Failed to connect to Sergeant NATS: %v", err)
	}
	defer sergeantConn.Close()
	log.Println("[BRIDGE] Connected to Sergeant NATS")

	// For bidirectional subjects, track recent messages
	recent := NewRecentMessages(5 * time.Second)

	subCount := 0

	// Captain-originated subjects: only forward Captain -> Sergeant
	for _, subject := range captainSubjects {
		subj := subject
		_, err := captainConn.Subscribe(subj, func(msg *nats.Msg) {
			log.Printf("[CAPTAIN->SGT] %s (%d bytes)", msg.Subject, len(msg.Data))
			sergeantConn.Publish(msg.Subject, msg.Data)
		})
		if err != nil {
			log.Printf("[BRIDGE] Warning: Failed to subscribe to %s on Captain: %v", subj, err)
		} else {
			subCount++
		}
	}

	// Sergeant-originated subjects: only forward Sergeant -> Captain
	for _, subject := range sergeantSubjects {
		subj := subject
		_, err := sergeantConn.Subscribe(subj, func(msg *nats.Msg) {
			log.Printf("[SGT->CAPTAIN] %s (%d bytes)", msg.Subject, len(msg.Data))
			captainConn.Publish(msg.Subject, msg.Data)
		})
		if err != nil {
			log.Printf("[BRIDGE] Warning: Failed to subscribe to %s on Sergeant: %v", subj, err)
		} else {
			subCount++
		}
	}

	// Bidirectional subjects: forward both ways with dedup
	for _, subject := range bidirectionalSubjects {
		subj := subject

		// Captain -> Sergeant
		_, err := captainConn.Subscribe(subj, func(msg *nats.Msg) {
			if recent.IsSeen(msg.Subject, msg.Data) {
				return // Skip duplicate
			}
			recent.Mark(msg.Subject, msg.Data)
			log.Printf("[CAPTAIN->SGT] %s (%d bytes)", msg.Subject, len(msg.Data))
			sergeantConn.Publish(msg.Subject, msg.Data)
		})
		if err != nil {
			log.Printf("[BRIDGE] Warning: Failed to subscribe to %s on Captain: %v", subj, err)
		} else {
			subCount++
		}

		// Sergeant -> Captain
		_, err = sergeantConn.Subscribe(subj, func(msg *nats.Msg) {
			if recent.IsSeen(msg.Subject, msg.Data) {
				return // Skip duplicate
			}
			recent.Mark(msg.Subject, msg.Data)
			log.Printf("[SGT->CAPTAIN] %s (%d bytes)", msg.Subject, len(msg.Data))
			captainConn.Publish(msg.Subject, msg.Data)
		})
		if err != nil {
			log.Printf("[BRIDGE] Warning: Failed to subscribe to %s on Sergeant: %v", subj, err)
		} else {
			subCount++
		}
	}

	log.Printf("[BRIDGE] Active subscriptions: %d", subCount)
	log.Println("===============================================")
	log.Println("  Bridge running. Press Ctrl+C to stop.")
	log.Println("===============================================")

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("[BRIDGE] Shutting down...")
}
