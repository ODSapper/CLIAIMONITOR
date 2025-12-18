# QA Issues Fix Plan

**Date**: 2025-12-16
**Status**: In Progress
**Execution**: Parallel subagents (haiku/sonnet)

---

## Issues to Fix

### Issue 1: MAH Setup SQL Syntax Error (CRITICAL)

**Location**: `MAH/internal/handlers/setup.go:119`

**Problem**: The SQL query uses `?` placeholder which is SQLite syntax. PostgreSQL uses `$1`, `$2`, etc.

**Current Code**:
```go
_, err = h.db.ExecContext(r.Context(), "UPDATE users SET role = 'admin' WHERE id = ?", user.ID)
```

**Fix**: Change to PostgreSQL-compatible syntax:
```go
_, err = h.db.ExecContext(r.Context(), "UPDATE users SET role = 'admin' WHERE id = $1", user.ID)
```

**Impact**: Blocks system initialization in PostgreSQL environments
**Effort**: Low (1 line change)
**Assigned**: Haiku subagent

---

### Issue 2: MAH Missing hosting_account_id Column (MEDIUM)

**Location**: `MAH/internal/handlers/metrics.go` (Prometheus metrics query)

**Problem**: The Prometheus metrics collection queries `domains.hosting_account_id` but this column may not exist if migration 015 wasn't run.

**Root Cause**: Migration `015_add_hosting_account_domain_support.sql` adds the column but may not have been executed on existing databases.

**Fix Options**:
1. Add the column via the metrics handler initialization check
2. Make the Prometheus query handle missing column gracefully
3. Create an auto-migration check

**Files to Check**:
- `MAH/internal/handlers/metrics.go` - Find the Prometheus query
- `MAH/db/migrations/015_add_hosting_account_domain_support.sql` - Reference

**Impact**: Causes error logging in Prometheus metrics
**Effort**: Medium (need to find and fix the query)
**Assigned**: Sonnet subagent

---

### Issue 3: MSS Audit Log Timeout (MEDIUM)

**Location**: `MSS/pkg/api/audit.go:115-161`

**Problem**: The `GetRecent()` method reads the ENTIRE audit log file sequentially:
```go
for {
    var entry AuditEntry
    if err := decoder.Decode(&entry); err != nil {
        break // EOF or decode error
    }
    entries = append(entries, entry)
}
```

For large audit logs, this causes timeouts because:
1. It reads the entire file into memory
2. No timeout or size limit on file reading
3. O(n) time complexity where n = total audit entries

**Fix Options**:
1. **In-memory buffer** (Recommended): Keep last N entries in a circular buffer
2. **Reverse file reading**: Read from end of file (more complex)
3. **Add timeout**: Wrap file reading with context timeout

**Impact**: Audit log endpoints timeout on large log files
**Effort**: Medium (need to implement efficient GetRecent)
**Assigned**: Sonnet subagent

---

## Execution Plan

### Phase 1: Parallel Fixes (3 subagents)

| Agent | Model | Task |
|-------|-------|------|
| Agent 1 | Haiku | Fix MAH setup.go SQL placeholder |
| Agent 2 | Sonnet | Fix MAH metrics.go hosting_account_id query |
| Agent 3 | Sonnet | Fix MSS audit.go GetRecent timeout |

### Phase 2: Verification

After all fixes complete:
1. Run `go build` on both MAH and MSS
2. Run `go test ./...` on both projects
3. Restart test environment and verify fixes

---

## Files Modified

### MAH Project
- `internal/handlers/setup.go` - SQL placeholder fix
- `internal/handlers/metrics.go` - Graceful handling of missing column

### MSS Project
- `pkg/api/audit.go` - Efficient GetRecent implementation

---

## Success Criteria

1. MAH setup POST endpoint succeeds in PostgreSQL
2. MAH metrics endpoint no longer logs hosting_account_id errors
3. MSS audit log endpoints respond within 500ms
4. All unit tests pass
5. Go build succeeds for both projects
