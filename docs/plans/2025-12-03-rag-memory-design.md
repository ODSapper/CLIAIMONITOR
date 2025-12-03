# RAG Memory System Design

**Date**: 2025-12-03
**Status**: Approved
**Author**: Captain (Claude Opus)

---

## Overview

Add a learning memory system to CLIAIMONITOR that enables agents (primarily Captain) to:
- Store knowledge learned from experience
- Retrieve relevant knowledge when facing similar situations
- Build continuity across sessions through episode summaries
- Replace clunky PowerShell heartbeat scripts with unified Go communication

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Skill Router Service                      │
│                    (Single Go process)                       │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐  │
│  │  Knowledge  │  │  Episode    │  │  Agent Comms        │  │
│  │  Queries    │  │  Queries    │  │  ──────────────     │  │
│  │             │  │             │  │  • Heartbeat recv   │  │
│  └──────┬──────┘  └──────┬──────┘  │  • Status updates   │  │
│         │                │         │  • Message routing  │  │
│         ▼                ▼         │  • Shutdown signals │  │
│  ┌─────────────────────────────┐  └──────────┬──────────┘  │
│  │        learning.db          │             │             │
│  └─────────────────────────────┘             │             │
│                                              ▼             │
│                               ┌─────────────────────────┐  │
│                               │    operational.db       │  │
│                               │  (agent state, tasks)   │  │
│                               └─────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
         ▲              ▲              ▲
         │              │              │
    ┌────┴────┐   ┌────┴────┐   ┌────┴────┐
    │ Agent 1 │   │ Agent 2 │   │ Captain │
    │  (MCP)  │   │  (MCP)  │   │         │
    └─────────┘   └─────────┘   └─────────┘
```

## Key Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Embedding approach | TF-IDF first, embeddings optional | Self-contained, no external APIs, works offline |
| Memory types | Episodes + Knowledge | Procedures already in operational.db |
| Knowledge capture | Manual via MCP tools | Start simple, avoid noise, agent decides what matters |
| Database separation | learning.db + operational.db | Keep RAG memory separate from operational state |
| Communication | Skill Router replaces heartbeat scripts | Single Go process, cleaner than PowerShell polling |

## Database Schema

### learning.db

```sql
-- Episodes: What happened (timestamped events for context)
CREATE TABLE episodes (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    agent_id TEXT NOT NULL,
    event_type TEXT NOT NULL,  -- 'action', 'error', 'decision', 'outcome'
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    project TEXT,
    importance REAL DEFAULT 0.5,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_episodes_session ON episodes(session_id);
CREATE INDEX idx_episodes_agent ON episodes(agent_id);
CREATE INDEX idx_episodes_project ON episodes(project);

-- Knowledge: What was learned (searchable solutions/patterns)
CREATE TABLE knowledge (
    id TEXT PRIMARY KEY,
    category TEXT NOT NULL,    -- 'error_solution', 'pattern', 'best_practice', 'gotcha'
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    tags TEXT,                 -- JSON array of tags
    source TEXT,
    use_count INTEGER DEFAULT 0,
    last_used DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_knowledge_category ON knowledge(category);

-- TF-IDF index table
CREATE TABLE knowledge_terms (
    knowledge_id TEXT NOT NULL,
    term TEXT NOT NULL,
    tf REAL NOT NULL,
    FOREIGN KEY (knowledge_id) REFERENCES knowledge(id) ON DELETE CASCADE
);

CREATE INDEX idx_terms_term ON knowledge_terms(term);

-- Document frequency for IDF calculation
CREATE TABLE term_stats (
    term TEXT PRIMARY KEY,
    doc_count INTEGER DEFAULT 1
);
```

### Knowledge Categories

| Category | Use Case | Example |
|----------|----------|---------|
| `error_solution` | How to fix specific errors | "SQL scan error with DECIMAL→int: use float64 intermediates" |
| `pattern` | Code patterns that work well | "AuthProvider public paths pattern for Next.js" |
| `best_practice` | General good approaches | "Always read file before editing" |
| `gotcha` | Things that trip you up | "PATCH not PUT for partial updates" |

## MCP Tools

### store_knowledge
Store something learned for future retrieval.

```
Parameters:
  - category: string  // error_solution, pattern, best_practice, gotcha
  - title: string     // Brief summary
  - content: string   // Full details
  - tags: []string    // Optional tags for filtering
Returns:
  - knowledge_id: string
```

### search_knowledge
Find relevant learnings using TF-IDF search.

```
Parameters:
  - query: string     // Natural language query
  - category: string  // Optional filter
  - limit: int        // Default 5
Returns:
  - results: [{id, title, content, category, relevance_score}]
```

### record_episode
Log what happened in current session.

```
Parameters:
  - event_type: string  // action, error, decision, outcome
  - title: string
  - content: string
  - project: string     // Optional
  - importance: float   // 0-1, default 0.5
Returns:
  - episode_id: string
```

### get_recent_episodes
Get context from current/recent sessions.

```
Parameters:
  - session_id: string  // Optional, defaults to current
  - limit: int          // Default 10
Returns:
  - episodes: [{id, event_type, title, content, created_at}]
```

### search_episodes
Find past similar situations.

```
Parameters:
  - query: string
  - project: string     // Optional filter
  - limit: int
Returns:
  - results: [{id, title, content, session_id, relevance_score}]
```

## Skill Router

The Skill Router categorizes incoming queries and routes to the appropriate data source:

| Query Type | Route To | Example |
|------------|----------|---------|
| `knowledge` | learning.db → knowledge | "How do I fix auth redirect?" |
| `episode` | learning.db → episodes | "What happened last session?" |
| `operational` | operational.db → agents/tasks | "What agents are running?" |
| `recon` | operational.db → recon | "What vulnerabilities in MSS?" |

### Agent Communication Endpoints

Replace PowerShell heartbeat scripts:

```
POST /heartbeat      - Agent check-in
POST /status         - Status update
GET  /messages       - SSE stream for incoming messages/signals
POST /signal         - Send signal to another agent
GET  /shutdown-check - Quick poll: should I shut down?
```

## Phased Implementation

### Phase 1: Learning Memory for Captain
**Goal**: Add RAG memory so Captain can learn from sessions

Tasks:
1. Add learning.db schema and migration (004_learning_db.sql)
2. Implement TF-IDF search in pure Go
3. Add 5 MCP tools to handlers.go
4. Register tools in server.go
5. Test with real usage

**Deliverable**: Captain can store and retrieve knowledge

### Phase 2: Skill Router Service
**Goal**: Replace PowerShell heartbeat scripts with Go service

Tasks:
1. Build Skill Router HTTP handlers
2. Add agent communication endpoints
3. Integrate query routing (learning vs operational)
4. Update agent spawner to use HTTP instead of PowerShell
5. Remove heartbeat script spawning

**Deliverable**: No more PowerShell heartbeat scripts

### Phase 3: Port to CLIAIRMONITOR
**Goal**: Standalone version for local LLM deployment

Tasks:
1. Copy learning memory + Skill Router to CLIAIRMONITOR
2. Add LLM interface abstraction
3. Add context builder with token budget
4. Test with Aider + Qwen3-coder

**Deliverable**: Airgapped CLIAIRMONITOR with RAG memory

## Example Usage

When fixing a bug:
```
1. search_knowledge("SQL DECIMAL scan error")
   → Finds: "Use float64 intermediates for DECIMAL columns"
   → Apply the fix

2. store_knowledge(
     category: "error_solution",
     title: "PATCH vs PUT for partial updates",
     content: "Use PATCH not PUT when updating single fields...",
     tags: ["api", "http", "rest"]
   )
```

When starting a session:
```
1. get_recent_episodes(limit: 5)
   → See what happened last time

2. search_knowledge("planner dashboard")
   → Get relevant context for current work
```

## Files to Create/Modify

### New Files
- `internal/memory/learning.go` - LearningDB implementation
- `internal/memory/tfidf.go` - TF-IDF search implementation
- `internal/memory/migrations/004_learning_db.sql` - Schema
- `internal/router/router.go` - Skill Router service
- `internal/router/comms.go` - Agent communication handlers

### Modified Files
- `internal/mcp/handlers.go` - Add learning MCP tools
- `internal/mcp/server.go` - Register new tools
- `internal/server/server.go` - Mount Skill Router endpoints
- `internal/agents/spawner.go` - Remove heartbeat script spawning

## Success Criteria

Phase 1 complete when:
- [ ] Can store knowledge via MCP tool
- [ ] Can search and retrieve relevant knowledge
- [ ] Can record and query episodes
- [ ] TF-IDF search returns sensible results

Phase 2 complete when:
- [ ] Agents check in via HTTP, not PowerShell
- [ ] Skill Router routes queries correctly
- [ ] No heartbeat PowerShell processes spawned
- [ ] Shutdown signals work via HTTP

Phase 3 complete when:
- [ ] CLIAIRMONITOR runs standalone
- [ ] Works with local LLM (Aider + Qwen)
- [ ] Context builder respects token limits
- [ ] RAG retrieval improves responses
