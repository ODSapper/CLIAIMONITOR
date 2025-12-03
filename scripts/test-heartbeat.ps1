# Test script for agent heartbeat functionality
# Verifies:
# 1. dbctl utility works
# 2. Heartbeat script can write to DB
# 3. Shutdown signal is detected
# 4. Marker file is created

param(
    [string]$DBPath = "data/memory.db",
    [string]$DBCtlPath = "bin/dbctl.exe",
    [int]$TestIntervalSeconds = 3
)

$ErrorActionPreference = "Stop"

Write-Host ""
Write-Host "======================================" -ForegroundColor Cyan
Write-Host "  Agent Heartbeat Test Suite" -ForegroundColor Cyan
Write-Host "======================================" -ForegroundColor Cyan
Write-Host ""

# Resolve paths
$projectRoot = $PWD
if (-not [System.IO.Path]::IsPathRooted($DBPath)) {
    $DBPath = Join-Path $projectRoot $DBPath
}
if (-not [System.IO.Path]::IsPathRooted($DBCtlPath)) {
    $DBCtlPath = Join-Path $projectRoot $DBCtlPath
}

$testAgentID = "test-heartbeat-$(Get-Date -Format 'HHmmss')"
$markerPath = Join-Path (Split-Path $DBPath) "shutdown-$testAgentID.flag"

Write-Host "[TEST] Test Agent ID: $testAgentID" -ForegroundColor Yellow
Write-Host "[TEST] DB Path: $DBPath" -ForegroundColor DarkGray
Write-Host "[TEST] DBCtl Path: $DBCtlPath" -ForegroundColor DarkGray
Write-Host ""

# Step 1: Build dbctl if not exists
Write-Host "[TEST] Step 1: Checking dbctl..." -ForegroundColor Cyan
if (-not (Test-Path $DBCtlPath)) {
    Write-Host "[TEST] Building dbctl..." -ForegroundColor Yellow
    $buildOutput = go build -o $DBCtlPath ./cmd/dbctl 2>&1
    if ($LASTEXITCODE -ne 0) {
        Write-Host "[TEST] FAILED: Could not build dbctl" -ForegroundColor Red
        Write-Host $buildOutput
        exit 1
    }
    Write-Host "[TEST] Built dbctl successfully" -ForegroundColor Green
} else {
    Write-Host "[TEST] dbctl already exists" -ForegroundColor Green
}
Write-Host ""

# Step 2: Create test agent entry in DB
Write-Host "[TEST] Step 2: Creating test agent entry in DB..." -ForegroundColor Cyan

# We need to use the Go app or direct SQL to create the agent
# For now, let's use a simple SQL command via dbctl extension
# Actually, let's create a minimal SQL file and use sqlite3 if available, or create via Go

# Check if agent_control table exists
Write-Host "[TEST] Checking database schema..." -ForegroundColor Yellow

# Create a small Go script to insert test agent
$tempInsertScript = @"
package main

import (
	"database/sql"
	"fmt"
	"os"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	db, err := sql.Open("sqlite3", "$($DBPath.Replace('\', '/'))?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open DB: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// Insert test agent
	_, err = db.Exec(``
		INSERT OR REPLACE INTO agent_control
		(agent_id, config_name, role, status, spawned_at)
		VALUES (?, ?, ?, ?, datetime('now'))
	``, "$testAgentID", "test-config", "test", "starting")

	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to insert agent: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Test agent created successfully")
}
"@

$tempInsertPath = Join-Path $env:TEMP "insert_test_agent.go"
Set-Content -Path $tempInsertPath -Value $tempInsertScript

$insertOutput = go run $tempInsertPath 2>&1
if ($LASTEXITCODE -ne 0) {
    Write-Host "[TEST] FAILED: Could not create test agent" -ForegroundColor Red
    Write-Host $insertOutput
    exit 1
}

Write-Host "[TEST] Test agent created in DB" -ForegroundColor Green
Write-Host ""

# Step 3: Start heartbeat script in background
Write-Host "[TEST] Step 3: Starting heartbeat script..." -ForegroundColor Cyan
$heartbeatJob = Start-Job -ScriptBlock {
    param($scriptPath, $agentID, $dbPath, $interval, $dbctlPath)
    & $scriptPath -AgentID $agentID -DBPath $dbPath -IntervalSeconds $interval -DBCtlPath $dbctlPath
} -ArgumentList @(
    (Join-Path $projectRoot "scripts/agent-heartbeat.ps1"),
    $testAgentID,
    $DBPath,
    $TestIntervalSeconds,
    $DBCtlPath
)

Write-Host "[TEST] Heartbeat script started (Job ID: $($heartbeatJob.Id))" -ForegroundColor Green
Write-Host ""

# Step 4: Wait and verify heartbeats
Write-Host "[TEST] Step 4: Verifying heartbeats..." -ForegroundColor Cyan
Write-Host "[TEST] Waiting $(($TestIntervalSeconds * 2)) seconds for heartbeats..." -ForegroundColor Yellow

Start-Sleep -Seconds ($TestIntervalSeconds * 2)

# Check agent info
$agentInfo = & $DBCtlPath -db $DBPath -action get-agent -agent $testAgentID -json 2>&1
if ($LASTEXITCODE -eq 0) {
    $agent = $agentInfo | ConvertFrom-Json
    Write-Host "[TEST] Agent Status: $($agent.status)" -ForegroundColor Green
    Write-Host "[TEST] Last Heartbeat: $($agent.heartbeat_at)" -ForegroundColor Green

    if ($null -eq $agent.heartbeat_at) {
        Write-Host "[TEST] WARNING: No heartbeat recorded yet!" -ForegroundColor Yellow
    }
} else {
    Write-Host "[TEST] WARNING: Could not retrieve agent info" -ForegroundColor Yellow
    Write-Host $agentInfo
}

# Show job output
Write-Host ""
Write-Host "[TEST] Heartbeat job output:" -ForegroundColor DarkGray
Receive-Job -Job $heartbeatJob | ForEach-Object { Write-Host "  $_" -ForegroundColor DarkGray }
Write-Host ""

# Step 5: Test shutdown signal
Write-Host "[TEST] Step 5: Testing shutdown signal..." -ForegroundColor Cyan

# Set shutdown flag via SQL
$tempShutdownScript = @"
package main

import (
	"database/sql"
	"fmt"
	"os"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	db, err := sql.Open("sqlite3", "$($DBPath.Replace('\', '/'))?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open DB: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	_, err = db.Exec(``
		UPDATE agent_control
		SET shutdown_flag = 1, shutdown_reason = ?
		WHERE agent_id = ?
	``, "Test shutdown from test script", "$testAgentID")

	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to set shutdown flag: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Shutdown flag set")
}
"@

$tempShutdownPath = Join-Path $env:TEMP "set_shutdown_flag.go"
Set-Content -Path $tempShutdownPath -Value $tempShutdownScript

$shutdownOutput = go run $tempShutdownPath 2>&1
if ($LASTEXITCODE -eq 0) {
    Write-Host "[TEST] Shutdown flag set in DB" -ForegroundColor Green
} else {
    Write-Host "[TEST] FAILED: Could not set shutdown flag" -ForegroundColor Red
    Write-Host $shutdownOutput
    Stop-Job -Job $heartbeatJob
    Remove-Job -Job $heartbeatJob
    exit 1
}

Write-Host "[TEST] Waiting for heartbeat script to detect shutdown..." -ForegroundColor Yellow
Write-Host ""

# Wait for job to complete (max 30 seconds)
$timeout = 30
$waited = 0
while ($heartbeatJob.State -eq "Running" -and $waited -lt $timeout) {
    Start-Sleep -Seconds 1
    $waited++

    # Show job output in real-time
    $output = Receive-Job -Job $heartbeatJob
    if ($output) {
        $output | ForEach-Object { Write-Host "  [JOB] $_" -ForegroundColor DarkGray }
    }
}

# Step 6: Verify shutdown marker file
Write-Host ""
Write-Host "[TEST] Step 6: Verifying shutdown marker file..." -ForegroundColor Cyan

if (Test-Path $markerPath) {
    $markerContent = Get-Content -Path $markerPath -Raw
    Write-Host "[TEST] SUCCESS: Shutdown marker file created!" -ForegroundColor Green
    Write-Host "[TEST] Marker path: $markerPath" -ForegroundColor Green
    Write-Host "[TEST] Reason: $markerContent" -ForegroundColor Green
} else {
    Write-Host "[TEST] FAILED: Shutdown marker file not found!" -ForegroundColor Red
    Write-Host "[TEST] Expected path: $markerPath" -ForegroundColor Red
}

# Check job state
Write-Host ""
Write-Host "[TEST] Heartbeat job final state: $($heartbeatJob.State)" -ForegroundColor $(if ($heartbeatJob.State -eq "Completed") { "Green" } else { "Yellow" })

# Get final output
$finalOutput = Receive-Job -Job $heartbeatJob
if ($finalOutput) {
    Write-Host ""
    Write-Host "[TEST] Final job output:" -ForegroundColor DarkGray
    $finalOutput | ForEach-Object { Write-Host "  $_" -ForegroundColor DarkGray }
}

# Cleanup
Write-Host ""
Write-Host "[TEST] Step 7: Cleanup..." -ForegroundColor Cyan

Stop-Job -Job $heartbeatJob -ErrorAction SilentlyContinue
Remove-Job -Job $heartbeatJob -ErrorAction SilentlyContinue

if (Test-Path $markerPath) {
    Remove-Item -Path $markerPath -Force
    Write-Host "[TEST] Removed marker file" -ForegroundColor DarkGray
}

# Remove test agent from DB
$tempCleanupScript = @"
package main

import (
	"database/sql"
	"fmt"
	"os"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	db, err := sql.Open("sqlite3", "$($DBPath.Replace('\', '/'))?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open DB: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	_, err = db.Exec("DELETE FROM agent_control WHERE agent_id = ?", "$testAgentID")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to delete test agent: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Test agent removed")
}
"@

$tempCleanupPath = Join-Path $env:TEMP "cleanup_test_agent.go"
Set-Content -Path $tempCleanupPath -Value $tempCleanupScript
$cleanupOutput = go run $tempCleanupPath 2>&1

if ($LASTEXITCODE -eq 0) {
    Write-Host "[TEST] Removed test agent from DB" -ForegroundColor DarkGray
}

# Cleanup temp files
Remove-Item -Path $tempInsertPath -Force -ErrorAction SilentlyContinue
Remove-Item -Path $tempShutdownPath -Force -ErrorAction SilentlyContinue
Remove-Item -Path $tempCleanupPath -Force -ErrorAction SilentlyContinue

Write-Host ""
Write-Host "======================================" -ForegroundColor Cyan
Write-Host "  Test Complete" -ForegroundColor Cyan
Write-Host "======================================" -ForegroundColor Cyan
Write-Host ""
