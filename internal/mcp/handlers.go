package mcp

import (
	"time"

	"github.com/CLIAIMONITOR/internal/events"
	"github.com/CLIAIMONITOR/internal/types"
	"github.com/google/uuid"
)

// ToolCallbacks interface for tool handlers to call back into services
type ToolCallbacks struct {
	OnRegisterAgent          func(agentID, role string) (interface{}, error)
	OnReportStatus           func(agentID, status, task string) (interface{}, error)
	OnReportMetrics          func(agentID string, metrics *types.AgentMetrics) (interface{}, error)
	OnRequestHumanInput      func(req *types.HumanInputRequest) (interface{}, error)
	OnRequestStopApproval    func(req *types.StopApprovalRequest) (interface{}, error)
	OnGetStopRequestByID     func(id string) *types.StopApprovalRequest
	OnLogActivity            func(activity *types.ActivityLog) (interface{}, error)
	OnGetAgentMetrics        func() (interface{}, error)
	OnGetPendingQuestions    func() (interface{}, error)
	OnGetPendingStopRequests func() (interface{}, error)
	OnRespondStopRequest     func(id string, approved bool, response string) (interface{}, error)
	OnEscalateAlert          func(alert *types.Alert) (interface{}, error)
	OnSubmitJudgment         func(judgment *types.SupervisorJudgment) (interface{}, error)
	OnGetAgentList           func() (interface{}, error)
	OnGetMyTasks             func(agentID, status string) (interface{}, error)
	OnClaimTask              func(agentID, taskID string) (interface{}, error)
	OnUpdateTaskProgress     func(agentID, taskID, status, note string) (interface{}, error)
	OnCompleteTask           func(agentID, taskID, summary string) (interface{}, error)
	OnSubmitReconReport      func(agentID string, report map[string]interface{}) (interface{}, error)
	OnRequestGuidance        func(agentID string, guidance map[string]interface{}) (interface{}, error)
	OnReportProgress         func(agentID string, progress map[string]interface{}) (interface{}, error)
	OnSignalCaptain          func(agentID, signal, context string) (interface{}, error)

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
}

// RegisterDefaultTools registers all standard MCP tools
// This is called during server setup with callbacks to other services
func RegisterDefaultTools(s *Server, callbacks ToolCallbacks) {
	// register_agent - Agent identifies itself
	s.RegisterTool(ToolDefinition{
		Name:        "register_agent",
		Description: "Register this agent with the dashboard",
		Parameters: map[string]ParameterDef{
			"agent_id": {Type: "string", Description: "The agent's unique ID", Required: true},
			"role":     {Type: "string", Description: "The agent's role", Required: true},
		},
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			role, _ := params["role"].(string)
			return callbacks.OnRegisterAgent(agentID, role)
		},
	})

	// report_status - Agent updates its status
	s.RegisterTool(ToolDefinition{
		Name:        "report_status",
		Description: "Report current agent status and activity",
		Parameters: map[string]ParameterDef{
			"status":       {Type: "string", Description: "Status: connected, working, idle, blocked", Required: true},
			"current_task": {Type: "string", Description: "What the agent is currently doing", Required: false},
		},
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			status, _ := params["status"].(string)
			task, _ := params["current_task"].(string)
			return callbacks.OnReportStatus(agentID, status, task)
		},
	})

	// report_metrics - Agent reports its metrics
	s.RegisterTool(ToolDefinition{
		Name:        "report_metrics",
		Description: "Report agent metrics (tokens, tests, etc.)",
		Parameters: map[string]ParameterDef{
			"tokens_used":         {Type: "number", Description: "Total tokens used", Required: false},
			"estimated_cost":      {Type: "number", Description: "Estimated cost in USD", Required: false},
			"failed_tests":        {Type: "number", Description: "Number of failed tests", Required: false},
			"consecutive_rejects": {Type: "number", Description: "Consecutive rejected submissions", Required: false},
		},
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			metrics := &types.AgentMetrics{
				AgentID:     agentID,
				LastUpdated: time.Now(),
			}
			if v, ok := params["tokens_used"].(float64); ok {
				metrics.TokensUsed = int64(v)
			}
			if v, ok := params["estimated_cost"].(float64); ok {
				metrics.EstimatedCost = v
			}
			if v, ok := params["failed_tests"].(float64); ok {
				metrics.FailedTests = int(v)
			}
			if v, ok := params["consecutive_rejects"].(float64); ok {
				metrics.ConsecutiveRejects = int(v)
			}
			return callbacks.OnReportMetrics(agentID, metrics)
		},
	})

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

	// request_stop_approval - Agent requests permission to stop
	// This tool BLOCKS until approval is received, then returns the response
	s.RegisterTool(ToolDefinition{
		Name:        "request_stop_approval",
		Description: "Request approval from supervisor before stopping work. MUST be called before stopping for ANY reason. This will WAIT for approval and return the supervisor's response with any new task assignment.",
		Parameters: map[string]ParameterDef{
			"reason":         {Type: "string", Description: "Why stopping: task_complete, blocked, error, needs_input, other", Required: true},
			"context":        {Type: "string", Description: "Details about why you want to stop", Required: true},
			"work_completed": {Type: "string", Description: "Summary of what you accomplished", Required: true},
		},
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			reason, _ := params["reason"].(string)
			context, _ := params["context"].(string)
			workCompleted, _ := params["work_completed"].(string)

			req := &types.StopApprovalRequest{
				ID:            uuid.New().String(),
				AgentID:       agentID,
				Reason:        reason,
				Context:       context,
				WorkCompleted: workCompleted,
				CreatedAt:     time.Now(),
				Reviewed:      false,
			}

			// Submit the request
			_, err := callbacks.OnRequestStopApproval(req)
			if err != nil {
				return nil, err
			}

			// Poll for approval response (max 10 minutes, check every 5 seconds)
			maxWait := 10 * time.Minute
			pollInterval := 5 * time.Second
			deadline := time.Now().Add(maxWait)

			for time.Now().Before(deadline) {
				// Check if request has been reviewed
				updated := callbacks.OnGetStopRequestByID(req.ID)
				if updated != nil && updated.Reviewed {
					// Return the approval response
					return map[string]interface{}{
						"status":      "reviewed",
						"approved":    updated.Approved,
						"response":    updated.Response,
						"reviewed_by": updated.ReviewedBy,
						"next_task":   updated.Response, // Response typically contains next task if not approved to stop
					}, nil
				}
				time.Sleep(pollInterval)
			}

			// Timeout - return timeout status
			return map[string]interface{}{
				"status":  "timeout",
				"message": "No response received within 10 minutes. You may proceed with caution or try again.",
			}, nil
		},
	})

	// signal_captain - Agent signals Captain for attention
	s.RegisterTool(ToolDefinition{
		Name:        "signal_captain",
		Description: "Signal Captain that you need attention. Use when stopping, blocked, completed, or encountering errors.",
		Parameters: map[string]ParameterDef{
			"signal": {
				Type:        "string",
				Description: "Signal type: stopped, blocked, completed, error, need_guidance",
				Required:    true,
			},
			"context": {
				Type:        "string",
				Description: "Brief explanation of why you're signaling",
				Required:    true,
			},
			"work_completed": {
				Type:        "string",
				Description: "Summary of work completed (for stopped/completed signals)",
				Required:    false,
			},
		},
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			signal, _ := params["signal"].(string)
			context, _ := params["context"].(string)
			workCompleted, _ := params["work_completed"].(string)

			// Validate signal type
			validSignals := map[string]bool{
				"stopped":       true,
				"blocked":       true,
				"completed":     true,
				"error":         true,
				"need_guidance": true,
			}
			if !validSignals[signal] {
				return map[string]interface{}{
					"status":  "error",
					"message": "Invalid signal. Use: stopped, blocked, completed, error, need_guidance",
				}, nil
			}

			// Include work summary in context if provided
			fullContext := context
			if workCompleted != "" {
				fullContext = context + "\n\nWork completed:\n" + workCompleted
			}

			return callbacks.OnSignalCaptain(agentID, signal, fullContext)
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

	// get_pending_stop_requests - Supervisor checks stop approval queue
	s.RegisterTool(ToolDefinition{
		Name:        "get_pending_stop_requests",
		Description: "Get pending stop approval requests from agents (supervisor only)",
		Parameters:  map[string]ParameterDef{},
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			return callbacks.OnGetPendingStopRequests()
		},
	})

	// respond_stop_request - Supervisor approves or denies stop request
	s.RegisterTool(ToolDefinition{
		Name:        "respond_stop_request",
		Description: "Approve or deny an agent's stop request (supervisor only)",
		Parameters: map[string]ParameterDef{
			"request_id": {Type: "string", Description: "The stop request ID", Required: true},
			"approved":   {Type: "boolean", Description: "Whether to approve the stop", Required: true},
			"response":   {Type: "string", Description: "Message to the agent (instructions if denied)", Required: true},
		},
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			requestID, _ := params["request_id"].(string)
			approved, _ := params["approved"].(bool)
			response, _ := params["response"].(string)
			return callbacks.OnRespondStopRequest(requestID, approved, response)
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
