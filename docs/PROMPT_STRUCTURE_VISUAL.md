# Simplified Agent Prompt Structure - Visual Guide

## The 4-Step Workflow (Visual)

```
┌─────────────────────────────────────────────────────────────────┐
│                       Agent Lifecycle                            │
└─────────────────────────────────────────────────────────────────┘

┌──────────────┐
│   STEP 1     │  register_agent(agent_id, role)
│  REGISTER    │  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
│              │  Tell Captain you're online
└──────┬───────┘  Returns: {status: "registered"}
       │
       ▼
┌──────────────────────────────────────────┐
│         STEP 2: EXECUTE TASK             │
│                                          │
│  ┌────────────────────────────────────┐ │
│  │ Read requirements, understand task │ │
│  └────────────────────────────────────┘ │
│                   ↓                      │
│  ┌────────────────────────────────────┐ │
│  │ Implement / Review / Test / Analyze│ │
│  │ Write code, check logic, findings  │ │
│  └────────────────────────────────────┘ │
│                   ↓                      │
│  ┌────────────────────────────────────┐ │
│  │ Verify quality (tests, coverage)   │ │
│  │ Commit and push changes            │ │
│  └────────────────────────────────────┘ │
│                   ↓                      │
│           Work Complete                 │
│                                          │
└──────────────────┬───────────────────────┘
                   │
                   ▼
┌──────────────┐
│   STEP 3     │  signal_captain(signal, context, work_completed)
│   SIGNAL     │  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
│              │  Tell Captain about task status
└──────┬───────┘  Signals: completed | blocked | error | need_guidance
       │          Returns: {status: "signaled"}
       │
       ▼
┌──────────────────────────────────────────┐
│    STEP 4: REQUEST STOP APPROVAL         │
│                                          │
│   request_stop_approval(                 │
│     reason="task_complete",              │
│     context="...",                       │
│     work_completed="..."                 │
│   )                                      │
│                                          │
│   Returns: {status: "pending",           │
│             request_id: "..."}           │
│                                          │
│   Wait for supervisor response           │
│                                          │
└──────────────────┬───────────────────────┘
                   │
         ┌─────────┴─────────┐
         │                   │
         ▼                   ▼
    APPROVED             REJECTED
         │                   │
         ▼                   ▼
      exit()          Continue work or
                   request different approval

```

---

## Architecture Comparison

### BEFORE (Old Architecture)
```
┌──────────────────────────────────────────────────┐
│         Complex State Management                 │
├──────────────────────────────────────────────────┤
│                                                  │
│  SSE Connection Setup                            │
│  └─ GET /mcp/sse with X-Agent-ID header         │
│  └─ Receive session ID                          │
│  └─ POST to /mcp/messages with session_id       │
│                                                  │
│  Heartbeat Loop                                  │
│  └─ Every 15 seconds send ping                  │
│  └─ Track connection state                      │
│  └─ Reconnect on failure                        │
│                                                  │
│  Status Polling                                  │
│  └─ Every 5 seconds poll /api/agent/status      │
│  └─ Check if task complete                      │
│  └─ Retry on timeout                            │
│                                                  │
│  Custom Endpoints                                │
│  └─ POST /api/agents/report with results        │
│  └─ POST /api/agents/wait-for-approval          │
│  └─ Custom error handling                       │
│                                                  │
│  Implicit Exit                                   │
│  └─ Exit when task done (no approval needed)    │
│  └─ Risk: supervisor doesn't know you exited    │
│  └─ Risk: work in progress is lost              │
│                                                  │
└──────────────────────────────────────────────────┘

Problems: 50-80 lines of infrastructure code
          8+ failure modes
          Complex state machine
          Implicit, uncontrolled exits
```

### AFTER (New Architecture - Simplified)
```
┌──────────────────────────────────────────────────┐
│        Pure MCP-Based (Stateless)                │
├──────────────────────────────────────────────────┤
│                                                  │
│  1. Register (One call)                         │
│     register_agent(agent_id, role)              │
│                                                  │
│  2. Work (Your code here)                       │
│     [Implement/Review/Test/Analyze]             │
│                                                  │
│  3. Signal (One call)                           │
│     signal_captain(signal, context,...)         │
│                                                  │
│  4. Request Approval (One call, then wait)      │
│     request_stop_approval(reason, context,...)  │
│                                                  │
│  Connection Details:                             │
│  └─ HTTP POST to /mcp endpoint                  │
│  └─ X-Agent-ID header per request               │
│  └─ Stateless (no session tracking)             │
│  └─ MCP tools handle everything                 │
│                                                  │
│  Benefits:                                       │
│  └─ 30-40 lines of code (vs 50-80)             │
│  └─ 2 failure modes (vs 8+)                     │
│  └─ Simple, predictable flow                    │
│  └─ Explicit supervisor control                 │
│  └─ No heartbeats, no polling                   │
│                                                  │
└──────────────────────────────────────────────────┘

Benefits: Simpler, faster, more reliable
          Supervisor has full control
          No connection state management
          Explicit approval gates
```

---

## Template Structure (Visual)

```
┌─────────────────────────────────────────────────────┐
│           Agent Prompt Template                     │
├─────────────────────────────────────────────────────┤
│                                                     │
│  # Identity Section                                 │
│  ├─ Role & Name                                     │
│  ├─ MCP Endpoint (http://localhost:3000/mcp)       │
│  ├─ Agent ID (team-agenttype###)                   │
│  └─ Project Path                                    │
│                                                     │
│  # Workflow Section (THE 4 STEPS)                   │
│  ├─ Step 1: Register Agent                         │
│  │  └─ register_agent(agent_id, role)              │
│  │                                                  │
│  ├─ Step 2: Execute Task [CUSTOMIZABLE]            │
│  │  ├─ Understand requirements                     │
│  │  ├─ Implement/Review/Analyze                    │
│  │  ├─ Verify quality                              │
│  │  └─ Commit/Document                             │
│  │                                                  │
│  ├─ Step 3: Signal Completion                      │
│  │  └─ signal_captain(signal, context,...)         │
│  │                                                  │
│  └─ Step 4: Request Approval                       │
│     └─ request_stop_approval(...) + wait           │
│                                                     │
│  # Support Sections                                 │
│  ├─ Success Criteria (Checklist)                    │
│  ├─ Key MCP Tools (Reference)                       │
│  ├─ Important Rules (Don'ts)                        │
│  └─ Error Handling (What if stuck?)                │
│                                                     │
└─────────────────────────────────────────────────────┘

Sections 1, 3, 4, & support: STANDARDIZED
Section 2 (task execution): CUSTOMIZABLE PER ROLE
```

---

## MCP Tools Hierarchy

```
┌─────────────────────────────────────────────────────┐
│              MCP Tools Available                    │
├─────────────────────────────────────────────────────┤
│                                                     │
│  ⭐ ESSENTIAL (Called by every agent)              │
│  ├─ register_agent(agent_id, role)                │
│  ├─ signal_captain(signal, context, work_done)    │
│  └─ request_stop_approval(reason, context, work)  │
│                                                     │
│  📝 COMMON (Most agents use these)                 │
│  ├─ log_activity(action, details)                 │
│  ├─ report_progress(status, pct, note)            │
│  ├─ request_human_input(question, context)        │
│  └─ get_my_tasks(status_filter)                   │
│                                                     │
│  🔧 SPECIALIZED (Role-specific)                   │
│  ├─ complete_task(task_id, summary)               │
│  ├─ store_knowledge(knowledge_dict)               │
│  ├─ search_knowledge(query, category)             │
│  ├─ record_episode(episode_data)                  │
│  └─ [35+ more available]                          │
│                                                     │
│  🎯 ADVANCED (For complex scenarios)               │
│  ├─ request_guidance(context)                     │
│  ├─ save_context(key, value, priority)            │
│  ├─ get_context(key)                              │
│  └─ [More for special cases]                      │
│                                                     │
└─────────────────────────────────────────────────────┘

For most tasks: Only need the 3 ESSENTIAL tools
For standard workflows: Add a few COMMON tools
For special roles: Add SPECIALIZED tools
For advanced: Use ADVANCED tools as needed
```

---

## Workflow by Role (Simplified View)

### Implementation (Green)
```
Register → Read Task → Implement → Test → Commit → Signal → Approve
           └──────────── STEP 2: Do Work ────────────┘
```

### Code Review (Purple)
```
Register → Read Code → Review → Document Issues → Signal → Approve
           └──────────── STEP 2: Do Work ────────────┘
```

### Security Audit (Red)
```
Register → Scan Code → Find Issues → Document → Signal → Approve
           └────────────── STEP 2: Do Work ──────────┘
```

### Reconnaissance (Snake)
```
Register → Analyze → Map Structure → Report Findings → Signal → Approve
           └──────────────── STEP 2: Do Work ─────────────┘
```

### Testing
```
Register → Run Tests → Capture Results → Report Metrics → Signal → Approve
           └────────────────── STEP 2: Do Work ──────────────┘
```

---

## Document Architecture

```
┌─────────────────────────────────────────────────────────┐
│        Simplified Prompt Documentation Suite            │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  📖 LEARNING PATH                                       │
│  ├─ SIMPLIFIED_PROMPT_README.md                        │
│  │  └─ (You are here) - Navigation & overview          │
│  │                                                      │
│  ├─ SIMPLIFIED_PROMPT_SUMMARY.md                       │
│  │  └─ Executive overview (5 min read)                 │
│  │                                                      │
│  ├─ AGENT_PROMPT_CHEATSHEET.md                         │
│  │  └─ Quick reference during development (5 min)      │
│  │                                                      │
│  └─ AGENT_PROMPT_TEMPLATE.md                           │
│     └─ Complete reference guide (20 min deep dive)     │
│                                                         │
│  📋 IMPLEMENTATION PATH                                 │
│  ├─ AGENT_PROMPT_VARIANTS.md                           │
│  │  └─ Copy-paste templates for 5 roles                │
│  │     (Pick your role, 5 min to customize)            │
│  │                                                      │
│  └─ configs/prompts/[role].md                          │
│     └─ Actual deployed prompts                         │
│                                                         │
│  🔄 MIGRATION PATH                                      │
│  └─ MIGRATE_PROMPTS_GUIDE.md                           │
│     └─ Step-by-step conversion (15 min)                │
│                                                         │
│  📊 VISUAL GUIDE (This file)                            │
│  └─ PROMPT_STRUCTURE_VISUAL.md                         │
│     └─ Diagrams and visual explanations                │
│                                                         │
└─────────────────────────────────────────────────────────┘

                  Start Here
                      │
                      ▼
        SIMPLIFIED_PROMPT_README.md (You are here)
                      │
        ┌─────────────┼─────────────┐
        │             │             │
    LEARN        IMPLEMENT       MIGRATE
        │             │             │
        ▼             ▼             ▼
    CHEATSHEET    VARIANTS       GUIDE
     TEMPLATE     configs/
```

---

## Success Criteria at a Glance

```
✅ PROMPT IS GOOD IF:
├─ Has clear 4-step workflow
├─ No SSE or heartbeat references
├─ Uses only MCP tools
├─ Calls register_agent first
├─ Calls signal_captain on completion
├─ Calls request_stop_approval before exit
├─ Is < 50 lines
├─ Has no polling loops
├─ Has no connection state code
└─ Tests pass with actual agents

❌ PROMPT NEEDS FIXING IF:
├─ Still references SSE connections
├─ Has heartbeat/ping code
├─ Uses custom HTTP endpoints
├─ Doesn't call register_agent
├─ Exits without request_stop_approval
├─ Has polling loops
├─ Manages connection state
├─ Is > 80 lines
└─ Agents can't register or stop
```

---

## Time to Productive

### New Agent (Using Templates)
```
Read CHEATSHEET (5 min)
         ↓
Copy VARIANT (2 min)
         ↓
Customize for project (3 min)
         ↓
Test with agent (10 min)
         ↓
DEPLOY (1 min)

Total: ~20 minutes
```

### Converting Existing Prompt
```
Read MIGRATION GUIDE (10 min)
         ↓
Apply find & replace (5 min)
         ↓
Validate with checklist (5 min)
         ↓
Test with agent (10 min)
         ↓
DEPLOY (1 min)

Total: ~30 minutes
```

### Understanding Architecture
```
Read SUMMARY (5 min)
    ↓
Read CHEATSHEET (5 min)
    ↓
Skim TEMPLATE (10 min)
    ↓
Review VARIANTS (5 min)
    ↓
UNDERSTAND (1 min)

Total: ~30 minutes to deep understanding
```

---

## One-Minute Summary

**Old Way**: Complex SSE + heartbeats + polling + custom endpoints = 50-80 lines of infrastructure code

**New Way**: Simple 4-step workflow using MCP tools = 30-40 lines total

**The Pattern**:
1. `register_agent()` - Tell Captain you're here
2. Work - Do your actual task
3. `signal_captain()` - Tell Captain you're done
4. `request_stop_approval()` - Ask permission to exit

**That's it.** Use the templates. Customize task details. Deploy. Done.

---

## Navigation Quick Links

Start with one of these:

- **I want to create a new agent** → `AGENT_PROMPT_VARIANTS.md`
- **I want to understand the architecture** → `AGENT_PROMPT_TEMPLATE.md`
- **I want a quick reference** → `AGENT_PROMPT_CHEATSHEET.md`
- **I'm converting an old prompt** → `MIGRATE_PROMPTS_GUIDE.md`
- **I need an overview** → `SIMPLIFIED_PROMPT_SUMMARY.md`
- **I'm lost** → Start here with this document

---

**Ready to start?** Pick your path above and follow the links.

**Questions?** Check the FAQ section in the document that matches your path.

**Let's go!** 🚀
