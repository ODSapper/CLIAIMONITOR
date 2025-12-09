// internal/tasks/sources.go
package tasks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// TaskSourceInterface defines operations for task sources
type TaskSourceInterface interface {
	// FetchPendingTasks retrieves tasks that are available to claim
	FetchPendingTasks() ([]*Task, error)

	// ClaimTask marks a task as claimed by an agent
	ClaimTask(taskID string, agentID string) error

	// CompleteTask marks a task as complete with result details
	CompleteTask(taskID string, result TaskResult) error

	// GetName returns a human-readable name for this source
	GetName() string
}

// TaskResult contains details about task completion
type TaskResult struct {
	Branch      string `json:"branch,omitempty"`
	PRUrl       string `json:"pr_url,omitempty"`
	TokensUsed  int64  `json:"tokens_used,omitempty"`
	Success     bool   `json:"success"`
	ErrorMsg    string `json:"error_msg,omitempty"`
	CompletedBy string `json:"completed_by,omitempty"`
}

// LocalTaskSource implements TaskSourceInterface using the local task queue
type LocalTaskSource struct {
	queue *Queue
	store *Store
}

// NewLocalTaskSource creates a task source backed by the local queue
func NewLocalTaskSource(queue *Queue, store *Store) TaskSourceInterface {
	return &LocalTaskSource{
		queue: queue,
		store: store,
	}
}

// FetchPendingTasks returns all tasks with status=pending from local queue
func (l *LocalTaskSource) FetchPendingTasks() ([]*Task, error) {
	tasks := l.queue.GetByStatus(StatusPending)
	return tasks, nil
}

// ClaimTask assigns the task to an agent
func (l *LocalTaskSource) ClaimTask(taskID string, agentID string) error {
	task := l.queue.GetByID(taskID)
	if task == nil {
		return fmt.Errorf("task %s not found", taskID)
	}

	task.AssignedTo = agentID
	now := time.Now()
	task.StartedAt = &now
	task.UpdatedAt = now

	if err := task.TransitionTo(StatusAssigned); err != nil {
		return fmt.Errorf("failed to transition task to assigned: %w", err)
	}

	l.queue.Update(task)

	if l.store != nil {
		if err := l.store.Save(task); err != nil {
			// Log but don't fail - queue is source of truth
			fmt.Printf("[LocalTaskSource] Warning: failed to persist task %s: %v\n", taskID, err)
		}
	}

	return nil
}

// CompleteTask marks the task as merged and saves completion details
func (l *LocalTaskSource) CompleteTask(taskID string, result TaskResult) error {
	task := l.queue.GetByID(taskID)
	if task == nil {
		return fmt.Errorf("task %s not found", taskID)
	}

	task.Branch = result.Branch
	task.PRUrl = result.PRUrl
	now := time.Now()
	task.CompletedAt = &now
	task.UpdatedAt = now

	if result.TokensUsed > 0 {
		if task.Metadata == nil {
			task.Metadata = make(map[string]string)
		}
		task.Metadata["tokens_used"] = fmt.Sprintf("%d", result.TokensUsed)
	}

	if result.CompletedBy != "" {
		if task.Metadata == nil {
			task.Metadata = make(map[string]string)
		}
		task.Metadata["completed_by"] = result.CompletedBy
	}

	if !result.Success {
		if task.Metadata == nil {
			task.Metadata = make(map[string]string)
		}
		task.Metadata["error"] = result.ErrorMsg
		if err := task.TransitionTo(StatusBlocked); err != nil {
			return fmt.Errorf("failed to transition task to blocked: %w", err)
		}
	} else {
		// Success path: review -> approved -> merged
		if task.Status != StatusApproved {
			if err := task.TransitionTo(StatusReview); err != nil {
				return fmt.Errorf("failed to transition task to review: %w", err)
			}
			if err := task.TransitionTo(StatusApproved); err != nil {
				return fmt.Errorf("failed to transition task to approved: %w", err)
			}
		}
		if err := task.TransitionTo(StatusMerged); err != nil {
			return fmt.Errorf("failed to transition task to merged: %w", err)
		}
	}

	l.queue.Update(task)

	if l.store != nil {
		if err := l.store.Save(task); err != nil {
			fmt.Printf("[LocalTaskSource] Warning: failed to persist task %s: %v\n", taskID, err)
		}
	}

	return nil
}

// GetName returns the source name
func (l *LocalTaskSource) GetName() string {
	return "Local Queue"
}

// ExternalTaskSource implements TaskSourceInterface for external APIs like Magnolia Planner
type ExternalTaskSource struct {
	name       string
	baseURL    string
	apiKey     string
	teamID     string
	httpClient *http.Client
}

// NewExternalTaskSource creates a task source backed by an external API
func NewExternalTaskSource(name, baseURL, apiKey, teamID string) TaskSourceInterface {
	return &ExternalTaskSource{
		name:    name,
		baseURL: baseURL,
		apiKey:  apiKey,
		teamID:  teamID,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// plannerTask matches the Magnolia Planner API task schema
type plannerTask struct {
	ID           string              `json:"id"`
	Title        string              `json:"title"`
	Repo         string              `json:"repo"`
	Priority     int                 `json:"priority"`
	Status       string              `json:"status"`
	Requirements []plannerRequirement `json:"requirements,omitempty"`
	Description  string              `json:"description,omitempty"`
	ClaimedBy    string              `json:"claimed_by,omitempty"`
	Branch       string              `json:"branch,omitempty"`
	PRUrl        string              `json:"pr_url,omitempty"`
}

type plannerRequirement struct {
	Text     string `json:"text"`
	Required bool   `json:"required"`
}

// FetchPendingTasks calls external API to get pending tasks
func (e *ExternalTaskSource) FetchPendingTasks() ([]*Task, error) {
	url := fmt.Sprintf("%s/api/v1/tasks?status=pending", e.baseURL)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if e.apiKey != "" {
		req.Header.Set("X-API-Key", e.apiKey)
	}

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch tasks from %s: %w", e.name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("external API returned status %d: %s", resp.StatusCode, string(body))
	}

	var response struct {
		Tasks []plannerTask `json:"tasks"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert external format to internal Task format
	tasks := make([]*Task, 0, len(response.Tasks))
	for _, pt := range response.Tasks {
		task := &Task{
			ID:          pt.ID,
			Title:       pt.Title,
			Description: pt.Description,
			Priority:    pt.Priority,
			Status:      StatusPending,
			Source:      TaskSource(e.name),
			Repo:        pt.Repo,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			Metadata:    make(map[string]string),
		}

		// Convert requirements
		if len(pt.Requirements) > 0 {
			task.Requirements = make([]Requirement, len(pt.Requirements))
			for i, req := range pt.Requirements {
				task.Requirements[i] = Requirement{
					Text:     req.Text,
					Required: req.Required,
					Met:      false,
				}
			}
		}

		// Store external task ID in metadata
		task.Metadata["external_id"] = pt.ID
		task.Metadata["external_source"] = e.name

		tasks = append(tasks, task)
	}

	return tasks, nil
}

// ClaimTask calls external API to claim a task
func (e *ExternalTaskSource) ClaimTask(taskID string, agentID string) error {
	url := fmt.Sprintf("%s/api/v1/tasks/%s/claim", e.baseURL, taskID)

	payload := map[string]interface{}{
		"team_id": e.teamID,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal claim request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create claim request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if e.apiKey != "" {
		req.Header.Set("X-API-Key", e.apiKey)
	}

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to claim task from %s: %w", e.name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("external API claim failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// CompleteTask calls external API to mark task as implemented
func (e *ExternalTaskSource) CompleteTask(taskID string, result TaskResult) error {
	url := fmt.Sprintf("%s/api/v1/tasks/%s/implemented", e.baseURL, taskID)

	payload := map[string]interface{}{
		"team_id": e.teamID,
		"branch":  result.Branch,
		"pr_url":  result.PRUrl,
	}

	if result.TokensUsed > 0 {
		payload["tokens_used"] = result.TokensUsed
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal complete request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create complete request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if e.apiKey != "" {
		req.Header.Set("X-API-Key", e.apiKey)
	}

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to complete task on %s: %w", e.name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("external API complete failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// GetName returns the source name
func (e *ExternalTaskSource) GetName() string {
	return e.name
}

// TaskBroker coordinates multiple task sources
type TaskBroker struct {
	sources []TaskSourceInterface
}

// NewTaskBroker creates a new task broker with multiple sources
func NewTaskBroker(sources ...TaskSourceInterface) *TaskBroker {
	return &TaskBroker{
		sources: sources,
	}
}

// FetchAllPendingTasks retrieves tasks from all configured sources
func (b *TaskBroker) FetchAllPendingTasks() (map[string][]*Task, error) {
	result := make(map[string][]*Task)
	var lastErr error

	for _, source := range b.sources {
		tasks, err := source.FetchPendingTasks()
		if err != nil {
			// Log error but continue with other sources
			fmt.Printf("[TaskBroker] Error fetching from %s: %v\n", source.GetName(), err)
			lastErr = err
			continue
		}
		result[source.GetName()] = tasks
	}

	// Only return error if all sources failed
	if len(result) == 0 && lastErr != nil {
		return nil, lastErr
	}

	return result, nil
}

// GetSource returns a specific source by name
func (b *TaskBroker) GetSource(name string) TaskSourceInterface {
	for _, source := range b.sources {
		if source.GetName() == name {
			return source
		}
	}
	return nil
}

// AddSource adds a new task source to the broker
func (b *TaskBroker) AddSource(source TaskSourceInterface) {
	b.sources = append(b.sources, source)
}

// RemoveSource removes a task source by name
func (b *TaskBroker) RemoveSource(name string) bool {
	for i, source := range b.sources {
		if source.GetName() == name {
			b.sources = append(b.sources[:i], b.sources[i+1:]...)
			return true
		}
	}
	return false
}

// ListSources returns names of all configured sources
func (b *TaskBroker) ListSources() []string {
	names := make([]string, len(b.sources))
	for i, source := range b.sources {
		names[i] = source.GetName()
	}
	return names
}
