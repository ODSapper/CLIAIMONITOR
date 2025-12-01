package metrics

import (
	"fmt"
	"sync"
	"time"

	"github.com/CLIAIMONITOR/internal/types"
	"github.com/google/uuid"
)

// AlertEngine checks metrics against thresholds and generates alerts
type AlertEngine interface {
	SetThresholds(thresholds types.AlertThresholds)
	GetThresholds() types.AlertThresholds
	CheckMetrics(metrics map[string]*types.AgentMetrics) []*types.Alert
	CheckAgentStatus(agents map[string]*types.Agent) []*types.Alert
	CheckEscalationQueue(pendingCount int) *types.Alert
}

// AlertChecker implements AlertEngine
type AlertChecker struct {
	mu         sync.RWMutex
	thresholds types.AlertThresholds
	// Track alerts to avoid duplicates
	recentAlerts map[string]time.Time
}

// NewAlertEngine creates a new alert engine
func NewAlertEngine(thresholds types.AlertThresholds) *AlertChecker {
	return &AlertChecker{
		thresholds:   thresholds,
		recentAlerts: make(map[string]time.Time),
	}
}

// SetThresholds updates alert thresholds
func (a *AlertChecker) SetThresholds(thresholds types.AlertThresholds) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.thresholds = thresholds
}

// GetThresholds returns current thresholds
func (a *AlertChecker) GetThresholds() types.AlertThresholds {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.thresholds
}

// shouldAlert checks if we should create an alert (avoids duplicates)
func (a *AlertChecker) shouldAlert(key string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Clean old alerts (older than 5 minutes)
	now := time.Now()
	for k, t := range a.recentAlerts {
		if now.Sub(t) > 5*time.Minute {
			delete(a.recentAlerts, k)
		}
	}

	// Check if we recently sent this alert
	if _, exists := a.recentAlerts[key]; exists {
		return false
	}

	a.recentAlerts[key] = now
	return true
}

// CheckMetrics examines all agent metrics and returns alerts
func (a *AlertChecker) CheckMetrics(metrics map[string]*types.AgentMetrics) []*types.Alert {
	a.mu.RLock()
	thresholds := a.thresholds
	a.mu.RUnlock()

	var alerts []*types.Alert

	for agentID, m := range metrics {
		// Check failed tests
		if thresholds.FailedTestsMax > 0 && m.FailedTests >= thresholds.FailedTestsMax {
			key := fmt.Sprintf("failed_tests_%s", agentID)
			if a.shouldAlert(key) {
				alerts = append(alerts, &types.Alert{
					ID:        uuid.New().String(),
					Type:      "failed_tests",
					AgentID:   agentID,
					Message:   fmt.Sprintf("Agent %s has %d failed tests (threshold: %d)", agentID, m.FailedTests, thresholds.FailedTestsMax),
					Severity:  "warning",
					CreatedAt: time.Now(),
				})
			}
		}

		// Check idle time
		if thresholds.IdleTimeMaxSeconds > 0 && !m.IdleSince.IsZero() {
			idleSeconds := int(time.Since(m.IdleSince).Seconds())
			if idleSeconds >= thresholds.IdleTimeMaxSeconds {
				key := fmt.Sprintf("idle_%s", agentID)
				if a.shouldAlert(key) {
					alerts = append(alerts, &types.Alert{
						ID:        uuid.New().String(),
						Type:      "idle_timeout",
						AgentID:   agentID,
						Message:   fmt.Sprintf("Agent %s has been idle for %d seconds", agentID, idleSeconds),
						Severity:  "warning",
						CreatedAt: time.Now(),
					})
				}
			}
		}

		// Check token usage
		if thresholds.TokenUsageMax > 0 && m.TokensUsed >= thresholds.TokenUsageMax {
			key := fmt.Sprintf("tokens_%s", agentID)
			if a.shouldAlert(key) {
				alerts = append(alerts, &types.Alert{
					ID:        uuid.New().String(),
					Type:      "token_usage",
					AgentID:   agentID,
					Message:   fmt.Sprintf("Agent %s has used %d tokens (threshold: %d)", agentID, m.TokensUsed, thresholds.TokenUsageMax),
					Severity:  "warning",
					CreatedAt: time.Now(),
				})
			}
		}

		// Check consecutive rejects
		if thresholds.ConsecutiveRejectsMax > 0 && m.ConsecutiveRejects >= thresholds.ConsecutiveRejectsMax {
			key := fmt.Sprintf("rejects_%s", agentID)
			if a.shouldAlert(key) {
				alerts = append(alerts, &types.Alert{
					ID:        uuid.New().String(),
					Type:      "consecutive_rejects",
					AgentID:   agentID,
					Message:   fmt.Sprintf("Agent %s has %d consecutive rejections", agentID, m.ConsecutiveRejects),
					Severity:  "critical",
					CreatedAt: time.Now(),
				})
			}
		}
	}

	return alerts
}

// CheckAgentStatus checks agent connection status
func (a *AlertChecker) CheckAgentStatus(agents map[string]*types.Agent) []*types.Alert {
	var alerts []*types.Alert

	for agentID, agent := range agents {
		// Check for disconnected agents
		if agent.Status == types.StatusDisconnected {
			key := fmt.Sprintf("disconnected_%s", agentID)
			if a.shouldAlert(key) {
				alerts = append(alerts, &types.Alert{
					ID:        uuid.New().String(),
					Type:      "agent_disconnected",
					AgentID:   agentID,
					Message:   fmt.Sprintf("Agent %s has disconnected", agentID),
					Severity:  "critical",
					CreatedAt: time.Now(),
				})
			}
		}

		// Check for blocked agents
		if agent.Status == types.StatusBlocked {
			key := fmt.Sprintf("blocked_%s", agentID)
			if a.shouldAlert(key) {
				alerts = append(alerts, &types.Alert{
					ID:        uuid.New().String(),
					Type:      "agent_blocked",
					AgentID:   agentID,
					Message:   fmt.Sprintf("Agent %s is blocked: %s", agentID, agent.CurrentTask),
					Severity:  "warning",
					CreatedAt: time.Now(),
				})
			}
		}
	}

	return alerts
}

// CheckEscalationQueue checks pending escalation count
func (a *AlertChecker) CheckEscalationQueue(pendingCount int) *types.Alert {
	a.mu.RLock()
	thresholds := a.thresholds
	a.mu.RUnlock()

	if thresholds.EscalationQueueMax <= 0 {
		return nil
	}

	if pendingCount >= thresholds.EscalationQueueMax {
		key := "escalation_queue"
		if a.shouldAlert(key) {
			return &types.Alert{
				ID:        uuid.New().String(),
				Type:      "escalation_queue",
				Message:   fmt.Sprintf("Escalation queue has %d items (threshold: %d)", pendingCount, thresholds.EscalationQueueMax),
				Severity:  "critical",
				CreatedAt: time.Now(),
			}
		}
	}

	return nil
}
