package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/CLIAIMONITOR/internal/agents"
	"github.com/CLIAIMONITOR/internal/memory"
	"github.com/CLIAIMONITOR/internal/supervisor"
	"github.com/CLIAIMONITOR/internal/types"
	"github.com/gorilla/mux"
)

// CoordinationHandler handles Captain coordination endpoints
type CoordinationHandler struct {
	memDB      memory.MemoryDB
	reconRepo  memory.ReconRepository
	parser     supervisor.ReportParser
	engine     supervisor.DecisionEngine
	dispatcher supervisor.Dispatcher
}

// NewCoordinationHandler creates a new coordination handler
func NewCoordinationHandler(memDB memory.MemoryDB, spawner agents.Spawner, configs map[string]types.AgentConfig) *CoordinationHandler {
	// Try to cast to ReconRepository
	reconRepo, ok := memDB.(memory.ReconRepository)
	if !ok {
		// Use nil if not available - methods will handle gracefully
		reconRepo = nil
	}

	return &CoordinationHandler{
		memDB:      memDB,
		reconRepo:  reconRepo,
		parser:     supervisor.NewReportParser(),
		engine:     supervisor.NewDecisionEngine(memDB),
		dispatcher: supervisor.NewDispatcher(memDB, spawner, configs),
	}
}

// RegisterRoutes registers coordination API routes
func (h *CoordinationHandler) RegisterRoutes(r *mux.Router) {
	r.HandleFunc("/coordination/analyze", h.handleAnalyzeReport).Methods("POST")
	r.HandleFunc("/coordination/dispatch", h.handleDispatch).Methods("POST")
	r.HandleFunc("/coordination/status/{id}", h.handleGetStatus).Methods("GET")
	r.HandleFunc("/coordination/abort/{id}", h.handleAbortDispatch).Methods("POST")
	r.HandleFunc("/coordination/history", h.handleGetHistory).Methods("GET")
	r.HandleFunc("/coordination/plans", h.handleListPlans).Methods("GET")
	r.HandleFunc("/coordination/plans/{id}", h.handleGetPlan).Methods("GET")
}

// handleAnalyzeReport analyzes a Snake reconnaissance report and produces an action plan
func (h *CoordinationHandler) handleAnalyzeReport(w http.ResponseWriter, r *http.Request) {
	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Failed to read request body")
		return
	}
	defer r.Body.Close()

	// Determine content type
	contentType := r.Header.Get("Content-Type")

	var report *supervisor.ReconReport

	// Parse based on content type
	switch contentType {
	case "application/yaml", "text/yaml":
		report, err = h.parser.ParseYAML(body)
	case "application/json":
		report, err = h.parser.ParseJSON(body)
	default:
		// Try JSON first, then YAML
		report, err = h.parser.ParseJSON(body)
		if err != nil {
			report, err = h.parser.ParseYAML(body)
		}
	}

	if err != nil {
		respondError(w, http.StatusBadRequest, "Failed to parse report: "+err.Error())
		return
	}

	// Validate report
	if err := supervisor.ValidateReport(report); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid report: "+err.Error())
		return
	}

	// Store report in recon repository
	if err := h.storeReconReport(report); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to store report: "+err.Error())
		return
	}

	// Analyze report and generate action plan
	plan, err := h.engine.AnalyzeReport(r.Context(), report)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to analyze report: "+err.Error())
		return
	}

	// Store action plan
	if err := h.storeActionPlan(plan); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to store action plan: "+err.Error())
		return
	}

	// Return action plan
	respondJSON(w, map[string]interface{}{
		"report_id": report.ID,
		"plan":      plan,
		"message":   "Report analyzed successfully",
	})
}

// handleDispatch executes an action plan by spawning agents
func (h *CoordinationHandler) handleDispatch(w http.ResponseWriter, r *http.Request) {
	// Limit request size to prevent DoS
	limitRequestSize(r, MaxPayloadSize)

	var req struct {
		PlanID string `json:"plan_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil{
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.PlanID == "" {
		respondError(w, http.StatusBadRequest, "plan_id is required")
		return
	}

	// Retrieve action plan
	plan, err := h.getActionPlan(req.PlanID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Action plan not found: "+err.Error())
		return
	}

	// Check if human approval is required
	if plan.RequiresHuman {
		respondError(w, http.StatusForbidden, "Plan requires human approval: "+plan.EscalationReason)
		return
	}

	// Execute plan
	result, err := h.dispatcher.ExecutePlan(r.Context(), plan)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to execute plan: "+err.Error())
		return
	}

	respondJSON(w, map[string]interface{}{
		"dispatch_id": result.DispatchID,
		"result":      result,
		"message":     "Dispatch initiated successfully",
	})
}

// handleGetStatus retrieves the status of a dispatch
func (h *CoordinationHandler) handleGetStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	dispatchID := vars["id"]

	if dispatchID == "" {
		respondError(w, http.StatusBadRequest, "dispatch_id is required")
		return
	}

	status, err := h.dispatcher.GetDispatchStatus(r.Context(), dispatchID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Dispatch not found: "+err.Error())
		return
	}

	respondJSON(w, status)
}

// handleAbortDispatch aborts a running dispatch
func (h *CoordinationHandler) handleAbortDispatch(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	dispatchID := vars["id"]

	if dispatchID == "" {
		respondError(w, http.StatusBadRequest, "dispatch_id is required")
		return
	}

	if err := h.dispatcher.AbortDispatch(r.Context(), dispatchID); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to abort dispatch: "+err.Error())
		return
	}

	respondJSON(w, map[string]string{
		"status":  "aborted",
		"message": "Dispatch aborted successfully",
	})
}

// handleGetHistory retrieves dispatch history
func (h *CoordinationHandler) handleGetHistory(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	query := r.URL.Query()
	status := query.Get("status")
	limit := 50
	offset := 0

	// Parse limit
	if l := query.Get("limit"); l != "" {
		if parsed, err := parsePositiveInt(l); err == nil {
			limit = parsed
		}
	}

	// Parse offset
	if o := query.Get("offset"); o != "" {
		if parsed, err := parsePositiveInt(o); err == nil {
			offset = parsed
		}
	}

	filter := supervisor.DispatchFilter{
		Status: status,
		Limit:  limit,
		Offset: offset,
	}

	dispatches, err := h.dispatcher.ListDispatches(r.Context(), filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to retrieve dispatches: "+err.Error())
		return
	}

	respondJSON(w, map[string]interface{}{
		"dispatches": dispatches,
		"count":      len(dispatches),
		"limit":      limit,
		"offset":     offset,
	})
}

// handleListPlans retrieves stored action plans
func (h *CoordinationHandler) handleListPlans(w http.ResponseWriter, r *http.Request) {
	// This would query stored plans from memory DB
	// For now, return empty list
	respondJSON(w, map[string]interface{}{
		"plans": []interface{}{},
		"count": 0,
	})
}

// handleGetPlan retrieves a specific action plan
func (h *CoordinationHandler) handleGetPlan(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	planID := vars["id"]

	if planID == "" {
		respondError(w, http.StatusBadRequest, "plan_id is required")
		return
	}

	plan, err := h.getActionPlan(planID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Action plan not found: "+err.Error())
		return
	}

	respondJSON(w, plan)
}

// Helper functions

func (h *CoordinationHandler) storeReconReport(report *supervisor.ReconReport) error {
	// If reconRepo is not available, skip storage but don't error
	if h.reconRepo == nil {
		// Just store as learning for now
		return h.storeAsLearning(report)
	}

	// Convert to memory.Environment and memory.ReconScan
	env := &memory.Environment{
		ID:          report.Environment,
		Name:        report.Environment,
		Description: "Environment from reconnaissance report",
		EnvType:     "customer",
	}

	// Register environment (upsert)
	ctx := context.Background()
	if err := h.reconRepo.RegisterEnvironment(ctx, env); err != nil {
		// Log error but continue - environment might already exist
	}

	// Create scan record
	scan := &memory.ReconScan{
		ID:               report.ID,
		EnvID:            report.Environment,
		AgentID:          report.AgentID,
		ScanType:         report.Mission,
		Mission:          report.Mission,
		StartedAt:        report.Timestamp,
		Status:           "completed",
		TotalFilesScanned: report.Summary.TotalFilesScanned,
		LanguagesDetected: report.Summary.Languages,
		FrameworksDetected: report.Summary.Frameworks,
		SecurityScore:     report.Summary.SecurityScore,
	}

	completedAt := report.Timestamp
	scan.CompletedAt = &completedAt

	// Record scan
	if err := h.reconRepo.RecordScan(ctx, scan); err != nil {
		return err
	}

	// Store findings
	findings := make([]*memory.ReconFinding, 0)

	// Convert critical findings
	for _, f := range report.Findings.Critical {
		finding := &memory.ReconFinding{
			ID:              f.ID,
			ScanID:          report.ID,
			EnvID:           report.Environment,
			FindingType:     f.Type,
			Severity:        "critical",
			Title:           f.Description,
			Description:     f.Description,
			Location:        f.Location,
			Recommendation:  f.Recommendation,
			Status:          "open",
		}
		findings = append(findings, finding)
	}

	// Convert high findings
	for _, f := range report.Findings.High {
		finding := &memory.ReconFinding{
			ID:              f.ID,
			ScanID:          report.ID,
			EnvID:           report.Environment,
			FindingType:     f.Type,
			Severity:        "high",
			Title:           f.Description,
			Description:     f.Description,
			Location:        f.Location,
			Recommendation:  f.Recommendation,
			Status:          "open",
		}
		findings = append(findings, finding)
	}

	// Convert medium and low findings similarly
	for _, f := range report.Findings.Medium {
		finding := &memory.ReconFinding{
			ID:              f.ID,
			ScanID:          report.ID,
			EnvID:           report.Environment,
			FindingType:     f.Type,
			Severity:        "medium",
			Title:           f.Description,
			Description:     f.Description,
			Location:        f.Location,
			Recommendation:  f.Recommendation,
			Status:          "open",
		}
		findings = append(findings, finding)
	}

	for _, f := range report.Findings.Low {
		finding := &memory.ReconFinding{
			ID:              f.ID,
			ScanID:          report.ID,
			EnvID:           report.Environment,
			FindingType:     f.Type,
			Severity:        "low",
			Title:           f.Description,
			Description:     f.Description,
			Location:        f.Location,
			Recommendation:  f.Recommendation,
			Status:          "open",
		}
		findings = append(findings, finding)
	}

	// Save all findings
	if len(findings) > 0 {
		if err := h.reconRepo.SaveFindings(ctx, findings); err != nil {
			return err
		}
	}

	return nil
}

func (h *CoordinationHandler) storeAsLearning(report *supervisor.ReconReport) error {
	// Store report as agent learning
	reportJSON, err := h.marshalToJSON(report)
	if err != nil {
		return err
	}

	learning := &memory.AgentLearning{
		AgentID:  report.AgentID,
		Category: "reconnaissance",
		Title:    fmt.Sprintf("Recon: %s - %s", report.Environment, report.Mission),
		Content:  reportJSON,
	}

	return h.memDB.StoreAgentLearning(learning)
}

func (h *CoordinationHandler) storeActionPlan(plan *supervisor.ActionPlan) error {
	// Store action plan as agent learning for now
	// In production, this would use a dedicated action plan storage
	planJSON, err := h.marshalToJSON(plan)
	if err != nil {
		return err
	}

	learning := &memory.AgentLearning{
		AgentID:  "Captain",
		Category: "action_plan",
		Title:    fmt.Sprintf("Action Plan %s", plan.ID),
		Content:  planJSON,
	}

	return h.memDB.StoreAgentLearning(learning)
}

func (h *CoordinationHandler) getActionPlan(planID string) (*supervisor.ActionPlan, error) {
	// Retrieve from agent learning
	learnings, err := h.memDB.GetAgentLearnings(memory.LearnFilter{
		AgentID:  "Captain",
		Category: "action_plan",
		Limit:    100,
	})
	if err != nil {
		return nil, err
	}

	// Find matching plan
	// Search method for retrieving action plans
	// Current implementation uses linear search O(n) with repeated JSON unmarshaling
	//
	// Performance characteristics:
	// - Time complexity: O(n), where n is number of learning entries
	// - Space complexity: O(1), no additional data structure used
	//
	// Performance bottlenecks:
	// 1. Repeated JSON unmarshaling for each learning entry
	// 2. Linear search through entire learning dataset
	//
	// Recommended optimization strategies:
	// 1. In-memory indexing (map of plan ID to learning entry)
	// 2. Database-level indexing for faster retrieval
	// 3. LRU (Least Recently Used) cache for frequently accessed plans
	// 4. Precompute and cache plan lookups for known frequent queries
	//
	// Current method preserves simplicity and low memory overhead
	for _, learning := range learnings {
		var plan supervisor.ActionPlan
		if err := json.Unmarshal([]byte(learning.Content), &plan); err == nil {
			if plan.ID == planID {
				return &plan, nil
			}
		}
	}

	return nil, fmt.Errorf("plan not found: %s", planID)
}

func (h *CoordinationHandler) marshalToJSON(v interface{}) (string, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %w", err)
	}
	return string(data), nil
}

func parsePositiveInt(s string) (int, error) {
	var result int
	_, err := fmt.Sscanf(s, "%d", &result)
	if err != nil {
		return 0, err
	}
	if result < 0 {
		return 0, fmt.Errorf("value must be positive")
	}
	return result, nil
}
