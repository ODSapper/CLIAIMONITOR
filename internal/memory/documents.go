package memory

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// CreateDocument inserts a new document and returns its ID
func (m *SQLiteMemoryDB) CreateDocument(doc *Document) error {
	if doc == nil {
		return fmt.Errorf("document cannot be nil")
	}

	// Marshal tags to JSON
	tagsJSON, err := json.Marshal(doc.Tags)
	if err != nil {
		return fmt.Errorf("failed to marshal tags: %w", err)
	}

	result, err := m.db.Exec(`
		INSERT INTO documents (
			doc_type, title, content, format,
			author_id, project_id, task_id, assignment_id,
			tags, status, version, parent_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		doc.DocType, doc.Title, doc.Content, doc.Format,
		nullString(doc.AuthorID), nullString(doc.ProjectID),
		nullString(doc.TaskID), nullInt64(doc.AssignmentID),
		string(tagsJSON), doc.Status, doc.Version, nullInt64(doc.ParentID),
	)
	if err != nil {
		return fmt.Errorf("failed to insert document: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get document ID: %w", err)
	}

	doc.ID = id
	doc.CreatedAt = time.Now()
	doc.UpdatedAt = time.Now()

	return nil
}

// GetDocument retrieves a document by ID
func (m *SQLiteMemoryDB) GetDocument(id int64) (*Document, error) {
	var doc Document
	var tagsJSON string
	var authorID, projectID, taskID sql.NullString
	var assignmentID, parentID sql.NullInt64
	var archivedAt sql.NullTime

	err := m.db.QueryRow(`
		SELECT id, doc_type, title, content, format,
			   author_id, project_id, task_id, assignment_id,
			   tags, status, version, parent_id,
			   created_at, updated_at, archived_at
		FROM documents
		WHERE id = ?`,
		id,
	).Scan(
		&doc.ID, &doc.DocType, &doc.Title, &doc.Content, &doc.Format,
		&authorID, &projectID, &taskID, &assignmentID,
		&tagsJSON, &doc.Status, &doc.Version, &parentID,
		&doc.CreatedAt, &doc.UpdatedAt, &archivedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("document not found: %d", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get document: %w", err)
	}

	// Unmarshal tags
	if err := json.Unmarshal([]byte(tagsJSON), &doc.Tags); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tags: %w", err)
	}

	// Convert nullable fields
	doc.AuthorID = authorID.String
	doc.ProjectID = projectID.String
	doc.TaskID = taskID.String
	if assignmentID.Valid {
		doc.AssignmentID = &assignmentID.Int64
	}
	if parentID.Valid {
		doc.ParentID = &parentID.Int64
	}
	if archivedAt.Valid {
		doc.ArchivedAt = &archivedAt.Time
	}

	return &doc, nil
}

// GetDocumentsByType retrieves documents by type with optional limit
func (m *SQLiteMemoryDB) GetDocumentsByType(docType string, limit int) ([]*Document, error) {
	if limit <= 0 {
		limit = 100 // Default limit
	}

	rows, err := m.db.Query(`
		SELECT id, doc_type, title, content, format,
			   author_id, project_id, task_id, assignment_id,
			   tags, status, version, parent_id,
			   created_at, updated_at, archived_at
		FROM documents
		WHERE doc_type = ?
		ORDER BY created_at DESC
		LIMIT ?`,
		docType, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query documents by type: %w", err)
	}
	defer rows.Close()

	return m.scanDocuments(rows)
}

// GetDocumentsByProject retrieves documents by project ID with optional limit
func (m *SQLiteMemoryDB) GetDocumentsByProject(projectID string, limit int) ([]*Document, error) {
	if limit <= 0 {
		limit = 100 // Default limit
	}

	rows, err := m.db.Query(`
		SELECT id, doc_type, title, content, format,
			   author_id, project_id, task_id, assignment_id,
			   tags, status, version, parent_id,
			   created_at, updated_at, archived_at
		FROM documents
		WHERE project_id = ?
		ORDER BY created_at DESC
		LIMIT ?`,
		projectID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query documents by project: %w", err)
	}
	defer rows.Close()

	return m.scanDocuments(rows)
}

// GetDocumentsByAuthor retrieves documents by author ID with optional limit
func (m *SQLiteMemoryDB) GetDocumentsByAuthor(authorID string, limit int) ([]*Document, error) {
	if limit <= 0 {
		limit = 100 // Default limit
	}

	rows, err := m.db.Query(`
		SELECT id, doc_type, title, content, format,
			   author_id, project_id, task_id, assignment_id,
			   tags, status, version, parent_id,
			   created_at, updated_at, archived_at
		FROM documents
		WHERE author_id = ?
		ORDER BY created_at DESC
		LIMIT ?`,
		authorID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query documents by author: %w", err)
	}
	defer rows.Close()

	return m.scanDocuments(rows)
}

// SearchDocuments performs full-text search on title and content
func (m *SQLiteMemoryDB) SearchDocuments(query string, limit int) ([]*Document, error) {
	if limit <= 0 {
		limit = 100 // Default limit
	}

	rows, err := m.db.Query(`
		SELECT d.id, d.doc_type, d.title, d.content, d.format,
			   d.author_id, d.project_id, d.task_id, d.assignment_id,
			   d.tags, d.status, d.version, d.parent_id,
			   d.created_at, d.updated_at, d.archived_at
		FROM documents d
		INNER JOIN documents_fts fts ON d.id = fts.rowid
		WHERE documents_fts MATCH ?
		ORDER BY rank
		LIMIT ?`,
		query, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to search documents: %w", err)
	}
	defer rows.Close()

	return m.scanDocuments(rows)
}

// UpdateDocument updates an existing document and increments version
func (m *SQLiteMemoryDB) UpdateDocument(doc *Document) error {
	if doc == nil || doc.ID == 0 {
		return fmt.Errorf("document ID is required")
	}

	// Marshal tags to JSON
	tagsJSON, err := json.Marshal(doc.Tags)
	if err != nil {
		return fmt.Errorf("failed to marshal tags: %w", err)
	}

	// Increment version
	doc.Version++

	result, err := m.db.Exec(`
		UPDATE documents
		SET doc_type = ?, title = ?, content = ?, format = ?,
		    author_id = ?, project_id = ?, task_id = ?, assignment_id = ?,
		    tags = ?, status = ?, version = ?, parent_id = ?
		WHERE id = ?`,
		doc.DocType, doc.Title, doc.Content, doc.Format,
		nullString(doc.AuthorID), nullString(doc.ProjectID),
		nullString(doc.TaskID), nullInt64(doc.AssignmentID),
		string(tagsJSON), doc.Status, doc.Version, nullInt64(doc.ParentID),
		doc.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update document: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("document not found: %d", doc.ID)
	}

	doc.UpdatedAt = time.Now()

	return nil
}

// ArchiveDocument sets status to 'archived' and sets archived_at timestamp
func (m *SQLiteMemoryDB) ArchiveDocument(id int64) error {
	result, err := m.db.Exec(`
		UPDATE documents
		SET status = 'archived', archived_at = CURRENT_TIMESTAMP
		WHERE id = ?`,
		id,
	)
	if err != nil {
		return fmt.Errorf("failed to archive document: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("document not found: %d", id)
	}

	return nil
}

// GetConfig retrieves a configuration entry by type
func (m *SQLiteMemoryDB) GetConfig(configType string) (*ConfigEntry, error) {
	var config ConfigEntry

	err := m.db.QueryRow(`
		SELECT id, config_type, content, format, version, created_at, updated_at
		FROM config_store
		WHERE config_type = ?`,
		configType,
	).Scan(
		&config.ID, &config.ConfigType, &config.Content,
		&config.Format, &config.Version, &config.CreatedAt, &config.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("config not found: %s", configType)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get config: %w", err)
	}

	return &config, nil
}

// SaveConfig inserts or updates a configuration entry
func (m *SQLiteMemoryDB) SaveConfig(configType, content, format string) error {
	// Check if config exists
	var existingID int64
	var existingVersion int
	err := m.db.QueryRow(`
		SELECT id, version FROM config_store WHERE config_type = ?`,
		configType,
	).Scan(&existingID, &existingVersion)

	if err == sql.ErrNoRows {
		// Insert new config
		_, err := m.db.Exec(`
			INSERT INTO config_store (config_type, content, format, version)
			VALUES (?, ?, ?, 1)`,
			configType, content, format,
		)
		if err != nil {
			return fmt.Errorf("failed to insert config: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("failed to check existing config: %w", err)
	} else {
		// Update existing config
		_, err := m.db.Exec(`
			UPDATE config_store
			SET content = ?, format = ?, version = version + 1, updated_at = CURRENT_TIMESTAMP
			WHERE id = ?`,
			content, format, existingID,
		)
		if err != nil {
			return fmt.Errorf("failed to update config: %w", err)
		}
	}

	return nil
}

// GetAllConfigs retrieves all configuration entries
func (m *SQLiteMemoryDB) GetAllConfigs() ([]*ConfigEntry, error) {
	rows, err := m.db.Query(`
		SELECT id, config_type, content, format, version, created_at, updated_at
		FROM config_store
		ORDER BY config_type`)
	if err != nil {
		return nil, fmt.Errorf("failed to query configs: %w", err)
	}
	defer rows.Close()

	var configs []*ConfigEntry
	for rows.Next() {
		var config ConfigEntry
		err := rows.Scan(
			&config.ID, &config.ConfigType, &config.Content,
			&config.Format, &config.Version, &config.CreatedAt, &config.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan config: %w", err)
		}
		configs = append(configs, &config)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating configs: %w", err)
	}

	return configs, nil
}

// scanDocuments is a helper function to scan document rows
func (m *SQLiteMemoryDB) scanDocuments(rows *sql.Rows) ([]*Document, error) {
	var documents []*Document

	for rows.Next() {
		var doc Document
		var tagsJSON string
		var authorID, projectID, taskID sql.NullString
		var assignmentID, parentID sql.NullInt64
		var archivedAt sql.NullTime

		err := rows.Scan(
			&doc.ID, &doc.DocType, &doc.Title, &doc.Content, &doc.Format,
			&authorID, &projectID, &taskID, &assignmentID,
			&tagsJSON, &doc.Status, &doc.Version, &parentID,
			&doc.CreatedAt, &doc.UpdatedAt, &archivedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan document: %w", err)
		}

		// Unmarshal tags
		if err := json.Unmarshal([]byte(tagsJSON), &doc.Tags); err != nil {
			return nil, fmt.Errorf("failed to unmarshal tags: %w", err)
		}

		// Convert nullable fields
		doc.AuthorID = authorID.String
		doc.ProjectID = projectID.String
		doc.TaskID = taskID.String
		if assignmentID.Valid {
			doc.AssignmentID = &assignmentID.Int64
		}
		if parentID.Valid {
			doc.ParentID = &parentID.Int64
		}
		if archivedAt.Valid {
			doc.ArchivedAt = &archivedAt.Time
		}

		documents = append(documents, &doc)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating documents: %w", err)
	}

	return documents, nil
}
