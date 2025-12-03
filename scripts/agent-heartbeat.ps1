# Agent Heartbeat Monitor
# Spawned alongside each Claude agent to handle:
# 1. Writing heartbeats via HTTP POST every N seconds
# 2. Monitoring agent health

param(
    [Parameter(Mandatory=$true)]
    [string]$AgentID,

    [string]$ServerURL = "http://localhost:3000",

    [int]$IntervalSeconds = 30,

    [string]$CurrentTask = "initializing"
)

$ErrorActionPreference = "Stop"

Write-Host "[HEARTBEAT] Started monitor for agent: $AgentID" -ForegroundColor Cyan
Write-Host "[HEARTBEAT] Server: $ServerURL" -ForegroundColor DarkGray
Write-Host "[HEARTBEAT] Interval: ${IntervalSeconds}s" -ForegroundColor DarkGray

$heartbeatCount = 0
$heartbeatURL = "$ServerURL/api/heartbeat"

while ($true) {
    try {
        # Prepare heartbeat payload
        $body = @{
            agent_id = $AgentID
            status = "working"
            current_task = $CurrentTask
        } | ConvertTo-Json

        # Send heartbeat via HTTP POST
        $response = Invoke-RestMethod -Uri $heartbeatURL -Method Post -Body $body -ContentType "application/json" -TimeoutSec 5

        $heartbeatCount++
        Write-Host "[HEARTBEAT] #$heartbeatCount OK" -ForegroundColor DarkGreen

    } catch {
        Write-Host "[HEARTBEAT] Warning: Failed to send heartbeat - $($_.Exception.Message)" -ForegroundColor Yellow
    }

    Start-Sleep -Seconds $IntervalSeconds
}
