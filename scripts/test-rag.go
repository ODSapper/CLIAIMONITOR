// +build ignore

package main

import (
	"fmt"
	"os"

	"github.com/CLIAIMONITOR/internal/memory"
)

func main() {
	dbPath := "data/memory.db"
	if len(os.Args) > 1 {
		dbPath = os.Args[1]
	}

	fmt.Println("=== RAG Memory Test ===")
	fmt.Printf("Database: %s\n\n", dbPath)

	// Open database
	db, err := memory.NewMemoryDB(dbPath)
	if err != nil {
		fmt.Printf("ERROR: Failed to open database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// Get learning DB interface
	learningDB := db.AsLearningDB()

	// Store some test knowledge
	fmt.Println("1. Storing test knowledge...")

	knowledge := &memory.Knowledge{
		AgentType: "captain",
		Category:  "pattern",
		Title:     "TF-IDF Search Implementation",
		Content:   "Implemented TF-IDF search for RAG memory. Uses term frequency and inverse document frequency to rank search results without external APIs.",
		Tags:      []string{"tfidf", "search", "rag", "memory"},
	}

	err = learningDB.StoreKnowledge(knowledge)
	if err != nil {
		fmt.Printf("ERROR: Failed to store knowledge: %v\n", err)
	} else {
		fmt.Printf("   Stored knowledge: %s\n", knowledge.Title)
	}

	// Store another piece of knowledge
	knowledge2 := &memory.Knowledge{
		AgentType: "developer",
		Category:  "error_solution",
		Title:     "SQL DECIMAL Scan Fix",
		Content:   "When scanning DECIMAL columns in Go, use float64 instead of string to avoid type mismatch errors.",
		Tags:      []string{"sql", "database", "golang", "decimal"},
	}

	err = learningDB.StoreKnowledge(knowledge2)
	if err != nil {
		fmt.Printf("ERROR: Failed to store knowledge: %v\n", err)
	} else {
		fmt.Printf("   Stored knowledge: %s\n", knowledge2.Title)
	}

	// Record an episode
	fmt.Println("\n2. Recording test episode...")

	episode := &memory.Episode{
		SessionID:  "test-session-001",
		AgentID:    "Captain",
		AgentType:  "captain",
		EventType:  "decision",
		Title:      "Chose TF-IDF for search",
		Content:    "Decided to use TF-IDF for RAG search because it's simple, fast, and doesn't require external APIs.",
		Project:    "CLIAIMONITOR",
		Importance: 0.8,
	}

	err = learningDB.RecordEpisode(episode)
	if err != nil {
		fmt.Printf("ERROR: Failed to record episode: %v\n", err)
	} else {
		fmt.Printf("   Recorded episode: %s\n", episode.Title)
	}

	// Search knowledge
	fmt.Println("\n3. Searching knowledge for 'TF-IDF search'...")

	results, err := learningDB.SearchKnowledge("TF-IDF search", "", 5)
	if err != nil {
		fmt.Printf("ERROR: Failed to search: %v\n", err)
	} else {
		fmt.Printf("   Found %d results:\n", len(results))
		for i, r := range results {
			fmt.Printf("   [%d] %s (score: %.3f)\n", i+1, r.Title, r.RelevanceScore)
			fmt.Printf("       Category: %s, AgentType: %s\n", r.Category, r.AgentType)
		}
	}

	// Search by agent type
	fmt.Println("\n4. Searching knowledge for 'SQL' filtered by developer agent type...")

	results2, err := learningDB.SearchKnowledgeByType("SQL", "developer", "", 5)
	if err != nil {
		fmt.Printf("ERROR: Failed to search: %v\n", err)
	} else {
		fmt.Printf("   Found %d results:\n", len(results2))
		for i, r := range results2 {
			fmt.Printf("   [%d] %s (score: %.3f)\n", i+1, r.Title, r.RelevanceScore)
			fmt.Printf("       Category: %s, AgentType: %s\n", r.Category, r.AgentType)
		}
	}

	// Get recent episodes
	fmt.Println("\n5. Getting recent episodes...")

	episodes, err := learningDB.GetRecentEpisodes("", 5)
	if err != nil {
		fmt.Printf("ERROR: Failed to get episodes: %v\n", err)
	} else {
		fmt.Printf("   Found %d episodes:\n", len(episodes))
		for i, ep := range episodes {
			fmt.Printf("   [%d] %s (%s)\n", i+1, ep.Title, ep.EventType)
			fmt.Printf("       Agent: %s, AgentType: %s, Project: %s\n", ep.AgentID, ep.AgentType, ep.Project)
		}
	}

	// Get stats
	fmt.Println("\n6. Knowledge stats...")

	stats, err := learningDB.GetKnowledgeStats()
	if err != nil {
		fmt.Printf("ERROR: Failed to get stats: %v\n", err)
	} else {
		fmt.Printf("   Total knowledge entries: %d\n", stats.TotalKnowledge)
		fmt.Printf("   Total episodes: %d\n", stats.TotalEpisodes)
		fmt.Printf("   By category: %v\n", stats.ByCategory)
	}

	fmt.Println("\n=== RAG Memory Test Complete ===")
}
