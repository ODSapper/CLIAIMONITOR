# QA Issues Fix Plan - Round 2

**Date**: 2025-12-16
**Status**: Ready for Execution
**Execution**: Parallel subagents (haiku/sonnet)

---

## Root Cause Analysis

### Issue 1: MAH Domains Table Missing Columns (CRITICAL)
**Location**: `MAH/Dockerfile.prebuilt` line 17

**Problem**: The Docker container copies `db/schema.sql` (SQLite version) instead of `db/schema.postgres.sql` (PostgreSQL version). The PostgreSQL schema includes:
- `locked` column (line 307)
- `hosting_account_id` column (line 313)
- `domain_type` column (line 314)
- `document_root` column (line 315)
- `ssl_enabled` column (line 316)

But the SQLite schema does NOT have these columns, causing:
- `/api/v1/domains` returns 500 error
- Prometheus metrics fail with "column d.hosting_account_id does not exist"

**Fix**: Update Dockerfile.prebuilt to copy schema.postgres.sql when using PostgreSQL:
```dockerfile
# Copy appropriate schema based on database type
COPY db/schema.postgres.sql /app/db/schema.sql
```

OR update docker-entrypoint.sh to detect and run the correct schema.

**Impact**: Blocks domain management, metrics collection
**Effort**: Low (1 line change in Dockerfile)
**Assigned**: Haiku subagent

---

### Issue 2: MSS Firewall API Requires Session Auth (MEDIUM)
**Location**: `MSS/pkg/api/server.go` lines 360-369

**Problem**: The X-API-Key authentication middleware is only applied to `/api/suite/*` endpoints:
```go
s.mux.HandleFunc("/api/suite/tier-sync", s.apiKeyAuth.Middleware(...))
s.mux.HandleFunc("/api/suite/tier-list", s.apiKeyAuth.Middleware(...))
```

But firewall endpoints (`/api/firewall/*`, `/api/audit/*`) use session-based authentication through cookies, making them inaccessible to API clients.

**Fix Options**:
1. **Option A (Recommended)**: Add API key auth to firewall endpoints as alternative to session auth
2. **Option B**: Create separate API-only firewall endpoints under `/api/v2/firewall/*`
3. **Option C**: Support both auth methods on existing endpoints

**Files to Modify**:
- `MSS/pkg/api/server.go` - Add apiKeyAuth middleware to firewall routes
- `MSS/pkg/api/auth_middleware.go` - Optional: Create combo auth middleware

**Impact**: Cannot programmatically manage firewall
**Effort**: Medium (need to add auth to multiple routes)
**Assigned**: Sonnet subagent

---

### Issue 3: MAH Health Reports SQLite (LOW)
**Location**: `MAH/internal/api/health.go` line 76

**Problem**: The health handler hardcodes SQLite:
```go
response := HealthResponse{
    ...
    DatabaseType: "sqlite",  // HARDCODED!
}
```

This is misleading when PostgreSQL is actually being used.

**Fix**: Detect actual database driver from connection:
```go
// Detect database type from driver
func (h *HealthHandler) getDatabaseType() string {
    if h.db == nil {
        return "none"
    }
    // Use driver name from sql.DB
    return h.db.Driver().String() // or check connection string
}
```

**Impact**: Cosmetic/debugging issue
**Effort**: Low (small code change)
**Assigned**: Haiku subagent

---

### Issue 4: No Seed Data for Testing (LOW)
**Location**: `MAH/db/seed.sql`

**Problem**: No products/packages configured, making it impossible to test provisioning workflow without manual data entry.

**Fix**: Add seed data to db/seed.sql:
```sql
-- Test products
INSERT INTO products (name, slug, type, description, provider_type, active) VALUES
('Basic Hosting', 'basic-hosting', 'shared_hosting', 'Basic shared hosting plan', 'local', TRUE),
('Pro Hosting', 'pro-hosting', 'shared_hosting', 'Professional hosting plan', 'local', TRUE);

-- Test pricing
INSERT INTO pricing (product_id, option_combination, monthly_price, annual_price) VALUES
(1, '{}', 5.99, 59.99),
(2, '{}', 12.99, 129.99);
```

**Impact**: Testing blocked without manual setup
**Effort**: Low (add SQL inserts)
**Assigned**: Haiku subagent

---

## Execution Plan

### Phase 1: Parallel Fixes (4 subagents)

| Agent | Model | Task | Files |
|-------|-------|------|-------|
| Agent 1 | Haiku | Fix MAH Dockerfile to use PostgreSQL schema | `MAH/Dockerfile.prebuilt`, `MAH/Dockerfile.worker.prebuilt` |
| Agent 2 | Sonnet | Add API key auth to MSS firewall endpoints | `MSS/pkg/api/server.go` |
| Agent 3 | Haiku | Fix MAH health.go database type detection | `MAH/internal/api/health.go` |
| Agent 4 | Haiku | Add seed data for testing | `MAH/db/seed.sql` |

### Phase 2: Rebuild & Verify
After all fixes complete:
1. Rebuild MAH Linux binaries: `GOOS=linux GOARCH=amd64 go build -o mah-linux ./cmd/mah`
2. Rebuild MSS Linux binary: `GOOS=linux GOARCH=amd64 go build -o mss-linux ./cmd/mss`
3. Run `go build` and `go test` on both projects
4. Restart Docker environment
5. Verify fixes with API tests

---

## Files to Modify

### MAH Project
- `Dockerfile.prebuilt` - Use schema.postgres.sql
- `Dockerfile.worker.prebuilt` - Use schema.postgres.sql
- `internal/api/health.go` - Detect actual database type
- `db/seed.sql` - Add test products and pricing

### MSS Project
- `pkg/api/server.go` - Add API key auth to firewall routes

---

## Success Criteria

1. `/api/v1/domains` returns 200 OK (not 500)
2. Prometheus metrics no longer log "hosting_account_id does not exist"
3. MSS firewall endpoints accessible via X-API-Key header
4. MAH health endpoint reports correct database type
5. Seed data allows immediate provisioning testing
6. All unit tests pass
7. Go build succeeds for both projects
