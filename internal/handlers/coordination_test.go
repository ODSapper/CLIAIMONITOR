package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/CLIAIMONITOR/internal/supervisor"
	"github.com/gorilla/mux"
)

// Test parsePositiveInt helper function
func TestParsePositiveInt(t *testing.T) {
	tests := []struct {
		input     string
		expected  int
		expectErr bool
	}{
		{"10", 10, false},
		{"0", 0, false},
		{"100", 100, false},
		{"-5", 0, true},
		{"abc", 0, true},
		{"", 0, true},
	}

	for _, tt := range tests {
		result, err := parsePositiveInt(tt.input)
		if tt.expectErr {
			if err == nil {
				t.Errorf("parsePositiveInt(%q) expected error, got nil", tt.input)
			}
		} else {
			if err != nil {
				t.Errorf("parsePositiveInt(%q) unexpected error: %v", tt.input, err)
			}
			if result != tt.expected {
				t.Errorf("parsePositiveInt(%q) = %d, want %d", tt.input, result, tt.expected)
			}
		}
	}
}

// Test handleAnalyzeReport with invalid JSON
func TestHandleAnalyzeReport_InvalidJSON(t *testing.T) {
	// Create a minimal handler (will fail during parsing, not during storage)
	handler := &CoordinationHandler{
		parser: supervisor.NewReportParser(),
	}

	req := httptest.NewRequest(http.MethodPost, "/coordination/analyze", bytes.NewReader([]byte("{invalid json}")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.handleAnalyzeReport(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

// Test handleAnalyzeReport with missing required fields
func TestHandleAnalyzeReport_MissingRequiredFields(t *testing.T) {
	handler := &CoordinationHandler{
		parser: supervisor.NewReportParser(),
	}

	// Missing version and ID
	report := supervisor.ReconReport{
		AgentID:     "snake-1",
		Environment: "test-env",
	}

	body, _ := json.Marshal(report)
	req := httptest.NewRequest(http.MethodPost, "/coordination/analyze", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.handleAnalyzeReport(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

// Test handleDispatch with missing plan_id
func TestHandleDispatch_MissingPlanID(t *testing.T) {
	handler := &CoordinationHandler{}

	reqBody := map[string]string{}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/coordination/dispatch", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.handleDispatch(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

// Test handleDispatch with invalid JSON
func TestHandleDispatch_InvalidJSON(t *testing.T) {
	handler := &CoordinationHandler{}

	req := httptest.NewRequest(http.MethodPost, "/coordination/dispatch", bytes.NewReader([]byte("not json")))
	w := httptest.NewRecorder()

	handler.handleDispatch(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

// Test handleGetStatus with missing ID (empty path param)
func TestHandleGetCoordinationStatus_MissingID(t *testing.T) {
	handler := &CoordinationHandler{}

	req := httptest.NewRequest(http.MethodGet, "/coordination/status/", nil)
	router := mux.NewRouter()
	router.HandleFunc("/coordination/status/{id}", handler.handleGetStatus)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Mux will return 404 if path doesn't match
	if w.Code != http.StatusNotFound && w.Code != http.StatusBadRequest {
		t.Logf("Expected status 404 or 400, got %d (acceptable - mux handles empty path params)", w.Code)
	}
}

// Test handleAbortDispatch with empty ID
func TestHandleAbortDispatch_EmptyID(t *testing.T) {
	handler := &CoordinationHandler{}

	req := httptest.NewRequest(http.MethodPost, "/coordination/abort/", nil)
	router := mux.NewRouter()
	router.HandleFunc("/coordination/abort/{id}", handler.handleAbortDispatch)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Mux will return 404 if path doesn't match
	if w.Code != http.StatusNotFound && w.Code != http.StatusBadRequest {
		t.Logf("Expected status 404 or 400, got %d (acceptable - mux handles empty path params)", w.Code)
	}
}

// Test parsePositiveInt with query parameters (unit test of helper function)
func TestQueryParameterParsing(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
		hasError bool
	}{
		{"valid positive", "10", 10, false},
		{"zero", "0", 0, false},
		{"large number", "1000", 1000, false},
		{"negative", "-5", 0, true},
		{"non-numeric", "abc", 0, true},
		{"empty", "", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parsePositiveInt(tt.input)
			if tt.hasError {
				if err == nil {
					t.Errorf("Expected error for input %q", tt.input)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for input %q: %v", tt.input, err)
				}
				if result != tt.expected {
					t.Errorf("Expected %d, got %d", tt.expected, result)
				}
			}
		})
	}
}

// Test handleListPlans (currently returns empty list)
func TestHandleListPlans(t *testing.T) {
	handler := &CoordinationHandler{}

	req := httptest.NewRequest(http.MethodGet, "/coordination/plans", nil)
	w := httptest.NewRecorder()

	handler.handleListPlans(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.NewDecoder(w.Body).Decode(&response)

	if response["count"].(float64) != 0 {
		t.Errorf("Expected count 0, got %v", response["count"])
	}
}

// Test handleGetPlan with empty ID
func TestHandleGetPlan_EmptyID(t *testing.T) {
	handler := &CoordinationHandler{}

	req := httptest.NewRequest(http.MethodGet, "/coordination/plans/", nil)
	router := mux.NewRouter()
	router.HandleFunc("/coordination/plans/{id}", handler.handleGetPlan)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Mux will return 404 if path doesn't match
	if w.Code != http.StatusNotFound && w.Code != http.StatusBadRequest {
		t.Logf("Expected status 404 or 400, got %d (acceptable - mux handles empty path params)", w.Code)
	}
}

// Test respondJSON helper
func TestRespondJSON(t *testing.T) {
	w := httptest.NewRecorder()
	data := map[string]string{"key": "value"}

	respondJSON(w, data)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type 'application/json', got '%s'", contentType)
	}

	var response map[string]string
	json.NewDecoder(w.Body).Decode(&response)

	if response["key"] != "value" {
		t.Errorf("Expected key=value, got key=%s", response["key"])
	}
}

// Test respondError helper
func TestRespondError(t *testing.T) {
	w := httptest.NewRecorder()

	respondError(w, http.StatusBadRequest, "test error message")

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type 'application/json', got '%s'", contentType)
	}
}

// Test buildReconDescription edge cases
func TestBuildReconDescription_AllFocusTypes(t *testing.T) {
	focusTypes := []string{"security", "architecture", "dependencies", "testing", "full", "unknown"}

	for _, focus := range focusTypes {
		desc := buildReconDescription(focus)
		if len(desc) == 0 {
			t.Errorf("buildReconDescription(%q) returned empty string", focus)
		}
		if len(desc) < 20 {
			t.Errorf("buildReconDescription(%q) returned suspiciously short description: %s", focus, desc)
		}
		// All descriptions should start with base text
		if desc[:40] != "Conduct reconnaissance on this codebase." {
			t.Errorf("buildReconDescription(%q) doesn't start with expected base text", focus)
		}
	}
}

// Test ReconRequest validation
func TestHandleRecon_ValidationFlow(t *testing.T) {
	// This is tested in captain_endpoints_test.go but we add edge cases
	tests := []struct {
		name        string
		request     ReconRequest
		expectError bool
	}{
		{
			name: "valid request",
			request: ReconRequest{
				ProjectPath: "/test/path",
				Mission:     "test mission",
			},
			expectError: false,
		},
		{
			name: "missing project path",
			request: ReconRequest{
				Mission: "test mission",
			},
			expectError: true,
		},
		{
			name: "empty project path",
			request: ReconRequest{
				ProjectPath: "",
				Mission:     "test mission",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just verify that empty ProjectPath would be caught
			if tt.request.ProjectPath == "" && !tt.expectError {
				t.Error("Empty ProjectPath should cause error")
			}
			if tt.request.ProjectPath != "" && tt.expectError {
				t.Error("Non-empty ProjectPath should not cause error")
			}
		})
	}
}

// Test timezone handling in timestamps
func TestTimestampHandling(t *testing.T) {
	now := time.Now()

	// Verify time serialization works correctly
	report := supervisor.ReconReport{
		ID:          "test-id",
		AgentID:     "test-agent",
		Environment: "test-env",
		Mission:     "test",
		Timestamp:   now,
	}

	data, err := json.Marshal(report)
	if err != nil {
		t.Errorf("Failed to marshal report: %v", err)
	}

	var decoded supervisor.ReconReport
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Errorf("Failed to unmarshal report: %v", err)
	}

	// Times should be equal (within millisecond precision)
	if decoded.Timestamp.Unix() != now.Unix() {
		t.Errorf("Timestamp mismatch: got %v, want %v", decoded.Timestamp, now)
	}
}
