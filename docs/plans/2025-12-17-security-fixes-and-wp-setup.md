# Security Fixes and WordPress Docker Setup Plan

**Date:** 2025-12-17
**Objective:** Fix all security issues found + set up WordPress test sites accessible on localhost

---

## Part 1: Security Fixes (Parallel Execution)

### Stream A: MAH Fixes (Agent 1)

#### HIGH-001: Disable Directory Listing on /static/
**File:** `C:\Users\Admin\Documents\VS Projects\MAH\cmd\mah\main.go` (line 322)
**Current:**
```go
r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.Dir(staticPath))))
```
**Fix:** Create a custom file server that returns 403 for directory requests

#### HIGH-002: Enable HSTS in Docker/Development Mode
**File:** `C:\Users\Admin\Documents\VS Projects\MAH\internal\middleware\security.go` (line 25-28)
**Current:**
```go
// HSTS in production
if os.Getenv("ENV") == "production" {
    w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
}
```
**Fix:** Add env var `FORCE_HSTS=true` option for Docker/testing OR always enable HSTS

#### MED-004: Add Secure and SameSite to Session Cookies
**File:** Search for cookie setting code in auth handlers
**Fix:** Add `Secure: true, SameSite: http.SameSiteStrictMode` when ENV=production or HTTPS

---

### Stream B: MSS Fixes (Agent 2)

#### HIGH-003: Document/Standardize Dual Auth Methods
**File:** `C:\Users\Admin\Documents\VS Projects\MSS\pkg\api\server.go` (authMiddleware ~line 473)
**Current:** Both X-API-Key and Basic Auth work on /api/status
**Fix Options:**
1. Document this as intentional (API key for service-to-service, Basic Auth for admin)
2. OR: Create separate endpoints for each auth type
**Recommendation:** Document as intentional - this is actually useful for flexibility

#### MED-001: Add Server-Side Auth to MSS Dashboard
**File:** `C:\Users\Admin\Documents\VS Projects\MSS\pkg\api\server.go`
**Fix:** Ensure `/dashboard` route goes through authMiddleware

#### LOW-003: Remove/Protect Debug Endpoints
**Check:** Verify if /debug/pprof is actually exposed
**Fix:** If exposed, remove or protect with auth

---

### Stream C: Cleanup (Agent 3 - Parallel)

#### Clean Up Test Reports
Move or archive test reports to a dedicated location:
```
CLIAIMONITOR/
├── test-reports/           # Keep as archive
│   └── 2025-12-17-integration-test/
│       ├── reseller-setup-report.json
│       ├── security-monitor-phase1.json
│       ├── enduser1-wp-report.json
│       ├── enduser2-wp-report.json
│       ├── ctf-security-report.json
│       └── test-evidence.txt
├── INTEGRATION_TEST_FINAL_REPORT.md  → Move to docs/reports/
└── SECURITY_EXECUTIVE_SUMMARY.md     → Move to docs/reports/
```

---

## Part 2: WordPress Docker Setup

### Goal
Two WordPress test sites accessible like MSS/MAH:
- **Site 1:** http://localhost:8081 (wp1.localhost)
- **Site 2:** http://localhost:8082 (wp2.localhost)

### Current State
`docker-compose.wordpress-test.yml` already exists with:
- wordpress1 on port 8081
- wordpress2 on port 8082
- Shared MySQL database

### Missing Pieces
1. **init-wordpress-dbs.sql** - Need to create this to initialize both databases
2. **wordpress-security/** directory - Referenced but may not exist
3. **Auto-install script** - WP needs to be installed after first boot

### Implementation

#### 1. Create Database Init Script
**File:** `mss-suite/docker/init-wordpress-dbs.sql`
```sql
CREATE DATABASE IF NOT EXISTS wordpress1;
CREATE DATABASE IF NOT EXISTS wordpress2;
GRANT ALL PRIVILEGES ON wordpress1.* TO 'wordpress'@'%';
GRANT ALL PRIVILEGES ON wordpress2.* TO 'wordpress'@'%';
FLUSH PRIVILEGES;
```

#### 2. Create WordPress Security Directory
```
mss-suite/docker/wordpress-security/
├── mu-plugins/
│   └── security-hardening.php
├── .htaccess
├── robots.txt
└── .well-known/
    └── security.txt
```

#### 3. Create WP Auto-Install Script
Create a shell script that:
1. Waits for WordPress containers to be healthy
2. Runs wp-cli to install WordPress
3. Creates admin user
4. Configures permalink structure

#### 4. Update Docker Compose
Add depends_on and healthcheck improvements

---

## Execution Plan

### Phase 1: Parallel Fixes (~30 min)

| Agent | Tasks | Est. Time |
|-------|-------|-----------|
| **Agent A** | MAH: Directory listing fix, HSTS fix, Session cookies | 20 min |
| **Agent B** | MSS: Document dual auth, Dashboard auth check | 15 min |
| **Agent C** | Cleanup: Organize test reports, move docs | 10 min |

### Phase 2: WordPress Setup (~20 min)

| Task | Est. Time |
|------|-----------|
| Create init SQL script | 5 min |
| Create security directory structure | 5 min |
| Create auto-install script | 10 min |
| Test full stack startup | 10 min |

### Phase 3: Verification

```bash
# Start full stack
cd mss-suite/docker
docker compose -f docker-compose.mss-mah.local.yml \
               -f docker-compose.wordpress-test.yml \
               --env-file .env.mss-mah up -d

# Verify all services
curl http://localhost:8080/health  # MAH
curl http://localhost:8090/api/health  # MSS
curl http://localhost:8081/  # WordPress 1
curl http://localhost:8082/  # WordPress 2

# Verify security fixes
curl -sI http://localhost:8080/static/ | grep -E "HTTP|403"  # Should be 403
curl -sI http://localhost:8080 | grep -i "Strict-Transport"  # Should have HSTS
```

---

## Files to Create/Modify

### New Files
1. `mss-suite/docker/init-wordpress-dbs.sql`
2. `mss-suite/docker/wordpress-security/mu-plugins/security-hardening.php`
3. `mss-suite/docker/wordpress-security/.htaccess`
4. `mss-suite/docker/wordpress-security/robots.txt`
5. `mss-suite/docker/wordpress-security/.well-known/security.txt`
6. `mss-suite/docker/wp-auto-install.sh`

### Modified Files
1. `MAH/cmd/mah/main.go` - Add no-listing file server
2. `MAH/internal/middleware/security.go` - HSTS always or config option
3. `MAH/internal/auth/sessions.go` (or similar) - Session cookie security
4. `MSS/pkg/api/server.go` - Dashboard auth check
5. `mss-suite/docker/docker-compose.wordpress-test.yml` - Add auto-install

---

## Success Criteria

### Security Fixes
- [ ] `curl http://localhost:8080/static/` returns 403 (not directory listing)
- [ ] `curl -sI http://localhost:8080` includes `Strict-Transport-Security`
- [ ] MSS `/dashboard` requires authentication
- [ ] All session cookies have `Secure` and `SameSite` attributes (in production)

### WordPress Setup
- [ ] http://localhost:8081 shows WordPress site 1
- [ ] http://localhost:8082 shows WordPress site 2
- [ ] Both sites are pre-installed with admin credentials
- [ ] Sites are isolated (separate databases)
- [ ] Security hardening plugins active

---

## Estimated Total Time
- **Phase 1 (Parallel):** 20-30 minutes
- **Phase 2 (WordPress):** 20-30 minutes
- **Verification:** 10 minutes
- **Total:** ~1 hour
