# Agent Autonomy and Error Handling Fix

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix agent spawning so agents work autonomously without asking clarifying questions and capture errors from frozen agents.

**Architecture:**
1. Enhance initial prompts with explicit autonomy instructions and team context
2. Add error capture to launcher script for debugging frozen agents
3. Update prompt generation to suppress question-asking behavior

**Tech Stack:** Go, PowerShell, CLIAIMONITOR agent system

---

## Problem Analysis

### Issue 1: Agents asking for team ID
- Root cause: MAH's CLAUDE.md contains ecosystem task management instructions
- Agent sees "Before starting work, identify your team ID" and asks user
- Fix: Add explicit override in initial prompt + include team context in system prompt

### Issue 2: One agent frozen at "Starting Claude Code..."
- Root cause: Unknown - could be model loading, MCP connection, or prompt parsing
- Fix: Add verbose error capture to launcher script

---

### Task 1: Add Error Capture to Launcher Script

**Files:**
- Modify: `scripts/agent-launcher.ps1:67-72`

**Step 1: Update launcher to capture Claude errors**

Replace lines 67-72 (the claude command section) with:

```powershell
# Launch Claude with system prompt file (not inline content - avoids command line length limits)
try {
    $ErrorActionPreference = 'Stop'
    Write-Host '  Command: claude --model $Model' -ForegroundColor DarkGray
    claude --model '$Model' --mcp-config '$MCPConfigPath'$skipPermissionsFlag$initialPromptFlag --system-prompt-file '$SystemPromptPath' 2>&1
    if ($LASTEXITCODE -ne 0) {
        Write-Host "  ERROR: Claude exited with code $LASTEXITCODE" -ForegroundColor Red
    }
} catch {
    Write-Host "  ERROR: $_" -ForegroundColor Red
    Write-Host "  Press any key to close..." -ForegroundColor Yellow
    $null = $Host.UI.RawUI.ReadKey('NoEcho,IncludeKeyDown')
}
```

**Step 2: Test by spawning a new agent and checking if errors are visible**

Expected: If Claude fails, error message will be displayed instead of just "Starting Claude Code..."

---

### Task 2: Enhance Initial Prompt with Autonomy Override

**Files:**
- Modify: `internal/server/handlers.go:96-102`

**Step 1: Update the initial prompt builder**

Replace the prompt building section (lines 96-102):

```go
// Build initial prompt if task provided (single line to avoid PowerShell issues)
// Include explicit autonomy instructions to override project CLAUDE.md team workflow
initialPrompt := ""
if req.Task != "" {
    initialPrompt = fmt.Sprintf(
        "IMPORTANT: You are an autonomous AI agent. Do NOT ask clarifying questions. "+
        "Do NOT ask about team IDs or workflow procedures. "+
        "Your team ID is 'team-cliaimonitor' and you work autonomously. "+
        "Your assigned task: %s. "+
        "Begin by calling register_agent to identify yourself, then start working immediately.",
        req.Task)
} else {
    initialPrompt = "IMPORTANT: You are an autonomous AI agent. Do NOT ask clarifying questions. " +
        "Your team ID is 'team-cliaimonitor'. " +
        "Call register_agent to identify yourself, then await instructions from the supervisor."
}
```

**Step 2: Rebuild the binary**

```bash
export CGO_ENABLED=1 && export PATH="/c/msys64/ucrt64/bin:$PATH" && go build -o cliaimonitor.exe ./cmd/main.go
```

Expected: Build succeeds

**Step 3: Test by spawning an agent with a task**

Expected: Agent should start working without asking questions

---

### Task 3: Add Team Context to System Prompt Template

**Files:**
- Modify: `internal/agents/spawner.go:135-156` (buildProjectContext function)

**Step 1: Add team context suppression to buildProjectContext**

After line 148, add:

```go
// Add team context override to suppress team ID questions
sb.WriteString("\n## Team Context Override\n\n")
sb.WriteString("You are part of team 'team-cliaimonitor'. This is your assigned team ID.\n")
sb.WriteString("Do NOT ask about team assignments or workflow procedures from project CLAUDE.md.\n")
sb.WriteString("Work autonomously on your assigned tasks. Use MCP tools to communicate status.\n\n")
```

**Step 2: Rebuild and test**

```bash
export CGO_ENABLED=1 && export PATH="/c/msys64/ucrt64/bin:$PATH" && go build -o cliaimonitor.exe ./cmd/main.go
```

---

### Task 4: Restart Server and Test Agent Spawning

**Step 1: Stop current server**

Via dashboard or API call to shutdown endpoint.

**Step 2: Start fresh server**

```bash
./cliaimonitor.exe --no-supervisor
```

**Step 3: Spawn test agent with task**

```bash
curl -X POST http://localhost:3000/api/agents/spawn \
  -H "Content-Type: application/json" \
  -d '{"config_name":"SNTGreen","project_path":"C:/Users/Admin/Documents/VS Projects/MAH","task":"Test: Run go build and report the result"}'
```

**Step 4: Verify agent behavior**

- Check Windows Terminal tab opens
- Check agent doesn't ask for team ID
- Check agent calls register_agent
- Check agent starts working on task
- If error, check terminal for error message from Task 1 changes

---

## Verification Checklist

- [ ] Launcher script shows errors if Claude fails to start
- [ ] Initial prompt includes autonomy override
- [ ] System prompt includes team context override
- [ ] Agent starts without asking clarifying questions
- [ ] Agent calls register_agent on startup
- [ ] Agent begins working on assigned task

---

## Execution Ready

Plan complete and saved. Two execution options:

**1. Subagent-Driven (this session)** - I dispatch fresh subagent per task, review between tasks, fast iteration

**2. Parallel Session (separate)** - Open new session with executing-plans, batch execution with checkpoints

Which approach?
