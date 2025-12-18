# MAH API Infrastructure Audit
**Date:** 2025-12-14
**Project:** MAH (Magnolia Account Hosting)
**Scope:** Assess REST API, GraphQL, WebSocket, API Key Management, Rate Limiting, OpenAPI Specs, and CLI tooling

---

## Executive Summary

The MAH project has **solid REST API foundations** with chi router, JWT auth, service token support, rate limiting, and audit logging. However, several areas require enhancement for Phase 4C tasks:

| Category | Status | Coverage |
|----------|--------|----------|
| **REST API** | COMPLETE | `/api/v1` with 30+ endpoints |
| **GraphQL** | MISSING | No GraphQL schema or resolvers |
| **WebSocket** | MISSING | No real-time event streaming |
| **API Key Management** | PARTIAL | Service tokens exist, needs UI/admin panel |
| **Rate Limiting** | IMPLEMENTED | Trust-tier aware limiting, needs tuning |
| **OpenAPI Specs** | MINIMAL | 1 spec file (accounts.yaml), many endpoints undocumented |
| **CLI Tool** | MISSING | No dedicated MAH CLI (gh-magnolia exists for planner) |

---

## 1. REST API Coverage - COMPLETE

### Location
- **Main Router:** `C:\Users\Admin\Documents\VS Projects\MAH\cmd\mah\main.go` (lines 281-556)
- **API Handlers:** `C:\Users\Admin\Documents\VS Projects\MAH\internal\api\`
- **Router Library:** `github.com/go-chi/chi/v5`

### API v1 Endpoints (30+)

#### User & Auth
- `GET /api/v1/me` - Current user info
- JWT Bearer token auth required for all `/api/v1` routes

#### Services
- `GET /api/v1/services` - List services
- `GET /api/v1/services/{id}` - Service details

#### Hosting Accounts
- `GET /api/v1/hosting-accounts` - List accounts
- `GET /api/v1/hosting-accounts/{id}` - Account details
- `GET /api/v1/hosting-accounts/{id}/php` - PHP config
- `PUT /api/v1/hosting-accounts/{id}/php/version` - Set PHP version
- `PUT /api/v1/hosting-accounts/{id}/php/settings` - Update PHP settings

#### Account Lifecycle (NEW)
- `POST /api/v1/accounts` - Create account
- `GET /api/v1/accounts` - List accounts
- `GET /api/v1/accounts/{id}` - Account details
- `DELETE /api/v1/accounts/{id}` - Delete account
- `POST /api/v1/accounts/{id}/suspend` - Suspend account
- `POST /api/v1/accounts/{id}/unsuspend` - Unsuspend account
- `GET /api/v1/accounts/{id}/status` - Provisioning status

#### PHP Management
- `GET /api/v1/php/versions` - Available PHP versions

#### Domains
- `GET /api/v1/domains` - List domains
- `GET /api/v1/domains/{id}` - Domain details

#### Files (Phase 1)
- `api.RegisterFileRoutes(r, db)` - Not detailed in main

#### Databases (Phase 1)
- `api.RegisterDatabaseRoutes(r, dbAPIHandler)` - Not detailed in main

#### FTP (Phase 1)
- `api.RegisterFTPRoutes(r, db)` - Not detailed in main

#### Invoices
- `GET /api/v1/invoices` - List invoices
- `GET /api/v1/invoices/{id}` - Invoice details

#### Metrics
- `GET /api/v1/metrics/overview` - User metrics overview
- `GET /api/v1/metrics/service/{id}` - Service metrics
- `GET /api/v1/metrics/service/{id}/history` - Historical metrics
- `GET /api/v1/admin/metrics/top-disk-users` - Admin: top disk users
- `GET /api/v1/admin/metrics/top-bandwidth-users` - Admin: top bandwidth users
- `GET /api/v1/admin/metrics/over-quota` - Admin: over-quota services

#### Security Alerts (Service Token Auth)
- `POST /api/v1/security/alerts` - Create security alert

#### Billing (Webhooks, no auth)
- `RegisterBillingRoutes(r, db, webhookSecretKey)` - Stripe/payment webhooks

### Implementation Details
```go
// API authentication middleware
r.Route("/api/v1", func(r chi.Router) {
    r.Use(api.RequireAPIAuth(queries))  // JWT Bearer token validation
    // ... endpoints
})
```

### Strengths
- Clear separation of concerns (handlers, middleware, database layers)
- Proper HTTP status codes
- JSON response marshaling
- Middleware pattern for auth and CORS

---

## 2. GraphQL Support - MISSING

**Status:** NOT IMPLEMENTED

### Findings
- No GraphQL server, schema, or resolvers in the codebase
- No `*.graphql` or `*.gql` files found
- No GraphQL dependencies in go.mod
- Some mention of "schema" in test files but related to database, not GraphQL

### Recommendation for Phase 4C
If GraphQL is required:
1. Add `github.com/99designs/gqlgen` dependency
2. Create `api/schema.graphql` with schema definitions
3. Implement resolvers in `internal/api/graphql/`
4. Wire GraphQL handler in main router (example pattern exists in handlers)

---

## 3. WebSocket Real-Time Events - MISSING

**Status:** NOT IMPLEMENTED

### Findings
- No WebSocket handlers found (searched for `websocket`, `WS`, `gorilla/websocket`)
- Some references to WebSocket in unrelated contexts (AWS DNS, S3, etc.)
- No real-time event streaming capability

### Recommendation for Phase 4C
Needed for:
- Real-time service status updates
- Provisioning progress notifications
- Live metrics streaming

Suggested implementation:
1. Use `github.com/gorilla/websocket` package
2. Create `internal/websocket/` handler
3. Implement event bus pattern with channels
4. Register at `/ws` or `/api/v1/ws`

---

## 4. API Key Management - PARTIAL

### Exists: Service Tokens

**Location:** `C:\Users\Admin\Documents\VS Projects\MAH\internal\auth\service_tokens.go`

#### Features Implemented
✓ Generate service tokens (CLI only: `mah create-service-token`)
✓ SHA256 hashing for secure storage
✓ Bearer token validation
✓ Scope-based access control (JSON array of scopes)
✓ Token expiration support
✓ Last-used tracking
✓ Middleware: `RequireServiceAuth(db, requiredScope)`

#### Example Token Creation
```bash
mah create-service-token mss-ai '["security:alerts"]' 'MSS-AI service token'
# Output: plaintext token (shown once)
```

#### Scope Examples
- `security:alerts` - Post security alerts
- `admin:read` - Read admin data
- `api:write` - Write API operations

### Missing: Admin UI

**NOT IMPLEMENTED**

- No service token management in admin panel
- No token listing/viewing/revocation UI
- No scope management UI
- No token creation form in web interface

### Recommendation for Phase 4C
1. Add service token CRUD to `/admin/api-tokens/` routes
2. Create admin template: `templates/admin/api_tokens.html`
3. Add handlers in `internal/handlers/admin/api_tokens.go`
4. Features needed:
   - List all tokens (truncated for security)
   - Create with scope selector
   - Revoke/deactivate
   - View last-used time
   - Set expiration dates

---

## 5. Rate Limiting - IMPLEMENTED

**Location:** `C:\Users\Admin\Documents\VS Projects\MAH\internal\ratelimit\ratelimit.go`

### Features
✓ Trust-tier aware rate limiting (5 tiers: Suspicious, Unknown, Authenticated, Whitelist, Admin)
✓ Per-IP and per-user tracking
✓ Hourly rate limit windows
✓ Dynamic limits based on user tier:
  - Suspicious (tier 0): 10 req/hour
  - Unknown (tier 1): 100 req/hour
  - Authenticated (tier 2): 5,000 req/hour
  - Whitelist (tier 3): 10,000 req/hour
  - Admin (tier 4): 50,000 req/hour

✓ Middleware support: `MiddlewareWithTrustTier(getTrustTier func)`
✓ Environment bypass: `DISABLE_RATE_LIMIT=true`
✓ Graceful cleanup and shutdown

### Current Usage in main.go
```go
// Auth endpoints: 5 attempts per 15 minutes
authLimiter := ratelimit.New(context.Background(), 5, 15*time.Minute)
r.With(authLimiter.Middleware).Post("/login", authHandler.Login)
r.With(authLimiter.Middleware).Post("/register", authHandler.Register)

// Impersonation: 3 attempts per minute
impersonateLimiter := ratelimit.New(context.Background(), 3, 1*time.Minute)
```

### Security Note
- IP detection: Uses `RemoteAddr` only (cannot be spoofed)
- Warns against trusting `X-Forwarded-For` without validated proxy
- Has function `getTrustedProxyIP()` for future proxy support

---

## 6. OpenAPI Specification - MINIMAL

**Location:** `C:\Users\Admin\Documents\VS Projects\MAH\api\openapi\`

### Exists
- **accounts.yaml** (109 lines) - Basic OpenAPI 3.0 spec
  - Covers account CRUD operations
  - Bearer auth specified
  - Proper request/response schemas
  - Basic error codes

### Missing Specs (30+ endpoints not documented)
- Hosting accounts endpoints
- File management API
- Database management API
- FTP management API
- Domain management API
- Billing/invoices API
- Metrics API
- PHP management API
- Security alerts API

### Format
```yaml
openapi: 3.0.0
info:
  title: MAH Accounts API
  version: 1.0.0
servers:
  - url: /api/v1
components:
  securitySchemes:
    BearerAuth:
      type: http
      scheme: bearer
```

### Recommendation for Phase 4C
1. Generate using Swagger tooling or manually document remaining endpoints
2. Create separate YAML files:
   - `hosting_accounts.yaml`
   - `domains.yaml`
   - `files.yaml`
   - `databases.yaml`
   - `ftp.yaml`
   - `invoices.yaml`
   - `metrics.yaml`
3. Use `openapi-generator` or Swagger UI for auto-documentation at `/docs`

---

## 7. CLI Tool - MISSING (Dedicated MAH CLI)

**Status:** NOT IMPLEMENTED

### What Exists
- Planner CLI: `C:\Users\Admin\Documents\VS Projects\MAH\apps\gh-magnolia\` (for task management)
- Service token generator: Built into main binary (`mah create-service-token`)

### What's Missing
No dedicated MAH CLI for:
- User account management
- Service lifecycle operations
- Domain operations
- Billing operations
- Admin tasks
- API token management

### Recommendation for Phase 4C
Create `C:\Users\Admin\Documents\VS Projects\MAH\cmd\mah-cli\` with:

```go
// Example structure
cmd/
  mah-cli/
    main.go
    commands/
      accounts.go  // create, list, suspend, delete
      domains.go   // register, list, dns, ssl
      services.go  // start, stop, status
      billing.go   // list-invoices, create-invoice
      admin.go     // token management, user management
      auth.go      // login, logout, set-token
```

Helpful patterns exist in CLIAIMONITOR project for agent CLI.

---

## 8. Infrastructure Dependencies

### HTTP Router
- **Library:** `github.com/go-chi/chi/v5`
- **Status:** Well-established, nested routing support
- **Usage:** Clean router pattern with middleware composition

### Authentication
- **JWT:** `internal/jwt/` with handler + middleware
- **Sessions:** `internal/auth/sessions.go`
- **Service Tokens:** `internal/auth/service_tokens.go`

### Middleware Stack
```go
r.Use(chimw.Logger)
r.Use(chimw.Recoverer)
r.Use(middleware.SecurityHeaders)
r.Use(middleware.CORS)
r.Use(middleware.CSRF)
r.Use(i18n.LocaleMiddleware)
```

### Database
- **Driver:** PostgreSQL via `github.com/jackc/pgx/v5`
- **Queries:** Generated via `internal/database/` (sqlc pattern)

### Job Queue
- **Library:** `github.com/hibiken/asynq` (Redis-backed)
- **Workers:** `internal/worker/` with scheduled tasks
- **Current tasks:** Backup scheduler (every 5 min), SSL renewal (daily 2am)

### Audit Logging
- **Location:** `internal/audit/`
- **Features:** Log rotation, HMAC integrity, configurable paths
- **Configuration:** Via env variables

---

## 9. Security Observations

### Good Practices
✓ Service tokens hashed with SHA256
✓ CSRF protection middleware
✓ Security headers middleware
✓ Audit logging with HMAC
✓ Rate limiting per trust-tier
✓ JWT expiration handling
✓ Password reset token flow

### Areas to Enhance
- API key rotation mechanism (currently no expiration UI)
- Rate limit configurations hardcoded (consider config file)
- Service token scope validation could be more granular
- API documentation for security boundaries missing

---

## 10. Phase 4C Task Alignment

### Task: "Implement phase 4C API infrastructure"

**What's ready for Phase 4C:**
1. ✓ REST API endpoints (30+ documented above)
2. ✓ JWT Bearer token authentication
3. ✓ Service token authentication (CLI only, needs UI)
4. ✓ Rate limiting framework
5. ✓ Audit logging

**What needs building:**
1. GraphQL schema + resolvers (if required)
2. WebSocket real-time events (if required)
3. Service token admin UI
4. Complete OpenAPI documentation (30+ missing specs)
5. Dedicated MAH CLI tool
6. API gateway patterns (optional)
7. API versioning strategy (currently v1 only)

---

## File Manifest

### Core API Files
- `cmd/mah/main.go` (712 lines) - Router setup, all endpoints
- `internal/api/handlers.go` (100+ lines) - Base handler patterns
- `internal/api/accounts.go` - Account API handlers
- `internal/api/billing.go` - Billing API handlers
- `internal/api/domains.go` - Domain API handlers
- `internal/api/databases.go` - Database API handlers
- `internal/api/files.go` - File API handlers
- `internal/api/ftp.go` - FTP API handlers
- `internal/api/health.go` - Health check endpoints
- `internal/api/middleware.go` - API auth middleware
- `internal/api/php.go` - PHP API handlers
- `internal/api/resource_metrics.go` - Metrics endpoints
- `internal/api/security.go` - Security API handlers

### Auth & Tokens
- `internal/auth/handlers.go` (12.5 KB) - Login/register/JWT
- `internal/auth/service_tokens.go` (1.5 KB) - Service token generation + validation
- `internal/auth/jwt_handler.go` - JWT creation
- `internal/auth/jwt_middleware.go` - JWT validation
- `internal/auth/lockout.go` - Account lockout logic
- `internal/auth/impersonation.go` - Admin impersonation

### Rate Limiting
- `internal/ratelimit/ratelimit.go` (9.9 KB) - Trust-tier rate limiter

### Documentation
- `api/openapi/accounts.yaml` (2.9 KB) - Accounts API spec

### Handlers
- `internal/handlers/` - Main request handlers (35 files)
- `internal/handlers/admin/` - Admin-specific handlers (8+ files)

---

## Recommendations Summary

### Priority 1 (Critical for Phase 4C)
1. Complete OpenAPI specs for all 30+ endpoints
2. Add service token management to admin panel
3. Implement REST endpoint documentation/swagger UI

### Priority 2 (Enhances Functionality)
1. Build dedicated `mah-cli` tool
2. Implement WebSocket for real-time status updates
3. Add GraphQL API (if cross-cutting queries needed)

### Priority 3 (Future Enhancement)
1. API gateway/rate limit at edge
2. API versioning strategy (v2 planning)
3. Webhook system for external integrations
4. API request logging/analytics dashboard

---

**Report Generated:** 2025-12-14 by Claude Code
**Next Steps:** Review findings with team, prioritize Phase 4C tasks accordingly
