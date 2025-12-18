# MAH Remaining Tasks Implementation Plan
**Date**: 2025-12-15
**Excludes**: 4B tasks (WordPress Go replacement)
**Focus**: Enable traditional PHP WordPress to run easily on MAH

---

## Executive Summary

14 pending tasks organized into 4 waves for parallel execution. WordPress (PHP) hosting is already well-supported in MAH with existing toolkit - just needs UI wiring.

---

## Task Inventory (Excluding 4B)

### Priority 1 - WordPress PHP Support
| Task ID | Title | Complexity |
|---------|-------|------------|
| MAH-3F-001 | Implement WordPress ListSites and ShowInstallForm handlers | Medium |

### Priority 2 - Developer Tools (4F Series)
| Task ID | Title | Complexity |
|---------|-------|------------|
| MAH-4F-002 | Database web IDE | High |
| MAH-4F-005 | SSL debugging tools | Medium |
| MAH-4F-006 | Email debugging | Medium |
| MAH-4F-007 | Performance profiler | High |
| MAH-4F-008 | One-click dev environments | Medium |
| MAH-4F-009 | Collaborative access | Medium |

### Priority 2 - Core Features (TODO Series)
| Task ID | Title | Complexity |
|---------|-------|------------|
| MAH-TODO-010 | Backup file deletion cleanup | Low |
| MAH-TODO-011 | Email quota file updates | Low |
| MAH-TODO-012 | Product limits configuration parsing | Medium |
| MAH-TODO-013 | Email template rendering | Medium |

### Priority 3 - Polish & Infrastructure
| Task ID | Title | Complexity |
|---------|-------|------------|
| MAH-4F-010 | Dark mode | Low |
| MAH-TODO-014 | Archive max file size tests | Low |
| SUITE-ALPHA-005 | Add Prometheus + Grafana monitoring | Medium |

---

## Wave 1: WordPress & Core Cleanup (4 tasks)

### MAH-3F-001: WordPress ListSites and ShowInstallForm handlers
**Files to modify**:
- `internal/panel/wordpress/handler.go` - Add ListSites, ShowInstallForm methods
- `internal/panel/wordpress/sites.templ` - Create template (if not exists)
- `internal/panel/wordpress/install.templ` - Create install form template

**Implementation**:
```go
// In handler.go - add these methods:
func (h *Handler) ListSites(w http.ResponseWriter, r *http.Request) {
    // Query wordpress_sites table
    // Render sites list with status, version, last update
}

func (h *Handler) ShowInstallForm(w http.ResponseWriter, r *http.Request) {
    // Show form with:
    // - Site title, admin email, admin username
    // - Domain/subdomain selection
    // - PHP version selection
    // - Database auto-provisioning option
}
```

**Existing foundation**: WordPress toolkit at `internal/panel/wordpress/` already has:
- WP-CLI integration (`cli.go`)
- Installation logic (`handler.go:InstallWordPress`)
- Database schema (`db/migrations/009_wordpress_toolkit.sql`)

### MAH-TODO-010: Backup file deletion cleanup
**Files**: `internal/backup/cleanup.go` or similar
**Implementation**: Add cleanup job for expired backup files

### MAH-TODO-011: Email quota file updates
**Files**: `internal/email/quota.go`
**Implementation**: Track email quota in filesystem quota files

### MAH-TODO-014: Archive max file size tests
**Files**: `internal/archive/*_test.go`
**Implementation**: Add test cases for max file size limits

---

## Wave 2: Developer Tools - Part 1 (3 tasks)

### MAH-4F-002: Database web IDE
**New files**:
- `internal/panel/database/ide_handler.go`
- `internal/panel/database/ide.templ`
- `web/static/js/db-ide.js` (SQL editor)

**Features**:
- SQL query editor with syntax highlighting
- Query execution against user's MySQL/PostgreSQL
- Results table view with export (CSV, JSON)
- Query history
- Table browser with structure view

**Security**:
- Queries run as user's DB user (not root)
- Query timeout limits
- Result set size limits

### MAH-4F-005: SSL debugging tools
**New files**:
- `internal/panel/ssl/debug_handler.go`
- `internal/panel/ssl/debug.templ`

**Features**:
- Certificate chain viewer
- Expiry checker with warnings
- SSL Labs grade integration
- Certificate decoder (PEM/DER)
- CSR generator and validator
- Mixed content scanner

### MAH-4F-006: Email debugging
**New files**:
- `internal/panel/email/debug_handler.go`
- `internal/panel/email/debug.templ`

**Features**:
- SMTP test sender (send test email)
- SPF/DKIM/DMARC record checker
- Mail queue viewer
- Delivery log viewer
- Blacklist checker (Spamhaus, etc.)

---

## Wave 3: Developer Tools - Part 2 (3 tasks)

### MAH-4F-007: Performance profiler
**New files**:
- `internal/panel/profiler/handler.go`
- `internal/panel/profiler/profiler.templ`
- `web/static/js/profiler.js`

**Features**:
- Request timing breakdown
- Database query analysis (slow query log)
- Memory/CPU usage graphs
- PHP profiling (Xdebug integration)
- Node.js profiling (--inspect integration)
- Lighthouse scores integration

### MAH-4F-008: One-click dev environments
**New files**:
- `internal/panel/devenv/handler.go`
- `internal/panel/devenv/devenv.templ`

**Features**:
- Staging environment creation (clone production)
- Development branch deployment
- Environment variable management
- Database seeding with sanitized data
- One-click reset to clean state

**Integration**: Use existing Docker runtime (`internal/runtime/docker/`)

### MAH-4F-009: Collaborative access
**New files**:
- `internal/panel/collab/handler.go`
- `internal/panel/collab/collab.templ`
- `db/migrations/XXX_collaborative_access.sql`

**Features**:
- Invite team members by email
- Role-based access (Admin, Developer, Viewer)
- SSH key management per collaborator
- Activity audit log
- Access token generation for CI/CD

---

## Wave 4: Polish & Infrastructure (3 tasks)

### MAH-4F-010: Dark mode
**Files to modify**:
- `web/static/css/styles.css` - Add CSS variables for dark theme
- `web/templates/layout.templ` - Add theme toggle
- `web/static/js/theme.js` - Theme persistence in localStorage

**Implementation**:
```css
:root {
  --bg-primary: #ffffff;
  --text-primary: #1a1a1a;
  /* ... */
}

[data-theme="dark"] {
  --bg-primary: #1a1a1a;
  --text-primary: #f5f5f5;
  /* ... */
}
```

### MAH-TODO-012: Product limits configuration parsing
**Files**: `internal/billing/limits.go`
**Implementation**: Parse product limit configs from database/YAML

### MAH-TODO-013: Email template rendering
**Files**: `internal/email/templates/`
**Implementation**: Templ-based email templates with variable substitution

### SUITE-ALPHA-005: Prometheus + Grafana monitoring
**Repo**: mss-suite
**Files**:
- `docker/docker-compose.monitoring.yml`
- `docker/prometheus/prometheus.yml`
- `docker/grafana/dashboards/mah.json`

**Implementation**:
- Add Prometheus container
- Add Grafana container with pre-configured dashboards
- Expose metrics endpoint from MAH (`/metrics`)
- Expose metrics endpoint from MSS (`/metrics`)

---

## Parallel Execution Strategy

```
Wave 1 (4 agents, ~2 hours)
├── Agent A: MAH-3F-001 (WordPress UI)
├── Agent B: MAH-TODO-010 (Backup cleanup)
├── Agent C: MAH-TODO-011 (Email quota)
└── Agent D: MAH-TODO-014 (Archive tests)

Wave 2 (3 agents, ~3 hours)
├── Agent A: MAH-4F-002 (Database IDE)
├── Agent B: MAH-4F-005 (SSL debugging)
└── Agent C: MAH-4F-006 (Email debugging)

Wave 3 (3 agents, ~3 hours)
├── Agent A: MAH-4F-007 (Performance profiler)
├── Agent B: MAH-4F-008 (Dev environments)
└── Agent C: MAH-4F-009 (Collaborative access)

Wave 4 (3 agents, ~2 hours)
├── Agent A: MAH-4F-010 (Dark mode)
├── Agent B: MAH-TODO-012 + MAH-TODO-013 (Limits + Email templates)
└── Agent C: SUITE-ALPHA-005 (Prometheus/Grafana)
```

---

## WordPress PHP Hosting - Current State

MAH already supports WordPress hosting well:

### Existing Features (in `internal/panel/wordpress/`)
- One-click WordPress installation via WP-CLI
- Auto-updates for core, plugins, themes
- Staging environment creation
- Security hardening (disable editor, XML-RPC)
- Site cloning with database migration
- Vulnerability scanning
- Multisite network management
- Backup/restore operations

### What MAH-3F-001 Adds
The missing piece is **UI wiring**:
- `ListSites` handler to show all WordPress installations
- `ShowInstallForm` handler to render the installation wizard

### PHP Support (Already Exists)
Located in `internal/handlers/php_handler.go`:
- PHP version selection per account
- PHP-FPM management (start/stop/restart)
- PHP configuration editing
- Error log viewing

---

## Dependencies & Prerequisites

### For Database IDE (MAH-4F-002)
- Monaco Editor or CodeMirror for SQL highlighting
- Query execution sandboxing

### For Performance Profiler (MAH-4F-007)
- Xdebug PHP extension available
- Node.js --inspect flag support
- Lighthouse CLI or PageSpeed API

### For Prometheus/Grafana (SUITE-ALPHA-005)
- MAH needs `/metrics` endpoint (add `promhttp` handler)
- MSS needs `/metrics` endpoint

---

## Risk Mitigation

| Risk | Mitigation |
|------|------------|
| Database IDE SQL injection | Run queries as user's DB user, not admin |
| Dev environment data leakage | Sanitize production data before cloning |
| Collaborative access privilege escalation | Strict RBAC with audit logging |
| Performance profiler overhead | Profiling disabled by default, time-limited |

---

## Success Criteria

1. **WordPress Hosting**: User can install WordPress in < 5 clicks
2. **Developer Tools**: All 4F tools functional and accessible from panel
3. **Core Features**: Backup cleanup, email quota working in background
4. **Infrastructure**: Grafana dashboards showing MAH/MSS metrics

---

## Recommended Execution Order

**Start immediately** (no dependencies):
- Wave 1: All 4 tasks

**After Wave 1**:
- Wave 2: Developer tools part 1

**After Wave 2**:
- Wave 3: Developer tools part 2

**After Wave 3**:
- Wave 4: Polish and monitoring

Total: 14 tasks across 4 waves, ~10 hours parallel execution time
