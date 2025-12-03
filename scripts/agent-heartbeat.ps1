# Agent Heartbeat Monitor
# Spawned alongside each Claude agent to handle:
# 1. Writing heartbeats to DB every N seconds
# 2. Checking for shutdown signals
# 3. Creating shutdown marker file when signaled

param(
    [Parameter(Mandatory=$true)]
    [string]$AgentID,

    [string]$DBPath = "data/memory.db",

    [int]$IntervalSeconds = 30,

    [string]$DBCtlPath = "bin/dbctl.exe"
)

$ErrorActionPreference = "Stop"

# Resolve paths to absolute if relative
$projectRoot = Split-Path (Split-Path $PSScriptRoot -Parent) -Parent

if (-not [System.IO.Path]::IsPathRooted($DBPath)) {
    $DBPath = Join-Path $projectRoot $DBPath
}

if (-not [System.IO.Path]::IsPathRooted($DBCtlPath)) {
    $DBCtlPath = Join-Path $projectRoot $DBCtlPath
}

# Verify dbctl exists
if (-not (Test-Path $DBCtlPath)) {
    Write-Host "[HEARTBEAT] ERROR: dbctl not found at: $DBCtlPath" -ForegroundColor Red
    Write-Host "[HEARTBEAT] Please run: go build -o bin/dbctl.exe ./cmd/dbctl" -ForegroundColor Yellow
    exit 1
}

Write-Host "[HEARTBEAT] Started monitor for agent: $AgentID" -ForegroundColor Cyan
Write-Host "[HEARTBEAT] DB: $DBPath" -ForegroundColor DarkGray
Write-Host "[HEARTBEAT] Interval: ${IntervalSeconds}s" -ForegroundColor DarkGray
Write-Host "[HEARTBEAT] DBCtl: $DBCtlPath" -ForegroundColor DarkGray

$heartbeatCount = 0

while ($true) {
    try {
        # 1. Write heartbeat
        $result = & $DBCtlPath -db $DBPath -action heartbeat -agent $AgentID 2>&1

        if ($LASTEXITCODE -eq 0) {
            $heartbeatCount++
            Write-Host "[HEARTBEAT] #$heartbeatCount OK" -ForegroundColor DarkGreen
        } else {
            Write-Host "[HEARTBEAT] Warning: Failed to update heartbeat - $result" -ForegroundColor Yellow
        }

        # 2. Check shutdown flag
        $shutdownCheck = & $DBCtlPath -db $DBPath -action check-shutdown -agent $AgentID 2>&1

        if ($LASTEXITCODE -eq 0) {
            # Parse output: first line is flag (0/1), second line is reason (if flag=1)
            $lines = $shutdownCheck -split "`n"
            $shutdownFlag = $lines[0].Trim()

            if ($shutdownFlag -eq "1") {
                $reason = if ($lines.Length -gt 1) { $lines[1].Trim() } else { "No reason provided" }

                Write-Host "" -ForegroundColor Cyan
                Write-Host "[HEARTBEAT] ========================================" -ForegroundColor Cyan
                Write-Host "[HEARTBEAT] SHUTDOWN SIGNAL RECEIVED" -ForegroundColor Yellow
                Write-Host "[HEARTBEAT] ========================================" -ForegroundColor Cyan
                Write-Host "[HEARTBEAT] Reason: $reason" -ForegroundColor Yellow

                # Create shutdown marker file for agent to detect
                $markerPath = Join-Path (Split-Path $DBPath) "shutdown-$AgentID.flag"
                Set-Content -Path $markerPath -Value $reason

                Write-Host "[HEARTBEAT] Created shutdown marker: $markerPath" -ForegroundColor Green
                Write-Host "[HEARTBEAT] Agent should detect this file and exit gracefully" -ForegroundColor Green
                Write-Host "[HEARTBEAT] Exiting heartbeat monitor..." -ForegroundColor Cyan
                Write-Host "" -ForegroundColor Cyan

                exit 0
            }
        } else {
            Write-Host "[HEARTBEAT] Warning: Failed to check shutdown flag - $shutdownCheck" -ForegroundColor Yellow
        }

    } catch {
        Write-Host "[HEARTBEAT] Error: $_" -ForegroundColor Red
    }

    Start-Sleep -Seconds $IntervalSeconds
}
