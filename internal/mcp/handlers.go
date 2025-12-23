package mcp

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/CLIAIMONITOR/internal/events"
	"github.com/CLIAIMONITOR/internal/types"
	"github.com/CLIAIMONITOR/internal/wezterm"
	"github.com/google/uuid"
)

// ToolCallbacks interface for tool handlers to call back into services
type ToolCallbacks struct {
	OnRequestHumanInput   func(req *types.HumanInputRequest) (interface{}, error)
	OnLogActivity         func(activity *types.ActivityLog) (interface{}, error)
	OnGetAgentMetrics     func() (interface{}, error)
	OnGetPendingQuestions func() (interface{}, error)
	OnEscalateAlert       func(alert *types.Alert) (interface{}, error)
	OnSubmitJudgment      func(judgment *types.SupervisorJudgment) (interface{}, error)
	OnGetAgentList        func() (interface{}, error)
	OnGetMyTasks          func(agentID, status string) (interface{}, error)
	OnClaimTask           func(agentID, taskID string) (interface{}, error)
	OnUpdateTaskProgress  func(agentID, taskID, status, note string) (interface{}, error)
	OnCompleteTask        func(agentID, taskID, summary string) (interface{}, error)
	OnSubmitReconReport   func(agentID string, report map[string]interface{}) (interface{}, error)
	OnRequestGuidance     func(agentID string, guidance map[string]interface{}) (interface{}, error)
	OnReportProgress      func(agentID string, progress map[string]interface{}) (interface{}, error)

	// Learning memory callbacks
	OnStoreKnowledge    func(agentID string, knowledge map[string]interface{}) (interface{}, error)
	OnSearchKnowledge   func(query, category string, limit int) (interface{}, error)
	OnRecordEpisode     func(agentID string, episode map[string]interface{}) (interface{}, error)
	OnGetRecentEpisodes func(sessionID string, limit int) (interface{}, error)
	OnSearchEpisodes    func(query, project string, limit int) (interface{}, error)

	// Skill Router callbacks (replaces PowerShell heartbeat)
	OnSkillQuery func(agentID, query string, limit int) (interface{}, error)

	// Captain context callbacks (for session persistence)
	OnSaveContext   func(key, value string, priority, maxAgeHours int) (interface{}, error)
	OnGetContext    func(key string) (interface{}, error)
	OnGetAllContext func() (interface{}, error)
	OnLogSession    func(sessionID, eventType, summary, details, agentID string) (interface{}, error)

	// Captain messages callbacks (human -> Captain chat)
	OnGetCaptainMessages   func() (interface{}, error)
	OnMarkMessagesRead     func(ids []string) (interface{}, error)
	OnSendCaptainResponse  func(text string) (interface{}, error)

	// Metrics analysis callbacks
	OnGetMetricsByModel func(modelFilter string) (interface{}, error)

	// SGT workflow callbacks
	OnDispatchTask       func(taskID, assignTo, assignmentType, branchName string) (interface{}, error)
	OnAcceptAssignment   func(agentID string, assignmentID int64) (interface{}, error)
	OnGetMyAssignment    func(agentID string) (interface{}, error)
	OnLogWorker          func(agentID string, assignmentID int64, workerType, description string) (interface{}, error)
	OnSubmitForReview    func(agentID string, assignmentID int64, branchName string) (interface{}, error)
	OnSubmitReviewResult func(agentID string, assignmentID int64, approved bool, feedback string) (interface{}, error)
	OnCompleteWorker     func(agentID string, workerID int64, status, result, model string, tokensUsed int64) (interface{}, error)

	// Metrics by agent type
	OnGetMetricsByAgentType func() (interface{}, error)
	OnGetMetricsByAgent     func() (interface{}, error)

	// Review Board callbacks (multi-reviewer Fagan-style inspection)
	OnCreateReviewBoard   func(assignmentID int64, reviewerCount int, complexity int, riskLevel string) (interface{}, error)
	OnSubmitDefect        func(agentID string, boardID int64, defect map[string]interface{}) (interface{}, error)
	OnRecordReviewerVote  func(boardID int64, reviewerID string, approved bool, confidence int, defectsFound int, tokensUsed int64) (interface{}, error)
	OnFinalizeBoard       func(boardID int64) (interface{}, error)
	OnGetAgentLeaderboard func(role string, limit int) (interface{}, error)
	OnGetDefectCategories func() (interface{}, error)

	// Document storage callbacks
	OnSaveDocument     func(agentID string, doc map[string]interface{}) (interface{}, error)
	OnGetDocument      func(id int64) (interface{}, error)
	OnSearchDocuments  func(query, docType, projectID, authorID string, limit int) (interface{}, error)
	OnListMyDocuments  func(agentID, docType string, limit int) (interface{}, error)
}

// RegisterDefaultTools registers all standard MCP tools
// This is called during server setup with callbacks to other services
func RegisterDefaultTools(s *Server, callbacks ToolCallbacks) {
	// request_human_input - Agent needs human answer
	s.RegisterTool(ToolDefinition{
		Name:        "request_human_input",
		Description: "Request input from a human operator",
		Parameters: map[string]ParameterDef{
			"question": {Type: "string", Description: "The question to ask", Required: true},
			"context":  {Type: "string", Description: "Additional context", Required: false},
		},
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			question, _ := params["question"].(string)
			context, _ := params["context"].(string)

			req := &types.HumanInputRequest{
				ID:        uuid.New().String(),
				AgentID:   agentID,
				Question:  question,
				Context:   context,
				CreatedAt: time.Now(),
				Answered:  false,
			}
			return callbacks.OnRequestHumanInput(req)
		},
	})

	// log_activity - General activity logging
	s.RegisterTool(ToolDefinition{
		Name:        "log_activity",
		Description: "Log an activity for the dashboard",
		Parameters: map[string]ParameterDef{
			"action":  {Type: "string", Description: "The action performed", Required: true},
			"details": {Type: "string", Description: "Additional details", Required: false},
		},
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			action, _ := params["action"].(string)
			details, _ := params["details"].(string)

			activity := &types.ActivityLog{
				ID:        uuid.New().String(),
				AgentID:   agentID,
				Action:    action,
				Details:   details,
				Timestamp: time.Now(),
			}
			return callbacks.OnLogActivity(activity)
		},
	})

	// Task workflow tools
	registerTaskTools(s, callbacks)

	// Supervisor-only tools
	registerSupervisorTools(s, callbacks)

	// Snake reconnaissance tools
	registerSnakeTools(s, callbacks)
}

// registerTaskTools adds task workflow management tools
func registerTaskTools(s *Server, callbacks ToolCallbacks) {
	// get_my_tasks - List tasks assigned to the calling agent
	s.RegisterTool(ToolDefinition{
		Name:        "get_my_tasks",
		Description: "Get workflow tasks assigned to you",
		Parameters: map[string]ParameterDef{
			"status": {Type: "string", Description: "Filter by status (pending, assigned, in_progress, completed, blocked)", Required: false},
		},
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			status, _ := params["status"].(string)
			return callbacks.OnGetMyTasks(agentID, status)
		},
	})

	// claim_task - Claim an unassigned pending task
	s.RegisterTool(ToolDefinition{
		Name:        "claim_task",
		Description: "Claim a pending task to work on. Only works for unassigned tasks.",
		Parameters: map[string]ParameterDef{
			"task_id": {Type: "string", Description: "The ID of the task to claim", Required: true},
		},
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			taskID, _ := params["task_id"].(string)
			return callbacks.OnClaimTask(agentID, taskID)
		},
	})

	// update_task_progress - Update status of your assigned task
	s.RegisterTool(ToolDefinition{
		Name:        "update_task_progress",
		Description: "Update progress on a task you're working on",
		Parameters: map[string]ParameterDef{
			"task_id": {Type: "string", Description: "The task ID", Required: true},
			"status":  {Type: "string", Description: "New status: in_progress or blocked", Required: true},
			"note":    {Type: "string", Description: "Optional progress note", Required: false},
		},
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			taskID, _ := params["task_id"].(string)
			status, _ := params["status"].(string)
			note, _ := params["note"].(string)
			return callbacks.OnUpdateTaskProgress(agentID, taskID, status, note)
		},
	})

	// complete_task - Mark task as completed with summary
	s.RegisterTool(ToolDefinition{
		Name:        "complete_task",
		Description: "Mark a task as completed with a summary of what was done",
		Parameters: map[string]ParameterDef{
			"task_id": {Type: "string", Description: "The task ID", Required: true},
			"summary": {Type: "string", Description: "Summary of work completed", Required: true},
		},
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			taskID, _ := params["task_id"].(string)
			summary, _ := params["summary"].(string)
			return callbacks.OnCompleteTask(agentID, taskID, summary)
		},
	})
}

// registerSupervisorTools adds supervisor-specific tools
func registerSupervisorTools(s *Server, callbacks ToolCallbacks) {
	// get_agent_metrics - Supervisor retrieves all metrics
	s.RegisterTool(ToolDefinition{
		Name:        "get_agent_metrics",
		Description: "Get metrics for all agents (supervisor only)",
		Parameters:  map[string]ParameterDef{},
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			return callbacks.OnGetAgentMetrics()
		},
	})

	// get_pending_questions - Supervisor checks human input queue
	s.RegisterTool(ToolDefinition{
		Name:        "get_pending_questions",
		Description: "Get pending human input requests (supervisor only)",
		Parameters:  map[string]ParameterDef{},
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			return callbacks.OnGetPendingQuestions()
		},
	})

	// escalate_alert - Supervisor creates alert
	s.RegisterTool(ToolDefinition{
		Name:        "escalate_alert",
		Description: "Create an alert for human attention",
		Parameters: map[string]ParameterDef{
			"type":     {Type: "string", Description: "Alert type", Required: true},
			"message":  {Type: "string", Description: "Alert message", Required: true},
			"severity": {Type: "string", Description: "warning or critical", Required: true},
			"agent_id": {Type: "string", Description: "Related agent ID (optional)", Required: false},
		},
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			alertType, _ := params["type"].(string)
			message, _ := params["message"].(string)
			severity, _ := params["severity"].(string)
			relatedAgent, _ := params["agent_id"].(string)

			alert := &types.Alert{
				ID:        uuid.New().String(),
				Type:      alertType,
				AgentID:   relatedAgent,
				Message:   message,
				Severity:  severity,
				CreatedAt: time.Now(),
			}
			return callbacks.OnEscalateAlert(alert)
		},
	})

	// submit_judgment - Supervisor records decision
	s.RegisterTool(ToolDefinition{
		Name:        "submit_judgment",
		Description: "Record a supervisor judgment/decision",
		Parameters: map[string]ParameterDef{
			"agent_id":  {Type: "string", Description: "Agent being judged", Required: true},
			"issue":     {Type: "string", Description: "The issue observed", Required: true},
			"decision":  {Type: "string", Description: "The decision made", Required: true},
			"reasoning": {Type: "string", Description: "Reasoning for decision", Required: true},
			"action":    {Type: "string", Description: "Action: restart, pause, escalate, continue", Required: true},
		},
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			targetAgentID, _ := params["agent_id"].(string)
			issue, _ := params["issue"].(string)
			decision, _ := params["decision"].(string)
			reasoning, _ := params["reasoning"].(string)
			action, _ := params["action"].(string)

			judgment := &types.SupervisorJudgment{
				ID:        uuid.New().String(),
				AgentID:   targetAgentID,
				Issue:     issue,
				Decision:  decision,
				Reasoning: reasoning,
				Action:    action,
				Timestamp: time.Now(),
			}
			return callbacks.OnSubmitJudgment(judgment)
		},
	})

	// get_agent_list - Get all agents and their status
	s.RegisterTool(ToolDefinition{
		Name:        "get_agent_list",
		Description: "Get list of all agents and their current status",
		Parameters:  map[string]ParameterDef{},
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			return callbacks.OnGetAgentList()
		},
	})
}

// registerSnakeTools adds Snake reconnaissance agent tools
func registerSnakeTools(s *Server, callbacks ToolCallbacks) {
	// submit_recon_report - Snake submits reconnaissance findings
	s.RegisterTool(ToolDefinition{
		Name:        "submit_recon_report",
		Description: "Submit reconnaissance findings to Captain",
		Parameters: map[string]ParameterDef{
			"environment": {Type: "string", Description: "Target environment name (e.g., 'CLIAIMONITOR', 'customer-acme')", Required: true},
			"mission":     {Type: "string", Description: "Mission type (e.g., 'initial_recon', 'security_audit')", Required: true},
			"findings":    {Type: "object", Description: "Findings object with critical, high, medium, low arrays", Required: true},
			"summary":     {Type: "object", Description: "Summary object with scan statistics", Required: true},
			"recommendations": {Type: "object", Description: "Recommendations object with immediate, short_term, long_term arrays", Required: true},
		},
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			// Build complete report with metadata
			report := map[string]interface{}{
				"agent_id":    agentID,
				"timestamp":   time.Now().Format(time.RFC3339),
				"environment": params["environment"],
				"mission":     params["mission"],
				"findings":    params["findings"],
				"summary":     params["summary"],
				"recommendations": params["recommendations"],
			}
			return callbacks.OnSubmitReconReport(agentID, report)
		},
	})

	// request_guidance - Snake asks Captain for direction
	s.RegisterTool(ToolDefinition{
		Name:        "request_guidance",
		Description: "Request guidance from Captain on ambiguous situation",
		Parameters: map[string]ParameterDef{
			"situation":      {Type: "string", Description: "Description of the ambiguous or unclear situation", Required: true},
			"options":        {Type: "array", Description: "Array of possible courses of action", Required: true},
			"recommendation": {Type: "string", Description: "Your recommended approach", Required: true},
		},
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			guidance := map[string]interface{}{
				"agent_id":       agentID,
				"timestamp":      time.Now().Format(time.RFC3339),
				"situation":      params["situation"],
				"options":        params["options"],
				"recommendation": params["recommendation"],
			}
			return callbacks.OnRequestGuidance(agentID, guidance)
		},
	})

	// report_progress - Snake reports scan progress
	s.RegisterTool(ToolDefinition{
		Name:        "report_progress",
		Description: "Report reconnaissance progress at key milestones",
		Parameters: map[string]ParameterDef{
			"phase":            {Type: "string", Description: "Current scan phase (e.g., 'architecture', 'security')", Required: true},
			"percent_complete": {Type: "number", Description: "Estimated completion percentage (0-100)", Required: true},
			"files_scanned":    {Type: "number", Description: "Number of files scanned so far", Required: true},
			"findings_so_far":  {Type: "number", Description: "Count of findings discovered so far", Required: true},
		},
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			progress := map[string]interface{}{
				"agent_id":         agentID,
				"timestamp":        time.Now().Format(time.RFC3339),
				"phase":            params["phase"],
				"percent_complete": params["percent_complete"],
				"files_scanned":    params["files_scanned"],
				"findings_so_far":  params["findings_so_far"],
			}
			return callbacks.OnReportProgress(agentID, progress)
		},
	})

	// Learning memory tools
	registerLearningTools(s, callbacks)
}

// registerLearningTools adds RAG memory tools for knowledge storage and retrieval
func registerLearningTools(s *Server, callbacks ToolCallbacks) {
	// store_knowledge - Store something learned for future retrieval
	s.RegisterTool(ToolDefinition{
		Name:        "store_knowledge",
		Description: "Store knowledge learned from experience for future retrieval. Use this to save solutions, patterns, best practices, and gotchas.",
		Parameters: map[string]ParameterDef{
			"category": {Type: "string", Description: "Category: error_solution, pattern, best_practice, gotcha", Required: true},
			"title":    {Type: "string", Description: "Brief title/summary of the knowledge", Required: true},
			"content":  {Type: "string", Description: "Full details of what was learned", Required: true},
			"tags":     {Type: "array", Description: "Optional tags for filtering (e.g., ['api', 'http'])", Required: false},
		},
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			if callbacks.OnStoreKnowledge == nil {
				return map[string]interface{}{"error": "Learning memory not configured"}, nil
			}
			knowledge := map[string]interface{}{
				"agent_id":   agentID,
				"timestamp":  time.Now().Format(time.RFC3339),
				"category":   params["category"],
				"title":      params["title"],
				"content":    params["content"],
				"tags":       params["tags"],
			}
			return callbacks.OnStoreKnowledge(agentID, knowledge)
		},
	})

	// search_knowledge - Find relevant knowledge using TF-IDF search
	s.RegisterTool(ToolDefinition{
		Name:        "search_knowledge",
		Description: "Search stored knowledge for relevant information. Returns matching knowledge entries ranked by relevance.",
		Parameters: map[string]ParameterDef{
			"query":    {Type: "string", Description: "Natural language query to search for", Required: true},
			"category": {Type: "string", Description: "Optional category filter: error_solution, pattern, best_practice, gotcha", Required: false},
			"limit":    {Type: "number", Description: "Maximum results to return (default: 5)", Required: false},
		},
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			if callbacks.OnSearchKnowledge == nil {
				return map[string]interface{}{"error": "Learning memory not configured"}, nil
			}
			query, _ := params["query"].(string)
			category, _ := params["category"].(string)
			limit := 5
			if l, ok := params["limit"].(float64); ok {
				limit = int(l)
			}
			return callbacks.OnSearchKnowledge(query, category, limit)
		},
	})

	// record_episode - Log what happened in current session
	s.RegisterTool(ToolDefinition{
		Name:        "record_episode",
		Description: "Record an episode of what happened. Use for logging actions, errors, decisions, and outcomes for session continuity.",
		Parameters: map[string]ParameterDef{
			"event_type":  {Type: "string", Description: "Type: action, error, decision, outcome", Required: true},
			"title":       {Type: "string", Description: "Brief title of what happened", Required: true},
			"content":     {Type: "string", Description: "Full details of the event", Required: true},
			"project":     {Type: "string", Description: "Optional project/repo name", Required: false},
			"importance":  {Type: "number", Description: "Importance 0-1 (default: 0.5)", Required: false},
		},
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			if callbacks.OnRecordEpisode == nil {
				return map[string]interface{}{"error": "Learning memory not configured"}, nil
			}
			importance := 0.5
			if imp, ok := params["importance"].(float64); ok {
				importance = imp
			}
			episode := map[string]interface{}{
				"agent_id":   agentID,
				"timestamp":  time.Now().Format(time.RFC3339),
				"event_type": params["event_type"],
				"title":      params["title"],
				"content":    params["content"],
				"project":    params["project"],
				"importance": importance,
			}
			return callbacks.OnRecordEpisode(agentID, episode)
		},
	})

	// get_recent_episodes - Get context from current/recent sessions
	s.RegisterTool(ToolDefinition{
		Name:        "get_recent_episodes",
		Description: "Get recent episodes for context. Useful for understanding what happened recently.",
		Parameters: map[string]ParameterDef{
			"session_id": {Type: "string", Description: "Optional session ID (defaults to current)", Required: false},
			"limit":      {Type: "number", Description: "Maximum results (default: 10)", Required: false},
		},
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			if callbacks.OnGetRecentEpisodes == nil {
				return map[string]interface{}{"error": "Learning memory not configured"}, nil
			}
			sessionID, _ := params["session_id"].(string)
			limit := 10
			if l, ok := params["limit"].(float64); ok {
				limit = int(l)
			}
			return callbacks.OnGetRecentEpisodes(sessionID, limit)
		},
	})

	// search_episodes - Find past similar situations
	s.RegisterTool(ToolDefinition{
		Name:        "search_episodes",
		Description: "Search past episodes for similar situations. Useful for finding what happened before with similar contexts.",
		Parameters: map[string]ParameterDef{
			"query":   {Type: "string", Description: "Search query", Required: true},
			"project": {Type: "string", Description: "Optional project filter", Required: false},
			"limit":   {Type: "number", Description: "Maximum results (default: 5)", Required: false},
		},
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			if callbacks.OnSearchEpisodes == nil {
				return map[string]interface{}{"error": "Learning memory not configured"}, nil
			}
			query, _ := params["query"].(string)
			project, _ := params["project"].(string)
			limit := 5
			if l, ok := params["limit"].(float64); ok {
				limit = int(l)
			}
			return callbacks.OnSearchEpisodes(query, project, limit)
		},
	})

	// Skill Router tools
	registerSkillRouterTools(s, callbacks)

	// WezTerm control tools
	registerWezTermTools(s)
}

// registerSkillRouterTools adds the skill router query tool
func registerSkillRouterTools(s *Server, callbacks ToolCallbacks) {
	// skill_query - Route queries to the appropriate data source
	s.RegisterTool(ToolDefinition{
		Name:        "skill_query",
		Description: "Smart query router that automatically routes your question to the right data source (knowledge, episodes, agents, tasks, recon). Use this when you need information but aren't sure which specific tool to use.",
		Parameters: map[string]ParameterDef{
			"query": {Type: "string", Description: "Natural language query (e.g., 'how do I fix auth redirect', 'what agents are running', 'what happened last session')", Required: true},
			"limit": {Type: "number", Description: "Maximum results (default: 10)", Required: false},
		},
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			if callbacks.OnSkillQuery == nil {
				return map[string]interface{}{"error": "Skill Router not configured"}, nil
			}
			query, _ := params["query"].(string)
			limit := 10
			if l, ok := params["limit"].(float64); ok {
				limit = int(l)
			}
			return callbacks.OnSkillQuery(agentID, query, limit)
		},
	})

	// Register Captain context tools
	registerContextTools(s, callbacks)
}

// registerContextTools adds Captain context persistence tools
func registerContextTools(s *Server, callbacks ToolCallbacks) {
	// save_context - Save context to memory.db for persistence across restarts
	s.RegisterTool(ToolDefinition{
		Name:        "save_context",
		Description: "Save context to memory.db for persistence across restarts. Use this to remember important information between sessions.",
		Parameters: map[string]ParameterDef{
			"key":           {Type: "string", Description: "Context key (e.g., 'current_focus', 'recent_work', 'pending_tasks')", Required: true},
			"value":         {Type: "string", Description: "The context content to save", Required: true},
			"priority":      {Type: "number", Description: "Priority 1-10, higher = more important to preserve (default: 5)", Required: false},
			"max_age_hours": {Type: "number", Description: "Auto-expire after this many hours, 0 = never expire (default: 24)", Required: false},
		},
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			if callbacks.OnSaveContext == nil {
				return map[string]interface{}{"error": "Context persistence not configured"}, nil
			}
			key, _ := params["key"].(string)
			value, _ := params["value"].(string)
			priority := 5
			if p, ok := params["priority"].(float64); ok {
				priority = int(p)
			}
			maxAgeHours := 24
			if m, ok := params["max_age_hours"].(float64); ok {
				maxAgeHours = int(m)
			}
			return callbacks.OnSaveContext(key, value, priority, maxAgeHours)
		},
	})

	// get_context - Get a specific context entry
	s.RegisterTool(ToolDefinition{
		Name:        "get_context",
		Description: "Get a specific context entry from memory.db.",
		Parameters: map[string]ParameterDef{
			"key": {Type: "string", Description: "Context key to retrieve", Required: true},
		},
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			if callbacks.OnGetContext == nil {
				return map[string]interface{}{"error": "Context persistence not configured"}, nil
			}
			key, _ := params["key"].(string)
			return callbacks.OnGetContext(key)
		},
	})

	// get_all_context - Get all saved context entries
	s.RegisterTool(ToolDefinition{
		Name:        "get_all_context",
		Description: "Get all saved context entries from memory.db. Use this at startup to restore session state.",
		Parameters:  map[string]ParameterDef{},
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			if callbacks.OnGetAllContext == nil {
				return map[string]interface{}{"error": "Context persistence not configured"}, nil
			}
			return callbacks.OnGetAllContext()
		},
	})

	// log_session - Log a significant event to the session log
	s.RegisterTool(ToolDefinition{
		Name:        "log_session",
		Description: "Log a significant event to the session log for historical tracking.",
		Parameters: map[string]ParameterDef{
			"event_type": {Type: "string", Description: "Event type: startup, command, spawn, decision, error, shutdown", Required: true},
			"summary":    {Type: "string", Description: "Brief summary of the event", Required: true},
			"details":    {Type: "string", Description: "Optional detailed information", Required: false},
		},
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			if callbacks.OnLogSession == nil {
				return map[string]interface{}{"error": "Session logging not configured"}, nil
			}
			eventType, _ := params["event_type"].(string)
			summary, _ := params["summary"].(string)
			details, _ := params["details"].(string)
			// Use agent ID as session ID
			return callbacks.OnLogSession(agentID, eventType, summary, details, agentID)
		},
	})

	// get_captain_messages - Captain polls for messages from human
	s.RegisterTool(ToolDefinition{
		Name:        "get_captain_messages",
		Description: "Get unread messages from human sent via dashboard chat. Captain should poll this periodically.",
		Parameters:  map[string]ParameterDef{},
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			if callbacks.OnGetCaptainMessages == nil {
				return map[string]interface{}{"messages": []interface{}{}, "count": 0}, nil
			}
			return callbacks.OnGetCaptainMessages()
		},
	})

	// mark_messages_read - Captain marks messages as read after processing
	s.RegisterTool(ToolDefinition{
		Name:        "mark_messages_read",
		Description: "Mark captain messages as read after processing them.",
		Parameters: map[string]ParameterDef{
			"message_ids": {Type: "array", Description: "Array of message IDs to mark as read", Required: true},
		},
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			if callbacks.OnMarkMessagesRead == nil {
				return map[string]interface{}{"error": "Message marking not configured"}, nil
			}
			// Extract message IDs from params
			var ids []string
			if idsRaw, ok := params["message_ids"].([]interface{}); ok {
				for _, id := range idsRaw {
					if s, ok := id.(string); ok {
						ids = append(ids, s)
					}
				}
			}
			return callbacks.OnMarkMessagesRead(ids)
		},
	})

	// send_captain_response - Captain sends a response to the dashboard chat
	s.RegisterTool(ToolDefinition{
		Name:        "send_captain_response",
		Description: "Send a response message to the dashboard chat. Use this to reply to human messages.",
		Parameters: map[string]ParameterDef{
			"text": {Type: "string", Description: "The response message to send to the dashboard", Required: true},
		},
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			if callbacks.OnSendCaptainResponse == nil {
				return map[string]interface{}{"error": "Captain response not configured"}, nil
			}
			text, _ := params["text"].(string)
			if text == "" {
				return map[string]interface{}{"error": "text parameter required"}, nil
			}
			return callbacks.OnSendCaptainResponse(text)
		},
	})

	// Register metrics tools
	registerMetricsTools(s, callbacks)
}

// registerMetricsTools adds metrics analysis tools
func registerMetricsTools(s *Server, callbacks ToolCallbacks) {
	// get_metrics_by_model - Get aggregated metrics per model for cost comparison
	s.RegisterTool(ToolDefinition{
		Name:        "get_metrics_by_model",
		Description: "Get aggregated token usage and cost metrics grouped by model. Useful for comparing costs across different models (e.g., haiku vs sonnet vs opus). Returns report count, total tokens, total cost, and average tokens per report for each model.",
		Parameters: map[string]ParameterDef{
			"model": {
				Type:        "string",
				Description: "Optional model filter (e.g., 'claude-3-5-sonnet-20241022'). If not provided, returns metrics for all models.",
				Required:    false,
			},
		},
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			if callbacks.OnGetMetricsByModel == nil {
				return map[string]interface{}{"error": "Metrics analysis not configured"}, nil
			}
			modelFilter, _ := params["model"].(string)
			return callbacks.OnGetMetricsByModel(modelFilter)
		},
	})

	// Register SGT workflow tools
	registerSGTWorkflowTools(s, callbacks)
}

// registerSGTWorkflowTools adds task dispatch and review tools for SGT workflow
func registerSGTWorkflowTools(s *Server, callbacks ToolCallbacks) {
	// dispatch_task - Captain assigns task to SGT
	s.RegisterTool(ToolDefinition{
		Name:        "dispatch_task",
		Description: "Dispatch a task to an SGT agent for implementation or review. Only Captain should use this.",
		Parameters: map[string]ParameterDef{
			"task_id":         {Type: "string", Description: "The task ID to dispatch", Required: true},
			"assign_to":       {Type: "string", Description: "Agent ID to assign to (e.g., 'SGT-Green', 'SGT-Purple')", Required: true},
			"assignment_type": {Type: "string", Description: "Type: 'implementation', 'review', or 'rework'", Required: true},
			"branch_name":     {Type: "string", Description: "Git branch name for this work", Required: false},
		},
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			if callbacks.OnDispatchTask == nil {
				return map[string]interface{}{"error": "Task dispatch not configured"}, nil
			}
			taskID, _ := params["task_id"].(string)
			assignTo, _ := params["assign_to"].(string)
			assignmentType, _ := params["assignment_type"].(string)
			branchName, _ := params["branch_name"].(string)
			return callbacks.OnDispatchTask(taskID, assignTo, assignmentType, branchName)
		},
	})

	// accept_assignment - SGT accepts dispatched work
	s.RegisterTool(ToolDefinition{
		Name:        "accept_assignment",
		Description: "Accept a task assignment and begin work. SGTs use this to acknowledge receipt.",
		Parameters: map[string]ParameterDef{
			"assignment_id": {Type: "number", Description: "The assignment ID to accept", Required: true},
		},
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			if callbacks.OnAcceptAssignment == nil {
				return map[string]interface{}{"error": "Assignment acceptance not configured"}, nil
			}
			assignmentID := int64(params["assignment_id"].(float64))
			return callbacks.OnAcceptAssignment(agentID, assignmentID)
		},
	})

	// get_my_assignment - SGT checks for pending assignments
	s.RegisterTool(ToolDefinition{
		Name:        "get_my_assignment",
		Description: "Get your current active assignment. SGTs use this to check for work.",
		Parameters:  map[string]ParameterDef{},
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			if callbacks.OnGetMyAssignment == nil {
				return map[string]interface{}{"error": "Assignment retrieval not configured"}, nil
			}
			return callbacks.OnGetMyAssignment(agentID)
		},
	})

	// log_worker - SGT logs sub-agent work
	s.RegisterTool(ToolDefinition{
		Name:        "log_worker",
		Description: "Log a sub-agent task for tracking. SGTs use this when spawning haiku/sonnet workers.",
		Parameters: map[string]ParameterDef{
			"assignment_id": {Type: "number", Description: "The parent assignment ID", Required: true},
			"worker_type":   {Type: "string", Description: "Type: 'haiku', 'sonnet', or 'subagent'", Required: true},
			"description":   {Type: "string", Description: "What this worker is doing", Required: true},
		},
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			if callbacks.OnLogWorker == nil {
				return map[string]interface{}{"error": "Worker logging not configured"}, nil
			}
			assignmentID := int64(params["assignment_id"].(float64))
			workerType, _ := params["worker_type"].(string)
			description, _ := params["description"].(string)
			return callbacks.OnLogWorker(agentID, assignmentID, workerType, description)
		},
	})

	// submit_for_review - SGT Green submits completed work
	s.RegisterTool(ToolDefinition{
		Name:        "submit_for_review",
		Description: "Submit completed implementation for review. SGT Green uses this when code is ready.",
		Parameters: map[string]ParameterDef{
			"assignment_id": {Type: "number", Description: "The assignment ID", Required: true},
			"branch_name":   {Type: "string", Description: "Git branch with the changes", Required: true},
		},
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			if callbacks.OnSubmitForReview == nil {
				return map[string]interface{}{"error": "Review submission not configured"}, nil
			}
			assignmentID := int64(params["assignment_id"].(float64))
			branchName, _ := params["branch_name"].(string)
			return callbacks.OnSubmitForReview(agentID, assignmentID, branchName)
		},
	})

	// submit_review_result - SGT Purple submits review verdict
	s.RegisterTool(ToolDefinition{
		Name:        "submit_review_result",
		Description: "Submit review verdict. SGT Purple uses this after reviewing code.",
		Parameters: map[string]ParameterDef{
			"assignment_id": {Type: "number", Description: "The assignment ID being reviewed", Required: true},
			"approved":      {Type: "boolean", Description: "Whether the code passes review", Required: true},
			"feedback":      {Type: "string", Description: "Review feedback (required if not approved)", Required: false},
		},
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			if callbacks.OnSubmitReviewResult == nil {
				return map[string]interface{}{"error": "Review result submission not configured"}, nil
			}
			assignmentID := int64(params["assignment_id"].(float64))
			approved, _ := params["approved"].(bool)
			feedback, _ := params["feedback"].(string)
			return callbacks.OnSubmitReviewResult(agentID, assignmentID, approved, feedback)
		},
	})

	// complete_worker - SGT marks a sub-agent task as complete with metrics
	s.RegisterTool(ToolDefinition{
		Name:        "complete_worker",
		Description: "Mark a sub-agent worker task as complete. Use this to track sub-agent token usage when they finish.",
		Parameters: map[string]ParameterDef{
			"worker_id":   {Type: "number", Description: "The worker ID from log_worker", Required: true},
			"status":      {Type: "string", Description: "Result status: 'completed' or 'failed'", Required: true},
			"result":      {Type: "string", Description: "Summary of what the worker accomplished", Required: false},
			"model":       {Type: "string", Description: "Model used (e.g., 'claude-3-5-haiku-20241022')", Required: true},
			"tokens_used": {Type: "number", Description: "Estimated tokens used by this worker", Required: true},
		},
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			if callbacks.OnCompleteWorker == nil {
				return map[string]interface{}{"error": "Worker completion not configured"}, nil
			}
			workerID := int64(params["worker_id"].(float64))
			status, _ := params["status"].(string)
			result, _ := params["result"].(string)
			model, _ := params["model"].(string)
			tokensUsed := int64(params["tokens_used"].(float64))
			return callbacks.OnCompleteWorker(agentID, workerID, status, result, model, tokensUsed)
		},
	})

	// get_metrics_by_agent_type - Get breakdown by captain/sgt/spawned/subagent
	s.RegisterTool(ToolDefinition{
		Name:        "get_metrics_by_agent_type",
		Description: "Get aggregated metrics by agent type (captain, sgt, spawned_window, subagent). Useful for understanding where costs are going.",
		Parameters:  map[string]ParameterDef{},
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			if callbacks.OnGetMetricsByAgentType == nil {
				return map[string]interface{}{"error": "Agent type metrics not configured"}, nil
			}
			return callbacks.OnGetMetricsByAgentType()
		},
	})

	// get_metrics_by_agent - Get per-agent metrics breakdown
	s.RegisterTool(ToolDefinition{
		Name:        "get_metrics_by_agent",
		Description: "Get metrics for each individual agent. Shows tokens, cost, and parent relationship for sub-agents.",
		Parameters:  map[string]ParameterDef{},
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			if callbacks.OnGetMetricsByAgent == nil {
				return map[string]interface{}{"error": "Agent metrics not configured"}, nil
			}
			return callbacks.OnGetMetricsByAgent()
		},
	})

	// Register Review Board tools
	registerReviewBoardTools(s, callbacks)

	// Register Document storage tools
	registerDocumentTools(s, callbacks)
}

// RegisterWaitForEventsTool registers the wait_for_events tool for real-time event polling
func RegisterWaitForEventsTool(s *Server, bus *events.Bus) {
	s.RegisterTool(ToolDefinition{
		Name:        "wait_for_events",
		Description: "Wait for events to be published to this agent. Blocks until an event arrives or timeout occurs. Use this for real-time notifications.",
		Parameters: map[string]ParameterDef{
			"timeout_seconds": {
				Type:        "number",
				Description: "Timeout in seconds (default: 60, min: 1, max: 300)",
				Required:    false,
			},
			"event_types": {
				Type:        "array",
				Description: "Optional array of event types to filter (e.g., ['message', 'alert', 'task']). If not provided, all event types are received.",
				Required:    false,
			},
		},
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			// Parse timeout with clamping
			timeout := 60.0 // Default 60 seconds
			if t, ok := params["timeout_seconds"].(float64); ok {
				timeout = t
			}

			// Clamp timeout between 1 and 300 seconds
			if timeout < 1 {
				timeout = 1
			}
			if timeout > 300 {
				timeout = 300
			}

			timeoutDuration := time.Duration(timeout) * time.Second

			// Parse event types filter
			var eventTypes []events.EventType
			if typesRaw, ok := params["event_types"].([]interface{}); ok {
				for _, t := range typesRaw {
					if typeStr, ok := t.(string); ok {
						eventTypes = append(eventTypes, events.EventType(typeStr))
					}
				}
			}

			// Check for pending events first (replay from store)
			if pending, err := bus.GetPendingEvents(agentID, eventTypes); err == nil && len(pending) > 0 {
				// Return the first pending event
				firstEvent := pending[0]
				// Mark as delivered so it won't be returned again
				bus.MarkDelivered(firstEvent.ID)
				return map[string]interface{}{
					"status":        "event_received",
					"event":         eventToMap(firstEvent),
					"pending_count": len(pending) - 1,
				}, nil
			}

			// Subscribe to bus for real-time events
			ch := bus.Subscribe(agentID, eventTypes)
			defer bus.Unsubscribe(agentID, ch)

			// Wait for event or timeout
			select {
			case event := <-ch:
				// Event received
				return map[string]interface{}{
					"status":        "event_received",
					"event":         eventToMap(&event),
					"pending_count": 0,
				}, nil

			case <-time.After(timeoutDuration):
				// Timeout
				return map[string]interface{}{
					"status":  "timeout",
					"message": "No events received within timeout period",
				}, nil
			}
		},
	})
}

// RegisterSendToAgentTool registers the send_to_agent tool for Captain-to-SGT messaging
func RegisterSendToAgentTool(s *Server, bus *events.Bus) {
	s.RegisterTool(ToolDefinition{
		Name:        "send_to_agent",
		Description: "Send a message or task assignment to a specific agent. The target agent will receive this via wait_for_events. Use this to assign new work to persistent agents without spawning new windows.",
		Parameters: map[string]ParameterDef{
			"target_agent": {
				Type:        "string",
				Description: "The agent ID to send the message to (e.g., 'team-sgtgreen001')",
				Required:    true,
			},
			"message_type": {
				Type:        "string",
				Description: "Type of message: 'new_task', 'instruction', 'stop', 'ping'",
				Required:    true,
			},
			"task_id": {
				Type:        "string",
				Description: "Task ID if assigning a new task",
				Required:    false,
			},
			"assignment_id": {
				Type:        "number",
				Description: "Assignment ID if this is a dispatched task",
				Required:    false,
			},
			"content": {
				Type:        "string",
				Description: "Message content or task description",
				Required:    true,
			},
			"branch_name": {
				Type:        "string",
				Description: "Git branch name for task work",
				Required:    false,
			},
		},
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			targetAgent, _ := params["target_agent"].(string)
			messageType, _ := params["message_type"].(string)
			content, _ := params["content"].(string)
			taskID, _ := params["task_id"].(string)
			branchName, _ := params["branch_name"].(string)

			if targetAgent == "" {
				return map[string]interface{}{"error": "target_agent is required"}, nil
			}
			if messageType == "" {
				return map[string]interface{}{"error": "message_type is required"}, nil
			}
			if content == "" {
				return map[string]interface{}{"error": "content is required"}, nil
			}

			payload := map[string]interface{}{
				"message_type": messageType,
				"content":      content,
			}
			if taskID != "" {
				payload["task_id"] = taskID
			}
			if branchName != "" {
				payload["branch_name"] = branchName
			}
			if assignmentID, ok := params["assignment_id"].(float64); ok {
				payload["assignment_id"] = int(assignmentID)
			}

			event := &events.Event{
				Type:      events.EventType("agent_message"),
				Source:    agentID,
				Target:    targetAgent,
				Priority:  events.PriorityHigh,
				Payload:   payload,
				CreatedAt: time.Now(),
			}
			bus.Publish(event)

			return map[string]interface{}{
				"status":       "sent",
				"target_agent": targetAgent,
				"message_type": messageType,
				"event_id":     event.ID,
			}, nil
		},
	})
}

// registerReviewBoardTools adds Review Board tools for multi-reviewer Fagan-style inspection
func registerReviewBoardTools(s *Server, callbacks ToolCallbacks) {
	// create_review_board - Purple SGT creates a review board for multi-reviewer inspection
	s.RegisterTool(ToolDefinition{
		Name:        "create_review_board",
		Description: "Create a review board for multi-reviewer Fagan-style inspection. Purple SGT uses this internally.",
		Parameters: map[string]ParameterDef{
			"assignment_id":  {Type: "number", Description: "Assignment to review", Required: true},
			"reviewer_count": {Type: "number", Description: "Number of reviewers (1-5)", Required: true},
			"complexity":     {Type: "number", Description: "Complexity score 0-100", Required: false},
			"risk_level":     {Type: "string", Description: "low, medium, high, critical", Required: false},
		},
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			if callbacks.OnCreateReviewBoard == nil {
				return map[string]interface{}{"error": "Review Board not configured"}, nil
			}
			assignmentID := int64(params["assignment_id"].(float64))
			reviewerCount := int(params["reviewer_count"].(float64))
			complexity := 50 // Default
			if c, ok := params["complexity"].(float64); ok {
				complexity = int(c)
			}
			riskLevel := "medium" // Default
			if r, ok := params["risk_level"].(string); ok {
				riskLevel = r
			}
			return callbacks.OnCreateReviewBoard(assignmentID, reviewerCount, complexity, riskLevel)
		},
	})

	// submit_defect - Record a defect finding during code review
	s.RegisterTool(ToolDefinition{
		Name:        "submit_defect",
		Description: "Record a defect finding during code review. Categories: LOGIC, DATA, INTERFACE, DOCS, SYNTAX, STANDARDS (Fagan) or SECURITY, PERFORMANCE, TESTING, ARCHITECTURE, STYLE (Modern).",
		Parameters: map[string]ParameterDef{
			"board_id":       {Type: "number", Description: "Review board ID", Required: true},
			"category":       {Type: "string", Description: "Defect category", Required: true},
			"severity":       {Type: "string", Description: "critical, high, medium, low, info", Required: true},
			"title":          {Type: "string", Description: "Brief title of the defect", Required: true},
			"description":    {Type: "string", Description: "Detailed description", Required: true},
			"file_path":      {Type: "string", Description: "File path where defect was found", Required: false},
			"line_start":     {Type: "number", Description: "Starting line number", Required: false},
			"line_end":       {Type: "number", Description: "Ending line number", Required: false},
			"suggested_fix":  {Type: "string", Description: "Suggested fix or remediation", Required: false},
		},
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			if callbacks.OnSubmitDefect == nil {
				return map[string]interface{}{"error": "Review Board not configured"}, nil
			}
			boardID := int64(params["board_id"].(float64))
			defect := map[string]interface{}{
				"category":    params["category"],
				"severity":    params["severity"],
				"title":       params["title"],
				"description": params["description"],
			}
			if filePath, ok := params["file_path"].(string); ok {
				defect["file_path"] = filePath
			}
			if lineStart, ok := params["line_start"].(float64); ok {
				defect["line_start"] = int(lineStart)
			}
			if lineEnd, ok := params["line_end"].(float64); ok {
				defect["line_end"] = int(lineEnd)
			}
			if suggestedFix, ok := params["suggested_fix"].(string); ok {
				defect["suggested_fix"] = suggestedFix
			}
			return callbacks.OnSubmitDefect(agentID, boardID, defect)
		},
	})

	// record_reviewer_vote - Record a sub-agent reviewer's verdict
	s.RegisterTool(ToolDefinition{
		Name:        "record_reviewer_vote",
		Description: "Record a sub-agent reviewer's verdict on the code.",
		Parameters: map[string]ParameterDef{
			"board_id":      {Type: "number", Description: "Review board ID", Required: true},
			"reviewer_id":   {Type: "string", Description: "Reviewer identifier", Required: true},
			"approved":      {Type: "boolean", Description: "Whether the code passes review", Required: true},
			"confidence":    {Type: "number", Description: "Confidence level 0-100", Required: false},
			"defects_found": {Type: "number", Description: "Number of defects found", Required: false},
			"tokens_used":   {Type: "number", Description: "Tokens used by this reviewer", Required: false},
		},
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			if callbacks.OnRecordReviewerVote == nil {
				return map[string]interface{}{"error": "Review Board not configured"}, nil
			}
			boardID := int64(params["board_id"].(float64))
			reviewerID, _ := params["reviewer_id"].(string)
			approved, _ := params["approved"].(bool)
			confidence := 0
			if c, ok := params["confidence"].(float64); ok {
				confidence = int(c)
			}
			defectsFound := 0
			if d, ok := params["defects_found"].(float64); ok {
				defectsFound = int(d)
			}
			tokensUsed := int64(0)
			if t, ok := params["tokens_used"].(float64); ok {
				tokensUsed = int64(t)
			}
			return callbacks.OnRecordReviewerVote(boardID, reviewerID, approved, confidence, defectsFound, tokensUsed)
		},
	})

	// finalize_board - Finalize review board after all reviewers done
	s.RegisterTool(ToolDefinition{
		Name:        "finalize_board",
		Description: "Finalize review board after all reviewers done. Applies consensus rules and updates quality scores.",
		Parameters: map[string]ParameterDef{
			"board_id": {Type: "number", Description: "Review board ID to finalize", Required: true},
		},
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			if callbacks.OnFinalizeBoard == nil {
				return map[string]interface{}{"error": "Review Board not configured"}, nil
			}
			boardID := int64(params["board_id"].(float64))
			return callbacks.OnFinalizeBoard(boardID)
		},
	})

	// get_agent_leaderboard - Get quality scores leaderboard
	s.RegisterTool(ToolDefinition{
		Name:        "get_agent_leaderboard",
		Description: "Get agent quality scores leaderboard showing authors and reviewers ranked by quality.",
		Parameters: map[string]ParameterDef{
			"role":  {Type: "string", Description: "Filter: author, reviewer, or empty for all", Required: false},
			"limit": {Type: "number", Description: "Max results, default 20", Required: false},
		},
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			if callbacks.OnGetAgentLeaderboard == nil {
				return map[string]interface{}{"error": "Review Board not configured"}, nil
			}
			role, _ := params["role"].(string)
			limit := 20
			if l, ok := params["limit"].(float64); ok {
				limit = int(l)
			}
			return callbacks.OnGetAgentLeaderboard(role, limit)
		},
	})

	// get_defect_categories - Get list of valid defect categories
	s.RegisterTool(ToolDefinition{
		Name:        "get_defect_categories",
		Description: "Get list of valid defect categories with descriptions.",
		Parameters:  map[string]ParameterDef{},
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			if callbacks.OnGetDefectCategories == nil {
				return map[string]interface{}{"error": "Review Board not configured"}, nil
			}
			return callbacks.OnGetDefectCategories()
		},
	})
}

// registerDocumentTools adds document storage tools for agents to save work products
func registerDocumentTools(s *Server, callbacks ToolCallbacks) {
	// save_document - Save a document to the database
	s.RegisterTool(ToolDefinition{
		Name:        "save_document",
		Description: "Save a document (plan, report, review, etc.) to the database for future reference. Use this to preserve your work products.",
		Parameters: map[string]ParameterDef{
			"doc_type": {
				Type:        "string",
				Description: "Document type: plan, report, review, test_report, agent_work, config",
				Required:    true,
			},
			"title": {
				Type:        "string",
				Description: "Document title",
				Required:    true,
			},
			"content": {
				Type:        "string",
				Description: "Document content",
				Required:    true,
			},
			"format": {
				Type:        "string",
				Description: "Content format: markdown, json, yaml, text (default: markdown)",
				Required:    false,
			},
			"project_id": {
				Type:        "string",
				Description: "Optional project ID",
				Required:    false,
			},
			"task_id": {
				Type:        "string",
				Description: "Optional task ID",
				Required:    false,
			},
			"tags": {
				Type:        "array",
				Description: "Optional tags for filtering",
				Required:    false,
			},
		},
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			if callbacks.OnSaveDocument == nil {
				return map[string]interface{}{"error": "Document storage not configured"}, nil
			}
			doc := map[string]interface{}{
				"doc_type": params["doc_type"],
				"title":    params["title"],
				"content":  params["content"],
				"format":   "markdown", // default
			}
			if format, ok := params["format"].(string); ok && format != "" {
				doc["format"] = format
			}
			if projectID, ok := params["project_id"].(string); ok {
				doc["project_id"] = projectID
			}
			if taskID, ok := params["task_id"].(string); ok {
				doc["task_id"] = taskID
			}
			if tags, ok := params["tags"].([]interface{}); ok {
				doc["tags"] = tags
			}
			return callbacks.OnSaveDocument(agentID, doc)
		},
	})

	// get_document - Get a document by ID
	s.RegisterTool(ToolDefinition{
		Name:        "get_document",
		Description: "Get a document by its ID. Returns the full document with metadata.",
		Parameters: map[string]ParameterDef{
			"id": {
				Type:        "number",
				Description: "Document ID",
				Required:    true,
			},
		},
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			if callbacks.OnGetDocument == nil {
				return map[string]interface{}{"error": "Document storage not configured"}, nil
			}
			id := int64(params["id"].(float64))
			return callbacks.OnGetDocument(id)
		},
	})

	// search_documents - Search documents by query or filters
	s.RegisterTool(ToolDefinition{
		Name:        "search_documents",
		Description: "Search documents by query text or filters. Searches in titles and content. Returns matching documents with metadata.",
		Parameters: map[string]ParameterDef{
			"query": {
				Type:        "string",
				Description: "Search query (searches title and content)",
				Required:    false,
			},
			"doc_type": {
				Type:        "string",
				Description: "Filter by document type: plan, report, review, test_report, agent_work, config",
				Required:    false,
			},
			"project_id": {
				Type:        "string",
				Description: "Filter by project ID",
				Required:    false,
			},
			"author_id": {
				Type:        "string",
				Description: "Filter by author (agent ID)",
				Required:    false,
			},
			"limit": {
				Type:        "number",
				Description: "Maximum results to return (default: 20)",
				Required:    false,
			},
		},
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			if callbacks.OnSearchDocuments == nil {
				return map[string]interface{}{"error": "Document storage not configured"}, nil
			}
			query, _ := params["query"].(string)
			docType, _ := params["doc_type"].(string)
			projectID, _ := params["project_id"].(string)
			authorID, _ := params["author_id"].(string)
			limit := 20
			if l, ok := params["limit"].(float64); ok {
				limit = int(l)
			}
			return callbacks.OnSearchDocuments(query, docType, projectID, authorID, limit)
		},
	})

	// list_my_documents - List documents created by the calling agent
	s.RegisterTool(ToolDefinition{
		Name:        "list_my_documents",
		Description: "List documents you created. Useful for seeing your own work history.",
		Parameters: map[string]ParameterDef{
			"doc_type": {
				Type:        "string",
				Description: "Optional filter by document type: plan, report, review, test_report, agent_work, config",
				Required:    false,
			},
			"limit": {
				Type:        "number",
				Description: "Maximum results to return (default: 20)",
				Required:    false,
			},
		},
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			if callbacks.OnListMyDocuments == nil {
				return map[string]interface{}{"error": "Document storage not configured"}, nil
			}
			docType, _ := params["doc_type"].(string)
			limit := 20
			if l, ok := params["limit"].(float64); ok {
				limit = int(l)
			}
			return callbacks.OnListMyDocuments(agentID, docType, limit)
		},
	})
}

// registerWezTermTools adds WezTerm pane control tools for Captain
func registerWezTermTools(s *Server) {
	// wezterm_spawn_pane - Spawn a new pane in WezTerm
	s.RegisterTool(ToolDefinition{
		Name:        "wezterm_spawn_pane",
		Description: "Spawn a new pane in WezTerm by splitting an existing pane. Returns the new pane ID.",
		Parameters: map[string]ParameterDef{
			"direction": {
				Type:        "string",
				Description: "Split direction: right, left, top, or bottom",
				Required:    true,
			},
			"command": {
				Type:        "string",
				Description: "Command to run in the new pane (e.g., 'cmd.exe', 'powershell.exe')",
				Required:    true,
			},
			"pane_id": {
				Type:        "string",
				Description: "Optional: target pane to split (if not provided, splits the active pane)",
				Required:    false,
			},
			"cwd": {
				Type:        "string",
				Description: "Optional: working directory for the new pane",
				Required:    false,
			},
		},
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			direction, _ := params["direction"].(string)
			command, _ := params["command"].(string)
			paneID, _ := params["pane_id"].(string)
			cwd, _ := params["cwd"].(string)

			if direction == "" || command == "" {
				return map[string]interface{}{"error": "direction and command are required"}, nil
			}

			// Validate direction
			validDirections := map[string]bool{"right": true, "left": true, "top": true, "bottom": true}
			if !validDirections[direction] {
				return map[string]interface{}{"error": "invalid direction, must be: right, left, top, or bottom"}, nil
			}

			// Build wezterm cli command
			args := []string{"cli", "split-pane", "--" + direction}
			if paneID != "" {
				args = append(args, "--pane-id", paneID)
			}
			if cwd != "" {
				args = append(args, "--cwd", cwd)
			}
			args = append(args, "--", command)

			cmd := exec.Command("wezterm.exe", args...)
			output, err := cmd.CombinedOutput()
			if err != nil {
				return map[string]interface{}{
					"success": false,
					"error":   fmt.Sprintf("failed to spawn pane: %v, output: %s", err, string(output)),
				}, nil
			}

			newPaneID := strings.TrimSpace(string(output))
			return map[string]interface{}{
				"success":     true,
				"new_pane_id": newPaneID,
			}, nil
		},
	})

	// wezterm_list_panes - List all panes in WezTerm
	// Uses centralized WezTerm ops for thread safety
	s.RegisterTool(ToolDefinition{
		Name:        "wezterm_list_panes",
		Description: "List all panes in WezTerm with their IDs, titles, and working directories.",
		Parameters:  map[string]ParameterDef{},
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			panes, err := wezterm.Get().ListPanes()
			if err != nil {
				return map[string]interface{}{
					"success": false,
					"error":   err.Error(),
				}, nil
			}

			return map[string]interface{}{
				"success": true,
				"panes":   panes,
				"count":   len(panes),
			}, nil
		},
	})

	// wezterm_send_text - Send text/command to a specific pane
	// Uses centralized WezTerm ops for thread safety
	s.RegisterTool(ToolDefinition{
		Name:        "wezterm_send_text",
		Description: "Send text or a command to a specific pane. Useful for programmatically controlling terminals.",
		Parameters: map[string]ParameterDef{
			"pane_id": {
				Type:        "string",
				Description: "Target pane ID",
				Required:    true,
			},
			"text": {
				Type:        "string",
				Description: "Text or command to send to the pane",
				Required:    true,
			},
			"execute": {
				Type:        "boolean",
				Description: "If true, append Enter key (CR+LF) to execute the command. Default: false",
				Required:    false,
			},
		},
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			paneIDStr, _ := params["pane_id"].(string)
			text, _ := params["text"].(string)
			execute, _ := params["execute"].(bool)

			if paneIDStr == "" || text == "" {
				return map[string]interface{}{"error": "pane_id and text are required"}, nil
			}

			paneID, err := strconv.Atoi(paneIDStr)
			if err != nil {
				return map[string]interface{}{
					"success": false,
					"error":   fmt.Sprintf("invalid pane_id: %s", paneIDStr),
				}, nil
			}

			// Use centralized WezTerm ops
			if err := wezterm.Get().SendText(paneID, text, execute); err != nil {
				return map[string]interface{}{
					"success": false,
					"error":   err.Error(),
				}, nil
			}

			return map[string]interface{}{
				"success":  true,
				"executed": execute,
			}, nil
		},
	})

	// wezterm_close_pane - Close a specific pane
	// Uses graceful shutdown: sends Ctrl+C, then exit, then kills pane
	s.RegisterTool(ToolDefinition{
		Name:        "wezterm_close_pane",
		Description: "Close a specific pane by its ID. Uses graceful shutdown (sends exit signal first) to prevent WezTerm freezes on Windows.",
		Parameters: map[string]ParameterDef{
			"pane_id": {
				Type:        "string",
				Description: "Pane ID to close",
				Required:    true,
			},
		},
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			paneIDStr, _ := params["pane_id"].(string)

			if paneIDStr == "" {
				return map[string]interface{}{"error": "pane_id is required"}, nil
			}

			paneID, err := strconv.Atoi(paneIDStr)
			if err != nil {
				return map[string]interface{}{
					"success": false,
					"error":   fmt.Sprintf("invalid pane_id: %s", paneIDStr),
				}, nil
			}

			// Use graceful shutdown: Ctrl+C -> exit -> kill
			// This prevents Windows conpty hangs
			if err := wezterm.Get().GracefulKillPane(paneID); err != nil {
				return map[string]interface{}{
					"success": false,
					"error":   err.Error(),
				}, nil
			}

			return map[string]interface{}{
				"success": true,
			}, nil
		},
	})

	// wezterm_close_panes - Close multiple panes with graceful shutdown
	// CRITICAL: Use this instead of calling wezterm cli via Bash to avoid freezing WezTerm
	s.RegisterTool(ToolDefinition{
		Name:        "wezterm_close_panes",
		Description: "Close multiple panes by their IDs with graceful shutdown (sends exit signals first, 500ms+ delay between each). ALWAYS use this instead of calling 'wezterm cli kill-pane' via Bash to prevent WezTerm from freezing.",
		Parameters: map[string]ParameterDef{
			"pane_ids": {
				Type:        "array",
				Description: "Array of pane IDs to close (e.g., [2, 3, 4])",
				Required:    true,
			},
		},
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			paneIDsRaw, ok := params["pane_ids"].([]interface{})
			if !ok || len(paneIDsRaw) == 0 {
				return map[string]interface{}{"error": "pane_ids array is required"}, nil
			}

			// Parse pane IDs
			var paneIDs []int
			for _, idRaw := range paneIDsRaw {
				var paneID int
				switch v := idRaw.(type) {
				case float64:
					paneID = int(v)
				case string:
					var err error
					paneID, err = strconv.Atoi(v)
					if err != nil {
						return map[string]interface{}{
							"success": false,
							"error":   fmt.Sprintf("invalid pane_id: %v", idRaw),
						}, nil
					}
				default:
					return map[string]interface{}{
						"success": false,
						"error":   fmt.Sprintf("invalid pane_id type: %T", idRaw),
					}, nil
				}
				paneIDs = append(paneIDs, paneID)
			}

			// Use graceful shutdown: Ctrl+C -> exit -> kill for each pane
			// This prevents Windows conpty hangs
			errors := wezterm.Get().GracefulKillPanes(paneIDs)

			// Build result with per-pane status
			results := make([]map[string]interface{}, len(paneIDs))
			successCount := 0
			for i, paneID := range paneIDs {
				if i < len(errors) && errors[i] != nil {
					results[i] = map[string]interface{}{
						"pane_id": paneID,
						"success": false,
						"error":   errors[i].Error(),
					}
				} else {
					results[i] = map[string]interface{}{
						"pane_id": paneID,
						"success": true,
					}
					successCount++
				}
			}

			return map[string]interface{}{
				"success":       successCount == len(paneIDs),
				"total":         len(paneIDs),
				"closed":        successCount,
				"failed":        len(paneIDs) - successCount,
				"results":       results,
			}, nil
		},
	})

	// wezterm_focus_pane - Focus/activate a specific pane
	// Uses centralized WezTerm ops for thread safety
	s.RegisterTool(ToolDefinition{
		Name:        "wezterm_focus_pane",
		Description: "Focus or activate a specific pane by its ID, bringing it to the foreground.",
		Parameters: map[string]ParameterDef{
			"pane_id": {
				Type:        "string",
				Description: "Pane ID to focus",
				Required:    true,
			},
		},
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			paneIDStr, _ := params["pane_id"].(string)

			if paneIDStr == "" {
				return map[string]interface{}{"error": "pane_id is required"}, nil
			}

			paneID, err := strconv.Atoi(paneIDStr)
			if err != nil {
				return map[string]interface{}{
					"success": false,
					"error":   fmt.Sprintf("invalid pane_id: %s", paneIDStr),
				}, nil
			}

			if err := wezterm.Get().FocusPane(paneID); err != nil {
				return map[string]interface{}{
					"success": false,
					"error":   err.Error(),
				}, nil
			}

			return map[string]interface{}{
				"success": true,
			}, nil
		},
	})

	// wezterm_get_text - Read text content from a pane
	// Uses centralized WezTerm ops for thread safety
	s.RegisterTool(ToolDefinition{
		Name:        "wezterm_get_text",
		Description: "Read the text content of a WezTerm pane. Useful for seeing what's displayed in agent terminals.",
		Parameters: map[string]ParameterDef{
			"pane_id": {
				Type:        "string",
				Description: "Pane ID to read from",
				Required:    true,
			},
			"start_line": {
				Type:        "number",
				Description: "Starting line number (0 = first line of screen, negative = scrollback). Default: -50",
				Required:    false,
			},
			"end_line": {
				Type:        "number",
				Description: "Ending line number. Default: bottom of screen",
				Required:    false,
			},
		},
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			paneIDStr, _ := params["pane_id"].(string)

			if paneIDStr == "" {
				return map[string]interface{}{"error": "pane_id is required"}, nil
			}

			paneID, err := strconv.Atoi(paneIDStr)
			if err != nil {
				return map[string]interface{}{
					"success": false,
					"error":   fmt.Sprintf("invalid pane_id: %s", paneIDStr),
				}, nil
			}

			// Parse optional line range
			startLine := -50 // Default
			if sl, ok := params["start_line"].(float64); ok {
				startLine = int(sl)
			}

			endLine := 0 // 0 means bottom of screen
			if el, ok := params["end_line"].(float64); ok {
				endLine = int(el)
			}

			text, err := wezterm.Get().GetPaneText(paneID, startLine, endLine)
			if err != nil {
				return map[string]interface{}{
					"success": false,
					"error":   err.Error(),
				}, nil
			}

			return map[string]interface{}{
				"success": true,
				"text":    text,
				"pane_id": paneIDStr,
			}, nil
		},
	})
}

// eventToMap converts an Event to a map for JSON serialization
func eventToMap(event *events.Event) map[string]interface{} {
	return map[string]interface{}{
		"id":         event.ID,
		"type":       string(event.Type),
		"source":     event.Source,
		"target":     event.Target,
		"priority":   event.Priority,
		"payload":    event.Payload,
		"created_at": event.CreatedAt.Format(time.RFC3339),
	}
}
