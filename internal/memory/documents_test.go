package memory

import (
	"path/filepath"
	"testing"
)

func TestDocumentCRUD(t *testing.T) {
	// Create temp database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_documents.db")

	db, err := NewMemoryDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Test CreateDocument
	doc := &Document{
		DocType:   "plan",
		Title:     "Test Implementation Plan",
		Content:   "# Plan\n\nImplement feature X",
		Format:    "markdown",
		AuthorID:  "captain",
		ProjectID: "CLIAIMONITOR",
		TaskID:    "TASK-123",
		Tags:      []string{"plan", "feature-x"},
		Status:    "active",
		Version:   1,
	}

	err = db.CreateDocument(doc)
	if err != nil {
		t.Fatalf("Failed to create document: %v", err)
	}
	if doc.ID == 0 {
		t.Error("Document ID should be set after creation")
	}

	// Test GetDocument
	retrieved, err := db.GetDocument(doc.ID)
	if err != nil {
		t.Fatalf("Failed to get document: %v", err)
	}
	if retrieved.Title != doc.Title {
		t.Errorf("Expected title %s, got %s", doc.Title, retrieved.Title)
	}
	if len(retrieved.Tags) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(retrieved.Tags))
	}

	// Test UpdateDocument
	retrieved.Content = "# Updated Plan\n\nNew content"
	retrieved.Tags = append(retrieved.Tags, "updated")
	err = db.UpdateDocument(retrieved)
	if err != nil {
		t.Fatalf("Failed to update document: %v", err)
	}
	if retrieved.Version != 2 {
		t.Errorf("Expected version 2, got %d", retrieved.Version)
	}

	// Verify update
	updated, err := db.GetDocument(doc.ID)
	if err != nil {
		t.Fatalf("Failed to get updated document: %v", err)
	}
	if updated.Content != "# Updated Plan\n\nNew content" {
		t.Error("Document content was not updated")
	}
	if len(updated.Tags) != 3 {
		t.Errorf("Expected 3 tags, got %d", len(updated.Tags))
	}

	// Test GetDocumentsByType
	docs, err := db.GetDocumentsByType("plan", 10)
	if err != nil {
		t.Fatalf("Failed to get documents by type: %v", err)
	}
	if len(docs) != 1 {
		t.Errorf("Expected 1 document, got %d", len(docs))
	}

	// Test GetDocumentsByProject
	docs, err = db.GetDocumentsByProject("CLIAIMONITOR", 10)
	if err != nil {
		t.Fatalf("Failed to get documents by project: %v", err)
	}
	if len(docs) != 1 {
		t.Errorf("Expected 1 document, got %d", len(docs))
	}

	// Test GetDocumentsByAuthor
	docs, err = db.GetDocumentsByAuthor("captain", 10)
	if err != nil {
		t.Fatalf("Failed to get documents by author: %v", err)
	}
	if len(docs) != 1 {
		t.Errorf("Expected 1 document, got %d", len(docs))
	}

	// Test ArchiveDocument
	err = db.ArchiveDocument(doc.ID)
	if err != nil {
		t.Fatalf("Failed to archive document: %v", err)
	}

	archived, err := db.GetDocument(doc.ID)
	if err != nil {
		t.Fatalf("Failed to get archived document: %v", err)
	}
	if archived.Status != "archived" {
		t.Errorf("Expected status 'archived', got %s", archived.Status)
	}
	if archived.ArchivedAt == nil {
		t.Error("ArchivedAt should be set")
	}
}

func TestDocumentWithAssignment(t *testing.T) {
	// Create temp database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_doc_assignment.db")

	db, err := NewMemoryDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create a task assignment first
	assignment := &TaskAssignment{
		TaskID:         "TASK-456",
		AssignedTo:     "sgt-green",
		AssignedBy:     "captain",
		AssignmentType: "implementation",
		Status:         "in_progress",
	}
	err = db.CreateAssignment(assignment)
	if err != nil {
		t.Fatalf("Failed to create assignment: %v", err)
	}

	// Create document linked to assignment
	doc := &Document{
		DocType:      "agent_work",
		Title:        "Implementation Report",
		Content:      "Work completed successfully",
		Format:       "markdown",
		AuthorID:     "sgt-green",
		TaskID:       "TASK-456",
		AssignmentID: &assignment.ID,
		Tags:         []string{"report", "implementation"},
		Status:       "active",
		Version:      1,
	}

	err = db.CreateDocument(doc)
	if err != nil {
		t.Fatalf("Failed to create document with assignment: %v", err)
	}

	// Verify retrieval
	retrieved, err := db.GetDocument(doc.ID)
	if err != nil {
		t.Fatalf("Failed to get document: %v", err)
	}
	if retrieved.AssignmentID == nil {
		t.Error("AssignmentID should be set")
	}
	if *retrieved.AssignmentID != assignment.ID {
		t.Errorf("Expected assignment ID %d, got %d", assignment.ID, *retrieved.AssignmentID)
	}
}

func TestConfigStore(t *testing.T) {
	// Create temp database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_config.db")

	db, err := NewMemoryDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Test SaveConfig (insert)
	err = db.SaveConfig("teams", "team_config_content", "yaml")
	if err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Test GetConfig
	config, err := db.GetConfig("teams")
	if err != nil {
		t.Fatalf("Failed to get config: %v", err)
	}
	if config.ConfigType != "teams" {
		t.Errorf("Expected config type 'teams', got %s", config.ConfigType)
	}
	if config.Content != "team_config_content" {
		t.Error("Config content does not match")
	}
	if config.Version != 1 {
		t.Errorf("Expected version 1, got %d", config.Version)
	}

	// Test SaveConfig (update)
	err = db.SaveConfig("teams", "updated_content", "yaml")
	if err != nil {
		t.Fatalf("Failed to update config: %v", err)
	}

	// Verify update
	updated, err := db.GetConfig("teams")
	if err != nil {
		t.Fatalf("Failed to get updated config: %v", err)
	}
	if updated.Content != "updated_content" {
		t.Error("Config content was not updated")
	}
	if updated.Version != 2 {
		t.Errorf("Expected version 2, got %d", updated.Version)
	}

	// Test GetAllConfigs
	err = db.SaveConfig("projects", "project_config", "yaml")
	if err != nil {
		t.Fatalf("Failed to save second config: %v", err)
	}

	configs, err := db.GetAllConfigs()
	if err != nil {
		t.Fatalf("Failed to get all configs: %v", err)
	}
	if len(configs) != 2 {
		t.Errorf("Expected 2 configs, got %d", len(configs))
	}

	// Test GetConfig for non-existent
	_, err = db.GetConfig("nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent config")
	}
}

func TestDocumentVersioning(t *testing.T) {
	// Create temp database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_versions.db")

	db, err := NewMemoryDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create original document
	doc := &Document{
		DocType:   "plan",
		Title:     "Original Plan",
		Content:   "Original content",
		Format:    "markdown",
		AuthorID:  "captain",
		ProjectID: "TEST",
		Tags:      []string{"plan"},
		Status:    "active",
		Version:   1,
	}

	err = db.CreateDocument(doc)
	if err != nil {
		t.Fatalf("Failed to create document: %v", err)
	}

	// Create superseding document
	newDoc := &Document{
		DocType:   "plan",
		Title:     "Updated Plan",
		Content:   "Updated content",
		Format:    "markdown",
		AuthorID:  "captain",
		ProjectID: "TEST",
		Tags:      []string{"plan", "v2"},
		Status:    "active",
		Version:   1,
		ParentID:  &doc.ID,
	}

	err = db.CreateDocument(newDoc)
	if err != nil {
		t.Fatalf("Failed to create new version: %v", err)
	}

	// Mark old document as superseded
	doc.Status = "superseded"
	err = db.UpdateDocument(doc)
	if err != nil {
		t.Fatalf("Failed to update old document: %v", err)
	}

	// Verify relationship
	oldDoc, err := db.GetDocument(doc.ID)
	if err != nil {
		t.Fatalf("Failed to get old document: %v", err)
	}
	if oldDoc.Status != "superseded" {
		t.Error("Old document should be superseded")
	}

	newDocRetrieved, err := db.GetDocument(newDoc.ID)
	if err != nil {
		t.Fatalf("Failed to get new document: %v", err)
	}
	if newDocRetrieved.ParentID == nil {
		t.Error("New document should have parent ID")
	}
	if *newDocRetrieved.ParentID != doc.ID {
		t.Error("Parent ID should match old document")
	}
}

func TestDocumentSearch(t *testing.T) {
	// Create temp database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_search.db")

	db, err := NewMemoryDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create multiple documents
	docs := []*Document{
		{
			DocType:   "plan",
			Title:     "Database Migration Plan",
			Content:   "Plan to migrate database schema",
			Format:    "markdown",
			AuthorID:  "captain",
			ProjectID: "TEST",
			Tags:      []string{"database", "migration"},
			Status:    "active",
			Version:   1,
		},
		{
			DocType:   "report",
			Title:     "Security Audit Report",
			Content:   "Security vulnerabilities found in authentication",
			Format:    "markdown",
			AuthorID:  "security-agent",
			ProjectID: "TEST",
			Tags:      []string{"security", "audit"},
			Status:    "active",
			Version:   1,
		},
		{
			DocType:   "review",
			Title:     "Code Review: Database Module",
			Content:   "Review of database access patterns",
			Format:    "markdown",
			AuthorID:  "reviewer",
			ProjectID: "TEST",
			Tags:      []string{"review", "database"},
			Status:    "active",
			Version:   1,
		},
	}

	for _, doc := range docs {
		err := db.CreateDocument(doc)
		if err != nil {
			t.Fatalf("Failed to create document: %v", err)
		}
	}

	// Search for "database"
	results, err := db.SearchDocuments("database", 10)
	if err != nil {
		t.Fatalf("Failed to search documents: %v", err)
	}
	// FTS5 should find at least 2 documents containing "database"
	if len(results) < 2 {
		t.Errorf("Expected at least 2 results for 'database', got %d", len(results))
	}

	// Search for "security"
	results, err = db.SearchDocuments("security", 10)
	if err != nil {
		t.Fatalf("Failed to search documents: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Expected 1 result for 'security', got %d", len(results))
	}

	// Search for "authentication"
	results, err = db.SearchDocuments("authentication", 10)
	if err != nil {
		t.Fatalf("Failed to search documents: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Expected 1 result for 'authentication', got %d", len(results))
	}
}
