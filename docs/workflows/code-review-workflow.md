# Code Review Workflow

## Overview

All code changes follow this workflow to ensure quality before merge:

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   GREEN     │────►│   PURPLE    │────►│   CAPTAIN   │
│   (Coder)   │     │  (Reviewer) │     │   (Final)   │
└─────────────┘     └─────────────┘     └─────────────┘
      │                   │                    │
      │ implements        │ reviews            │ approves
      │ feature/fix       │ code               │ & merges
      ▼                   ▼                    ▼
   Branch            Approve/Request      Merge PR
   + Commits         Changes              to main
```

## Participants

| Role | Agent Type | Responsibility |
|------|------------|----------------|
| Coder | Green (SNTGreen, OpusGreen) | Implement features/fixes |
| Reviewer | Purple (SNTPurple, OpusPurple) | Review code quality |
| Orchestrator | Captain | Final review, merge decision |

## Workflow Steps

### 1. Captain Assigns Task
- Captain receives task from queue or human
- Captain spawns Green agent with task description
- Green agent registers and begins work

### 2. Green Implements
- Green creates branch: `task/{TASK-ID}-description`
- Green implements feature/fix
- Green runs tests locally
- Green commits changes
- Green calls `signal_captain` with signal="completed"

### 3. Purple Reviews
- Captain spawns Purple agent to review Green's work
- Purple checks:
  - Code correctness and logic
  - Security vulnerabilities
  - Test coverage
  - Style and conventions
- If APPROVED: Purple calls `signal_captain` with signal="completed" and notes approval
- If CHANGES NEEDED: Purple calls `signal_captain` with signal="blocked" and lists issues

### 4. Iteration (if needed)
- If Purple requests changes:
  - Captain notifies Green of issues
  - Green makes corrections
  - Green signals completion again
  - Purple re-reviews
  - Loop until approved (max 3 cycles, then escalate)

### 5. Captain Final Review
- Captain receives Purple's approval
- Captain does final sanity check
- Captain creates PR (or approves existing)
- Captain merges to main

## MCP Tools Used

### Green Agent
- `register_agent` - On startup
- `report_status` - During work (status="working")
- `report_metrics` - After significant work (tokens, tests)
- `log_activity` - On commits and milestones
- `signal_captain` - On completion or blockers

### Purple Agent
- `register_agent` - On startup
- `report_status` - During review
- `log_activity` - Log issues found
- `signal_captain` - On approval or rejection

### Captain
- `get_agent_list` - Monitor active agents
- `get_agent_metrics` - Track costs
- `escalate_alert` - If workflow stuck

## Escalation Path

| Condition | Action |
|-----------|--------|
| 3+ review rejections | Escalate to human |
| Agent blocked >10min | Captain investigates |
| Security issue found | Immediate human alert |
| Tests failing | Block merge, notify Green |

## Example Flow

```
1. Captain: "SNTGreen, implement user authentication for task TASK-123"
2. SNTGreen: register_agent, report_status("working"), implements, signal_captain("completed")
3. Captain: "SNTPurple, review SNTGreen's auth implementation"
4. SNTPurple: register_agent, reviews code, signal_captain("completed", "APPROVED - code looks good")
5. Captain: Reviews, creates PR, merges
```
