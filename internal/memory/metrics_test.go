package memory

import (
	"os"
	"testing"
)

func TestRecordMetricsHistory(t *testing.T) {
	// Create temporary database
	tmpFile := "test_metrics.db"
	defer os.Remove(tmpFile)
	defer os.Remove(tmpFile + "-shm")
	defer os.Remove(tmpFile + "-wal")

	db, err := NewMemoryDB(tmpFile)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	// Test recording metrics history
	err = db.RecordMetricsHistory(
		"test-agent-1",
		"claude-sonnet-4-5",
		50000,
		0.25,
		"TASK-123",
	)
	if err != nil {
		t.Errorf("RecordMetricsHistory failed: %v", err)
	}

	// Test recording with empty task ID
	err = db.RecordMetricsHistory(
		"test-agent-2",
		"claude-haiku-4",
		10000,
		0.05,
		"",
	)
	if err != nil {
		t.Errorf("RecordMetricsHistory with empty task_id failed: %v", err)
	}

	// Verify the data was recorded by querying the view
	metrics, err := db.GetMetricsByModel("")
	if err != nil {
		t.Fatalf("GetMetricsByModel failed: %v", err)
	}

	if len(metrics) == 0 {
		t.Errorf("Expected at least one model in metrics, got none")
	}

	// Check that we have both models
	modelMap := make(map[string]*ModelMetrics)
	for _, m := range metrics {
		modelMap[m.Model] = m
	}

	if sonnet, ok := modelMap["claude-sonnet-4-5"]; ok {
		if sonnet.TotalTokens != 50000 {
			t.Errorf("Expected 50000 tokens for sonnet, got %d", sonnet.TotalTokens)
		}
		if sonnet.TotalCost != 0.25 {
			t.Errorf("Expected 0.25 cost for sonnet, got %f", sonnet.TotalCost)
		}
	} else {
		t.Errorf("Expected to find claude-sonnet-4-5 in metrics")
	}

	if haiku, ok := modelMap["claude-haiku-4"]; ok {
		if haiku.TotalTokens != 10000 {
			t.Errorf("Expected 10000 tokens for haiku, got %d", haiku.TotalTokens)
		}
		if haiku.TotalCost != 0.05 {
			t.Errorf("Expected 0.05 cost for haiku, got %f", haiku.TotalCost)
		}
	} else {
		t.Errorf("Expected to find claude-haiku-4 in metrics")
	}
}

func TestGetMetricsByModelFilter(t *testing.T) {
	// Create temporary database
	tmpFile := "test_metrics_filter.db"
	defer os.Remove(tmpFile)
	defer os.Remove(tmpFile + "-shm")
	defer os.Remove(tmpFile + "-wal")

	db, err := NewMemoryDB(tmpFile)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	// Record multiple metrics for different models
	db.RecordMetricsHistory("agent1", "claude-sonnet-4-5", 50000, 0.25, "TASK-1")
	db.RecordMetricsHistory("agent2", "claude-sonnet-4-5", 30000, 0.15, "TASK-2")
	db.RecordMetricsHistory("agent3", "claude-haiku-4", 10000, 0.05, "TASK-3")

	// Test filtering by model
	metrics, err := db.GetMetricsByModel("claude-sonnet-4-5")
	if err != nil {
		t.Fatalf("GetMetricsByModel failed: %v", err)
	}

	if len(metrics) != 1 {
		t.Errorf("Expected 1 model in filtered results, got %d", len(metrics))
	}

	if metrics[0].Model != "claude-sonnet-4-5" {
		t.Errorf("Expected claude-sonnet-4-5, got %s", metrics[0].Model)
	}

	if metrics[0].TotalTokens != 80000 {
		t.Errorf("Expected 80000 total tokens, got %d", metrics[0].TotalTokens)
	}

	if metrics[0].ReportCount != 2 {
		t.Errorf("Expected 2 reports, got %d", metrics[0].ReportCount)
	}
}
