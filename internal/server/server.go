package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/CLIAIMONITOR/internal/agents"
	"github.com/CLIAIMONITOR/internal/captain"
	"github.com/CLIAIMONITOR/internal/events"
	"github.com/CLIAIMONITOR/internal/handlers"
	"github.com/CLIAIMONITOR/internal/mcp"
	"github.com/CLIAIMONITOR/internal/memory"
	"github.com/CLIAIMONITOR/internal/metrics"
	natslib "github.com/CLIAIMONITOR/internal/nats"
	"github.com/CLIAIMONITOR/internal/notifications"
	"github.com/CLIAIMONITOR/internal/notifications/external"
	"github.com/CLIAIMONITOR/internal/persistence"
	"github.com/CLIAIMONITOR/internal/router"
	"github.com/CLIAIMONITOR/internal/tasks"
	"github.com/CLIAIMONITOR/internal/types"
	"github.com/CLIAIMONITOR/web"
	"github.com/gorilla/mux"
	"gopkg.in/yaml.v3"
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

	// Task system
	taskQueue *tasks.Queue
	taskStore *tasks.Store

	// Event bus for real-time notifications
	eventBus     *events.Bus
	eventStore   *events.SQLiteStore
	notifyRouter *notifications.Router

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

// loadNotificationConfig loads notification configuration from YAML file
func loadNotificationConfig(configPath string) *types.NotificationsConfig {
	data, err := os.ReadFile(configPath)
	if err != nil {
		log.Printf("[NOTIFY] Config not found at %s, notifications disabled", configPath)
		return nil
	}

	var config types.NotificationsConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		log.Printf("[NOTIFY] Failed to parse config: %v", err)
		return nil
	}

	return &config
}

// parseEventTypes converts string event types to events.EventType
func parseEventTypes(types []string) []events.EventType {
	result := make([]events.EventType, 0, len(types))
	for _, t := range types {
		result = append(result, events.EventType(t))
	}
	return result
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
		s.store.SetCaptainConnected(true) // Captain available when NATS is up
		log.Printf("[CAPTAIN] Captain connected (NATS available)")
	} else {
		s.store.SetNATSConnected(false)
		s.store.SetCaptainConnected(false)
	}

	// Initialize task system
	s.taskQueue = tasks.NewQueue()
	s.taskStore = tasks.NewStore(memDB.(*memory.SQLiteMemoryDB).DB())
	if err := s.taskStore.Init(); err != nil {
		log.Printf("[TASKS] Warning: Failed to initialize task store: %v", err)
	} else {
		// Load persisted tasks into queue
		savedTasks, err := s.taskStore.GetAll()
		if err != nil {
			log.Printf("[TASKS] Warning: Failed to load tasks: %v", err)
		} else {
			for _, t := range savedTasks {
				s.taskQueue.Add(t)
			}
			log.Printf("[TASKS] Loaded %d persisted tasks", len(savedTasks))
		}
	}

	// Initialize event store using the same database connection
	var eventStore *events.SQLiteStore
	if sqliteDB, ok := memDB.(*memory.SQLiteMemoryDB); ok {
		var err error
		eventStore, err = events.NewSQLiteStore(sqliteDB.DB())
		if err != nil {
			log.Printf("[EVENTS] Warning: Failed to initialize event store: %v", err)
		}
	}

	// Initialize event bus (works with nil store)
	eventBus := events.NewBus(eventStore)
	log.Printf("[EVENTS] Event bus initialized (store: %v)", eventStore != nil)

	// Assign to server struct
	s.eventBus = eventBus
	s.eventStore = eventStore

	// Initialize notification router
	notifyRouter := notifications.NewRouter(nil)

	// Load notification config
	configPath := filepath.Join(basePath, "configs", "notifications.yaml")
	if notifyConfig := loadNotificationConfig(configPath); notifyConfig != nil {
		if notifyConfig.Slack.Enabled && notifyConfig.Slack.WebhookURL != "" {
			notifyRouter.AddChannel(external.NewSlackNotifier(external.SlackConfig{
				WebhookURL:  notifyConfig.Slack.WebhookURL,
				Channel:     notifyConfig.Slack.Channel,
				Username:    notifyConfig.Slack.Username,
				IconEmoji:   notifyConfig.Slack.IconEmoji,
				EventTypes:  parseEventTypes(notifyConfig.Slack.EventTypes),
				MinPriority: notifyConfig.Slack.MinPriority,
			}))
			log.Printf("[NOTIFY] Slack channel enabled")
		}
		if notifyConfig.Discord.Enabled && notifyConfig.Discord.WebhookURL != "" {
			notifyRouter.AddChannel(external.NewDiscordNotifier(external.DiscordConfig{
				WebhookURL:  notifyConfig.Discord.WebhookURL,
				Username:    notifyConfig.Discord.Username,
				AvatarURL:   notifyConfig.Discord.AvatarURL,
				EventTypes:  parseEventTypes(notifyConfig.Discord.EventTypes),
				MinPriority: notifyConfig.Discord.MinPriority,
			}))
			log.Printf("[NOTIFY] Discord channel enabled")
		}
		if notifyConfig.Email.Enabled && notifyConfig.Email.SMTPHost != "" {
			notifyRouter.AddChannel(external.NewEmailNotifier(external.EmailConfig{
				SMTPHost:    notifyConfig.Email.SMTPHost,
				SMTPPort:    notifyConfig.Email.SMTPPort,
				Username:    notifyConfig.Email.Username,
				Password:    notifyConfig.Email.Password,
				From:        notifyConfig.Email.From,
				To:          notifyConfig.Email.To,
				EventTypes:  parseEventTypes(notifyConfig.Email.EventTypes),
				MinPriority: notifyConfig.Email.MinPriority,
			}))
			log.Printf("[NOTIFY] Email channel enabled")
		}
	}
	log.Printf("[NOTIFY] Router initialized with %d channels", len(notifyRouter.GetChannels()))

	// Assign to server struct
	s.notifyRouter = notifyRouter

	// Start notification routing goroutine
	if s.eventBus != nil && s.notifyRouter != nil {
		go func() {
			sub := s.eventBus.Subscribe("all", nil)
			log.Printf("[NOTIFY] Started routing events to notification channels")
			for event := range sub {
				s.notifyRouter.Route(event)
			}
		}()
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
	api.HandleFunc("/metrics/by-model", s.handleGetMetricsByModel).Methods("GET")
	api.HandleFunc("/metrics/by-agent-type", s.handleGetMetricsByAgentType).Methods("GET")
	api.HandleFunc("/metrics/by-agent", s.handleGetMetricsByAgent).Methods("GET")
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

	// Task management routes
	taskHandler := handlers.NewTasksHandler(s.taskQueue, s.taskStore)
	api.HandleFunc("/tasks", taskHandler.HandleList).Methods("GET")
	api.HandleFunc("/tasks", taskHandler.HandleCreate).Methods("POST")
	api.HandleFunc("/tasks/{id}", taskHandler.HandleGet).Methods("GET")
	api.HandleFunc("/tasks/{id}", taskHandler.HandleUpdate).Methods("PATCH", "PUT")
	api.HandleFunc("/tasks/{id}", taskHandler.HandleDelete).Methods("DELETE")
	api.HandleFunc("/agents/{agent_id}/tasks", taskHandler.HandleAgentTasks).Methods("GET")

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

	// Captain health endpoint
	api.HandleFunc("/captain/health", s.handleCaptainHealth).Methods("GET")

	// Captain context endpoints (for session persistence)
	api.HandleFunc("/captain/context", s.handleGetCaptainContext).Methods("GET")
	api.HandleFunc("/captain/context", s.handleSetCaptainContext).Methods("POST")
	api.HandleFunc("/captain/context/{key}", s.handleDeleteCaptainContext).Methods("DELETE")
	api.HandleFunc("/captain/context/summary", s.handleGetCaptainContextSummary).Methods("GET")

	// Escalation & Captain Control endpoints
	api.HandleFunc("/escalation/{id}/respond", s.handleSubmitEscalationResponse).Methods("POST")
	api.HandleFunc("/captain/command", s.handleSendCaptainCommand).Methods("POST")
	api.HandleFunc("/nats/status", s.handleGetNATSStatus).Methods("GET")

	// WebSocket
	s.router.HandleFunc("/ws", s.handleWebSocket)

	// MCP endpoints
	s.router.HandleFunc("/mcp/sse", s.mcp.ServeSSE)
	s.router.HandleFunc("/mcp/messages/", s.mcp.ServeMessage)

	// Static files
	staticFS, err := fs.Sub(web.StaticFiles, ".")
	if err != nil {
		log.Printf("[SERVER] Warning: Failed to create static file system: %v", err)
	} else {
		s.router.PathPrefix("/").Handler(http.FileServer(http.FS(staticFS)))
	}
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

			// Persist to SQLite for historical tracking
			// Determine agent type based on ID pattern
			agentType := memory.AgentTypeSpawnedWindow
			if strings.HasPrefix(agentID, "Captain") {
				agentType = memory.AgentTypeCaptain
			} else if strings.Contains(agentID, "sgt") || strings.Contains(agentID, "SGT") {
				agentType = memory.AgentTypeSGT
			}

			if err := s.memDB.RecordMetricsWithType(
				agentID,
				m.Model,
				agentType,
				"",    // parent agent (SGTs don't have parents, workers do)
				m.TokensUsed,
				m.EstimatedCost,
				m.TaskID,
				nil, // assignment ID - could be tracked if needed
			); err != nil {
				fmt.Printf("[METRICS] Warning: failed to persist metrics to DB: %v\n", err)
			}

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
			// Get the request first to find the agent ID
			req := s.store.GetStopRequestByID(id)
			if req == nil {
				return nil, fmt.Errorf("stop request not found: %s", id)
			}

			s.store.RespondStopRequest(id, approved, response, "supervisor")
			s.broadcastState()

			// Publish event to the requesting agent via event bus
			if s.eventBus != nil {
				event := events.NewEvent(
					events.EventStopApproval,
					"supervisor",
					req.AgentID,
					events.PriorityHigh,
					map[string]interface{}{
						"request_id":  id,
						"approved":    approved,
						"response":    response,
						"reviewed_by": "supervisor",
					},
				)
				s.eventBus.Publish(event)
				log.Printf("[SERVER] Published stop_approval event to %s: approved=%v", req.AgentID, approved)
			}

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
				a.Status = types.StatusWorking
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

			// Publish to event bus so Captain's wait_for_events receives it
			if s.eventBus != nil {
				event := &events.Event{
					Type:      events.EventType("agent_signal"),
					Source:    agentID,
					Target:    "Captain",
					Payload:   map[string]interface{}{"signal": signal, "context": context},
					CreatedAt: time.Now(),
				}
				s.eventBus.Publish(event)
				log.Printf("[SERVER] Published agent_signal event: agent=%s, signal=%s, target=Captain", agentID, signal)
			}

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

		// Captain context callbacks
		OnSaveContext: func(key, value string, priority, maxAgeHours int) (interface{}, error) {
			if err := s.memDB.SetContext(key, value, priority, maxAgeHours); err != nil {
				return nil, fmt.Errorf("failed to save context: %w", err)
			}
			return map[string]interface{}{
				"success": true,
				"key":     key,
				"message": "Context saved to memory.db",
			}, nil
		},

		OnGetContext: func(key string) (interface{}, error) {
			ctx, err := s.memDB.GetContext(key)
			if err != nil {
				return nil, fmt.Errorf("failed to get context: %w", err)
			}
			if ctx == nil {
				return map[string]interface{}{
					"found": false,
					"key":   key,
				}, nil
			}
			return map[string]interface{}{
				"found":         true,
				"key":           ctx.Key,
				"value":         ctx.Value,
				"priority":      ctx.Priority,
				"max_age_hours": ctx.MaxAgeHours,
				"updated_at":    ctx.UpdatedAt,
			}, nil
		},

		OnGetAllContext: func() (interface{}, error) {
			contexts, err := s.memDB.GetAllContext()
			if err != nil {
				return nil, fmt.Errorf("failed to get all context: %w", err)
			}
			var items []map[string]interface{}
			for _, ctx := range contexts {
				items = append(items, map[string]interface{}{
					"key":           ctx.Key,
					"value":         ctx.Value,
					"priority":      ctx.Priority,
					"max_age_hours": ctx.MaxAgeHours,
					"updated_at":    ctx.UpdatedAt,
				})
			}
			return map[string]interface{}{
				"contexts": items,
				"count":    len(items),
			}, nil
		},

		OnLogSession: func(sessionID, eventType, summary, details, agentID string) (interface{}, error) {
			if err := s.memDB.LogSessionEvent(sessionID, eventType, summary, details, agentID); err != nil {
				return nil, fmt.Errorf("failed to log session event: %w", err)
			}
			return map[string]interface{}{
				"success":    true,
				"session_id": sessionID,
				"event_type": eventType,
			}, nil
		},

		OnGetCaptainMessages: func() (interface{}, error) {
			messages := s.store.GetUnreadCaptainMessages()
			var items []map[string]interface{}
			for _, msg := range messages {
				items = append(items, map[string]interface{}{
					"id":         msg.ID,
					"type":       msg.Type,
					"text":       msg.Text,
					"payload":    msg.Payload,
					"from":       msg.From,
					"created_at": msg.CreatedAt,
				})
			}
			return map[string]interface{}{
				"messages": items,
				"count":    len(items),
			}, nil
		},

		OnMarkMessagesRead: func(ids []string) (interface{}, error) {
			s.store.MarkCaptainMessagesRead(ids)
			return map[string]interface{}{
				"success":      true,
				"marked_count": len(ids),
			}, nil
		},

		OnSendCaptainResponse: func(text string) (interface{}, error) {
			s.store.AddCaptainMessage(&types.CaptainMessage{
				ID:        fmt.Sprintf("captain-msg-%d", time.Now().UnixNano()),
				Type:      "response",
				Text:      text,
				From:      "captain",
				CreatedAt: time.Now(),
			})
			s.broadcastState()
			return map[string]interface{}{
				"success": true,
				"message": "Response sent to dashboard",
			}, nil
		},

		OnGetMetricsByModel: func(modelFilter string) (interface{}, error) {
			metrics, err := s.memDB.GetMetricsByModel(modelFilter)
			if err != nil {
				return nil, fmt.Errorf("failed to get metrics by model: %w", err)
			}
			return map[string]interface{}{
				"metrics": metrics,
				"count":   len(metrics),
			}, nil
		},

		// SGT workflow callbacks
		OnDispatchTask: func(taskID, assignTo, assignmentType, branchName string) (interface{}, error) {
			assignment := &memory.TaskAssignment{
				TaskID:         taskID,
				AssignedTo:     assignTo,
				AssignedBy:     "Captain",
				AssignmentType: assignmentType,
				Status:         "pending",
				BranchName:     branchName,
				ReviewAttempt:  1,
			}
			if err := s.memDB.CreateAssignment(assignment); err != nil {
				return nil, err
			}

			// Log activity
			s.store.AddActivity(&types.ActivityLog{
				ID:        fmt.Sprintf("dispatch-%d", time.Now().UnixNano()),
				AgentID:   "Captain",
				Action:    "dispatched_task",
				Details:   fmt.Sprintf("Task %s assigned to %s (type: %s)", taskID, assignTo, assignmentType),
				Timestamp: time.Now(),
			})

			s.broadcastState()

			return map[string]interface{}{
				"status":        "dispatched",
				"assignment_id": assignment.ID,
				"assigned_to":   assignTo,
			}, nil
		},

		OnAcceptAssignment: func(agentID string, assignmentID int64) (interface{}, error) {
			if err := s.memDB.UpdateAssignmentStatus(assignmentID, "in_progress"); err != nil {
				return nil, err
			}

			// Log activity
			s.store.AddActivity(&types.ActivityLog{
				ID:        fmt.Sprintf("accept-%d", time.Now().UnixNano()),
				AgentID:   agentID,
				Action:    "accepted_assignment",
				Details:   fmt.Sprintf("Assignment %d accepted", assignmentID),
				Timestamp: time.Now(),
			})

			s.broadcastState()

			return map[string]interface{}{
				"status":        "accepted",
				"assignment_id": assignmentID,
			}, nil
		},

		OnGetMyAssignment: func(agentID string) (interface{}, error) {
			// First check for pending assignments
			assignments, err := s.memDB.GetAssignmentsByAgent(agentID, "pending")
			if err != nil {
				return nil, err
			}
			if len(assignments) > 0 {
				return map[string]interface{}{"assignment": assignments[0]}, nil
			}

			// Then check for active (in_progress) assignments
			active, err := s.memDB.GetActiveAssignment(agentID)
			if err != nil {
				return nil, err
			}
			return map[string]interface{}{"assignment": active}, nil
		},

		OnLogWorker: func(agentID string, assignmentID int64, workerType, description string) (interface{}, error) {
			worker := &memory.AssignmentWorker{
				AssignmentID:    assignmentID,
				WorkerType:      workerType,
				TaskDescription: description,
				Status:          "pending",
			}
			if err := s.memDB.AddWorker(worker); err != nil {
				return nil, err
			}

			return map[string]interface{}{
				"status":    "logged",
				"worker_id": worker.ID,
			}, nil
		},

		OnSubmitForReview: func(agentID string, assignmentID int64, branchName string) (interface{}, error) {
			if err := s.memDB.CompleteAssignment(assignmentID, "completed", ""); err != nil {
				return nil, err
			}

			// Log activity
			s.store.AddActivity(&types.ActivityLog{
				ID:        fmt.Sprintf("review-%d", time.Now().UnixNano()),
				AgentID:   agentID,
				Action:    "submitted_for_review",
				Details:   fmt.Sprintf("Assignment %d submitted for review (branch: %s)", assignmentID, branchName),
				Timestamp: time.Now(),
			})

			s.broadcastState()

			return map[string]interface{}{
				"status":        "submitted",
				"assignment_id": assignmentID,
				"branch_name":   branchName,
				"message":       "Work submitted. Captain will route to reviewer.",
			}, nil
		},

		OnSubmitReviewResult: func(agentID string, assignmentID int64, approved bool, feedback string) (interface{}, error) {
			status := "approved"
			if !approved {
				status = "rejected"
			}

			if err := s.memDB.CompleteAssignment(assignmentID, status, feedback); err != nil {
				return nil, err
			}

			// Log activity
			s.store.AddActivity(&types.ActivityLog{
				ID:        fmt.Sprintf("review-result-%d", time.Now().UnixNano()),
				AgentID:   agentID,
				Action:    "submitted_review_result",
				Details:   fmt.Sprintf("Assignment %d reviewed: %s", assignmentID, status),
				Timestamp: time.Now(),
			})

			s.broadcastState()

			return map[string]interface{}{
				"status":        status,
				"assignment_id": assignmentID,
				"feedback":      feedback,
				"message":       "Review complete. Captain will process result.",
			}, nil
		},

		OnCompleteWorker: func(agentID string, workerID int64, status, result, model string, tokensUsed int64) (interface{}, error) {
			// Update worker status
			if err := s.memDB.UpdateWorkerStatus(workerID, status, result, tokensUsed); err != nil {
				return nil, err
			}

			// Calculate cost based on model
			costPer1kTokens := 0.003 // Default for sonnet
			if strings.Contains(model, "haiku") {
				costPer1kTokens = 0.00025
			} else if strings.Contains(model, "opus") {
				costPer1kTokens = 0.015
			}
			estimatedCost := float64(tokensUsed) / 1000.0 * costPer1kTokens

			// Record metrics with subagent type
			workerAgentID := fmt.Sprintf("worker-%d", workerID)
			if err := s.memDB.RecordMetricsWithType(
				workerAgentID,
				model,
				memory.AgentTypeSubagent,
				agentID, // parent is the SGT
				tokensUsed,
				estimatedCost,
				"",  // no task ID
				nil, // assignment can be looked up from worker
			); err != nil {
				// Log but don't fail
				fmt.Printf("[METRICS] Warning: failed to record worker metrics: %v\n", err)
			}

			return map[string]interface{}{
				"status":         "recorded",
				"worker_id":      workerID,
				"tokens_used":    tokensUsed,
				"estimated_cost": estimatedCost,
			}, nil
		},

		OnGetMetricsByAgentType: func() (interface{}, error) {
			metrics, err := s.memDB.GetMetricsByAgentType()
			if err != nil {
				return nil, fmt.Errorf("failed to get metrics by agent type: %w", err)
			}
			return map[string]interface{}{
				"metrics": metrics,
				"count":   len(metrics),
			}, nil
		},

		OnGetMetricsByAgent: func() (interface{}, error) {
			metrics, err := s.memDB.GetMetricsByAgent()
			if err != nil {
				return nil, fmt.Errorf("failed to get metrics by agent: %w", err)
			}
			return map[string]interface{}{
				"metrics": metrics,
				"count":   len(metrics),
			}, nil
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

	// Register event bus tools for real-time notifications
	if s.eventBus != nil {
		mcp.RegisterWaitForEventsTool(s.mcp, s.eventBus)
		mcp.RegisterSendToAgentTool(s.mcp, s.eventBus)
		log.Printf("[MCP] Registered wait_for_events and send_to_agent tools")
	}
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
