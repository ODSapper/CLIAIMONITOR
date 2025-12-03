// +build ignore

package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	db, err := sql.Open("sqlite3", "../data/memory.db?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	ctx := context.Background()
	scanTime := time.Now()
	scanID := fmt.Sprintf("SCAN-%s", scanTime.Format("20060102-150405"))

	// Register environments
	environments := []struct {
		ID, Name, Desc, Type, Path string
	}{
		{"mah", "MAH - Magnolia Auto Host", "Hosting platform (WHMCS/cPanel replacement)", "internal", "C:\\Users\\Admin\\Documents\\VS Projects\\MAH"},
		{"mss", "MSS - Magnolia Secure Server", "Firewall & IPS (CSF/LFD replacement)", "internal", "C:\\Users\\Admin\\Documents\\VS Projects\\MSS"},
		{"mss-ai", "MSS-AI - Magnolia Secure Server AI", "Multi-agent AI system", "internal", "C:\\Users\\Admin\\Documents\\VS Projects\\mss-ai"},
		{"planner", "Planner - Magnolia Ecosystem Orchestrator", "Task management & coordination API", "internal", "C:\\Users\\Admin\\Documents\\VS Projects\\planner"},
		{"mss-suite", "MSS-Suite - Unified Installer", "MSS + MSS-AI integrated installer", "internal", "C:\\Users\\Admin\\Documents\\VS Projects\\mss-suite"},
	}

	for _, env := range environments {
		_, err := db.ExecContext(ctx, `
			INSERT INTO environments (id, name, description, env_type, base_path, metadata)
			VALUES (?, ?, ?, ?, ?, '{}')
			ON CONFLICT(id) DO UPDATE SET
				name = excluded.name,
				description = excluded.description,
				base_path = excluded.base_path`,
			env.ID, env.Name, env.Desc, env.Type, env.Path)
		if err != nil {
			log.Printf("Error inserting environment %s: %v", env.ID, err)
		}
	}
	fmt.Println("Registered 5 environments")

	// Record scans
	scans := []struct {
		ID, EnvID, Agent, Mission, Score string
		Files                            int
		Languages, Frameworks            []string
	}{
		{scanID + "-MAH", "mah", "Snake-MAH", "initial_recon", "C", 200, []string{"Go", "Templ", "SQL", "HTML/HTMX"}, []string{"Chi", "sqlc", "Asynq", "golang-jwt"}},
		{scanID + "-MSS", "mss", "Snake-MSS", "initial_recon", "B+", 150, []string{"Go", "YAML", "Shell"}, []string{"google/nftables", "hpcloud/tail", "bbolt"}},
		{scanID + "-MSSAI", "mss-ai", "Snake-MSSAI", "initial_recon", "C", 125, []string{"Go"}, []string{"Gorilla Mux", "BoltDB", "vLLM", "OpenTelemetry"}},
		{scanID + "-PLANNER", "planner", "Snake-Planner", "initial_recon", "D", 47, []string{"Go", "TypeScript", "SQL"}, []string{"Next.js", "Vercel", "PostgreSQL", "Redis"}},
		{scanID + "-SUITE", "mss-suite", "Snake-Suite", "initial_recon", "C", 95, []string{"Bash", "Go", "YAML"}, []string{"systemd", "docker-compose"}},
	}

	for _, s := range scans {
		langs, _ := json.Marshal(s.Languages)
		frameworks, _ := json.Marshal(s.Frameworks)
		summary, _ := json.Marshal(map[string]interface{}{
			"total_files_scanned": s.Files,
			"languages":           s.Languages,
			"frameworks":          s.Frameworks,
			"security_score":      s.Score,
		})

		_, err := db.ExecContext(ctx, `
			INSERT INTO recon_scans (id, env_id, agent_id, scan_type, mission, status, summary,
				total_files_scanned, languages_detected, frameworks_detected, security_score, completed_at)
			VALUES (?, ?, ?, 'initial', ?, 'completed', ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
			ON CONFLICT(id) DO UPDATE SET status = 'completed'`,
			s.ID, s.EnvID, s.Agent, s.Mission, string(summary), s.Files, string(langs), string(frameworks), s.Score)
		if err != nil {
			log.Printf("Error inserting scan %s: %v", s.ID, err)
		}
	}
	fmt.Println("Recorded 5 scans")

	// Insert findings
	findings := []struct {
		ID, ScanID, EnvID, Type, Severity, Title, Desc, Location, Recommendation string
	}{
		// MAH Critical
		{"MAH-CRIT-001", scanID + "-MAH", "mah", "security", "critical", "Unsafe CSP Configuration", "CSP header uses 'unsafe-inline' for scripts and styles, defeating XSS protection", "internal/middleware/security.go:17", "Remove 'unsafe-inline' and use nonces or hashes for inline scripts"},
		{"MAH-CRIT-002", scanID + "-MAH", "mah", "security", "critical", "Incomplete Whitelist Implementation", "isWhitelistedIP() always returns false - admin IP whitelist non-functional", "internal/auth/jwt_handler.go:135", "Implement whitelist checking from database or config file"},
		// MAH High
		{"MAH-HIGH-001", scanID + "-MAH", "mah", "security", "high", "CSRF Token HttpOnly Disabled", "CSRF cookie HttpOnly=false allows JavaScript access, increases XSS attack surface", "internal/middleware/csrf.go:75", "Review HTMX token patterns; consider server-side CSRF alternatives"},
		{"MAH-HIGH-002", scanID + "-MAH", "mah", "security", "high", "Injection Pattern String Matching", "Request validation uses string matching for SQL/XSS detection instead of structured escaping", "internal/middleware/request_validation.go:182-210", "Use parameterized queries as primary defense"},
		{"MAH-HIGH-003", scanID + "-MAH", "mah", "security", "high", "SESSION_SECRET Length Not Validated", "No enforcement of minimum 32-char length for session secret", "internal/auth/sessions.go:23-25", "Add length validation on startup"},
		{"MAH-HIGH-004", scanID + "-MAH", "mah", "security", "high", "Hardcoded Admin IP List", "Admin IP detection hardcoded to localhost only", "internal/auth/jwt_handler.go:114-130", "Move admin IPs to configuration"},

		// MSS High
		{"MSS-HIGH-001", scanID + "-MSS", "mss", "security", "high", "Command Injection Risk in iptables", "exec.Command with user input in ipset/iptables commands - mitigated by ValidateIPForCommand()", "pkg/firewall/iptables.go:286,325,437,575", "Ensure all exec.Command calls use positional arguments only"},
		{"MSS-HIGH-002", scanID + "-MSS", "mss", "architecture", "high", "Race Condition in Monitor.whitelist", "Whitelist map read without synchronization in trackAttempt()", "pkg/monitor/monitor.go:30-45,75-80", "Add RWMutex for dynamic updates"},
		// MSS Medium
		{"MSS-MED-001", scanID + "-MSS", "mss", "architecture", "medium", "No Persistent Block Metadata", "Temporary blocks lost on restart - kernel tables persist but metadata doesn't", "pkg/firewall/nft.go:79-84", "Add BoltDB persistence for block metadata"},
		{"MSS-MED-002", scanID + "-MSS", "mss", "security", "medium", "IPv6 Validation Incomplete", "Regex pattern may miss some valid compressed IPv6 notation", "pkg/iputils/validator.go:20,75-81", "Use net.ParseIP() for final validation"},

		// mss-ai Critical
		{"MSSAI-CRIT-001", scanID + "-MSSAI", "mss-ai", "security", "critical", "Hardcoded Default Credentials", "admin123/user123 auto-created on startup, bypass authentication", "cmd/server/main.go:48-49", "Remove hardcoded credentials, require strong password on first startup"},
		{"MSSAI-CRIT-002", scanID + "-MSSAI", "mss-ai", "security", "critical", "Insecure TLS in API Testing", "InsecureSkipVerify allows MITM attacks in agent tools", "pkg/agent/tools/api_testing.go:77", "Remove skip_tls_verify or restrict to admin-only"},
		{"MSSAI-CRIT-003", scanID + "-MSSAI", "mss-ai", "architecture", "critical", "Dry Run Unimplemented", "Approval system relies on dry run but marked TODO", "pkg/agent/executor.go:247", "Implement complete dry run mechanism for high-risk tools"},
		// mss-ai High
		{"MSSAI-HIGH-001", scanID + "-MSSAI", "mss-ai", "security", "high", "Insecure Temp File Handling", "CleanTempFilesTool no path validation - arbitrary file deletion possible", "pkg/agent/tools/maintenance.go", "Implement strict path validation and whitelist"},
		{"MSSAI-HIGH-002", scanID + "-MSSAI", "mss-ai", "security", "high", "JWT Secret Too Short", "Development config uses 31-char secret", "config/config.local.yaml:27", "Enforce minimum 32-character validation"},
		{"MSSAI-HIGH-003", scanID + "-MSSAI", "mss-ai", "security", "high", "Trust Tier Header Spoofable", "X-Trust-Tier header accepted without source IP validation", "pkg/api/rate.go:81", "Validate header only from whitelisted proxy IPs"},
		{"MSSAI-HIGH-004", scanID + "-MSSAI", "mss-ai", "security", "high", "No Pre-Auth Rate Limit", "LoginHandler applies no rate limiting before trust tier check", "pkg/api/auth_handlers.go:78-88", "Add pre-auth rate limit based on IP"},
		{"MSSAI-HIGH-005", scanID + "-MSSAI", "mss-ai", "security", "high", "Unencrypted BoltDB Storage", "Refresh tokens and approvals stored in plaintext", "pkg/storage/bolt.go", "Implement at-rest encryption for sensitive buckets"},

		// Planner Critical (excluding env file exposure per user request)
		{"PLAN-CRIT-001", scanID + "-PLANNER", "planner", "dependency", "critical", "17 NPM Vulnerabilities", "5 critical in js-yaml, form-data, hawk - DoS, code injection, prototype pollution", "web/package.json:11-26", "Run npm audit fix to patch vulnerabilities"},
		{"PLAN-CRIT-002", scanID + "-PLANNER", "planner", "security", "critical", "CORS Wildcard Policy", "Access-Control-Allow-Origin: * allows any origin", "api/index.go:122", "Implement whitelist of allowed origins"},
		{"PLAN-CRIT-003", scanID + "-PLANNER", "planner", "security", "critical", "Weak Admin Secret", "magnolia-admin-secret-2024 is predictable", "apps/mtls-api/index.go:376-385", "Use strong random secrets (min 32 bytes)"},
		// Planner High
		{"PLAN-HIGH-001", scanID + "-PLANNER", "planner", "security", "high", "SQL Limit Bypass", "LIMIT/OFFSET not capped - DoS via limit=999999", "api/index.go:278-320", "Cap limit to safe maximum (100)"},
		{"PLAN-HIGH-002", scanID + "-PLANNER", "planner", "security", "high", "Error Message Information Leak", "Database errors returned directly to client", "api/index.go:373-374", "Return generic errors, log details server-side"},
		{"PLAN-HIGH-003", scanID + "-PLANNER", "planner", "security", "high", "Missing Input Validation", "Task/Team IDs accepted without format validation", "api/index.go:190,180", "Validate IDs match pattern [A-Z0-9-]+"},
		{"PLAN-HIGH-004", scanID + "-PLANNER", "planner", "security", "high", "Rate Limit Not Distributed", "In-memory rate limiter bypassed across Vercel instances", "apps/mtls-api/index.go:256-288", "Use Redis for distributed rate limiting"},
		{"PLAN-HIGH-005", scanID + "-PLANNER", "planner", "security", "high", "Missing Auth on Reads", "All GET operations public without authentication", "api/index.go:449", "Require authentication for all operations"},

		// mss-suite Critical
		{"SUITE-CRIT-001", scanID + "-SUITE", "mss-suite", "security", "critical", "NoNewPrivileges Disabled", "MSS service allows privilege escalation if binary compromised", "systemd/mss.service:29", "Set NoNewPrivileges=true"},
		{"SUITE-CRIT-002", scanID + "-SUITE", "mss-suite", "security", "critical", "TLS Verification Disabled", "tls_skip_verify: true enables MITM attacks", "config/mss-ai-defaults.yaml:18", "Change to false in production"},
		{"SUITE-CRIT-003", scanID + "-SUITE", "mss-suite", "security", "critical", "Services Run as Root", "Both MSS and MSS-AI run as root with no isolation", "systemd/mss.service:9-10", "Create dedicated service users"},
		{"SUITE-CRIT-004", scanID + "-SUITE", "mss-suite", "security", "critical", "Symlink Attack in Temp Cleanup", "Predictable mktemp with rm -rf enables arbitrary deletion", "install/suite-install.sh:531-573", "Use trap cleanup and verify artifacts before rm"},
		// mss-suite High
		{"SUITE-HIGH-001", scanID + "-SUITE", "mss-suite", "security", "high", "No Binary Integrity Check", "Downloaded binaries have no checksum/signature verification", "install/suite-install.sh:505-522", "Add SHA256/GPG verification"},
		{"SUITE-HIGH-002", scanID + "-SUITE", "mss-suite", "security", "high", "Insecure Secret Permissions", "JWT secrets owned by root with no group sharing", "install/suite-install.sh:730,742", "Create magnolia group with chmod 640"},
		{"SUITE-HIGH-003", scanID + "-SUITE", "mss-suite", "security", "high", "No Certificate Rotation", "Self-signed certs with 365-day validity, no rotation", "install/suite-install.sh:829-848", "Integrate Let's Encrypt, add expiry monitoring"},
		{"SUITE-HIGH-004", scanID + "-SUITE", "mss-suite", "security", "high", "No Version Validation", "Version strings passed directly to git checkout", "install/suite-install.sh:108-114", "Validate semantic version format"},
	}

	inserted := 0
	for _, f := range findings {
		_, err := db.ExecContext(ctx, `
			INSERT INTO recon_findings (id, scan_id, env_id, finding_type, severity, title, description, location, recommendation, status, metadata)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'open', '{}')
			ON CONFLICT(id) DO UPDATE SET
				title = excluded.title,
				description = excluded.description,
				location = excluded.location,
				recommendation = excluded.recommendation,
				updated_at = CURRENT_TIMESTAMP`,
			f.ID, f.ScanID, f.EnvID, f.Type, f.Severity, f.Title, f.Desc, f.Location, f.Recommendation)
		if err != nil {
			log.Printf("Error inserting finding %s: %v", f.ID, err)
		} else {
			inserted++
		}
	}
	fmt.Printf("Inserted %d findings\n", inserted)

	// Update environment last_scanned
	for _, env := range environments {
		db.ExecContext(ctx, `UPDATE environments SET last_scanned = CURRENT_TIMESTAMP WHERE id = ?`, env.ID)
	}
	fmt.Println("Updated environment last_scanned timestamps")

	fmt.Println("\nReconnaissance data successfully saved to memory.db")
}
