package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"github.com/CLIAIMONITOR/internal/agents"
	"github.com/CLIAIMONITOR/internal/captain"
	"github.com/CLIAIMONITOR/internal/handlers"
	"github.com/CLIAIMONITOR/internal/mcp"
	"github.com/CLIAIMONITOR/internal/memory"
	"github.com/CLIAIMONITOR/internal/metrics"
	natslib "github.com/CLIAIMONITOR/internal/nats"
	"github.com/CLIAIMONITOR/internal/notifications"
	"github.com/CLIAIMONITOR/internal/persistence"
	"github.com/CLIAIMONITOR/internal/router"
	"github.com/CLIAIMONITOR/internal/types"
	"github.com/CLIAIMONITOR/web"
	"github.com/gorilla/mux"
)

// Server is the main HTTP server
type Server struct {
	httpServer *http.Server
	router     *mux.Router
	hub        *Hub

	// Dependencies
	store             *persistence.JSONStore
	spawner           *agents.ProcessSpawner
	mcp               *mcp.Server
	metrics           *metrics.MetricsCollector
	alerts            *metrics.AlertChecker
	config            *types.TeamsConfig
	projectsConfig    *types.ProjectsConfig
	memDB             memory.MemoryDB
	notifications     *notifications.Manager
	captain           *captain.Captain
	captainSupervisor *captain.CaptainSupervisor
	cleanup           *CleanupService
	basePath          string

	// NATS messaging
	natsServer *natslib.EmbeddedServer
	natsClient *natslib.Client
	natsBridge *NATSBridge

	// Instance metadata
	port      int
	startTime time.Time

	// Background tasks
	stopChan chan struct{}

	// Shutdown signaling - external code can listen to this
	ShutdownChan chan struct{}

	// Cleanup service context
	cleanupCtx    context.Context
	cleanupCancel context.CancelFunc
}

// NewServer creates a new server instance
func NewServer(
	store *persistence.JSONStore,
	spawner *agents.ProcessSpawner,
	mcpServer *mcp.Server,
	metricsCollector *metrics.MetricsCollector,
	alertEngine *metrics.AlertChecker,
	config *types.TeamsConfig,
	projectsConfig *types.ProjectsConfig,
	memDB memory.MemoryDB,
	basePath string,
	port int,
) *Server {
	// Initialize notification manager
	notificationMgr := notifications.NewDefaultManager()

	// Build agent configs map
	agentConfigs := make(map[string]types.AgentConfig)
	for _, cfg := range config.Agents {
		agentConfigs[cfg.Name] = cfg
	}

	// Initialize Captain orchestrator
	cap := captain.NewCaptain(basePath, spawner, memDB, agentConfigs)

	// Initialize embedded NATS server
	natsConfig := natslib.EmbeddedServerConfig{
		Port:      4222,
		JetStream: true,
		DataDir:   filepath.Join(basePath, "data", "nats"),
	}
	natsServer, err := natslib.NewEmbeddedServer(natsConfig)
	if err != nil {
		log.Printf("[NATS] Warning: Failed to create NATS server: %v", err)
	} else {
		if err := natsServer.Start(); err != nil {
			log.Printf("[NATS] Warning: Failed to start NATS server: %v", err)
		} else {
			log.Printf("[NATS] Embedded server started on %s", natsServer.URL())
		}
	}

	// Create NATS client with "server" as client ID
	var natsClient *natslib.Client
	if natsServer != nil && natsServer.IsRunning() {
		client, err := natslib.NewClient(natsServer.URL(), "server")
		if err != nil {
			log.Printf("[NATS] Warning: Failed to create client: %v", err)
		} else {
			natsClient = client
			log.Printf("[NATS] Client connected as 'server'")
		}
	}

	s := &Server{
		hub:            NewHub(),
		store:          store,
		spawner:        spawner,
		mcp:            mcpServer,
		metrics:        metricsCollector,
		alerts:         alertEngine,
		config:         config,
		projectsConfig: projectsConfig,
		memDB:          memDB,
		notifications:  notificationMgr,
		captain:        cap,
		basePath:       basePath,
		port:           port,
		startTime:      time.Now(),
		stopChan:       make(chan struct{}),
		ShutdownChan:   make(chan struct{}),
		natsServer:     natsServer,
		natsClient:     natsClient,
		natsBridge:     nil, // initialized after struct creation
	}

	// Initialize NATS bridge for message handling
	if s.natsClient != nil {
		s.natsBridge = NewNATSBridge(s, s.natsClient)
	}

	// Pass NATS URL to spawner
	if s.natsServer != nil && s.natsServer.IsRunning() {
		s.spawner.SetNATSURL(s.natsServer.URL())
	}

	// Initialize NATS connection status in store
	if s.natsServer != nil && s.natsServer.IsRunning() {
		s.store.SetNATSConnected(true)
	} else {
		s.store.SetNATSConnected(false)
	}

	s.setupRoutes()
	s.setupMCPCallbacks()

	// Initialize cleanup service
	s.cleanupCtx, s.cleanupCancel = context.WithCancel(context.Background())
	s.cleanup = NewCleanupService(s.memDB, s.store, s.hub)

	return s
}

// setupRoutes configures HTTP routes
func (s *Server) setupRoutes() {
	s.router = mux.NewRouter()

	// API routes
	api := s.router.PathPrefix("/api").Subrouter()
	api.HandleFunc("/state", s.handleGetState).Methods("GET")
	api.HandleFunc("/projects", s.handleGetProjects).Methods("GET")
	api.HandleFunc("/agents/spawn", s.handleSpawnAgent).Methods("POST")
	api.HandleFunc("/agents/{id}/stop", s.handleStopAgent).Methods("POST")
	api.HandleFunc("/agents/{id}/graceful-stop", s.handleGracefulStopAgent).Methods("POST")
	api.HandleFunc("/agents/cleanup", s.handleCleanupAgents).Methods("POST")
	api.HandleFunc("/agents/db", s.handleGetAgentsFromDB).Methods("GET")
	api.HandleFunc("/human-input/{id}", s.handleAnswerHumanInput).Methods("POST")
	api.HandleFunc("/alerts", s.handleGetAlerts).Methods("GET")
	api.HandleFunc("/alerts/clear", s.handleClearAllAlerts).Methods("POST")
	api.HandleFunc("/alerts/{id}/ack", s.handleAcknowledgeAlert).Methods("POST")
	api.HandleFunc("/thresholds", s.handleUpdateThresholds).Methods("PUT")
	api.HandleFunc("/metrics/reset", s.handleResetMetrics).Methods("POST")
	api.HandleFunc("/health", s.handleHealthCheck).Methods("GET")
	api.HandleFunc("/shutdown", s.handleShutdown).Methods("POST")
	api.HandleFunc("/stats", s.handleGetStats).Methods("GET")

	// Notification API routes
	api.HandleFunc("/notifications/banner", s.handleGetBanner).Methods("GET")
	api.HandleFunc("/notifications/banner/clear", s.handleClearBanner).Methods("POST")

	// Stop request management routes
	api.HandleFunc("/stop-requests", s.handleGetStopRequests).Methods("GET")
	api.HandleFunc("/stop-requests/{id}/respond", s.handleRespondStopRequest).Methods("POST")

	// Supervisor API routes
	supervisorHandler := handlers.NewSupervisorHandler(s.memDB)
	supervisorHandler.RegisterRoutes(api)

	// Coordination API routes (Captain's decision engine)
	coordinationHandler := handlers.NewCoordinationHandler(s.memDB, s.spawner, s.getAgentConfigsMap())
	coordinationHandler.RegisterRoutes(api)

	// Captain orchestration routes
	captainHandler := handlers.NewCaptainHandler(s.captain, s.store)
	api.HandleFunc("/captain/decide", captainHandler.HandleDecideMode).Methods("POST")
	api.HandleFunc("/captain/execute", captainHandler.HandleExecuteMission).Methods("POST")
	api.HandleFunc("/captain/execute/parallel", captainHandler.HandleExecuteParallel).Methods("POST")
	api.HandleFunc("/captain/import-tasks", captainHandler.HandleImportTasks).Methods("POST")
	api.HandleFunc("/captain/subagents", captainHandler.HandleActiveSubagents).Methods("GET")
	api.HandleFunc("/captain/api-key", captainHandler.HandleSetAPIKey).Methods("POST")
	api.HandleFunc("/captain/recon", captainHandler.HandleRecon).Methods("POST")
	// New Captain endpoints
	api.HandleFunc("/captain/task", captainHandler.HandleSubmitTask).Methods("POST")
	api.HandleFunc("/captain/status", captainHandler.HandleGetStatus).Methods("GET")
	api.HandleFunc("/captain/trigger-recon", captainHandler.HandleTriggerRecon).Methods("POST")
	api.HandleFunc("/captain/escalations", captainHandler.HandleGetEscalations).Methods("GET")
	api.HandleFunc("/captain/escalation/{id}/respond", captainHandler.HandleRespondToEscalation).Methods("POST")

	// Captain Supervisor (terminal process) endpoints
	api.HandleFunc("/captain/terminal/status", s.handleCaptainTerminalStatus).Methods("GET")
	api.HandleFunc("/captain/terminal/restart", s.handleCaptainTerminalRestart).Methods("POST")

	// WebSocket
	s.router.HandleFunc("/ws", s.handleWebSocket)

	// MCP endpoints
	s.router.HandleFunc("/mcp/sse", s.mcp.ServeSSE)
	s.router.HandleFunc("/mcp/messages/", s.mcp.ServeMessage)

	// Static files
	staticFS, _ := fs.Sub(web.StaticFiles, ".")
	s.router.PathPrefix("/").Handler(http.FileServer(http.FS(staticFS)))
}

// setupMCPCallbacks wires MCP tool handlers to services
func (s *Server) setupMCPCallbacks() {
	callbacks := mcp.ToolCallbacks{
		OnRegisterAgent: func(agentID, role string) (interface{}, error) {
			// Update agent status
			s.store.UpdateAgent(agentID, func(a *types.Agent) {
				a.Status = types.StatusConnected
				a.LastSeen = time.Now()
			})

			// Update status in database
			if s.memDB != nil {
				s.memDB.UpdateStatus(agentID, "connected", "")
			}

			s.broadcastState()
			return map[string]string{"status": "registered"}, nil
		},

		OnReportStatus: func(agentID, status, task string) (interface{}, error) {
			s.store.UpdateAgent(agentID, func(a *types.Agent) {
				a.Status = types.AgentStatus(status)
				a.CurrentTask = task
				a.LastSeen = time.Now()
			})

			// Update status in database
			if s.memDB != nil {
				s.memDB.UpdateStatus(agentID, status, task)
			}

			// Update metrics idle tracking
			if status == string(types.StatusIdle) {
				s.metrics.SetAgentIdle(agentID)
			} else {
				s.metrics.SetAgentActive(agentID)
			}

			s.broadcastState()
			return map[string]string{"status": "updated"}, nil
		},

		OnReportMetrics: func(agentID string, m *types.AgentMetrics) (interface{}, error) {
			s.metrics.UpdateAgentMetrics(agentID, m)
			s.store.UpdateMetrics(agentID, m)
			s.checkAlerts()
			s.broadcastState()
			return map[string]string{"status": "recorded"}, nil
		},

		OnRequestHumanInput: func(req *types.HumanInputRequest) (interface{}, error) {
			s.store.AddHumanRequest(req)
			s.hub.BroadcastAlert(&types.Alert{
				ID:        req.ID,
				Type:      "human_input_needed",
				AgentID:   req.AgentID,
				Message:   req.Question,
				Severity:  "warning",
				CreatedAt: time.Now(),
			})
			s.broadcastState()
			return map[string]string{"request_id": req.ID, "status": "pending"}, nil
		},

		OnLogActivity: func(activity *types.ActivityLog) (interface{}, error) {
			s.store.AddActivity(activity)
			s.hub.BroadcastActivity(activity)
			return map[string]string{"status": "logged"}, nil
		},

		OnGetAgentMetrics: func() (interface{}, error) {
			return s.metrics.GetAllMetrics(), nil
		},

		OnGetPendingQuestions: func() (interface{}, error) {
			return s.store.GetPendingRequests(), nil
		},

		OnEscalateAlert: func(alert *types.Alert) (interface{}, error) {
			s.store.AddAlert(alert)
			s.hub.BroadcastAlert(alert)
			s.broadcastState()
			return map[string]string{"alert_id": alert.ID}, nil
		},

		OnSubmitJudgment: func(judgment *types.SupervisorJudgment) (interface{}, error) {
			s.store.AddJudgment(judgment)

			// Handle action
			switch judgment.Action {
			case "restart":
				// Stop agent - respawn would need to be triggered separately
				agent := s.store.GetAgent(judgment.AgentID)
				if agent != nil {
					s.spawner.StopAgent(judgment.AgentID)
					s.spawner.CleanupAgentFiles(judgment.AgentID)
				}
			case "pause":
				s.store.UpdateAgent(judgment.AgentID, func(a *types.Agent) {
					a.Status = types.StatusBlocked
				})
			}

			s.broadcastState()
			return map[string]string{"status": "recorded"}, nil
		},

		OnGetAgentList: func() (interface{}, error) {
			return s.store.GetState().Agents, nil
		},

		OnRequestStopApproval: func(req *types.StopApprovalRequest) (interface{}, error) {
			s.store.AddStopRequest(req)
			// Alert supervisor about pending stop request
			s.hub.BroadcastAlert(&types.Alert{
				ID:        req.ID,
				Type:      "stop_approval_needed",
				AgentID:   req.AgentID,
				Message:   fmt.Sprintf("Agent %s wants to stop: %s", req.AgentID, req.Reason),
				Severity:  "warning",
				CreatedAt: time.Now(),
			})
			s.broadcastState()
			return map[string]string{"request_id": req.ID, "status": "pending_approval"}, nil
		},

		OnGetStopRequestByID: func(id string) *types.StopApprovalRequest {
			return s.store.GetStopRequestByID(id)
		},

		OnGetPendingStopRequests: func() (interface{}, error) {
			return s.store.GetPendingStopRequests(), nil
		},

		OnRespondStopRequest: func(id string, approved bool, response string) (interface{}, error) {
			s.store.RespondStopRequest(id, approved, response, "supervisor")
			s.broadcastState()
			return map[string]string{"status": "responded", "approved": fmt.Sprintf("%v", approved)}, nil
		},

		OnGetMyTasks: func(agentID, status string) (interface{}, error) {
			tasks, err := s.memDB.GetTasks(memory.TaskFilter{
				AssignedAgentID: agentID,
				Status:          status,
				Limit:           50,
			})
			if err != nil {
				return nil, err
			}
			return map[string]interface{}{
				"tasks": tasks,
				"count": len(tasks),
			}, nil
		},

		OnClaimTask: func(agentID, taskID string) (interface{}, error) {
			task, err := s.memDB.GetTask(taskID)
			if err != nil {
				return nil, fmt.Errorf("task not found: %v", err)
			}
			if task.AssignedAgentID != "" {
				return nil, fmt.Errorf("task already assigned to %s", task.AssignedAgentID)
			}
			if task.Status != "pending" {
				return nil, fmt.Errorf("task is not pending (current status: %s)", task.Status)
			}
			err = s.memDB.UpdateTaskStatus(taskID, "assigned", agentID)
			if err != nil {
				return nil, err
			}
			return map[string]interface{}{
				"success": true,
				"task_id": taskID,
				"message": "Task claimed successfully",
			}, nil
		},

		OnUpdateTaskProgress: func(agentID, taskID, status, note string) (interface{}, error) {
			// Validate status
			if status != "in_progress" && status != "blocked" {
				return nil, fmt.Errorf("invalid status: %s (must be 'in_progress' or 'blocked')", status)
			}
			err := s.memDB.UpdateTaskStatus(taskID, status, agentID)
			if err != nil {
				return nil, err
			}
			// Store note as learning if provided
			if note != "" {
				s.memDB.StoreAgentLearning(&memory.AgentLearning{
					AgentID:  agentID,
					Category: "task_progress",
					Title:    fmt.Sprintf("Progress on task %s", taskID),
					Content:  note,
				})
			}
			return map[string]interface{}{
				"success": true,
				"task_id": taskID,
				"status":  status,
			}, nil
		},

		OnCompleteTask: func(agentID, taskID, summary string) (interface{}, error) {
			err := s.memDB.UpdateTaskStatus(taskID, "completed", agentID)
			if err != nil {
				return nil, err
			}
			// Store summary as agent learning
			s.memDB.StoreAgentLearning(&memory.AgentLearning{
				AgentID:  agentID,
				Category: "task_completion",
				Title:    fmt.Sprintf("Completed task %s", taskID),
				Content:  summary,
			})
			return map[string]interface{}{
				"success": true,
				"task_id": taskID,
				"message": "Task completed",
			}, nil
		},

		// Snake reconnaissance callbacks
		OnSubmitReconReport: func(agentID string, report map[string]interface{}) (interface{}, error) {
			// Log the report as activity
			s.store.AddActivity(&types.ActivityLog{
				ID:        fmt.Sprintf("recon-%d", time.Now().Unix()),
				AgentID:   agentID,
				Action:    "submitted_recon_report",
				Details:   fmt.Sprintf("Environment: %v, Mission: %v", report["environment"], report["mission"]),
				Timestamp: time.Now(),
			})

			// Store report as agent learning for future reference
			reportJSON, _ := json.Marshal(report)
			s.memDB.StoreAgentLearning(&memory.AgentLearning{
				AgentID:  agentID,
				Category: "reconnaissance",
				Title:    fmt.Sprintf("Recon: %v - %v", report["environment"], report["mission"]),
				Content:  string(reportJSON),
			})

			// Alert if critical findings
			if findings, ok := report["findings"].(map[string]interface{}); ok {
				if critical, ok := findings["critical"].([]interface{}); ok && len(critical) > 0 {
					s.hub.BroadcastAlert(&types.Alert{
						ID:        fmt.Sprintf("critical-finding-%d", time.Now().Unix()),
						Type:      "critical_security_finding",
						AgentID:   agentID,
						Message:   fmt.Sprintf("%s found %d critical issues in %v", agentID, len(critical), report["environment"]),
						Severity:  "critical",
						CreatedAt: time.Now(),
					})
				}
			}

			s.broadcastState()
			return map[string]interface{}{
				"status":  "received",
				"message": "Reconnaissance report submitted successfully",
			}, nil
		},

		OnRequestGuidance: func(agentID string, guidance map[string]interface{}) (interface{}, error) {
			// Log guidance request
			s.store.AddActivity(&types.ActivityLog{
				ID:        fmt.Sprintf("guidance-%d", time.Now().Unix()),
				AgentID:   agentID,
				Action:    "requested_guidance",
				Details:   fmt.Sprintf("Situation: %v", guidance["situation"]),
				Timestamp: time.Now(),
			})

			// Alert Captain/Supervisor
			s.hub.BroadcastAlert(&types.Alert{
				ID:        fmt.Sprintf("guidance-req-%d", time.Now().Unix()),
				Type:      "guidance_requested",
				AgentID:   agentID,
				Message:   fmt.Sprintf("%s requests guidance: %v", agentID, guidance["situation"]),
				Severity:  "warning",
				CreatedAt: time.Now(),
			})

			s.broadcastState()
			return map[string]interface{}{
				"status":  "queued",
				"message": "Guidance request sent to Captain",
			}, nil
		},

		OnReportProgress: func(agentID string, progress map[string]interface{}) (interface{}, error) {
			// Update agent status with progress
			s.store.UpdateAgent(agentID, func(a *types.Agent) {
				a.CurrentTask = fmt.Sprintf("Scanning: %v (%v%% complete)", progress["phase"], progress["percent_complete"])
				a.LastSeen = time.Now()
			})

			// Log progress
			s.store.AddActivity(&types.ActivityLog{
				ID:        fmt.Sprintf("progress-%d", time.Now().Unix()),
				AgentID:   agentID,
				Action:    "reported_progress",
				Details:   fmt.Sprintf("Phase: %v, Progress: %v%%, Files: %v, Findings: %v", progress["phase"], progress["percent_complete"], progress["files_scanned"], progress["findings_so_far"]),
				Timestamp: time.Now(),
			})

			s.broadcastState()
			return map[string]interface{}{
				"status": "recorded",
			}, nil
		},

		OnSignalCaptain: func(agentID, signal, context string) (interface{}, error) {
			// Map signal to agent status
			statusMap := map[string]string{
				"stopped":       "stopped",
				"blocked":       "blocked",
				"completed":     "completed",
				"error":         "error",
				"need_guidance": "waiting",
			}
			status := statusMap[signal]
			if status == "" {
				status = signal
			}

			// Update agent status in DB
			if err := s.memDB.UpdateStatus(agentID, status, context); err != nil {
				// Log but don't fail
				fmt.Printf("[SIGNAL] Warning: failed to update agent status in DB: %v\n", err)
			}

			// Update dashboard state
			s.store.UpdateAgent(agentID, func(a *types.Agent) {
				a.Status = types.AgentStatus(status)
				a.CurrentTask = context
				a.LastSeen = time.Now()
			})

			// Create alert for Captain
			s.store.AddAlert(&types.Alert{
				ID:        fmt.Sprintf("signal-%d", time.Now().UnixNano()),
				Type:      "agent_signal",
				AgentID:   agentID,
				Message:   fmt.Sprintf("Agent %s signaled: %s - %s", agentID, signal, context),
				Severity:  "info",
				CreatedAt: time.Now(),
			})

			s.hub.BroadcastAlert(&types.Alert{
				ID:        fmt.Sprintf("signal-%d", time.Now().UnixNano()),
				Type:      "agent_signal",
				AgentID:   agentID,
				Message:   fmt.Sprintf("Agent %s: %s", agentID, signal),
				Severity:  "info",
				CreatedAt: time.Now(),
			})

			s.broadcastState()
			return map[string]interface{}{
				"status": "acknowledged",
				"signal": signal,
			}, nil
		},

		// Learning memory callbacks
		OnStoreKnowledge: func(agentID string, knowledge map[string]interface{}) (interface{}, error) {
			learningDB := s.memDB.AsLearningDB()

			// Extract fields from map
			category, _ := knowledge["category"].(string)
			title, _ := knowledge["title"].(string)
			content, _ := knowledge["content"].(string)

			var tags []string
			if tagsRaw, ok := knowledge["tags"].([]interface{}); ok {
				for _, t := range tagsRaw {
					if str, ok := t.(string); ok {
						tags = append(tags, str)
					}
				}
			}

			k := &memory.Knowledge{
				Category: category,
				Title:    title,
				Content:  content,
				Tags:     tags,
				Source:   agentID,
			}

			if err := learningDB.StoreKnowledge(k); err != nil {
				return nil, fmt.Errorf("failed to store knowledge: %w", err)
			}

			return map[string]interface{}{
				"knowledge_id": k.ID,
				"status":       "stored",
			}, nil
		},

		OnSearchKnowledge: func(query, category string, limit int) (interface{}, error) {
			learningDB := s.memDB.AsLearningDB()

			results, err := learningDB.SearchKnowledge(query, category, limit)
			if err != nil {
				return nil, fmt.Errorf("failed to search knowledge: %w", err)
			}

			// Convert to response format
			var items []map[string]interface{}
			for _, k := range results {
				items = append(items, map[string]interface{}{
					"id":              k.ID,
					"category":        k.Category,
					"title":           k.Title,
					"content":         k.Content,
					"tags":            k.Tags,
					"use_count":       k.UseCount,
					"relevance_score": k.RelevanceScore,
				})
				// Increment use count for retrieved knowledge
				learningDB.IncrementUseCount(k.ID)
			}

			return map[string]interface{}{
				"results": items,
				"count":   len(items),
			}, nil
		},

		OnRecordEpisode: func(agentID string, episode map[string]interface{}) (interface{}, error) {
			learningDB := s.memDB.AsLearningDB()

			eventType, _ := episode["event_type"].(string)
			title, _ := episode["title"].(string)
			content, _ := episode["content"].(string)
			project, _ := episode["project"].(string)
			importance := 0.5
			if imp, ok := episode["importance"].(float64); ok {
				importance = imp
			}

			// Use agent ID as session ID for now
			sessionID := agentID

			ep := &memory.Episode{
				SessionID:  sessionID,
				AgentID:    agentID,
				EventType:  eventType,
				Title:      title,
				Content:    content,
				Project:    project,
				Importance: importance,
			}

			if err := learningDB.RecordEpisode(ep); err != nil {
				return nil, fmt.Errorf("failed to record episode: %w", err)
			}

			return map[string]interface{}{
				"episode_id": ep.ID,
				"status":     "recorded",
			}, nil
		},

		OnGetRecentEpisodes: func(sessionID string, limit int) (interface{}, error) {
			learningDB := s.memDB.AsLearningDB()

			episodes, err := learningDB.GetRecentEpisodes(sessionID, limit)
			if err != nil {
				return nil, fmt.Errorf("failed to get episodes: %w", err)
			}

			var items []map[string]interface{}
			for _, ep := range episodes {
				items = append(items, map[string]interface{}{
					"id":         ep.ID,
					"session_id": ep.SessionID,
					"agent_id":   ep.AgentID,
					"event_type": ep.EventType,
					"title":      ep.Title,
					"content":    ep.Content,
					"project":    ep.Project,
					"importance": ep.Importance,
					"created_at": ep.CreatedAt,
				})
			}

			return map[string]interface{}{
				"episodes": items,
				"count":    len(items),
			}, nil
		},

		OnSearchEpisodes: func(query, project string, limit int) (interface{}, error) {
			learningDB := s.memDB.AsLearningDB()

			episodes, err := learningDB.SearchEpisodes(query, project, limit)
			if err != nil {
				return nil, fmt.Errorf("failed to search episodes: %w", err)
			}

			var items []map[string]interface{}
			for _, ep := range episodes {
				items = append(items, map[string]interface{}{
					"id":         ep.ID,
					"session_id": ep.SessionID,
					"agent_id":   ep.AgentID,
					"event_type": ep.EventType,
					"title":      ep.Title,
					"content":    ep.Content,
					"project":    ep.Project,
					"importance": ep.Importance,
					"created_at": ep.CreatedAt,
				})
			}

			return map[string]interface{}{
				"results": items,
				"count":   len(items),
			}, nil
		},

		// Skill Router callback
		OnSkillQuery: func(agentID, query string, limit int) (interface{}, error) {
			skillRouter := router.NewSkillRouter(s.memDB)
			result, err := skillRouter.RouteQuery(query, limit)
			if err != nil {
				return nil, fmt.Errorf("skill query failed: %w", err)
			}
			return result, nil
		},
	}

	mcp.RegisterDefaultTools(s.mcp, callbacks)

	// Set connection callbacks
	s.mcp.SetConnectionCallbacks(
		func(agentID string) {
			// Agent connected
			s.store.UpdateAgent(agentID, func(a *types.Agent) {
				a.Status = types.StatusConnected
				a.LastSeen = time.Now()
			})
			s.broadcastState()
		},
		func(agentID string) {
			// Agent disconnected
			s.store.UpdateAgent(agentID, func(a *types.Agent) {
				a.Status = types.StatusDisconnected
			})

			s.checkAlerts()
			s.broadcastState()
		},
	)

	// Set shutdown checker
	s.mcp.SetShutdownChecker(func(agentID string) bool {
		state := s.store.GetState()
		if agent, ok := state.Agents[agentID]; ok {
			return agent.ShutdownRequested
		}
		return false
	})

	// Set tool call callback for token estimation
	// Since Claude agents cannot introspect their own token usage,
	// we estimate based on MCP tool calls (roughly 500 tokens per call)
	s.mcp.SetToolCallCallback(func(agentID string, toolName string) {
		const tokensPerCall = 500
		const costPer1kTokens = 0.003 // Rough estimate

		// Get current metrics or create new
		state := s.store.GetState()
		agentMetrics := state.Metrics[agentID]
		if agentMetrics == nil {
			agentMetrics = &types.AgentMetrics{}
		}

		// Add estimated tokens
		newTokens := agentMetrics.TokensUsed + tokensPerCall
		newCost := float64(newTokens) / 1000.0 * costPer1kTokens

		s.store.UpdateMetrics(agentID, &types.AgentMetrics{
			TokensUsed:         newTokens,
			EstimatedCost:      newCost,
			FailedTests:        agentMetrics.FailedTests,
			ConsecutiveRejects: agentMetrics.ConsecutiveRejects,
		})
		s.broadcastState()
	})
}

// Start starts the HTTP server
func (s *Server) Start(addr string) error {
	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: s.router,
	}

	// Start hub
	go s.hub.Run()

	// Start background tasks
	go s.backgroundTasks()

	// Start auto-cleanup service
	go s.cleanup.Start(s.cleanupCtx)

	// Start NATS message bridge
	if s.natsBridge != nil {
		if err := s.natsBridge.Start(); err != nil {
			log.Printf("[NATS] Warning: Failed to start NATS bridge: %v", err)
		} else {
			log.Printf("[NATS] Bridge started, processing messages")
		}
	}

	fmt.Printf("Dashboard ready at http://localhost%s\n", addr)
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	close(s.stopChan)

	// Stop cleanup service
	if s.cleanupCancel != nil {
		s.cleanupCancel()
	}

	// Stop NATS bridge
	if s.natsBridge != nil {
		s.natsBridge.Stop()
	}

	// Shutdown NATS
	if s.natsClient != nil {
		s.natsClient.Close()
	}
	if s.natsServer != nil {
		s.natsServer.Shutdown()
		log.Printf("[NATS] Server shutdown complete")
	}

	// Save state
	s.store.Save()

	return s.httpServer.Shutdown(ctx)
}

// RequestShutdown signals the server to shut down gracefully
// This is safe to call multiple times - subsequent calls are no-ops
func (s *Server) RequestShutdown() {
	select {
	case <-s.ShutdownChan:
		// Already closed
	default:
		close(s.ShutdownChan)
	}
}

// SetCaptainSupervisor sets the captain supervisor reference for API endpoints
func (s *Server) SetCaptainSupervisor(supervisor *captain.CaptainSupervisor) {
	s.captainSupervisor = supervisor
}

// backgroundTasks runs periodic tasks
func (s *Server) backgroundTasks() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopChan:
			return
		case <-ticker.C:
			s.checkAlerts()
			s.checkAgentHealth()
			s.metrics.TakeSnapshot()
		}
	}
}

// checkAlerts evaluates alert conditions
func (s *Server) checkAlerts() {
	state := s.store.GetState()

	// Check metrics-based alerts
	metricsAlerts := s.alerts.CheckMetrics(state.Metrics)
	for _, alert := range metricsAlerts {
		s.store.AddAlert(alert)
		s.hub.BroadcastAlert(alert)
	}

	// Check agent status alerts
	statusAlerts := s.alerts.CheckAgentStatus(state.Agents)
	for _, alert := range statusAlerts {
		s.store.AddAlert(alert)
		s.hub.BroadcastAlert(alert)
	}

	// Check escalation queue
	pendingCount := len(s.store.GetPendingRequests())
	queueAlert := s.alerts.CheckEscalationQueue(pendingCount)
	if queueAlert != nil {
		s.store.AddAlert(queueAlert)
		s.hub.BroadcastAlert(queueAlert)
	}
}

// checkAgentHealth verifies agent processes are still running
func (s *Server) checkAgentHealth() {
	state := s.store.GetState()

	for agentID, agent := range state.Agents {
		if agent.Status != types.StatusDisconnected && agent.PID > 0 {
			if !s.spawner.IsAgentRunning(agent.PID) {
				s.store.UpdateAgent(agentID, func(a *types.Agent) {
					a.Status = types.StatusDisconnected
				})
			}
		}
	}
}

// broadcastState sends current state to all WebSocket clients
func (s *Server) broadcastState() {
	s.hub.BroadcastState(s.store.GetState())
}

// getAgentConfig finds agent config by name
func (s *Server) getAgentConfig(name string) *types.AgentConfig {
	// Check regular agents first
	for _, cfg := range s.config.Agents {
		if cfg.Name == name {
			return &cfg
		}
	}
	// Check supervisor config
	if s.config.Supervisor.Name == name {
		return &s.config.Supervisor
	}
	return nil
}

// getAgentConfigsMap returns all agent configs as a map by name
func (s *Server) getAgentConfigsMap() map[string]types.AgentConfig {
	configs := make(map[string]types.AgentConfig)
	for _, cfg := range s.config.Agents {
		configs[cfg.Name] = cfg
	}
	// Include supervisor in the map
	if s.config.Supervisor.Name != "" {
		configs[s.config.Supervisor.Name] = s.config.Supervisor
	}
	return configs
}
