package memory

import (
	"database/sql"
	"fmt"
	"os"
	"time"
)

// GetPromptTemplate retrieves a prompt template by name
func (m *SQLiteMemoryDB) GetPromptTemplate(name string) (*PromptTemplate, error) {
	query := `
		SELECT id, name, role, content, description, version, is_active, created_at, updated_at
		FROM prompt_templates
		WHERE name = ? AND is_active = 1
	`

	var template PromptTemplate
	var description sql.NullString

	err := m.db.QueryRow(query, name).Scan(
		&template.ID,
		&template.Name,
		&template.Role,
		&template.Content,
		&description,
		&template.Version,
		&template.IsActive,
		&template.CreatedAt,
		&template.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get prompt template: %w", err)
	}

	if description.Valid {
		template.Description = description.String
	}

	return &template, nil
}

// GetPromptTemplateByRole retrieves a prompt template by role (fallback if name not found)
func (m *SQLiteMemoryDB) GetPromptTemplateByRole(role string) (*PromptTemplate, error) {
	query := `
		SELECT id, name, role, content, description, version, is_active, created_at, updated_at
		FROM prompt_templates
		WHERE role = ? AND is_active = 1
		ORDER BY version DESC
		LIMIT 1
	`

	var template PromptTemplate
	var description sql.NullString

	err := m.db.QueryRow(query, role).Scan(
		&template.ID,
		&template.Name,
		&template.Role,
		&template.Content,
		&description,
		&template.Version,
		&template.IsActive,
		&template.CreatedAt,
		&template.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get prompt template by role: %w", err)
	}

	if description.Valid {
		template.Description = description.String
	}

	return &template, nil
}

// GetAllPromptTemplates retrieves all active prompt templates
func (m *SQLiteMemoryDB) GetAllPromptTemplates() ([]*PromptTemplate, error) {
	query := `
		SELECT id, name, role, content, description, version, is_active, created_at, updated_at
		FROM prompt_templates
		WHERE is_active = 1
		ORDER BY name
	`

	rows, err := m.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query prompt templates: %w", err)
	}
	defer rows.Close()

	var templates []*PromptTemplate
	for rows.Next() {
		var template PromptTemplate
		var description sql.NullString

		err := rows.Scan(
			&template.ID,
			&template.Name,
			&template.Role,
			&template.Content,
			&description,
			&template.Version,
			&template.IsActive,
			&template.CreatedAt,
			&template.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan prompt template: %w", err)
		}

		if description.Valid {
			template.Description = description.String
		}

		templates = append(templates, &template)
	}

	return templates, nil
}

// SavePromptTemplate saves or updates a prompt template
func (m *SQLiteMemoryDB) SavePromptTemplate(template *PromptTemplate) error {
	// Check if template exists
	existing, err := m.GetPromptTemplate(template.Name)
	if err != nil {
		return err
	}

	if existing != nil {
		// Update existing template, increment version
		query := `
			UPDATE prompt_templates
			SET role = ?, content = ?, description = ?, version = version + 1, is_active = ?
			WHERE name = ?
		`
		_, err := m.db.Exec(query,
			template.Role,
			template.Content,
			nullString(template.Description),
			template.IsActive,
			template.Name,
		)
		if err != nil {
			return fmt.Errorf("failed to update prompt template: %w", err)
		}
	} else {
		// Insert new template
		query := `
			INSERT INTO prompt_templates (name, role, content, description, version, is_active, created_at, updated_at)
			VALUES (?, ?, ?, ?, 1, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		`
		result, err := m.db.Exec(query,
			template.Name,
			template.Role,
			template.Content,
			nullString(template.Description),
			template.IsActive,
		)
		if err != nil {
			return fmt.Errorf("failed to insert prompt template: %w", err)
		}

		id, err := result.LastInsertId()
		if err == nil {
			template.ID = id
		}
	}

	return nil
}

// DeletePromptTemplate soft-deletes a prompt template by marking it inactive
func (m *SQLiteMemoryDB) DeletePromptTemplate(name string) error {
	query := `UPDATE prompt_templates SET is_active = 0 WHERE name = ?`
	_, err := m.db.Exec(query, name)
	if err != nil {
		return fmt.Errorf("failed to delete prompt template: %w", err)
	}
	return nil
}

// SeedDefaultPrompts loads default prompts from configs/prompts if DB is empty
// This is called during initialization to migrate from files to DB
func (m *SQLiteMemoryDB) SeedDefaultPrompts(promptsDir string) error {
	// Always sync from files - this ensures file updates are reflected in DB
	return m.SyncPromptTemplates(promptsDir)
}

// SyncPromptTemplates syncs prompt templates from files to DB
// Updates existing templates if file content differs, creates new ones if missing
func (m *SQLiteMemoryDB) SyncPromptTemplates(promptsDir string) error {
	// Default prompts that should exist
	defaults := []struct {
		name        string
		role        string
		description string
	}{
		{"engineer", "engineer", "General-purpose engineering agent"},
		{"go-developer", "go_developer", "Go/Golang specialist"},
		{"security", "security", "Security analysis and hardening"},
		{"code-auditor", "code_auditor", "Code review and quality auditing"},
		{"snake", "snake", "Reconnaissance and intelligence gathering"},
		{"sgt-green", "sgt_green", "Implementation Sergeant - orchestrates implementation work"},
		{"sgt-purple", "sgt_purple", "Review Sergeant - orchestrates Fagan-style code reviews"},
	}

	synced := 0
	for _, def := range defaults {
		// Read from file if exists
		filePath := promptsDir + "/" + def.name + ".md"
		content, err := readFileContent(filePath)
		if err != nil {
			// Skip if file doesn't exist - don't create placeholders
			continue
		}

		// Check existing template
		existing, err := m.GetPromptTemplate(def.name)
		if err != nil {
			return fmt.Errorf("failed to check template %s: %w", def.name, err)
		}

		// Skip if content is identical
		if existing != nil && existing.Content == content {
			continue
		}

		template := &PromptTemplate{
			Name:        def.name,
			Role:        def.role,
			Content:     content,
			Description: def.description,
			IsActive:    true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}

		if err := m.SavePromptTemplate(template); err != nil {
			return fmt.Errorf("failed to sync %s: %w", def.name, err)
		}

		if existing != nil {
			fmt.Printf("[PROMPTS] Updated %s from file (v%d -> v%d)\n", def.name, existing.Version, existing.Version+1)
		} else {
			fmt.Printf("[PROMPTS] Created %s from file\n", def.name)
		}
		synced++
	}

	if synced > 0 {
		fmt.Printf("[PROMPTS] Synced %d prompt templates from files\n", synced)
	}
	return nil
}

// readFileContent reads file content as string
func readFileContent(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
