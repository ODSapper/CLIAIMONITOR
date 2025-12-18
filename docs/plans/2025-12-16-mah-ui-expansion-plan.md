# MAH UI Expansion Plan

**Date**: 2025-12-16
**Status**: Ready for Execution
**Execution**: Parallel subagents (haiku/sonnet)

---

## Overview

This plan addresses three major areas:
1. **Tickets System** - Full implementation for support/bug reports
2. **Local Asset Bundling** - Remove CDN dependencies from MAH
3. **UI Fixes** - Fix broken buttons and missing handlers

---

## Part 1: Tickets System (Full Feature)

### Database Schema (Already Exists)
```sql
-- tickets: id, user_id, service_id, subject, status, priority, timestamps
-- ticket_replies: id, ticket_id, user_id, message, is_staff, created_at
```

### Files to Create

#### 1.1 SQL Queries
**File**: `MAH/db/queries/tickets.sql`
```sql
-- name: CreateTicket :one
-- name: GetTicket :one
-- name: ListTicketsByUser :many
-- name: ListAllTickets :many (admin)
-- name: UpdateTicketStatus :exec
-- name: CreateTicketReply :one
-- name: GetTicketReplies :many
-- name: CountOpenTickets :one
-- name: CloseTicket :exec
```

#### 1.2 Handler
**File**: `MAH/internal/handlers/tickets.go`
```go
type TicketsHandler struct {
    db      *sql.DB
    queries *database.Queries
    mss     *mss.Client  // For alert notifications
}

// User endpoints
func (h *TicketsHandler) List(w, r)           // GET /tickets
func (h *TicketsHandler) New(w, r)            // GET /tickets/new
func (h *TicketsHandler) Create(w, r)         // POST /tickets
func (h *TicketsHandler) Detail(w, r)         // GET /tickets/{id}
func (h *TicketsHandler) Reply(w, r)          // POST /tickets/{id}/reply

// Admin endpoints
func (h *TicketsHandler) AdminList(w, r)      // GET /admin/tickets
func (h *TicketsHandler) AdminDetail(w, r)    // GET /admin/tickets/{id}
func (h *TicketsHandler) AdminReply(w, r)     // POST /admin/tickets/{id}/reply
func (h *TicketsHandler) UpdateStatus(w, r)   // POST /admin/tickets/{id}/status
func (h *TicketsHandler) AssignTicket(w, r)   // POST /admin/tickets/{id}/assign
```

#### 1.3 Templates
**Files**:
- `MAH/templ/tickets.templ` - User ticket list
- `MAH/templ/ticket_new.templ` - Create ticket form
- `MAH/templ/ticket_detail.templ` - Ticket view with replies
- `MAH/templ/admin/tickets.templ` - Admin ticket management
- `MAH/templ/admin/ticket_detail.templ` - Admin ticket view

#### 1.4 Routes (add to main.go)
```go
// User ticket routes
r.Get("/tickets", ticketsHandler.List)
r.Get("/tickets/new", ticketsHandler.New)
r.Post("/tickets", ticketsHandler.Create)
r.Get("/tickets/{id}", ticketsHandler.Detail)
r.Post("/tickets/{id}/reply", ticketsHandler.Reply)

// Admin ticket routes
r.Get("/admin/tickets", ticketsHandler.AdminList)
r.Get("/admin/tickets/{id}", ticketsHandler.AdminDetail)
r.Post("/admin/tickets/{id}/reply", ticketsHandler.AdminReply)
r.Post("/admin/tickets/{id}/status", ticketsHandler.UpdateStatus)
```

#### 1.5 MSS Alert Integration
When ticket created/replied:
```go
// Notify via MSS alert system
h.mss.CreateAlert(mss.Alert{
    Type:     "ticket",
    Severity: mapPriorityToSeverity(ticket.Priority),
    Message:  fmt.Sprintf("New ticket: %s", ticket.Subject),
    Source:   "MAH",
    Metadata: map[string]string{
        "ticket_id": ticket.ID,
        "user_id":   ticket.UserID,
    },
})
```

---

## Part 2: Local Asset Bundling

### Current CDN Dependencies
| Library | Version | Size (gzip) | Action |
|---------|---------|-------------|--------|
| Tailwind CSS | runtime | N/A | Build locally |
| HTMX | 1.9.10 | ~14KB | Download |
| Alpine.js | 3.x | ~15KB | Download |

### Files to Create/Download

#### 2.1 Download JavaScript Libraries
```bash
# HTMX
curl -o static/js/htmx.min.js https://unpkg.com/htmx.org@1.9.10/dist/htmx.min.js

# Alpine.js
curl -o static/js/alpine.min.js https://unpkg.com/alpinejs@3.13.3/dist/cdn.min.js
```

#### 2.2 Build Tailwind CSS Locally
**File**: `MAH/tailwind.config.js`
```javascript
module.exports = {
  content: ["./templ/**/*.templ", "./templ/**/*.go"],
  theme: {
    extend: {
      colors: {
        'neon-cyan': '#00f0ff',
        'neon-magenta': '#ff00ff',
        'neon-green': '#00ff88',
        'bg-primary': '#0f172a',
        'bg-secondary': '#1e293b',
        'sidebar-hover': '#334155',
      }
    }
  }
}
```

**File**: `MAH/static/css/input.css`
```css
@tailwind base;
@tailwind components;
@tailwind utilities;

/* Custom theme styles from theme.css */
```

**Build command**:
```bash
npx tailwindcss -i ./static/css/input.css -o ./static/css/tailwind.min.css --minify
```

#### 2.3 Update Templates
Replace CDN references in all .templ files:

**Before**:
```html
<script src="https://cdn.tailwindcss.com"></script>
<script src="https://unpkg.com/htmx.org@1.9.10"></script>
<script src="https://unpkg.com/alpinejs@3.x.x/dist/cdn.min.js"></script>
```

**After**:
```html
<link rel="stylesheet" href="/static/css/tailwind.min.css"/>
<script src="/static/js/htmx.min.js"></script>
<script src="/static/js/alpine.min.js" defer></script>
```

**Files to Update**:
- `templ/layout.templ`
- `templ/setup.templ`
- `templ/admin/layout.templ`
- `templ/cloud/layout.templ`
- `templ/language_selector.templ`
- `templ/panel/cron.templ`
- `templ/panel/email.templ`
- `templ/panel/ftp.templ`
- `templ/panel/email_debug.templ`

---

## Part 3: UI Fixes

### 3.1 Admin Account Button Paths
**File**: `MAH/templ/admin/account_row.templ`

| Line | Current | Fix |
|------|---------|-----|
| 101 | `/admin/accounts/{id}/password` | `/admin/hosting-accounts/{id}/password` |
| 109 | `/admin/accounts/{id}` (delete) | `/admin/hosting-accounts/{id}` |

### 3.2 Account Search Handler
**File**: `MAH/internal/handlers/admin/accounts_tree.go`
```go
func (h *AccountsTreeHandler) SearchAccounts(w, r) {
    query := r.URL.Query().Get("q")
    // Search by username, email, domain
    // Return HTMX fragment with results
}
```

**Route** (main.go):
```go
r.Get("/admin/accounts/search", accountsTreeHandler.SearchAccounts)
```

### 3.3 Theme Toggle Mobile Fix
**File**: `MAH/static/css/theme.css`
```css
#theme-toggle {
    position: fixed;
    bottom: 20px;
    right: 20px;
    z-index: 50;
}

/* Mobile: move above potential bottom navs */
@media (max-width: 768px) {
    #theme-toggle {
        bottom: 80px;
    }
}
```

---

## Execution Plan

### Phase 1: Parallel Implementation (5 subagents)

| Agent | Model | Task | Files |
|-------|-------|------|-------|
| Agent 1 | Sonnet | Tickets SQL queries + handler | queries/tickets.sql, handlers/tickets.go |
| Agent 2 | Sonnet | Tickets templates (user + admin) | templ/tickets*.templ, templ/admin/tickets*.templ |
| Agent 3 | Haiku | Download JS + setup Tailwind build | static/js/*, tailwind.config.js, package.json |
| Agent 4 | Haiku | Update all templates to use local assets | templ/*.templ (CDN → local) |
| Agent 5 | Haiku | UI fixes (button paths, search, mobile) | account_row.templ, accounts_tree.go, theme.css |

### Phase 2: Integration
1. Add ticket routes to main.go
2. Run `sqlc generate` for new queries
3. Run `templ generate` for templates
4. Run `npx tailwindcss build` for CSS
5. Run `go build ./...`

### Phase 3: Rebuild & Test
1. Rebuild Linux binary
2. Restart Docker environment
3. Complete setup wizard
4. Test all ticket functionality
5. Verify all buttons work
6. Confirm no CDN requests in network tab

---

## Success Criteria

### Tickets
- [ ] User can create ticket from `/tickets/new`
- [ ] User sees their tickets at `/tickets`
- [ ] User can reply to tickets
- [ ] Admin sees all tickets at `/admin/tickets`
- [ ] Admin can reply and change status
- [ ] MSS alerts created for new tickets
- [ ] Ticket link in sidebar works (no 404)

### Local Assets
- [ ] No requests to cdn.tailwindcss.com
- [ ] No requests to unpkg.com
- [ ] No requests to jsdelivr.net
- [ ] All styling works offline
- [ ] All HTMX interactions work offline
- [ ] All Alpine.js dropdowns work offline

### UI Fixes
- [ ] Admin delete account button works
- [ ] Admin password reset button works
- [ ] Admin account search works
- [ ] Theme toggle doesn't overlap on mobile

---

## File Summary

### New Files (14)
```
db/queries/tickets.sql
internal/handlers/tickets.go
templ/tickets.templ
templ/ticket_new.templ
templ/ticket_detail.templ
templ/admin/tickets.templ
templ/admin/ticket_detail.templ
static/js/htmx.min.js
static/js/alpine.min.js
static/css/tailwind.min.css
static/css/input.css
tailwind.config.js
package.json
```

### Modified Files (12)
```
cmd/mah/main.go (add routes)
templ/layout.templ (local assets)
templ/setup.templ (local assets)
templ/admin/layout.templ (local assets)
templ/cloud/layout.templ (local assets)
templ/language_selector.templ (local assets)
templ/panel/cron.templ (local assets)
templ/panel/email.templ (local assets)
templ/panel/ftp.templ (local assets)
templ/panel/email_debug.templ (local assets)
templ/admin/account_row.templ (button paths)
internal/handlers/admin/accounts_tree.go (search)
static/css/theme.css (mobile fix)
```
