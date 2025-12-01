package agents

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/CLIAIMONITOR/internal/types"
)

func TestLoadProjectsConfig(t *testing.T) {
	// Create temp config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "projects.yaml")

	content := `
scan_path: "/test/path"
projects:
  - name: TestProject
    path: "/test/path/project"
    description: "Test project"
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	config, err := LoadProjectsConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if config.ScanPath != "/test/path" {
		t.Errorf("Expected scan_path /test/path, got %s", config.ScanPath)
	}

	if len(config.Projects) != 1 {
		t.Fatalf("Expected 1 project, got %d", len(config.Projects))
	}

	if config.Projects[0].Name != "TestProject" {
		t.Errorf("Expected project name TestProject, got %s", config.Projects[0].Name)
	}
}

func TestLoadProjectsConfigNonExistent(t *testing.T) {
	_, err := LoadProjectsConfig("/nonexistent/path/projects.yaml")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

func TestLoadProjectsConfigInvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "projects.yaml")

	if err := os.WriteFile(configPath, []byte("invalid: yaml: content:"), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	_, err := LoadProjectsConfig(configPath)
	if err == nil {
		t.Error("Expected error for invalid YAML")
	}
}

func TestDiscoverProjects(t *testing.T) {
	tmpDir := t.TempDir()

	// Create project with CLAUDE.md
	projectWithClaudeMD := filepath.Join(tmpDir, "project-with-claude")
	if err := os.MkdirAll(projectWithClaudeMD, 0755); err != nil {
		t.Fatalf("Failed to create dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectWithClaudeMD, "CLAUDE.md"), []byte("# Test"), 0644); err != nil {
		t.Fatalf("Failed to write CLAUDE.md: %v", err)
	}

	// Create project without CLAUDE.md
	projectWithoutClaudeMD := filepath.Join(tmpDir, "project-without-claude")
	if err := os.MkdirAll(projectWithoutClaudeMD, 0755); err != nil {
		t.Fatalf("Failed to create dir: %v", err)
	}

	discovered, err := DiscoverProjects(tmpDir)
	if err != nil {
		t.Fatalf("Failed to discover projects: %v", err)
	}

	if len(discovered) != 1 {
		t.Fatalf("Expected 1 discovered project, got %d", len(discovered))
	}

	if discovered[0].Name != "project-with-claude" {
		t.Errorf("Expected discovered project name project-with-claude, got %s", discovered[0].Name)
	}

	if !discovered[0].HasClaudeMD {
		t.Error("Expected HasClaudeMD to be true")
	}
}

func TestGetAllProjects(t *testing.T) {
	tmpDir := t.TempDir()

	// Create explicit project dir
	explicitDir := filepath.Join(tmpDir, "explicit-project")
	if err := os.MkdirAll(explicitDir, 0755); err != nil {
		t.Fatalf("Failed to create dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(explicitDir, "CLAUDE.md"), []byte("# Test"), 0644); err != nil {
		t.Fatalf("Failed to write CLAUDE.md: %v", err)
	}

	// Create discoverable project dir
	discoverDir := filepath.Join(tmpDir, "discover-project")
	if err := os.MkdirAll(discoverDir, 0755); err != nil {
		t.Fatalf("Failed to create dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(discoverDir, "CLAUDE.md"), []byte("# Discovered"), 0644); err != nil {
		t.Fatalf("Failed to write CLAUDE.md: %v", err)
	}

	config := &types.ProjectsConfig{
		ScanPath: tmpDir,
		Projects: []types.ProjectConfig{
			{
				Name:        "ExplicitProject",
				Path:        explicitDir,
				Description: "Explicitly defined",
			},
		},
	}

	projects, err := GetAllProjects(config)
	if err != nil {
		t.Fatalf("Failed to get all projects: %v", err)
	}

	// Should have explicit + discovered (minus duplicates)
	if len(projects) != 2 {
		t.Fatalf("Expected 2 projects, got %d", len(projects))
	}

	// First should be explicit
	if projects[0].Name != "ExplicitProject" {
		t.Errorf("Expected first project to be ExplicitProject, got %s", projects[0].Name)
	}
	if !projects[0].HasClaudeMD {
		t.Error("Expected explicit project to have HasClaudeMD true")
	}
}

func TestGetProjectByName(t *testing.T) {
	projects := []types.ProjectConfig{
		{Name: "Project1", Path: "/path1"},
		{Name: "Project2", Path: "/path2"},
	}

	found := GetProjectByName(projects, "Project2")
	if found == nil {
		t.Fatal("Expected to find Project2")
	}
	if found.Path != "/path2" {
		t.Errorf("Expected path /path2, got %s", found.Path)
	}

	notFound := GetProjectByName(projects, "NonExistent")
	if notFound != nil {
		t.Error("Expected nil for non-existent project")
	}
}

func TestGetProjectByPath(t *testing.T) {
	projects := []types.ProjectConfig{
		{Name: "Project1", Path: "/path1"},
		{Name: "Project2", Path: "/path2"},
	}

	found := GetProjectByPath(projects, "/path1")
	if found == nil {
		t.Fatal("Expected to find project by path")
	}
	if found.Name != "Project1" {
		t.Errorf("Expected name Project1, got %s", found.Name)
	}
}

func TestValidateProjectPath(t *testing.T) {
	tmpDir := t.TempDir()

	// Create valid project with .git
	validGit := filepath.Join(tmpDir, "valid-git")
	if err := os.MkdirAll(filepath.Join(validGit, ".git"), 0755); err != nil {
		t.Fatalf("Failed to create .git dir: %v", err)
	}

	// Create valid project with CLAUDE.md
	validClaudeMD := filepath.Join(tmpDir, "valid-claude")
	if err := os.MkdirAll(validClaudeMD, 0755); err != nil {
		t.Fatalf("Failed to create dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(validClaudeMD, "CLAUDE.md"), []byte("# Test"), 0644); err != nil {
		t.Fatalf("Failed to write CLAUDE.md: %v", err)
	}

	// Test valid git project
	if err := ValidateProjectPath(validGit, tmpDir); err != nil {
		t.Errorf("Expected valid git project to pass: %v", err)
	}

	// Test valid CLAUDE.md project
	if err := ValidateProjectPath(validClaudeMD, tmpDir); err != nil {
		t.Errorf("Expected valid CLAUDE.md project to pass: %v", err)
	}

	// Test relative path
	if err := ValidateProjectPath("relative/path", ""); err == nil {
		t.Error("Expected error for relative path")
	}

	// Test non-existent path
	if err := ValidateProjectPath(filepath.Join(tmpDir, "nonexistent"), ""); err == nil {
		t.Error("Expected error for non-existent path")
	}

	// Test path outside scan_path
	if err := ValidateProjectPath("/some/other/path", tmpDir); err == nil {
		t.Error("Expected error for path outside scan_path")
	}
}

func TestReadClaudeMD(t *testing.T) {
	tmpDir := t.TempDir()

	// Create CLAUDE.md
	content := "# Test Project\n\nThis is a test."
	if err := os.WriteFile(filepath.Join(tmpDir, "CLAUDE.md"), []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write CLAUDE.md: %v", err)
	}

	result, err := ReadClaudeMD(tmpDir)
	if err != nil {
		t.Fatalf("Failed to read CLAUDE.md: %v", err)
	}

	if result != content {
		t.Errorf("Expected content %q, got %q", content, result)
	}

	// Test non-existent
	_, err = ReadClaudeMD("/nonexistent/path")
	if err == nil {
		t.Error("Expected error for non-existent CLAUDE.md")
	}
}

func TestProjectValidationError(t *testing.T) {
	err := &ProjectValidationError{
		Path:   "/test/path",
		Reason: "test reason",
	}

	expected := "invalid project path /test/path: test reason"
	if err.Error() != expected {
		t.Errorf("Expected error %q, got %q", expected, err.Error())
	}
}
