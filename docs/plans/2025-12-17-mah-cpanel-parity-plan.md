# MAH cPanel Parity Implementation Plan

**Date:** December 17, 2025
**Goal:** Full cPanel feature parity with room to expand beyond WordPress hosting
**Approach:** Hybrid (integrate existing tools where sensible, build custom when we can do better)
**Parallelism:** 8+ agents per wave

---

## Executive Summary

Based on comprehensive testing, MAH has excellent security (93/100) but only 26% feature completeness vs cPanel. This plan organizes ~50 tasks across 6 waves with maximum parallelism.

**Estimated Total Scope:** 45-55 implementation tasks
**Repos Involved:** MAH, MSS, CLIAIMONITOR, mss-suite

---

## Wave 0: Unblock Testing & Quick Wins (Day 1)

**Purpose:** Remove blockers identified in QA testing so subsequent waves can be validated.
**Parallelism:** 4 agents

| Task ID | Task | Agent | Repo | Approach |
|---------|------|-------|------|----------|
| W0-1 | Seed admin user in database migration | Agent-1 | MAH | Build |
| W0-2 | Fix static asset 404s | Agent-2 | MAH | Fix |
| W0-3 | Add HSTS header for production | Agent-3 | MAH | Fix |
| W0-4 | Set Secure flag on session cookies | Agent-4 | MAH | Fix |

**Deliverables:**
- Admin login works out-of-box
- Static assets load correctly
- Security headers production-ready

**Validation:** Run `tests/qa/test_mah.py` - all tests should pass

---

## Wave 1: Core Authentication & API Foundation (Days 2-3)

**Purpose:** Build the foundation that all other features depend on.
**Parallelism:** 6 agents
**Dependencies:** Wave 0 complete

| Task ID | Task | Agent | Repo | Approach |
|---------|------|-------|------|----------|
| W1-1 | API token generation UI + endpoints | Agent-1 | MAH | Build |
| W1-2 | API token middleware/validation | Agent-2 | MAH | Build |
| W1-3 | User roles system (admin/reseller/customer) | Agent-3 | MAH | Build |
| W1-4 | Permission middleware by role | Agent-4 | MAH | Build |
| W1-5 | User profile management page | Agent-5 | MAH | Build |
| W1-6 | Rate limiting per API token | Agent-6 | MAH/MSS | Build |

**Deliverables:**
- Users can generate/revoke API tokens
- Role-based access control working
- API rate limiting per token

---

## Wave 2: Reseller Management (Days 4-6)

**Purpose:** Core reseller functionality - the target market.
**Parallelism:** 8 agents
**Dependencies:** Wave 1 complete (roles, permissions)

| Task ID | Task | Agent | Repo | Approach |
|---------|------|-------|------|----------|
| W2-1 | Reseller creation API + UI | Agent-1 | MAH | Build |
| W2-2 | Reseller customer management (CRUD) | Agent-2 | MAH | Build |
| W2-3 | Package/plan system (adjustable quotas) | Agent-3 | MAH | Build |
| W2-4 | Resource allocation API (scalable/adjustable) | Agent-4 | MAH | Build |
| W2-5 | Reseller billing integration hooks | Agent-5 | MAH | Build |
| W2-6 | White-label/branding settings | Agent-6 | MAH | Build |
| W2-7 | Reseller dashboard/metrics | Agent-7 | MAH | Build |
| W2-8 | Customer impersonation (support tool) | Agent-8 | MAH | Build |

**Deliverables:**
- Admin can create resellers
- Resellers can create/manage customers
- Packages with adjustable disk/bandwidth/site limits
- Resource allocation API designed for auto-scaling (AWS, etc.)
- Reseller-specific branding options

---

## Wave 3A: WordPress Hosting Features (Days 7-10)

**Purpose:** WordPress-specific features - the primary use case.
**Parallelism:** 8 agents
**Dependencies:** Wave 2 complete (customer management)

| Task ID | Task | Agent | Repo | Approach |
|---------|------|-------|------|----------|
| W3A-1 | WordPress auto-installer (WP-CLI based) | Agent-1 | MAH | Build (custom) |
| W3A-2 | WordPress site listing/management UI | Agent-2 | MAH | Build |
| W3A-3 | One-click staging environment | Agent-3 | MAH | Build (custom) |
| W3A-4 | WordPress version management/updates | Agent-4 | MAH | Build |
| W3A-5 | Plugin/theme management UI | Agent-5 | MAH | Build |
| W3A-6 | WordPress backup/restore | Agent-6 | MAH | Build (custom) |
| W3A-7 | WordPress security scanner integration | Agent-7 | MAH/MSS | Integrate (MSS) |
| W3A-8 | WordPress performance monitoring | Agent-8 | MAH | Build |

**Deliverables:**
- One-click WordPress installation
- Staging environment cloning
- Automated backups
- Security scanning via MSS

---

## Wave 3B: File & Database Management (Days 7-10, parallel with 3A)

**Purpose:** Essential hosting features that work across all site types.
**Parallelism:** 6 agents
**Dependencies:** Wave 2 complete

| Task ID | Task | Agent | Repo | Approach |
|---------|------|-------|------|----------|
| W3B-1 | File Manager UI (tree view, upload, edit) | Agent-1 | MAH | Build (use FileBrowser-like approach) |
| W3B-2 | File Manager API (list, CRUD, upload) | Agent-2 | MAH | Build |
| W3B-3 | phpMyAdmin integration (SSO login) | Agent-3 | MAH/mss-suite | Integrate |
| W3B-4 | Database creation UI (MySQL/MariaDB) | Agent-4 | MAH | Build |
| W3B-5 | Database user management | Agent-5 | MAH | Build |
| W3B-6 | Database backup/export UI | Agent-6 | MAH | Build |

**Deliverables:**
- Web-based file manager with code editor
- Seamless phpMyAdmin access
- Database creation/management without CLI

---

## Wave 4: Domain & DNS Management (Days 11-13)

**Purpose:** Domain management for multi-domain hosting. Most users will use Cloudflare or external DNS.
**Parallelism:** 5 agents
**Dependencies:** Wave 3A complete

| Task ID | Task | Agent | Repo | Approach |
|---------|------|-------|------|----------|
| W4-1 | Domain addition/management API | Agent-1 | MAH | Build |
| W4-2 | Domain management UI | Agent-2 | MAH | Build |
| W4-3 | Local DNS zone file generation | Agent-3 | MAH | Build |
| W4-4 | Subdomain management | Agent-4 | MAH | Build |
| W4-5 | Cloudflare API integration (optional) | Agent-5 | MAH | Integrate |

**Deliverables:**
- Add/remove domains to hosting accounts
- Local zone files for internal DNS
- Optional Cloudflare sync for those who want it
- Subdomain management

---

## Wave 5A: SSL & Security Features (Days 14-16)

**Purpose:** SSL certificate management and enhanced security.
**Parallelism:** 6 agents
**Dependencies:** Wave 4 complete (domain management)

| Task ID | Task | Agent | Repo | Approach |
|---------|------|-------|------|----------|
| W5A-1 | Let's Encrypt auto-provisioning | Agent-1 | MAH | Integrate (certbot/acme.sh) |
| W5A-2 | SSL certificate management UI | Agent-2 | MAH | Build |
| W5A-3 | Custom SSL upload support | Agent-3 | MAH | Build |
| W5A-4 | SSL auto-renewal system | Agent-4 | MAH | Build |
| W5A-5 | IP blocking/firewall rules UI | Agent-5 | MAH/MSS | Integrate (MSS) |
| W5A-6 | 2FA for user accounts | Agent-6 | MAH | Build |

**Deliverables:**
- One-click Let's Encrypt SSL
- SSL certificate dashboard
- Firewall integration with MSS

---

## Wave 5B: Backup & Monitoring (Days 14-16, parallel with 5A)

**Purpose:** Data protection and visibility with flexible storage backends.
**Parallelism:** 6 agents
**Dependencies:** Wave 4 complete

| Task ID | Task | Agent | Repo | Approach |
|---------|------|-------|------|----------|
| W5B-1 | Full account backup system (local) | Agent-1 | MAH | Build |
| W5B-2 | Backup scheduling UI | Agent-2 | MAH | Build |
| W5B-3 | One-click restore | Agent-3 | MAH | Build |
| W5B-4 | Offsite backup providers (S3, B2, etc.) | Agent-4 | MAH | Build |
| W5B-5 | Resource usage dashboard | Agent-5 | MAH | Build |
| W5B-6 | Error log viewer | Agent-6 | MAH | Build |

**Deliverables:**
- Scheduled backups with retention (local storage)
- Offsite backup to S3, Backblaze B2, or compatible providers
- One-click full restore from any source
- Resource monitoring dashboard

---

## Wave 6: UX Polish & Production Hardening (Days 17-19)

**Purpose:** Professional user experience and production readiness.
**Parallelism:** 8 agents
**Dependencies:** All prior waves complete

| Task ID | Task | Agent | Repo | Approach |
|---------|------|-------|------|----------|
| W6-1 | Inline form validation (all forms) | Agent-1 | MAH | Build |
| W6-2 | Contextual help system | Agent-2 | MAH | Build |
| W6-3 | Search/command palette | Agent-3 | MAH | Build |
| W6-4 | Improved error messages | Agent-4 | MAH | Build |
| W6-5 | Onboarding flow for new users | Agent-5 | MAH | Build |
| W6-6 | Remove unsafe-inline from CSP | Agent-6 | MAH | Fix |
| W6-7 | Hide server version headers | Agent-7 | MAH/mss-suite | Fix |
| W6-8 | Audit logging system | Agent-8 | MAH | Build |

**Deliverables:**
- Professional UX matching/exceeding cPanel
- Security hardening complete
- Audit trail for compliance

---

## Dependency Graph

```
Wave 0 (Blockers)
    │
    ▼
Wave 1 (Auth/API Foundation)
    │
    ▼
Wave 2 (Reseller Management)
    │
    ├─────────────────────────────┐
    ▼                             ▼
Wave 3A (WordPress)    Wave 3B (File/DB)
    │                             │
    └──────────┬──────────────────┘
               ▼
         Wave 4 (Domain/DNS)
               │
    ┌──────────┴──────────┐
    ▼                     ▼
Wave 5A (SSL/Security)  Wave 5B (Backup/Monitor)
    │                     │
    └──────────┬──────────┘
               ▼
      Wave 6 (UX/Production)
```

---

## Integration Points (Existing Tools)

| Feature | Existing Tool | Integration Approach |
|---------|---------------|---------------------|
| Database Admin | phpMyAdmin | SSO via MAH session → phpMyAdmin |
| SSL Certs | Let's Encrypt/certbot | certbot wrapper with hooks |
| Security | MSS | API integration for IP blocking, scanning |
| DNS Sync | Cloudflare API | Optional zone sync for Cloudflare users |
| Offsite Backup | S3/B2 APIs | Pluggable storage backend interface |

---

## Custom Build Rationale

| Feature | Why Custom |
|---------|------------|
| WordPress Installer | WP-CLI is powerful; custom UI gives us control over UX |
| Staging Environment | Unique differentiator; no good off-shelf solution |
| File Manager | Web-based editors done well are a UX advantage |
| Reseller System | Core business logic; needs tight MAH integration |
| Backup System | Hosting-specific; needs database + files + configs |

---

## Resource Estimates

| Wave | Tasks | Agents | Duration |
|------|-------|--------|----------|
| 0 | 4 | 4 | 1 day |
| 1 | 6 | 6 | 2 days |
| 2 | 8 | 8 | 3 days |
| 3A | 8 | 8 | 4 days |
| 3B | 6 | 6 | 4 days (parallel with 3A) |
| 4 | 5 | 5 | 3 days |
| 5A | 6 | 6 | 3 days |
| 5B | 6 | 6 | 3 days (parallel with 5A) |
| 6 | 8 | 8 | 3 days |

**Total Tasks:** ~57
**Total Calendar Days:** ~19 days (with parallel waves)

---

## Success Criteria

1. **All QA tests pass** - Run `tests/qa/test_mah.py` with 100% pass rate
2. **Reseller workflow complete** - Admin → Reseller → Customer → WordPress site
3. **Feature parity checklist:**
   - [ ] File Manager functional
   - [ ] Database management functional
   - [ ] WordPress one-click install
   - [ ] SSL auto-provisioning (Let's Encrypt)
   - [ ] Backup/restore functional (local + offsite)
   - [ ] Domain/subdomain management
   - [ ] Reseller branding/white-label
   - [ ] Resource allocation adjustable
4. **Security score 95+** on re-assessment
5. **UX score 85+** on re-assessment

---

## Next Steps

1. **Approve this plan** or request adjustments
2. **Start Wave 0** with 4 agents on blockers
3. **Set up task tracking** in Planner API for all tasks
4. **Run QA validation** after each wave before proceeding

---

## Design Decisions (Resolved)

| Question | Decision |
|----------|----------|
| Email hosting? | **Dropped** - Most users have SMTP or use WordPress plugins |
| Backup storage? | **Local + Offsite** - S3/B2 as optional providers |
| Domain management? | **Zone management only** - Assume Cloudflare/external DNS |
| Resource allocation? | **Adjustable** - API designed for auto-scaling (AWS, etc.) |

## Open Questions

1. What WordPress themes/plugins should be pre-bundled for one-click installs?
2. Should staging environments be on same server or separate containers?
3. What metrics should feed into auto-scaling decisions? (CPU, memory, requests, disk?)
