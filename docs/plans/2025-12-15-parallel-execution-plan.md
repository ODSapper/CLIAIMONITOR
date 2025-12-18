# Parallel Execution Plan - 2025-12-15

## Task Inventory: 66 Pending Tasks

### By Category:

| Series | Count | Complexity | Recommended Model |
|--------|-------|------------|-------------------|
| 4B (Go WordPress) | 10 | VERY HIGH | Skip for now - needs architectural decisions |
| 4A (Modern App Hosting) | 10 | MEDIUM-HIGH | Sonnet |
| 4C (API-First) | 9 | MEDIUM | Mix (Haiku for CRUD, Sonnet for complex) |
| 4D (AI-Ready) | 8 | MEDIUM-HIGH | Sonnet |
| 4E (Multi-Server) | 8 | HIGH | Sonnet |
| 4F (Developer Experience) | 8 | MEDIUM | Mix |
| TODO fixes | 8 | LOW-MEDIUM | Haiku |
| Phase 3 | 3 | MEDIUM | Mix |
| Suite | 1 | MEDIUM | Sonnet |

## Constraint: Single Repository (MAH)

All tasks modify the same repo, so we must:
1. Assign tasks to DIFFERENT packages/directories
2. Create separate branches for each task
3. Merge sequentially after each wave

## Parallel Execution Strategy

### WAVE 1: Simple/Independent Tasks (6 agents)
*Focus: Low-risk, isolated changes that don't overlap*

| Task | Title | Model | Package/Files | Est. Tokens |
|------|-------|-------|---------------|-------------|
| MAH-TODO-007 | PHP settings template | Haiku | internal/php/ | 5K |
| MAH-TODO-008 | System command handler | Haiku | internal/system/ | 5K |
| MAH-TODO-009 | Metrics collector fields | Haiku | internal/metrics/ | 5K |
| MAH-4C-004 | API key management | Haiku | internal/api/keys/ | 8K |
| MAH-4C-005 | Rate limiting feedback | Haiku | internal/middleware/ | 8K |
| MAH-4D-010 | Audit trail for AI | Haiku | internal/audit/ | 8K |

**Rationale**: All touch different packages, simple CRUD/feature additions

---

### WAVE 2: Medium Complexity (5 agents)
*Focus: Feature additions requiring more logic*

| Task | Title | Model | Package/Files | Est. Tokens |
|------|-------|-------|---------------|-------------|
| MAH-4A-006 | Env vars manager | Sonnet | internal/deploy/env/ | 15K |
| MAH-4A-007 | Build logs & history | Sonnet | internal/deploy/logs/ | 15K |
| MAH-4C-009 | Zapier/n8n integration | Haiku | internal/integrations/ | 10K |
| MAH-4D-008 | Incident timeline | Sonnet | internal/incidents/ | 15K |
| MAH-3E-001 | Admin reports | Sonnet | internal/admin/reports/ | 15K |

---

### WAVE 3: Core Features (4 agents)
*Focus: Significant new capabilities*

| Task | Title | Model | Package/Files | Est. Tokens |
|------|-------|-------|---------------|-------------|
| MAH-4A-001 | Git deployment | Sonnet | internal/deploy/git/ | 20K |
| MAH-4A-002 | GitHub/GitLab webhooks | Sonnet | internal/webhooks/ | 20K |
| MAH-4C-003 | WebSocket events | Sonnet | internal/ws/ | 20K |
| MAH-3D-001 | Cron job manager | Sonnet | internal/cron/ | 20K |

---

### WAVE 4: App Hosting (4 agents)
*Focus: Runtime environment support*

| Task | Title | Model | Package/Files | Est. Tokens |
|------|-------|-------|---------------|-------------|
| MAH-4A-003 | Node.js hosting (PM2) | Sonnet | internal/runtime/nodejs/ | 25K |
| MAH-4A-004 | Python hosting | Sonnet | internal/runtime/python/ | 25K |
| MAH-4A-005 | Static site generators | Sonnet | internal/runtime/static/ | 25K |
| MAH-4A-010 | Docker container hosting | Sonnet | internal/runtime/docker/ | 30K |

---

### WAVE 5: Advanced Features (4 agents)
*Focus: Complex integrations*

| Task | Title | Model | Package/Files | Est. Tokens |
|------|-------|-------|---------------|-------------|
| MAH-4C-002 | GraphQL API | Sonnet | internal/graphql/ | 30K |
| MAH-4C-008 | CLI tool (mah-cli) | Sonnet | cmd/mah-cli/ | 30K |
| MAH-4D-009 | Slack/Discord bot | Sonnet | internal/chatops/ | 25K |
| MAH-4F-001 | SSH web terminal | Sonnet | internal/terminal/ | 30K |

---

### WAVE 6: Multi-Server Foundation (3 agents)
*Focus: Clustering infrastructure*

| Task | Title | Model | Package/Files | Est. Tokens |
|------|-------|-------|---------------|-------------|
| MAH-4E-001 | Agent-based architecture | Sonnet | internal/agent/ | 35K |
| MAH-4E-003 | Account placement algorithms | Sonnet | internal/placement/ | 25K |
| MAH-4E-004 | Live migration | Sonnet | internal/migration/ | 30K |

---

### DEFERRED: Phase 4B (Go-Powered WordPress)
*Reason: Requires architectural decisions, very high complexity*

- MAH-4B-001 through MAH-4B-010 (10 tasks)
- These should be planned separately with human input
- Each task is 50K+ tokens and highly interdependent

---

## Execution Protocol

### For Each Wave:

1. **Pre-flight**
   - Claim tasks on Planner API
   - Create branches: `task/{TASK-ID}-description`

2. **Spawn Agents** (parallel)
   - Haiku: Simple tasks (< 10K tokens expected)
   - Sonnet: Medium/complex tasks (10K-35K tokens)

3. **Monitor**
   - Track via CLIAIMONITOR dashboard
   - Watch for stop requests

4. **Review** (parallel with Haiku reviewers)
   - Code review each branch
   - Fix critical issues

5. **Merge**
   - Sequential merge to master
   - Resolve any conflicts
   - Mark tasks complete on Planner

---

## Cost Estimate

| Model | Tasks | Avg Tokens | Total Tokens | Est. Cost |
|-------|-------|------------|--------------|-----------|
| Haiku | 12 | 7K | 84K | ~$0.08 |
| Sonnet | 24 | 22K | 528K | ~$1.60 |
| **Total** | **36** | - | **612K** | **~$1.70** |

*Note: 4B series (10 tasks) excluded from estimate*

---

## Recommended First Wave

Start with **WAVE 1** (6 Haiku agents):
- Low risk, isolated changes
- Fast execution (~5 min each)
- Good test of parallel infrastructure
- Total cost: ~$0.05

```bash
# Tasks for Wave 1:
MAH-TODO-007, MAH-TODO-008, MAH-TODO-009, MAH-4C-004, MAH-4C-005, MAH-4D-010
```
