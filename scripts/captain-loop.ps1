# Captain Loop - Autonomous monitoring with Haiku-powered decisions
# Runs continuously, polls API, handles routine tasks, escalates when needed

param(
    [string]$ServerURL = "http://localhost:3000",
    [int]$PollIntervalSeconds = 30,
    [string]$Model = "claude-3-5-haiku-20241022",
    [switch]$DryRun  # Don't actually call claude, just log what would happen
)

$ErrorActionPreference = "Continue"

# Track what we've already processed to avoid duplicates
$processedStopRequests = @{}
$processedAlerts = @{}
$lastHeartbeatCheck = @{}

Write-Host ""
Write-Host "  ================================================" -ForegroundColor Cyan
Write-Host "    CAPTAIN LOOP - Autonomous Monitoring" -ForegroundColor Green
Write-Host "  ================================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "  Server:   $ServerURL" -ForegroundColor Yellow
Write-Host "  Interval: ${PollIntervalSeconds}s" -ForegroundColor Yellow
Write-Host "  Model:    $Model" -ForegroundColor Yellow
Write-Host "  DryRun:   $DryRun" -ForegroundColor Yellow
Write-Host ""
Write-Host "  Press Ctrl+C to stop" -ForegroundColor DarkGray
Write-Host ""

function Get-ApiData {
    param([string]$Endpoint)
    try {
        $response = Invoke-RestMethod -Uri "$ServerURL$Endpoint" -Method GET -TimeoutSec 10
        return $response
    } catch {
        Write-Host "  [ERROR] Failed to fetch $Endpoint" -ForegroundColor Red
        return $null
    }
}

function Send-ApiPost {
    param([string]$Endpoint, [hashtable]$Body)
    try {
        $json = $Body | ConvertTo-Json -Depth 10
        $response = Invoke-RestMethod -Uri "$ServerURL$Endpoint" -Method POST -Body $json -ContentType "application/json" -TimeoutSec 10
        return $response
    } catch {
        Write-Host "  [ERROR] Failed to POST $Endpoint" -ForegroundColor Red
        return $null
    }
}

function Invoke-HaikuDecision {
    param(
        [string]$Context,
        [string]$Question
    )

    $prompt = @"
You are Captain's assistant, making quick decisions about agent management.

CONTEXT:
$Context

QUESTION:
$Question

RESPOND WITH EXACTLY ONE OF:
- APPROVE: <brief reason>
- RESPAWN: <brief reason>
- ESCALATE_CAPTAIN: <why full Captain needed>
- ESCALATE_HUMAN: <why human input needed>
- SKIP: <brief reason>

Be concise. One line only.
"@

    if ($DryRun) {
        Write-Host "  [DRY-RUN] Would ask Haiku: $Question" -ForegroundColor Magenta
        return "SKIP: Dry run mode"
    }

    try {
        $result = & claude --print --model $Model $prompt 2>&1
        return $result
    } catch {
        Write-Host "  [ERROR] Haiku call failed: $_" -ForegroundColor Red
        return "ESCALATE_HUMAN: Haiku call failed"
    }
}

function Process-StopRequests {
    $stopRequests = Get-ApiData "/api/stop-requests"

    if (-not $stopRequests -or -not $stopRequests.stop_requests) {
        return
    }

    foreach ($request in $stopRequests.stop_requests) {
        # Skip already processed or reviewed
        if ($request.reviewed -or $processedStopRequests[$request.id]) {
            continue
        }

        Write-Host ""
        Write-Host "  [STOP REQUEST] $($request.agent_id)" -ForegroundColor Yellow
        Write-Host "    Reason: $($request.reason)" -ForegroundColor DarkGray
        Write-Host "    Context: $($request.context.Substring(0, [Math]::Min(100, $request.context.Length)))..." -ForegroundColor DarkGray

        $context = @"
Agent: $($request.agent_id)
Reason: $($request.reason)
Work Completed: $($request.work_completed.Substring(0, [Math]::Min(500, $request.work_completed.Length)))
"@

        $decision = Invoke-HaikuDecision -Context $context -Question "Should this stop request be approved?"

        Write-Host "    Decision: $decision" -ForegroundColor Cyan

        if ($decision -match "^APPROVE") {
            # Auto-approve
            $response = Send-ApiPost "/api/stop-requests/$($request.id)/respond" @{
                approved = $true
                response = "Auto-approved by Captain Loop: $decision"
            }
            if ($response.success) {
                Write-Host "    -> Approved" -ForegroundColor Green
            }
        }
        elseif ($decision -match "^ESCALATE_HUMAN") {
            Write-Host "    -> NEEDS HUMAN INPUT" -ForegroundColor Red
            Write-Host ""
            Write-Host "  !!! HUMAN ATTENTION REQUIRED !!!" -ForegroundColor Red -BackgroundColor Yellow
            Write-Host "  Agent: $($request.agent_id)" -ForegroundColor White
            Write-Host "  Reason: $decision" -ForegroundColor White
            Write-Host ""
            # Could also spawn a notification or write to a file
        }
        elseif ($decision -match "^ESCALATE_CAPTAIN") {
            Write-Host "    -> Needs full Captain review" -ForegroundColor Yellow
            # Could spawn full Captain session here
        }

        $processedStopRequests[$request.id] = $true
    }
}

function Process-StaleAgents {
    $heartbeats = Get-ApiData "/api/heartbeats"

    if (-not $heartbeats) {
        return
    }

    $now = Get-Date
    $staleThreshold = 120  # seconds

    foreach ($agentId in $heartbeats.PSObject.Properties.Name) {
        $info = $heartbeats.$agentId

        if (-not $info.last_seen) {
            continue
        }

        $lastSeen = [DateTime]::Parse($info.last_seen)
        $age = ($now - $lastSeen).TotalSeconds

        if ($age -gt $staleThreshold) {
            # Check if we already handled this
            if ($lastHeartbeatCheck[$agentId] -eq $info.last_seen) {
                continue
            }

            Write-Host ""
            Write-Host "  [STALE AGENT] $agentId" -ForegroundColor Yellow
            Write-Host "    Last seen: $([int]$age) seconds ago" -ForegroundColor DarkGray
            Write-Host "    Task: $($info.current_task)" -ForegroundColor DarkGray

            $context = @"
Agent: $agentId
Last seen: $([int]$age) seconds ago
Status: $($info.status)
Current task: $($info.current_task)
"@

            $decision = Invoke-HaikuDecision -Context $context -Question "Agent appears dead. Should we respawn it to continue its task?"

            Write-Host "    Decision: $decision" -ForegroundColor Cyan

            if ($decision -match "^RESPAWN") {
                Write-Host "    -> Would respawn (handled by server's auto-respawn)" -ForegroundColor Green
            }
            elseif ($decision -match "^ESCALATE") {
                Write-Host "    -> Needs attention" -ForegroundColor Yellow
            }

            $lastHeartbeatCheck[$agentId] = $info.last_seen
        }
    }
}

function Process-HumanRequests {
    $state = Get-ApiData "/api/state"

    if (-not $state -or -not $state.human_requests) {
        return
    }

    foreach ($requestId in $state.human_requests.PSObject.Properties.Name) {
        $request = $state.human_requests.$requestId

        # Skip already answered or processed
        if ($request.answered -or $processedAlerts[$requestId]) {
            continue
        }

        Write-Host ""
        Write-Host "  !!! HUMAN INPUT REQUESTED !!!" -ForegroundColor Red -BackgroundColor Yellow
        Write-Host "  Agent: $($request.agent_id)" -ForegroundColor White
        Write-Host "  Question: $($request.question)" -ForegroundColor White
        Write-Host "  Context: $($request.context)" -ForegroundColor DarkGray
        Write-Host ""

        $processedAlerts[$requestId] = $true
    }
}

# Main loop
$loopCount = 0
while ($true) {
    $loopCount++
    $timestamp = Get-Date -Format "HH:mm:ss"

    Write-Host "[$timestamp] Loop #$loopCount - Checking..." -ForegroundColor DarkGray

    try {
        # Check for pending stop requests
        Process-StopRequests

        # Check for stale agents (heartbeat monitoring)
        Process-StaleAgents

        # Check for human input requests
        Process-HumanRequests

    } catch {
        Write-Host "  [ERROR] Loop error: $_" -ForegroundColor Red
    }

    Start-Sleep -Seconds $PollIntervalSeconds
}
