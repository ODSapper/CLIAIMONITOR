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

const (
	// MaxReviewCycles is the maximum number of review-rework cycles before escalation
	MaxReviewCycles = 3
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
	basePath          string

	// Task system
	taskQueue *tasks.Queue
	taskStore *tasks.Store

	// Event bus for real-time notifications
	eventBus     *events.Bus
	eventStore   *events.SQLiteStore
	notifyRouter *notifications.Router

	// Instance metadata
	port      int
	startTime time.Time

	// Background tasks
	stopChan chan struct{}

	// Shutdown signaling - external code can listen to this
	ShutdownChan chan struct{}
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
	}

	// Seed default prompts from files if DB is empty
	if s.memDB != nil {
		if sqliteDB, ok := s.memDB.(*memory.SQLiteMemoryDB); ok {
			promptsDir := filepath.Join(basePath, "configs", "prompts")
			if err := sqliteDB.SeedDefaultPrompts(promptsDir); err != nil {
				log.Printf("[SERVER] Warning: Failed to seed default prompts: %v", err)
			}
		}
	}

	// Seed configs from YAML files if DB is empty
	if s.memDB != nil {
		configsDir := filepath.Join(basePath, "configs")

		// Seed teams.yaml
		teamsPath := filepath.Join(configsDir, "teams.yaml")
		if content, err := os.ReadFile(teamsPath); err == nil {
			if existing, _ := s.memDB.GetConfig("teams"); existing == nil {
				if err := s.memDB.SaveConfig("teams", string(content), "yaml"); err != nil {
					log.Printf("[SERVER] Warning: Failed to seed teams config: %v", err)
				} else {
					log.Printf("[SERVER] Seeded teams config from teams.yaml")
				}
			}
		}

		// Seed projects.yaml
		projectsPath := filepath.Join(configsDir, "projects.yaml")
		if content, err := os.ReadFile(projectsPath); err == nil {
			if existing, _ := s.memDB.GetConfig("projects"); existing == nil {
				if err := s.memDB.SaveConfig("projects", string(content), "yaml"); err != nil {
					log.Printf("[SERVER] Warning: Failed to seed projects config: %v", err)
				} else {
					log.Printf("[SERVER] Seeded projects config from projects.yaml")
				}
			}
		}

		// Seed notifications.yaml
		notificationsPath := filepath.Join(configsDir, "notifications.yaml")
		if content, err := os.ReadFile(notificationsPath); err == nil {
			if existing, _ := s.memDB.GetConfig("notifications"); existing == nil {
				if err := s.memDB.SaveConfig("notifications", string(content), "yaml"); err != nil {
					log.Printf("[SERVER] Warning: Failed to seed notifications config: %v", err)
				} else {
					log.Printf("[SERVER] Seeded notifications config from notifications.yaml")
				}
			}
		}
	}

	// Connect spawner to memDB
	if s.spawner != nil && s.memDB != nil {
		s.spawner.SetMemoryDB(s.memDB)
	}

	// Initialize connection status in store (SSE-based, pure MCP)
	s.store.SetCaptainConnected(false) // Will be set true when Captain registers via MCP

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

	return s
}

// setupRoutes configures HTTP routes
func (s *Server) setupRoutes() {
	s.router = mux.NewRouter()

	// Apply security middleware globally to all routes
	s.router.Use(SecurityHeadersMiddleware)

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

	// Captain pane ID (for WezTerm spawning)
	api.HandleFunc("/captain/pane", s.handleSetCaptainPaneID).Methods("POST")
	api.HandleFunc("/captain/pane", s.handleGetCaptainPaneID).Methods("GET")

	// Captain context endpoints (for session persistence)
	api.HandleFunc("/captain/context", s.handleGetCaptainContext).Methods("GET")
	api.HandleFunc("/captain/context", s.handleSetCaptainContext).Methods("POST")
	api.HandleFunc("/captain/context/{key}", s.handleDeleteCaptainContext).Methods("DELETE")
	api.HandleFunc("/captain/context/summary", s.handleGetCaptainContextSummary).Methods("GET")

	// Review Board / Leaderboard endpoints
	api.HandleFunc("/leaderboard", s.handleGetLeaderboard).Methods("GET")
	api.HandleFunc("/review-boards", s.handleGetReviewBoards).Methods("GET")
	api.HandleFunc("/defect-categories", s.handleGetDefectCategories).Methods("GET")

	// Escalation & Captain Control endpoints
	api.HandleFunc("/escalation/{id}/respond", s.handleSubmitEscalationResponse).Methods("POST")
	api.HandleFunc("/captain/command", s.handleSendCaptainCommand).Methods("POST")

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
			// Get current assignment to check review_attempt count
			assignment, err := s.memDB.GetAssignment(assignmentID)
			if err != nil {
				return nil, fmt.Errorf("failed to get assignment: %w", err)
			}
			if assignment == nil {
				return nil, fmt.Errorf("assignment %d not found", assignmentID)
			}

			status := "approved"
			message := "Review complete. Captain will process result."

			if !approved {
				// Check if we've exceeded max review cycles
				if assignment.ReviewAttempt >= MaxReviewCycles {
					// Escalate to human
					status = "escalated"
					message = fmt.Sprintf("ESCALATED: Assignment exceeded %d review cycles. Human review required.", MaxReviewCycles)

					// Create escalation for Captain
					s.store.AddActivity(&types.ActivityLog{
						ID:        fmt.Sprintf("escalation-%d", time.Now().UnixNano()),
						AgentID:   agentID,
						Action:    "escalated_for_human_review",
						Details:   fmt.Sprintf("Assignment %d escalated after %d failed reviews. Last feedback: %s", assignmentID, assignment.ReviewAttempt, feedback),
						Timestamp: time.Now(),
					})

					// Mark as escalated
					if err := s.memDB.CompleteAssignment(assignmentID, status, feedback); err != nil {
						return nil, err
					}

					log.Printf("[REVIEW-ESCALATION] Assignment %d exceeded %d review cycles, escalating to human", assignmentID, MaxReviewCycles)
				} else {
					// Request rework - increments review_attempt and sets status to "rework"
					status = "rework"
					message = fmt.Sprintf("Code needs rework (attempt %d/%d). Assignment returned to coder.", assignment.ReviewAttempt+1, MaxReviewCycles)

					if err := s.memDB.RequestRework(assignmentID, feedback); err != nil {
						return nil, err
					}

					log.Printf("[REVIEW-REJECTED] Assignment %d needs rework, attempt %d/%d", assignmentID, assignment.ReviewAttempt+1, MaxReviewCycles)
				}
			} else {
				// Approved
				log.Printf("[REVIEW-APPROVED] Assignment %d approved after %d attempt(s)", assignmentID, assignment.ReviewAttempt)

				// Mark as approved
				if err := s.memDB.CompleteAssignment(assignmentID, status, feedback); err != nil {
					return nil, err
				}
			}

			// Log activity
			s.store.AddActivity(&types.ActivityLog{
				ID:        fmt.Sprintf("review-result-%d", time.Now().UnixNano()),
				AgentID:   agentID,
				Action:    "submitted_review_result",
				Details:   fmt.Sprintf("Assignment %d reviewed: %s (attempt %d)", assignmentID, status, assignment.ReviewAttempt),
				Timestamp: time.Now(),
			})

			s.broadcastState()

			return map[string]interface{}{
				"status":         status,
				"assignment_id":  assignmentID,
				"feedback":       feedback,
				"review_attempt": assignment.ReviewAttempt,
				"max_cycles":     MaxReviewCycles,
				"escalated":      status == "escalated",
				"message":        message,
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

		// Review Board callbacks
		OnCreateReviewBoard: func(assignmentID int64, reviewerCount int, complexity int, riskLevel string) (interface{}, error) {
			// Validate reviewer count 1-5
			if reviewerCount < 1 {
				reviewerCount = 1
			}
			if reviewerCount > 5 {
				reviewerCount = 5
			}
			if riskLevel == "" {
				riskLevel = "medium"
			}

			board := &memory.ReviewBoard{
				AssignmentID:    assignmentID,
				ReviewerCount:   reviewerCount,
				Status:          "pending",
				ComplexityScore: complexity,
				RiskLevel:       riskLevel,
			}

			if err := s.memDB.CreateReviewBoard(board); err != nil {
				return nil, err
			}

			s.logActivity("[REVIEW-BOARD]", fmt.Sprintf("Created board %d for assignment %d with %d reviewers", board.ID, assignmentID, reviewerCount))

			return map[string]interface{}{
				"board_id":       board.ID,
				"assignment_id":  assignmentID,
				"reviewer_count": reviewerCount,
				"status":         "created",
				"message":        fmt.Sprintf("Review board created. Assign %d reviewers.", reviewerCount),
			}, nil
		},

		OnSubmitDefect: func(agentID string, boardID int64, defect map[string]interface{}) (interface{}, error) {
			d := &memory.ReviewDefect{
				BoardID:     boardID,
				ReviewerID:  agentID,
				Category:    defect["category"].(string),
				Severity:    defect["severity"].(string),
				Title:       defect["title"].(string),
				Description: defect["description"].(string),
				Status:      "open",
			}

			// Optional fields
			if v, ok := defect["file_path"].(string); ok {
				d.FilePath = v
			}
			if v, ok := defect["line_start"].(float64); ok {
				d.LineStart = int(v)
			}
			if v, ok := defect["line_end"].(float64); ok {
				d.LineEnd = int(v)
			}
			if v, ok := defect["suggested_fix"].(string); ok {
				d.SuggestedFix = v
			}

			if err := s.memDB.CreateDefect(d); err != nil {
				return nil, err
			}

			return map[string]interface{}{
				"defect_id": d.ID,
				"board_id":  boardID,
				"category":  d.Category,
				"severity":  d.Severity,
				"status":    "recorded",
			}, nil
		},

		OnRecordReviewerVote: func(boardID int64, reviewerID string, approved bool, confidence int, defectsFound int, tokensUsed int64) (interface{}, error) {
			vote := &memory.ReviewerVote{
				BoardID:         boardID,
				ReviewerID:      reviewerID,
				Approved:        approved,
				ConfidenceScore: confidence,
				DefectsFound:    defectsFound,
				TokensUsed:      tokensUsed,
			}

			if err := s.memDB.CreateReviewerVote(vote); err != nil {
				return nil, err
			}

			verdict := "rejected"
			if approved {
				verdict = "approved"
			}
			s.logActivity("[REVIEWER-VOTE]", fmt.Sprintf("Reviewer %s voted %s on board %d (%d defects)", reviewerID, verdict, boardID, defectsFound))

			return map[string]interface{}{
				"vote_id":     vote.ID,
				"board_id":    boardID,
				"reviewer_id": reviewerID,
				"approved":    approved,
				"status":      "recorded",
			}, nil
		},

		OnFinalizeBoard: func(boardID int64) (interface{}, error) {
			// Calculate consensus
			consensus, err := s.memDB.CalculateConsensus(boardID)
			if err != nil {
				return nil, err
			}

			// Get board to update
			board, err := s.memDB.GetReviewBoard(boardID)
			if err != nil {
				return nil, err
			}

			// Update board with final verdict
			board.Status = "completed"
			board.FinalVerdict = consensus.Decision
			board.AggregatedFeedback = consensus.AggregatedFeedback
			now := time.Now()
			board.CompletedAt = &now

			if err := s.memDB.UpdateReviewBoard(board); err != nil {
				return nil, err
			}

			// Update quality scores
			if err := s.memDB.UpdateQualityScoresAfterReview(boardID, consensus); err != nil {
				// Log but don't fail
				log.Printf("[REVIEW-BOARD] Failed to update quality scores: %v", err)
			}

			// Generate and save review report to documents table
			report, err := s.memDB.GenerateReviewReport(boardID)
			if err != nil {
				// Log but don't fail - report generation is nice-to-have
				log.Printf("[REVIEW-BOARD] Failed to generate report: %v", err)
			} else {
				// Get assignment to determine project context
				assignment, err := s.memDB.GetAssignment(board.AssignmentID)
				projectID := "unknown"
				if err == nil && assignment != nil {
					// Use task ID as project identifier if available
					if assignment.TaskID != "" {
						projectID = assignment.TaskID
					}
				}

				reportTitle := fmt.Sprintf("Review Board #%d - %s", boardID, consensus.Decision)
				if err := s.memDB.SaveReviewReport(boardID, reportTitle, report, projectID); err != nil {
					// Log but don't fail
					log.Printf("[REVIEW-BOARD] Failed to save report: %v", err)
				} else {
					log.Printf("[REVIEW-BOARD] Saved review report for board %d", boardID)
				}
			}

			s.logActivity("[REVIEW-FINALIZED]", fmt.Sprintf("Board %d: %s (votes: %d/%d, defects: %d)", boardID, consensus.Decision, consensus.VotesFor, consensus.VotesFor+consensus.VotesAgainst, consensus.TotalDefects))

			return map[string]interface{}{
				"board_id":         boardID,
				"decision":         consensus.Decision,
				"approved":         consensus.Approved,
				"votes_for":        consensus.VotesFor,
				"votes_against":    consensus.VotesAgainst,
				"total_defects":    consensus.TotalDefects,
				"critical_defects": consensus.CriticalDefects,
				"high_defects":     consensus.HighDefects,
				"feedback":         consensus.AggregatedFeedback,
			}, nil
		},

		OnGetAgentLeaderboard: func(role string, limit int) (interface{}, error) {
			if limit <= 0 {
				limit = 20
			}
			scores, err := s.memDB.GetAgentLeaderboard(role, limit)
			if err != nil {
				return nil, err
			}

			return map[string]interface{}{
				"leaderboard": scores,
				"count":       len(scores),
				"role_filter": role,
			}, nil
		},

		OnGetDefectCategories: func() (interface{}, error) {
			categories, err := s.memDB.GetDefectCategories()
			if err != nil {
				return nil, err
			}

			return map[string]interface{}{
				"categories": categories,
				"count":      len(categories),
			}, nil
		},

		// Document storage callbacks
		OnSaveDocument: func(agentID string, doc map[string]interface{}) (interface{}, error) {
			// Extract fields from map
			docType, _ := doc["doc_type"].(string)
			title, _ := doc["title"].(string)
			content, _ := doc["content"].(string)
			format := "markdown" // default
			if f, ok := doc["format"].(string); ok && f != "" {
				format = f
			}

			// Build Document object
			document := &memory.Document{
				DocType:   docType,
				Title:     title,
				Content:   content,
				Format:    format,
				AuthorID:  agentID,
				Status:    "active",
				Version:   1,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}

			// Optional fields
			if projectID, ok := doc["project_id"].(string); ok {
				document.ProjectID = projectID
			}
			if taskID, ok := doc["task_id"].(string); ok {
				document.TaskID = taskID
			}
			if tagsRaw, ok := doc["tags"].([]interface{}); ok {
				var tags []string
				for _, t := range tagsRaw {
					if s, ok := t.(string); ok {
						tags = append(tags, s)
					}
				}
				document.Tags = tags
			}

			// Save to database
			if err := s.memDB.CreateDocument(document); err != nil {
				return nil, fmt.Errorf("failed to save document: %w", err)
			}

			s.logActivity("[DOCUMENT]", fmt.Sprintf("Agent %s saved document: %s (type=%s, id=%d)", agentID, title, docType, document.ID))

			return map[string]interface{}{
				"id":      document.ID,
				"status":  "saved",
				"message": fmt.Sprintf("Document saved with ID %d", document.ID),
			}, nil
		},

		OnGetDocument: func(id int64) (interface{}, error) {
			doc, err := s.memDB.GetDocument(id)
			if err != nil {
				return nil, fmt.Errorf("failed to get document: %w", err)
			}

			return map[string]interface{}{
				"document": doc,
			}, nil
		},

		OnSearchDocuments: func(query, docType, projectID, authorID string, limit int) (interface{}, error) {
			if limit <= 0 {
				limit = 20
			}

			var docs []*memory.Document
			var err error

			// If query is provided, use full-text search
			if query != "" {
				docs, err = s.memDB.SearchDocuments(query, limit)
			} else if docType != "" {
				docs, err = s.memDB.GetDocumentsByType(docType, limit)
			} else if projectID != "" {
				docs, err = s.memDB.GetDocumentsByProject(projectID, limit)
			} else if authorID != "" {
				docs, err = s.memDB.GetDocumentsByAuthor(authorID, limit)
			} else {
				// No filters - return recent documents by type
				docs, err = s.memDB.GetDocumentsByType("", limit)
			}

			if err != nil {
				return nil, fmt.Errorf("failed to search documents: %w", err)
			}

			// Apply additional filters if provided
			if query == "" && len(docs) > 0 {
				var filtered []*memory.Document
				for _, doc := range docs {
					match := true
					if docType != "" && doc.DocType != docType {
						match = false
					}
					if projectID != "" && doc.ProjectID != projectID {
						match = false
					}
					if authorID != "" && doc.AuthorID != authorID {
						match = false
					}
					if match {
						filtered = append(filtered, doc)
					}
				}
				docs = filtered
			}

			return map[string]interface{}{
				"documents": docs,
				"count":     len(docs),
			}, nil
		},

		OnListMyDocuments: func(agentID, docType string, limit int) (interface{}, error) {
			if limit <= 0 {
				limit = 20
			}

			docs, err := s.memDB.GetDocumentsByAuthor(agentID, limit)
			if err != nil {
				return nil, fmt.Errorf("failed to list documents: %w", err)
			}

			// Filter by doc type if provided
			if docType != "" {
				var filtered []*memory.Document
				for _, doc := range docs {
					if doc.DocType == docType {
						filtered = append(filtered, doc)
					}
				}
				docs = filtered
			}

			return map[string]interface{}{
				"documents": docs,
				"count":     len(docs),
			}, nil
		},
	}

	mcp.RegisterDefaultTools(s.mcp, callbacks)

	// Set connection callbacks
	s.mcp.SetConnectionCallbacks(
		func(agentID string) {
			if err := s.atomicAgentUpdate(agentID, "connected", ""); err != nil {
				log.Printf("[MCP] ERROR: Failed to mark agent %s as connected: %v", agentID, err)
			}
			log.Printf("[MCP] Agent %s SSE connected", agentID)
			s.broadcastState()
		},
		func(agentID string) {
			log.Printf("[MCP] Agent %s SSE disconnected", agentID)

			// Record the disconnect time but don't change status yet
			s.store.UpdateAgent(agentID, func(a *types.Agent) {
				// Keep current status - don't assume anything
				// Just update LastSeen so verification can check timing
				a.LastSeen = time.Now()
			})

			// Schedule verification in 30 seconds
			// This gives the agent time to reconnect if it's just between calls
			go func() {
				time.Sleep(30 * time.Second)
				s.verifyAgentStatus(agentID)
			}()

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

	fmt.Printf("Dashboard ready at http://localhost%s\n", addr)
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	close(s.stopChan)

	// Shutdown WebSocket hub to close all channels properly
	if s.hub != nil {
		s.hub.Shutdown()
		log.Printf("[HUB] WebSocket hub shutdown complete")
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

// logActivity is a helper to log activities with a consistent format
func (s *Server) logActivity(action, details string) {
	s.store.AddActivity(&types.ActivityLog{
		ID:        fmt.Sprintf("%s-%d", strings.ToLower(strings.ReplaceAll(action, " ", "-")), time.Now().UnixNano()),
		AgentID:   "system",
		Action:    action,
		Details:   details,
		Timestamp: time.Now(),
	})
}

// atomicAgentUpdate updates the JSONStore (in-memory state).
// Note: Previously this also updated SQLite DB, but agent_control table has been removed.
func (s *Server) atomicAgentUpdate(agentID, status, task string) error {
	// Update JSONStore (in-memory)
	s.store.UpdateAgent(agentID, func(a *types.Agent) {
		a.Status = types.AgentStatus(status)
		a.CurrentTask = task
		a.LastSeen = time.Now()
	})

	return nil
}

// verifyAgentStatus checks if an agent is actually running and updates status accordingly.
// Used after SSE disconnects to verify if agent is still alive.
func (s *Server) verifyAgentStatus(agentID string) {
	state := s.store.GetState()
	agent, ok := state.Agents[agentID]
	if !ok {
		return
	}

	// Check if process is still running
	if agent.PID > 0 && s.spawner.IsAgentRunning(agent.PID) {
		// Process still alive - keep current status
		log.Printf("[VERIFY] Agent %s (PID %d) still running", agentID, agent.PID)
		return
	}

	// Check last heartbeat time (allow 60 second grace period)
	if time.Since(agent.LastSeen) < 60*time.Second {
		log.Printf("[VERIFY] Agent %s last seen %v ago, keeping status", agentID, time.Since(agent.LastSeen))
		return
	}

	// Process not running and no recent heartbeat - mark as disconnected
	log.Printf("[VERIFY] Agent %s appears dead (PID %d, last seen %v ago), marking disconnected",
		agentID, agent.PID, time.Since(agent.LastSeen))

	if err := s.atomicAgentUpdate(agentID, "disconnected", ""); err != nil {
		log.Printf("[VERIFY] Failed to update agent %s status: %v", agentID, err)
	}
	s.broadcastState()
}
