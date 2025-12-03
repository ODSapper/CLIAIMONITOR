# Simple manual test for heartbeat functionality
# This is a minimal test that doesn't require Job scheduling

param(
    [string]$TestDuration = 20
)

Write-Host ""
Write-Host "================================" -ForegroundColor Cyan
Write-Host "  Heartbeat Manual Test" -ForegroundColor Cyan
Write-Host "================================" -ForegroundColor Cyan
Write-Host ""

$projectRoot = $PWD
$dbPath = Join-Path $projectRoot "data/memory.db"
$dbCtlPath = Join-Path $projectRoot "bin/dbctl.exe"
$testAgentID = "manual-test-$(Get-Date -Format 'HHmmss')"
$markerPath = Join-Path (Join-Path $projectRoot "data") "shutdown-$testAgentID.flag"

Write-Host "Test Agent ID: $testAgentID" -ForegroundColor Yellow
Write-Host ""

# Step 1: Ensure dbctl is built
if (-not (Test-Path $dbCtlPath)) {
    Write-Host "Building dbctl..." -ForegroundColor Yellow
    go build -o $dbCtlPath ./cmd/dbctl
    if ($LASTEXITCODE -ne 0) {
        Write-Host "FAILED to build dbctl" -ForegroundColor Red
        exit 1
    }
}

# Step 2: Create test agent using Go
Write-Host "Creating test agent in database..." -ForegroundColor Cyan

$createScript = @"
package main
import (
    "database/sql"
    "fmt"
    "os"
    _ "github.com/mattn/go-sqlite3"
)
func main() {
    db, _ := sql.Open("sqlite3", "data/memory.db?_journal_mode=WAL")
    defer db.Close()
    _, err := db.Exec(``INSERT INTO agent_control (agent_id, config_name, role, status, spawned_at) VALUES (?, 'test', 'tester', 'starting', datetime('now'))``, "$testAgentID")
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
    fmt.Println("Created")
}
"@

$tempCreate = Join-Path $env:TEMP "create_agent.go"
Set-Content -Path $tempCreate -Value $createScript
$output = go run $tempCreate 2>&1

if ($LASTEXITCODE -ne 0) {
    Write-Host "FAILED to create test agent" -ForegroundColor Red
    Write-Host $output
    exit 1
}

Write-Host "Agent created successfully" -ForegroundColor Green
Write-Host ""

# Step 3: Start heartbeat script in separate window
Write-Host "Starting heartbeat monitor in background..." -ForegroundColor Cyan
Write-Host "(Check for new PowerShell window)" -ForegroundColor Yellow
Write-Host ""

$heartbeatScript = Join-Path $projectRoot "scripts/agent-heartbeat.ps1"
$heartbeatProcess = Start-Process powershell.exe -ArgumentList @(
    "-NoProfile",
    "-ExecutionPolicy", "Bypass",
    "-File", $heartbeatScript,
    "-AgentID", $testAgentID,
    "-IntervalSeconds", "5",
    "-DBPath", $dbPath,
    "-DBCtlPath", $dbCtlPath
) -PassThru -WindowStyle Normal

Write-Host "Heartbeat process started (PID: $($heartbeatProcess.Id))" -ForegroundColor Green
Write-Host ""

# Step 4: Monitor heartbeats
Write-Host "Monitoring heartbeats for $TestDuration seconds..." -ForegroundColor Cyan
Write-Host "Press Ctrl+C to stop early" -ForegroundColor DarkGray
Write-Host ""

$iterations = [Math]::Floor($TestDuration / 5)
for ($i = 1; $i -le $iterations; $i++) {
    Start-Sleep -Seconds 5

    # Check agent info
    $info = & $dbCtlPath -db $dbPath -action get-agent -agent $testAgentID -json 2>&1
    if ($LASTEXITCODE -eq 0) {
        $agent = $info | ConvertFrom-Json
        Write-Host "[$i/$iterations] Status: $($agent.status) | Last heartbeat: $($agent.heartbeat_at)" -ForegroundColor Green
    } else {
        Write-Host "[$i/$iterations] Error getting agent info" -ForegroundColor Red
    }
}

Write-Host ""
Write-Host "Testing shutdown signal..." -ForegroundColor Cyan

# Step 5: Set shutdown flag
$shutdownScript = @"
package main
import (
    "database/sql"
    _ "github.com/mattn/go-sqlite3"
)
func main() {
    db, _ := sql.Open("sqlite3", "data/memory.db?_journal_mode=WAL")
    defer db.Close()
    db.Exec(``UPDATE agent_control SET shutdown_flag=1, shutdown_reason='Manual test shutdown' WHERE agent_id=?``, "$testAgentID")
}
"@

$tempShutdown = Join-Path $env:TEMP "shutdown_agent.go"
Set-Content -Path $tempShutdown -Value $shutdownScript
go run $tempShutdown 2>&1 | Out-Null

Write-Host "Shutdown flag set in database" -ForegroundColor Yellow
Write-Host "Waiting for heartbeat script to detect..." -ForegroundColor Yellow
Write-Host ""

# Wait for process to exit or marker file
$maxWait = 15
$waited = 0
while ($waited -lt $maxWait) {
    if (Test-Path $markerPath) {
        Write-Host "SUCCESS: Shutdown marker file detected!" -ForegroundColor Green
        $reason = Get-Content $markerPath -Raw
        Write-Host "Reason: $reason" -ForegroundColor Green
        break
    }

    if ($heartbeatProcess.HasExited) {
        Write-Host "Heartbeat process has exited" -ForegroundColor Yellow
        break
    }

    Start-Sleep -Seconds 1
    $waited++
}

if (-not (Test-Path $markerPath)) {
    Write-Host "WARNING: Marker file not created within $maxWait seconds" -ForegroundColor Yellow
} else {
    Write-Host "Marker path: $markerPath" -ForegroundColor DarkGray
}

Write-Host ""
Write-Host "Cleaning up..." -ForegroundColor Cyan

# Kill process if still running
if (-not $heartbeatProcess.HasExited) {
    Write-Host "Stopping heartbeat process..." -ForegroundColor Yellow
    Stop-Process -Id $heartbeatProcess.Id -Force -ErrorAction SilentlyContinue
}

# Remove marker file
if (Test-Path $markerPath) {
    Remove-Item $markerPath -Force
}

# Remove test agent from DB
$cleanupScript = @"
package main
import (
    "database/sql"
    _ "github.com/mattn/go-sqlite3"
)
func main() {
    db, _ := sql.Open("sqlite3", "data/memory.db?_journal_mode=WAL")
    defer db.Close()
    db.Exec("DELETE FROM agent_control WHERE agent_id=?", "$testAgentID")
}
"@

$tempCleanup = Join-Path $env:TEMP "cleanup_agent.go"
Set-Content -Path $tempCleanup -Value $cleanupScript
go run $tempCleanup 2>&1 | Out-Null

# Remove temp files
Remove-Item $tempCreate -Force -ErrorAction SilentlyContinue
Remove-Item $tempShutdown -Force -ErrorAction SilentlyContinue
Remove-Item $tempCleanup -Force -ErrorAction SilentlyContinue

Write-Host "Cleanup complete" -ForegroundColor Green
Write-Host ""
Write-Host "================================" -ForegroundColor Cyan
Write-Host "  Test Complete" -ForegroundColor Cyan
Write-Host "================================" -ForegroundColor Cyan
Write-Host ""
