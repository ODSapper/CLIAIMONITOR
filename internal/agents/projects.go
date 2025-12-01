package agents

import (
	"os"
	"path/filepath"

	"github.com/CLIAIMONITOR/internal/types"
	"gopkg.in/yaml.v3"
)

// LoadProjectsConfig loads project configuration from YAML
func LoadProjectsConfig(configPath string) (*types.ProjectsConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config types.ProjectsConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// DiscoverProjects scans a directory for projects with CLAUDE.md
func DiscoverProjects(scanPath string) ([]types.ProjectConfig, error) {
	var discovered []types.ProjectConfig

	entries, err := os.ReadDir(scanPath)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		projectPath := filepath.Join(scanPath, entry.Name())
		claudeMDPath := filepath.Join(projectPath, "CLAUDE.md")

		// Check if CLAUDE.md exists
		if _, err := os.Stat(claudeMDPath); err == nil {
			discovered = append(discovered, types.ProjectConfig{
				Name:        entry.Name(),
				Path:        projectPath,
				Description: "Auto-discovered project",
				HasClaudeMD: true,
			})
		}
	}

	return discovered, nil
}

// GetAllProjects returns merged list of explicit and discovered projects
// Explicit projects take precedence over auto-discovered ones
func GetAllProjects(config *types.ProjectsConfig) ([]types.ProjectConfig, error) {
	// Start with explicit projects and mark HasClaudeMD
	projects := make([]types.ProjectConfig, 0, len(config.Projects))
	explicitPaths := make(map[string]bool)

	for _, p := range config.Projects {
		proj := p
		// Check if CLAUDE.md exists for explicit projects
		claudeMDPath := filepath.Join(proj.Path, "CLAUDE.md")
		if _, err := os.Stat(claudeMDPath); err == nil {
			proj.HasClaudeMD = true
		}
		projects = append(projects, proj)
		explicitPaths[proj.Path] = true
	}

	// Add discovered projects that aren't explicitly defined
	if config.ScanPath != "" {
		discovered, err := DiscoverProjects(config.ScanPath)
		if err != nil {
			// Log error but continue with explicit projects
			return projects, nil
		}

		for _, d := range discovered {
			if !explicitPaths[d.Path] {
				projects = append(projects, d)
			}
		}
	}

	return projects, nil
}

// GetProjectByName finds a project by name
func GetProjectByName(projects []types.ProjectConfig, name string) *types.ProjectConfig {
	for i := range projects {
		if projects[i].Name == name {
			return &projects[i]
		}
	}
	return nil
}

// GetProjectByPath finds a project by path
func GetProjectByPath(projects []types.ProjectConfig, path string) *types.ProjectConfig {
	for i := range projects {
		if projects[i].Path == path {
			return &projects[i]
		}
	}
	return nil
}

// ValidateProjectPath checks if a path is a valid project directory
func ValidateProjectPath(path string, scanPath string) error {
	// Must be absolute
	if !filepath.IsAbs(path) {
		return &ProjectValidationError{Path: path, Reason: "path must be absolute"}
	}

	// Must exist
	info, err := os.Stat(path)
	if err != nil {
		return &ProjectValidationError{Path: path, Reason: "path does not exist"}
	}

	// Must be a directory
	if !info.IsDir() {
		return &ProjectValidationError{Path: path, Reason: "path is not a directory"}
	}

	// Must be under scan_path (if provided)
	if scanPath != "" {
		relPath, err := filepath.Rel(scanPath, path)
		if err != nil || filepath.HasPrefix(relPath, "..") {
			return &ProjectValidationError{Path: path, Reason: "path is not within allowed directory"}
		}
	}

	// Must have .git or CLAUDE.md
	gitPath := filepath.Join(path, ".git")
	claudeMDPath := filepath.Join(path, "CLAUDE.md")
	if _, err := os.Stat(gitPath); err != nil {
		if _, err := os.Stat(claudeMDPath); err != nil {
			return &ProjectValidationError{Path: path, Reason: "path is not a valid project (no .git or CLAUDE.md)"}
		}
	}

	return nil
}

// ReadClaudeMD reads the CLAUDE.md file from a project
func ReadClaudeMD(projectPath string) (string, error) {
	claudeMDPath := filepath.Join(projectPath, "CLAUDE.md")
	data, err := os.ReadFile(claudeMDPath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ProjectValidationError represents a project path validation error
type ProjectValidationError struct {
	Path   string
	Reason string
}

func (e *ProjectValidationError) Error() string {
	return "invalid project path " + e.Path + ": " + e.Reason
}
