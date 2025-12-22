package mcp

import (
	"fmt"
	"net/http"
	"sync"
)

const (
	// MaxConnectionsPerAgent limits connections per individual agent
	MaxConnectionsPerAgent = 5
	// MaxTotalConnections limits total SSE connections across all agents
	MaxTotalConnections = 100
)

// ConnectionLimiter tracks and limits SSE connections
type ConnectionLimiter struct {
	mu                sync.RWMutex
	perAgentCount     map[string]int // agentID -> connection count
	totalConnections  int
	maxPerAgent       int
	maxTotal          int
}

// NewConnectionLimiter creates a new connection limiter
func NewConnectionLimiter(maxPerAgent, maxTotal int) *ConnectionLimiter {
	return &ConnectionLimiter{
		perAgentCount: make(map[string]int),
		maxPerAgent:   maxPerAgent,
		maxTotal:      maxTotal,
	}
}

// TryAcquire attempts to acquire a connection slot for the given agent
// Returns true if allowed, false if limit exceeded
func (cl *ConnectionLimiter) TryAcquire(agentID string) bool {
	cl.mu.Lock()
	defer cl.mu.Unlock()

	// Check global limit
	if cl.totalConnections >= cl.maxTotal {
		return false
	}

	// Check per-agent limit
	currentCount := cl.perAgentCount[agentID]
	if currentCount >= cl.maxPerAgent {
		return false
	}

	// Acquire slot
	cl.perAgentCount[agentID]++
	cl.totalConnections++
	return true
}

// Release releases a connection slot for the given agent
func (cl *ConnectionLimiter) Release(agentID string) {
	cl.mu.Lock()
	defer cl.mu.Unlock()

	if count, ok := cl.perAgentCount[agentID]; ok && count > 0 {
		cl.perAgentCount[agentID]--
		if cl.perAgentCount[agentID] == 0 {
			delete(cl.perAgentCount, agentID)
		}
		cl.totalConnections--
	}
}

// GetStats returns current connection statistics
func (cl *ConnectionLimiter) GetStats() (perAgent map[string]int, total int) {
	cl.mu.RLock()
	defer cl.mu.RUnlock()

	// Create a copy to avoid race conditions
	perAgent = make(map[string]int, len(cl.perAgentCount))
	for k, v := range cl.perAgentCount {
		perAgent[k] = v
	}
	total = cl.totalConnections
	return
}

// HandleLimitExceeded sends a 429 Too Many Requests response
func (cl *ConnectionLimiter) HandleLimitExceeded(w http.ResponseWriter, agentID string) {
	cl.mu.RLock()
	currentCount := cl.perAgentCount[agentID]
	totalCount := cl.totalConnections
	cl.mu.RUnlock()

	var message string
	if totalCount >= cl.maxTotal {
		message = fmt.Sprintf("Global connection limit exceeded (%d/%d connections)", totalCount, cl.maxTotal)
	} else if currentCount >= cl.maxPerAgent {
		message = fmt.Sprintf("Per-agent connection limit exceeded for %s (%d/%d connections)", agentID, currentCount, cl.maxPerAgent)
	} else {
		message = "Connection limit exceeded"
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Retry-After", "10") // Suggest retry after 10 seconds
	w.WriteHeader(http.StatusTooManyRequests)

	// Send JSON error response
	fmt.Fprintf(w, `{"error": "%s", "error_code": "ERR_429", "retry_after": 10}`, message)
}
