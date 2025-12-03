# Security Remediation Deployment Plan

**Date**: 2025-12-02
**Author**: Captain (Orchestrator)
**Status**: Pending Approval

## Executive Summary

Deploy agents to fix 56 pending security and build issues across the Magnolia ecosystem. This plan prioritizes critical security vulnerabilities, then build issues, then lower-priority fixes.

## Task Inventory

| Repo | P1 (Critical) | P2 (High) | P3+ (Lower) | Total |
|------|---------------|-----------|-------------|-------|
| mss-ai | 10 | 1 | 4 | 15 |
| MSS | 7 | 3 | 1 | 11 |
| MAH | 7 | 2 | 1 | 10 |
| mss-suite | 6 | 2 | 2 | 10 |
| planner | 6 | 1 | 1 | 8 |
| magnolia-dev | 1 | 1 | 0 | 2 |
| **Total** | **37** | **10** | **9** | **56** |

---

## Phase 0: Pre-flight Checks (Manual)

Before deploying agents, verify each project can build:

```powershell
# Check each project builds
cd "C:\Users\Admin\Documents\VS Projects\MAH" && go build ./...
cd "C:\Users\Admin\Documents\VS Projects\MSS" && go build ./...
cd "C:\Users\Admin\Documents\VS Projects\mss-ai" && go build ./...
cd "C:\Users\Admin\Documents\VS Projects\planner" && go build ./...
```

**Pre-conditions**:
- [ ] All projects checked out to clean branches
- [ ] No uncommitted changes in working directories
- [ ] CLIAIMONITOR server running at localhost:3000

---

## Phase 1: Build Fixes (Blocking Issues)

These must be fixed first as they prevent testing.

### Wave 1A: MSS Build Fixes
**Agent**: OpusGreen (Priority 1 implementation)
**Project**: MSS
**Tasks**:
1. `MSS-BUILD-001`: Fix duplicate FirewallInterface declaration
2. `MSS-BUILD-002`: Fix yaml.v2 vs v3 import mismatch
3. `MSS-TEST-001`: Fix 62 test failures (path issue)

**Verification**: `go build ./... && go test ./...`

### Wave 1B: MAH Build Fixes (Parallel with 1A)
**Agent**: OpusGreen
**Project**: MAH
**Tasks**:
1. `MAH-BUILD-001`: Fix reseller database schema (7 compilation errors)

**Verification**: `make generate && make build`

---

## Phase 2: Critical Security (Priority 1)

Deploy OpusRed (Security) agents for critical vulnerabilities.

### Wave 2A: MSS-AI Critical (3 tasks)
**Agent**: OpusRed
**Project**: mss-ai
**Tasks**:
1. `MSSAI-CRIT-001`: Remove hardcoded credentials (admin123/user123)
2. `MSSAI-CRIT-002`: Remove InsecureSkipVerify in API testing
3. `MSSAI-CRIT-003`: Implement dry run mechanism

**Claim via API**:
```bash
for ID in MSSAI-CRIT-001 MSSAI-CRIT-002 MSSAI-CRIT-003; do
  curl -X POST "https://plannerprojectmss.vercel.app/api/v1/tasks/$ID/claim" \
    -H "X-API-Key: orchestrator" -H "Content-Type: application/json" \
    -d '{"team_id":"agent-opusred"}'
done
```

### Wave 2B: MAH Critical (2 tasks) - Parallel
**Agent**: OpusRed
**Project**: MAH
**Tasks**:
1. `MAH-CRIT-001`: Fix unsafe CSP configuration
2. `MAH-CRIT-002`: Fix isWhitelistedIP() always returns false

### Wave 2C: MSS-Suite Critical (3 tasks) - Parallel
**Agent**: OpusRed
**Project**: mss-suite
**Tasks**:
1. `SUITE-CRIT-001`: Enable NoNewPrivileges in systemd
2. `SUITE-CRIT-002`: Disable tls_skip_verify
3. `SUITE-CRIT-004`: Fix symlink attack in temp cleanup

### Wave 2D: Planner Critical (2 tasks) - Parallel
**Agent**: OpusRed
**Project**: planner
**Tasks**:
1. `PLAN-CRIT-001`: Fix 17 NPM vulnerabilities
2. `PLAN-CRIT-003`: Replace weak admin secret

---

## Phase 3: High-Priority Security (Priority 1)

Deploy OpusGreen agents for high-severity issues.

### Wave 3A: MSS-AI High Security (7 tasks)
**Agent**: OpusGreen
**Project**: mss-ai
**Batch 1**:
1. `SEC-AI-001`: Wire ValidatePrompt() into API handlers
2. `SEC-AI-002`: Fix command injection in systemctl
3. `SEC-AI-003`: Fix shell injection in security audit

**Batch 2**:
4. `MSSAI-HIGH-001`: Add path validation to CleanTempFilesTool
5. `MSSAI-HIGH-002`: Enforce 32-char JWT secret minimum
6. `MSSAI-HIGH-004`: Add pre-auth rate limiting
7. `MSSAI-HIGH-005`: Implement BoltDB encryption

### Wave 3B: MAH High Security (5 tasks) - Parallel
**Agent**: OpusGreen
**Project**: MAH
**Tasks**:
1. `SEC-MAH-001`: Fix command injection in local provisioning
2. `MAH-HIGH-001`: Fix CSRF cookie HttpOnly=false
3. `MAH-HIGH-002`: Replace string matching SQL/XSS detection
4. `MAH-HIGH-004`: Move admin IP list to configuration

### Wave 3C: MSS High Security (4 tasks) - Parallel
**Agent**: OpusGreen
**Project**: MSS
**Tasks**:
1. `SEC-MSS-001`: Change default credentials
2. `SEC-MSS-002`: Generate HMAC key for audit logs
3. `MSS-HIGH-001`: Audit exec.Command for injection
4. `MSS-HIGH-002`: Fix race condition in Monitor.whitelist

### Wave 3D: MSS-Suite High Security (3 tasks) - Parallel
**Agent**: OpusGreen
**Project**: mss-suite
**Tasks**:
1. `SUITE-HIGH-002`: Fix JWT secret file permissions
2. `SUITE-HIGH-003`: Implement certificate rotation
3. `SUITE-HIGH-004`: Validate version strings

### Wave 3E: Planner High Security (4 tasks) - Parallel
**Agent**: OpusGreen
**Project**: planner
**Tasks**:
1. `SEC-PLAN-001`: Rotate PostgreSQL credentials
2. `SEC-PLAN-002`: Add input sanitization for prompt injection
3. `PLAN-HIGH-001`: Cap SQL LIMIT/OFFSET
4. `PLAN-HIGH-005`: Require auth for GET operations

---

## Phase 4: Medium Priority (Priority 2)

Deploy SNTGreen (Sonnet) agents for standard fixes.

### Wave 4A: All Priority 2 Tasks
**Agent**: SNTGreen (one per repo, parallel)
**Tasks by Repo**:

**MAH** (2 tasks):
- `MAH-SEC-001`: Fix scope validation bypass
- `MAH-SEC-002`: Validate SESSION_SECRET length

**MSS** (3 tasks):
- `MSS-MED-001`: Add BoltDB persistence for blocks
- `MSS-MED-002`: Fix IPv6 validation
- `MSS-SEC-001`: Add HMAC key generation

**mss-ai** (1 task):
- `MAI-SEC-001`: Load trust tier IPs from config

**mss-suite** (2 tasks):
- `SEC-SUITE-001`: Add binary checksum verification
- `SEC-SUITE-002`: Create dedicated service users

**planner** (1 task):
- `PLAN-SEC-001`: Restrict CORS to allowed origins

---

## Phase 5: Verification & Code Review

### Wave 5A: Run All Tests
**Agent**: SNTGreen (parallel per repo)
**Command per repo**:
```bash
# MAH
cd MAH && make generate && go test ./...

# MSS
cd MSS && go test ./...

# mss-ai
cd mss-ai && make test

# planner
cd planner && npm test && go test ./...
```

### Wave 5B: Code Review
**Agent**: SNTPurple (Code Auditor)
**Task**: Review all changes before merge
- Check for regressions
- Verify security fixes are complete
- Ensure no new vulnerabilities introduced

### Wave 5C: Security Scan (Snake)
**Agent**: Snake
**Task**: Re-run reconnaissance on all repos
- Verify findings are resolved
- Update security scores
- Generate final report

---

## Agent Deployment Commands

### Spawn Implementation Agent
```powershell
# Via CLIAIMONITOR API
curl -X POST http://localhost:3000/api/spawn -H "Content-Type: application/json" -d '{
  "agent_type": "OpusRed",
  "project_path": "C:\\Users\\Admin\\Documents\\VS Projects\\mss-ai",
  "initial_prompt": "Claim and implement MSSAI-CRIT-001. Remove hardcoded credentials."
}'
```

### Spawn via Captain Orchestration
```powershell
# Start Captain orchestration loop
curl -X POST http://localhost:3000/api/captain/start

# Add tasks to queue
curl -X POST http://localhost:3000/api/captain/queue -d '{
  "tasks": ["MSSAI-CRIT-001", "MSSAI-CRIT-002", "MSSAI-CRIT-003"]
}'
```

---

## Parallelization Strategy

```
Timeline (not to scale):

Phase 1 ████████ Build fixes (must complete first)
        │
        ├─ Wave 1A: MSS Build ───────────┐
        └─ Wave 1B: MAH Build ───────────┤
                                         │
Phase 2 ████████████████ Critical ───────┤
        │                                │
        ├─ Wave 2A: mss-ai CRIT ─────────┤
        ├─ Wave 2B: MAH CRIT ────────────┤ (parallel)
        ├─ Wave 2C: mss-suite CRIT ──────┤
        └─ Wave 2D: planner CRIT ────────┤
                                         │
Phase 3 ████████████████████████ High ───┤
        │                                │
        ├─ Wave 3A: mss-ai HIGH ─────────┤
        ├─ Wave 3B: MAH HIGH ────────────┤ (parallel)
        ├─ Wave 3C: MSS HIGH ────────────┤
        ├─ Wave 3D: mss-suite HIGH ──────┤
        └─ Wave 3E: planner HIGH ────────┤
                                         │
Phase 4 ████████████ Medium ─────────────┤
        │                                │
        └─ All P2 tasks (parallel) ──────┤
                                         │
Phase 5 ████████ Verify ─────────────────┘
        │
        ├─ Wave 5A: Tests ───────────────┐
        ├─ Wave 5B: Code Review ─────────┤
        └─ Wave 5C: Security Scan ───────┘
```

**Maximum Parallel Agents**: 5 (one per major repo)
**Estimated Agent-Hours**:
- Phase 1: 2 agents × ~30 min = 1 hour
- Phase 2: 4 agents × ~45 min = 45 min (parallel)
- Phase 3: 5 agents × ~1 hour = 1 hour (parallel)
- Phase 4: 5 agents × ~30 min = 30 min (parallel)
- Phase 5: 3 agents × ~30 min = 30 min

**Total Elapsed Time**: ~4 hours (with parallelization)

---

## Risk Mitigation

1. **Build Failures**: Phase 1 must complete before Phase 2
2. **Test Failures**: Run tests after each wave, pause if >10% failure rate
3. **Merge Conflicts**: Each agent works on separate files; review before merge
4. **Agent Stalls**: Captain monitors for 5-min inactivity, escalates to human
5. **Security Regressions**: Phase 5C Snake scan verifies no new vulnerabilities

---

## Success Criteria

- [ ] All P1 tasks completed and merged
- [ ] All projects build successfully
- [ ] All project tests pass (>90% pass rate)
- [ ] Security scan shows no critical/high findings
- [ ] All changes code-reviewed by SNTPurple

---

## Rollback Plan

If issues arise:
1. **Per-task rollback**: `git revert <commit>` for individual fixes
2. **Per-phase rollback**: Restore from branch checkpoint before phase
3. **Full rollback**: Return to pre-deployment state via backup branches

Branch naming: `security-remediation-2025-12-02-phase-N`

---

## Approval

- [ ] Captain approval (automatic)
- [ ] Human approval (required for deployment)

**Next Step**: Upon approval, execute Phase 1 (Build Fixes)
