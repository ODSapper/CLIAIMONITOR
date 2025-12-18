# Review Board MCP Tool Wiring Summary

## Overview
Successfully wired up all 6 Review Board MCP tool callbacks in `internal/server/server.go`.

## Callbacks Added (lines 1381-1555)

### 1. OnCreateReviewBoard
- **Purpose**: Creates a new review board for code review
- **Validates**: Reviewer count (1-5), risk level defaults to "medium"
- **Returns**: Board ID, assignment ID, reviewer count, status, and confirmation message
- **Logs**: Activity to dashboard with board details

### 2. OnSubmitDefect
- **Purpose**: Records a defect found during code review
- **Handles**: Required fields (category, severity, title, description) and optional fields (file path, line numbers, suggested fix)
- **Returns**: Defect ID, board ID, category, severity, and status
- **Stores**: Complete defect record in memory.db

### 3. OnRecordReviewerVote
- **Purpose**: Records an individual reviewer's vote on code quality
- **Tracks**: Approval status, confidence score, defects found, tokens used
- **Returns**: Vote ID, board ID, reviewer ID, approval status
- **Logs**: Reviewer verdict to dashboard activity log

### 4. OnFinalizeBoard
- **Purpose**: Calculates consensus and finalizes the review board
- **Process**:
  1. Calls `CalculateConsensus()` to aggregate reviewer votes
  2. Updates board status to "completed"
  3. Sets final verdict and aggregated feedback
  4. Updates quality scores for participating reviewers
- **Returns**: Comprehensive consensus data (decision, votes, defect counts, feedback)
- **Logs**: Final verdict with vote breakdown

### 5. OnGetAgentLeaderboard
- **Purpose**: Retrieves quality score leaderboard for reviewers
- **Filters**: By role (optional), with configurable limit (default: 20)
- **Returns**: Leaderboard array with agent scores

### 6. OnGetDefectCategories
- **Purpose**: Retrieves list of valid defect categories from database
- **Returns**: Categories array with count

## Helper Method Added

### logActivity(action, details string)
- **Location**: Lines 1791-1800
- **Purpose**: Consistent activity logging for Review Board operations
- **Format**: Uses system agent ID, generates unique IDs with nanosecond timestamps

## Integration Points

1. **Database Layer**: All callbacks use `s.memDB` for persistence
2. **Activity Logging**: Uses `s.store.AddActivity()` for dashboard updates
3. **Consensus Calculation**: Relies on `memory.CalculateConsensus()` for vote aggregation
4. **Quality Scoring**: Updates agent quality scores via `memory.UpdateQualityScoresAfterReview()`

## Validation

- ✅ Code compiles without errors
- ✅ All 6 callbacks match MCP handler signatures
- ✅ Error handling follows existing patterns
- ✅ Activity logging integrated with dashboard
- ✅ Database operations properly called

## Next Steps

Captain can now dispatch Purple SGT with review tasks, which will:
1. Create a review board with `create_review_board`
2. Spawn multiple sub-agent reviewers (Haiku/Sonnet)
3. Each reviewer uses `submit_defect` to log findings
4. Each reviewer uses `record_reviewer_vote` to submit verdict
5. Purple SGT uses `finalize_board` to calculate consensus
6. Dashboard shows leaderboard via `get_agent_leaderboard`

## Files Modified

- `internal/server/server.go`: Added 6 callbacks + helper method (~180 lines)

