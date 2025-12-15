package memory

import (
	"database/sql"
	"fmt"
	"time"
)

// ReviewBoard represents a multi-reviewer review session
type ReviewBoard struct {
	ID                 int64
	AssignmentID       int64
	ReviewerCount      int
	Status             string // pending, in_progress, completed, escalated
	ComplexityScore    int
	RiskLevel          string
	FinalVerdict       string
	AggregatedFeedback string
	CreatedAt          time.Time
	StartedAt          *time.Time
	CompletedAt        *time.Time
}

// ReviewDefect represents an individual defect finding from a reviewer
type ReviewDefect struct {
	ID              int64
	BoardID         int64
	ReviewerID      string
	Category        string
	Severity        string
	FilePath        string
	LineStart       int
	LineEnd         int
	Title           string
	Description     string
	SuggestedFix    string
	Status          string
	ResolutionNotes string
	ResolvedBy      string
	ResolvedAt      *time.Time
	CreatedAt       time.Time
}

// ReviewerVote represents a reviewer's final verdict
type ReviewerVote struct {
	ID                int64
	BoardID           int64
	ReviewerID        string
	Approved          bool
	ConfidenceScore   int
	DefectsFound      int
	ReviewTimeSeconds int
	TokensUsed        int64
	StartedAt         time.Time
	CompletedAt       *time.Time
}

// AgentQualityScore represents aggregate performance metrics for an agent
type AgentQualityScore struct {
	ID                     int64
	AgentID                string
	Role                   string // author, reviewer
	TotalSubmissions       int
	ApprovedFirstTry       int
	TotalApprovals         int
	TotalReviewCycles      int
	TotalDefectsReceived   int
	CriticalDefectsReceived int
	TotalReviews           int
	DefectsFound           int
	TruePositives          int
	FalsePositives         int
	CriticalFinds          int
	TotalTokensUsed        int64
	TotalCost              float64
	ValueDelivered         float64
	ApprovalRate           float64
	FirstPassRate          float64
	AvgReviewCycles        float64
	DefectDensity          float64
	DetectionAccuracy      float64
	DefectFindRate         float64
	CostEfficiency         float64
	QualityScore           float64
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

// DefectCategory represents a defect classification
type DefectCategory struct {
	Code            string
	Name            string
	CategoryType    string
	Description     string
	DefaultSeverity string
}

// ConsensusResult represents the aggregated review decision
type ConsensusResult struct {
	Approved           bool
	Decision           string
	HasCriticalDefects bool
	HasHighDefects     bool
	MajorityApproved   bool
	VotesFor           int
	VotesAgainst       int
	TotalDefects       int
	CriticalDefects    int
	HighDefects        int
	AggregatedFeedback string
}

// CreateReviewBoard creates a new review board
func (m *SQLiteMemoryDB) CreateReviewBoard(board *ReviewBoard) error {
	query := `
		INSERT INTO review_boards (
			assignment_id, reviewer_count, status, complexity_score, risk_level,
			final_verdict, aggregated_feedback, started_at, completed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := m.db.Exec(
		query,
		board.AssignmentID,
		board.ReviewerCount,
		board.Status,
		board.ComplexityScore,
		board.RiskLevel,
		nullString(board.FinalVerdict),
		nullString(board.AggregatedFeedback),
		nullTime(board.StartedAt),
		nullTime(board.CompletedAt),
	)
	if err != nil {
		return fmt.Errorf("failed to create review board: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get review board ID: %w", err)
	}

	board.ID = id
	return nil
}

// GetReviewBoard retrieves a review board by ID
func (m *SQLiteMemoryDB) GetReviewBoard(id int64) (*ReviewBoard, error) {
	query := `
		SELECT id, assignment_id, reviewer_count, status, complexity_score, risk_level,
		       final_verdict, aggregated_feedback, created_at, started_at, completed_at
		FROM review_boards
		WHERE id = ?
	`

	var board ReviewBoard
	var finalVerdict, aggregatedFeedback sql.NullString
	var startedAt, completedAt sql.NullTime

	err := m.db.QueryRow(query, id).Scan(
		&board.ID, &board.AssignmentID, &board.ReviewerCount, &board.Status,
		&board.ComplexityScore, &board.RiskLevel, &finalVerdict, &aggregatedFeedback,
		&board.CreatedAt, &startedAt, &completedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("review board not found: %d", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get review board: %w", err)
	}

	if finalVerdict.Valid {
		board.FinalVerdict = finalVerdict.String
	}
	if aggregatedFeedback.Valid {
		board.AggregatedFeedback = aggregatedFeedback.String
	}
	if startedAt.Valid {
		t := startedAt.Time
		board.StartedAt = &t
	}
	if completedAt.Valid {
		t := completedAt.Time
		board.CompletedAt = &t
	}

	return &board, nil
}

// GetReviewBoardByAssignment retrieves a review board by assignment ID
func (m *SQLiteMemoryDB) GetReviewBoardByAssignment(assignmentID int64) (*ReviewBoard, error) {
	query := `
		SELECT id, assignment_id, reviewer_count, status, complexity_score, risk_level,
		       final_verdict, aggregated_feedback, created_at, started_at, completed_at
		FROM review_boards
		WHERE assignment_id = ?
		ORDER BY created_at DESC
		LIMIT 1
	`

	var board ReviewBoard
	var finalVerdict, aggregatedFeedback sql.NullString
	var startedAt, completedAt sql.NullTime

	err := m.db.QueryRow(query, assignmentID).Scan(
		&board.ID, &board.AssignmentID, &board.ReviewerCount, &board.Status,
		&board.ComplexityScore, &board.RiskLevel, &finalVerdict, &aggregatedFeedback,
		&board.CreatedAt, &startedAt, &completedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get review board by assignment: %w", err)
	}

	if finalVerdict.Valid {
		board.FinalVerdict = finalVerdict.String
	}
	if aggregatedFeedback.Valid {
		board.AggregatedFeedback = aggregatedFeedback.String
	}
	if startedAt.Valid {
		t := startedAt.Time
		board.StartedAt = &t
	}
	if completedAt.Valid {
		t := completedAt.Time
		board.CompletedAt = &t
	}

	return &board, nil
}

// UpdateReviewBoard updates an existing review board
func (m *SQLiteMemoryDB) UpdateReviewBoard(board *ReviewBoard) error {
	query := `
		UPDATE review_boards
		SET reviewer_count = ?, status = ?, complexity_score = ?, risk_level = ?,
		    final_verdict = ?, aggregated_feedback = ?, started_at = ?, completed_at = ?
		WHERE id = ?
	`

	_, err := m.db.Exec(
		query,
		board.ReviewerCount,
		board.Status,
		board.ComplexityScore,
		board.RiskLevel,
		nullString(board.FinalVerdict),
		nullString(board.AggregatedFeedback),
		nullTime(board.StartedAt),
		nullTime(board.CompletedAt),
		board.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update review board: %w", err)
	}

	return nil
}

// CreateDefect creates a new defect finding
func (m *SQLiteMemoryDB) CreateDefect(defect *ReviewDefect) error {
	query := `
		INSERT INTO review_defects (
			board_id, reviewer_id, category, severity, file_path, line_start, line_end,
			title, description, suggested_fix, status, resolution_notes, resolved_by, resolved_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := m.db.Exec(
		query,
		defect.BoardID,
		defect.ReviewerID,
		defect.Category,
		defect.Severity,
		nullString(defect.FilePath),
		nullInt(defect.LineStart),
		nullInt(defect.LineEnd),
		defect.Title,
		defect.Description,
		nullString(defect.SuggestedFix),
		defect.Status,
		nullString(defect.ResolutionNotes),
		nullString(defect.ResolvedBy),
		nullTime(defect.ResolvedAt),
	)
	if err != nil {
		return fmt.Errorf("failed to create defect: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get defect ID: %w", err)
	}

	defect.ID = id
	return nil
}

// GetBoardDefects retrieves all defects for a review board
func (m *SQLiteMemoryDB) GetBoardDefects(boardID int64) ([]*ReviewDefect, error) {
	query := `
		SELECT id, board_id, reviewer_id, category, severity, file_path, line_start, line_end,
		       title, description, suggested_fix, status, resolution_notes, resolved_by, resolved_at, created_at
		FROM review_defects
		WHERE board_id = ?
		ORDER BY severity DESC, created_at ASC
	`

	rows, err := m.db.Query(query, boardID)
	if err != nil {
		return nil, fmt.Errorf("failed to query board defects: %w", err)
	}
	defer rows.Close()

	var defects []*ReviewDefect
	for rows.Next() {
		var d ReviewDefect
		var filePath, suggestedFix, resolutionNotes, resolvedBy sql.NullString
		var lineStart, lineEnd sql.NullInt64
		var resolvedAt sql.NullTime

		if err := rows.Scan(
			&d.ID, &d.BoardID, &d.ReviewerID, &d.Category, &d.Severity,
			&filePath, &lineStart, &lineEnd, &d.Title, &d.Description,
			&suggestedFix, &d.Status, &resolutionNotes, &resolvedBy, &resolvedAt, &d.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan defect: %w", err)
		}

		if filePath.Valid {
			d.FilePath = filePath.String
		}
		if lineStart.Valid {
			d.LineStart = int(lineStart.Int64)
		}
		if lineEnd.Valid {
			d.LineEnd = int(lineEnd.Int64)
		}
		if suggestedFix.Valid {
			d.SuggestedFix = suggestedFix.String
		}
		if resolutionNotes.Valid {
			d.ResolutionNotes = resolutionNotes.String
		}
		if resolvedBy.Valid {
			d.ResolvedBy = resolvedBy.String
		}
		if resolvedAt.Valid {
			t := resolvedAt.Time
			d.ResolvedAt = &t
		}

		defects = append(defects, &d)
	}

	return defects, rows.Err()
}

// GetDefectsByReviewer retrieves defects found by a specific reviewer
func (m *SQLiteMemoryDB) GetDefectsByReviewer(boardID int64, reviewerID string) ([]*ReviewDefect, error) {
	query := `
		SELECT id, board_id, reviewer_id, category, severity, file_path, line_start, line_end,
		       title, description, suggested_fix, status, resolution_notes, resolved_by, resolved_at, created_at
		FROM review_defects
		WHERE board_id = ? AND reviewer_id = ?
		ORDER BY severity DESC, created_at ASC
	`

	rows, err := m.db.Query(query, boardID, reviewerID)
	if err != nil {
		return nil, fmt.Errorf("failed to query defects by reviewer: %w", err)
	}
	defer rows.Close()

	var defects []*ReviewDefect
	for rows.Next() {
		var d ReviewDefect
		var filePath, suggestedFix, resolutionNotes, resolvedBy sql.NullString
		var lineStart, lineEnd sql.NullInt64
		var resolvedAt sql.NullTime

		if err := rows.Scan(
			&d.ID, &d.BoardID, &d.ReviewerID, &d.Category, &d.Severity,
			&filePath, &lineStart, &lineEnd, &d.Title, &d.Description,
			&suggestedFix, &d.Status, &resolutionNotes, &resolvedBy, &resolvedAt, &d.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan defect: %w", err)
		}

		if filePath.Valid {
			d.FilePath = filePath.String
		}
		if lineStart.Valid {
			d.LineStart = int(lineStart.Int64)
		}
		if lineEnd.Valid {
			d.LineEnd = int(lineEnd.Int64)
		}
		if suggestedFix.Valid {
			d.SuggestedFix = suggestedFix.String
		}
		if resolutionNotes.Valid {
			d.ResolutionNotes = resolutionNotes.String
		}
		if resolvedBy.Valid {
			d.ResolvedBy = resolvedBy.String
		}
		if resolvedAt.Valid {
			t := resolvedAt.Time
			d.ResolvedAt = &t
		}

		defects = append(defects, &d)
	}

	return defects, rows.Err()
}

// CreateReviewerVote creates a new reviewer vote
func (m *SQLiteMemoryDB) CreateReviewerVote(vote *ReviewerVote) error {
	query := `
		INSERT INTO reviewer_votes (
			board_id, reviewer_id, approved, confidence_score, defects_found,
			review_time_seconds, tokens_used, started_at, completed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := m.db.Exec(
		query,
		vote.BoardID,
		vote.ReviewerID,
		vote.Approved,
		vote.ConfidenceScore,
		vote.DefectsFound,
		nullInt(vote.ReviewTimeSeconds),
		vote.TokensUsed,
		vote.StartedAt,
		nullTime(vote.CompletedAt),
	)
	if err != nil {
		return fmt.Errorf("failed to create reviewer vote: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get reviewer vote ID: %w", err)
	}

	vote.ID = id
	return nil
}

// GetReviewerVotes retrieves all votes for a review board
func (m *SQLiteMemoryDB) GetReviewerVotes(boardID int64) ([]*ReviewerVote, error) {
	query := `
		SELECT id, board_id, reviewer_id, approved, confidence_score, defects_found,
		       review_time_seconds, tokens_used, started_at, completed_at
		FROM reviewer_votes
		WHERE board_id = ?
		ORDER BY completed_at ASC
	`

	rows, err := m.db.Query(query, boardID)
	if err != nil {
		return nil, fmt.Errorf("failed to query reviewer votes: %w", err)
	}
	defer rows.Close()

	var votes []*ReviewerVote
	for rows.Next() {
		var v ReviewerVote
		var reviewTimeSeconds sql.NullInt64
		var completedAt sql.NullTime

		if err := rows.Scan(
			&v.ID, &v.BoardID, &v.ReviewerID, &v.Approved, &v.ConfidenceScore,
			&v.DefectsFound, &reviewTimeSeconds, &v.TokensUsed, &v.StartedAt, &completedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan reviewer vote: %w", err)
		}

		if reviewTimeSeconds.Valid {
			v.ReviewTimeSeconds = int(reviewTimeSeconds.Int64)
		}
		if completedAt.Valid {
			t := completedAt.Time
			v.CompletedAt = &t
		}

		votes = append(votes, &v)
	}

	return votes, rows.Err()
}

// GetOrCreateQualityScore retrieves or creates an agent quality score
func (m *SQLiteMemoryDB) GetOrCreateQualityScore(agentID, role string) (*AgentQualityScore, error) {
	// Try to get existing score
	query := `
		SELECT id, agent_id, role, total_submissions, approved_first_try, total_approvals,
		       total_review_cycles, total_defects_received, critical_defects_received,
		       total_reviews, defects_found, true_positives, false_positives, critical_finds,
		       total_tokens_used, total_cost, value_delivered, approval_rate, first_pass_rate,
		       avg_review_cycles, defect_density, detection_accuracy, defect_find_rate,
		       cost_efficiency, quality_score, created_at, updated_at
		FROM agent_quality_scores
		WHERE agent_id = ?
	`

	var score AgentQualityScore
	err := m.db.QueryRow(query, agentID).Scan(
		&score.ID, &score.AgentID, &score.Role, &score.TotalSubmissions,
		&score.ApprovedFirstTry, &score.TotalApprovals, &score.TotalReviewCycles,
		&score.TotalDefectsReceived, &score.CriticalDefectsReceived, &score.TotalReviews,
		&score.DefectsFound, &score.TruePositives, &score.FalsePositives, &score.CriticalFinds,
		&score.TotalTokensUsed, &score.TotalCost, &score.ValueDelivered, &score.ApprovalRate,
		&score.FirstPassRate, &score.AvgReviewCycles, &score.DefectDensity, &score.DetectionAccuracy,
		&score.DefectFindRate, &score.CostEfficiency, &score.QualityScore, &score.CreatedAt, &score.UpdatedAt,
	)
	if err == nil {
		return &score, nil
	}
	if err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to get quality score: %w", err)
	}

	// Create new score
	insertQuery := `
		INSERT INTO agent_quality_scores (agent_id, role, quality_score)
		VALUES (?, ?, 50)
	`

	result, err := m.db.Exec(insertQuery, agentID, role)
	if err != nil {
		return nil, fmt.Errorf("failed to create quality score: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get quality score ID: %w", err)
	}

	// Return newly created score
	score = AgentQualityScore{
		ID:           id,
		AgentID:      agentID,
		Role:         role,
		QualityScore: 50,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	return &score, nil
}

// UpdateQualityScore updates an agent quality score
func (m *SQLiteMemoryDB) UpdateQualityScore(score *AgentQualityScore) error {
	query := `
		UPDATE agent_quality_scores
		SET total_submissions = ?, approved_first_try = ?, total_approvals = ?,
		    total_review_cycles = ?, total_defects_received = ?, critical_defects_received = ?,
		    total_reviews = ?, defects_found = ?, true_positives = ?, false_positives = ?,
		    critical_finds = ?, total_tokens_used = ?, total_cost = ?, value_delivered = ?,
		    approval_rate = ?, first_pass_rate = ?, avg_review_cycles = ?, defect_density = ?,
		    detection_accuracy = ?, defect_find_rate = ?, cost_efficiency = ?, quality_score = ?,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`

	_, err := m.db.Exec(
		query,
		score.TotalSubmissions, score.ApprovedFirstTry, score.TotalApprovals,
		score.TotalReviewCycles, score.TotalDefectsReceived, score.CriticalDefectsReceived,
		score.TotalReviews, score.DefectsFound, score.TruePositives, score.FalsePositives,
		score.CriticalFinds, score.TotalTokensUsed, score.TotalCost, score.ValueDelivered,
		score.ApprovalRate, score.FirstPassRate, score.AvgReviewCycles, score.DefectDensity,
		score.DetectionAccuracy, score.DefectFindRate, score.CostEfficiency, score.QualityScore,
		score.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update quality score: %w", err)
	}

	return nil
}

// GetAgentLeaderboard retrieves top agents by quality score
func (m *SQLiteMemoryDB) GetAgentLeaderboard(role string, limit int) ([]*AgentQualityScore, error) {
	var query string
	var args []interface{}

	if role != "" {
		query = `
			SELECT id, agent_id, role, total_submissions, approved_first_try, total_approvals,
			       total_review_cycles, total_defects_received, critical_defects_received,
			       total_reviews, defects_found, true_positives, false_positives, critical_finds,
			       total_tokens_used, total_cost, value_delivered, approval_rate, first_pass_rate,
			       avg_review_cycles, defect_density, detection_accuracy, defect_find_rate,
			       cost_efficiency, quality_score, created_at, updated_at
			FROM agent_quality_scores
			WHERE role = ?
			ORDER BY quality_score DESC
			LIMIT ?
		`
		args = []interface{}{role, limit}
	} else {
		query = `
			SELECT id, agent_id, role, total_submissions, approved_first_try, total_approvals,
			       total_review_cycles, total_defects_received, critical_defects_received,
			       total_reviews, defects_found, true_positives, false_positives, critical_finds,
			       total_tokens_used, total_cost, value_delivered, approval_rate, first_pass_rate,
			       avg_review_cycles, defect_density, detection_accuracy, defect_find_rate,
			       cost_efficiency, quality_score, created_at, updated_at
			FROM agent_quality_scores
			ORDER BY quality_score DESC
			LIMIT ?
		`
		args = []interface{}{limit}
	}

	rows, err := m.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query agent leaderboard: %w", err)
	}
	defer rows.Close()

	var scores []*AgentQualityScore
	for rows.Next() {
		var s AgentQualityScore
		if err := rows.Scan(
			&s.ID, &s.AgentID, &s.Role, &s.TotalSubmissions, &s.ApprovedFirstTry,
			&s.TotalApprovals, &s.TotalReviewCycles, &s.TotalDefectsReceived,
			&s.CriticalDefectsReceived, &s.TotalReviews, &s.DefectsFound,
			&s.TruePositives, &s.FalsePositives, &s.CriticalFinds, &s.TotalTokensUsed,
			&s.TotalCost, &s.ValueDelivered, &s.ApprovalRate, &s.FirstPassRate,
			&s.AvgReviewCycles, &s.DefectDensity, &s.DetectionAccuracy, &s.DefectFindRate,
			&s.CostEfficiency, &s.QualityScore, &s.CreatedAt, &s.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan quality score: %w", err)
		}
		scores = append(scores, &s)
	}

	return scores, rows.Err()
}

// GetDefectCategories retrieves all defect categories
func (m *SQLiteMemoryDB) GetDefectCategories() ([]*DefectCategory, error) {
	query := `
		SELECT code, name, category_type, description, default_severity
		FROM defect_categories
		ORDER BY category_type, code
	`

	rows, err := m.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query defect categories: %w", err)
	}
	defer rows.Close()

	var categories []*DefectCategory
	for rows.Next() {
		var c DefectCategory
		var description sql.NullString

		if err := rows.Scan(&c.Code, &c.Name, &c.CategoryType, &description, &c.DefaultSeverity); err != nil {
			return nil, fmt.Errorf("failed to scan defect category: %w", err)
		}

		if description.Valid {
			c.Description = description.String
		}

		categories = append(categories, &c)
	}

	return categories, rows.Err()
}

// CalculateConsensus analyzes reviewer votes and defects to determine consensus
func (m *SQLiteMemoryDB) CalculateConsensus(boardID int64) (*ConsensusResult, error) {
	// Get all votes
	votes, err := m.GetReviewerVotes(boardID)
	if err != nil {
		return nil, fmt.Errorf("failed to get reviewer votes: %w", err)
	}

	if len(votes) == 0 {
		return nil, fmt.Errorf("no votes found for board %d", boardID)
	}

	// Get all defects
	defects, err := m.GetBoardDefects(boardID)
	if err != nil {
		return nil, fmt.Errorf("failed to get board defects: %w", err)
	}

	// Count votes
	votesFor := 0
	votesAgainst := 0
	for _, vote := range votes {
		if vote.Approved {
			votesFor++
		} else {
			votesAgainst++
		}
	}

	// Count defects by severity
	criticalCount := 0
	highCount := 0
	for _, defect := range defects {
		if defect.Severity == "critical" {
			criticalCount++
		} else if defect.Severity == "high" {
			highCount++
		}
	}

	// Determine consensus
	majorityApproved := votesFor > votesAgainst
	hasCritical := criticalCount > 0
	hasHigh := highCount > 0

	// Decision logic: Reject if any critical defects or majority rejects
	approved := majorityApproved && !hasCritical
	decision := "approved"
	if hasCritical {
		decision = "rejected_critical"
	} else if !majorityApproved {
		decision = "rejected_majority"
	}

	// Build aggregated feedback
	feedback := fmt.Sprintf("Review completed with %d approvals and %d rejections. ", votesFor, votesAgainst)
	if len(defects) > 0 {
		feedback += fmt.Sprintf("Found %d total defects (%d critical, %d high). ", len(defects), criticalCount, highCount)
	} else {
		feedback += "No defects found. "
	}

	return &ConsensusResult{
		Approved:           approved,
		Decision:           decision,
		HasCriticalDefects: hasCritical,
		HasHighDefects:     hasHigh,
		MajorityApproved:   majorityApproved,
		VotesFor:           votesFor,
		VotesAgainst:       votesAgainst,
		TotalDefects:       len(defects),
		CriticalDefects:    criticalCount,
		HighDefects:        highCount,
		AggregatedFeedback: feedback,
	}, nil
}

// UpdateQualityScoresAfterReview updates agent quality scores based on review results
func (m *SQLiteMemoryDB) UpdateQualityScoresAfterReview(boardID int64, consensus *ConsensusResult) error {
	return m.withTx(func(tx *sql.Tx) error {
		// Get assignment to find author
		var authorID string
		err := tx.QueryRow(`
			SELECT ta.assigned_to
			FROM review_boards rb
			JOIN task_assignments ta ON rb.assignment_id = ta.id
			WHERE rb.id = ?
		`, boardID).Scan(&authorID)
		if err != nil {
			return fmt.Errorf("failed to get author from board: %w", err)
		}

		// Get reviewer votes
		votes, err := m.GetReviewerVotes(boardID)
		if err != nil {
			return fmt.Errorf("failed to get votes: %w", err)
		}

		// Update author metrics
		authorScore, err := m.GetOrCreateQualityScore(authorID, "author")
		if err != nil {
			return fmt.Errorf("failed to get author score: %w", err)
		}

		authorScore.TotalSubmissions++
		authorScore.TotalReviewCycles++
		authorScore.TotalDefectsReceived += consensus.TotalDefects
		authorScore.CriticalDefectsReceived += consensus.CriticalDefects

		if consensus.Approved {
			authorScore.TotalApprovals++
			if authorScore.TotalReviewCycles == 1 {
				authorScore.ApprovedFirstTry++
			}
		}

		// Calculate author metrics
		if authorScore.TotalSubmissions > 0 {
			authorScore.ApprovalRate = float64(authorScore.TotalApprovals) / float64(authorScore.TotalSubmissions)
			authorScore.FirstPassRate = float64(authorScore.ApprovedFirstTry) / float64(authorScore.TotalSubmissions)
			authorScore.DefectDensity = float64(authorScore.TotalDefectsReceived) / float64(authorScore.TotalSubmissions)
		}
		if authorScore.TotalApprovals > 0 {
			authorScore.AvgReviewCycles = float64(authorScore.TotalReviewCycles) / float64(authorScore.TotalApprovals)
		}

		// Calculate author quality score (0-100)
		authorScore.QualityScore = (authorScore.FirstPassRate * 40) +
			(authorScore.ApprovalRate * 30) +
			(1.0 - (authorScore.DefectDensity / 10.0)) * 30

		if authorScore.QualityScore < 0 {
			authorScore.QualityScore = 0
		}
		if authorScore.QualityScore > 100 {
			authorScore.QualityScore = 100
		}

		if err := m.UpdateQualityScore(authorScore); err != nil {
			return fmt.Errorf("failed to update author score: %w", err)
		}

		// Update reviewer metrics
		for _, vote := range votes {
			reviewerScore, err := m.GetOrCreateQualityScore(vote.ReviewerID, "reviewer")
			if err != nil {
				return fmt.Errorf("failed to get reviewer score: %w", err)
			}

			reviewerScore.TotalReviews++
			reviewerScore.DefectsFound += vote.DefectsFound
			reviewerScore.TotalTokensUsed += vote.TokensUsed

			// Count critical finds
			defects, err := m.GetDefectsByReviewer(boardID, vote.ReviewerID)
			if err != nil {
				return fmt.Errorf("failed to get reviewer defects: %w", err)
			}
			for _, d := range defects {
				if d.Severity == "critical" {
					reviewerScore.CriticalFinds++
				}
			}

			// Calculate reviewer metrics
			if reviewerScore.TotalReviews > 0 {
				reviewerScore.DefectFindRate = float64(reviewerScore.DefectsFound) / float64(reviewerScore.TotalReviews)
			}

			// Detection accuracy needs manual updates for true/false positives
			// This would be done when defects are marked as acknowledged/disputed
			totalFindings := reviewerScore.TruePositives + reviewerScore.FalsePositives
			if totalFindings > 0 {
				reviewerScore.DetectionAccuracy = float64(reviewerScore.TruePositives) / float64(totalFindings)
			} else {
				reviewerScore.DetectionAccuracy = 1.0 // Assume accurate if no disputes
			}

			// Calculate cost efficiency
			if reviewerScore.TotalCost > 0 {
				reviewerScore.CostEfficiency = reviewerScore.ValueDelivered / reviewerScore.TotalCost
			}

			// Calculate reviewer quality score (0-100)
			reviewerScore.QualityScore = (reviewerScore.DetectionAccuracy * 40) +
				(reviewerScore.DefectFindRate * 30) +
				(reviewerScore.CostEfficiency * 30)

			if reviewerScore.QualityScore < 0 {
				reviewerScore.QualityScore = 0
			}
			if reviewerScore.QualityScore > 100 {
				reviewerScore.QualityScore = 100
			}

			if err := m.UpdateQualityScore(reviewerScore); err != nil {
				return fmt.Errorf("failed to update reviewer score: %w", err)
			}
		}

		return nil
	})
}

// Helper function

// nullInt converts an int to sql.NullInt64
func nullInt(i int) sql.NullInt64 {
	if i == 0 {
		return sql.NullInt64{Valid: false}
	}
	return sql.NullInt64{
		Int64: int64(i),
		Valid: true,
	}
}
