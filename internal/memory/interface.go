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

	// Captain context operations
	SetContext(key, value string, priority int, maxAgeHours int) error
	GetContext(key string) (*CaptainContext, error)
	GetAllContext() ([]*CaptainContext, error)
	GetContextByPriority(minPriority int) ([]*CaptainContext, error)
	DeleteContext(key string) error
	CleanExpiredContext() (int, error)

	// Captain session log
	LogSessionEvent(sessionID, eventType, summary, details, agentID string) error
	GetSessionLog(sessionID string, limit int) ([]*SessionLogEntry, error)
	GetRecentSessionLog(limit int) ([]*SessionLogEntry, error)

	// Metrics history
	RecordMetricsHistory(agentID, model string, tokensUsed int64, estimatedCost float64, taskID string) error

	// Metrics analysis
	GetMetricsByModel(modelFilter string) ([]*ModelMetrics, error)
	GetMetricsByAgentType() ([]*AgentTypeMetrics, error)
	GetMetricsByAgent() ([]*AgentMetricsSummary, error)
	RecordMetricsWithType(agentID, model, agentType, parentAgent string, tokensUsed int64, estimatedCost float64, taskID string, assignmentID *int64) error

	// Task assignments (SGT workflow)
	CreateAssignment(assignment *TaskAssignment) error
	GetAssignment(id int64) (*TaskAssignment, error)
	GetAssignmentsByTask(taskID string) ([]*TaskAssignment, error)
	GetAssignmentsByAgent(agentID string, status string) ([]*TaskAssignment, error)
	GetActiveAssignment(agentID string) (*TaskAssignment, error)
	UpdateAssignmentStatus(id int64, status string) error
	CompleteAssignment(id int64, status string, feedback string) error
	RequestRework(id int64, feedback string) error // Increment review_attempt, set status to "rework"
	AddWorker(worker *AssignmentWorker) error
	UpdateWorkerStatus(id int64, status, result string, tokensUsed int64) error
	GetWorkersByAssignment(assignmentID int64) ([]*AssignmentWorker, error)

	// Prompt template operations
	GetPromptTemplate(name string) (*PromptTemplate, error)
	GetPromptTemplateByRole(role string) (*PromptTemplate, error)
	GetAllPromptTemplates() ([]*PromptTemplate, error)
	SavePromptTemplate(template *PromptTemplate) error
	DeletePromptTemplate(name string) error

	// Review Board operations
	CreateReviewBoard(board *ReviewBoard) error
	GetReviewBoard(id int64) (*ReviewBoard, error)
	GetReviewBoardByAssignment(assignmentID int64) (*ReviewBoard, error)
	UpdateReviewBoard(board *ReviewBoard) error
	CreateDefect(defect *ReviewDefect) error
	GetBoardDefects(boardID int64) ([]*ReviewDefect, error)
	GetDefectsByReviewer(boardID int64, reviewerID string) ([]*ReviewDefect, error)
	CreateReviewerVote(vote *ReviewerVote) error
	GetReviewerVotes(boardID int64) ([]*ReviewerVote, error)
	GetOrCreateQualityScore(agentID, role string) (*AgentQualityScore, error)
	UpdateQualityScore(score *AgentQualityScore) error
	GetAgentLeaderboard(role string, limit int) ([]*AgentQualityScore, error)
	GetDefectCategories() ([]*DefectCategory, error)
	CalculateConsensus(boardID int64) (*ConsensusResult, error)
	UpdateQualityScoresAfterReview(boardID int64, consensus *ConsensusResult) error
	GenerateReviewReport(boardID int64) (string, error)
	SaveReviewReport(boardID int64, title, content, projectID string) error

	// Document operations
	CreateDocument(doc *Document) error
	GetDocument(id int64) (*Document, error)
	GetDocumentsByType(docType string, limit int) ([]*Document, error)
	GetDocumentsByProject(projectID string, limit int) ([]*Document, error)
	GetDocumentsByAuthor(authorID string, limit int) ([]*Document, error)
	SearchDocuments(query string, limit int) ([]*Document, error)
	UpdateDocument(doc *Document) error
	ArchiveDocument(id int64) error

	// Config store operations
	GetConfig(configType string) (*ConfigEntry, error)
	SaveConfig(configType, content, format string) error
	GetAllConfigs() ([]*ConfigEntry, error)

	// Health check
	Health() (*HealthStatus, error)

	// Lifecycle
	Close() error
}

// HealthStatus represents the health of the memory database
type HealthStatus struct {
	Connected       bool   `json:"connected"`
	SchemaVersion   int    `json:"schema_version"`
	AgentCount      int    `json:"agent_count"`
	TaskCount       int    `json:"task_count"`
	LearningCount   int    `json:"learning_count"`
	ContextCount    int    `json:"context_count"`
	DBPath          string `json:"db_path"`
	DBSizeBytes     int64  `json:"db_size_bytes"`
	LastContextSave string `json:"last_context_save,omitempty"`
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

// CaptainContext stores key-value context for Captain resumption
type CaptainContext struct {
	ID          int64
	Key         string
	Value       string
	Priority    int       // 1-10, higher = more important
	MaxAgeHours int       // Auto-expire after this many hours (0 = never)
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// SessionLogEntry records significant Captain events
type SessionLogEntry struct {
	ID        int64
	SessionID string
	EventType string // 'startup', 'command', 'spawn', 'decision', 'error', 'shutdown'
	Summary   string
	Details   string
	AgentID   string
	CreatedAt time.Time
}

// ModelMetrics represents aggregated metrics per model from the metrics_by_model view
type ModelMetrics struct {
	Model              string  `json:"model"`
	ReportCount        int     `json:"report_count"`
	TotalTokens        int64   `json:"total_tokens"`
	TotalCost          float64 `json:"total_cost"`
	AvgTokensPerReport float64 `json:"avg_tokens_per_report"`
}

// TaskAssignment tracks task handoffs between Captain and SGTs
type TaskAssignment struct {
	ID             int64
	TaskID         string
	AssignedTo     string
	AssignedBy     string
	AssignmentType string
	Status         string
	BranchName     string
	ReviewFeedback string
	ReviewAttempt  int
	WorkerCount    int
	StartedAt      *time.Time
	CompletedAt    *time.Time
	CreatedAt      time.Time
}

// AssignmentWorker tracks sub-agent work within an assignment
type AssignmentWorker struct {
	ID              int64
	AssignmentID    int64
	WorkerType      string
	WorkerID        string
	TaskDescription string
	Status          string
	Result          string
	TokensUsed      int64
	StartedAt       *time.Time
	CompletedAt     *time.Time
	CreatedAt       time.Time
}

// PromptTemplate stores agent system prompt templates
type PromptTemplate struct {
	ID          int64
	Name        string // Unique identifier, e.g., 'sgt-green', 'engineer'
	Role        string // Agent role, e.g., 'supervisor', 'engineer', 'security'
	Content     string // Full prompt template with {{PLACEHOLDERS}}
	Description string // Human-readable description
	Version     int
	IsActive    bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Document represents an internal work product (plan, report, review, etc.)
type Document struct {
	ID           int64
	DocType      string // 'plan', 'report', 'review', 'test_report', 'agent_work', 'config'
	Title        string
	Content      string
	Format       string // 'markdown', 'json', 'yaml', 'text'
	AuthorID     string
	ProjectID    string
	TaskID       string
	AssignmentID *int64
	Tags         []string // Stored as JSON
	Status       string   // 'draft', 'active', 'archived', 'superseded'
	Version      int
	ParentID     *int64
	CreatedAt    time.Time
	UpdatedAt    time.Time
	ArchivedAt   *time.Time
}

// ConfigEntry represents a stored configuration (teams.yaml, projects.yaml, etc.)
type ConfigEntry struct {
	ID         int64
	ConfigType string
	Content    string
	Format     string
	Version    int
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// Common context keys for Captain
const (
	CtxKeyCurrentFocus   = "current_focus"    // What Captain is currently working on
	CtxKeyRecentWork     = "recent_work"      // Summary of recent completed work
	CtxKeyPendingTasks   = "pending_tasks"    // Tasks waiting to be done
	CtxKeyActiveAgents   = "active_agents"    // Currently spawned agents and their tasks
	CtxKeyUserPrefs      = "user_preferences" // Human's stated preferences
	CtxKeyLastSession    = "last_session"     // Summary of previous session
	CtxKeyKnownIssues    = "known_issues"     // Issues discovered but not yet fixed
	CtxKeyProjectContext = "project_context"  // Key project information
)
