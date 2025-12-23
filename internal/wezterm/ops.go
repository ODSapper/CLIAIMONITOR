// Package wezterm provides centralized WezTerm CLI operations with rate limiting
// to prevent lockups when multiple pane operations occur in quick succession.
package wezterm

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// PaneInfo represents WezTerm pane information
type PaneInfo struct {
	PaneID    int    `json:"pane_id"`
	WindowID  int    `json:"window_id"`
	TabID     int    `json:"tab_id"`
	Title     string `json:"title"`
	CWD       string `json:"cwd"`
	IsActive  bool   `json:"is_active"`
	TopRow    int    `json:"top_row"`
	LeftCol   int    `json:"left_col"`
}

// Ops provides thread-safe WezTerm CLI operations with rate limiting
type Ops struct {
	mu              sync.Mutex
	lastPaneOp      time.Time
	minOpInterval   time.Duration
	commandTimeout  time.Duration
}

// Global singleton instance
var (
	instance     *Ops
	instanceOnce sync.Once
)

// Get returns the singleton Ops instance
func Get() *Ops {
	instanceOnce.Do(func() {
		instance = &Ops{
			minOpInterval:  500 * time.Millisecond, // 500ms between pane operations (increased from 200ms to prevent WezTerm freeze)
			commandTimeout: 10 * time.Second,       // 10s timeout per command
		}
	})
	return instance
}

// waitForInterval ensures minimum interval between pane operations
func (o *Ops) waitForInterval() {
	elapsed := time.Since(o.lastPaneOp)
	if elapsed < o.minOpInterval {
		time.Sleep(o.minOpInterval - elapsed)
	}
	o.lastPaneOp = time.Now()
}

// runCommand executes a WezTerm CLI command with timeout
func (o *Ops) runCommand(ctx context.Context, args ...string) ([]byte, error) {
	// Create command with timeout context
	ctx, cancel := context.WithTimeout(ctx, o.commandTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "wezterm.exe", args...)
	output, err := cmd.CombinedOutput()

	if ctx.Err() == context.DeadlineExceeded {
		return nil, fmt.Errorf("command timed out after %v", o.commandTimeout)
	}

	return output, err
}

// KillPane closes a WezTerm pane by ID with proper synchronization
func (o *Ops) KillPane(paneID int) error {
	return o.KillPaneContext(context.Background(), paneID)
}

// KillPaneContext closes a WezTerm pane with context support
func (o *Ops) KillPaneContext(ctx context.Context, paneID int) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.waitForInterval()

	log.Printf("[WEZTERM] Closing pane %d", paneID)

	output, err := o.runCommand(ctx, "cli", "kill-pane", "--pane-id", strconv.Itoa(paneID))
	if err != nil {
		return fmt.Errorf("failed to close pane %d: %w (output: %s)", paneID, err, string(output))
	}

	log.Printf("[WEZTERM] Successfully closed pane %d", paneID)
	return nil
}

// KillPanes closes multiple panes sequentially with proper delays
func (o *Ops) KillPanes(paneIDs []int) []error {
	return o.KillPanesContext(context.Background(), paneIDs)
}

// KillPanesContext closes multiple panes with context support
func (o *Ops) KillPanesContext(ctx context.Context, paneIDs []int) []error {
	var errors []error

	for _, paneID := range paneIDs {
		select {
		case <-ctx.Done():
			errors = append(errors, fmt.Errorf("context cancelled while closing pane %d", paneID))
			return errors
		default:
		}

		if err := o.KillPaneContext(ctx, paneID); err != nil {
			log.Printf("[WEZTERM] Warning: Failed to close pane %d: %v", paneID, err)
			errors = append(errors, err)
		}
	}

	return errors
}

// GracefulKillPane sends an exit signal to the pane before killing it.
// This helps prevent Windows conpty hangs by allowing the process to clean up.
func (o *Ops) GracefulKillPane(paneID int) error {
	return o.GracefulKillPaneContext(context.Background(), paneID)
}

// GracefulKillPaneContext sends exit signal then kills the pane with context support.
// Sequence: Send Ctrl+C -> wait 300ms -> Send "exit" -> wait 500ms -> kill pane
// This gives Windows conpty time to clean up the pseudo-console properly.
func (o *Ops) GracefulKillPaneContext(ctx context.Context, paneID int) error {
	log.Printf("[WEZTERM] Gracefully closing pane %d (sending exit signals first)", paneID)

	// Step 1: Send Ctrl+C to interrupt any running process
	// We don't hold the mutex here since SendTextContext handles its own locking
	ctrlC := "\x03" // ASCII ETX (Ctrl+C)
	if err := o.SendTextContext(ctx, paneID, ctrlC, false); err != nil {
		log.Printf("[WEZTERM] Warning: Failed to send Ctrl+C to pane %d: %v", paneID, err)
		// Continue anyway - process might already be idle
	}

	// Wait for interrupt to be processed
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(300 * time.Millisecond):
	}

	// Step 2: Send "exit" command to terminate shell gracefully
	if err := o.SendTextContext(ctx, paneID, "exit", true); err != nil {
		log.Printf("[WEZTERM] Warning: Failed to send exit to pane %d: %v", paneID, err)
		// Continue anyway - we'll force kill
	}

	// Wait for shell to exit and conpty to clean up
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(500 * time.Millisecond):
	}

	// Step 3: Now kill the pane (process should be gone or exiting)
	return o.KillPaneContext(ctx, paneID)
}

// GracefulKillPanes closes multiple panes with graceful shutdown for each.
// This is the recommended method for closing agent panes to prevent WezTerm freezes.
func (o *Ops) GracefulKillPanes(paneIDs []int) []error {
	return o.GracefulKillPanesContext(context.Background(), paneIDs)
}

// GracefulKillPanesContext closes multiple panes gracefully with context support.
func (o *Ops) GracefulKillPanesContext(ctx context.Context, paneIDs []int) []error {
	var errors []error

	for i, paneID := range paneIDs {
		select {
		case <-ctx.Done():
			errors = append(errors, fmt.Errorf("context cancelled while closing pane %d", paneID))
			return errors
		default:
		}

		log.Printf("[WEZTERM] Gracefully closing pane %d/%d", i+1, len(paneIDs))

		if err := o.GracefulKillPaneContext(ctx, paneID); err != nil {
			log.Printf("[WEZTERM] Warning: Failed to gracefully close pane %d: %v", paneID, err)
			errors = append(errors, err)
		}

		// Additional delay between panes to let WezTerm's internal state settle
		if i < len(paneIDs)-1 {
			select {
			case <-ctx.Done():
				return errors
			case <-time.After(300 * time.Millisecond):
			}
		}
	}

	return errors
}

// ListPanes returns all WezTerm panes
func (o *Ops) ListPanes() ([]PaneInfo, error) {
	return o.ListPanesContext(context.Background())
}

// ListPanesContext returns all WezTerm panes with context support
func (o *Ops) ListPanesContext(ctx context.Context) ([]PaneInfo, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	output, err := o.runCommand(ctx, "cli", "list", "--format", "json")
	if err != nil {
		return nil, fmt.Errorf("failed to list panes: %w", err)
	}

	var panes []PaneInfo
	if err := json.Unmarshal(output, &panes); err != nil {
		return nil, fmt.Errorf("failed to parse pane list: %w", err)
	}

	return panes, nil
}

// GetPaneText reads text from a pane
func (o *Ops) GetPaneText(paneID int, startLine, endLine int) (string, error) {
	return o.GetPaneTextContext(context.Background(), paneID, startLine, endLine)
}

// GetPaneTextContext reads text from a pane with context support
func (o *Ops) GetPaneTextContext(ctx context.Context, paneID int, startLine, endLine int) (string, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	args := []string{"cli", "get-text", "--pane-id", strconv.Itoa(paneID)}
	if startLine != 0 {
		args = append(args, "--start-line", strconv.Itoa(startLine))
	}
	if endLine != 0 {
		args = append(args, "--end-line", strconv.Itoa(endLine))
	}

	output, err := o.runCommand(ctx, args...)
	if err != nil {
		return "", fmt.Errorf("failed to get pane text: %w", err)
	}

	return string(output), nil
}

// SendText sends text to a pane
func (o *Ops) SendText(paneID int, text string, execute bool) error {
	return o.SendTextContext(context.Background(), paneID, text, execute)
}

// SendTextContext sends text to a pane with context support
func (o *Ops) SendTextContext(ctx context.Context, paneID int, text string, execute bool) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if execute {
		text = text + "\r\n"
	}

	ctx, cancel := context.WithTimeout(ctx, o.commandTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "wezterm.exe", "cli", "send-text", "--pane-id", strconv.Itoa(paneID), "--no-paste")
	cmd.Stdin = strings.NewReader(text)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to send text: %w (output: %s)", err, string(output))
	}

	return nil
}

// FocusPane activates a specific pane
func (o *Ops) FocusPane(paneID int) error {
	return o.FocusPaneContext(context.Background(), paneID)
}

// FocusPaneContext activates a specific pane with context support
func (o *Ops) FocusPaneContext(ctx context.Context, paneID int) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	output, err := o.runCommand(ctx, "cli", "activate-pane", "--pane-id", strconv.Itoa(paneID))
	if err != nil {
		return fmt.Errorf("failed to focus pane: %w (output: %s)", err, string(output))
	}

	return nil
}

// SpawnPane splits an existing pane to create a new one
func (o *Ops) SpawnPane(direction string, fromPaneID int, cwd string) (int, error) {
	return o.SpawnPaneContext(context.Background(), direction, fromPaneID, cwd)
}

// SpawnPaneContext splits an existing pane with context support
func (o *Ops) SpawnPaneContext(ctx context.Context, direction string, fromPaneID int, cwd string) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.waitForInterval()

	args := []string{"cli", "split-pane", "--" + direction}
	if fromPaneID > 0 {
		args = append(args, "--pane-id", strconv.Itoa(fromPaneID))
	}
	if cwd != "" {
		args = append(args, "--cwd", cwd)
	}
	args = append(args, "--", "cmd.exe")

	output, err := o.runCommand(ctx, args...)
	if err != nil {
		return 0, fmt.Errorf("failed to spawn pane: %w (output: %s)", err, string(output))
	}

	paneIDStr := strings.TrimSpace(string(output))
	paneID, err := strconv.Atoi(paneIDStr)
	if err != nil {
		return 0, fmt.Errorf("failed to parse pane ID from output: %s", paneIDStr)
	}

	return paneID, nil
}

// SpawnWindow creates a new WezTerm window
func (o *Ops) SpawnWindow(cwd string) (int, error) {
	return o.SpawnWindowContext(context.Background(), cwd)
}

// SpawnWindowContext creates a new WezTerm window with context support
func (o *Ops) SpawnWindowContext(ctx context.Context, cwd string) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.waitForInterval()

	args := []string{"cli", "spawn", "--new-window"}
	if cwd != "" {
		args = append(args, "--cwd", cwd)
	}
	args = append(args, "--", "cmd.exe")

	output, err := o.runCommand(ctx, args...)
	if err != nil {
		return 0, fmt.Errorf("failed to spawn window: %w (output: %s)", err, string(output))
	}

	paneIDStr := strings.TrimSpace(string(output))
	paneID, err := strconv.Atoi(paneIDStr)
	if err != nil {
		return 0, fmt.Errorf("failed to parse pane ID from output: %s", paneIDStr)
	}

	return paneID, nil
}
