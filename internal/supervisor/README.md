# Supervisor Package

This package contains the supervisor backend logic for workflow scanning and deployment planning.

## Files

- **scanner.go** - Workflow file discovery and parsing
  - ScanForWorkflows() - Discovers CLAUDE.md, plans, workflows
  - ParseCLAUDEmd() - Extracts repository context
  - ParseWorkflowYAML() - Parses tasks from YAML

- **planner.go** - Deployment strategy and agent proposals
  - AnalyzeTasks() - Task complexity analysis
  - ProposeAgents() - Agent spawning recommendations
  - CreateDeploymentPlan() - Full deployment strategy generation

## Usage

```go
import "github.com/CLIAIMONITOR/internal/supervisor"

// Scan repository
scanner := supervisor.NewScanner(memDB)
result, err := scanner.ScanForWorkflows(repoID)

// Create deployment plan
planner := supervisor.NewPlanner(memDB)
plan, err := planner.CreateDeploymentPlan(repoID)
deploymentID, err := planner.StoreDeploymentPlan(plan)
```
