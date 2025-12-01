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
	"github.com/CLIAIMONITOR/internal/mcp"
	"github.com/CLIAIMONITOR/internal/metrics"
	"github.com/CLIAIMONITOR/internal/persistence"
	"github.com/CLIAIMONITOR/internal/server"
	"github.com/CLIAIMONITOR/internal/types"
)

func main() {
	// Parse command line flags
	port := flag.Int("port", 3000, "HTTP server port")
	configPath := flag.String("config", "configs/teams.yaml", "Team configuration file")
	projectsPath := flag.String("projects", "configs/projects.yaml", "Projects configuration file")
	statePath := flag.String("state", "data/state.json", "State persistence file")
	noSupervisor := flag.Bool("no-supervisor", false, "Don't auto-spawn supervisor")
	mcpHost := flag.String("mcp-host", "localhost", "MCP server hostname (for agents to connect)")
	flag.Parse()

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
	mcpServerURL := fmt.Sprintf("http://%s:%d/mcp/sse", *mcpHost, *port)
	spawner := agents.NewSpawner(basePath, mcpServerURL)
	mcpServer := mcp.NewServer()
	metricsCollector := metrics.NewCollector()
	alertEngine := metrics.NewAlertEngine(state.Thresholds)

	fmt.Println("  Components initialized")

	// Create server
	srv := server.NewServer(
		store,
		spawner,
		mcpServer,
		metricsCollector,
		alertEngine,
		config,
		projectsConfig,
		basePath,
	)

	// Setup graceful shutdown
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	// Start server in goroutine
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- srv.Start(fmt.Sprintf(":%d", *port))
	}()

	// Wait a moment for server to start
	time.Sleep(500 * time.Millisecond)

	// Spawn supervisor unless disabled
	if !*noSupervisor {
		fmt.Println()
		fmt.Println("  Spawning Supervisor agent...")

		pid, err := spawner.SpawnSupervisor(config.Supervisor)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  Warning: Failed to spawn supervisor: %v\n", err)
		} else {
			// Add supervisor to state
			supervisor := &types.Agent{
				ID:          "Supervisor",
				ConfigName:  config.Supervisor.Name,
				Role:        types.RoleSupervisor,
				Model:       config.Supervisor.Model,
				Color:       config.Supervisor.Color,
				Status:      types.StatusStarting,
				PID:         pid,
				ProjectPath: basePath,
				SpawnedAt:   time.Now(),
				LastSeen:    time.Now(),
			}
			store.AddAgent(supervisor)
			fmt.Printf("  Supervisor spawned (PID: %d)\n", pid)
		}
	}

	fmt.Println()
	fmt.Println("  Press Ctrl+C to shutdown")
	fmt.Println()

	// Wait for shutdown signal or server error
	select {
	case err := <-serverErr:
		if err != nil {
			fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		}
	case <-shutdown:
		fmt.Println()
		fmt.Println("Shutting down...")
	}

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Stop all agents
	fmt.Println("Stopping agents...")
	for agentID := range store.GetState().Agents {
		if err := spawner.StopAgent(agentID); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to stop %s: %v\n", agentID, err)
		}
	}

	// Cleanup all generated config and prompt files
	spawner.CleanupAllAgentFiles()

	// Shutdown server
	if err := srv.Shutdown(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Shutdown error: %v\n", err)
	}

	// Final save
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

	return dir, nil
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
