// internal/tasks/sources_test.go
package tasks

import (
	"testing"
)

func TestLocalTaskSource_FetchPendingTasks(t *testing.T) {
	queue := NewQueue()
	source := NewLocalTaskSource(queue, nil)

	// Add some tasks
	task1 := NewTask("Task 1", "Description 1", 3)
	task2 := NewTask("Task 2", "Description 2", 5)
	task3 := NewTask("Task 3", "Description 3", 1)

	queue.Add(task1)
	queue.Add(task2)
	queue.Add(task3)

	// Fetch pending tasks
	tasks, err := source.FetchPendingTasks()
	if err != nil {
		t.Fatalf("FetchPendingTasks failed: %v", err)
	}

	if len(tasks) != 3 {
		t.Errorf("Expected 3 tasks, got %d", len(tasks))
	}

	// Verify all tasks are pending
	for _, task := range tasks {
		if task.Status != StatusPending {
			t.Errorf("Expected status %s, got %s", StatusPending, task.Status)
		}
	}
}

func TestLocalTaskSource_ClaimTask(t *testing.T) {
	queue := NewQueue()
	source := NewLocalTaskSource(queue, nil)

	task := NewTask("Test Task", "Test Description", 3)
	queue.Add(task)

	// Claim the task
	err := source.ClaimTask(task.ID, "agent-001")
	if err != nil {
		t.Fatalf("ClaimTask failed: %v", err)
	}

	// Verify task was claimed
	claimed := queue.GetByID(task.ID)
	if claimed == nil {
		t.Fatal("Task not found in queue after claim")
	}

	if claimed.AssignedTo != "agent-001" {
		t.Errorf("Expected AssignedTo='agent-001', got '%s'", claimed.AssignedTo)
	}

	if claimed.Status != StatusAssigned {
		t.Errorf("Expected status %s, got %s", StatusAssigned, claimed.Status)
	}

	if claimed.StartedAt == nil {
		t.Error("StartedAt should be set after claim")
	}
}

func TestLocalTaskSource_CompleteTask_Success(t *testing.T) {
	queue := NewQueue()
	source := NewLocalTaskSource(queue, nil)

	task := NewTask("Test Task", "Test Description", 3)
	task.Status = StatusApproved // Move to a state ready for merge
	queue.Add(task)

	// Complete the task
	result := TaskResult{
		Branch:      "task/TEST-001-description",
		PRUrl:       "https://github.com/org/repo/pull/123",
		TokensUsed:  5000,
		Success:     true,
		CompletedBy: "agent-001",
	}

	err := source.CompleteTask(task.ID, result)
	if err != nil {
		t.Fatalf("CompleteTask failed: %v", err)
	}

	// Verify task was completed
	completed := queue.GetByID(task.ID)
	if completed == nil {
		t.Fatal("Task not found in queue after completion")
	}

	if completed.Status != StatusMerged {
		t.Errorf("Expected status %s, got %s", StatusMerged, completed.Status)
	}

	if completed.Branch != result.Branch {
		t.Errorf("Expected branch '%s', got '%s'", result.Branch, completed.Branch)
	}

	if completed.PRUrl != result.PRUrl {
		t.Errorf("Expected PRUrl '%s', got '%s'", result.PRUrl, completed.PRUrl)
	}

	if completed.CompletedAt == nil {
		t.Error("CompletedAt should be set after completion")
	}

	if completed.Metadata["tokens_used"] != "5000" {
		t.Errorf("Expected tokens_used='5000', got '%s'", completed.Metadata["tokens_used"])
	}

	if completed.Metadata["completed_by"] != "agent-001" {
		t.Errorf("Expected completed_by='agent-001', got '%s'", completed.Metadata["completed_by"])
	}
}

func TestLocalTaskSource_CompleteTask_Failure(t *testing.T) {
	queue := NewQueue()
	source := NewLocalTaskSource(queue, nil)

	task := NewTask("Test Task", "Test Description", 3)
	task.Status = StatusInProgress
	queue.Add(task)

	// Complete the task with failure
	result := TaskResult{
		Success:  false,
		ErrorMsg: "Tests failed",
	}

	err := source.CompleteTask(task.ID, result)
	if err != nil {
		t.Fatalf("CompleteTask failed: %v", err)
	}

	// Verify task was blocked
	blocked := queue.GetByID(task.ID)
	if blocked == nil {
		t.Fatal("Task not found in queue after completion")
	}

	if blocked.Status != StatusBlocked {
		t.Errorf("Expected status %s, got %s", StatusBlocked, blocked.Status)
	}

	if blocked.Metadata["error"] != "Tests failed" {
		t.Errorf("Expected error='Tests failed', got '%s'", blocked.Metadata["error"])
	}
}

func TestTaskBroker_AddRemoveSources(t *testing.T) {
	broker := NewTaskBroker()

	if len(broker.ListSources()) != 0 {
		t.Error("Expected empty broker initially")
	}

	// Add a source
	queue := NewQueue()
	source := NewLocalTaskSource(queue, nil)
	broker.AddSource(source)

	sources := broker.ListSources()
	if len(sources) != 1 {
		t.Errorf("Expected 1 source, got %d", len(sources))
	}

	if sources[0] != "Local Queue" {
		t.Errorf("Expected 'Local Queue', got '%s'", sources[0])
	}

	// Get the source
	retrieved := broker.GetSource("Local Queue")
	if retrieved == nil {
		t.Error("Failed to retrieve source by name")
	}

	// Remove the source
	removed := broker.RemoveSource("Local Queue")
	if !removed {
		t.Error("Failed to remove source")
	}

	if len(broker.ListSources()) != 0 {
		t.Error("Expected empty broker after removal")
	}

	// Try to remove non-existent source
	removed = broker.RemoveSource("Nonexistent")
	if removed {
		t.Error("Should not remove non-existent source")
	}
}

func TestTaskBroker_FetchAllPendingTasks(t *testing.T) {
	// Create queue with tasks
	queue := NewQueue()
	task1 := NewTask("Task 1", "From local queue", 3)
	queue.Add(task1)

	// Create source
	source := NewLocalTaskSource(queue, nil)

	// Create broker with one source
	broker := NewTaskBroker(source)

	// Fetch all tasks
	allTasks, err := broker.FetchAllPendingTasks()
	if err != nil {
		t.Fatalf("FetchAllPendingTasks failed: %v", err)
	}

	if len(allTasks) != 1 {
		t.Errorf("Expected tasks from 1 source, got %d", len(allTasks))
	}

	// Verify we got the task
	totalTasks := 0
	for _, tasks := range allTasks {
		totalTasks += len(tasks)
	}

	if totalTasks != 1 {
		t.Errorf("Expected 1 total task, got %d", totalTasks)
	}

	// Verify the task content
	localTasks, ok := allTasks["Local Queue"]
	if !ok {
		t.Error("Expected to find tasks from 'Local Queue' source")
	}

	if len(localTasks) > 0 && localTasks[0].Title != "Task 1" {
		t.Errorf("Expected task title 'Task 1', got '%s'", localTasks[0].Title)
	}
}

func TestTaskBroker_GetSource(t *testing.T) {
	queue := NewQueue()
	source := NewLocalTaskSource(queue, nil)
	broker := NewTaskBroker(source)

	// Get existing source
	retrieved := broker.GetSource("Local Queue")
	if retrieved == nil {
		t.Error("Failed to retrieve existing source")
	}

	if retrieved.GetName() != "Local Queue" {
		t.Errorf("Expected 'Local Queue', got '%s'", retrieved.GetName())
	}

	// Get non-existent source
	notFound := broker.GetSource("Nonexistent")
	if notFound != nil {
		t.Error("Should return nil for non-existent source")
	}
}

func TestLocalTaskSource_GetName(t *testing.T) {
	queue := NewQueue()
	source := NewLocalTaskSource(queue, nil)

	if source.GetName() != "Local Queue" {
		t.Errorf("Expected 'Local Queue', got '%s'", source.GetName())
	}
}

func TestTaskResult_Structure(t *testing.T) {
	// Test that TaskResult can hold all expected fields
	result := TaskResult{
		Branch:      "task/TEST-001",
		PRUrl:       "https://github.com/test",
		TokensUsed:  1000,
		Success:     true,
		ErrorMsg:    "",
		CompletedBy: "agent-001",
	}

	if result.Branch != "task/TEST-001" {
		t.Error("Branch field not set correctly")
	}

	if result.TokensUsed != 1000 {
		t.Error("TokensUsed field not set correctly")
	}

	if !result.Success {
		t.Error("Success field not set correctly")
	}
}

func TestLocalTaskSource_ClaimTask_NotFound(t *testing.T) {
	queue := NewQueue()
	source := NewLocalTaskSource(queue, nil)

	err := source.ClaimTask("nonexistent-id", "agent-001")
	if err == nil {
		t.Error("Expected error when claiming non-existent task")
	}
}

func TestLocalTaskSource_CompleteTask_NotFound(t *testing.T) {
	queue := NewQueue()
	source := NewLocalTaskSource(queue, nil)

	result := TaskResult{Success: true}
	err := source.CompleteTask("nonexistent-id", result)
	if err == nil {
		t.Error("Expected error when completing non-existent task")
	}
}

func TestExternalTaskSource_GetName(t *testing.T) {
	source := NewExternalTaskSource(
		"Test Source",
		"https://example.com",
		"test-key",
		"test-team",
	)

	if source.GetName() != "Test Source" {
		t.Errorf("Expected 'Test Source', got '%s'", source.GetName())
	}
}

// Integration test: Full workflow
func TestTaskWorkflow_LocalSource(t *testing.T) {
	queue := NewQueue()
	source := NewLocalTaskSource(queue, nil)

	// Step 1: Create and add task
	task := NewTask("Full Workflow Test", "Test complete workflow", 3)
	queue.Add(task)

	// Step 2: Fetch pending tasks
	pending, err := source.FetchPendingTasks()
	if err != nil || len(pending) != 1 {
		t.Fatal("Failed to fetch pending tasks")
	}

	// Step 3: Claim task
	if err := source.ClaimTask(task.ID, "agent-001"); err != nil {
		t.Fatalf("Failed to claim task: %v", err)
	}

	claimed := queue.GetByID(task.ID)
	if claimed.Status != StatusAssigned {
		t.Error("Task should be assigned after claim")
	}

	// Step 4: Move through workflow states
	claimed.Status = StatusInProgress
	queue.Update(claimed)

	claimed.Status = StatusReview
	queue.Update(claimed)

	claimed.Status = StatusApproved
	queue.Update(claimed)

	// Step 5: Complete task
	result := TaskResult{
		Branch:      "task/TEST-workflow",
		PRUrl:       "https://github.com/org/repo/pull/999",
		TokensUsed:  7500,
		Success:     true,
		CompletedBy: "agent-001",
	}

	if err := source.CompleteTask(task.ID, result); err != nil {
		t.Fatalf("Failed to complete task: %v", err)
	}

	// Verify final state
	completed := queue.GetByID(task.ID)
	if completed.Status != StatusMerged {
		t.Errorf("Expected final status %s, got %s", StatusMerged, completed.Status)
	}

	if completed.Branch != result.Branch {
		t.Error("Branch not set correctly")
	}

	if completed.PRUrl != result.PRUrl {
		t.Error("PRUrl not set correctly")
	}

	if completed.CompletedAt == nil {
		t.Error("CompletedAt should be set")
	}

	// Verify task is no longer in pending
	stillPending, _ := source.FetchPendingTasks()
	if len(stillPending) != 0 {
		t.Error("Completed task should not appear in pending list")
	}
}

func TestTaskBroker_MultipleSources_Prioritization(t *testing.T) {
	// Scenario: Tasks from a single source should be prioritizable
	// Captain can decide which to work on based on priority

	queue := NewQueue()
	highPriority := NewTask("Critical Bug", "Fix production issue", 1)
	lowPriority := NewTask("Documentation", "Update README", 7)
	queue.Add(highPriority)
	queue.Add(lowPriority)

	source := NewLocalTaskSource(queue, nil)
	broker := NewTaskBroker(source)

	allTasks, err := broker.FetchAllPendingTasks()
	if err != nil {
		t.Fatal(err)
	}

	// Collect all tasks and find highest priority
	var allTasksList []*Task
	for _, tasks := range allTasks {
		allTasksList = append(allTasksList, tasks...)
	}

	if len(allTasksList) != 2 {
		t.Errorf("Expected 2 tasks total, got %d", len(allTasksList))
	}

	// Find highest priority task
	var highestPriority *Task
	for _, task := range allTasksList {
		if highestPriority == nil || task.Priority < highestPriority.Priority {
			highestPriority = task
		}
	}

	if highestPriority.Priority != 1 {
		t.Errorf("Expected highest priority to be 1, got %d", highestPriority.Priority)
	}

	if highestPriority.Title != "Critical Bug" {
		t.Errorf("Wrong task selected as highest priority: %s", highestPriority.Title)
	}
}
