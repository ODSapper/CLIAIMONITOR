package memory

import "time"

// MemoryDB is the main interface for cross-session memory operations
// This interface allows other work streams to develop in parallel without
// depending on the concrete implementation
type MemoryDB interface {
	// Repository operations
	DiscoverRepo(basePath string) (*Repo, error)
	GetRepo(repoID string) (*Repo, error)
	GetRepoByPath(basePath string) (*Repo, error)
	UpdateRepoScan(repoID string) error
	SetRepoRescan(repoID string, needsRescan bool) error

	// Repository files
	StoreRepoFile(file *RepoFile) error
	GetRepoFiles(repoID string, fileType string) ([]*RepoFile, error)
	GetRepoFile(repoID, filePath string) (*RepoFile, error)

	// Agent learning operations
	StoreAgentLearning(learning *AgentLearning) error
	GetAgentLearnings(filter LearnFilter) ([]*AgentLearning, error)
	GetRecentLearnings(limit int) ([]*AgentLearning, error)

	// Context summaries
	StoreContextSummary(summary *ContextSummary) error
	GetRecentSummaries(limit int) ([]*ContextSummary, error)
	GetSummariesByAgent(agentID string, limit int) ([]*ContextSummary, error)
	GetSummariesBySession(sessionID string) ([]*ContextSummary, error)

	// Workflow tasks
	CreateTask(task *WorkflowTask) error
	CreateTasks(tasks []*WorkflowTask) error
	GetTask(taskID string) (*WorkflowTask, error)
	GetTasks(filter TaskFilter) ([]*WorkflowTask, error)
	UpdateTaskStatus(taskID, status, agentID string) error
	UpdateTask(task *WorkflowTask) error

	// Human decisions
	StoreDecision(decision *HumanDecision) error
	GetRecentDecisions(limit int) ([]*HumanDecision, error)
	GetDecisionsByAgent(agentID string, limit int) ([]*HumanDecision, error)

	// Deployment operations
	CreateDeployment(deployment *Deployment) error
	GetDeployment(deploymentID int64) (*Deployment, error)
	GetRecentDeployments(repoID string, limit int) ([]*Deployment, error)
	UpdateDeploymentStatus(deploymentID int64, status string) error

	// Agent control operations
	RegisterAgent(agent *AgentControl) error
	UpdateStatus(agentID, status, currentTask string) error
	SetShutdownFlag(agentID string, reason string) error
	ClearShutdownFlag(agentID string) error
	MarkStopped(agentID, reason string) error
	RemoveAgent(agentID string) error
	GetAgent(agentID string) (*AgentControl, error)
	GetAllAgents() ([]*AgentControl, error)
	GetStaleAgents(threshold time.Duration) ([]*AgentControl, error)
	GetAgentsByStatus(status string) ([]*AgentControl, error)
	CheckShutdownFlag(agentID string) (bool, string, error)

	// Learning memory access
	AsLearningDB() LearningDB

	// Lifecycle
	Close() error
}

// Repo represents a discovered repository
type Repo struct {
	ID            string
	BasePath      string
	GitRemote     string
	ClaudeMDHash  string
	DiscoveredAt  time.Time
	LastScanned   time.Time
	NeedsRescan   bool
}

// NeedsRescan checks if the repository needs to be rescanned
func (r *Repo) ShouldRescan() bool {
	return r.NeedsRescan || time.Since(r.LastScanned) > 24*time.Hour
}

// RepoFile represents a discovered file in a repository
type RepoFile struct {
	RepoID       string
	FilePath     string
	FileType     string // 'claude_md', 'workflow_yaml', 'plan_yaml'
	ContentHash  string
	Content      string
	DiscoveredAt time.Time
	UpdatedAt    time.Time
}

// AgentLearning represents knowledge accumulated by an agent
type AgentLearning struct {
	ID        int64
	AgentID   string
	AgentType string // 'coder', 'tester', 'reviewer', 'supervisor'
	Category  string // 'error_pattern', 'solution', 'best_practice', 'workflow_insight'
	Title     string
	Content   string
	RepoID    string
	CreatedAt time.Time
}

// LearnFilter filters agent learnings
type LearnFilter struct {
	AgentID   string
	AgentType string
	Category  string
	RepoID    string
	Limit     int
	Since     time.Time
}

// WorkflowTask represents a task parsed from workflow files
type WorkflowTask struct {
	ID              string
	RepoID          string
	SourceFile      string
	Title           string
	Description     string
	Priority        string // 'low', 'medium', 'high', 'critical'
	Status          string // 'pending', 'assigned', 'in_progress', 'completed', 'blocked'
	AssignedAgentID string
	ParentTaskID    string
	EstimatedEffort string // 'small', 'medium', 'large'
	Tags            string // JSON array
	CreatedAt       time.Time
	UpdatedAt       time.Time
	CompletedAt     *time.Time
}

// TaskFilter filters workflow tasks
type TaskFilter struct {
	RepoID          string
	Status          string
	AssignedAgentID string
	Priority        string
	ParentTaskID    string
	Limit           int
	Offset          int
}

// ContextSummary stores session context before compaction
type ContextSummary struct {
	ID          int64
	SessionID   string
	AgentID     string
	Summary     string
	FullContext string // Optional: full context before compaction
	RepoID      string
	CreatedAt   time.Time
}

// HumanDecision records human guidance and approvals
type HumanDecision struct {
	ID            int64
	Context       string
	Question      string
	Answer        string
	DecisionType  string // 'approval', 'guidance', 'clarification', 'rejection'
	AgentID       string
	RelatedTaskID string
	RepoID        string
	CreatedAt     time.Time
}

// Deployment tracks agent spawning history
type Deployment struct {
	ID             int64
	RepoID         string
	DeploymentPlan string // JSON
	ProposedAt     time.Time
	ApprovedAt     *time.Time
	ExecutedAt     *time.Time
	Status         string // 'proposed', 'approved', 'executing', 'completed', 'failed'
	AgentConfigs   string // JSON array
	Result         string
}
