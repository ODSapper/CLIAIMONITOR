# Recon Memory System - Quick Start Guide

## For Snake Agents

When you complete a reconnaissance scan:

```go
// 1. Report your scan
scan := &memory.ReconScan{
    ID:       "SCAN-20251202-001",
    EnvID:    "target-env",
    AgentID:  "Snake001",
    ScanType: "initial",
    Status:   "running",
}
db.RecordScan(ctx, scan)

// 2. Submit findings as you discover them
finding := &memory.ReconFinding{
    ID:             "VULN-001",
    ScanID:         "SCAN-20251202-001",
    EnvID:          "target-env",
    FindingType:    "security",      // security, architecture, dependency, process, performance
    Severity:       "critical",      // critical, high, medium, low, info
    Title:          "SQL Injection in Login",
    Description:    "Detailed explanation of the vulnerability...",
    Location:       "src/auth/login.go:45",
    Recommendation: "Use prepared statements with parameterized queries",
    Status:         "open",
}
db.SaveFinding(ctx, finding)

// 3. Complete scan with summary
summary := &memory.ScanSummary{
    TotalFiles:    342,
    Languages:     []string{"go", "python", "javascript"},
    Frameworks:    []string{"chi", "flask", "react"},
    SecurityScore: "B",
    CriticalCount: 2,
    HighCount:     5,
    MediumCount:   10,
    LowCount:      3,
}
db.CompleteScan(ctx, "SCAN-20251202-001", summary)

// 4. Sync to layers (usually done automatically by Captain)
lm := memory.NewLayerManager(db, repoPath)
lm.SyncToWarmLayer(ctx, "target-env")
lm.SyncToHotLayer(ctx, "target-env")
```

## For Captain Agent

Query findings to make decisions:

```go
// Get all open critical findings
criticalFindings, _ := db.GetFindings(ctx, memory.FindingFilter{
    EnvID:    envID,
    Severity: "critical",
    Status:   "open",
})

// Check if environment needs attention
status, _ := lm.GetLayerStatus(ctx, envID)
if status.ColdLayer.CriticalCount > 0 {
    // Escalate to human or spawn worker agents
}

// Get latest scan results
latestScan, _ := db.GetLatestScan(ctx, envID)
if latestScan.SecurityScore == "F" {
    // Take action
}
```

## For Worker Agents

Update finding status when fixed:

```go
// Mark finding as resolved
db.UpdateFindingStatus(ctx, "VULN-001", "resolved", "OpusGreen",
    "Applied input sanitization in commit abc123")

// Record the change
db.RecordFindingChange(ctx, &memory.FindingHistoryEntry{
    FindingID:  "VULN-001",
    ChangedBy:  "OpusGreen",
    ChangeType: "status_change",
    OldValue:   "open",
    NewValue:   "resolved",
    Notes:      "Fixed in PR #123",
})

// Sync changes to layers
lm.SyncToWarmLayer(ctx, envID)
lm.SyncToHotLayer(ctx, envID)
```

## For Humans

### View Findings

**Quick Summary**: Check CLAUDE.md's "Recon Intelligence" section

**Detailed Review**: Browse markdown files in `docs/recon/`
- `vulnerabilities.md` - Security issues
- `architecture.md` - Design problems
- `dependencies.md` - Library issues
- `infrastructure.md` - Process/deployment issues

**Historical Analysis**: Query SQLite database directly

```sql
-- Find trends
SELECT DATE(discovered_at), severity, COUNT(*)
FROM recon_findings
WHERE env_id = 'prod'
GROUP BY DATE(discovered_at), severity;

-- Check resolution rate
SELECT
    status,
    COUNT(*) as count,
    ROUND(COUNT(*) * 100.0 / (SELECT COUNT(*) FROM recon_findings), 2) as percent
FROM recon_findings
GROUP BY status;
```

### Manage Findings

Use the memory API or update the database:

```bash
# Mark as false positive
sqlite3 data/memory.db \
  "UPDATE recon_findings SET status='false_positive',
   resolution_notes='Not actually vulnerable' WHERE id='VULN-001'"

# Ignore a finding
sqlite3 data/memory.db \
  "UPDATE recon_findings SET status='ignored',
   resolution_notes='Accepted risk' WHERE id='ARCH-001'"
```

Then sync changes:
```bash
# Re-sync layers (would typically be in code)
go run ./cmd/sync-layers --env-id prod
```

## Finding IDs

Use these prefixes for finding IDs:
- `VULN-###` - Security vulnerabilities
- `ARCH-###` - Architecture issues
- `DEP-###` - Dependency problems
- `PROC-###` - Process/infrastructure
- `PERF-###` - Performance issues

## Severity Guidelines

**Critical**: Immediate action required
- Remote code execution
- SQL injection
- Authentication bypass
- Data exposure

**High**: Schedule soon
- XSS vulnerabilities
- Missing rate limiting
- Weak cryptography
- Major architecture flaws

**Medium**: Should fix
- Outdated dependencies
- Missing input validation
- Poor error handling
- Code quality issues

**Low**: Nice to fix
- Minor code smells
- Documentation gaps
- Style inconsistencies

**Info**: Informational only
- Observations
- Recommendations
- Best practices

## Layer Sync Strategy

**When to Sync**:
- After every scan completes
- When findings are resolved
- Before spawning worker agents
- On schedule (nightly for production)

**Performance**:
- Cold → Warm: ~100ms for 1000 findings
- Cold → Hot: ~50ms (only critical/high)
- Warm files: Human browsing speed
- Hot section: Auto-loaded every session

## Troubleshooting

**Missing findings in CLAUDE.md?**
- Check severity (only critical/high appear)
- Check status (only open findings)
- Verify last sync timestamp

**Markdown files outdated?**
- Run `lm.SyncToWarmLayer()`
- Check write permissions on docs/recon/

**Database locked error?**
- Only one writer at a time
- Use transactions for batch operations
- Check for abandoned connections

**Can't find environment?**
- Ensure `RegisterEnvironment()` was called
- Check environment ID matches

## Best Practices

1. **Always register environment first** before recording scans
2. **Use batch operations** for multiple findings: `SaveFindings()`
3. **Include location** in findings for faster resolution
4. **Write clear recommendations** - specific, actionable
5. **Update CLAUDE.md** after resolving critical issues
6. **Record history** when changing finding status
7. **Use meaningful IDs** that indicate finding type

## Example Workflow

```
1. Snake001 spawned → targets environment "acme-prod"
2. Snake registers environment if not exists
3. Snake creates scan record
4. Snake discovers 10 findings → saves each
5. Snake completes scan with summary
6. Captain reviews critical findings from CLAUDE.md
7. Captain spawns OpusRed to fix VULN-001
8. OpusRed fixes code, marks finding resolved
9. System re-syncs layers
10. CLAUDE.md updated, markdown regenerated
11. Human reviews progress in vulnerabilities.md
```

## Integration Points

- **MCP Tools**: Snake agents use `submit_recon_report` tool
- **Captain Decision Engine**: Reads from cold layer
- **Worker Spawning**: Captain includes relevant findings in context
- **Human Dashboard**: Future web UI will query cold layer
- **CI/CD**: Can gate on critical finding count

---

For full documentation, see [README.md](README.md)
