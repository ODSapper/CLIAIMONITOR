# Multi-Agent Reseller/WordPress Integration Test Plan

**Date:** 2025-12-17
**Environment:** Docker (mss-suite)
**Endpoints:** MSS: http://localhost:8090, MAH: http://localhost:8080

---

## Test Objectives

1. Validate full reseller workflow (package creation, account provisioning)
2. Test end-user WordPress installation and functionality
3. Active security probing during setup (IDOR, auth bypass, injection)
4. Comprehensive CTF-style security testing after setup

---

## Agent Architecture

```
                    ┌─────────────────────┐
                    │     CAPTAIN         │
                    │  (Orchestrator)     │
                    └─────────┬───────────┘
                              │
        ┌─────────────────────┼─────────────────────┐
        │                     │                     │
        ▼                     ▼                     ▼
┌───────────────┐    ┌───────────────┐    ┌───────────────┐
│ RESELLER      │    │ SECURITY      │    │ CTF SECURITY  │
│ AGENT         │    │ MONITOR       │    │ AGENT         │
│ (Phase 1)     │    │ (Phase 1-2)   │    │ (Phase 3)     │
└───────┬───────┘    └───────────────┘    └───────────────┘
        │
        ├──────────────┬──────────────┐
        ▼              ▼              ▼
┌───────────────┐ ┌───────────────┐
│ END-USER 1    │ │ END-USER 2    │
│ AGENT         │ │ AGENT         │
│ (Phase 2)     │ │ (Phase 2)     │
└───────────────┘ └───────────────┘
```

---

## Phase 1: Environment Setup (Reseller + Security Monitor)

### Reseller Agent Tasks
**Runs:** First
**Model:** Sonnet

1. **Admin Authentication**
   - Login to MAH as admin
   - Obtain session token
   - Verify admin privileges

2. **Create Reseller Profile**
   ```json
   {
     "name": "TestReseller Inc",
     "contact_email": "reseller@test.local",
     "status": "active"
   }
   ```

3. **Create 2 WP Hosting Packages**
   - **Package 1: WP-Basic**
     ```json
     {
       "name": "WP-Basic",
       "slug": "wp-basic",
       "disk_quota_mb": 1024,
       "bandwidth_quota_mb": 10240,
       "email_accounts": 5,
       "databases": 1,
       "monthly_price": 9.99
     }
     ```
   - **Package 2: WP-Pro**
     ```json
     {
       "name": "WP-Pro",
       "slug": "wp-pro",
       "disk_quota_mb": 5120,
       "bandwidth_quota_mb": 51200,
       "email_accounts": 25,
       "databases": 5,
       "monthly_price": 24.99
     }
     ```

4. **Create 2 End-User Accounts**
   - **User 1: wpuser1**
     - Username: `wpuser1`
     - Domain: `site1.test.local`
     - Package: WP-Basic
   - **User 2: wpuser2**
     - Username: `wpuser2`
     - Domain: `site2.test.local`
     - Package: WP-Pro

5. **Mock Billing Events**
   - Create invoices for both accounts
   - Mark as "paid" (simulated)
   - Verify billing records created

6. **Output:** Credentials file with user details

### Security Monitor Agent Tasks
**Runs:** Parallel with Reseller Agent
**Model:** Sonnet

1. **Monitor Authentication Flows**
   - Watch for session token patterns
   - Check for token reuse vulnerabilities
   - Verify CSRF protection active

2. **Test IDOR Vulnerabilities**
   - Attempt to access `/api/reseller/{other_id}/accounts`
   - Try modifying account IDs in requests
   - Test package ID manipulation

3. **Test Injection Attempts**
   - SQL injection in username/domain fields
   - XSS in package names
   - Command injection in domain names

4. **Test Authorization Bypass**
   - Access admin endpoints without auth
   - Use expired/invalid tokens
   - Try privilege escalation

5. **Output:** Security findings report (issues found during setup)

---

## Phase 2: End-User WordPress Setup (Parallel)

### End-User Agent 1 Tasks
**Runs:** After Phase 1 completes
**Model:** Sonnet
**Credentials:** wpuser1 / site1.test.local

1. **User Authentication**
   - Login as wpuser1
   - Verify can only see own resources
   - Check dashboard access

2. **WordPress Installation**
   ```json
   {
     "domain": "site1.test.local",
     "path": "/var/www/site1",
     "title": "Test Site 1",
     "admin_user": "wpadmin1",
     "admin_email": "admin@site1.test.local",
     "admin_pass": "SecureWP1Pass!"
   }
   ```

3. **Full Functional Testing**
   - Login to WP admin
   - Create a test blog post
   - Upload a test image
   - Install and activate a plugin (Akismet or similar)
   - Change theme
   - Test frontend loads correctly

4. **Output:** WP Site 1 functional test report

### End-User Agent 2 Tasks
**Runs:** Parallel with End-User Agent 1
**Model:** Sonnet
**Credentials:** wpuser2 / site2.test.local

1. **User Authentication**
   - Login as wpuser2
   - Verify isolated from wpuser1 data

2. **WordPress Installation**
   ```json
   {
     "domain": "site2.test.local",
     "path": "/var/www/site2",
     "title": "Test Site 2",
     "admin_user": "wpadmin2",
     "admin_email": "admin@site2.test.local",
     "admin_pass": "SecureWP2Pass!"
   }
   ```

3. **Full Functional Testing**
   - Same tests as End-User Agent 1
   - Different content to verify isolation

4. **Output:** WP Site 2 functional test report

---

## Phase 3: CTF Security Testing

### CTF Security Agent Tasks
**Runs:** After Phase 2 completes
**Model:** Sonnet

1. **WordPress Security Scan**
   - Scan both WP installations for vulnerabilities
   - Check wp-config.php permissions
   - Verify debug mode disabled
   - Check file permissions
   - Look for default credentials

2. **Cross-Site Isolation Tests**
   - Verify Site 1 cannot access Site 2 files
   - Test database isolation
   - Check for shared session vulnerabilities

3. **MSS Firewall Testing**
   - Test blocked IP functionality
   - Verify rate limiting
   - Test security headers on all endpoints
   - Check CORS policies

4. **MAH API Attack Scenarios**
   - Full OWASP Top 10 testing
   - Broken authentication
   - Broken access control
   - Injection attacks
   - Security misconfiguration
   - SSRF attempts

5. **Privilege Escalation Tests**
   - User 1 trying to access User 2 resources
   - User trying to access reseller resources
   - Reseller trying to access admin resources

6. **Output:** Comprehensive CTF security report

---

## Execution Timeline

```
Time    │ Reseller Agent │ Security Monitor │ End-User 1 │ End-User 2 │ CTF Agent
────────┼────────────────┼──────────────────┼────────────┼────────────┼───────────
T+0     │ START          │ START            │            │            │
T+1     │ Auth + Setup   │ Monitor Auth     │            │            │
T+2     │ Create Pkgs    │ Test IDOR        │            │            │
T+3     │ Create Users   │ Test Injection   │            │            │
T+4     │ Mock Billing   │ Test AuthZ       │            │            │
T+5     │ COMPLETE       │ REPORT           │ START      │ START      │
T+6     │                │                  │ Auth       │ Auth       │
T+7     │                │                  │ Install WP │ Install WP │
T+8     │                │                  │ Test WP    │ Test WP    │
T+9     │                │                  │ COMPLETE   │ COMPLETE   │ START
T+10    │                │                  │            │            │ WP Scan
T+11    │                │                  │            │            │ Isolation
T+12    │                │                  │            │            │ MSS Test
T+13    │                │                  │            │            │ MAH Attack
T+14    │                │                  │            │            │ Priv Esc
T+15    │                │                  │            │            │ COMPLETE
```

---

## Success Criteria

### Functional
- [ ] Reseller can create packages and accounts
- [ ] Users can login and only see own resources
- [ ] WordPress installs successfully for both users
- [ ] WP admin functions work (posts, plugins, themes)
- [ ] Billing records created correctly

### Security
- [ ] No IDOR vulnerabilities found
- [ ] No SQL injection possible
- [ ] CSRF protection working
- [ ] User isolation enforced
- [ ] Rate limiting functional
- [ ] Security headers present

---

## Environment Details

| Service | URL | Auth |
|---------|-----|------|
| MSS | http://localhost:8090 | Basic Auth: admin/TestAdmin123! |
| MAH | http://localhost:8080 | Session-based |
| PostgreSQL | localhost:58046 | mah/mah_password |
| Redis | localhost:58047 | changeme |

---

## Notes

- LOCAL_PROVISION_MOCK=true means WordPress installation will be simulated
- Security testing should use non-destructive techniques
- All findings should be documented with reproduction steps
