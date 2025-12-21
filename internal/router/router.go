// Package router provides the Skill Router service for query routing and agent communication.
// It replaces PowerShell heartbeat scripts with unified Go-based communication.
package router

import (
	"fmt"
	"strings"

	"github.com/CLIAIMONITOR/internal/memory"
)

// QueryType represents the type of query being made
type QueryType string

const (
	QueryTypeKnowledge   QueryType = "knowledge"   // RAG knowledge queries
	QueryTypeEpisode     QueryType = "episode"     // Episode/history queries
	QueryTypeOperational QueryType = "operational" // Agent/task operational queries
	QueryTypeRecon       QueryType = "recon"       // Reconnaissance/security queries
	QueryTypeUnknown     QueryType = "unknown"     // Couldn't classify
)

// SkillRouter routes queries to the appropriate data source
type SkillRouter struct {
	memDB      memory.MemoryDB
	learningDB memory.LearningDB
}

// NewSkillRouter creates a new skill router
func NewSkillRouter(memDB memory.MemoryDB) *SkillRouter {
	return &SkillRouter{
		memDB:      memDB,
		learningDB: memDB.AsLearningDB(),
	}
}

// ClassifyQuery determines what type of query this is based on keywords
func (r *SkillRouter) ClassifyQuery(query string) QueryType {
	query = strings.ToLower(query)

	// Knowledge patterns - things learned, solutions, patterns
	knowledgePatterns := []string{
		"how do i", "how to", "what is the fix", "solution for",
		"pattern for", "best practice", "gotcha", "learned",
		"error solution", "fix for", "what worked", "remember when",
	}
	for _, p := range knowledgePatterns {
		if strings.Contains(query, p) {
			return QueryTypeKnowledge
		}
	}

	// Episode patterns - what happened, session history
	episodePatterns := []string{
		"what happened", "last session", "previous", "history",
		"what did", "earlier", "before", "decision made",
		"action taken", "last time", "recent events",
	}
	for _, p := range episodePatterns {
		if strings.Contains(query, p) {
			return QueryTypeEpisode
		}
	}

	// Operational patterns - agents, tasks, status
	operationalPatterns := []string{
		"agent", "task", "running", "status", "spawn",
		"stop", "workflow", "pending", "assigned", "blocked",
		"who is", "which agents", "active tasks",
	}
	for _, p := range operationalPatterns {
		if strings.Contains(query, p) {
			return QueryTypeOperational
		}
	}

	// Recon patterns - security, vulnerabilities
	reconPatterns := []string{
		"vulnerability", "security", "finding", "scan",
		"critical", "cve", "exposure", "threat", "risk",
		"environment", "remediation",
	}
	for _, p := range reconPatterns {
		if strings.Contains(query, p) {
			return QueryTypeRecon
		}
	}

	return QueryTypeUnknown
}

// RouteQuery routes a query to the appropriate handler based on type
func (r *SkillRouter) RouteQuery(query string, limit int) (*QueryResult, error) {
	queryType := r.ClassifyQuery(query)

	switch queryType {
	case QueryTypeKnowledge:
		return r.queryKnowledge(query, limit)
	case QueryTypeEpisode:
		return r.queryEpisodes(query, limit)
	case QueryTypeOperational:
		return r.queryOperational(query, limit)
	case QueryTypeRecon:
		return r.queryRecon(query, limit)
	default:
		// Try knowledge first as fallback, then episodes
		result, err := r.queryKnowledge(query, limit)
		if err == nil && len(result.Items) > 0 {
			return result, nil
		}
		return r.queryEpisodes(query, limit)
	}
}

// QueryResult holds the results of a routed query
type QueryResult struct {
	QueryType QueryType     `json:"query_type"`
	Query     string        `json:"query"`
	Items     []interface{} `json:"items"`
	Count     int           `json:"count"`
	Source    string        `json:"source"` // "learning.db" or "operational.db"
}

// queryKnowledge searches the knowledge base
func (r *SkillRouter) queryKnowledge(query string, limit int) (*QueryResult, error) {
	if limit <= 0 {
		limit = 5
	}

	results, err := r.learningDB.SearchKnowledge(query, "", limit)
	if err != nil {
		return nil, fmt.Errorf("knowledge search failed: %w", err)
	}

	items := make([]interface{}, len(results))
	for i, k := range results {
		items[i] = map[string]interface{}{
			"id":              k.ID,
			"category":        k.Category,
			"title":           k.Title,
			"content":         k.Content,
			"tags":            k.Tags,
			"relevance_score": k.RelevanceScore,
		}
		// Track usage
		r.learningDB.IncrementUseCount(k.ID)
	}

	return &QueryResult{
		QueryType: QueryTypeKnowledge,
		Query:     query,
		Items:     items,
		Count:     len(items),
		Source:    "learning.db",
	}, nil
}

// queryEpisodes searches episodes
func (r *SkillRouter) queryEpisodes(query string, limit int) (*QueryResult, error) {
	if limit <= 0 {
		limit = 10
	}

	results, err := r.learningDB.SearchEpisodes(query, "", limit)
	if err != nil {
		return nil, fmt.Errorf("episode search failed: %w", err)
	}

	items := make([]interface{}, len(results))
	for i, ep := range results {
		items[i] = map[string]interface{}{
			"id":         ep.ID,
			"session_id": ep.SessionID,
			"agent_id":   ep.AgentID,
			"event_type": ep.EventType,
			"title":      ep.Title,
			"content":    ep.Content,
			"project":    ep.Project,
			"importance": ep.Importance,
			"created_at": ep.CreatedAt,
		}
	}

	return &QueryResult{
		QueryType: QueryTypeEpisode,
		Query:     query,
		Items:     items,
		Count:     len(items),
		Source:    "learning.db",
	}, nil
}

// queryOperational searches operational data (agents, tasks)
func (r *SkillRouter) queryOperational(query string, limit int) (*QueryResult, error) {
	if limit <= 0 {
		limit = 20
	}

	query = strings.ToLower(query)
	var items []interface{}

	// Check if asking about agents
	// NOTE: Agent queries now handled via in-memory JSONStore instead of database
	// This section is disabled as agent_control table is removed
	if strings.Contains(query, "agent") {
		// TODO: Query agents from JSONStore if needed
		items = append(items, map[string]interface{}{
			"type":    "note",
			"message": "Agent queries now handled via in-memory store",
		})
	}

	// Check if asking about tasks
	if strings.Contains(query, "task") {
		status := ""
		if strings.Contains(query, "pending") {
			status = "pending"
		} else if strings.Contains(query, "assigned") {
			status = "assigned"
		} else if strings.Contains(query, "in_progress") || strings.Contains(query, "in progress") {
			status = "in_progress"
		} else if strings.Contains(query, "blocked") {
			status = "blocked"
		}

		tasks, err := r.memDB.GetTasks(memory.TaskFilter{
			Status: status,
			Limit:  limit,
		})
		if err != nil {
			return nil, fmt.Errorf("task query failed: %w", err)
		}
		for _, t := range tasks {
			items = append(items, map[string]interface{}{
				"type":        "task",
				"id":          t.ID,
				"title":       t.Title,
				"status":      t.Status,
				"priority":    t.Priority,
				"assigned_to": t.AssignedAgentID,
			})
		}
	}

	return &QueryResult{
		QueryType: QueryTypeOperational,
		Query:     query,
		Items:     items,
		Count:     len(items),
		Source:    "operational.db",
	}, nil
}

// queryRecon searches reconnaissance data
func (r *SkillRouter) queryRecon(query string, limit int) (*QueryResult, error) {
	if limit <= 0 {
		limit = 10
	}

	// Get reconnaissance findings from memory DB
	// The recon data is stored via the existing recon repository
	learnings, err := r.memDB.GetAgentLearnings(memory.LearnFilter{
		Category: "reconnaissance",
		Limit:    limit,
	})
	if err != nil {
		return nil, fmt.Errorf("recon query failed: %w", err)
	}

	items := make([]interface{}, len(learnings))
	for i, l := range learnings {
		items[i] = map[string]interface{}{
			"id":         l.ID,
			"agent_id":   l.AgentID,
			"title":      l.Title,
			"content":    l.Content,
			"created_at": l.CreatedAt,
		}
	}

	return &QueryResult{
		QueryType: QueryTypeRecon,
		Query:     query,
		Items:     items,
		Count:     len(items),
		Source:    "operational.db",
	}, nil
}
