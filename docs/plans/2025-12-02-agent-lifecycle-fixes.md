# Agent Lifecycle Fixes Plan

**Date**: 2025-12-02
**Issues**: Process killing closes all terminals, agents hang on spawn, redundant CONTEXT LOADED response

## Problem Analysis

### Issue 1: Process Killing Closes All Terminals (or Wrong Processes)

**Root Cause**: When we spawn an agent:
1. `SpawnAgent()` runs `powershell.exe -File agent-launcher.ps1`
2. The launcher script runs `Start-Process wt.exe new-tab ...` (or `powershell.exe`)
3. The PID returned is the **launcher script's PID**, not the Windows Terminal tab
4. The launcher script exits immediately after spawning WT
5. When we try to kill by PID, we're killing a dead/reused PID

**Current Flow**:
```
SpawnAgent() -> powershell launcher.ps1 (PID: 1234) -> wt.exe new-tab -> PowerShell tab (PID: ???) -> claude.exe (PID: ???)
                        ^                                                        ^
                        |                                                        |
                   We track this PID                              Actual process we want to control
                   (dies immediately)
```

**Why This Is Hard**:
- Windows Terminal spawns tabs as child processes of its main process
- We have no direct way to get the tab's PowerShell PID from `Start-Process wt.exe`
- Multiple agents = multiple tabs in same WT instance

### Issue 2: Agents Hang on Spawn

**Potential Causes**:
1. Claude waiting for input but getting garbled initial prompt
2. MCP server connection issues
3. System prompt injection failing
4. PowerShell script errors not visible

### Issue 3: Redundant CONTEXT LOADED Response

**Locations**:
- `internal/server/handlers.go:99` - handleSpawnAgent
- `internal/agents/spawner.go:300` - SpawnSupervisor
- `internal/captain/captain.go:340` - SpawnTeamAgent
- `internal/captain/supervisor.go:202` - getCaptainLaunchCommand
- `internal/supervisor/dispatcher.go:358` - SpawnTeamAgent
- `configs/prompts/snake.md` - Snake agent prompt
- `data/captain-prompt.md` - Captain prompt

---

## Solution Design

### Fix 1: Reliable Single-Agent Process Tracking

**Approach**: Track processes by **Window Title** instead of PID

Windows Terminal tabs get titles from the `-title` parameter. We can:
1. Set unique window titles: `CLIAIMONITOR-{AgentID}`
2. Find processes by window title using PowerShell
3. Kill specific tab by finding its process

**Implementation**:

1. **Modify `agent-launcher.ps1`**:
   - Set a unique, identifiable window title format: `CLIAIMONITOR-{AgentID}`
   - Write the actual PowerShell tab's PID to a file for tracking

2. **Add PID tracking file**:
   - Each agent writes its PID to `data/pids/{AgentID}.pid`
   - Main process reads this to get actual PID

3. **New termination approach**:
   - Read PID from file if available
   - Fallback: Find process by window title
   - Use `taskkill /PID` on the actual process

**New Flow**:
```
SpawnAgent() -> launcher.ps1 -> wt.exe new-tab (title: CLIAIMONITOR-Agent001)
                                    |
                                    v
                             PowerShell tab writes PID to data/pids/Agent001.pid
                                    |
                                    v
                             claude.exe runs

StopAgent() -> Read data/pids/Agent001.pid -> taskkill /PID {actual_pid}
           -> OR: Find by window title -> taskkill
```

### Fix 2: Agent Spawn Hanging

**Debugging steps**:
1. Add more verbose logging to launcher script
2. Ensure Claude is getting a clean initial prompt without escaping issues
3. Add timeout detection for agent registration

**Quick fix**: Remove the mandatory first response requirement - let agents start naturally

### Fix 3: Remove CONTEXT LOADED Requirement

**Changes**:
1. Remove `MANDATORY FIRST ACTION` from initial prompts
2. Keep system prompts simple - just identity and instructions
3. Trust MCP registration as the source of truth for agent identity

---

## Implementation Steps

### Step 1: Update agent-launcher.ps1

```powershell
# Set identifiable window title
$Host.UI.RawUI.WindowTitle = "CLIAIMONITOR-$AgentID"

# Write PID to tracking file after starting
$pidDir = Join-Path (Split-Path $ProjectPath -Parent) "CLIAIMONITOR\data\pids"
if (-not (Test-Path $pidDir)) { New-Item -ItemType Directory -Path $pidDir -Force }
$PID | Out-File -FilePath (Join-Path $pidDir "$AgentID.pid") -Encoding ASCII
```

### Step 2: Add PID file reading to spawner.go

```go
func (s *ProcessSpawner) GetAgentPID(agentID string) (int, error) {
    pidFile := filepath.Join(s.basePath, "data", "pids", agentID+".pid")
    data, err := os.ReadFile(pidFile)
    if err != nil {
        return 0, err
    }
    return strconv.Atoi(strings.TrimSpace(string(data)))
}
```

### Step 3: Update StopAgentWithReason to use PID file

```go
func (s *ProcessSpawner) StopAgentWithReason(agentID string, reason string) error {
    // Try to get actual PID from file
    pid, err := s.GetAgentPID(agentID)
    if err == nil {
        instance.KillProcess(pid)
    } else {
        // Fallback: kill by window title
        s.killByWindowTitle(agentID)
    }
    // ... rest of cleanup
}
```

### Step 4: Add window title fallback killer

```go
func (s *ProcessSpawner) killByWindowTitle(agentID string) error {
    title := fmt.Sprintf("CLIAIMONITOR-%s", agentID)
    cmd := exec.Command("powershell.exe", "-Command",
        fmt.Sprintf(`Get-Process | Where-Object {$_.MainWindowTitle -eq '%s'} | Stop-Process -Force`, title))
    return cmd.Run()
}
```

### Step 5: Remove CONTEXT LOADED from all locations

Update these files to remove the mandatory first response:
- `internal/server/handlers.go`
- `internal/agents/spawner.go`
- `internal/captain/captain.go`
- `internal/captain/supervisor.go`
- `internal/supervisor/dispatcher.go`
- `configs/prompts/snake.md`
- `data/captain-prompt.md`

---

## Files to Modify

1. `scripts/agent-launcher.ps1` - Add PID tracking, set unique window title
2. `internal/agents/spawner.go` - Add PID file reading, window title fallback
3. `internal/server/handlers.go` - Remove CONTEXT LOADED from spawn
4. `internal/captain/captain.go` - Remove CONTEXT LOADED
5. `internal/captain/supervisor.go` - Remove CONTEXT LOADED
6. `internal/supervisor/dispatcher.go` - Remove CONTEXT LOADED
7. `configs/prompts/snake.md` - Remove CONTEXT LOADED section
8. `data/captain-prompt.md` - Remove CONTEXT LOADED section

---

## Testing Plan

1. Spawn single agent - verify PID file created
2. Stop single agent - verify only that agent's tab closes
3. Spawn multiple agents - verify independent termination
4. Test graceful shutdown timeout
5. Test cleanup of stale agents
