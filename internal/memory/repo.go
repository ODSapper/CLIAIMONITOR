package memory

import (
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// DiscoverRepo detects a repository and creates/updates its entry
// Returns existing repo if already known, or creates new entry
func (m *SQLiteMemoryDB) DiscoverRepo(basePath string) (*Repo, error) {
	// Normalize path
	absPath, err := filepath.Abs(basePath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve base path: %w", err)
	}

	// Try to get git remote
	gitRemote := getGitRemote(absPath)

	// Generate repo ID
	repoID := generateRepoID(gitRemote, absPath)

	// Check if repo already exists
	existing, err := m.GetRepo(repoID)
	if err == nil {
		// Repo exists - check if path or remote changed
		if existing.BasePath != absPath || existing.GitRemote != gitRemote {
			// Update paths
			_, err := m.db.Exec(`
				UPDATE repos
				SET base_path = ?, git_remote = ?, needs_rescan = 1
				WHERE id = ?`,
				absPath, gitRemote, repoID,
			)
			if err != nil {
				return nil, fmt.Errorf("failed to update repo paths: %w", err)
			}
			existing.BasePath = absPath
			existing.GitRemote = gitRemote
			existing.NeedsRescan = true
		}

		// Check if CLAUDE.md changed
		claudeHash := hashCLAUDEmd(absPath)
		if claudeHash != existing.ClaudeMDHash {
			if err := m.SetRepoRescan(repoID, true); err != nil {
				return nil, fmt.Errorf("failed to mark repo for rescan: %w", err)
			}
			existing.NeedsRescan = true
			existing.ClaudeMDHash = claudeHash
		}

		return existing, nil
	}

	// New repo - create entry
	claudeHash := hashCLAUDEmd(absPath)

	_, err = m.db.Exec(`
		INSERT INTO repos (id, base_path, git_remote, claude_md_hash, needs_rescan)
		VALUES (?, ?, ?, ?, 1)`,
		repoID, absPath, gitRemote, claudeHash,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create repo: %w", err)
	}

	return m.GetRepo(repoID)
}

// GetRepo retrieves a repository by ID
func (m *SQLiteMemoryDB) GetRepo(repoID string) (*Repo, error) {
	var repo Repo
	var lastScanned sql.NullTime
	var needsRescan int

	err := m.db.QueryRow(`
		SELECT id, base_path, git_remote, claude_md_hash, discovered_at, last_scanned, needs_rescan
		FROM repos
		WHERE id = ?`,
		repoID,
	).Scan(
		&repo.ID,
		&repo.BasePath,
		&repo.GitRemote,
		&repo.ClaudeMDHash,
		&repo.DiscoveredAt,
		&lastScanned,
		&needsRescan,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("repo not found: %s", repoID)
		}
		return nil, fmt.Errorf("failed to get repo: %w", err)
	}

	if lastScanned.Valid {
		repo.LastScanned = lastScanned.Time
	}
	repo.NeedsRescan = needsRescan == 1

	return &repo, nil
}

// GetRepoByPath retrieves a repository by base path
func (m *SQLiteMemoryDB) GetRepoByPath(basePath string) (*Repo, error) {
	absPath, err := filepath.Abs(basePath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve path: %w", err)
	}

	var repo Repo
	var lastScanned sql.NullTime
	var needsRescan int

	err = m.db.QueryRow(`
		SELECT id, base_path, git_remote, claude_md_hash, discovered_at, last_scanned, needs_rescan
		FROM repos
		WHERE base_path = ?`,
		absPath,
	).Scan(
		&repo.ID,
		&repo.BasePath,
		&repo.GitRemote,
		&repo.ClaudeMDHash,
		&repo.DiscoveredAt,
		&lastScanned,
		&needsRescan,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("repo not found for path: %s", absPath)
		}
		return nil, fmt.Errorf("failed to get repo by path: %w", err)
	}

	if lastScanned.Valid {
		repo.LastScanned = lastScanned.Time
	}
	repo.NeedsRescan = needsRescan == 1

	return &repo, nil
}

// UpdateRepoScan marks a repository as scanned
func (m *SQLiteMemoryDB) UpdateRepoScan(repoID string) error {
	_, err := m.db.Exec(`
		UPDATE repos
		SET last_scanned = CURRENT_TIMESTAMP, needs_rescan = 0
		WHERE id = ?`,
		repoID,
	)
	if err != nil {
		return fmt.Errorf("failed to update repo scan time: %w", err)
	}
	return nil
}

// SetRepoRescan sets the needs_rescan flag
func (m *SQLiteMemoryDB) SetRepoRescan(repoID string, needsRescan bool) error {
	rescan := 0
	if needsRescan {
		rescan = 1
	}

	_, err := m.db.Exec(`
		UPDATE repos
		SET needs_rescan = ?
		WHERE id = ?`,
		rescan, repoID,
	)
	if err != nil {
		return fmt.Errorf("failed to set repo rescan flag: %w", err)
	}
	return nil
}

// StoreRepoFile stores or updates a discovered file
func (m *SQLiteMemoryDB) StoreRepoFile(file *RepoFile) error {
	_, err := m.db.Exec(`
		INSERT INTO repo_files (repo_id, file_path, file_type, content_hash, content, updated_at)
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(repo_id, file_path) DO UPDATE SET
			file_type = excluded.file_type,
			content_hash = excluded.content_hash,
			content = excluded.content,
			updated_at = CURRENT_TIMESTAMP`,
		file.RepoID, file.FilePath, file.FileType, file.ContentHash, file.Content,
	)
	if err != nil {
		return fmt.Errorf("failed to store repo file: %w", err)
	}
	return nil
}

// GetRepoFiles retrieves all files of a specific type for a repo
// If fileType is empty, returns all files
func (m *SQLiteMemoryDB) GetRepoFiles(repoID string, fileType string) ([]*RepoFile, error) {
	query := `
		SELECT repo_id, file_path, file_type, content_hash, content, discovered_at, updated_at
		FROM repo_files
		WHERE repo_id = ?`
	args := []interface{}{repoID}

	if fileType != "" {
		query += " AND file_type = ?"
		args = append(args, fileType)
	}

	query += " ORDER BY file_path"

	rows, err := m.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query repo files: %w", err)
	}
	defer rows.Close()

	var files []*RepoFile
	for rows.Next() {
		var file RepoFile
		err := rows.Scan(
			&file.RepoID,
			&file.FilePath,
			&file.FileType,
			&file.ContentHash,
			&file.Content,
			&file.DiscoveredAt,
			&file.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan repo file: %w", err)
		}
		files = append(files, &file)
	}

	return files, rows.Err()
}

// GetRepoFile retrieves a specific file
func (m *SQLiteMemoryDB) GetRepoFile(repoID, filePath string) (*RepoFile, error) {
	var file RepoFile
	err := m.db.QueryRow(`
		SELECT repo_id, file_path, file_type, content_hash, content, discovered_at, updated_at
		FROM repo_files
		WHERE repo_id = ? AND file_path = ?`,
		repoID, filePath,
	).Scan(
		&file.RepoID,
		&file.FilePath,
		&file.FileType,
		&file.ContentHash,
		&file.Content,
		&file.DiscoveredAt,
		&file.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("repo file not found: %s", filePath)
		}
		return nil, fmt.Errorf("failed to get repo file: %w", err)
	}

	return &file, nil
}

// Helper functions

// getGitRemote attempts to get the git remote URL for a repository
func getGitRemote(basePath string) string {
	cmd := exec.Command("git", "config", "--get", "remote.origin.url")
	cmd.Dir = basePath
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

// hashCLAUDEmd reads and hashes the CLAUDE.md file if it exists
func hashCLAUDEmd(basePath string) string {
	claudePath := filepath.Join(basePath, "CLAUDE.md")
	data, err := os.ReadFile(claudePath)
	if err != nil {
		return ""
	}
	return hashString(string(data))
}
