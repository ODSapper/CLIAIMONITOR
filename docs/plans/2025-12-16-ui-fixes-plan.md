# MAH UI Fixes Plan

**Date**: 2025-12-16
**Status**: Ready for Execution
**Execution**: Parallel subagents (haiku/sonnet)

---

## UI Architecture Overview

- **Template Engine**: templ (Go template system)
- **Styling**: Tailwind CSS (CDN)
- **Interactivity**: HTMX 1.9.10 (CDN)
- **Router**: Chi v5

---

## Issues to Fix

### Issue 1: Missing /tickets Route (MEDIUM)
**Location**: `MAH/templ/layout.templ` line 336

**Problem**: The sidebar navigation has a link to `/tickets`:
```html
<a href="/tickets" class="...">Tickets</a>
```
But there is no handler, template, or route defined for tickets. This results in a 404 error.

**Fix Options**:
1. **Option A (Recommended)**: Remove the tickets link from sidebar until feature is implemented
2. **Option B**: Create a placeholder tickets page with "Coming Soon" message
3. **Option C**: Implement full tickets system (too large for this sprint)

**Files to Modify**:
- `MAH/templ/layout.templ` - Remove or comment out tickets link

**Impact**: 404 error when clicking Tickets in sidebar
**Effort**: Low (remove link)
**Assigned**: Haiku subagent

---

### Issue 2: CDN Dependency Risk (LOW)
**Location**: `MAH/templ/layout.templ` (head section)

**Problem**: HTMX and Tailwind are loaded from CDN. If CDN fails:
- HTMX: All dynamic buttons/forms stop working
- Tailwind: All styling breaks (though fallback CSS exists in theme.css)

**Current**:
```html
<script src="https://unpkg.com/htmx.org@1.9.10"></script>
<script src="https://cdn.tailwindcss.com"></script>
```

**Fix**: Add local fallback copies:
1. Download htmx.min.js to `static/js/htmx.min.js`
2. Build Tailwind CSS to `static/css/tailwind.css` (or use CDN with fallback)

**Files to Modify**:
- Download HTMX to `MAH/static/js/htmx.min.js`
- Update `MAH/templ/layout.templ` to use local files or add fallback

**Impact**: UI breaks if CDN unavailable
**Effort**: Low-Medium
**Assigned**: Haiku subagent

---

### Issue 3: Setup Flow Completion (INFO)
**Location**: `MAH/internal/handlers/setup.go`

**Status**: Not a bug - this is intended behavior.

**How it works**:
1. All routes check `system_settings.setup_complete = 'true'`
2. If not set, redirects to `/setup`
3. Setup form creates admin user and sets flag
4. After setup, login works normally

**No fix needed** - users must complete setup wizard on first run.

---

### Issue 4: Theme Toggle Button Mobile Overlap (LOW)
**Location**: `MAH/static/css/theme.css`

**Problem**: Theme toggle button has `position: fixed; bottom: 20px; right: 20px` which may overlap content on mobile devices or small screens.

**Fix**: Add responsive positioning or move to header/sidebar.

**Files to Modify**:
- `MAH/static/css/theme.css`
- Optionally `MAH/templ/layout.templ`

**Impact**: Minor UX issue on mobile
**Effort**: Low
**Assigned**: Haiku subagent

---

### Issue 5: Admin Accounts Delete Button Path Mismatch (MEDIUM)
**Location**: `MAH/templ/admin/account_row.templ` line 109

**Problem**: Delete button uses `/admin/accounts/{id}` but route is defined as `/admin/hosting-accounts/{id}`:
```go
// Template uses:
hx-delete={ "/admin/accounts/" + fmt.Sprint(accountID) }

// Route defined as:
r.Delete("/admin/hosting-accounts/{id}", hostingAccountsAdminHandler.TerminateAccount)
```

**Fix**: Update template to use correct path `/admin/hosting-accounts/{id}`

**Files to Modify**:
- `MAH/templ/admin/account_row.templ`

**Impact**: Delete account button returns 404
**Effort**: Low
**Assigned**: Haiku subagent

---

### Issue 6: Admin Password Reset Button Path Mismatch (MEDIUM)
**Location**: `MAH/templ/admin/account_row.templ` line 101

**Problem**: Password reset button uses `/admin/accounts/{id}/password` but route is `/admin/hosting-accounts/{id}/password`:
```go
// Template uses:
hx-post={ "/admin/accounts/" + fmt.Sprint(accountID) + "/password" }

// Route defined as:
r.Post("/admin/hosting-accounts/{id}/password", hostingAccountsAdminHandler.ResetPassword)
```

**Fix**: Update template to use correct path

**Files to Modify**:
- `MAH/templ/admin/account_row.templ`

**Impact**: Password reset button returns 404
**Effort**: Low
**Assigned**: Haiku subagent

---

### Issue 7: Admin Account Search Path (MEDIUM)
**Location**: `MAH/templ/admin/accounts_tree.templ` line 60

**Problem**: Search input uses `/admin/accounts/search` but this route doesn't exist:
```html
hx-get="/admin/accounts/search"
```

Routes defined in main.go:
- `/admin/accounts/tree` ✓
- `/admin/accounts/server/{id}` ✓
- `/admin/accounts/reseller/{id}` ✓
- `/admin/accounts/search` ✗ MISSING

**Fix Options**:
1. Add search route handler
2. Remove search functionality from UI

**Files to Modify**:
- `MAH/internal/handlers/admin/accounts_tree.go` - Add SearchAccounts handler
- `MAH/cmd/mah/main.go` - Add route

**Impact**: Account search doesn't work
**Effort**: Medium
**Assigned**: Sonnet subagent

---

## Execution Plan

### Phase 1: Parallel Fixes (4 subagents)

| Agent | Model | Tasks |
|-------|-------|-------|
| Agent 1 | Haiku | Fix Issues 1, 4: Remove tickets link, fix mobile theme button |
| Agent 2 | Haiku | Fix Issues 5, 6: Fix admin account button paths |
| Agent 3 | Haiku | Fix Issue 2: Add local HTMX fallback |
| Agent 4 | Sonnet | Fix Issue 7: Add account search handler |

### Phase 2: Regenerate Templates
After template fixes:
```bash
cd MAH && templ generate
```

### Phase 3: Rebuild & Verify
1. Run `go build ./...`
2. Rebuild Linux binary
3. Restart Docker environment
4. Test all fixed UI elements

---

## Files to Modify

### Templates (require `templ generate` after)
- `templ/layout.templ` - Remove tickets link
- `templ/admin/account_row.templ` - Fix button paths

### Static Assets
- `static/js/htmx.min.js` - Add local copy
- `static/css/theme.css` - Fix mobile positioning

### Go Code
- `internal/handlers/admin/accounts_tree.go` - Add search handler
- `cmd/mah/main.go` - Add search route

---

## Success Criteria

1. No 404 errors when clicking sidebar links
2. Admin account buttons (delete, password reset) work
3. Account search works or is removed
4. HTMX works even if CDN fails
5. Theme toggle doesn't overlap content on mobile
6. All templ files regenerate without errors
7. Go build succeeds
