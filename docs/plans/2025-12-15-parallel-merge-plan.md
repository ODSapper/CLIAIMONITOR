# Parallel Merge Plan - 2025-12-15

## Current State
- **Master**: 17 commits ahead of origin with cluster, events, OpenAPI, cron, git-deploy merged
- **Unmerged branches**: 12 MAH task branches
- **Stashes**: 33 stashes, including security fixes in stash@{0}

## Parallel Execution Strategy

### Wave A: Analysis (3 Haiku agents - 2 min each)

| Agent | Task | Focus |
|-------|------|-------|
| Haiku-1 | Analyze stash@{0} | Extract security fixes, identify missing imports/functions |
| Haiku-2 | Check runtime dirs | List files in internal/runtime/{nodejs,python,static,docker} |
| Haiku-3 | Branch diff analysis | For each unmerged branch, count unique commits vs master |

### Wave B: Integration (3 Sonnet agents - 5 min each)

| Agent | Task | Files |
|-------|------|-------|
| Sonnet-1 | Fix & apply stash@{0} security fixes | webhooks, crypto, admin_reports |
| Sonnet-2 | Merge MAH-4F-004-scheduler | internal/scheduler/* |
| Sonnet-3 | Recover runtime code from stashes | Python, Static, Docker hosting |

### Wave C: Verification (2 Haiku agents)

| Agent | Task |
|-------|------|
| Haiku-4 | Build verification + test run |
| Haiku-5 | Git log summary + push to origin |

## Branch Priority

**High (Wave 4 features)**:
1. MAH-4F-004-scheduler - Has scheduler implementation
2. MAH-4A-004-python-hosting - Python/Gunicorn
3. MAH-4A-005-static-sites - Static site generators
4. MAH-4A-010-docker-hosting - Docker containers

**Medium (Already partially merged)**:
- MAH-4C-010-openapi - OpenAPI already in master
- MAH-4D-001-event-stream - Events already in master
- MAH-4E-002-cluster-health - Cluster already in master

**Low (Admin UI - defer)**:
- MAH-ADMIN-004, 005, 006 - Admin routes
- MAH-DOM-001 - Domain validation
- MAH-P3-002 - E2E tests

## Execution Commands

```bash
# Wave A - Run in parallel
# Agent 1: git stash show -p stash@{0} | head -200
# Agent 2: ls -la internal/runtime/*/
# Agent 3: for b in $(git branch --no-merged master | grep MAH); do echo "$b: $(git log --oneline master..$b | wc -l) commits"; done

# Wave B - Run in parallel after Wave A
# Each agent gets specific merge/fix task

# Wave C - Sequential after Wave B
# Build + push
```

## Success Criteria
- [ ] All security fixes from stash@{0} applied
- [ ] Scheduler (MAH-4F-004) merged
- [ ] Runtime hosting code (Python, Static, Docker) recovered
- [ ] Build passes
- [ ] Pushed to origin/master
