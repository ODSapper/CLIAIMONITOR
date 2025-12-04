# Agent Heartbeat Monitor
# Spawned alongside each Claude agent to handle:
# 1. Writing heartbeats via HTTP POST every N seconds
# 2. Monitoring agent health - exits when Claude process dies

param(
    [Parameter(Mandatory=$true)]
    [string]$AgentID,

    [string]$ServerURL = "http://localhost:3000",

    [int]$IntervalSeconds = 15,

    [string]$CurrentTask = "initializing",

    [int]$ClaudePID = 0
)

$ErrorActionPreference = "Stop"

Write-Host "[HEARTBEAT] Started monitor for agent: $AgentID" -ForegroundColor Cyan
Write-Host "[HEARTBEAT] Server: $ServerURL" -ForegroundColor DarkGray
Write-Host "[HEARTBEAT] Interval: ${IntervalSeconds}s" -ForegroundColor DarkGray
if ($ClaudePID -gt 0) {
    Write-Host "[HEARTBEAT] Monitoring Claude PID: $ClaudePID" -ForegroundColor DarkGray
}

$heartbeatCount = 0
$heartbeatURL = "$ServerURL/api/heartbeat"

# Function to check if Claude process is still alive
function Test-ClaudeAlive {
    # Method 1: Check PID file for PowerShell launcher PID
    $pidFile = "C:\Users\Admin\Documents\VS Projects\CLIAIMONITOR\data\pids\$AgentID.pid"
    if (Test-Path $pidFile) {
        $launcherPID = [int](Get-Content $pidFile -ErrorAction SilentlyContinue)

        # Check if launcher PowerShell is still alive
        $launcherProc = Get-Process -Id $launcherPID -ErrorAction SilentlyContinue
        if ($null -eq $launcherProc) {
            # Launcher is dead - Claude definitely not running
            return $false
        }

        # Launcher is alive - check if it has a claude.exe child process
        # Get child processes of the launcher
        $children = Get-CimInstance Win32_Process -Filter "ParentProcessId=$launcherPID" -ErrorAction SilentlyContinue
        $claudeChild = $children | Where-Object { $_.Name -eq "claude.exe" }

        if ($claudeChild) {
            return $true
        }

        # No claude.exe child - maybe Claude hasn't started yet or already exited
        # Give a grace period for initial startup (first 60 seconds)
        $launcherAge = (Get-Date) - $launcherProc.StartTime
        if ($launcherAge.TotalSeconds -lt 60) {
            return $true  # Still starting up
        }

        # Launcher alive but no Claude child after 60s = Claude exited
        return $false
    }

    # No PID file, can't determine status
    return $true
}

while ($true) {
    # Check if Claude process is still running
    if (-not (Test-ClaudeAlive)) {
        Write-Host "[HEARTBEAT] Claude process died - sending final disconnected status" -ForegroundColor Red

        # Send one final "disconnected" heartbeat
        try {
            $body = @{
                agent_id = $AgentID
                status = "disconnected"
                current_task = "process exited"
            } | ConvertTo-Json
            Invoke-RestMethod -Uri $heartbeatURL -Method Post -Body $body -ContentType "application/json" -TimeoutSec 5
        } catch {
            # Ignore errors on final heartbeat
        }

        Write-Host "[HEARTBEAT] Exiting heartbeat monitor" -ForegroundColor Yellow
        exit 0
    }

    try {
        # Prepare heartbeat payload
        $body = @{
            agent_id = $AgentID
            status = "working"
            current_task = $CurrentTask
        } | ConvertTo-Json

        # Send heartbeat via HTTP POST
        $response = Invoke-RestMethod -Uri $heartbeatURL -Method Post -Body $body -ContentType "application/json" -TimeoutSec 5

        # Check if server told us to stop (shutdown_requested on server side)
        if ($response.stop -eq $true) {
            Write-Host "[HEARTBEAT] Server requested stop: $($response.reason)" -ForegroundColor Yellow
            Write-Host "[HEARTBEAT] Exiting heartbeat monitor" -ForegroundColor Yellow
            exit 0
        }

        $heartbeatCount++
        Write-Host "[HEARTBEAT] #$heartbeatCount OK" -ForegroundColor DarkGreen

    } catch {
        Write-Host "[HEARTBEAT] Warning: Failed to send heartbeat - $($_.Exception.Message)" -ForegroundColor Yellow
    }

    Start-Sleep -Seconds $IntervalSeconds
}
