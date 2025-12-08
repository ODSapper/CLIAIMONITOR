// internal/git/git_test.go
package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestBranchNameSanitization(t *testing.T) {
	tests := []struct {
		taskID   string
		title    string
		expected string
	}{
		{"TASK-001", "Fix auth bug", "task/TASK-001-fix-auth-bug"},
		{"TASK-002", "Add rate limiting!", "task/TASK-002-add-rate-limiting"},
		{"TASK-003", "This is a very long title that should be truncated", "task/TASK-003-this-is-a-very-long-title-that"},
	}

	for _, tt := range tests {
		result := BranchName(tt.taskID, tt.title)
		if result != tt.expected {
			t.Errorf("BranchName(%q, %q) = %q, want %q", tt.taskID, tt.title, result, tt.expected)
		}
	}
}

func TestGitOperationsInTempRepo(t *testing.T) {
	// Skip if git not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "git-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	// Configure git
	exec.Command("git", "-C", tmpDir, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", tmpDir, "config", "user.name", "Test").Run()

	// Create initial commit
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("initial"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "initial").Run()

	// Test Git operations
	g := New(tmpDir)

	// Test CreateBranch
	branch := "task/TASK-001-test"
	if err := g.CreateBranch(branch); err != nil {
		t.Errorf("CreateBranch failed: %v", err)
	}

	// Verify we're on the new branch
	current, err := g.CurrentBranch()
	if err != nil {
		t.Errorf("CurrentBranch failed: %v", err)
	}
	if current != branch {
		t.Errorf("expected branch %q, got %q", branch, current)
	}
}
