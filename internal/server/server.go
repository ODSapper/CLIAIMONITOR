package server

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"time"

	"github.com/CLIAIMONITOR/internal/agents"
	"github.com/CLIAIMONITOR/internal/mcp"
	"github.com/CLIAIMONITOR/internal/memory"
	"github.com/CLIAIMONITOR/internal/metrics"
	"github.com/CLIAIMONITOR/internal/notifications"
	"github.com/CLIAIMONITOR/internal/persistence"
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
	store          *persistence.JSONStore
	spawner        *agents.ProcessSpawner
	mcp            *mcp.Server
	metrics        *metrics.MetricsCollector
	alerts         *metrics.AlertChecker
	config         *types.TeamsConfig
	projectsConfig *types.ProjectsConfig
	memDB          memory.MemoryDB
	notifications  *notifications.Manager
	basePath       string

	// Instance metadata
	port      int
	startTime time.Time

	// Background tasks
	stopChan chan struct{}
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

	s := &Server{
		hub:               NewHub(),
		store:             store,
		spawner:           spawner,
		mcp:               mcpServer,
		metrics:           metricsCollector,
		alerts:            alertEngine,
		config:            config,
		projectsConfig:    projectsConfig,
		memDB:             memDB,
		notifications:     notificationMgr,
		basePath:          basePath,
		port:              port,
		startTime:         time.Now(),
		stopChan:          make(chan struct{}),
	}

	s.setupRoutes()
	s.setupMCPCallbacks()

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
	api.HandleFunc("/human-input/{id}", s.handleAnswerHumanInput).Methods("POST")
	api.HandleFunc("/alerts/clear", s.handleClearAllAlerts).Methods("POST")
	api.HandleFunc("/alerts/{id}/ack", s.handleAcknowledgeAlert).Methods("POST")
	api.HandleFunc("/thresholds", s.handleUpdateThresholds).Methods("PUT")
	api.HandleFunc("/metrics/reset", s.handleResetMetrics).Methods("POST")
	api.HandleFunc("/health", s.handleHealthCheck).Methods("GET")
	api.HandleFunc("/shutdown", s.handleShutdown).Methods("POST")

	// Notification API routes
	api.HandleFunc("/notifications/banner", s.handleGetBanner).Methods("GET")
	api.HandleFunc("/notifications/banner/clear", s.handleClearBanner).Methods("POST")

	// WebSocket
	s.router.HandleFunc("/ws", s.handleWebSocket)

	// MCP endpoints
	s.router.HandleFunc("/mcp/sse", s.mcp.ServeSSE)
	s.router.HandleFunc("/mcp/message", s.mcp.ServeMessage)

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

			// Check if supervisor
			if agentID == "Supervisor" {
				s.store.SetSupervisorConnected(true)
				s.hub.BroadcastSupervisorStatus(true, s.store.GetAgent(agentID))
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

		OnGetPendingStopRequests: func() (interface{}, error) {
			return s.store.GetPendingStopRequests(), nil
		},

		OnRespondStopRequest: func(id string, approved bool, response string) (interface{}, error) {
			s.store.RespondStopRequest(id, approved, response, "supervisor")
			s.broadcastState()
			return map[string]string{"status": "responded", "approved": fmt.Sprintf("%v", approved)}, nil
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

			if agentID == "Supervisor" {
				s.store.SetSupervisorConnected(false)
				s.hub.BroadcastSupervisorStatus(false, nil)
			}

			s.checkAlerts()
			s.broadcastState()
		},
	)
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

	// Save state
	s.store.Save()

	return s.httpServer.Shutdown(ctx)
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

				if agentID == "Supervisor" {
					s.store.SetSupervisorConnected(false)
					s.hub.BroadcastSupervisorStatus(false, nil)
				}
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
	for _, cfg := range s.config.Agents {
		if cfg.Name == name {
			return &cfg
		}
	}
	return nil
}
