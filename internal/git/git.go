// internal/git/git.go
package git

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// Git provides git operations for a repository
type Git struct {
	repoPath string
}

// New creates a Git instance for the given repository path
func New(repoPath string) *Git {
	return &Git{repoPath: repoPath}
}

// BranchName creates a sanitized branch name from task ID and title
func BranchName(taskID, title string) string {
	// Lowercase and replace spaces with hyphens
	slug := strings.ToLower(title)
	slug = strings.ReplaceAll(slug, " ", "-")

	// Remove non-alphanumeric characters except hyphens
	reg := regexp.MustCompile(`[^a-z0-9-]`)
	slug = reg.ReplaceAllString(slug, "")

	// Remove consecutive hyphens
	reg = regexp.MustCompile(`-+`)
	slug = reg.ReplaceAllString(slug, "-")

	// Trim hyphens from ends
	slug = strings.Trim(slug, "-")

	// Truncate to reasonable length (30 chars for slug)
	if len(slug) > 30 {
		slug = slug[:30]
		// Don't end on a hyphen
		slug = strings.TrimRight(slug, "-")
	}

	return fmt.Sprintf("task/%s-%s", taskID, slug)
}

// run executes a git command and returns output
func (g *Git) run(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = g.repoPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, output)
	}
	return strings.TrimSpace(string(output)), nil
}

// CurrentBranch returns the current branch name
func (g *Git) CurrentBranch() (string, error) {
	return g.run("rev-parse", "--abbrev-ref", "HEAD")
}

// CreateBranch creates and checks out a new branch
func (g *Git) CreateBranch(name string) error {
	_, err := g.run("checkout", "-b", name)
	return err
}

// SwitchBranch switches to an existing branch
func (g *Git) SwitchBranch(name string) error {
	_, err := g.run("checkout", name)
	return err
}

// HasUncommittedChanges returns true if there are uncommitted changes
func (g *Git) HasUncommittedChanges() (bool, error) {
	output, err := g.run("status", "--porcelain")
	if err != nil {
		return false, err
	}
	return output != "", nil
}

// Add stages files for commit
func (g *Git) Add(paths ...string) error {
	args := append([]string{"add"}, paths...)
	_, err := g.run(args...)
	return err
}

// Commit creates a commit with the given message
func (g *Git) Commit(message string) error {
	_, err := g.run("commit", "-m", message)
	return err
}

// Push pushes the current branch to origin
func (g *Git) Push() error {
	branch, err := g.CurrentBranch()
	if err != nil {
		return err
	}
	_, err = g.run("push", "-u", "origin", branch)
	return err
}

// GetDiff returns the diff for staged changes
func (g *Git) GetDiff() (string, error) {
	return g.run("diff", "--staged")
}

// GetLog returns recent commit messages
func (g *Git) GetLog(count int) (string, error) {
	return g.run("log", fmt.Sprintf("-%d", count), "--oneline")
}
