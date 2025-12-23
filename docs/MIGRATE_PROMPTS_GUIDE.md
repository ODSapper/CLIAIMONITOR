# Migrating Existing Prompts to Simplified Structure

This guide helps you convert existing agent prompts to the new simplified MCP-based structure.

---

## Why Migrate?

The new simplified prompt structure:
- **Removes complexity**: No SSE, heartbeats, or connection management
- **Improves clarity**: 4-step workflow instead of complex state machines
- **Reduces errors**: Structured process with explicit approval gates
- **Enables control**: Supervisor has final say before agent stops
- **Supports learning**: Built-in knowledge capture

---

## Migration Checklist

For each existing prompt, complete these steps:

### Phase 1: Audit Current Prompt

```
□ Identify the agent role (Implementation, Review, Security, Testing, Recon)
□ Find all SSE/heartbeat/connection references
□ List all HTTP endpoints being called
□ Identify success criteria
□ Document the workflow
```

### Phase 2: Find & Replace

| Search | Replace With |
|--------|--------------|
| `connect_to_sse()` | Delete (MCP handles automatically) |
| `keep_alive()` or `ping()` | Delete (no heartbeats needed) |
| `POST /mcp/sse` | Use MCP tools instead |
| `/api/agents/status` | Use `log_activity()` and `signal_captain()` |
| `/api/agents/report` | Use `signal_captain()` |
| Custom status polling | Use `signal_captain()` and `request_stop_approval()` |
| `while not completed: check_status()` | Use `signal_captain()` then exit |

### Phase 3: Implement New Workflow

1. Add registration step
2. Update work instructions (keep substantial work same)
3. Add `signal_captain()` call
4. Add `request_stop_approval()` call
5. Remove all connection management code

### Phase 4: Test

- Verify agent registers
- Verify signal_captain works
- Verify request_stop_approval works
- Verify approval flow completes
- Verify agent exits cleanly

### Phase 5: Document

- Update prompt file
- Update any agent configuration
- Note any custom tools added

---

## Example: Converting Code Implementation Prompt

### BEFORE (Old Structure)

```markdown
# You are SGT-Green (Code Implementer)

## Setup
First, establish SSE connection to MCP server:
1. GET /mcp/sse with X-Agent-ID header to open stream
2. POST /mcp/sse to send messages (use session_id from stream)
3. Keep sending heartbeats every 15 seconds

## Workflow
1. Connect to MCP SSE
2. Register agent via RPC
3. Wait for task assignment
4. Poll /api/agents/status every 5 seconds
5. Implement code
6. Commit changes
7. POST /api/agents/report with results
8. Wait for response
9. Exit when complete

## Task Details
[IMPLEMENTATION DETAILS]

## Exit Requirements
- All tests passing
- Changes committed
- Status reported to dashboard
```

### AFTER (New Structure)

```markdown
# You are SGT-Green (Code Implementation Specialist)

**MCP Endpoint**: http://localhost:3000/mcp
**Agent ID**: [AGENT_ID]
**Project Path**: [PROJECT_PATH]

## Your Task
[IMPLEMENTATION DETAILS]

## Workflow

### 1. Register (Immediate)
register_agent(agent_id="[AGENT_ID]", role="CodeImplementer")

### 2. Implement
- Read requirements
- Examine existing code
- Write implementation with tests
- Ensure all tests pass
- Commit and push changes

### 3. Signal Completion
signal_captain(
  signal="completed",
  context="Implementation complete with all tests passing",
  work_completed="Implemented X, added Y tests, coverage Z%"
)

### 4. Request Approval to Stop
request_stop_approval(
  reason="task_complete",
  context="Implementation and testing finished, changes pushed",
  work_completed="[Summary of work]"
)

Wait for supervisor approval before exiting.

## Success Criteria
- [x] Code compiles
- [x] Tests pass
- [x] Coverage >= 80%
- [x] Changes committed
- [x] signal_captain called
- [x] request_stop_approval called
```

**Key Changes**:
- ✅ Removed SSE connection setup
- ✅ Removed heartbeat polling
- ✅ Removed status polling loop
- ✅ Replaced with simple MCP tool calls
- ✅ Added explicit approval request
- ✅ Simplified success criteria

---

## Common Migration Patterns

### Pattern 1: From Polling Loop

**Before**:
```
while agent.status != "complete":
  check_status_api()
  if has_changes:
    commit_and_report()
  sleep(5)
```

**After**:
```
# Do your work
signal_captain(signal="completed", context="...", work_completed="...")
request_stop_approval(reason="task_complete", context="...", work_completed="...")
exit()
```

### Pattern 2: From Custom HTTP Endpoints

**Before**:
```
POST /api/agents/custom-report
  {"status": "completed", "results": {...}}
POST /api/agents/wait-for-approval
  {"request_id": "123"}
```

**After**:
```
signal_captain(signal="completed", ...)
request_stop_approval(reason="task_complete", ...)
```

### Pattern 3: From Heartbeat Management

**Before**:
```
start_heartbeat_thread()
while working:
  send_heartbeat()
  do_work()
stop_heartbeat_thread()
```

**After**:
```
# No heartbeats needed at all
do_work()
```

### Pattern 4: From Connection State Management

**Before**:
```
if not connected:
  reconnect_sse()
if not session_valid:
  re_register()
check_connection_health()
```

**After**:
```
register_agent()  # One call, MCP handles rest
```

---

## Migration Examples by Role

### Security Auditor

**Before**: 28 lines of SSE setup + polling + custom report endpoints
**After**: 4-step workflow using MCP tools

```markdown
# You are SecurityAuditor

1. register_agent(agent_id="team-opusred001", role="SecurityAuditor")
2. Scan codebase for vulnerabilities
3. signal_captain(signal="completed", context="Audit complete", work_completed="Found N critical issues")
4. request_stop_approval(reason="task_complete", context="Security audit finished", work_completed="Report attached")

# Everything else stays the same (audit methodology, scanning, documentation)
```

### Code Reviewer

**Before**: Connection setup + polling status + manual exit handling
**After**: Register → Review → Signal → Approve

```markdown
# You are SGT-Purple

1. register_agent(agent_id="team-sntpurple001", role="CodeReviewer")
2. Review code for issues
3. signal_captain(signal="completed", context="Review done", work_completed="Found X issues")
4. request_stop_approval(reason="task_complete", context="Code review finished", work_completed="Issues documented")

# Everything else stays the same (review process, issue documentation)
```

### Reconnaissance

**Before**: SSE stream + custom report format + status checks
**After**: MCP tools + structured output

```markdown
# You are Snake

1. register_agent(agent_id="team-snake001", role="Reconnaissance")
2. Analyze codebase, document findings
3. signal_captain(signal="completed", context="Scan done", work_completed="Analysis complete")
4. request_stop_approval(reason="task_complete", context="Codebase analysis finished", work_completed="Findings in YAML")

# Everything else stays the same (analysis methodology, output format)
```

---

## Detailed Conversion Steps

### Step 1: Backup Original
```bash
cp configs/prompts/old-prompt.md configs/prompts/old-prompt.md.bak
```

### Step 2: Create New Structure
Use template from `AGENT_PROMPT_TEMPLATE.md` or appropriate variant from `AGENT_PROMPT_VARIANTS.md`

### Step 3: Copy Core Content
- Copy the task description
- Copy role-specific instructions (analysis, implementation, review steps)
- Copy success criteria
- Copy any special considerations

### Step 4: Remove Old Infrastructure
Delete sections about:
- SSE connections
- Heartbeat management
- Status polling loops
- Custom HTTP endpoints
- Session management
- Connection state handling

### Step 5: Add New Workflow
Insert the 4-step workflow:
```
1. register_agent()
2. [Existing work instructions]
3. signal_captain()
4. request_stop_approval()
```

### Step 6: Test
1. Have an agent run the new prompt
2. Verify registration succeeds
3. Verify signal_captain works
4. Verify request_stop_approval works
5. Verify approval flow completes
6. Verify agent exits cleanly

### Step 7: Update Configs
- Update `configs/teams.yaml` if needed
- Update any spawning scripts
- Update documentation

---

## Validation Checklist

Before considering a prompt migrated, verify:

### Structure
- [ ] Prompt has clear title/role
- [ ] MCP endpoint specified
- [ ] 4-step workflow present
- [ ] No SSE references
- [ ] No heartbeat code
- [ ] No polling loops

### Content
- [ ] Task description clear
- [ ] Success criteria defined
- [ ] Role-specific instructions present
- [ ] Tools documented
- [ ] Error handling described
- [ ] Important rules stated

### Functionality
- [ ] Agent registers successfully
- [ ] Task execution works
- [ ] signal_captain callable
- [ ] request_stop_approval callable
- [ ] Supervisor can approve/reject
- [ ] Agent exits cleanly

### Quality
- [ ] Prompt is concise (not verbose)
- [ ] Instructions are clear
- [ ] No unnecessary details
- [ ] Focuses on task execution
- [ ] Easy to understand at first read

---

## Troubleshooting Migration Issues

### Issue: Agent Hangs After Registration
**Cause**: Waiting for SSE stream after registering
**Fix**: Remove any `wait_for_connection()` or stream loops - MCP is stateless

### Issue: Agent Says "Connection Lost"
**Cause**: Checking connection status between tool calls
**Fix**: MCP doesn't track persistent connections. Each tool call is independent.

### Issue: Signal Not Received
**Cause**: Calling custom HTTP endpoint instead of `signal_captain` tool
**Fix**: Use `signal_captain()` MCP tool, not custom HTTP POST

### Issue: Agent Exits Before Approval
**Cause**: Not calling `request_stop_approval` before exit
**Fix**: ALWAYS call `request_stop_approval` - this is mandatory

### Issue: Supervisor Doesn't See Agent Status
**Cause**: Using old status reporting endpoints
**Fix**: Use `signal_captain()` and `log_activity()` tools

### Issue: Agent Keeps Polling for Task
**Cause**: Old prompt had polling loop for task assignment
**Fix**: Tasks are assigned via Captain, not fetched by agent

---

## Timeline & Rollout

### Phase 1: Convert High-Priority Prompts (Week 1)
- Code Implementation (SGT-Green)
- Code Review (SGT-Purple)
- Security Audit (OpusRed)

### Phase 2: Convert Supporting Roles (Week 2)
- Reconnaissance (Snake)
- Testing
- Planning

### Phase 3: Validate & Refine (Week 3)
- Run agents with new prompts
- Capture feedback
- Fix any issues
- Update documentation

### Phase 4: Deprecate Old Prompts (Week 4)
- Archive old prompt files
- Update all references
- Train team on new structure

---

## Quick Reference: Before & After

| Aspect | Before | After |
|--------|--------|-------|
| Connection | SSE stream + polling | HTTP POST to `/mcp` |
| Heartbeats | Every 15 seconds | None (stateless) |
| Status Checks | Poll API every 5 sec | Call `signal_captain()` |
| Work Reporting | Custom HTTP POST | `signal_captain()` tool |
| Exit Process | Implicit (check status) | `request_stop_approval()` |
| Complexity | Medium (state machine) | Low (4 steps) |
| Lines Added | ~30-50 (setup/polling) | ~5-10 (workflow) |
| Lines Removed | ~30-50 (connection mgmt) | Minimal |

---

## Support & Questions

When migrating your prompts:

1. **Reference template**: Use `AGENT_PROMPT_TEMPLATE.md`
2. **Check variants**: See `AGENT_PROMPT_VARIANTS.md` for your role
3. **Review cheatsheet**: `AGENT_PROMPT_CHEATSHEET.md` for quick answers
4. **Test thoroughly**: Verify all 4 workflow steps work
5. **Document changes**: Note what was modified and why

The new structure is simpler and more reliable. Invest in migration now, save maintenance complexity later.

---

## Examples Available

Pre-migrated prompts ready to use:

- `AGENT_PROMPT_VARIANTS.md` - Role-specific templates
- `configs/prompts/` - Individual prompt files (being updated)

Copy and customize as needed. All follow the same simplified structure.
