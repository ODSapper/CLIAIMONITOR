# Track C Implementation Summary

**Date**: 2025-12-02
**Status**: Complete
**Implementation**: Snake Agent Force - Coordination Protocol

## Overview

Implemented Track C of the Snake Agent Force design: the Captain's decision engine and agent dispatcher. This provides the coordination protocol that analyzes Snake reconnaissance reports and dispatches worker agents to address findings.

## Components Implemented

### 1. Decision Engine (`internal/supervisor/decision.go`)

**Purpose**: Analyzes reconnaissance reports and produces actionable plans

**Key Features**:
- **AnalyzeReport**: Parses Snake reports and generates comprehensive action plans
- **SelectMode**: Determines operational mode based on findings severity and complexity
  - Direct Control: High risk situations (>3 security findings)
  - Task Dispatch: Routine work with low risk
  - Hierarchical: Large scope (>20 findings)
- **RecommendAgents**: Selects appropriate agent types for each action
  - Security issues → SNTRed or OpusRed
  - Architecture work → OpusGreen
  - General coding → SNTGreen
- **RequiresEscalation**: Detects situations requiring human approval
  - Production security vulnerabilities
  - Architectural decisions with high impact
  - Customer-facing changes
  - Data loss risks

**Types**:
- `ActionPlan`: Complete plan with immediate, short-term, and long-term actions
- `AgentRecommendation`: Specific agent spawning recommendation
- `PlannedAction`: Individual remediation action with effort estimates
- `OperationalMode`: Direct Control | Task Dispatch | Hierarchical

### 2. Report Parser (`internal/supervisor/parser.go`)

**Purpose**: Parses Snake reconnaissance reports from various formats

**Key Features**:
- `ParseYAML`: Parse YAML reconnaissance reports
- `ParseJSON`: Parse JSON reconnaissance reports
- `ParseMCPReport`: Parse from MCP tool call parameters (for direct Snake→Captain communication)
- `ValidateReport`: Ensures reports have required fields

**Report Structure**:
```yaml
snake_report:
  agent_id: "Snake001"
  environment: "customer-acme"
  timestamp: "2025-12-02T10:30:00Z"
  mission: "initial_recon"
  findings:
    critical: [...]
    high: [...]
    medium: [...]
    low: [...]
  summary:
    total_files_scanned: 342
    languages: ["go", "typescript"]
    frameworks: ["chi", "react"]
    test_coverage: "23%"
    security_score: "C"
  recommendations:
    immediate: [...]
    short_term: [...]
    long_term: [...]
```

### 3. Agent Dispatcher (`internal/supervisor/dispatcher.go`)

**Purpose**: Executes action plans by spawning and coordinating agents

**Key Features**:
- `ExecutePlan`: Spawns agents according to action plan
- `SpawnAgent`: Spawns individual agent with task context
- `GetDispatchStatus`: Tracks progress of dispatched agents
- `AbortDispatch`: Cancel running dispatch and stop all agents
- `ListDispatches`: Query dispatch history

**Types**:
- `DispatchResult`: Result of executing a plan
- `SpawnedAgent`: Metadata about spawned agent
- `DispatchStatus`: Current status of all agents in a dispatch
- `DispatchFilter`: Query filter for dispatch history

**Features**:
- Concurrent agent spawning with delay between spawns
- Context-based cancellation support
- Agent status tracking (spawning, running, completed, failed)
- Integration with existing spawner infrastructure

### 4. HTTP Handlers (`internal/handlers/coordination.go`)

**Purpose**: REST API endpoints for Captain coordination

**Endpoints**:
- `POST /api/coordination/analyze` - Submit report, get action plan
- `POST /api/coordination/dispatch` - Execute action plan
- `GET /api/coordination/status/:id` - Get dispatch status
- `POST /api/coordination/abort/:id` - Abort dispatch
- `GET /api/coordination/history` - List past dispatches
- `GET /api/coordination/plans` - List stored plans
- `GET /api/coordination/plans/:id` - Get specific plan

**Features**:
- Multi-format support (YAML, JSON, auto-detect)
- Report validation before processing
- Integration with reconnaissance repository
- Fallback to agent learning storage when recon repo unavailable
- Human approval enforcement (blocks dispatch if RequiresHuman=true)

### 5. Tests

**Test Coverage**:
- `decision_test.go`: Decision engine logic
  - Mode selection based on findings
  - Escalation requirement detection
  - Report analysis and plan generation
  - Agent type recommendation
  - Effort estimation
- `parser_test.go`: Report parsing
  - YAML format parsing
  - JSON format parsing
  - MCP parameter parsing
  - Report validation

**All tests passing**: 14/14 tests pass

## Integration Points

### Server Integration

Modified `internal/server/server.go`:
- Added coordination handler registration in `setupRoutes()`
- Added `getAgentConfigsMap()` helper for dispatcher
- Coordination endpoints available at `/api/coordination/*`

### Memory System Integration

- Stores reconnaissance reports in recon repository (Track A)
- Falls back to agent learning if recon repo unavailable
- Action plans stored as agent learning with category "action_plan"
- Environment registration for tracking scanned environments

### Agent Spawning Integration

- Uses existing `ProcessSpawner` from `internal/agents/spawner.go`
- Generates agent IDs following naming conventions (Snake001, SNTGreen-123, etc.)
- Builds initial prompts with task context and rationale
- Respects agent configs from `configs/teams.yaml`

### MCP Tools Integration

- Ready for Snake agents using `submit_recon_report` tool (Track B)
- Report flows: Snake → MCP → Coordination Handler → Decision Engine → Dispatcher
- Agent registration happens via existing MCP callbacks

## Decision Framework

The Captain follows a 5-step decision process:

### 1. ASSESS Severity
- Any critical findings → Priority: critical
- Multiple high findings → Priority: high
- Mixed findings → Priority: medium
- Only low findings → Priority: low

### 2. ESTIMATE Effort
- Critical findings: 2 hours each
- High findings: 1 hour each
- Medium findings: 0.5 hours each
- Low findings: 0.25 hours each
- 20% coordination overhead applied

### 3. SELECT Mode
- >3 security findings → Direct Control
- >20 total findings → Hierarchical
- Otherwise → Task Dispatch

### 4. SELECT Agents
- Security + Opus needed → OpusRed
- Security only → SNTRed
- Architecture/complex → OpusGreen
- General coding → SNTGreen
- Code review → SNTPurple or OpusPurple

### 5. CHECK Escalation
Escalate to human if:
- Production security vulnerability
- Architectural change with >$10K impact
- Customer-facing changes
- Risk of data loss or irreversible changes

## Usage Example

### Captain receives Snake report:

```bash
# Snake submits reconnaissance via MCP
POST /api/coordination/analyze
Content-Type: application/yaml

<YAML report data>
```

### Captain analyzes and returns plan:

```json
{
  "report_id": "recon-1733140800",
  "plan": {
    "id": "plan-1733140800",
    "mode": "direct",
    "priority": "critical",
    "estimated_agents": 2,
    "estimated_hours": 4.8,
    "requires_human": false,
    "agent_recommendations": [
      {
        "agent_type": "SNTRed",
        "task": "Patch SQL injection (VULN-001)",
        "priority": 1,
        "finding_ids": ["VULN-001"],
        "rationale": "Security-related task requiring SNTRed expertise"
      },
      {
        "agent_type": "OpusGreen",
        "task": "Add rate limiting (ARCH-001)",
        "priority": 2,
        "finding_ids": ["ARCH-001"],
        "rationale": "Complex architectural work best handled by OpusGreen"
      }
    ]
  }
}
```

### Captain dispatches agents:

```bash
POST /api/coordination/dispatch
{
  "plan_id": "plan-1733140800"
}
```

### Returns dispatch result:

```json
{
  "dispatch_id": "dispatch-1733140800",
  "result": {
    "status": "running",
    "agents_spawned": [
      {
        "agent_id": "SNTRed-142",
        "agent_type": "SNTRed",
        "task": "Patch SQL injection (VULN-001)",
        "status": "running",
        "spawned_at": "2025-12-02T10:30:02Z"
      },
      {
        "agent_id": "OpusGreen-143",
        "agent_type": "OpusGreen",
        "task": "Add rate limiting (ARCH-001)",
        "status": "running",
        "spawned_at": "2025-12-02T10:30:04Z"
      }
    ]
  }
}
```

## Files Created

### Core Implementation
- `internal/supervisor/decision.go` (445 lines)
- `internal/supervisor/dispatcher.go` (303 lines)
- `internal/supervisor/parser.go` (288 lines)
- `internal/handlers/coordination.go` (471 lines)

### Tests
- `internal/supervisor/decision_test.go` (234 lines)
- `internal/supervisor/parser_test.go` (237 lines)

### Modified Files
- `internal/server/server.go` - Added coordination handler registration

**Total**: ~1,978 lines of code + tests

## Next Steps (Track D)

With Track C complete, the system is ready for:

1. **Bootstrap Kit** (Track D):
   - Lightweight deployment mode for infrastructure-poor environments
   - Phone home protocol for remote Captain deployment
   - Scale-up triggers and detection logic

2. **End-to-End Testing**:
   - Deploy Snake agent to real environment
   - Submit actual reconnaissance report
   - Verify Captain decision-making
   - Observe worker agent spawning and execution

3. **Production Refinements**:
   - Tune effort estimation algorithms
   - Refine agent selection heuristics
   - Add dispatch metrics and observability
   - Implement dispatch result aggregation

## Design Compliance

This implementation follows the Snake Agent Force design document:
- ✅ Decision framework (5 steps: ASSESS → ESTIMATE → SELECT → DISPATCH → MONITOR)
- ✅ Operational modes (Direct Control, Task Dispatch, Hierarchical)
- ✅ Escalation triggers (production security, architectural decisions, customer-facing)
- ✅ Agent type selection (security → Red, architecture → OpusGreen, general → SNTGreen)
- ✅ Report format (YAML/JSON with findings, summary, recommendations)
- ✅ Integration with memory system (3-layer architecture)
- ✅ MCP tool integration ready

## Conclusion

Track C is complete and functional. The coordination protocol is ready to:
- Receive reconnaissance reports from Snake agents
- Analyze findings and assess risk
- Generate comprehensive action plans
- Spawn appropriate worker agents
- Track dispatch progress
- Enforce human approval when needed

The system builds on Track A (Memory) and Track B (Snake Agent Type) to provide intelligent, autonomous coordination between reconnaissance and execution.
