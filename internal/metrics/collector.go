package metrics

import (
	"sync"
	"time"

	"github.com/CLIAIMONITOR/internal/types"
)

// Collector aggregates and stores agent metrics
type Collector interface {
	UpdateAgentMetrics(agentID string, metrics *types.AgentMetrics)
	GetAgentMetrics(agentID string) *types.AgentMetrics
	GetAllMetrics() map[string]*types.AgentMetrics
	SetAgentIdle(agentID string)
	SetAgentActive(agentID string)
	TakeSnapshot() types.MetricsSnapshot
	GetHistory() []types.MetricsSnapshot
	ResetHistory()
	IncrementFailedTests(agentID string)
	IncrementConsecutiveRejects(agentID string)
	ResetConsecutiveRejects(agentID string)
	RemoveAgent(agentID string)
}

// MetricsCollector implements Collector
type MetricsCollector struct {
	mu         sync.RWMutex
	metrics    map[string]*types.AgentMetrics
	history    []types.MetricsSnapshot
	maxHistory int
}

// NewCollector creates a new metrics collector
func NewCollector() *MetricsCollector {
	return &MetricsCollector{
		metrics:    make(map[string]*types.AgentMetrics),
		history:    []types.MetricsSnapshot{},
		maxHistory: 1000,
	}
}

// UpdateAgentMetrics updates or creates metrics for an agent
func (c *MetricsCollector) UpdateAgentMetrics(agentID string, metrics *types.AgentMetrics) {
	c.mu.Lock()
	defer c.mu.Unlock()

	existing := c.metrics[agentID]
	if existing == nil {
		c.metrics[agentID] = metrics
		return
	}

	// Merge: only update non-zero values
	if metrics.TokensUsed > 0 {
		existing.TokensUsed = metrics.TokensUsed
	}
	if metrics.EstimatedCost > 0 {
		existing.EstimatedCost = metrics.EstimatedCost
	}
	if metrics.FailedTests > 0 {
		existing.FailedTests = metrics.FailedTests
	}
	if metrics.ConsecutiveRejects > 0 {
		existing.ConsecutiveRejects = metrics.ConsecutiveRejects
	}
	existing.LastUpdated = time.Now()
}

// GetAgentMetrics returns metrics for a specific agent
func (c *MetricsCollector) GetAgentMetrics(agentID string) *types.AgentMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if m, ok := c.metrics[agentID]; ok {
		copy := *m
		return &copy
	}
	return nil
}

// GetAllMetrics returns all agent metrics
func (c *MetricsCollector) GetAllMetrics() map[string]*types.AgentMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string]*types.AgentMetrics)
	for k, v := range c.metrics {
		copy := *v
		result[k] = &copy
	}
	return result
}

// SetAgentIdle marks agent as idle, recording idle start time
func (c *MetricsCollector) SetAgentIdle(agentID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if m, ok := c.metrics[agentID]; ok {
		if m.IdleSince.IsZero() {
			m.IdleSince = time.Now()
		}
	} else {
		c.metrics[agentID] = &types.AgentMetrics{
			AgentID:     agentID,
			IdleSince:   time.Now(),
			LastUpdated: time.Now(),
		}
	}
}

// SetAgentActive clears idle status
func (c *MetricsCollector) SetAgentActive(agentID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if m, ok := c.metrics[agentID]; ok {
		m.IdleSince = time.Time{}
		m.LastUpdated = time.Now()
	}
}

// TakeSnapshot captures current metrics state
func (c *MetricsCollector) TakeSnapshot() types.MetricsSnapshot {
	c.mu.Lock()
	defer c.mu.Unlock()

	snapshot := types.MetricsSnapshot{
		Timestamp: time.Now(),
		Agents:    make(map[string]*types.AgentMetrics),
	}

	for k, v := range c.metrics {
		copy := *v
		snapshot.Agents[k] = &copy
	}

	c.history = append(c.history, snapshot)
	if len(c.history) > c.maxHistory {
		// Prune to exactly maxHistory items
		c.history = c.history[len(c.history)-c.maxHistory:]
	}

	return snapshot
}

// GetHistory returns metrics history
func (c *MetricsCollector) GetHistory() []types.MetricsSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]types.MetricsSnapshot, len(c.history))
	copy(result, c.history)
	return result
}

// ResetHistory clears metrics history
func (c *MetricsCollector) ResetHistory() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.history = []types.MetricsSnapshot{}
}

// IncrementFailedTests increases failed test count
func (c *MetricsCollector) IncrementFailedTests(agentID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if m, ok := c.metrics[agentID]; ok {
		m.FailedTests++
		m.LastUpdated = time.Now()
	}
}

// IncrementConsecutiveRejects increases rejection count
func (c *MetricsCollector) IncrementConsecutiveRejects(agentID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if m, ok := c.metrics[agentID]; ok {
		m.ConsecutiveRejects++
		m.LastUpdated = time.Now()
	}
}

// ResetConsecutiveRejects clears rejection count
func (c *MetricsCollector) ResetConsecutiveRejects(agentID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if m, ok := c.metrics[agentID]; ok {
		m.ConsecutiveRejects = 0
		m.LastUpdated = time.Now()
	}
}

// RemoveAgent removes an agent's metrics
func (c *MetricsCollector) RemoveAgent(agentID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.metrics, agentID)
}
