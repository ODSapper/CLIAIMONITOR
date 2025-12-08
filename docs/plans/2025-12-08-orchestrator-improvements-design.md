# CLIAIMONITOR Orchestrator Improvements Design

**Date**: 2025-12-08
**Status**: Draft

## Overview

Evolve CLIAIMONITOR into a self-sufficient orchestration system with Captain as team lead for `team-coop`. The dashboard becomes agent-centric, showing each agent's task queue and comprehensive metrics. Captain handles full git lifecycle automation.

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                    CLIAIMONITOR Dashboard (:3000)                   │
├─────────────────────────────────────────────────────────────────────┤
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │                   Agent Task Queues                          │   │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐    │   │
│  │  │ Captain  │  │ SNTGreen │  │ SNTPurple│  │ SNTRed   │    │   │
│  │  │ (Coord)  │  │ (Dev)    │  │ (Review) │  │ (Eng)    │    │   │
│  │  │ ──────── │  │ ──────── │  │ ──────── │  │ ──────── │    │   │
│  │  │ Task-003 │  │ Task-001 │  │ idle     │  │ Task-002 │    │   │
│  │  │ planning │  │ coding   │  │          │  │ testing  │    │   │
│  │  └──────────┘  └──────────┘  └──────────┘  └──────────┘    │   │
│  └─────────────────────────────────────────────────────────────┘   │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │  Metrics: Tokens | Progress | Health                         │   │
│  └─────────────────────────────────────────────────────────────┘   │
├─────────────────────────────────────────────────────────────────────┤
│  Task Sources: Captain Chat │ Dashboard │ CLI │ plans/*.yaml       │
└─────────────────────────────────────────────────────────────────────┘
```

### Key Concepts

- **Unified Task Queue**: All sources feed into priority-sorted queue
- **Agent Lanes**: Each agent has visible task queue and status
- **Captain Role**: Coordinates work, spawns review agents, manages git
- **Primary Input**: You describe goals to Captain, Captain plans and executes

## Task Lifecycle

### States

```
pending → assigned → in_progress → review → approved → merged
                         ↓            ↓
                      blocked      changes_requested
```

### Captain's Responsibilities

1. **Task Intake** - Receives goals from you, breaks into tasks
2. **Assignment** - Picks highest priority task, selects best-fit agent
3. **Branch Creation** - Creates `task/{task-id}-{description}` branch
4. **Progress Tracking** - Monitors agent activity, detects stuck agents
5. **Review Orchestration** - Runs tests/linters, spawns review agent
6. **PR & Merge** - Creates PR when approved, merges when CI passes
7. **Metrics Collection** - Tracks tokens, time for the team

### Agent Workflow

1. Receives task assignment via NATS
2. Works on assigned branch
3. Commits and signals "done" to Captain
4. Responds to change requests if any
5. Moves to next task when Captain approves

## Dashboard UI

### Agent-Centric Layout

```
┌─────────────────────────────────────────────────────────────────────────┐
│  CLIAIMONITOR                                    [+ New Task] [Settings]│
├─────────────────────────────────────────────────────────────────────────┤
│  Summary: 3 active │ 2 pending │ 1 in review │ 47,230 tokens today      │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌─────────────┐       │
│  │ ● Captain   │ │ ● SNTGreen │ │ ○ SNTPurple │ │ ● SNTRed    │       │
│  │   (coord)   │ │   (dev)     │ │   (review)  │ │   (eng)     │       │
│  ├─────────────┤ ├─────────────┤ ├─────────────┤ ├─────────────┤       │
│  │ ▶ TASK-003  │ │ ▶ TASK-001  │ │             │ │ ▶ TASK-002  │       │
│  │   planning  │ │   coding    │ │   idle      │ │   testing   │       │
│  │   5m 23s    │ │   12m 41s   │ │             │ │   3m 02s    │       │
│  ├─────────────┤ ├─────────────┤ │             │ ├─────────────┤       │
│  │ ◦ TASK-007  │ │ ◦ TASK-004  │ │             │ │             │       │
│  │ ◦ TASK-009  │ │             │ │             │ │             │       │
│  └─────────────┘ └─────────────┘ └─────────────┘ └─────────────┘       │
│                                                                         │
├─────────────────────────────────────────────────────────────────────────┤
│  Pending Queue (by priority)                                            │
│  ┌────┬──────────────────────────────────┬──────┬────────┬───────────┐ │
│  │ P1 │ Fix authentication bypass        │ MAH  │ unassigned │ 2h ago │ │
│  │ P3 │ Add rate limiting to API         │ MSS  │ unassigned │ 1d ago │ │
│  │ P5 │ Update dashboard colors          │ local│ unassigned │ 3d ago │ │
│  └────┴──────────────────────────────────┴──────┴────────┴───────────┘ │
└─────────────────────────────────────────────────────────────────────────┘
```

### UI Elements

- **Agent Cards**: Live status (●/○), current task, time elapsed, queued tasks
- **Summary Bar**: Quick stats across all agents
- **Pending Queue**: Priority-sorted unassigned tasks
- **Click Agent**: Expands to detailed metrics, activity, history

## Metrics System

### Efficiency Metrics

| Metric | Description | Display |
|--------|-------------|---------|
| Tokens per task | Average tokens to complete a task | Per agent + team |
| Cost estimate | Based on model pricing | Daily/weekly |
| Token velocity | Tokens/minute while active | Trend line |

### Progress Metrics

| Metric | Description | Display |
|--------|-------------|---------|
| Tasks completed | Count by timeframe | Today/week/total |
| Queue depth | Pending + assigned tasks | Per agent + total |
| Avg completion time | Assigned → merged | Per priority |
| Throughput | Tasks per hour | Rolling average |

### Health Metrics

| Metric | Description | Display |
|--------|-------------|---------|
| Agent status | idle / working / stuck / error | Live indicator |
| Time in state | Duration in current status | Warning threshold |
| Failed tests | Test failures per task | Count + trend |
| Review rejections | Changes requested count | Per agent |
| Last activity | Time since last action | Stale detection |

### Storage

- SQLite tables extending existing schema
- Historical data for trend analysis
- Configurable alert thresholds

## Task Input

### Primary Flow

```
You: "Add rate limiting to the API"
    ↓
Captain: Creates plan, breaks into tasks:
    - TASK-001: Research existing middleware (P3)
    - TASK-002: Implement rate limiter (P2)
    - TASK-003: Add tests (P3)
    - TASK-004: Update docs (P5)
    ↓
Captain: Assigns to agents, manages execution
```

### Task Sources

| Source | Use Case |
|--------|----------|
| **Captain Chat** | Primary. Describe goal, Captain plans |
| **Dashboard Board** | Visual overview, quick tasks, reprioritize |
| **CLI** | Quick one-off tasks |
| **Files** | Batch import via `plans/*.yaml` |

### Task Schema

```yaml
id: "TASK-001"
title: "Fix authentication bypass"
description: "Optional longer description"
priority: 1-7  # 1=critical, 7=background
source: "captain" | "dashboard" | "cli" | "file"
repo: "MAH" | "MSS" | "local"
requirements:
  - text: "Add input validation"
    required: true
status: pending | assigned | in_progress | review | approved | merged
assigned_to: "SNTGreen" | null
branch: "task/TASK-001-fix-auth" | null
created_at: timestamp
```

## Git Workflow

### Full Automation Flow

```
Captain receives task
       ↓
git checkout -b task/{id}-{slug}
       ↓
Assign to agent (works on branch)
       ↓
Agent commits, signals done
       ↓
Captain runs: tests, lint, build
       ↓ pass                    ↓ fail
Spawn review agent         Request changes → agent fixes
       ↓
Review passes
       ↓
git push, gh pr create
       ↓
CI passes
       ↓
gh pr merge --squash
       ↓
Task → merged
```

### Branch Naming

`task/{TASK-ID}-{short-description}`

Example: `task/TASK-001-fix-auth-bypass`

### PR Template

```markdown
## Summary
{Captain-generated summary of changes}

## Tasks Completed
- [x] TASK-001: Description

## Agents Involved
- SNTGreen (implementation)
- SNTPurple (review)

## Metrics
- Tokens used: 23,450
- Time: 45m

🤖 Generated by CLIAIMONITOR team-coop
```

### Merge Strategy

Squash merge for clean history. One commit per task.

### Error Handling

If CI fails post-PR, Captain reopens task and assigns agent to fix.

## Implementation Scope

### New Components

| Component | Description |
|-----------|-------------|
| `internal/tasks/` | Task queue, schema, priority sorting |
| `internal/git/` | Branch, PR, merge automation |
| `internal/planner/` | Captain planning logic |
| `web/taskboard/` | Agent-centric dashboard UI |

### Modified Components

| Component | Changes |
|-----------|---------|
| `internal/captain/` | Task assignment, review orchestration, git coordination |
| `internal/metrics/` | Efficiency/progress metrics, historical storage |
| `internal/handlers/` | API endpoints for tasks, metrics |
| `web/` | Replace dashboard with agent-centric view |
| Database | New tables: `tasks`, `task_metrics`, `task_history` |

### Unchanged

- NATS messaging
- Agent spawning
- MCP tools (extend only)

### New MCP Tools

| Tool | Purpose |
|------|---------|
| `get_assigned_task` | Agent fetches current task |
| `signal_task_done` | Agent signals completion |
| `request_task_change` | Agent flags blockers |

## Next Steps

1. Create implementation plan with detailed tasks
2. Set up git worktree for isolated development
3. Begin implementation phase by phase
