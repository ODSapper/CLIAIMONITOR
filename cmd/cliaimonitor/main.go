package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/CLIAIMONITOR/internal/agents"
	"github.com/CLIAIMONITOR/internal/captain"
	"github.com/CLIAIMONITOR/internal/instance"
	"github.com/CLIAIMONITOR/internal/mcp"
	"github.com/CLIAIMONITOR/internal/memory"
	"github.com/CLIAIMONITOR/internal/metrics"
	"github.com/CLIAIMONITOR/internal/persistence"
	"github.com/CLIAIMONITOR/internal/server"
	"github.com/CLIAIMONITOR/internal/types"
)

// ANSI color codes for terminal output
const (
	colorGreen = "\033[32m"
	colorReset = "\033[0m"
)

func main() {
	// Parse command line flags
	port := flag.Int("port", 3000, "HTTP server port")
	configPath := flag.String("config", "configs/teams.yaml", "Team configuration file")
	projectsPath := flag.String("projects", "configs/projects.yaml", "Projects configuration file")
	statePath := flag.String("state", "data/state.json", "State persistence file")
	mcpHost := flag.String("mcp-host", "localhost", "MCP server hostname (for agents to connect)")

	// Instance management flags
	status := flag.Bool("status", false, "Show status of running instance")
	stop := flag.Bool("stop", false, "Stop running instance gracefully")
	forceStop := flag.Bool("force-stop", false, "Force kill running instance")
	flag.Parse()

	// Handle status command
	if *status {
		showInstanceStatus(*statePath, *port)
		os.Exit(0)
	}

	// Handle stop commands
	if *stop || *forceStop {
		stopInstance(*statePath, *forceStop)
		os.Exit(0)
	}

	// Get base path (executable directory or current directory)
	basePath, err := getBasePath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to determine base path: %v\n", err)
		os.Exit(1)
	}

	// Resolve paths relative to base
	if !filepath.IsAbs(*configPath) {
		*configPath = filepath.Join(basePath, *configPath)
	}
	if !filepath.IsAbs(*projectsPath) {
		*projectsPath = filepath.Join(basePath, *projectsPath)
	}
	if !filepath.IsAbs(*statePath) {
		*statePath = filepath.Join(basePath, *statePath)
	}

	// Initialize instance manager
	pidFilePath := filepath.Join(basePath, "data", "cliaimonitor.pid")
	instanceMgr := instance.NewManager(pidFilePath, *statePath, *port)

	// Check for existing instance
	existingInfo, err := instanceMgr.CheckExistingInstance()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to check for existing instance: %v\n", err)
		os.Exit(1)
	}

	// Handle conflict if instance exists
	if existingInfo != nil && existingInfo.IsRunning {
		resolver := instance.NewConflictResolver(instanceMgr, instance.IsInteractive())
		if err := resolver.Resolve(existingInfo); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to resolve instance conflict: %v\n", err)
			os.Exit(1)
		}
		// Update port in case user chose "use different port"
		*port = instanceMgr.GetPort()
	}

	// Acquire exclusive lock
	if err := instanceMgr.AcquireLock(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to acquire instance lock: %v\n", err)
		os.Exit(1)
	}
	defer instanceMgr.ReleaseLock()

	// Initialize memory database
	dataDir := filepath.Join(basePath, "data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create data directory: %v\n", err)
		os.Exit(1)
	}

	memoryDBPath := filepath.Join(dataDir, "memory.db")
	memoryDB, err := memory.NewMemoryDB(memoryDBPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize memory database: %v\n", err)
		os.Exit(1)
	}
	defer memoryDB.Close()

	// Access learning layer interface for RAG-style queries
	learningDB := memoryDB.AsLearningDB()

	// NOTE: LM Studio embeddings integration deferred
	// Embedding provider would enable semantic search across memory
	// To implement: Configure embedding provider with LM Studio if available
	// embeddingProvider := memory.NewLMStudioEmbedding("http://localhost:1234/v1", "qwen2.5-coder-7b-instruct")
	// Currently using basic text search - semantic search can be added when LM Studio is reliably available

	_ = learningDB // learningDB is now available for use in agents/handlers

	// Start green output for server pane
	fmt.Print(colorGreen)
	fmt.Println("  Memory system initialized (operational + learning layers)")

	// Discover current repository
	repo, err := memoryDB.DiscoverRepo(basePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to discover repository: %v\n", err)
	} else {
		if repo.NeedsRescan {
			fmt.Printf("  Repository needs rescan (ID: %s)\n", repo.ID)
		} else {
			fmt.Printf("  Repository discovered (ID: %s)\n", repo.ID)
		}
	}

	// Load team configuration
	config, err := agents.LoadTeamsConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Load projects configuration
	projectsConfig, err := agents.LoadProjectsConfig(*projectsPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to load projects config: %v\n", err)
		// Use empty config if file doesn't exist
		projectsConfig = &types.ProjectsConfig{}
	}

	printBanner()

	// Initialize persistence
	store := persistence.NewJSONStore(*statePath)
	state, err := store.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load state: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("  State loaded from %s\n", *statePath)

	// Initialize components
	// Use new Streamable HTTP transport endpoint (/mcp) instead of legacy SSE (/mcp/sse)
	mcpServerURL := fmt.Sprintf("http://%s:%d/mcp", *mcpHost, *port)
	spawner := agents.NewSpawner(basePath, mcpServerURL, memoryDB)
	mcpServer := mcp.NewServer()
	metricsCollector := metrics.NewCollector()
	alertEngine := metrics.NewAlertEngine(state.Thresholds)

	fmt.Println("  Components initialized")

	// Pre-flight port check
	fmt.Printf("  Checking port %d availability...\n", *port)
	if !instance.IsPortAvailable(*port) {
		// Port occupied but no valid instance found
		procPID, _ := instance.GetProcessUsingPort(*port)
		fmt.Fprintf(os.Stderr, "\n  ERROR: Port %d is in use by process %d\n", *port, procPID)
		fmt.Fprintf(os.Stderr, "  Try: Use a different port with -port 8080\n")
		os.Exit(1)
	}
	fmt.Println("  Port available ✓")

	// Create server
	srv := server.NewServer(
		store,
		spawner,
		mcpServer,
		metricsCollector,
		alertEngine,
		config,
		projectsConfig,
		memoryDB,
		basePath,
		*port,
	)

	// Setup graceful shutdown
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	// Start server in goroutine
	serverErr := make(chan error, 1)

	go func() {
		serverErr <- srv.Start(fmt.Sprintf(":%d", *port))
	}()

	// Wait for server to bind or fail (poll with health check)
	serverStarted := false
	for i := 0; i < 50; i++ { // 5 second timeout (50 * 100ms)
		time.Sleep(100 * time.Millisecond)

		// Check if server failed
		select {
		case err := <-serverErr:
			fmt.Fprintf(os.Stderr, "Server failed to start: %v\n", err)
			os.Exit(1)
		default:
		}

		// Try health check
		if instance.HealthCheck(*port) == nil {
			serverStarted = true
			break
		}
	}

	if !serverStarted {
		fmt.Fprintf(os.Stderr, "Server failed to become ready within timeout\n")
		os.Exit(1)
	}

	fmt.Printf("  Dashboard ready at http://localhost:%d ✓\n", *port)
	fmt.Println()
	fmt.Println("  The Enrichment Center reminds you that the Weighted Companion Cube")
	fmt.Println("  will never threaten to stab you and, in fact, cannot speak.")

	// Write PID file NOW (after confirmed bind)
	if err := instanceMgr.WritePIDFile(os.Getpid(), *port, basePath); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to write PID file: %v\n", err)
	}

	// Convert config to map for Captain
	configMap := make(map[string]types.AgentConfig)
	for _, agent := range config.Agents {
		configMap[agent.Name] = agent
	}

	// Create cancellable context for Captain
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize and start Captain orchestrator (background task processor)
	captainOrchestrator := captain.NewCaptain(basePath, spawner, memoryDB, configMap)
	go captainOrchestrator.Run(ctx)
	fmt.Println("  Captain orchestrator started")

	// Initialize and start Captain Supervisor (manages interactive Captain terminal)
	captainSupervisor := captain.NewCaptainSupervisor(captain.SupervisorConfig{
		BasePath:   basePath,
		ServerPort: *port,
	})

	// Wire supervisor to server for API endpoints
	srv.SetCaptainSupervisor(captainSupervisor)

	// Wire Captain clean exit to server shutdown
	captainSupervisor.SetShutdownCallback(func() {
		// Signal the server to shut down when Captain exits cleanly
		srv.RequestShutdown()
	})

	// Start Captain in separate terminal
	fmt.Println()
	fmt.Println("  Starting Captain (separate terminal)...")
	if err := captainSupervisor.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to start Captain terminal: %v\n", err)
		fmt.Println("  You can manually restart Captain from the dashboard")
	} else {
		fmt.Println("  Captain terminal spawned ✓")
	}
	fmt.Println()

	// Wait for shutdown signal, API shutdown request, Captain clean exit, or server error
	select {
	case err := <-serverErr:
		if err != nil && err.Error() != "http: Server closed" {
			fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		}
	case <-shutdown:
		fmt.Println()
		fmt.Println("Shutting down (signal received)...")
	case <-srv.ShutdownChan:
		fmt.Println()
		fmt.Println("Shutting down (API request)...")
	case <-captainSupervisor.ShutdownChan():
		fmt.Println()
		fmt.Println("Shutting down (Captain exited cleanly)...")
	}

	// Graceful shutdown (cancel Captain context first)
	cancel()

	// Stop Captain supervisor (kills Captain terminal if still running)
	fmt.Println("Stopping Captain...")
	if err := captainSupervisor.Stop(); err != nil {
		fmt.Printf("  Note: Captain may have already exited: %v\n", err)
	}

	// Wait for graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	// Stop all agents - use PIDs from store since spawner may not track them correctly
	fmt.Println("Stopping agents...")
	currentState := store.GetState()
	for agentID, agent := range currentState.Agents {
		if agent.PID > 0 {
			// Try to kill the process directly using PID from store
			if err := instance.KillProcess(agent.PID); err != nil {
				// Process may already be dead, that's okay
				fmt.Printf("  Note: Agent %s (PID %d) may have already exited\n", agentID, agent.PID)
			} else {
				fmt.Printf("  Stopped agent %s (PID %d)\n", agentID, agent.PID)
			}
		}
	}

	// Cleanup all generated config and prompt files
	fmt.Println("Cleaning up agent files...")
	spawner.CleanupAllAgentFiles()

	// Clear all agents from state (they were killed above)
	for agentID := range currentState.Agents {
		store.RemoveAgent(agentID)
	}

	// Remove PID file BEFORE shutting down server
	fmt.Println("Removing PID file...")
	instanceMgr.RemovePIDFile()

	// Shutdown server
	fmt.Println("Shutting down HTTP server...")
	if err := srv.Shutdown(shutdownCtx); err != nil {
		fmt.Fprintf(os.Stderr, "Shutdown error: %v\n", err)
	}

	// Final save
	fmt.Println("Saving state...")
	if err := store.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save state: %v\n", err)
	}

	fmt.Println("Goodbye!")
}

// getBasePath returns the directory containing the executable,
// or the current working directory if running via `go run`
func getBasePath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return os.Getwd()
	}

	// Check if running from temp directory (go run)
	dir := filepath.Dir(exe)
	if filepath.Base(dir) == "exe" || filepath.Base(filepath.Dir(dir)) == "go-build" {
		return os.Getwd()
	}

	// Check if running from bin/ subdirectory - go up one level to project root
	if filepath.Base(dir) == "bin" {
		return filepath.Dir(dir), nil
	}

	return dir, nil
}

// showInstanceStatus displays information about the running instance
func showInstanceStatus(statePath string, port int) {
	basePath, _ := getBasePath()
	pidPath := filepath.Join(basePath, "data", "cliaimonitor.pid")
	mgr := instance.NewManager(pidPath, statePath, port)
	info, err := mgr.CheckExistingInstance()

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}

	if info == nil {
		fmt.Println("No CLIAIMONITOR instance is currently running")
		return
	}

	// Display formatted instance information
	fmt.Println()
	fmt.Println("╔═══════════════════════════════════════════════════════╗")
	fmt.Println("║         CLIAIMONITOR Instance Status                  ║")
	fmt.Println("╚═══════════════════════════════════════════════════════╝")
	fmt.Println()

	statusIcon := "✓"
	if !info.IsResponding {
		statusIcon = "✗"
	}

	fmt.Printf("Instance:    %s RUNNING\n", statusIcon)
	fmt.Printf("  PID:       %d\n", info.PID)
	fmt.Printf("  Port:      %d\n", info.Port)
	fmt.Printf("  Started:   %s (%s ago)\n",
		info.StartTime.Format("2006-01-02 15:04:05"),
		time.Since(info.StartTime).Round(time.Second))
	fmt.Printf("  Dashboard: http://localhost:%d\n", info.Port)
	fmt.Printf("  Health:    ")
	if info.IsResponding {
		fmt.Println("OK (responding)")
	} else {
		fmt.Println("DEGRADED (not responding)")
	}
	fmt.Println()

	// Load state for agent info
	store := persistence.NewJSONStore(statePath)
	state, err := store.Load()
	if err == nil && state != nil {
		activeAgents := 0
		for _, agent := range state.Agents {
			if agent.Status != types.StatusDisconnected {
				activeAgents++
			}
		}

		fmt.Printf("Active Agents: %d of %d\n", activeAgents, len(state.Agents))
		for _, agent := range state.Agents {
			if agent.Status != types.StatusDisconnected {
				fmt.Printf("  - %s (PID %d): %s\n", agent.ID, agent.PID, agent.Status)
			}
		}
		fmt.Println()

		activeAlerts := 0
		for _, alert := range state.Alerts {
			if !alert.Acknowledged {
				activeAlerts++
			}
		}
		if len(state.Alerts) > 0 {
			fmt.Printf("Alerts: %d unacknowledged of %d total\n", activeAlerts, len(state.Alerts))
			fmt.Println()
		}
	}

	fmt.Println("Actions:")
	fmt.Printf("  View dashboard:  http://localhost:%d\n", info.Port)
	fmt.Printf("  Stop instance:   cliaimonitor.exe -stop\n")
	fmt.Printf("  Force kill:      cliaimonitor.exe -force-stop\n")
	fmt.Println()
}

// stopInstance stops the running instance
func stopInstance(statePath string, force bool) {
	basePath, _ := getBasePath()
	pidPath := filepath.Join(basePath, "data", "cliaimonitor.pid")
	mgr := instance.NewManager(pidPath, statePath, 0)
	info, err := mgr.CheckExistingInstance()

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if info == nil {
		fmt.Println("No CLIAIMONITOR instance is currently running")
		return
	}

	if force {
		fmt.Printf("Force killing process %d...\n", info.PID)
		if err := instance.KillProcess(info.PID); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to kill process: %v\n", err)
			os.Exit(1)
		}
		time.Sleep(1 * time.Second)
		mgr.RemovePIDFile()
		fmt.Println("Instance terminated ✓")
	} else {
		fmt.Printf("Sending graceful shutdown request to instance on port %d...\n", info.Port)
		if err := instance.SendShutdownRequest(info.Port); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to send shutdown request: %v\n", err)
			fmt.Println("Try using -force-stop to force kill the process")
			os.Exit(1)
		}

		// Wait for process to exit
		fmt.Println("Waiting for graceful shutdown...")
		if instance.WaitForPortToBeAvailable(info.Port, 5*time.Second) {
			fmt.Println("Instance stopped successfully ✓")
		} else {
			fmt.Println("Warning: Instance may still be running")
			fmt.Println("Try: cliaimonitor.exe -force-stop")
		}
	}
}

func printBanner() {
	fmt.Println()
	fmt.Println("  ╔═══════════════════════════════════════════════════════╗")
	fmt.Println("  ║                                                       ║")
	fmt.Println("  ║              CLIAIMONITOR v1.0.0                       ║")
	fmt.Println("  ║       AI Agent Supervisor Dashboard                   ║")
	fmt.Println("  ║                                                       ║")
	fmt.Println("  ╚═══════════════════════════════════════════════════════╝")
	fmt.Println()
}
