# Snake Agent Force Design

**Date**: 2025-12-02
**Status**: Approved
**Owner**: Captain (Orchestrator Agent)

---

## Overview

Build the Magnolia Elite Agent Force - a military-style AI agent hierarchy with the Captain (Orchestrator) commanding Snake reconnaissance agents and Worker execution agents.

### Vision

```
Future: General (spawns Captains)
            ↓
Now:    Captain (Orchestrator) - Magnolia Elite Agent Force
            ↓ spawns
        Snake agents (recon/special ops)
        Worker agents (coding/review/testing)
```

### Use Cases

1. **Internal (Magnolia dev)**: Captain audits code, coordinates Snakes for security scanning, directs Workers for implementation across MAH, MSS, MSS-AI

2. **External (customer engagement)**: Captain deployed to new network, spawns Snake for recon, receives findings, recommends Magnolia solutions, spawns Workers to implement

---

## Command Structure

### Agent Hierarchy

| Agent | Model | Role | Reports To |
|-------|-------|------|------------|
| Captain | Opus | Strategic decisions, coordination | Human |
| Snake001-999 | Opus | Reconnaissance, security audits, special ops | Captain |
| SNTGreen/Purple/Red | Sonnet | Coding, review, testing | Captain |
| OpusGreen/Purple/Red | Opus | Complex coding, architecture | Captain |

### Operational Modes

Captain selects mode based on mission requirements:

**Mode 1: Direct Control**
- Captain spawns agents with specific instructions
- Used for: Critical security, sensitive ops, unfamiliar codebases
- Risk: High | Familiarity: Low

**Mode 2: Task Dispatch**
- Captain creates tasks in Planner API, agents self-assign
- Used for: Routine work, parallelizable tasks
- Risk: Low | Familiarity: High

**Mode 3: Hierarchical Command**
- Captain assigns objectives to Opus agents, they direct Sonnet workers
- Used for: Large engagements, multi-day efforts
- Risk: Medium | Complexity: High

---

## Persistence Architecture

### Three-Layer Memory

```
┌─────────────────────────────────────────────┐
│ HOT: CLAUDE.md                              │
│ - Auto-loaded every session                 │
│ - Critical context only (architecture,      │
│   key decisions, active threats)            │
│ - Max 500 lines dedicated to recon          │
└─────────────────────────────────────────────┘
                    ↓ references
┌─────────────────────────────────────────────┐
│ WARM: docs/recon/*.md                       │
│ - Detailed findings by category             │
│ - Human-readable, version-controlled        │
│ - Files: architecture.md, vulnerabilities.md│
│   dependencies.md, infrastructure.md        │
└─────────────────────────────────────────────┘
                    ↓ queries
┌─────────────────────────────────────────────┐
│ COLD: data/recon.db (SQLite)                │
│ - Raw scan results                          │
│ - Historical changes                        │
│ - Vulnerability tracking                    │
│ - Query-able for trends                     │
└─────────────────────────────────────────────┘
```

### Memory Operations

| Operation | Layer | Trigger |
|-----------|-------|---------|
| Load context | Hot | Session start (automatic) |
| Deep dive | Warm | Captain requests details |
| Historical query | Cold | Trend analysis, change detection |
| Update findings | All | Snake reports new intel |

---

## Snake Agent Specification

### Identity

- **Naming**: Snake + 3 digits (Snake001, Snake042, etc.)
- **Model**: claude-opus-4-5-20251101 (judgment required)
- **Color**: #2d5016 (military olive)

### Capabilities

1. **Codebase Reconnaissance**
   - Language/framework detection
   - Architecture pattern identification
   - Dependency health audit
   - Code quality assessment

2. **Security Scanning**
   - OWASP Top 10 vulnerability detection
   - Secrets/credential scanning
   - Authentication/authorization audit
   - Input validation assessment

3. **Infrastructure Assessment**
   - Service discovery
   - Network topology mapping
   - Deployment configuration review
   - CI/CD pipeline analysis

4. **Process Evaluation**
   - Test coverage analysis
   - Documentation state
   - Code review practices
   - Deployment procedures

### Report Format

```yaml
snake_report:
  agent_id: "Snake001"
  environment: "customer-acme"
  timestamp: "2025-12-02T10:30:00Z"
  mission: "initial_recon"

  findings:
    critical:
      - id: "VULN-001"
        type: "security"
        description: "SQL injection in login endpoint"
        location: "src/auth/login.go:45"
        recommendation: "Use parameterized queries"

    high:
      - id: "ARCH-001"
        type: "architecture"
        description: "No rate limiting on API endpoints"
        recommendation: "Implement middleware rate limiter"

    medium: []
    low: []

  summary:
    total_files_scanned: 342
    languages: ["go", "typescript", "sql"]
    frameworks: ["chi", "react"]
    test_coverage: "23%"
    security_score: "C"

  recommendations:
    immediate:
      - "Patch SQL injection (VULN-001)"
      - "Add rate limiting (ARCH-001)"
    short_term:
      - "Increase test coverage to 60%"
    long_term:
      - "Migrate to structured logging"
```

---

## Coordination Protocol

### Captain Decision Framework

When Snake reports findings:

```
1. ASSESS severity
   - Any critical? → Direct Control mode, immediate action
   - High only? → Evaluate scope
   - Medium/Low? → Queue for batch processing

2. ESTIMATE effort
   - < 2 hours work? → Single worker agent
   - 2-8 hours? → 2-3 parallel workers
   - > 8 hours? → Hierarchical command

3. SELECT agents
   - Security fixes → SNTRed or OpusRed
   - Architecture → OpusGreen
   - General coding → SNTGreen
   - Code review → SNTPurple or OpusPurple

4. DISPATCH with context
   - Include relevant recon findings
   - Set clear success criteria
   - Define report-back checkpoints

5. MONITOR and ADJUST
   - Track progress via MCP
   - Reassign if blocked
   - Escalate to human if needed
```

### Escalation Triggers

Captain escalates to human when:
- Critical security vulnerability in production
- Architectural decision with >$10K impact
- Customer-facing changes
- Agent repeatedly failing (>3 attempts)
- Conflicting requirements discovered

---

## Bootstrap Kit

### Lightweight Mode (Infrastructure-Poor)

```
bootstrap/
├── state.json          # Minimal state file
├── recon-summary.md    # Portable findings
└── bootstrap.ps1       # Setup script
```

Captain can operate with just these files in any environment.

### Scale-Up Triggers

| Condition | Action |
|-----------|--------|
| >3 agents needed | Spin up CLIAIMONITOR locally |
| Multi-day engagement | Phone home to Magnolia infrastructure |
| Customer requests dashboard | Deploy full CLIAIMONITOR |

### Phone Home Protocol

```
Captain → HTTPS POST → magnolia-hq.example.com/api/v1/reports
         (encrypted findings, status updates)

         ← Task assignments, priority overrides
```

---

## Implementation Plan

### Parallel Execution Strategy

Four independent tracks, two can run in parallel:

```
┌─────────────────────┐    ┌─────────────────────┐
│ TRACK A             │    │ TRACK B             │
│ Memory System       │    │ Snake Agent Type    │
│                     │    │                     │
│ - SQLite schema     │    │ - teams.yaml entry  │
│ - Repo interface    │    │ - System prompt     │
│ - CLAUDE.md updater │    │ - MCP report tools  │
│ - Markdown writer   │    │ - Spawner updates   │
└─────────┬───────────┘    └──────────┬──────────┘
          │                           │
          │ PARALLEL                  │
          └───────────┬───────────────┘
                      ↓
          ┌───────────────────────┐
          │ TRACK C               │
          │ Coordination Protocol │
          │                       │
          │ - Report parser       │
          │ - Decision engine     │
          │ - Agent dispatcher    │
          └───────────┬───────────┘
                      ↓
          ┌───────────────────────┐
          │ TRACK D               │
          │ Bootstrap Kit         │
          │                       │
          │ - Portable state      │
          │ - Setup scripts       │
          │ - Phone home client   │
          └───────────────────────┘
```

### Track A: Memory System

**Files to create/modify:**
- `internal/memory/recon.go` - Recon-specific storage
- `internal/memory/layers.go` - Hot/warm/cold layer management
- `internal/memory/claude_md.go` - CLAUDE.md updater
- `docs/recon/` - Template markdown files

**Tasks:**
1. Design SQLite schema for recon findings
2. Implement ReconRepository interface
3. Create markdown file writer for warm layer
4. Build CLAUDE.md section updater for hot layer
5. Write tests for all layers

### Track B: Snake Agent Type

**Files to create/modify:**
- `configs/teams.yaml` - Add Snake agent definition
- `configs/prompts/snake.md` - Snake system prompt
- `internal/mcp/tools.go` - Add snake report tools
- `internal/agents/spawner.go` - Handle Snake spawning

**Tasks:**
1. Define Snake agent in teams.yaml
2. Write Snake system prompt (recon focus)
3. Implement `submit_recon_report` MCP tool
4. Implement `request_guidance` MCP tool
5. Update spawner for Snake naming convention
6. Write tests

### Track C: Coordination Protocol

**Dependencies:** Track A (memory) + Track B (snake tools)

**Files to create/modify:**
- `internal/supervisor/decision.go` - Decision engine
- `internal/supervisor/dispatcher.go` - Agent dispatcher
- `internal/handlers/reports.go` - Report HTTP handlers

**Tasks:**
1. Implement report parser for Snake YAML format
2. Build decision engine (assess → estimate → select → dispatch)
3. Create agent dispatcher with mode selection
4. Add HTTP endpoints for report submission
5. Integration tests

### Track D: Bootstrap Kit

**Dependencies:** Track C (coordination)

**Files to create/modify:**
- `bootstrap/state.json` - Minimal state schema
- `bootstrap/bootstrap.ps1` - Windows setup script
- `bootstrap/bootstrap.sh` - Linux setup script
- `internal/bootstrap/phonehome.go` - Phone home client

**Tasks:**
1. Design minimal portable state format
2. Create bootstrap scripts for Windows/Linux
3. Implement phone home HTTP client
4. Add scale-up detection logic
5. Test in isolated environment

---

## Success Criteria

### Phase 1: Foundation (Tracks A+B parallel)
- [ ] Snake agent can be spawned via CLIAIMONITOR
- [ ] Snake can scan a codebase and produce structured report
- [ ] Findings persist across sessions in 3-layer memory
- [ ] CLAUDE.md automatically updated with critical findings

### Phase 2: Coordination (Track C)
- [ ] Captain receives and parses Snake reports
- [ ] Decision engine recommends appropriate response
- [ ] Captain can spawn workers based on findings
- [ ] Full reconnaissance → decision → execution flow works

### Phase 3: Portability (Track D)
- [ ] Captain can bootstrap in infrastructure-poor environment
- [ ] Phone home successfully sends reports to Magnolia HQ
- [ ] Scale-up correctly triggers CLIAIMONITOR deployment

### Phase 4: Battle Testing
- [ ] Deploy to Magnolia codebase, find real issues
- [ ] Successful customer demo (simulated)
- [ ] Parallel agent coordination under load

---

## Open Questions

1. **General agent**: When do we build the layer above Captain?
2. **Agent authentication**: How do Snakes prove identity when phoning home?
3. **Conflict resolution**: What if two Snakes report contradictory findings?
4. **Cost controls**: How do we limit token spend on large reconnaissance?

---

## Appendix: File Inventory

### New Files
```
configs/prompts/snake.md
internal/memory/recon.go
internal/memory/layers.go
internal/memory/claude_md.go
internal/supervisor/decision.go
internal/supervisor/dispatcher.go
internal/handlers/reports.go
internal/bootstrap/phonehome.go
bootstrap/state.json
bootstrap/bootstrap.ps1
bootstrap/bootstrap.sh
docs/recon/architecture.md
docs/recon/vulnerabilities.md
docs/recon/dependencies.md
docs/recon/infrastructure.md
```

### Modified Files
```
configs/teams.yaml
internal/mcp/tools.go
internal/mcp/server.go
internal/agents/spawner.go
CLAUDE.md (auto-updated by system)
```
