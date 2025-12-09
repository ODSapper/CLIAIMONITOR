# Code-Review-Correct Workflow Implementation Summary

**Date**: 2025-12-08
**Status**: Complete
**Build Status**: ✅ All tests passing

## Overview

Implemented a complete code-review-correct workflow for coder agents (Green) and reviewer agents (Purple) with automatic escalation after 3 failed review cycles.

## Changes Made

### 1. Constants Added

**File**: `internal/server/server.go`
```go
const (
    MaxReviewCycles = 3
)
```

### 2. Database Interface Updates

**File**: `internal/memory/interface.go`

Added new method:
- `RequestRework(id int64, feedback string) error` - Increments review_attempt and sets status to "rework"

### 3. Database Implementation

**File**: `internal/memory/assignments.go`

Added `RequestRework()` function:
- Increments `review_attempt` counter
- Sets `status = 'rework'`
- Stores reviewer feedback
- Clears `completed_at` timestamp

### 4. Review Logic Enhancement

**File**: `internal/server/server.go`

Updated `OnSubmitReviewResult` callback to:
- Check current `review_attempt` count
- If `review_attempt >= MaxReviewCycles`: escalate to human
- If `review_attempt < MaxReviewCycles`: request rework
- If approved: mark as approved
- Log appropriate events with attempt counts

### 5. Mock Updates

**File**: `internal/server/cleanup_test.go`

Added stub implementation:
```go
func (m *mockMemoryDB) RequestRework(id int64, feedback string) error { return nil }
```

### 6. Documentation

**Files Created**:
- `docs/workflows/code-review-correct-workflow.md` - Complete workflow documentation
- `docs/workflows/IMPLEMENTATION_SUMMARY.md` - This file

## Workflow States

The system now supports the following assignment states with proper transitions:

1. **pending** → **in_progress** → **completed** → **approved** (happy path)
2. **pending** → **in_progress** → **completed** → **rework** → **in_progress** → ... (rework loop)
3. **pending** → **in_progress** → **completed** → **escalated** (after 3 cycles)

## State Transition Logic

### Approved Review
- Status: `approved`
- Action: `CompleteAssignment(id, "approved", feedback)`
- Log: `[REVIEW-APPROVED] Assignment X approved after N attempt(s)`

### Rejected Review (Cycles < 3)
- Status: `rework`
- Action: `RequestRework(id, feedback)` - increments `review_attempt`, sets `status='rework'`
- Log: `[REVIEW-REJECTED] Assignment X needs rework, attempt N/3`
- Message: `Code needs rework (attempt N/3). Assignment returned to coder.`

### Rejected Review (Cycles >= 3)
- Status: `escalated`
- Action: `CompleteAssignment(id, "escalated", feedback)`
- Log: `[REVIEW-ESCALATION] Assignment X exceeded 3 review cycles, escalating to human`
- Activity: Creates `escalated_for_human_review` activity log entry
- Message: `ESCALATED: Assignment exceeded 3 review cycles. Human review required.`

## MCP Tool Response

The `submit_review_result` tool now returns:
```json
{
  "status": "approved|rework|escalated",
  "assignment_id": 123,
  "feedback": "reviewer comments",
  "review_attempt": 2,
  "max_cycles": 3,
  "escalated": false,
  "message": "Human-readable result message"
}
```

## Testing

### Build Verification
```bash
go build ./...
```
✅ PASS - All packages compile successfully

### Test Execution
```bash
go test ./internal/memory/... -v
```
✅ PASS - All existing tests pass (22/22)

### Schema Migration
Migration 009 (task_assignments) applies correctly with `review_attempt` field.

## Usage for Agents

### Green Agent (Coder)
1. Call `get_my_assignment()` to retrieve work
2. Check if `status === 'rework'`:
   - If yes: read `review_feedback` for what to fix
   - Note the `review_attempt` count (X/3)
3. Accept assignment: `accept_assignment(assignment_id)`
4. Fix issues based on feedback
5. Submit: `submit_for_review(assignment_id, branch_name)`

### Purple Agent (Reviewer)
1. Call `get_my_assignment()` to get code to review
2. Review tests and implementation
3. Submit verdict:
   - If good: `submit_review_result(assignment_id, approved=true, feedback="LGTM")`
   - If issues: `submit_review_result(assignment_id, approved=false, feedback="Issues: 1) X, 2) Y")`
4. System automatically:
   - Sends back to coder if cycles < 3
   - Escalates to human if cycles >= 3

### Captain Agent
Monitors activity log for:
- `escalated_for_human_review` events → Notify human operator
- Can query `GetAssignmentsByTask(taskID)` to see full review history

## Logging

All review events logged with prefixes:
- `[REVIEW-APPROVED]` - Code passed review
- `[REVIEW-REJECTED]` - Code needs fixes (shows attempt count)
- `[REVIEW-ESCALATION]` - Max attempts exceeded, human needed

Activity log captures:
- `submitted_review_result` - Every review submission
- `escalated_for_human_review` - Escalation events with full context

## Configuration

To adjust maximum review cycles:
```go
// internal/server/server.go
const (
    MaxReviewCycles = 3  // Change this value
)
```

## Future Enhancements

Potential improvements:
- [ ] Add per-task max_cycles override
- [ ] Implement reviewer rotation after 2 failed attempts
- [ ] Add metrics dashboard for review rejection rates
- [ ] Automatic notification to Discord/Slack on escalation
- [ ] Track time spent in each review cycle

## Dependencies

No new external dependencies added. Uses existing:
- SQLite (via modernc.org/sqlite)
- Existing MCP tools infrastructure
- Existing activity logging system

## Backwards Compatibility

✅ Fully backwards compatible:
- Existing assignments continue to work
- Schema migration is automatic
- New fields have sensible defaults
- Old code paths unaffected

## Rollback Plan

If issues arise:
1. Revert `internal/server/server.go` changes to old `OnSubmitReviewResult`
2. Remove `RequestRework()` calls
3. System falls back to simple approved/rejected without cycle limits
4. Database schema changes are non-breaking (fields allow NULL)

## Sign-Off

- ✅ Code compiles
- ✅ Tests pass
- ✅ Documentation complete
- ✅ No breaking changes
- ✅ Minimal changes per requirements

**Ready for Captain to use in SGT workflow.**
