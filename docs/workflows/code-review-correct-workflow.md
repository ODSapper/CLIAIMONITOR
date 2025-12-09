# Code-Review-Correct Workflow

## Overview

This document describes the code-review-correct workflow implemented in CLIAIMONITOR for SGT (Sergeant) agents.

## Workflow States

The workflow supports the following assignment states:

1. **pending** - Task assigned, waiting to be accepted
2. **in_progress** - Agent is working on the task
3. **completed** - Work submitted for review
4. **rework** - Reviewer rejected, needs corrections (increments review_attempt)
5. **approved** - Reviewer approved, ready to merge
6. **escalated** - Exceeded max review cycles, requires human intervention

## Review Cycle Limits

- **MaxReviewCycles**: 3 (defined in `internal/server/server.go`)
- After 3 failed review cycles, the assignment is automatically escalated to human review
- The `review_attempt` counter increments each time code is sent back for rework

## State Transitions

### Happy Path (Approved on First Try)
```
pending → in_progress → completed → approved
```

### Rework Loop (Corrections Needed)
```
pending → in_progress → completed → rework → in_progress → completed → approved
                                      ↑______(attempt 2)_____↓
```

### Escalation (Max Cycles Exceeded)
```
pending → in_progress → completed → rework → in_progress → completed → rework → in_progress → completed → escalated
                                      ↑______(attempt 2)_____↓          ↑______(attempt 3)_____↓
```

## Implementation Details

### Database Schema

The `task_assignments` table tracks:
- `review_attempt` (INTEGER) - Current review cycle (1-indexed)
- `review_feedback` (TEXT) - Feedback from reviewer
- `status` (TEXT) - Current state

### Key Functions

**`RequestRework(id int64, feedback string)`** (`internal/memory/assignments.go`)
- Increments `review_attempt` by 1
- Sets status to "rework"
- Stores reviewer feedback
- Clears `completed_at` timestamp

**`OnSubmitReviewResult`** (`internal/server/server.go`)
- Checks current `review_attempt` count
- If `review_attempt >= MaxReviewCycles`: escalate to human
- If `review_attempt < MaxReviewCycles`: request rework
- If approved: mark as approved

### MCP Tools

**`submit_review_result`**
- Parameters:
  - `assignment_id` (number) - The assignment being reviewed
  - `approved` (boolean) - Whether code passes review
  - `feedback` (string) - Review comments (required if not approved)
- Returns:
  - `status`: "approved", "rework", or "escalated"
  - `review_attempt`: Current attempt number
  - `max_cycles`: Maximum allowed cycles (3)
  - `escalated`: Boolean indicating if escalated
  - `message`: Human-readable result

**`get_my_assignment`**
- Returns the current active assignment for an agent
- Includes assignments in states: "pending", "accepted", "in_progress", "rework"

## Usage Example

### For Green (Coder) Agent

```javascript
// 1. Get current assignment
const result = await mcp.call("get_my_assignment");
const assignment = result.assignment;

// 2. Check if this is a rework
if (assignment.status === "rework") {
  console.log(`Rework needed (attempt ${assignment.review_attempt}/${maxCycles})`);
  console.log(`Feedback: ${assignment.review_feedback}`);
}

// 3. Accept and work on it
await mcp.call("accept_assignment", { assignment_id: assignment.id });

// 4. Submit for review when done
await mcp.call("submit_for_review", {
  assignment_id: assignment.id,
  branch_name: "task/TASK-001-feature"
});
```

### For Purple (Reviewer) Agent

```javascript
// 1. Get assignment to review
const result = await mcp.call("get_my_assignment");
const assignment = result.assignment;

// 2. Review the code
// ... review logic here ...

// 3a. If approved
await mcp.call("submit_review_result", {
  assignment_id: assignment.id,
  approved: true,
  feedback: "LGTM - tests are meaningful, implementation is correct"
});

// 3b. If rejected
await mcp.call("submit_review_result", {
  assignment_id: assignment.id,
  approved: false,
  feedback: "Issues: 1) Test doesn't validate edge case X, 2) Missing null check in Y"
});
```

## Monitoring

### Logs

Look for these log prefixes:
- `[REVIEW-APPROVED]` - Code approved
- `[REVIEW-REJECTED]` - Code needs rework, showing attempt X/3
- `[REVIEW-ESCALATION]` - Max cycles exceeded, escalated to human

### Activity Log

All review events are logged to the activity log with actions:
- `submitted_review_result` - Normal review completion
- `escalated_for_human_review` - Escalation event with full context

## Configuration

To change the maximum review cycles, update the constant in `internal/server/server.go`:

```go
const (
    MaxReviewCycles = 3  // Change this value
)
```

## Future Enhancements

Potential improvements:
- Per-task or per-project max cycles configuration
- Automatic re-assignment to different reviewer after 2 cycles
- Metrics tracking review rejection rates per agent
- Dashboard visualization of review pipeline
