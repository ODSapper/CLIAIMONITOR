# Wave 3 & 4 Parallel Review Plan

## Overview
8 branches to review across Wave 3 and Wave 4. Using parallel Haiku/Sonnet subagents for efficient review.

## Model Assignment Strategy

| Branch | Lines | Complexity | Model | Rationale |
|--------|-------|------------|-------|-----------|
| **Wave 3** |
| MAH-4A-001 Git deploy | ~1,700 | HIGH | Sonnet | Security-sensitive deployment logic |
| MAH-4A-002 GitHub/GitLab webhooks | 2,606 | HIGH | Sonnet | Signature verification, secrets handling |
| MAH-4C-003 WebSocket events | 1,314 | MEDIUM | Haiku | Standard concurrency patterns |
| MAH-3D-001 Cron manager | 1,530 | MEDIUM | Haiku | Scheduling logic, well-defined scope |
| **Wave 4** |
| MAH-4A-003 Node.js hosting | 2,619 | HIGH | Sonnet | PM2 process management, security |
| MAH-4A-004 Python hosting | 1,987 | MEDIUM | Haiku | Standard venv/Gunicorn patterns |
| MAH-4A-005 Static sites | 3,000 | MEDIUM | Haiku | Build detection, standard patterns |
| MAH-4A-010 Docker hosting | 2,692 | HIGH | Sonnet | Container security critical |

**Summary**: 4 Sonnet + 4 Haiku = 8 parallel reviews

## Review Checklist (for each agent)

1. **Security**
   - [ ] No command injection vulnerabilities
   - [ ] Proper input validation
   - [ ] Secrets not logged or exposed
   - [ ] SQL injection prevention (parameterized queries)

2. **Error Handling**
   - [ ] Sentinel errors defined
   - [ ] Proper error wrapping with context
   - [ ] No silent error swallowing

3. **Concurrency**
   - [ ] Proper mutex usage where needed
   - [ ] No race conditions
   - [ ] Context cancellation respected

4. **Code Quality**
   - [ ] Consistent naming conventions
   - [ ] No dead code
   - [ ] Proper resource cleanup (defer)

## Execution Plan

### Phase 1: Parallel Reviews (8 agents)
```
Sonnet agents: MAH-4A-001, MAH-4A-002, MAH-4A-003, MAH-4A-010
Haiku agents:  MAH-4C-003, MAH-3D-001, MAH-4A-004, MAH-4A-005
```

### Phase 2: Collect Results
- Wait for all reviews to complete
- Categorize: APPROVE / NEEDS_CHANGES
- Document specific issues for each NEEDS_CHANGES

### Phase 3: Fix Issues
- Spawn fix agents for branches with issues
- Use same model that found the issue (knows the context)
- Commit fixes to existing branches

### Phase 4: Re-review (if needed)
- Only for branches that had critical issues
- Quick Haiku pass to verify fixes

### Phase 5: Merge
- Sequential merge to master (Wave 3 first, then Wave 4)
- Resolve any conflicts
- Mark tasks as implemented on Planner API

## Expected Timeline
- Phase 1: ~5-10 min (parallel)
- Phase 2: ~1 min (collection)
- Phase 3: ~5-10 min per fix (parallel)
- Phase 4: ~2 min if needed
- Phase 5: ~5 min (sequential merges)
