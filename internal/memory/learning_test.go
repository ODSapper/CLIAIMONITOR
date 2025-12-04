package memory

import (
	"os"
	"testing"
)

func TestLearningDBKnowledge(t *testing.T) {
	// Create temp db
	tmpFile, err := os.CreateTemp("", "test-learning-*.db")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	db, err := NewMemoryDB(tmpFile.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	learningDB := db.AsLearningDB()

	// Store some knowledge
	k1 := &Knowledge{
		Category: "error_solution",
		Title:    "SQL DECIMAL scan error fix",
		Content:  "Use float64 intermediates when scanning DECIMAL columns to int64",
		Tags:     []string{"sql", "database", "golang"},
	}
	if err := learningDB.StoreKnowledge(k1); err != nil {
		t.Fatalf("StoreKnowledge failed: %v", err)
	}
	if k1.ID == "" {
		t.Error("Knowledge ID should be set after store")
	}

	k2 := &Knowledge{
		Category: "pattern",
		Title:    "AuthProvider public paths pattern",
		Content:  "When using AuthProvider in Next.js, define public paths array for unauthenticated routes",
		Tags:     []string{"nextjs", "auth", "pattern"},
	}
	if err := learningDB.StoreKnowledge(k2); err != nil {
		t.Fatalf("StoreKnowledge k2 failed: %v", err)
	}

	k3 := &Knowledge{
		Category: "gotcha",
		Title:    "PATCH not PUT for partial updates",
		Content:  "Use PATCH method not PUT when updating single fields in REST API",
		Tags:     []string{"api", "rest", "http"},
	}
	if err := learningDB.StoreKnowledge(k3); err != nil {
		t.Fatalf("StoreKnowledge k3 failed: %v", err)
	}

	// Search for knowledge
	t.Run("SearchSQL", func(t *testing.T) {
		results, err := learningDB.SearchKnowledge("SQL DECIMAL error", "", 5)
		if err != nil {
			t.Fatalf("SearchKnowledge failed: %v", err)
		}
		if len(results) == 0 {
			t.Error("Expected at least one result for SQL search")
		}
		if len(results) > 0 && results[0].ID != k1.ID {
			t.Errorf("Expected k1 to be first result, got %s", results[0].Title)
		}
	})

	t.Run("SearchAuth", func(t *testing.T) {
		results, err := learningDB.SearchKnowledge("AuthProvider NextJS", "", 5)
		if err != nil {
			t.Fatalf("SearchKnowledge failed: %v", err)
		}
		if len(results) == 0 {
			t.Error("Expected at least one result for auth search")
		}
		if len(results) > 0 && results[0].ID != k2.ID {
			t.Errorf("Expected k2 to be first result, got %s", results[0].Title)
		}
	})

	t.Run("SearchWithCategory", func(t *testing.T) {
		// Search for PATCH which is unique to the gotcha entry
		results, err := learningDB.SearchKnowledge("PATCH PUT REST", "gotcha", 5)
		if err != nil {
			t.Fatalf("SearchKnowledge failed: %v", err)
		}
		if len(results) == 0 {
			t.Error("Expected at least one result for PATCH search with gotcha category")
		}
		if len(results) > 0 && results[0].Category != "gotcha" {
			t.Errorf("Expected gotcha category, got %s", results[0].Category)
		}
	})

	// Verify GetKnowledge
	t.Run("GetKnowledge", func(t *testing.T) {
		k, err := learningDB.GetKnowledge(k1.ID)
		if err != nil {
			t.Fatalf("GetKnowledge failed: %v", err)
		}
		if k.Title != k1.Title {
			t.Errorf("Expected title %s, got %s", k1.Title, k.Title)
		}
	})

	// Verify use count increment
	t.Run("IncrementUseCount", func(t *testing.T) {
		err := learningDB.IncrementUseCount(k1.ID)
		if err != nil {
			t.Fatalf("IncrementUseCount failed: %v", err)
		}
		k, _ := learningDB.GetKnowledge(k1.ID)
		if k.UseCount != 1 {
			t.Errorf("Expected use count 1, got %d", k.UseCount)
		}
	})
}

func TestLearningDBEpisodes(t *testing.T) {
	// Create temp db
	tmpFile, err := os.CreateTemp("", "test-episodes-*.db")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	db, err := NewMemoryDB(tmpFile.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	learningDB := db.AsLearningDB()

	// Record some episodes
	ep1 := &Episode{
		SessionID:  "session-123",
		AgentID:    "Captain",
		EventType:  "decision",
		Title:      "Chose TF-IDF over embeddings",
		Content:    "Decided to use TF-IDF for initial RAG implementation to keep it self-contained",
		Project:    "CLIAIMONITOR",
		Importance: 0.8,
	}
	if err := learningDB.RecordEpisode(ep1); err != nil {
		t.Fatalf("RecordEpisode failed: %v", err)
	}
	if ep1.ID == "" {
		t.Error("Episode ID should be set after record")
	}

	ep2 := &Episode{
		SessionID:  "session-123",
		AgentID:    "Captain",
		EventType:  "error",
		Title:      "Build failed due to sql shadowing",
		Content:    "Variable named sql shadowed the database/sql package, renamed to queryStr",
		Project:    "CLIAIMONITOR",
		Importance: 0.6,
	}
	if err := learningDB.RecordEpisode(ep2); err != nil {
		t.Fatalf("RecordEpisode failed: %v", err)
	}

	ep3 := &Episode{
		SessionID:  "session-456",
		AgentID:    "Snake-001",
		EventType:  "action",
		Title:      "Completed security scan",
		Content:    "Finished scanning MSS for vulnerabilities, found 3 high severity issues",
		Project:    "MSS",
		Importance: 0.7,
	}
	if err := learningDB.RecordEpisode(ep3); err != nil {
		t.Fatalf("RecordEpisode failed: %v", err)
	}

	// Get recent episodes for session
	t.Run("GetRecentEpisodes", func(t *testing.T) {
		episodes, err := learningDB.GetRecentEpisodes("session-123", 10)
		if err != nil {
			t.Fatalf("GetRecentEpisodes failed: %v", err)
		}
		if len(episodes) != 2 {
			t.Errorf("Expected 2 episodes for session-123, got %d", len(episodes))
		}
	})

	// Get all recent episodes
	t.Run("GetAllRecentEpisodes", func(t *testing.T) {
		episodes, err := learningDB.GetRecentEpisodes("", 10)
		if err != nil {
			t.Fatalf("GetRecentEpisodes failed: %v", err)
		}
		if len(episodes) != 3 {
			t.Errorf("Expected 3 total episodes, got %d", len(episodes))
		}
	})

	// Search episodes
	t.Run("SearchEpisodes", func(t *testing.T) {
		episodes, err := learningDB.SearchEpisodes("TF-IDF embeddings", "", 5)
		if err != nil {
			t.Fatalf("SearchEpisodes failed: %v", err)
		}
		if len(episodes) == 0 {
			t.Error("Expected at least one result for TF-IDF search")
		}
	})

	// Search with project filter
	t.Run("SearchEpisodesWithProject", func(t *testing.T) {
		episodes, err := learningDB.SearchEpisodes("scan", "MSS", 5)
		if err != nil {
			t.Fatalf("SearchEpisodes failed: %v", err)
		}
		if len(episodes) == 0 {
			t.Error("Expected at least one result for MSS project search")
		}
		if len(episodes) > 0 && episodes[0].Project != "MSS" {
			t.Errorf("Expected MSS project, got %s", episodes[0].Project)
		}
	})
}

func TestKnowledgeStats(t *testing.T) {
	// Create temp db
	tmpFile, err := os.CreateTemp("", "test-stats-*.db")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	db, err := NewMemoryDB(tmpFile.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	learningDB := db.AsLearningDB()

	// Store knowledge in different categories
	learningDB.StoreKnowledge(&Knowledge{Category: "error_solution", Title: "Fix 1", Content: "Content 1"})
	learningDB.StoreKnowledge(&Knowledge{Category: "error_solution", Title: "Fix 2", Content: "Content 2"})
	learningDB.StoreKnowledge(&Knowledge{Category: "pattern", Title: "Pattern 1", Content: "Content 3"})

	// Record some episodes
	learningDB.RecordEpisode(&Episode{SessionID: "s1", AgentID: "a1", EventType: "action", Title: "Action 1", Content: "C1"})
	learningDB.RecordEpisode(&Episode{SessionID: "s1", AgentID: "a1", EventType: "error", Title: "Error 1", Content: "C2"})

	// Get stats
	stats, err := learningDB.GetKnowledgeStats()
	if err != nil {
		t.Fatalf("GetKnowledgeStats failed: %v", err)
	}

	if stats.TotalKnowledge != 3 {
		t.Errorf("Expected 3 total knowledge, got %d", stats.TotalKnowledge)
	}
	if stats.TotalEpisodes != 2 {
		t.Errorf("Expected 2 total episodes, got %d", stats.TotalEpisodes)
	}
	if stats.ByCategory["error_solution"] != 2 {
		t.Errorf("Expected 2 error_solution, got %d", stats.ByCategory["error_solution"])
	}
	if stats.ByCategory["pattern"] != 1 {
		t.Errorf("Expected 1 pattern, got %d", stats.ByCategory["pattern"])
	}
}
