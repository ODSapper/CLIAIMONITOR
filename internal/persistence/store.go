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

	// Metrics operations
	UpdateMetrics(agentID string, metrics *types.AgentMetrics)
	GetMetrics(agentID string) *types.AgentMetrics
	TakeMetricsSnapshot()

	// Human input operations
	AddHumanRequest(req *types.HumanInputRequest)
	AnswerHumanRequest(id string, answer string)
	GetPendingRequests() []*types.HumanInputRequest

	// Alert operations
	AddAlert(alert *types.Alert)
	AcknowledgeAlert(id string)
	GetActiveAlerts() []*types.Alert

	// Activity log
	AddActivity(activity *types.ActivityLog)

	// Judgment operations
	AddJudgment(judgment *types.SupervisorJudgment)

	// Counter for agent naming
	GetNextAgentNumber(configName string) int

	// Supervisor status
	SetSupervisorConnected(connected bool)
	IsSupervisorConnected() bool

	// Human checkin
	RecordHumanCheckin()
	GetLastHumanCheckin() time.Time

	// Thresholds
	SetThresholds(thresholds types.AlertThresholds)
	GetThresholds() types.AlertThresholds
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

// UpdateMetrics updates agent metrics
func (s *JSONStore) UpdateMetrics(agentID string, metrics *types.AgentMetrics) {
	s.mu.Lock()
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

// SetSupervisorConnected updates supervisor status
func (s *JSONStore) SetSupervisorConnected(connected bool) {
	s.mu.Lock()
	s.state.SupervisorConnected = connected
	s.mu.Unlock()
	s.scheduleSave()
}

// IsSupervisorConnected returns supervisor connection status
func (s *JSONStore) IsSupervisorConnected() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state.SupervisorConnected
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
