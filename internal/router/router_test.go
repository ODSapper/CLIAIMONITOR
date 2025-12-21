package router

import (
	"os"
	"testing"

	"github.com/CLIAIMONITOR/internal/memory"
)

func setupTestDB(t *testing.T) (memory.MemoryDB, func()) {
	tmpFile, err := os.CreateTemp("", "test-router-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	db, err := memory.NewMemoryDB(tmpFile.Name())
	if err != nil {
		os.Remove(tmpFile.Name())
		t.Fatal(err)
	}

	cleanup := func() {
		db.Close()
		os.Remove(tmpFile.Name())
	}

	return db, cleanup
}

func TestClassifyQuery(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	router := NewSkillRouter(db)

	tests := []struct {
		query    string
		expected QueryType
	}{
		// Knowledge queries
		{"how do I fix auth redirect", QueryTypeKnowledge},
		{"what is the solution for SQL error", QueryTypeKnowledge},
		{"best practice for error handling", QueryTypeKnowledge},
		{"pattern for authentication", QueryTypeKnowledge},
		{"gotcha with React hooks", QueryTypeKnowledge},

		// Episode queries
		{"what happened last session", QueryTypeEpisode},
		{"what did we do before", QueryTypeEpisode},
		{"show history of changes", QueryTypeEpisode},
		{"previous decisions made", QueryTypeEpisode},

		// Operational queries
		{"what agents are running", QueryTypeOperational},
		{"show pending tasks", QueryTypeOperational},
		{"which agents are active", QueryTypeOperational},
		{"assigned task status", QueryTypeOperational},

		// Recon queries
		{"vulnerability scan results", QueryTypeRecon},
		{"critical security findings", QueryTypeRecon},
		{"CVE exposure in MSS", QueryTypeRecon},
		{"threat risk assessment", QueryTypeRecon},

		// Unknown - should still work
		{"random query", QueryTypeUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			result := router.ClassifyQuery(tt.query)
			if result != tt.expected {
				t.Errorf("ClassifyQuery(%q) = %s, want %s", tt.query, result, tt.expected)
			}
		})
	}
}

func TestRouteQueryKnowledge(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Seed some knowledge
	learningDB := db.AsLearningDB()
	learningDB.StoreKnowledge(&memory.Knowledge{
		Category: "error_solution",
		Title:    "SQL DECIMAL scan fix",
		Content:  "Use float64 when scanning DECIMAL columns",
		Tags:     []string{"sql", "database"},
	})
	learningDB.StoreKnowledge(&memory.Knowledge{
		Category: "pattern",
		Title:    "Auth redirect pattern",
		Content:  "Store original URL before redirect, restore after login",
		Tags:     []string{"auth", "redirect"},
	})

	router := NewSkillRouter(db)

	// Test knowledge routing
	result, err := router.RouteQuery("how do I fix SQL DECIMAL error", 5)
	if err != nil {
		t.Fatalf("RouteQuery failed: %v", err)
	}

	if result.QueryType != QueryTypeKnowledge {
		t.Errorf("Expected knowledge query type, got %s", result.QueryType)
	}
	if result.Source != "learning.db" {
		t.Errorf("Expected learning.db source, got %s", result.Source)
	}
	if result.Count == 0 {
		t.Error("Expected at least one result")
	}
}

func TestRouteQueryEpisodes(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Seed some episodes
	learningDB := db.AsLearningDB()
	learningDB.RecordEpisode(&memory.Episode{
		SessionID:  "session-1",
		AgentID:    "Captain",
		EventType:  "decision",
		Title:      "Chose TF-IDF approach",
		Content:    "Decided to use TF-IDF for simplicity",
		Project:    "CLIAIMONITOR",
		Importance: 0.8,
	})

	router := NewSkillRouter(db)

	// Test episode routing
	result, err := router.RouteQuery("what happened in previous session", 5)
	if err != nil {
		t.Fatalf("RouteQuery failed: %v", err)
	}

	if result.QueryType != QueryTypeEpisode {
		t.Errorf("Expected episode query type, got %s", result.QueryType)
	}
	if result.Source != "learning.db" {
		t.Errorf("Expected learning.db source, got %s", result.Source)
	}
}

func TestRouteQueryOperational(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	router := NewSkillRouter(db)

	// Test operational routing (agent queries now handled via in-memory store)
	result, err := router.RouteQuery("what agents are running", 10)
	if err != nil {
		t.Fatalf("RouteQuery failed: %v", err)
	}

	if result.QueryType != QueryTypeOperational {
		t.Errorf("Expected operational query type, got %s", result.QueryType)
	}
	if result.Source != "operational.db" {
		t.Errorf("Expected operational.db source, got %s", result.Source)
	}
}

func TestAgentCommsHeartbeat(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	comms := NewAgentComms(db)

	// Test heartbeat processing
	req := &HeartbeatRequest{
		AgentID:     "test-agent-001",
		ConfigName:  "developer",
		ProjectPath: "/path/to/project",
		Status:      "working",
		CurrentTask: "implementing feature X",
	}

	resp, err := comms.ProcessHeartbeat(req)
	if err != nil {
		t.Fatalf("ProcessHeartbeat failed: %v", err)
	}

	if !resp.OK {
		t.Error("Expected OK response")
	}
	if resp.ShouldStop {
		t.Error("Should not stop (no shutdown flag)")
	}
}

func TestAgentCommsShutdown(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	comms := NewAgentComms(db)

	// Register shutdown channel first
	comms.RegisterShutdownChannel("test-agent-001")

	// Trigger shutdown
	comms.TriggerShutdown("test-agent-001")

	// Check shutdown
	check, err := comms.CheckShutdown("test-agent-001")
	if err != nil {
		t.Fatalf("CheckShutdown failed: %v", err)
	}

	if !check.ShouldStop {
		t.Error("Expected ShouldStop = true")
	}
	if check.Reason != "test shutdown" {
		t.Errorf("Expected reason 'test shutdown', got '%s'", check.Reason)
	}
}
