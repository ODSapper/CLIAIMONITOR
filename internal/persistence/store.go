package persistence

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/CLIAIMONITOR/internal/types"
)

// Store interface for state persistence
type Store interface {
	Load() (*types.DashboardState, error)
	Save() error
	GetState() *types.DashboardState
	ResetMetricsHistory() error

	// Agent operations
	AddAgent(agent *types.Agent)
	UpdateAgent(agentID string, updater func(*types.Agent))
	RemoveAgent(agentID string)
	GetAgent(agentID string) *types.Agent
	RequestAgentShutdown(agentID string, requestTime time.Time)

	// Metrics operations
	UpdateMetrics(agentID string, metrics *types.AgentMetrics)
	GetMetrics(agentID string) *types.AgentMetrics
	TakeMetricsSnapshot()

	// Human input operations
	AddHumanRequest(req *types.HumanInputRequest)
	AnswerHumanRequest(id string, answer string)
	GetPendingRequests() []*types.HumanInputRequest

	// Stop approval operations
	AddStopRequest(req *types.StopApprovalRequest)
	RespondStopRequest(id string, approved bool, response string, reviewedBy string)
	GetPendingStopRequests() []*types.StopApprovalRequest
	GetStopRequestByID(id string) *types.StopApprovalRequest

	// Captain messages (human -> Captain)
	AddCaptainMessage(msg *types.CaptainMessage)
	GetUnreadCaptainMessages() []*types.CaptainMessage
	MarkCaptainMessagesRead(ids []string)

	// Alert operations
	AddAlert(alert *types.Alert)
	AcknowledgeAlert(id string)
	ClearAllAlerts()
	GetActiveAlerts() []*types.Alert

	// Activity log
	AddActivity(activity *types.ActivityLog)

	// Judgment operations
	AddJudgment(judgment *types.SupervisorJudgment)

	// Counter for agent naming
	GetNextAgentNumber(configName string) int

	// Human checkin
	RecordHumanCheckin()
	GetLastHumanCheckin() time.Time

	// Thresholds
	SetThresholds(thresholds types.AlertThresholds)
	GetThresholds() types.AlertThresholds

	// Cleanup
	CleanupStaleAgents() int

	// Captain state
	SetCaptainConnected(connected bool)
	SetCaptainStatus(status string)
}

// JSONStore implements Store with JSON file persistence
type JSONStore struct {
	mu       sync.RWMutex
	filepath string
	state    *types.DashboardState

	// Debounced save
	saveTimer *time.Timer
	saveMu    sync.Mutex
}

// NewJSONStore creates a new JSON-backed store
func NewJSONStore(filepath string) *JSONStore {
	return &JSONStore{
		filepath: filepath,
		state:    types.NewDashboardState(),
	}
}

// Load reads state from JSON file
func (s *JSONStore) Load() (*types.DashboardState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Ensure directory exists
	dir := filepath.Dir(s.filepath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	data, err := os.ReadFile(s.filepath)
	if err != nil {
		if os.IsNotExist(err) {
			// Return default state if file doesn't exist
			s.state = types.NewDashboardState()
			return s.state, nil
		}
		return nil, err
	}

	var state types.DashboardState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}

	// Initialize maps if nil
	if state.Agents == nil {
		state.Agents = make(map[string]*types.Agent)
	}
	if state.Metrics == nil {
		state.Metrics = make(map[string]*types.AgentMetrics)
	}
	if state.HumanRequests == nil {
		state.HumanRequests = make(map[string]*types.HumanInputRequest)
	}
	if state.AgentCounters == nil {
		state.AgentCounters = make(map[string]int)
	}
	// Initialize SessionStats if not set
	if state.SessionStats.SessionStartedAt.IsZero() {
		state.SessionStats.SessionStartedAt = time.Now()
	}

	s.state = &state
	return s.state, nil
}

// Save writes state to JSON file
func (s *JSONStore) Save() error {
	s.mu.RLock()
	data, err := json.MarshalIndent(s.state, "", "  ")
	s.mu.RUnlock()

	if err != nil {
		return err
	}

	return os.WriteFile(s.filepath, data, 0644)
}

// scheduleSave debounces save operations
func (s *JSONStore) scheduleSave() {
	s.saveMu.Lock()
	defer s.saveMu.Unlock()

	if s.saveTimer != nil {
		s.saveTimer.Stop()
	}

	s.saveTimer = time.AfterFunc(500*time.Millisecond, func() {
		s.Save()
	})
}

// GetState returns current state (read-only snapshot)
func (s *JSONStore) GetState() *types.DashboardState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

// ResetMetricsHistory clears historical metrics
func (s *JSONStore) ResetMetricsHistory() error {
	s.mu.Lock()
	s.state.MetricsHistory = []types.MetricsSnapshot{}
	s.mu.Unlock()
	s.scheduleSave()
	return nil
}

// AddAgent adds a new agent to state
func (s *JSONStore) AddAgent(agent *types.Agent) {
	s.mu.Lock()
	s.state.Agents[agent.ID] = agent
	// Increment total agents spawned
	s.state.SessionStats.TotalAgentsSpawned++
	s.mu.Unlock()
	s.scheduleSave()
}

// UpdateAgent modifies an existing agent
func (s *JSONStore) UpdateAgent(agentID string, updater func(*types.Agent)) {
	s.mu.Lock()
	if agent, exists := s.state.Agents[agentID]; exists {
		updater(agent)
	}
	s.mu.Unlock()
	s.scheduleSave()
}

// RemoveAgent removes an agent from state
func (s *JSONStore) RemoveAgent(agentID string) {
	s.mu.Lock()
	delete(s.state.Agents, agentID)
	delete(s.state.Metrics, agentID)
	s.mu.Unlock()
	s.scheduleSave()
}

// GetAgent returns agent by ID
func (s *JSONStore) GetAgent(agentID string) *types.Agent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state.Agents[agentID]
}

// RequestAgentShutdown marks an agent for graceful shutdown
func (s *JSONStore) RequestAgentShutdown(agentID string, requestTime time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if agent, ok := s.state.Agents[agentID]; ok {
		agent.ShutdownRequested = true
		agent.ShutdownRequestedAt = &requestTime
		agent.Status = types.StatusStopping
		s.state.Agents[agentID] = agent
		s.scheduleSave()
	}
}

// UpdateMetrics updates agent metrics
func (s *JSONStore) UpdateMetrics(agentID string, metrics *types.AgentMetrics) {
	s.mu.Lock()
	// Calculate delta for token usage
	oldMetrics := s.state.Metrics[agentID]
	if oldMetrics != nil {
		tokenDelta := metrics.TokensUsed - oldMetrics.TokensUsed
		costDelta := metrics.EstimatedCost - oldMetrics.EstimatedCost
		if tokenDelta > 0 {
			s.state.SessionStats.TotalTokensUsed += tokenDelta
		}
		if costDelta > 0 {
			s.state.SessionStats.TotalEstimatedCost += costDelta
		}
	} else {
		// First time metrics for this agent
		s.state.SessionStats.TotalTokensUsed += metrics.TokensUsed
		s.state.SessionStats.TotalEstimatedCost += metrics.EstimatedCost
	}
	s.state.Metrics[agentID] = metrics
	s.mu.Unlock()
	s.scheduleSave()
}

// GetMetrics returns metrics for an agent
func (s *JSONStore) GetMetrics(agentID string) *types.AgentMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state.Metrics[agentID]
}

// TakeMetricsSnapshot saves current metrics to history
func (s *JSONStore) TakeMetricsSnapshot() {
	s.mu.Lock()
	snapshot := types.MetricsSnapshot{
		Timestamp: time.Now(),
		Agents:    make(map[string]*types.AgentMetrics),
	}
	for id, m := range s.state.Metrics {
		copy := *m
		snapshot.Agents[id] = &copy
	}
	s.state.MetricsHistory = append(s.state.MetricsHistory, snapshot)

	// Keep only last 1000 snapshots
	if len(s.state.MetricsHistory) > 1000 {
		s.state.MetricsHistory = s.state.MetricsHistory[len(s.state.MetricsHistory)-1000:]
	}
	s.mu.Unlock()
	s.scheduleSave()
}

// GetNextAgentNumber returns next number for agent naming
func (s *JSONStore) GetNextAgentNumber(configName string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.state.AgentCounters[configName]++
	num := s.state.AgentCounters[configName]
	s.scheduleSave()
	return num
}

// AddHumanRequest adds a human input request
func (s *JSONStore) AddHumanRequest(req *types.HumanInputRequest) {
	s.mu.Lock()
	s.state.HumanRequests[req.ID] = req
	s.mu.Unlock()
	s.scheduleSave()
}

// AnswerHumanRequest marks request as answered
func (s *JSONStore) AnswerHumanRequest(id string, answer string) {
	s.mu.Lock()
	if req, exists := s.state.HumanRequests[id]; exists {
		req.Answered = true
		req.Answer = answer
	}
	s.mu.Unlock()
	s.scheduleSave()
}

// GetPendingRequests returns unanswered requests
func (s *JSONStore) GetPendingRequests() []*types.HumanInputRequest {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var pending []*types.HumanInputRequest
	for _, req := range s.state.HumanRequests {
		if !req.Answered {
			pending = append(pending, req)
		}
	}
	return pending
}

// AddStopRequest adds a stop approval request
func (s *JSONStore) AddStopRequest(req *types.StopApprovalRequest) {
	s.mu.Lock()
	if s.state.StopRequests == nil {
		s.state.StopRequests = make(map[string]*types.StopApprovalRequest)
	}
	s.state.StopRequests[req.ID] = req
	s.mu.Unlock()
	s.scheduleSave()
}

// RespondStopRequest marks a stop request as reviewed
func (s *JSONStore) RespondStopRequest(id string, approved bool, response string, reviewedBy string) {
	s.mu.Lock()
	if req, exists := s.state.StopRequests[id]; exists {
		req.Reviewed = true
		req.Approved = approved
		req.Response = response
		req.ReviewedBy = reviewedBy
		// If approved, increment completed tasks
		if approved && req.Reason == "task_complete" {
			s.state.SessionStats.CompletedTasks++
		}
	}
	s.mu.Unlock()
	s.scheduleSave()
}

// GetPendingStopRequests returns unreviewed stop requests
func (s *JSONStore) GetPendingStopRequests() []*types.StopApprovalRequest {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var pending []*types.StopApprovalRequest
	for _, req := range s.state.StopRequests {
		if !req.Reviewed {
			pending = append(pending, req)
		}
	}
	return pending
}

// GetStopRequestByID returns a stop request by ID
func (s *JSONStore) GetStopRequestByID(id string) *types.StopApprovalRequest {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state.StopRequests[id]
}

// AddCaptainMessage adds a message from human to Captain
func (s *JSONStore) AddCaptainMessage(msg *types.CaptainMessage) {
	s.mu.Lock()
	if s.state.CaptainMessages == nil {
		s.state.CaptainMessages = []*types.CaptainMessage{}
	}
	s.state.CaptainMessages = append(s.state.CaptainMessages, msg)
	s.mu.Unlock()
	s.scheduleSave()
}

// GetUnreadCaptainMessages returns messages Captain hasn't read yet
func (s *JSONStore) GetUnreadCaptainMessages() []*types.CaptainMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var unread []*types.CaptainMessage
	for _, msg := range s.state.CaptainMessages {
		if !msg.Read {
			unread = append(unread, msg)
		}
	}
	return unread
}

// MarkCaptainMessagesRead marks specified messages as read
func (s *JSONStore) MarkCaptainMessagesRead(ids []string) {
	s.mu.Lock()
	idSet := make(map[string]bool)
	for _, id := range ids {
		idSet[id] = true
	}
	for _, msg := range s.state.CaptainMessages {
		if idSet[msg.ID] {
			msg.Read = true
		}
	}
	s.mu.Unlock()
	s.scheduleSave()
}

// AddAlert adds a new alert
func (s *JSONStore) AddAlert(alert *types.Alert) {
	s.mu.Lock()
	s.state.Alerts = append(s.state.Alerts, alert)
	s.mu.Unlock()
	s.scheduleSave()
}

// AcknowledgeAlert marks alert as acknowledged
func (s *JSONStore) AcknowledgeAlert(id string) {
	s.mu.Lock()
	for _, alert := range s.state.Alerts {
		if alert.ID == id {
			alert.Acknowledged = true
			break
		}
	}
	s.mu.Unlock()
	s.scheduleSave()
}

// ClearAllAlerts marks all alerts as acknowledged
func (s *JSONStore) ClearAllAlerts() {
	s.mu.Lock()
	for _, alert := range s.state.Alerts {
		alert.Acknowledged = true
	}
	s.mu.Unlock()
	s.scheduleSave()
}

// GetActiveAlerts returns unacknowledged alerts
func (s *JSONStore) GetActiveAlerts() []*types.Alert {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var active []*types.Alert
	for _, alert := range s.state.Alerts {
		if !alert.Acknowledged {
			active = append(active, alert)
		}
	}
	return active
}

// AddActivity adds activity log entry
func (s *JSONStore) AddActivity(activity *types.ActivityLog) {
	s.mu.Lock()
	s.state.ActivityLog = append(s.state.ActivityLog, activity)

	// Keep only last 500 entries
	if len(s.state.ActivityLog) > 500 {
		s.state.ActivityLog = s.state.ActivityLog[len(s.state.ActivityLog)-500:]
	}
	s.mu.Unlock()
	s.scheduleSave()
}

// AddJudgment records a supervisor judgment
func (s *JSONStore) AddJudgment(judgment *types.SupervisorJudgment) {
	s.mu.Lock()
	s.state.Judgments = append(s.state.Judgments, judgment)
	s.mu.Unlock()
	s.scheduleSave()
}

// RecordHumanCheckin updates last checkin time
func (s *JSONStore) RecordHumanCheckin() {
	s.mu.Lock()
	s.state.LastHumanCheckin = time.Now()
	s.mu.Unlock()
	s.scheduleSave()
}

// GetLastHumanCheckin returns last checkin time
func (s *JSONStore) GetLastHumanCheckin() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state.LastHumanCheckin
}

// SetThresholds updates alert thresholds
func (s *JSONStore) SetThresholds(thresholds types.AlertThresholds) {
	s.mu.Lock()
	s.state.Thresholds = thresholds
	s.mu.Unlock()
	s.scheduleSave()
}

// GetThresholds returns current thresholds
func (s *JSONStore) GetThresholds() types.AlertThresholds {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state.Thresholds
}

// CleanupStaleAgents removes disconnected agents where process is not running
func (s *JSONStore) CleanupStaleAgents() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	removedCount := 0
	for agentID, agent := range s.state.Agents {
		// Only consider disconnected agents
		if agent.Status == types.StatusDisconnected && agent.PID > 0 {
			// Check if process still exists
			process, err := os.FindProcess(agent.PID)
			if err != nil {
				// Process not found, remove agent
				delete(s.state.Agents, agentID)
				delete(s.state.Metrics, agentID)
				removedCount++
				continue
			}

			// On Windows, FindProcess always succeeds, so we need to send signal 0
			// to check if process is actually running
			err = process.Signal(os.Signal(nil))
			if err != nil {
				// Process not running, remove agent
				delete(s.state.Agents, agentID)
				delete(s.state.Metrics, agentID)
				removedCount++
			}
		}
	}

	if removedCount > 0 {
		s.scheduleSave()
	}

	return removedCount
}

// SetCaptainConnected updates Captain connection status
func (s *JSONStore) SetCaptainConnected(connected bool) {
	s.mu.Lock()
	s.state.CaptainConnected = connected
	s.mu.Unlock()
	s.scheduleSave()
}

// SetCaptainStatus updates Captain status (idle, busy, error)
func (s *JSONStore) SetCaptainStatus(status string) {
	s.mu.Lock()
	s.state.CaptainStatus = status
	s.mu.Unlock()
	s.scheduleSave()
}
