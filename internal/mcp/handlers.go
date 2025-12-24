package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/CLIAIMONITOR/internal/events"
	"github.com/CLIAIMONITOR/internal/wezterm"
)

// ToolCallbacks interface for tool handlers to call back into services
// SIMPLIFIED: Only callbacks for tools we actually use
type ToolCallbacks struct {
	// Captain context callbacks (for session persistence)
	OnSaveContext   func(key, value string, priority, maxAgeHours int) (interface{}, error)
	OnGetAllContext func() (interface{}, error)
	OnLogSession    func(sessionID, eventType, summary, details, agentID string) (interface{}, error)

	// Captain messages callbacks (human -> Captain chat)
	OnGetCaptainMessages  func() (interface{}, error)
	OnMarkMessagesRead    func(ids []string) (interface{}, error)
	OnSendCaptainResponse func(text string) (interface{}, error)
}

// RegisterDefaultTools registers all standard MCP tools
// SIMPLIFIED: Only essential tools for Captain workflow
func RegisterDefaultTools(s *Server, callbacks ToolCallbacks) {
	// Context persistence tools
	registerContextTools(s, callbacks)

	// WezTerm control tools
	registerWezTermTools(s)
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
			timeout := 60.0
			if t, ok := params["timeout_seconds"].(float64); ok {
				timeout = t
			}
			if timeout < 1 {
				timeout = 1
			}
			if timeout > 300 {
				timeout = 300
			}
			timeoutDuration := time.Duration(timeout) * time.Second

			var eventTypes []events.EventType
			if typesRaw, ok := params["event_types"].([]interface{}); ok {
				for _, t := range typesRaw {
					if typeStr, ok := t.(string); ok {
						eventTypes = append(eventTypes, events.EventType(typeStr))
					}
				}
			}

			// Check for pending events first
			if pending, err := bus.GetPendingEvents(agentID, eventTypes); err == nil && len(pending) > 0 {
				firstEvent := pending[0]
				bus.MarkDelivered(firstEvent.ID)
				return map[string]interface{}{
					"status":        "event_received",
					"event":         eventToMap(firstEvent),
					"pending_count": len(pending) - 1,
				}, nil
			}

			// Subscribe and wait
			ch := bus.Subscribe(agentID, eventTypes)
			defer bus.Unsubscribe(agentID, ch)

			select {
			case event := <-ch:
				return map[string]interface{}{
					"status":        "event_received",
					"event":         eventToMap(&event),
					"pending_count": 0,
				}, nil
			case <-time.After(timeoutDuration):
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
			"target_agent": {Type: "string", Description: "The agent ID to send the message to (e.g., 'team-sgtgreen001')", Required: true},
			"message_type": {Type: "string", Description: "Type of message: 'new_task', 'instruction', 'stop', 'ping'", Required: true},
			"task_id":      {Type: "string", Description: "Task ID if assigning a new task", Required: false},
			"assignment_id": {Type: "number", Description: "Assignment ID if this is a dispatched task", Required: false},
			"content":      {Type: "string", Description: "Message content or task description", Required: true},
			"branch_name":  {Type: "string", Description: "Git branch name for task work", Required: false},
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

// registerWezTermTools adds WezTerm pane control tools for Captain
func registerWezTermTools(s *Server) {
	// wezterm_list_panes - List all panes in WezTerm
	s.RegisterTool(ToolDefinition{
		Name:        "wezterm_list_panes",
		Description: "List all panes in WezTerm with their IDs, titles, and working directories.",
		Parameters:  map[string]ParameterDef{},
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			panes, err := wezterm.Get().ListPanes()
			if err != nil {
				return map[string]interface{}{"success": false, "error": err.Error()}, nil
			}
			return map[string]interface{}{"success": true, "panes": panes, "count": len(panes)}, nil
		},
	})

	// wezterm_send_text - Send text/command to a specific pane
	s.RegisterTool(ToolDefinition{
		Name:        "wezterm_send_text",
		Description: "Send text or a command to a specific pane. Useful for programmatically controlling terminals.",
		Parameters: map[string]ParameterDef{
			"pane_id": {Type: "string", Description: "Target pane ID", Required: true},
			"text":    {Type: "string", Description: "Text or command to send to the pane", Required: true},
			"execute": {Type: "boolean", Description: "If true, append Enter key (CR+LF) to execute the command. Default: false", Required: false},
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
				return map[string]interface{}{"success": false, "error": fmt.Sprintf("invalid pane_id: %s", paneIDStr)}, nil
			}

			if err := wezterm.Get().SendText(paneID, text, execute); err != nil {
				return map[string]interface{}{"success": false, "error": err.Error()}, nil
			}

			return map[string]interface{}{"success": true, "executed": execute}, nil
		},
	})

	// wezterm_close_pane - Close a specific pane
	s.RegisterTool(ToolDefinition{
		Name:        "wezterm_close_pane",
		Description: "Close a specific pane by its ID. Uses graceful shutdown (sends exit signal first) to prevent WezTerm freezes on Windows.",
		Parameters: map[string]ParameterDef{
			"pane_id": {Type: "string", Description: "Pane ID to close", Required: true},
		},
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			paneIDStr, _ := params["pane_id"].(string)
			if paneIDStr == "" {
				return map[string]interface{}{"error": "pane_id is required"}, nil
			}

			paneID, err := strconv.Atoi(paneIDStr)
			if err != nil {
				return map[string]interface{}{"success": false, "error": fmt.Sprintf("invalid pane_id: %s", paneIDStr)}, nil
			}

			if err := wezterm.Get().GracefulKillPane(paneID); err != nil {
				return map[string]interface{}{"success": false, "error": err.Error()}, nil
			}

			return map[string]interface{}{"success": true}, nil
		},
	})

	// wezterm_close_panes - Close multiple panes with graceful shutdown
	s.RegisterTool(ToolDefinition{
		Name:        "wezterm_close_panes",
		Description: "Close multiple panes by their IDs with graceful shutdown (sends exit signals first, 500ms+ delay between each). ALWAYS use this instead of calling 'wezterm cli kill-pane' via Bash to prevent WezTerm from freezing.",
		Parameters: map[string]ParameterDef{
			"pane_ids": {Type: "array", Description: "Array of pane IDs to close (e.g., [2, 3, 4])", Required: true},
		},
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			paneIDsRaw, ok := params["pane_ids"].([]interface{})
			if !ok || len(paneIDsRaw) == 0 {
				return map[string]interface{}{"error": "pane_ids array is required"}, nil
			}

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
						return map[string]interface{}{"success": false, "error": fmt.Sprintf("invalid pane_id: %v", idRaw)}, nil
					}
				default:
					return map[string]interface{}{"success": false, "error": fmt.Sprintf("invalid pane_id type: %T", idRaw)}, nil
				}
				paneIDs = append(paneIDs, paneID)
			}

			errors := wezterm.Get().GracefulKillPanes(paneIDs)

			results := make([]map[string]interface{}, len(paneIDs))
			successCount := 0
			for i, paneID := range paneIDs {
				if i < len(errors) && errors[i] != nil {
					results[i] = map[string]interface{}{"pane_id": paneID, "success": false, "error": errors[i].Error()}
				} else {
					results[i] = map[string]interface{}{"pane_id": paneID, "success": true}
					successCount++
				}
			}

			return map[string]interface{}{
				"success": successCount == len(paneIDs),
				"total":   len(paneIDs),
				"closed":  successCount,
				"failed":  len(paneIDs) - successCount,
				"results": results,
			}, nil
		},
	})

	// wezterm_focus_pane - Focus/activate a specific pane
	s.RegisterTool(ToolDefinition{
		Name:        "wezterm_focus_pane",
		Description: "Focus or activate a specific pane by its ID, bringing it to the foreground.",
		Parameters: map[string]ParameterDef{
			"pane_id": {Type: "string", Description: "Pane ID to focus", Required: true},
		},
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			paneIDStr, _ := params["pane_id"].(string)
			if paneIDStr == "" {
				return map[string]interface{}{"error": "pane_id is required"}, nil
			}

			paneID, err := strconv.Atoi(paneIDStr)
			if err != nil {
				return map[string]interface{}{"success": false, "error": fmt.Sprintf("invalid pane_id: %s", paneIDStr)}, nil
			}

			if err := wezterm.Get().FocusPane(paneID); err != nil {
				return map[string]interface{}{"success": false, "error": err.Error()}, nil
			}

			return map[string]interface{}{"success": true}, nil
		},
	})

	// wezterm_get_text - Read text content from a pane
	s.RegisterTool(ToolDefinition{
		Name:        "wezterm_get_text",
		Description: "Read the text content of a WezTerm pane. Useful for seeing what's displayed in agent terminals.",
		Parameters: map[string]ParameterDef{
			"pane_id":    {Type: "string", Description: "Pane ID to read from", Required: true},
			"start_line": {Type: "number", Description: "Starting line number (0 = first line of screen, negative = scrollback). Default: -50", Required: false},
			"end_line":   {Type: "number", Description: "Ending line number. Default: bottom of screen", Required: false},
		},
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			paneIDStr, _ := params["pane_id"].(string)
			if paneIDStr == "" {
				return map[string]interface{}{"error": "pane_id is required"}, nil
			}

			paneID, err := strconv.Atoi(paneIDStr)
			if err != nil {
				return map[string]interface{}{"success": false, "error": fmt.Sprintf("invalid pane_id: %s", paneIDStr)}, nil
			}

			startLine := -50
			if sl, ok := params["start_line"].(float64); ok {
				startLine = int(sl)
			}
			endLine := 0
			if el, ok := params["end_line"].(float64); ok {
				endLine = int(el)
			}

			text, err := wezterm.Get().GetPaneText(paneID, startLine, endLine)
			if err != nil {
				return map[string]interface{}{"success": false, "error": err.Error()}, nil
			}

			return map[string]interface{}{"success": true, "text": text, "pane_id": paneIDStr}, nil
		},
	})

	// spawn_agent - Spawn a new agent via the API (preferred over wezterm_spawn_pane)
	s.RegisterTool(ToolDefinition{
		Name:        "spawn_agent",
		Description: "Spawn a new Claude agent in a WezTerm pane. This is the preferred way to spawn agents as it properly sets up workspaces, window titles, and runs Claude CLI. Use this instead of wezterm_spawn_pane for agent spawning.",
		Parameters: map[string]ParameterDef{
			"config_name":  {Type: "string", Description: "Agent configuration name from teams.yaml (e.g., 'SNTGreen', 'HaikuPurple', 'Snake')", Required: true},
			"project_path": {Type: "string", Description: "Working directory path for the agent", Required: true},
			"task":         {Type: "string", Description: "Initial task/prompt for the agent to work on", Required: true},
		},
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			configName, _ := params["config_name"].(string)
			projectPath, _ := params["project_path"].(string)
			task, _ := params["task"].(string)

			if configName == "" || projectPath == "" || task == "" {
				return map[string]interface{}{"success": false, "error": "config_name, project_path, and task are all required"}, nil
			}

			reqBody := map[string]string{
				"config_name":  configName,
				"project_path": projectPath,
				"task":         task,
			}
			jsonBody, err := json.Marshal(reqBody)
			if err != nil {
				return map[string]interface{}{"success": false, "error": fmt.Sprintf("failed to marshal request: %v", err)}, nil
			}

			resp, err := http.Post("http://localhost:3000/api/agents/spawn", "application/json", bytes.NewBuffer(jsonBody))
			if err != nil {
				return map[string]interface{}{"success": false, "error": fmt.Sprintf("failed to call spawn API: %v", err)}, nil
			}
			defer resp.Body.Close()

			var result map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				return map[string]interface{}{"success": false, "error": fmt.Sprintf("failed to decode response: %v", err)}, nil
			}

			if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
				errMsg, _ := result["error"].(string)
				return map[string]interface{}{"success": false, "error": fmt.Sprintf("spawn API error: %s", errMsg)}, nil
			}

			return map[string]interface{}{
				"success":  true,
				"agent_id": result["agent_id"],
				"pane_id":  result["pane_id"],
				"config":   configName,
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
