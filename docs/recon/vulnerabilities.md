# Security Vulnerabilities

**Last Updated**: 2025-12-02
**Total Findings**: 31

This file contains security vulnerabilities and weaknesses discovered by Snake reconnaissance agents.

## About Security Findings

Security findings include:
- OWASP Top 10 vulnerabilities
- Authentication/authorization issues
- Input validation problems
- SQL injection risks
- XSS vulnerabilities
- CSRF weaknesses
- Insecure configurations
- Exposed credentials
- Cryptographic issues
- Access control flaws

## Critical Severity (10)

### MAH-CRIT-001: Unsafe CSP Configuration
- **Environment**: MAH (Magnolia Auto Host)
- **Location**: `internal/middleware/security.go:17`
- **Description**: CSP header uses 'unsafe-inline' for scripts and styles, defeating XSS protection
- **Recommendation**: Remove 'unsafe-inline' and use nonces or hashes for inline scripts

### MAH-CRIT-002: Incomplete Whitelist Implementation
- **Environment**: MAH (Magnolia Auto Host)
- **Location**: `internal/auth/jwt_handler.go:135`
- **Description**: isWhitelistedIP() always returns false - admin IP whitelist non-functional
- **Recommendation**: Implement whitelist checking from database or config file

### MSSAI-CRIT-001: Hardcoded Default Credentials
- **Environment**: MSS-AI (Magnolia Secure Server AI)
- **Location**: `cmd/server/main.go:48-49`
- **Description**: admin123/user123 auto-created on startup, bypass authentication
- **Recommendation**: Remove hardcoded credentials, require strong password on first startup

### MSSAI-CRIT-002: Insecure TLS in API Testing
- **Environment**: MSS-AI (Magnolia Secure Server AI)
- **Location**: `pkg/agent/tools/api_testing.go:77`
- **Description**: InsecureSkipVerify allows MITM attacks in agent tools
- **Recommendation**: Remove skip_tls_verify or restrict to admin-only

### PLAN-CRIT-002: CORS Wildcard Policy
- **Environment**: Planner (Magnolia Ecosystem Orchestrator)
- **Location**: `api/index.go:122`
- **Description**: Access-Control-Allow-Origin: * allows any origin
- **Recommendation**: Implement whitelist of allowed origins

### PLAN-CRIT-003: Weak Admin Secret
- **Environment**: Planner (Magnolia Ecosystem Orchestrator)
- **Location**: `apps/mtls-api/index.go:376-385`
- **Description**: magnolia-admin-secret-2024 is predictable
- **Recommendation**: Use strong random secrets (min 32 bytes)

### SUITE-CRIT-001: NoNewPrivileges Disabled
- **Environment**: MSS-Suite (Unified Installer)
- **Location**: `systemd/mss.service:29`
- **Description**: MSS service allows privilege escalation if binary compromised
- **Recommendation**: Set NoNewPrivileges=true

### SUITE-CRIT-002: TLS Verification Disabled
- **Environment**: MSS-Suite (Unified Installer)
- **Location**: `config/mss-ai-defaults.yaml:18`
- **Description**: tls_skip_verify: true enables MITM attacks
- **Recommendation**: Change to false in production

### SUITE-CRIT-003: Services Run as Root
- **Environment**: MSS-Suite (Unified Installer)
- **Location**: `systemd/mss.service:9-10`
- **Description**: Both MSS and MSS-AI run as root with no isolation
- **Recommendation**: Create dedicated service users

### SUITE-CRIT-004: Symlink Attack in Temp Cleanup
- **Environment**: MSS-Suite (Unified Installer)
- **Location**: `install/suite-install.sh:531-573`
- **Description**: Predictable mktemp with rm -rf enables arbitrary deletion
- **Recommendation**: Use trap cleanup and verify artifacts before rm

## High Severity (17)

### MAH-HIGH-001: CSRF Token HttpOnly Disabled
- **Environment**: MAH (Magnolia Auto Host)
- **Location**: `internal/middleware/csrf.go:75`
- **Description**: CSRF cookie HttpOnly=false allows JavaScript access, increases XSS attack surface
- **Recommendation**: Review HTMX token patterns; consider server-side CSRF alternatives

### MAH-HIGH-002: Injection Pattern String Matching
- **Environment**: MAH (Magnolia Auto Host)
- **Location**: `internal/middleware/request_validation.go:182-210`
- **Description**: Request validation uses string matching for SQL/XSS detection instead of structured escaping
- **Recommendation**: Use parameterized queries as primary defense

### MAH-HIGH-003: SESSION_SECRET Length Not Validated
- **Environment**: MAH (Magnolia Auto Host)
- **Location**: `internal/auth/sessions.go:23-25`
- **Description**: No enforcement of minimum 32-char length for session secret
- **Recommendation**: Add length validation on startup

### MAH-HIGH-004: Hardcoded Admin IP List
- **Environment**: MAH (Magnolia Auto Host)
- **Location**: `internal/auth/jwt_handler.go:114-130`
- **Description**: Admin IP detection hardcoded to localhost only
- **Recommendation**: Move admin IPs to configuration

### MSS-HIGH-001: Command Injection Risk in iptables
- **Environment**: MSS (Magnolia Secure Server)
- **Location**: `pkg/firewall/iptables.go:286,325,437,575`
- **Description**: exec.Command with user input in ipset/iptables commands - mitigated by ValidateIPForCommand()
- **Recommendation**: Ensure all exec.Command calls use positional arguments only

### MSSAI-HIGH-001: Insecure Temp File Handling
- **Environment**: MSS-AI (Magnolia Secure Server AI)
- **Location**: `pkg/agent/tools/maintenance.go`
- **Description**: CleanTempFilesTool no path validation - arbitrary file deletion possible
- **Recommendation**: Implement strict path validation and whitelist

### MSSAI-HIGH-002: JWT Secret Too Short
- **Environment**: MSS-AI (Magnolia Secure Server AI)
- **Location**: `config/config.local.yaml:27`
- **Description**: Development config uses 31-char secret
- **Recommendation**: Enforce minimum 32-character validation

### MSSAI-HIGH-003: Trust Tier Header Spoofable
- **Environment**: MSS-AI (Magnolia Secure Server AI)
- **Location**: `pkg/api/rate.go:81`
- **Description**: X-Trust-Tier header accepted without source IP validation
- **Recommendation**: Validate header only from whitelisted proxy IPs

### MSSAI-HIGH-004: No Pre-Auth Rate Limit
- **Environment**: MSS-AI (Magnolia Secure Server AI)
- **Location**: `pkg/api/auth_handlers.go:78-88`
- **Description**: LoginHandler applies no rate limiting before trust tier check
- **Recommendation**: Add pre-auth rate limit based on IP

### MSSAI-HIGH-005: Unencrypted BoltDB Storage
- **Environment**: MSS-AI (Magnolia Secure Server AI)
- **Location**: `pkg/storage/bolt.go`
- **Description**: Refresh tokens and approvals stored in plaintext
- **Recommendation**: Implement at-rest encryption for sensitive buckets

### PLAN-HIGH-001: SQL Limit Bypass
- **Environment**: Planner (Magnolia Ecosystem Orchestrator)
- **Location**: `api/index.go:278-320`
- **Description**: LIMIT/OFFSET not capped - DoS via limit=999999
- **Recommendation**: Cap limit to safe maximum (100)

### PLAN-HIGH-002: Error Message Information Leak
- **Environment**: Planner (Magnolia Ecosystem Orchestrator)
- **Location**: `api/index.go:373-374`
- **Description**: Database errors returned directly to client
- **Recommendation**: Return generic errors, log details server-side

### PLAN-HIGH-003: Missing Input Validation
- **Environment**: Planner (Magnolia Ecosystem Orchestrator)
- **Location**: `api/index.go:190,180`
- **Description**: Task/Team IDs accepted without format validation
- **Recommendation**: Validate IDs match pattern [A-Z0-9-]+

### PLAN-HIGH-004: Rate Limit Not Distributed
- **Environment**: Planner (Magnolia Ecosystem Orchestrator)
- **Location**: `apps/mtls-api/index.go:256-288`
- **Description**: In-memory rate limiter bypassed across Vercel instances
- **Recommendation**: Use Redis for distributed rate limiting

### PLAN-HIGH-005: Missing Auth on Reads
- **Environment**: Planner (Magnolia Ecosystem Orchestrator)
- **Location**: `api/index.go:449`
- **Description**: All GET operations public without authentication
- **Recommendation**: Require authentication for all operations

### SUITE-HIGH-001: No Binary Integrity Check
- **Environment**: MSS-Suite (Unified Installer)
- **Location**: `install/suite-install.sh:505-522`
- **Description**: Downloaded binaries have no checksum/signature verification
- **Recommendation**: Add SHA256/GPG verification

### SUITE-HIGH-002: Insecure Secret Permissions
- **Environment**: MSS-Suite (Unified Installer)
- **Location**: `install/suite-install.sh:730,742`
- **Description**: JWT secrets owned by root with no group sharing
- **Recommendation**: Create magnolia group with chmod 640

### SUITE-HIGH-003: No Certificate Rotation
- **Environment**: MSS-Suite (Unified Installer)
- **Location**: `install/suite-install.sh:829-848`
- **Description**: Self-signed certs with 365-day validity, no rotation
- **Recommendation**: Integrate Let's Encrypt, add expiry monitoring

### SUITE-HIGH-004: No Version Validation
- **Environment**: MSS-Suite (Unified Installer)
- **Location**: `install/suite-install.sh:108-114`
- **Description**: Version strings passed directly to git checkout
- **Recommendation**: Validate semantic version format

## Medium Severity (2)

### MSS-MED-002: IPv6 Validation Incomplete
- **Environment**: MSS (Magnolia Secure Server)
- **Location**: `pkg/iputils/validator.go:20,75-81`
- **Description**: Regex pattern may miss some valid compressed IPv6 notation
- **Recommendation**: Use net.ParseIP() for final validation

## Low Severity (0)

No low severity security vulnerabilities at this time.

---

## Summary by Environment

| Environment | Critical | High | Medium | Low | Total |
|-------------|----------|------|--------|-----|-------|
| MAH | 2 | 4 | 0 | 0 | 6 |
| MSS | 0 | 1 | 1 | 0 | 2 |
| MSS-AI | 2 | 5 | 0 | 0 | 7 |
| Planner | 2 | 5 | 0 | 0 | 7 |
| MSS-Suite | 4 | 4 | 0 | 0 | 8 |
| **Total** | **10** | **19** | **1** | **0** | **30** |

---

**Note**: This file is automatically generated and updated by the Snake Agent Force recon system. Manual edits may be overwritten. To modify findings, use the recon management tools or update the database directly.
