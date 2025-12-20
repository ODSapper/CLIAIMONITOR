package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"

	_ "github.com/mattn/go-sqlite3"
)

type Document struct {
	ID      int    `json:"id"`
	Content string `json:"content"`
	Title   string `json:"title"`
}

func main() {
	dbPath := "C:\\Users\\Admin\\Documents\\VS Projects\\CLIAIMONITOR\\data\\memory.db"

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Query documents table
	rows, err := db.Query("SELECT id, content FROM documents WHERE id IN (1, 2, 3) ORDER BY id")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	docs := make([]Document, 0)
	for rows.Next() {
		var doc Document
		if err := rows.Scan(&doc.ID, &doc.Content); err != nil {
			log.Fatal(err)
		}

		// Determine title based on ID
		switch doc.ID {
		case 1:
			doc.Title = "MAH Fagan Inspection Report"
		case 2:
			doc.Title = "MSS Fagan Inspection Report"
		case 3:
			doc.Title = "ORCH Fagan Inspection Report"
		}

		docs = append(docs, doc)
	}

	// Output as JSON
	output, err := json.MarshalIndent(docs, "", "  ")
	if err != nil {
		log.Fatal(err)
	}

	// Write to file
	outputPath := "C:\\Users\\Admin\\Documents\\VS Projects\\CLIAIMONITOR\\scripts\\inspection_reports.json"
	if err := os.WriteFile(outputPath, output, 0644); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Extracted %d documents to %s\n", len(docs), outputPath)

	// Also print summary
	for _, doc := range docs {
		fmt.Printf("\n=== %s (ID: %d) ===\n", doc.Title, doc.ID)
		fmt.Printf("Content length: %d bytes\n", len(doc.Content))
		fmt.Printf("First 200 chars: %s...\n", truncate(doc.Content, 200))
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
