# Agent Auto-Start with Initial Task Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make spawned agents automatically start working by providing an initial prompt message via the `-p` flag.

**Architecture:** Add optional `task` field to spawn API, pass through spawner to launcher script, which adds `-p "message"` to the Claude command.

**Tech Stack:** Go, PowerShell, Claude CLI

---

## Task 1: Update Spawn API Handler

**Files:**
- Modify: `internal/server/handlers.go:68-120`

**Step 1: Add task field to spawn request struct**

In `handleSpawnAgent`, update the request struct at line 69:

```go
var req struct {
    ConfigName  string `json:"config_name"`
    ProjectPath string `json:"project_path"`
    Task        string `json:"task"` // Optional initial task for agent
}
```

**Step 2: Build initial prompt and pass to spawner**

After line 95, before calling `s.spawner.SpawnAgent`, build the initial prompt:

```go
// Build initial prompt if task provided
initialPrompt := ""
if req.Task != "" {
    initialPrompt = fmt.Sprintf("Your assigned task: %s\n\nBegin by calling register_agent to identify yourself, then start working on this task.", req.Task)
} else {
    initialPrompt = "You have been spawned. Call register_agent to identify yourself, then await further instructions from the supervisor."
}

// Spawn agent with initial prompt
pid, err := s.spawner.SpawnAgent(*agentConfig, agentID, projectPath, initialPrompt)
```

**Step 3: Run build to verify syntax**

Run: `go build ./cmd/cliaimonitor/`
Expected: Error - SpawnAgent signature mismatch (expected, we'll fix in Task 2)

**Step 4: Commit partial progress**

```bash
git add internal/server/handlers.go
git commit -m "feat: add task field to spawn API request"
```

---

## Task 2: Update Spawner Interface and Implementation

**Files:**
- Modify: `internal/agents/spawner.go:16-22` (interface)
- Modify: `internal/agents/spawner.go:171-229` (SpawnAgent method)

**Step 1: Update Spawner interface**

At line 17, add `initialPrompt` parameter:

```go
type Spawner interface {
    SpawnAgent(config types.AgentConfig, agentID string, projectPath string, initialPrompt string) (pid int, err error)
    SpawnSupervisor(config types.AgentConfig) (pid int, err error)
    StopAgent(agentID string) error
    IsAgentRunning(pid int) bool
    GetRunningAgents() map[string]int
}
```

**Step 2: Update SpawnAgent method signature**

At line 172, add `initialPrompt` parameter:

```go
func (s *ProcessSpawner) SpawnAgent(config types.AgentConfig, agentID string, projectPath string, initialPrompt string) (int, error) {
```

**Step 3: Pass initialPrompt to launcher script**

After line 204 (after `-SystemPromptPath`), add the initial prompt argument:

```go
args := []string{
    "-ExecutionPolicy", "Bypass",
    "-File", scriptPath,
    "-AgentID", agentID,
    "-AgentName", config.Name,
    "-Model", config.Model,
    "-Role", string(config.Role),
    "-Color", config.Color,
    "-ProjectPath", projectPath,
    "-MCPConfigPath", mcpConfigPath,
    "-SystemPromptPath", promptPath,
    "-InitialPrompt", initialPrompt,
}
```

**Step 4: Update SpawnSupervisor to pass default prompt**

At line 232-233, update to provide supervisor-specific initial prompt:

```go
func (s *ProcessSpawner) SpawnSupervisor(config types.AgentConfig) (int, error) {
    initialPrompt := "You are the Supervisor. Call register_agent to identify yourself, then begin your monitoring cycle."
    return s.SpawnAgent(config, "Supervisor", s.basePath, initialPrompt)
}
```

**Step 5: Run build to verify**

Run: `go build ./cmd/cliaimonitor/`
Expected: Build succeeds (PowerShell script will fail at runtime until Task 3)

**Step 6: Commit**

```bash
git add internal/agents/spawner.go
git commit -m "feat: pass initial prompt through spawner to launcher"
```

---

## Task 3: Update PowerShell Launcher Script

**Files:**
- Modify: `scripts/agent-launcher.ps1`

**Step 1: Add InitialPrompt parameter**

After line 27 (after `$SkipPermissions`), add:

```powershell
    [Parameter(Mandatory=$false)]
    [string]$InitialPrompt = ""
```

**Step 2: Build Claude command with -p flag**

Replace line 70 (the `claude` command) with:

```powershell
# Build the initial prompt flag if provided
`$initialPromptFlag = ""
if ('$InitialPrompt' -ne '') {
    `$initialPromptFlag = " -p '$InitialPrompt'"
}

# Launch Claude with the prompt
claude --model '$Model' --mcp-config '$MCPConfigPath'$skipPermissionsFlag`$initialPromptFlag --append-system-prompt `$promptContent
```

**Step 3: Test manually**

Run: `powershell -File scripts/agent-launcher.ps1 -AgentID TestAgent -AgentName Test -Model claude-sonnet-4-5-20250929 -Role "Go Developer" -Color "#00ff00" -ProjectPath "C:/Users/Admin/Documents/VS Projects/CLIAIMONITOR" -MCPConfigPath "C:/Users/Admin/Documents/VS Projects/CLIAIMONITOR/configs/mcp/Supervisor-mcp.json" -SystemPromptPath "C:/Users/Admin/Documents/VS Projects/CLIAIMONITOR/configs/prompts/go-developer.md" -InitialPrompt "Test message" -SkipPermissions`

Expected: Windows Terminal opens with Claude starting and the initial prompt displayed

**Step 4: Commit**

```bash
git add scripts/agent-launcher.ps1
git commit -m "feat: add InitialPrompt parameter to launcher script"
```

---

## Task 4: Rebuild and Test Full Flow

**Files:**
- None (testing only)

**Step 1: Stop existing server**

Run: `curl -X POST http://localhost:3000/api/shutdown` or kill the process

**Step 2: Rebuild**

Run: `go build -o cliaimonitor.exe ./cmd/cliaimonitor/`
Expected: Build succeeds

**Step 3: Start server**

Run: `./cliaimonitor.exe`
Expected: Server starts, supervisor spawns with initial prompt

**Step 4: Test spawn with task**

Run: `curl -X POST -H "Content-Type: application/json" -d '{"config_name":"SNTGreen","project_path":"C:/Users/Admin/Documents/VS Projects/MAH","task":"Fix MAH-P2-005 S3 backup storage build failure"}' http://localhost:3000/api/agents/spawn`

Expected: Agent spawns, Windows Terminal opens, agent immediately starts working on task

**Step 5: Verify agent registers and works**

Run: `curl http://localhost:3000/api/state | jq '.agents'`
Expected: Agent shows `status: "connected"` and has activity in activity_log

**Step 6: Commit all changes**

```bash
git add -A
git commit -m "feat: agents auto-start with initial task prompt

- Add 'task' field to /api/agents/spawn endpoint
- Pass initial prompt through spawner to launcher
- Claude now starts with -p flag containing task instructions
- Agents immediately begin work instead of waiting for input"
```

---

## Summary of Changes

| File | Change |
|------|--------|
| `internal/server/handlers.go` | Add `task` field to spawn request, build initial prompt |
| `internal/agents/spawner.go` | Add `initialPrompt` param to interface and implementation |
| `scripts/agent-launcher.ps1` | Add `-InitialPrompt` param, pass as `-p` flag to Claude |

## Expected Outcome

After implementation:
1. Spawn API accepts optional `task` field
2. Agents start with initial prompt message
3. Agents immediately call `register_agent` and begin work
4. No more idle agents waiting for user input
5. Stop approval protocol will be followed (agents have task context)
