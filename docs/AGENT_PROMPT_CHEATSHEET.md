# Agent Prompt Cheatsheet

Quick reference for simplified agent prompts in CLIAIMONITOR.

---

## What Changed

| Old Way | New Way |
|---------|---------|
| SSE connections (`/mcp/sse`) | HTTP Streamable (`/mcp`) |
| Heartbeat polling | No heartbeats needed |
| Complex connection state | Stateless MCP |
| Custom HTTP endpoints | MCP tools only |
| Implicit exits | Explicit approval flow |

---

## The 4-Step Workflow

Every agent follows this pattern:

```
1. register_agent()         ← Tell Captain you're online
2. [Do your work]           ← Execute task
3. signal_captain()         ← Signal completion/blocking/error
4. request_stop_approval()  ← Get approval to exit (MANDATORY)
```

**That's it.** No heartbeats, no connection checks, no complex logic.

---

## Minimal Viable Agent Prompt

```markdown
# You are [ROLE]

You will: [TASK]

## Workflow

1. Call: register_agent(agent_id="[AGENT_ID]", role="[ROLE]")
2. Do your work
3. Call: signal_captain(signal="completed", context="Done", work_completed="Summary")
4. Call: request_stop_approval(reason="task_complete", context="...", work_completed="...")
5. Wait for approval, then exit

## Tools Available
- register_agent - Register at startup (REQUIRED)
- signal_captain - Signal completion/error (REQUIRED)
- request_stop_approval - Request exit approval (REQUIRED)
- log_activity - Log progress
- request_human_input - Ask for help
- [See MCP documentation for full list]

## Important
- Always register first
- Always signal when done
- Always request approval before stopping
- MCP is stateless (no heartbeats needed)
```

---

## MCP Tools Quick Reference

### Essential Tools (Use Every Time)

| Tool | Purpose | Returns |
|------|---------|---------|
| `register_agent(agent_id, role)` | Register with Captain | `{status: "registered"}` |
| `signal_captain(signal, context, work_completed)` | Signal status (completed/blocked/error) | `{status: "signaled"}` |
| `request_stop_approval(reason, context, work_completed)` | Request permission to exit | `{status: "pending", request_id: "..."}` |

**Signal types**: `"completed"` \| `"blocked"` \| `"error"` \| `"stopped"` \| `"need_guidance"`

### Common Tools (Use as Needed)

| Tool | Purpose |
|------|---------|
| `log_activity(action, details)` | Log work to dashboard |
| `report_progress(status, progress_pct, note)` | Send progress update |
| `request_human_input(question, context)` | Ask human for answer |
| `request_guidance(context)` | Ask Captain for guidance |
| `complete_task(task_id, summary)` | Mark task complete |

### Specialized Tools (Role-Specific)

| Tool | For | Purpose |
|------|-----|---------|
| `get_my_tasks(status)` | All | Get assigned tasks |
| `store_knowledge(knowledge)` | All | Save learnings |
| `search_knowledge(query, category)` | All | Query knowledge base |
| `submit_recon_report(report)` | Snake | Submit findings |
| `record_episode(episode)` | All | Log experience |

---

## Template by Role

### Implementation (Green)

```
1. register_agent("team-sntgreen001", "CodeImplementer")
2. Read requirements → implement → test → commit → push
3. signal_captain("completed", "Implementation done", "Implemented X, tests pass")
4. request_stop_approval("task_complete", "Work finished", "Summary")
5. Wait for approval & exit
```

### Review (Purple)

```
1. register_agent("team-sntpurple001", "CodeReviewer")
2. Review code → document issues → provide recommendations
3. signal_captain("completed", "Review done", "Found N issues, X critical")
4. request_stop_approval("task_complete", "Review finished", "Issues documented")
5. Wait for approval & exit
```

### Recon (Snake)

```
1. register_agent("team-snake001", "Reconnaissance")
2. Scan codebase → analyze → document findings
3. signal_captain("completed", "Scan done", "Found N issues, documented")
4. request_stop_approval("task_complete", "Analysis complete", "Findings in YAML")
5. Wait for approval & exit
```

### Security (Opus)

```
1. register_agent("team-opusred001", "SecurityAuditor")
2. Scan dependencies → review code → identify vulnerabilities
3. signal_captain("completed", "Audit done", "Found N critical, Y high")
4. request_stop_approval("task_complete", "Audit finished", "Risks documented")
5. Wait for approval & exit
```

### Testing

```
1. register_agent("team-sntgreen002", "TestExecutor")
2. Run tests → capture results → analyze coverage
3. signal_captain("completed", "Tests done", "X passed, Y failed, Z% coverage")
4. request_stop_approval("task_complete", "Tests finished", "Results documented")
5. Wait for approval & exit
```

---

## Prompt Structure Template

```markdown
# You are [NAME] ([ROLE])

**MCP Endpoint**: http://localhost:3000/mcp
**Agent ID**: [AGENT_ID]
**Role**: [ROLE_DESCRIPTION]
**Project**: [PROJECT_PATH]

## Your Task
[TASK_DESCRIPTION]

## Workflow

### Step 1: Register
register_agent(agent_id="[AGENT_ID]", role="[ROLE]")

### Step 2: Execute
[Role-specific instructions]

### Step 3: Signal
signal_captain(
  signal="completed",
  context="Task finished",
  work_completed="Summary of work done"
)

### Step 4: Request Approval
request_stop_approval(
  reason="task_complete",
  context="Finished work",
  work_completed="Summary of accomplishments"
)
Wait for supervisor response before exiting.

## Key Tools
- register_agent - Register at startup
- signal_captain - Signal completion
- request_stop_approval - Request exit
- log_activity - Log progress
- [others as needed]

## Rules
1. Always register first
2. Always signal completion
3. Always request approval before stopping
4. No heartbeats needed (MCP is stateless)
5. Use MCP tools only (no custom HTTP)
```

---

## What NOT To Do

| ❌ Don't | ✅ Do Instead |
|----------|-------------|
| Send heartbeats/pings | Let MCP handle connection automatically |
| Use SSE for connections | Use MCP `/mcp` endpoint (HTTP) |
| Check connection status | Trust MCP is connected |
| Exit without approval | Always call `request_stop_approval` |
| Use custom HTTP endpoints | Use MCP tools only |
| Hardcode timeouts for polling | Rely on MCP's event system |
| Assume persistent connections | Use stateless HTTP per-call |
| Skip registration | Always register first |

---

## Common Mistakes & Fixes

### Mistake 1: Exiting Without Approval
```
❌ Wrong:
  [finish work]
  exit()

✅ Right:
  [finish work]
  signal_captain(signal="completed", ...)
  request_stop_approval(reason="task_complete", ...)
  wait_for_supervisor_approval()
  exit()
```

### Mistake 2: Checking Connection Status
```
❌ Wrong:
  while not connected:
    try_connect()
    sleep(1)

✅ Right:
  register_agent()  # MCP handles rest automatically
  [start working]
```

### Mistake 3: Custom Exit Signals
```
❌ Wrong:
  POST /custom/agent/complete

✅ Right:
  signal_captain(signal="completed", ...)
```

### Mistake 4: Polling for Responses
```
❌ Wrong:
  stop_request = request_stop_approval(...)
  while not approved:
    check_status()
    sleep(1)

✅ Right:
  stop_request = request_stop_approval(...)
  wait_for_events(event_types=["stop_approval"])
  # Receives approval event when ready
```

### Mistake 5: Managing Connection State
```
❌ Wrong:
  sse_connection = connect_to_sse()
  sse_connection.keep_alive()
  sse_connection.handle_pings()

✅ Right:
  # MCP handles everything
  # Just use tools via POST requests
```

---

## Testing Your Prompt

Before deploying, verify:

1. **Registration works**
   - Agent calls `register_agent` successfully
   - Captain receives registration event

2. **Tools are accessible**
   - `signal_captain` can be called
   - `request_stop_approval` can be called
   - Tool responses are received

3. **Workflow completes**
   - Agent registers → works → signals → requests approval
   - Supervisor can approve/reject
   - Agent receives response

4. **No heartbeat polling**
   - Agent doesn't need to check connection status
   - No ping/pong messages
   - Stateless HTTP calls work fine

---

## Deployment Checklist

- [ ] Prompt follows simplified structure (register → work → signal → approve)
- [ ] No SSE or heartbeat references
- [ ] All MCP tools called via standard interface
- [ ] `request_stop_approval` called before any exit
- [ ] Structured output (YAML/JSON) for results
- [ ] Role-specific instructions clear
- [ ] Error handling documented
- [ ] Success criteria defined
- [ ] Tested with Captain before production

---

## Quick Links

- **Full Template**: See `AGENT_PROMPT_TEMPLATE.md`
- **Role Variants**: See `AGENT_PROMPT_VARIANTS.md`
- **MCP Architecture**: See `docs/plans/2025-12-21-pure-mcp-architecture.md`
- **Implementation Guide**: See `docs/plans/2025-12-21-streamable-http-agent-design.md`

---

## TL;DR

**Old way**: SSE + heartbeats + connection state = complex
**New way**: Register → Work → Signal → Approve = simple

Use the 4-step workflow. Use MCP tools. No heartbeats. Simple.
