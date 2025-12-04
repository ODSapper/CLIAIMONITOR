package memory

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// LearningDB provides RAG-style memory operations
type LearningDB interface {
	// Episodes - what happened
	RecordEpisode(episode *Episode) error
	GetRecentEpisodes(sessionID string, limit int) ([]*Episode, error)
	SearchEpisodes(query string, project string, limit int) ([]*Episode, error)

	// Knowledge - what was learned
	StoreKnowledge(knowledge *Knowledge) error
	SearchKnowledge(query string, category string, limit int) ([]*Knowledge, error)
	SearchKnowledgeByType(query string, agentType string, category string, limit int) ([]*Knowledge, error)
	GetKnowledge(id string) (*Knowledge, error)
	IncrementUseCount(id string) error

	// Maintenance
	GetKnowledgeStats() (*KnowledgeStats, error)
}

// Episode represents a timestamped event
type Episode struct {
	ID         string    `json:"id"`
	SessionID  string    `json:"session_id"`
	AgentID    string    `json:"agent_id"`
	AgentType  string    `json:"agent_type"` // captain, developer, recon, reviewer
	EventType  string    `json:"event_type"` // action, error, decision, outcome
	Title      string    `json:"title"`
	Content    string    `json:"content"`
	Project    string    `json:"project,omitempty"`
	Importance float64   `json:"importance"`
	CreatedAt  time.Time `json:"created_at"`
}

// Knowledge represents a searchable piece of learned information
type Knowledge struct {
	ID        string    `json:"id"`
	AgentType string    `json:"agent_type"` // captain, developer, recon, reviewer
	Category  string    `json:"category"`   // error_solution, pattern, best_practice, gotcha
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Tags      []string  `json:"tags,omitempty"`
	Source    string    `json:"source,omitempty"`
	UseCount  int       `json:"use_count"`
	LastUsed  *time.Time `json:"last_used,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	// Search result fields
	RelevanceScore float64 `json:"relevance_score,omitempty"`
}

// AgentTypes for knowledge filtering
const (
	AgentTypeCaptain   = "captain"
	AgentTypeDeveloper = "developer"
	AgentTypeRecon     = "recon"
	AgentTypeReviewer  = "reviewer"
)

// KnowledgeStats provides statistics about the knowledge base
type KnowledgeStats struct {
	TotalKnowledge   int            `json:"total_knowledge"`
	TotalEpisodes    int            `json:"total_episodes"`
	ByCategory       map[string]int `json:"by_category"`
	TotalTerms       int            `json:"total_terms"`
	MostUsed         []*Knowledge   `json:"most_used"`
}

// SQLiteLearningDB implements LearningDB using SQLite with TF-IDF
type SQLiteLearningDB struct {
	db *sql.DB
}

// NewLearningDB creates a LearningDB from an existing SQLite connection
func NewLearningDB(db *sql.DB) LearningDB {
	return &SQLiteLearningDB{db: db}
}

// AsLearningDB returns the LearningDB interface from SQLiteMemoryDB
func (m *SQLiteMemoryDB) AsLearningDB() LearningDB {
	return &SQLiteLearningDB{db: m.db}
}

// ============================================================
// Episode Operations
// ============================================================

// RecordEpisode stores an episode
func (l *SQLiteLearningDB) RecordEpisode(episode *Episode) error {
	if episode.ID == "" {
		episode.ID = uuid.New().String()
	}
	if episode.Importance == 0 {
		episode.Importance = 0.5
	}
	// Default to captain if not specified
	if episode.AgentType == "" {
		episode.AgentType = AgentTypeCaptain
	}

	_, err := l.db.Exec(`
		INSERT INTO episodes (id, session_id, agent_id, agent_type, event_type, title, content, project, importance, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, episode.ID, episode.SessionID, episode.AgentID, episode.AgentType, episode.EventType,
		episode.Title, episode.Content, episode.Project, episode.Importance, time.Now())

	return err
}

// GetRecentEpisodes retrieves recent episodes for a session
func (l *SQLiteLearningDB) GetRecentEpisodes(sessionID string, limit int) ([]*Episode, error) {
	if limit <= 0 {
		limit = 10
	}

	query := `
		SELECT id, session_id, agent_id, agent_type, event_type, title, content, project, importance, created_at
		FROM episodes
		WHERE session_id = ? OR ? = ''
		ORDER BY created_at DESC
		LIMIT ?
	`

	rows, err := l.db.Query(query, sessionID, sessionID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var episodes []*Episode
	for rows.Next() {
		var ep Episode
		var project, agentType sql.NullString
		err := rows.Scan(&ep.ID, &ep.SessionID, &ep.AgentID, &agentType, &ep.EventType,
			&ep.Title, &ep.Content, &project, &ep.Importance, &ep.CreatedAt)
		if err != nil {
			return nil, err
		}
		if project.Valid {
			ep.Project = project.String
		}
		if agentType.Valid {
			ep.AgentType = agentType.String
		}
		episodes = append(episodes, &ep)
	}

	return episodes, nil
}

// SearchEpisodes searches episodes using TF-IDF
func (l *SQLiteLearningDB) SearchEpisodes(query string, project string, limit int) ([]*Episode, error) {
	if limit <= 0 {
		limit = 5
	}

	// Simple keyword search for episodes (they don't have TF-IDF index)
	// Use LIKE for basic matching
	terms := tokenize(query)
	if len(terms) == 0 {
		return l.GetRecentEpisodes("", limit)
	}

	// Build query with LIKE clauses
	var conditions []string
	var args []interface{}
	for _, term := range terms {
		conditions = append(conditions, "(title LIKE ? OR content LIKE ?)")
		args = append(args, "%"+term+"%", "%"+term+"%")
	}

	queryStr := fmt.Sprintf(`
		SELECT id, session_id, agent_id, event_type, title, content, project, importance, created_at
		FROM episodes
		WHERE (%s)
	`, strings.Join(conditions, " OR "))

	if project != "" {
		queryStr += " AND project = ?"
		args = append(args, project)
	}

	queryStr += " ORDER BY created_at DESC LIMIT ?"
	args = append(args, limit)

	rows, err := l.db.Query(queryStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var episodes []*Episode
	for rows.Next() {
		var ep Episode
		var proj sql.NullString
		err := rows.Scan(&ep.ID, &ep.SessionID, &ep.AgentID, &ep.EventType,
			&ep.Title, &ep.Content, &proj, &ep.Importance, &ep.CreatedAt)
		if err != nil {
			return nil, err
		}
		if proj.Valid {
			ep.Project = proj.String
		}
		episodes = append(episodes, &ep)
	}

	return episodes, nil
}

// ============================================================
// Knowledge Operations
// ============================================================

// StoreKnowledge stores knowledge and builds TF-IDF index
func (l *SQLiteLearningDB) StoreKnowledge(knowledge *Knowledge) error {
	if knowledge.ID == "" {
		knowledge.ID = uuid.New().String()
	}
	// Default to captain if not specified
	if knowledge.AgentType == "" {
		knowledge.AgentType = AgentTypeCaptain
	}

	now := time.Now()
	tagsJSON, _ := json.Marshal(knowledge.Tags)

	// Start transaction
	tx, err := l.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Insert knowledge with agent_type
	_, err = tx.Exec(`
		INSERT INTO knowledge (id, agent_type, category, title, content, tags, source, use_count, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, 0, ?, ?)
	`, knowledge.ID, knowledge.AgentType, knowledge.Category, knowledge.Title, knowledge.Content,
		string(tagsJSON), knowledge.Source, now, now)
	if err != nil {
		return err
	}

	// Build TF-IDF index for this document
	text := knowledge.Title + " " + knowledge.Content
	terms := tokenize(text)
	termFreq := computeTermFrequency(terms)

	for term, tf := range termFreq {
		// Insert term frequency
		_, err = tx.Exec(`
			INSERT OR REPLACE INTO knowledge_terms (knowledge_id, term, tf)
			VALUES (?, ?, ?)
		`, knowledge.ID, term, tf)
		if err != nil {
			return err
		}

		// Update document frequency
		_, err = tx.Exec(`
			INSERT INTO term_stats (term, doc_count) VALUES (?, 1)
			ON CONFLICT(term) DO UPDATE SET doc_count = doc_count + 1
		`, term)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// SearchKnowledge searches using TF-IDF scoring
func (l *SQLiteLearningDB) SearchKnowledge(query string, category string, limit int) ([]*Knowledge, error) {
	if limit <= 0 {
		limit = 5
	}

	queryTerms := tokenize(query)
	if len(queryTerms) == 0 {
		return []*Knowledge{}, nil
	}

	// Get total document count for IDF
	var totalDocs int
	err := l.db.QueryRow("SELECT COUNT(*) FROM knowledge").Scan(&totalDocs)
	if err != nil {
		return nil, err
	}
	if totalDocs == 0 {
		return []*Knowledge{}, nil
	}

	// Get document frequencies for query terms
	placeholders := make([]string, len(queryTerms))
	args := make([]interface{}, len(queryTerms))
	for i, term := range queryTerms {
		placeholders[i] = "?"
		args[i] = term
	}

	termDocFreq := make(map[string]int)
	rows, err := l.db.Query(fmt.Sprintf(`
		SELECT term, doc_count FROM term_stats WHERE term IN (%s)
	`, strings.Join(placeholders, ",")), args...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var term string
		var count int
		rows.Scan(&term, &count)
		termDocFreq[term] = count
	}
	rows.Close()

	// Calculate IDF for each query term
	idf := make(map[string]float64)
	for _, term := range queryTerms {
		df := termDocFreq[term]
		if df == 0 {
			df = 1 // Avoid division by zero
		}
		idf[term] = math.Log(float64(totalDocs+1) / float64(df+1))
	}

	// Get all documents that contain any query term
	rows, err = l.db.Query(fmt.Sprintf(`
		SELECT DISTINCT knowledge_id FROM knowledge_terms WHERE term IN (%s)
	`, strings.Join(placeholders, ",")), args...)
	if err != nil {
		return nil, err
	}

	var docIDs []string
	for rows.Next() {
		var id string
		rows.Scan(&id)
		docIDs = append(docIDs, id)
	}
	rows.Close()

	if len(docIDs) == 0 {
		return []*Knowledge{}, nil
	}

	// Calculate TF-IDF score for each document
	type scoredDoc struct {
		id    string
		score float64
	}
	var scored []scoredDoc

	for _, docID := range docIDs {
		// Get term frequencies for this document
		rows, err := l.db.Query(`
			SELECT term, tf FROM knowledge_terms WHERE knowledge_id = ?
		`, docID)
		if err != nil {
			continue
		}

		docTermFreq := make(map[string]float64)
		for rows.Next() {
			var term string
			var tf float64
			rows.Scan(&term, &tf)
			docTermFreq[term] = tf
		}
		rows.Close()

		// Calculate score
		var score float64
		for _, term := range queryTerms {
			if tf, ok := docTermFreq[term]; ok {
				score += tf * idf[term]
			}
		}

		if score > 0 {
			scored = append(scored, scoredDoc{id: docID, score: score})
		}
	}

	// Sort by score descending
	for i := 0; i < len(scored); i++ {
		for j := i + 1; j < len(scored); j++ {
			if scored[j].score > scored[i].score {
				scored[i], scored[j] = scored[j], scored[i]
			}
		}
	}

	// Limit results
	if len(scored) > limit {
		scored = scored[:limit]
	}

	// Fetch full knowledge records
	var results []*Knowledge
	for _, s := range scored {
		k, err := l.GetKnowledge(s.id)
		if err != nil {
			continue
		}
		// Apply category filter if specified
		if category != "" && k.Category != category {
			continue
		}
		k.RelevanceScore = s.score
		results = append(results, k)
	}

	return results, nil
}

// SearchKnowledgeByType searches knowledge filtered by agent type
// This allows each agent type to have isolated knowledge
func (l *SQLiteLearningDB) SearchKnowledgeByType(query string, agentType string, category string, limit int) ([]*Knowledge, error) {
	if limit <= 0 {
		limit = 5
	}
	if agentType == "" {
		agentType = AgentTypeCaptain
	}

	queryTerms := tokenize(query)
	if len(queryTerms) == 0 {
		return []*Knowledge{}, nil
	}

	// Get total document count for this agent type
	var totalDocs int
	err := l.db.QueryRow("SELECT COUNT(*) FROM knowledge WHERE agent_type = ?", agentType).Scan(&totalDocs)
	if err != nil {
		return nil, err
	}
	if totalDocs == 0 {
		return []*Knowledge{}, nil
	}

	// Get document frequencies for query terms (filtered by agent type)
	placeholders := make([]string, len(queryTerms))
	termArgs := make([]interface{}, len(queryTerms)+1)
	for i, term := range queryTerms {
		placeholders[i] = "?"
		termArgs[i] = term
	}
	termArgs[len(queryTerms)] = agentType

	// Count documents containing each term for this agent type specifically
	termDocFreq := make(map[string]int)
	rows, err := l.db.Query(fmt.Sprintf(`
		SELECT kt.term, COUNT(DISTINCT kt.knowledge_id) as doc_count
		FROM knowledge_terms kt
		JOIN knowledge k ON k.id = kt.knowledge_id
		WHERE kt.term IN (%s) AND k.agent_type = ?
		GROUP BY kt.term
	`, strings.Join(placeholders, ",")), termArgs...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var term string
		var count int
		rows.Scan(&term, &count)
		termDocFreq[term] = count
	}
	rows.Close()

	// Calculate IDF for each query term
	// Use a minimum IDF of 0.1 to ensure terms present in all docs still get some weight
	idf := make(map[string]float64)
	for _, term := range queryTerms {
		df := termDocFreq[term]
		if df == 0 {
			df = 1
		}
		idfVal := math.Log(float64(totalDocs+1) / float64(df+1))
		if idfVal < 0.1 {
			idfVal = 0.1 // Minimum IDF to prevent zero scores
		}
		idf[term] = idfVal
	}

	// Get documents that match the agent type and contain query terms
	args := make([]interface{}, len(queryTerms)+1)
	for i, term := range queryTerms {
		args[i] = term
	}
	args[len(queryTerms)] = agentType
	querySQL := fmt.Sprintf(`
		SELECT DISTINCT kt.knowledge_id
		FROM knowledge_terms kt
		JOIN knowledge k ON k.id = kt.knowledge_id
		WHERE kt.term IN (%s) AND k.agent_type = ?
	`, strings.Join(placeholders, ","))

	rows, err = l.db.Query(querySQL, args...)
	if err != nil {
		return nil, err
	}

	var docIDs []string
	for rows.Next() {
		var id string
		rows.Scan(&id)
		docIDs = append(docIDs, id)
	}
	rows.Close()

	if len(docIDs) == 0 {
		return []*Knowledge{}, nil
	}

	// Calculate TF-IDF score for each document
	type scoredDoc struct {
		id    string
		score float64
	}
	var scored []scoredDoc

	for _, docID := range docIDs {
		rows, err := l.db.Query(`SELECT term, tf FROM knowledge_terms WHERE knowledge_id = ?`, docID)
		if err != nil {
			continue
		}

		docTermFreq := make(map[string]float64)
		for rows.Next() {
			var term string
			var tf float64
			rows.Scan(&term, &tf)
			docTermFreq[term] = tf
		}
		rows.Close()

		var score float64
		for _, term := range queryTerms {
			if tf, ok := docTermFreq[term]; ok {
				score += tf * idf[term]
			}
		}

		if score > 0 {
			scored = append(scored, scoredDoc{id: docID, score: score})
		}
	}

	// Sort by score descending
	for i := 0; i < len(scored); i++ {
		for j := i + 1; j < len(scored); j++ {
			if scored[j].score > scored[i].score {
				scored[i], scored[j] = scored[j], scored[i]
			}
		}
	}

	// Limit results
	if len(scored) > limit {
		scored = scored[:limit]
	}

	// Fetch full knowledge records
	var results []*Knowledge
	for _, s := range scored {
		k, err := l.GetKnowledge(s.id)
		if err != nil {
			continue
		}
		if category != "" && k.Category != category {
			continue
		}
		k.RelevanceScore = s.score
		results = append(results, k)
	}

	return results, nil
}

// GetKnowledge retrieves a single knowledge entry
func (l *SQLiteLearningDB) GetKnowledge(id string) (*Knowledge, error) {
	var k Knowledge
	var agentType sql.NullString
	var tagsJSON sql.NullString
	var source sql.NullString
	var lastUsed sql.NullTime

	err := l.db.QueryRow(`
		SELECT id, agent_type, category, title, content, tags, source, use_count, last_used, created_at, updated_at
		FROM knowledge WHERE id = ?
	`, id).Scan(&k.ID, &agentType, &k.Category, &k.Title, &k.Content, &tagsJSON, &source,
		&k.UseCount, &lastUsed, &k.CreatedAt, &k.UpdatedAt)

	if err != nil {
		return nil, err
	}

	if agentType.Valid {
		k.AgentType = agentType.String
	} else {
		k.AgentType = AgentTypeCaptain
	}
	if tagsJSON.Valid {
		json.Unmarshal([]byte(tagsJSON.String), &k.Tags)
	}
	if source.Valid {
		k.Source = source.String
	}
	if lastUsed.Valid {
		k.LastUsed = &lastUsed.Time
	}

	return &k, nil
}

// IncrementUseCount tracks when knowledge is used
func (l *SQLiteLearningDB) IncrementUseCount(id string) error {
	_, err := l.db.Exec(`
		UPDATE knowledge SET use_count = use_count + 1, last_used = ? WHERE id = ?
	`, time.Now(), id)
	return err
}

// GetKnowledgeStats returns statistics about the knowledge base
func (l *SQLiteLearningDB) GetKnowledgeStats() (*KnowledgeStats, error) {
	stats := &KnowledgeStats{
		ByCategory: make(map[string]int),
	}

	// Total knowledge
	l.db.QueryRow("SELECT COUNT(*) FROM knowledge").Scan(&stats.TotalKnowledge)

	// Total episodes
	l.db.QueryRow("SELECT COUNT(*) FROM episodes").Scan(&stats.TotalEpisodes)

	// Total terms
	l.db.QueryRow("SELECT COUNT(*) FROM term_stats").Scan(&stats.TotalTerms)

	// By category
	rows, _ := l.db.Query("SELECT category, COUNT(*) FROM knowledge GROUP BY category")
	if rows != nil {
		for rows.Next() {
			var cat string
			var count int
			rows.Scan(&cat, &count)
			stats.ByCategory[cat] = count
		}
		rows.Close()
	}

	// Most used
	rows, _ = l.db.Query(`
		SELECT id, category, title, content, use_count, created_at, updated_at
		FROM knowledge ORDER BY use_count DESC LIMIT 5
	`)
	if rows != nil {
		for rows.Next() {
			var k Knowledge
			rows.Scan(&k.ID, &k.Category, &k.Title, &k.Content, &k.UseCount, &k.CreatedAt, &k.UpdatedAt)
			stats.MostUsed = append(stats.MostUsed, &k)
		}
		rows.Close()
	}

	return stats, nil
}

// ============================================================
// TF-IDF Helpers
// ============================================================

var wordRegex = regexp.MustCompile(`[a-zA-Z0-9_]+`)

// tokenize splits text into lowercase terms
func tokenize(text string) []string {
	text = strings.ToLower(text)
	matches := wordRegex.FindAllString(text, -1)

	// Remove very short terms and stopwords
	var terms []string
	stopwords := map[string]bool{
		"the": true, "a": true, "an": true, "and": true, "or": true,
		"is": true, "in": true, "to": true, "of": true, "for": true,
		"it": true, "on": true, "at": true, "by": true, "this": true,
		"that": true, "with": true, "from": true, "as": true, "be": true,
		"was": true, "are": true, "been": true, "being": true, "have": true,
		"has": true, "had": true, "do": true, "does": true, "did": true,
		"will": true, "would": true, "could": true, "should": true,
		"i": true, "you": true, "we": true, "they": true, "he": true, "she": true,
	}

	for _, term := range matches {
		if len(term) >= 2 && !stopwords[term] {
			terms = append(terms, term)
		}
	}

	return terms
}

// computeTermFrequency calculates normalized term frequency
func computeTermFrequency(terms []string) map[string]float64 {
	counts := make(map[string]int)
	for _, term := range terms {
		counts[term]++
	}

	// Normalize by max frequency
	maxFreq := 0
	for _, count := range counts {
		if count > maxFreq {
			maxFreq = count
		}
	}

	tf := make(map[string]float64)
	for term, count := range counts {
		tf[term] = 0.5 + 0.5*float64(count)/float64(maxFreq)
	}

	return tf
}
