# Website Fixes Implementation Plan

**Date**: 2025-12-03
**Project**: Planner Dashboard
**Status**: Ready for Execution

---

## Summary of Issues Found

### 1. Backend Bugs

| Bug | Location | Issue | Fix |
|-----|----------|-------|-----|
| `/scores/team/{id}` type mismatch | `index.go:2607-2631` | `completion_points` is DECIMAL in DB but `int` in Go struct | Change struct fields to `float64` or use SQL CAST |
| Score display in HandleGetTeamScore | `index.go:2237-2253` | TeamScore struct uses `int` for bonus fields | Use intermediate float64 scan vars |

### 2. Teams Needing Rename

11 teams have auto-generated `(auto)` suffix names that need proper display names:

| Team ID | Current Name | Suggested Name |
|---------|--------------|----------------|
| claude-code | claude-code (auto) | Claude Code |
| team-captain | team-captain (auto) | Captain |
| team-haiku1 | team-haiku1 (auto) | Haiku Alpha |
| team-haiku2 | team-haiku2 (auto) | Haiku Beta |
| team-opusgreen007 | team-opusgreen007 (auto) | Opus Green 007 |
| team-opusgreen008 | team-opusgreen008 (auto) | Opus Green 008 |
| team-sntblack | team-sntblack (auto) | Sonnet Black |
| team-sntgreen001 | team-sntgreen001 (auto) | Sonnet Green 001 |
| team-sntgreen002 | team-sntgreen002 (auto) | Sonnet Green 002 |
| team-sntgreen003 | team-sntgreen003 (auto) | Sonnet Green 003 |
| team-sntred001 | team-sntred001 (auto) | Sonnet Red 001 |
| team-sntred002 | team-sntred002 (auto) | Sonnet Red 002 |
| team-sonnet1 | team-sonnet1 (auto) | Sonnet Alpha |
| team-testcase | team-testcase (auto) | Test Case |

### 3. Token Tracking

- Current: Shows 0 for all teams except team-opusgreen (200000)
- Issue: Tokens not being recorded during task workflow
- Fix: Verify workflow records tokens in `team_metrics` table

---

## Implementation Tasks

### Task 1: Fix TeamScore Type Mismatch (SONNET)

**File**: `apps/mtls-api/index.go`

**Problem**: Database stores `completion_points`, `efficiency_bonus`, `quality_bonus`, `velocity_bonus` as DECIMAL but Go struct uses `int`.

**Solution**: Use intermediate float64 variables when scanning, then convert to int.

**Changes at line 2607-2631**:

```go
// HandleGetTeamScore handles GET /api/scores/team/{id}
func HandleGetTeamScore(w http.ResponseWriter, r *http.Request, teamID string) {
	var score TeamScore

	// Use float64 for database DECIMAL columns
	var completionPts, efficiencyBonus, qualityBonus, velocityBonus, overallScore float64

	err := db.QueryRow(`
		SELECT team_id, completion_points, efficiency_bonus, quality_bonus, velocity_bonus,
		       overall_score, tasks_completed, tasks_claimed, tasks_reviewed,
		       COALESCE(avg_tokens_per_task, 0), COALESCE(avg_review_score, 0),
		       COALESCE(avg_completion_hours, 0), COALESCE(rank, 0), updated_at
		FROM team_scores WHERE team_id = $1
	`, teamID).Scan(
		&score.TeamID, &completionPts, &efficiencyBonus, &qualityBonus,
		&velocityBonus, &overallScore, &score.TasksCompleted, &score.TasksClaimed,
		&score.TasksReviewed, &score.AvgTokensPerTask, &score.AvgReviewScore,
		&score.AvgCompletionHrs, &score.Rank, &score.UpdatedAt,
	)

	// Convert floats to ints for struct
	score.CompletionPoints = int(completionPts)
	score.EfficiencyBonus = int(efficiencyBonus)
	score.QualityBonus = int(qualityBonus)
	score.VelocityBonus = int(velocityBonus)
	score.OverallScore = int(overallScore)

	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "not_found", "No score data for this team")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database_error", err.Error())
		return
	}

	json.NewEncoder(w).Encode(score)
}
```

**Verification**:
```bash
curl -s "https://plannerprojectmss.vercel.app/api/v1/scores/team/team-sonnet1" | python -m json.tool
```

---

### Task 2: Rename Auto-Generated Teams (HAIKU)

Use API to update team names:

```bash
# Rename all auto-generated teams
curl -X PUT "https://plannerprojectmss.vercel.app/api/v1/teams/claude-code" \
  -H "X-API-Key: team-captain" -H "Content-Type: application/json" \
  -d '{"name":"Claude Code"}'

curl -X PUT "https://plannerprojectmss.vercel.app/api/v1/teams/team-captain" \
  -H "X-API-Key: team-captain" -H "Content-Type: application/json" \
  -d '{"name":"Captain"}'

curl -X PUT "https://plannerprojectmss.vercel.app/api/v1/teams/team-haiku1" \
  -H "X-API-Key: team-captain" -H "Content-Type: application/json" \
  -d '{"name":"Haiku Alpha"}'

curl -X PUT "https://plannerprojectmss.vercel.app/api/v1/teams/team-haiku2" \
  -H "X-API-Key: team-captain" -H "Content-Type: application/json" \
  -d '{"name":"Haiku Beta"}'

curl -X PUT "https://plannerprojectmss.vercel.app/api/v1/teams/team-opusgreen007" \
  -H "X-API-Key: team-captain" -H "Content-Type: application/json" \
  -d '{"name":"Opus Green 007"}'

curl -X PUT "https://plannerprojectmss.vercel.app/api/v1/teams/team-opusgreen008" \
  -H "X-API-Key: team-captain" -H "Content-Type: application/json" \
  -d '{"name":"Opus Green 008"}'

curl -X PUT "https://plannerprojectmss.vercel.app/api/v1/teams/team-sntblack" \
  -H "X-API-Key: team-captain" -H "Content-Type: application/json" \
  -d '{"name":"Sonnet Black"}'

curl -X PUT "https://plannerprojectmss.vercel.app/api/v1/teams/team-sntgreen001" \
  -H "X-API-Key: team-captain" -H "Content-Type: application/json" \
  -d '{"name":"Sonnet Green 001"}'

curl -X PUT "https://plannerprojectmss.vercel.app/api/v1/teams/team-sntgreen002" \
  -H "X-API-Key: team-captain" -H "Content-Type: application/json" \
  -d '{"name":"Sonnet Green 002"}'

curl -X PUT "https://plannerprojectmss.vercel.app/api/v1/teams/team-sntgreen003" \
  -H "X-API-Key: team-captain" -H "Content-Type: application/json" \
  -d '{"name":"Sonnet Green 003"}'

curl -X PUT "https://plannerprojectmss.vercel.app/api/v1/teams/team-sntred001" \
  -H "X-API-Key: team-captain" -H "Content-Type: application/json" \
  -d '{"name":"Sonnet Red 001"}'

curl -X PUT "https://plannerprojectmss.vercel.app/api/v1/teams/team-sntred002" \
  -H "X-API-Key: team-captain" -H "Content-Type: application/json" \
  -d '{"name":"Sonnet Red 002"}'

curl -X PUT "https://plannerprojectmss.vercel.app/api/v1/teams/team-sonnet1" \
  -H "X-API-Key: team-captain" -H "Content-Type: application/json" \
  -d '{"name":"Sonnet Alpha"}'

curl -X PUT "https://plannerprojectmss.vercel.app/api/v1/teams/team-testcase" \
  -H "X-API-Key: team-captain" -H "Content-Type: application/json" \
  -d '{"name":"Test Case"}'
```

**Verification**:
```bash
curl -s "https://plannerprojectmss.vercel.app/api/v1/teams" | python -m json.tool | grep -A2 "team-sonnet1"
```

---

### Task 3: Token Tracking Validation (SONNET)

**Issue**: Token metrics not being recorded. Need to verify the workflow path.

**Investigation Steps**:

1. Check `recordTaskCompletionMetrics` function for proper token recording
2. Verify `team_metrics` table receives data during task completion
3. Ensure `tokens_used` or `ending_tokens` is passed in the `/implemented` request

**Key Code Path** (`index.go`):
- Line 1794: `recordTaskCompletionMetrics` - async metrics recording
- Line 2127: `updateTeamAggregates` - triggers score recalculation

**Fix**: If tokens aren't being recorded, check:
1. The `/implemented` endpoint is receiving `tokens_used` parameter
2. The `team_metrics` INSERT is executing correctly
3. The async goroutine completes before serverless timeout

**Test**:
```bash
# Mark a test task as implemented with token tracking
curl -X POST "https://plannerprojectmss.vercel.app/api/v1/tasks/TEST-001/implemented" \
  -H "X-API-Key: team-testcase" -H "Content-Type: application/json" \
  -d '{
    "team_id": "team-testcase",
    "branch": "test/token-tracking",
    "tokens_used": 50000
  }'

# Check if tokens were recorded
curl -s "https://plannerprojectmss.vercel.app/api/v1/scores/team/team-testcase" | python -m json.tool
```

---

### Task 4: Comprehensive API Testing (HAIKU)

Test all critical endpoints:

```bash
# 1. Health check
curl -s "https://plannerprojectmss.vercel.app/api/v1/health" | python -m json.tool

# 2. Stats
curl -s "https://plannerprojectmss.vercel.app/api/v1/stats" | python -m json.tool

# 3. Leaderboard
curl -s "https://plannerprojectmss.vercel.app/api/v1/leaderboard" | python -m json.tool

# 4. Teams list
curl -s "https://plannerprojectmss.vercel.app/api/v1/teams" -H "X-API-Key: team-captain" | python -m json.tool

# 5. Team score (after fix)
curl -s "https://plannerprojectmss.vercel.app/api/v1/scores/team/team-sonnet1" | python -m json.tool

# 6. Recalculate all scores
curl -X POST "https://plannerprojectmss.vercel.app/api/v1/scores/recalculate-all" -H "X-API-Key: team-captain"
```

---

## Execution Order

1. **Task 1** (SONNET): Fix TeamScore type mismatch in `index.go`
2. **Task 2** (HAIKU): Rename all auto-generated teams via API
3. **Task 3** (SONNET): Validate and fix token tracking workflow
4. **Task 4** (HAIKU): Comprehensive API testing and verification

---

## Deployment

After code changes:

```bash
cd "C:\Users\Admin\Documents\VS Projects\planner"
git add apps/mtls-api/index.go
git commit -m "fix: TeamScore decimal scan and token tracking

- Use float64 intermediates for DECIMAL columns in HandleGetTeamScore
- Ensures /scores/team/{id} endpoint works correctly
- Token tracking verification

🤖 Generated with Claude Code"
git push origin main
```

Vercel will auto-deploy on push to main.

---

## Success Criteria

- [ ] `/scores/team/{id}` returns valid JSON without errors
- [ ] All teams have proper display names (no "(auto)" suffix)
- [ ] Token tracking shows non-zero values for active teams
- [ ] Leaderboard displays correctly with all metrics
- [ ] All API endpoints respond without errors
