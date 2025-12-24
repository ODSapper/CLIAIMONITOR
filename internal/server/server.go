package server

import (
	"context"
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

	// MCP endpoint (POST-only JSON-RPC)
	s.router.HandleFunc("/mcp", s.mcp.ServeHTTP)

	// Static files
	staticFS, err := fs.Sub(web.StaticFiles, ".")
	if err != nil {
		log.Printf("[SERVER] Warning: Failed to create static file system: %v", err)
	} else {
		s.router.PathPrefix("/").Handler(http.FileServer(http.FS(staticFS)))
	}
}

// setupMCPCallbacks wires MCP tool handlers to services
// SIMPLIFIED: Only the callbacks we actually use
func (s *Server) setupMCPCallbacks() {
	callbacks := mcp.ToolCallbacks{
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
	}

	mcp.RegisterDefaultTools(s.mcp, callbacks)

	// Agent status is tracked via wezterm pane existence, not SSE connections

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
