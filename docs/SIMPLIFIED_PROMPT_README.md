# Simplified Agent Prompt Structure - Documentation Index

Welcome! This directory contains comprehensive documentation for the simplified agent prompt structure in CLIAIMONITOR.

## Quick Start (5 minutes)

**New to this?** Start here:
1. Read `SIMPLIFIED_PROMPT_SUMMARY.md` (this overview)
2. Skim `AGENT_PROMPT_CHEATSHEET.md` (quick reference)
3. Copy template from `AGENT_PROMPT_VARIANTS.md` for your role

## Documents

### 1. SIMPLIFIED_PROMPT_SUMMARY.md ⭐ START HERE
**Purpose**: Executive overview of the new simplified structure
**Audience**: Project leads, decision makers, quick starters
**Read Time**: 5 minutes
**Contains**:
- Architecture principles
- 4-step workflow overview
- Key improvements (what changed, what stayed)
- Success criteria
- File structure
- Common questions & answers

### 2. AGENT_PROMPT_CHEATSHEET.md 📋 QUICK REFERENCE
**Purpose**: One-page reference during development
**Audience**: Developers implementing agents
**Read Time**: 5 minutes
**Contains**:
- What changed vs old architecture
- 4-step workflow at a glance
- MCP tools quick lookup table
- Template by role (Implementation, Review, Recon, etc.)
- Common mistakes & how to fix them
- Deployment checklist

### 3. AGENT_PROMPT_VARIANTS.md 📝 COPY-PASTE READY
**Purpose**: Production-ready prompt templates for different roles
**Audience**: Developers creating agents
**Read Time**: 10-15 minutes to scan all, 2-3 minutes to copy one
**Contains**:
- Code Implementation (SGT-Green)
- Code Review (SGT-Purple)
- Security Auditor
- Reconnaissance (Snake)
- Test Execution

Each variant includes:
- Complete workflow explanation
- Role-specific instructions
- Success criteria checklist
- Key tools reference
- Example outputs

**How to Use**:
1. Find your role section
2. Copy the entire markdown block
3. Customize with your project path and task details
4. Deploy as agent prompt

### 4. AGENT_PROMPT_TEMPLATE.md 📖 COMPREHENSIVE REFERENCE
**Purpose**: Complete reference guide for all aspects of simplified prompts
**Audience**: Architects, advanced developers, prompt customizers
**Read Time**: 20 minutes
**Contains**:
- Full architecture explanation
- Detailed prompt structure breakdown
- Step-by-step workflow with rationale
- Complete MCP tools documentation
- Task-specific instruction templates for each role
- Error handling strategies
- Important rules and best practices
- Advanced customization guide
- Subagent vs Terminal mode differences

**When to Use**:
- Understanding the architecture deeply
- Creating custom roles not in VARIANTS
- Troubleshooting prompt issues
- Learning MCP best practices

### 5. MIGRATE_PROMPTS_GUIDE.md 🔄 CONVERSION HELP
**Purpose**: Step-by-step guide for converting existing prompts
**Audience**: DevOps, maintainers, teams with legacy prompts
**Read Time**: 15 minutes
**Contains**:
- Why migrate (benefits overview)
- Migration checklist
- Find & replace patterns
- Example conversions for each role
- Detailed step-by-step conversion process
- Validation checklist
- Troubleshooting common issues
- Timeline and rollout plan
- Before/after comparisons

**When to Use**:
- Converting existing agent prompts
- Updating legacy configurations
- Understanding what changed
- Validating converted prompts

## The 4-Step Workflow (Core Pattern)

Every agent follows this pattern:

```
Step 1: Register
  └─ Call: register_agent(agent_id="[ID]", role="[ROLE]")
     Purpose: Tell Captain you're online

Step 2: Execute Task
  └─ [Your actual work: analyze, implement, review, test, etc.]
     Purpose: Perform assigned task

Step 3: Signal Completion
  └─ Call: signal_captain(signal="completed", context="...", work_completed="...")
     Purpose: Notify Captain of task status

Step 4: Request Stop Approval
  └─ Call: request_stop_approval(reason="task_complete", context="...", work_completed="...")
     Then: Wait for supervisor response
     Purpose: Get permission to exit (supervisor has final say)
```

That's it. Everything else is customization within step 2.

## Key Principles

1. **Stateless HTTP** - Agent connects via `/mcp` endpoint, not SSE
2. **No Heartbeats** - Agent status tracked via pane existence + tool calls
3. **Pure MCP** - All communication via MCP tools, no custom HTTP
4. **Explicit Control** - Supervisor approves every agent stop
5. **Simple Workflow** - 4 steps instead of complex state machines

## Adoption Timeline

### Week 1: Learning & Planning
- [ ] Read SIMPLIFIED_PROMPT_SUMMARY.md
- [ ] Read AGENT_PROMPT_CHEATSHEET.md
- [ ] Review AGENT_PROMPT_VARIANTS.md
- [ ] Plan which prompts to convert

### Week 2: Implementation
- [ ] Create new agent prompts using VARIANTS
- [ ] OR convert existing prompts using MIGRATE guide
- [ ] Test with actual agents
- [ ] Fix any issues

### Week 3: Validation
- [ ] Run agents with new prompts
- [ ] Verify registration works
- [ ] Verify signal_captain works
- [ ] Verify request_stop_approval works
- [ ] Verify supervisor approval flow

### Week 4: Rollout
- [ ] Update all configurations
- [ ] Archive old prompts
- [ ] Update team documentation
- [ ] Monitor for issues

## File Locations

All agent prompt templates and examples are located in:
```
docs/
├── SIMPLIFIED_PROMPT_README.md          ← You are here
├── SIMPLIFIED_PROMPT_SUMMARY.md         ← Executive overview
├── AGENT_PROMPT_CHEATSHEET.md          ← Quick reference
├── AGENT_PROMPT_TEMPLATE.md            ← Full reference
├── AGENT_PROMPT_VARIANTS.md            ← Copy-paste templates
└── MIGRATE_PROMPTS_GUIDE.md            ← Conversion guide
```

Agent-specific prompts are in:
```
configs/prompts/
├── [being updated to simplified structure]
```

## Usage Scenarios

### Scenario 1: Creating a New Implementation Agent
1. Open `AGENT_PROMPT_VARIANTS.md`
2. Find "Code Implementation Agent (SGT-Green)" section
3. Copy the entire block
4. Customize with your project path and task
5. Deploy as agent prompt
6. Done!

### Scenario 2: Converting Existing Review Prompt
1. Open `MIGRATE_PROMPTS_GUIDE.md`
2. Follow "Pattern 1: Code Reviewer" section
3. Use find-and-replace patterns provided
4. Test with agent
5. Validate using checklist
6. Update configuration

### Scenario 3: Creating Custom Role (Not in Variants)
1. Open `AGENT_PROMPT_TEMPLATE.md`
2. Use base structure from section "Simplified Prompt Template Structure"
3. Customize section 2 (Workflow) with your role-specific steps
4. Customize section 4 (Task-Specific Instructions) with your role details
5. Test and iterate

### Scenario 4: Troubleshooting Agent Issues
1. Check `AGENT_PROMPT_CHEATSHEET.md` section "Common Mistakes & Fixes"
2. If not there, check `MIGRATE_PROMPTS_GUIDE.md` section "Troubleshooting Migration Issues"
3. If still not resolved, review `AGENT_PROMPT_TEMPLATE.md` section "Error Handling"

## MCP Tools Reference (Essentials)

These three tools are used in EVERY agent prompt:

| Tool | Purpose | Called By |
|------|---------|-----------|
| `register_agent(agent_id, role)` | Register at startup | Agent (step 1) |
| `signal_captain(signal, context, work_completed)` | Signal completion/error | Agent (step 3) |
| `request_stop_approval(reason, context, work_completed)` | Request exit permission | Agent (step 4) |

Additional tools available (see full documentation):
- `log_activity()` - Log progress to dashboard
- `report_progress()` - Send progress updates
- `request_human_input()` - Ask for help
- `get_my_tasks()` - List assigned tasks
- And 35+ more specialized tools

See `AGENT_PROMPT_TEMPLATE.md` section "Available MCP Tools" for complete list.

## Validation Checklist

Before deploying a prompt, verify:

- [ ] Agent registers successfully
- [ ] Agent performs task as specified
- [ ] signal_captain() can be called
- [ ] request_stop_approval() can be called
- [ ] Supervisor can approve/reject
- [ ] Agent exits cleanly after approval
- [ ] No SSE or heartbeat code
- [ ] No polling loops
- [ ] Prompt is clear and concise
- [ ] Role-specific success criteria defined

See `AGENT_PROMPT_CHEATSHEET.md` for deployment checklist.

## Common Questions

**Q: Why change the prompt structure?**
A: The new structure removes complexity (SSE, heartbeats, polling) while improving control (explicit approval). Result: simpler, more reliable agents.

**Q: Do I have to use these templates?**
A: For consistency and reliability, yes. For experimentation, the template is flexible - customize section 2 as needed.

**Q: What if my task is complex?**
A: The 4-step workflow is simple; task instructions (step 2) expand to cover complexity.

**Q: Can I skip the request_stop_approval call?**
A: No. This is mandatory. Supervisor needs final say on when agents stop.

**Q: How do I test if my prompt works?**
A: Run actual agent with new prompt, verify all 4 steps complete, check approval flow. See cheatsheet for details.

**Q: What about long-running tasks?**
A: Use `report_progress()` or `log_activity()` during work (step 2). Still complete with signal → request_approval.

## Troubleshooting

### "Agent won't register"
- Verify MCP endpoint is running (`http://localhost:3000/mcp`)
- Check X-Agent-ID header is set correctly
- See MIGRATE_PROMPTS_GUIDE.md "Troubleshooting" section

### "signal_captain not working"
- Verify you're calling MCP tool, not custom HTTP
- Check tool parameters match documentation
- See AGENT_PROMPT_CHEATSHEET.md "Common Mistakes"

### "Agent keeps waiting for approval"
- Verify supervisor is running
- Check approval response event system
- May need to approve manually via dashboard

### "Old prompt structure doesn't work"
- Use MIGRATE_PROMPTS_GUIDE.md for step-by-step conversion
- Follow validation checklist to ensure new prompt works

## Quick Links

- **Architecture Details**: See `docs/plans/2025-12-21-pure-mcp-architecture.md`
- **MCP Implementation**: See `internal/mcp/handlers.go`
- **Agent Spawning**: See `internal/agents/spawner.go`
- **Captain Orchestration**: See `internal/captain/captain.go`

## Feedback & Issues

Found a problem or have suggestions?
1. Check if it's in troubleshooting section
2. Review relevant documentation
3. Test against validation checklist
4. Document and report issues

## Version History

- **1.0** (2025-12-21): Initial release
  - Simplified prompt structure based on MCP Streamable HTTP
  - Removed SSE, heartbeats, connection state management
  - Introduced 4-step workflow (register → work → signal → approve)
  - Provided templates for 5 agent roles
  - Comprehensive documentation (5 documents)

---

## Start Here

**New to simplified prompts?**
1. Read: `SIMPLIFIED_PROMPT_SUMMARY.md` (5 min)
2. Skim: `AGENT_PROMPT_CHEATSHEET.md` (5 min)
3. Copy: Template from `AGENT_PROMPT_VARIANTS.md` (5 min)
4. Test: With actual agent (10 min)

**Total time to productive**: ~25 minutes

**Converting existing prompt?**
1. Read: `MIGRATE_PROMPTS_GUIDE.md` (15 min)
2. Follow: Step-by-step conversion (10 min)
3. Validate: Using provided checklist (5 min)
4. Deploy: With confidence

**Total time to migration**: ~30 minutes

---

**Questions?** Check the document index above for your use case.

**Ready?** Start with `SIMPLIFIED_PROMPT_SUMMARY.md`.

Good luck! 🚀
