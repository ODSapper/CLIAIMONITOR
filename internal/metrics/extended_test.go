// internal/metrics/extended_test.go
package metrics

import (
	"testing"
	"time"
)

func TestAgentMetricsEfficiency(t *testing.T) {
	m := &ExtendedAgentMetrics{
		TasksCompleted:   5,
		TotalTokens:      50000,
		TotalTimeSeconds: 3600,
	}

	tokensPerTask := m.TokensPerTask()
	if tokensPerTask != 10000 {
		t.Errorf("expected 10000 tokens/task, got %d", tokensPerTask)
	}
}

func TestAgentMetricsHealthStatus(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		metrics  *ExtendedAgentMetrics
		expected HealthStatus
	}{
		{
			name: "healthy",
			metrics: &ExtendedAgentMetrics{
				LastActivity:       now,
				ConsecutiveFailures: 0,
			},
			expected: HealthHealthy,
		},
		{
			name: "idle",
			metrics: &ExtendedAgentMetrics{
				LastActivity:       now.Add(-15 * time.Minute),
				ConsecutiveFailures: 0,
			},
			expected: HealthIdle,
		},
		{
			name: "stuck",
			metrics: &ExtendedAgentMetrics{
				LastActivity:       now.Add(-35 * time.Minute),
				ConsecutiveFailures: 0,
			},
			expected: HealthStuck,
		},
		{
			name: "failing",
			metrics: &ExtendedAgentMetrics{
				LastActivity:       now,
				ConsecutiveFailures: 3,
			},
			expected: HealthFailing,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := tt.metrics.HealthStatus()
			if status != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, status)
			}
		})
	}
}

func TestTeamMetricsAggregation(t *testing.T) {
	team := NewTeamMetrics("team-coop")

	team.AddAgentMetrics("SNTGreen", &ExtendedAgentMetrics{
		TasksCompleted:   3,
		TotalTokens:      30000,
		TotalTimeSeconds: 1800,
	})
	team.AddAgentMetrics("SNTPurple", &ExtendedAgentMetrics{
		TasksCompleted:   2,
		TotalTokens:      20000,
		TotalTimeSeconds: 1200,
	})

	if team.TotalTasks() != 5 {
		t.Errorf("expected 5 total tasks, got %d", team.TotalTasks())
	}
	if team.TotalTokens() != 50000 {
		t.Errorf("expected 50000 total tokens, got %d", team.TotalTokens())
	}
}
