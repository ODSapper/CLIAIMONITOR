package aider

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

// Agent represents a running Aider process
type Agent struct {
	ID          string
	ProjectPath string
	Model       string
	Bridge      *Bridge
	Process     *os.Process
	cmd         *exec.Cmd
	StartedAt   time.Time
}

// Spawner manages Aider CLI processes
type Spawner struct {
	natsConn *nats.Conn
	config   *Config
	agents   map[string]*Agent
	mu       sync.RWMutex
	stopCh   chan struct{}
	wg       sync.WaitGroup
}

// NewSpawner creates a new Aider process spawner
func NewSpawner(nc *nats.Conn, config *Config) *Spawner {
	s := &Spawner{
		natsConn: nc,
		config:   config,
		agents:   make(map[string]*Agent),
		stopCh:   make(chan struct{}),
	}

	// Start agent monitor
	s.wg.Add(1)
	go s.monitorAgents()

	return s
}

// SpawnAgent spawns a new Aider process with the given configuration
func (s *Spawner) SpawnAgent(agentConfig AgentConfig) (*Agent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Generate unique agent ID
	agentID := fmt.Sprintf("aider-%s", uuid.New().String()[:8])

	// Validate project path
	if agentConfig.ProjectPath == "" {
		return nil, fmt.Errorf("project path is required")
	}

	if _, err := os.Stat(agentConfig.ProjectPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("project path does not exist: %s", agentConfig.ProjectPath)
	}

	// Build Aider command with Qwen model
	cmd := exec.Command("aider",
		"--model", "ollama/qwen2.5-coder:32b",
		"--no-auto-commits",
		"--edit-format", "diff",
	)

	// Set working directory
	cmd.Dir = agentConfig.ProjectPath

	// Create pipes for stdin/stdout/stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the process
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start aider: %w", err)
	}

	log.Printf("[SPAWNER] Started Aider process (PID: %d) for agent %s", cmd.Process.Pid, agentID)

	// Create bridge
	bridge := NewBridge(agentID, s.natsConn, stdin, stdout, stderr)
	if err := bridge.Start(); err != nil {
		// Kill the process if bridge fails
		cmd.Process.Kill()
		return nil, fmt.Errorf("failed to start bridge: %w", err)
	}

	// Create agent record
	agent := &Agent{
		ID:          agentID,
		ProjectPath: agentConfig.ProjectPath,
		Model:       "ollama/qwen2.5-coder:32b",
		Bridge:      bridge,
		Process:     cmd.Process,
		cmd:         cmd,
		StartedAt:   time.Now(),
	}

	// Track agent
	s.agents[agentID] = agent

	log.Printf("[SPAWNER] Agent %s spawned successfully (project: %s)", agentID, agentConfig.ProjectPath)

	return agent, nil
}

// StopAgent gracefully stops an Aider agent
func (s *Spawner) StopAgent(agentID string) error {
	s.mu.Lock()
	agent, exists := s.agents[agentID]
	if !exists {
		s.mu.Unlock()
		return fmt.Errorf("agent %s not found", agentID)
	}
	delete(s.agents, agentID)
	s.mu.Unlock()

	log.Printf("[SPAWNER] Stopping agent %s (PID: %d)", agentID, agent.Process.Pid)

	// Stop bridge first
	agent.Bridge.Stop()

	// Send graceful shutdown via Aider's /quit command
	agent.Bridge.SendCommand("/quit")

	// Wait for process to exit gracefully (with timeout)
	done := make(chan error, 1)
	go func() {
		done <- agent.cmd.Wait()
	}()

	select {
	case err := <-done:
		if err != nil {
			log.Printf("[SPAWNER] Agent %s exited with error: %v", agentID, err)
		} else {
			log.Printf("[SPAWNER] Agent %s stopped gracefully", agentID)
		}
		return nil

	case <-time.After(5 * time.Second):
		// Graceful shutdown timed out, send SIGTERM
		log.Printf("[SPAWNER] Agent %s did not exit gracefully, sending SIGTERM", agentID)
		if err := agent.Process.Signal(syscall.SIGTERM); err != nil {
			log.Printf("[SPAWNER] Failed to send SIGTERM to agent %s: %v", agentID, err)
		}

		// Wait another 3 seconds
		select {
		case <-done:
			log.Printf("[SPAWNER] Agent %s stopped after SIGTERM", agentID)
			return nil
		case <-time.After(3 * time.Second):
			// SIGTERM timeout, force kill
			log.Printf("[SPAWNER] Agent %s did not respond to SIGTERM, force killing", agentID)
			if err := agent.Process.Kill(); err != nil {
				return fmt.Errorf("failed to kill agent %s: %w", agentID, err)
			}
			<-done // Wait for process to be reaped
			log.Printf("[SPAWNER] Agent %s force killed", agentID)
			return nil
		}
	}
}

// GetAgent retrieves an agent by ID
func (s *Spawner) GetAgent(agentID string) *Agent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.agents[agentID]
}

// ListAgents returns a list of all running agents
func (s *Spawner) ListAgents() []*Agent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	agents := make([]*Agent, 0, len(s.agents))
	for _, agent := range s.agents {
		agents = append(agents, agent)
	}
	return agents
}

// StopAll gracefully stops all running agents
func (s *Spawner) StopAll() {
	log.Println("[SPAWNER] Stopping all agents...")

	// Signal monitor to stop
	close(s.stopCh)

	// Get all agent IDs
	s.mu.RLock()
	agentIDs := make([]string, 0, len(s.agents))
	for id := range s.agents {
		agentIDs = append(agentIDs, id)
	}
	s.mu.RUnlock()

	// Stop each agent
	var wg sync.WaitGroup
	for _, id := range agentIDs {
		wg.Add(1)
		go func(agentID string) {
			defer wg.Done()
			if err := s.StopAgent(agentID); err != nil {
				log.Printf("[SPAWNER] Error stopping agent %s: %v", agentID, err)
			}
		}(id)
	}

	// Wait for all agents to stop
	wg.Wait()

	// Wait for monitor goroutine to exit
	s.wg.Wait()

	log.Println("[SPAWNER] All agents stopped")
}

// monitorAgents monitors running agents and detects crashed processes
func (s *Spawner) monitorAgents() {
	defer s.wg.Done()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return

		case <-ticker.C:
			s.checkAgents()
		}
	}
}

// checkAgents checks the health of all running agents
func (s *Spawner) checkAgents() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for id, agent := range s.agents {
		// Check if process is still running
		if !s.isProcessRunning(agent.Process) {
			log.Printf("[SPAWNER] Agent %s (PID: %d) has crashed or exited unexpectedly", id, agent.Process.Pid)

			// Stop bridge
			agent.Bridge.Stop()

			// Remove from tracking
			delete(s.agents, id)

			// Publish crash notification
			s.publishCrash(id, agent)
		}
	}
}

// isProcessRunning checks if a process is still running
func (s *Spawner) isProcessRunning(process *os.Process) bool {
	// On Windows, we can try to send signal 0 (null signal) to check if process exists
	// This works on Unix-like systems; on Windows we need a different approach
	err := process.Signal(syscall.Signal(0))
	return err == nil
}

// publishCrash publishes a crash notification to NATS
func (s *Spawner) publishCrash(agentID string, agent *Agent) {
	crashMsg := map[string]interface{}{
		"agent_id":  agentID,
		"status":    "crashed",
		"message":   fmt.Sprintf("Aider process crashed (PID: %d, uptime: %s)", agent.Process.Pid, time.Since(agent.StartedAt)),
		"timestamp": time.Now(),
	}

	subject := fmt.Sprintf("agent.%s.status", agentID)

	// Publish to NATS (marshaling is handled internally)
	data, err := json.Marshal(crashMsg)
	if err != nil {
		log.Printf("[SPAWNER] Failed to marshal crash notification: %v", err)
		return
	}

	if err := s.natsConn.Publish(subject, data); err != nil {
		log.Printf("[SPAWNER] Failed to publish crash notification: %v", err)
	}
}
