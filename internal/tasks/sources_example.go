// internal/tasks/sources_example.go
package tasks

import (
	"fmt"
)

// Example: Setting up a TaskBroker with multiple sources
//
// This example shows how Captain can pull tasks from both local queue
// and external sources like Magnolia Planner.
func ExampleTaskBroker() {
	// Assume we have a queue and store initialized
	var queue *Queue
	var store *Store

	// Create local task source
	localSource := NewLocalTaskSource(queue, store)

	// Create external task source for Magnolia Planner
	plannerSource := NewExternalTaskSource(
		"Magnolia Planner",                          // name
		"https://plannerprojectmss.vercel.app",      // baseURL
		"team-captain",                              // apiKey (team ID)
		"team-captain",                              // teamID
	)

	// Create broker with multiple sources
	broker := NewTaskBroker(localSource, plannerSource)

	// Fetch pending tasks from all sources
	allTasks, err := broker.FetchAllPendingTasks()
	if err != nil {
		fmt.Printf("Error fetching tasks: %v\n", err)
		return
	}

	// Display tasks by source
	for sourceName, tasks := range allTasks {
		fmt.Printf("\n=== Tasks from %s ===\n", sourceName)
		for _, task := range tasks {
			fmt.Printf("- [%s] %s (Priority: %d)\n", task.ID, task.Title, task.Priority)
		}
	}

	// Claim a task from a specific source
	if plannerTasks, ok := allTasks["Magnolia Planner"]; ok && len(plannerTasks) > 0 {
		task := plannerTasks[0]
		source := broker.GetSource("Magnolia Planner")

		if err := source.ClaimTask(task.ID, "agent-001"); err != nil {
			fmt.Printf("Failed to claim task: %v\n", err)
			return
		}

		fmt.Printf("Claimed task %s\n", task.ID)
	}

	// Complete a task with results
	result := TaskResult{
		Branch:      "task/MAH-001-implement-feature",
		PRUrl:       "https://github.com/org/repo/pull/123",
		TokensUsed:  15000,
		Success:     true,
		CompletedBy: "agent-001",
	}

	source := broker.GetSource("Magnolia Planner")
	if err := source.CompleteTask("MAH-001", result); err != nil {
		fmt.Printf("Failed to complete task: %v\n", err)
		return
	}

	fmt.Println("Task completed successfully")
}

// Example: Using TaskBroker to sync external tasks into local queue
//
// This shows how Captain can pull tasks from external sources
// and add them to the local queue for agent assignment.
func ExampleSyncExternalTasks() {
	var queue *Queue
	var store *Store

	// Setup broker
	localSource := NewLocalTaskSource(queue, store)
	plannerSource := NewExternalTaskSource(
		"Magnolia Planner",
		"https://plannerprojectmss.vercel.app",
		"team-captain",
		"team-captain",
	)

	_ = NewTaskBroker(localSource, plannerSource)

	// Fetch tasks from external source only
	externalTasks, err := plannerSource.FetchPendingTasks()
	if err != nil {
		fmt.Printf("Failed to fetch external tasks: %v\n", err)
		return
	}

	// Import external tasks into local queue
	imported := 0
	for _, task := range externalTasks {
		// Check if task already exists in local queue
		existing := queue.GetByID(task.ID)
		if existing != nil {
			continue // Skip duplicates
		}

		// Add to local queue
		queue.Add(task)
		if store != nil {
			store.Save(task)
		}
		imported++
	}

	fmt.Printf("Imported %d new tasks from external sources\n", imported)
}

// Example: Dynamic source management
//
// Shows how to add/remove task sources at runtime
func ExampleDynamicSources() {
	broker := NewTaskBroker()

	// Start with no sources
	fmt.Printf("Initial sources: %v\n", broker.ListSources())

	// Add sources dynamically
	var queue *Queue
	var store *Store

	localSource := NewLocalTaskSource(queue, store)
	broker.AddSource(localSource)

	plannerSource := NewExternalTaskSource(
		"Magnolia Planner",
		"https://plannerprojectmss.vercel.app",
		"team-captain",
		"team-captain",
	)
	broker.AddSource(plannerSource)

	fmt.Printf("Active sources: %v\n", broker.ListSources())
	// Output: Active sources: [Local Queue Magnolia Planner]

	// Remove a source
	if broker.RemoveSource("Magnolia Planner") {
		fmt.Println("Removed Magnolia Planner source")
	}

	fmt.Printf("Remaining sources: %v\n", broker.ListSources())
	// Output: Remaining sources: [Local Queue]
}
