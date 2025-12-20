# WezTerm Pane ID Tracking Flow

## Agent Spawn Flow

```
┌─────────────────────────────────────────────────────────────┐
│ SpawnAgent(config, agentID, projectPath, initialPrompt)    │
└─────────────────────────┬───────────────────────────────────┘
                          │
                          ▼
          ┌───────────────────────────────┐
          │ Try: wezterm cli spawn        │
          │   --new-window --cwd <path>   │
          │   -- cmd.exe /k <commands>    │
          └───────────┬───────────────────┘
                      │
           ┌──────────┴──────────┐
           │                     │
           ▼ SUCCESS             ▼ FAIL (no mux server)
    ┌──────────────┐      ┌─────────────────────┐
    │ Parse paneID │      │ Fallback:           │
    │ from stdout  │      │ wezterm start       │
    │              │      │ --always-new-process│
    └──────┬───────┘      └─────────┬───────────┘
           │                        │
           ▼                        ▼
    ┌──────────────┐         ┌────────────┐
    │ paneID = N   │         │ paneID = -1│
    └──────┬───────┘         └─────┬──────┘
           │                       │
           └───────────┬───────────┘
                       │
                       ▼
           ┌───────────────────────────┐
           │ if paneID > 0:            │
           │   SetAgentPaneID(agentID, │
           │                   paneID) │
           └───────────┬───────────────┘
                       │
                       ▼
           ┌───────────────────────────┐
           │ Log: "Agent spawned in    │
           │       pane N"             │
           └───────────────────────────┘
```

## Agent Stop Flow

```
┌─────────────────────────────────────────────────────────────┐
│ StopAgentWithReason(agentID, reason)                        │
└─────────────────────────┬───────────────────────────────────┘
                          │
                          ▼
          ┌───────────────────────────────┐
          │ 1. Set shutdown flag in DB    │
          └───────────┬───────────────────┘
                      │
                      ▼
          ┌───────────────────────────────┐
          │ 2. Kill heartbeat script      │
          └───────────┬───────────────────┘
                      │
                      ▼
          ┌───────────────────────────────┐
          │ 3. Mark agent stopped in DB   │
          └───────────┬───────────────────┘
                      │
                      ▼
          ┌───────────────────────────────┐
          │ 4. Remove from runningAgents  │
          └───────────┬───────────────────┘
                      │
                      ▼
     ┌────────────────────────────────────────┐
     │ 5. PRIMARY: Kill by pane ID            │
     │    if paneID, ok := GetAgentPaneID()   │
     └────────────┬───────────────────────────┘
                  │
      ┌───────────┴──────────┐
      │                      │
      ▼ paneID exists        ▼ no paneID
┌──────────────────┐   ┌────────────┐
│ wezterm cli      │   │ Skip       │
│ kill-pane        │   │            │
│ --pane-id N      │   │            │
└─────┬────────────┘   └─────┬──────┘
      │                      │
      ▼                      │
┌──────────────────┐         │
│ delete from      │         │
│ agentPanes map   │         │
└─────┬────────────┘         │
      │                      │
      └──────────┬───────────┘
                 │
                 ▼
     ┌───────────────────────────────┐
     │ 6. FALLBACK: Kill by PID file │
     │    - Read PID from file       │
     │    - Kill claude.exe children │
     │    - Kill PowerShell process  │
     └───────────┬───────────────────┘
                 │
                 ▼
     ┌───────────────────────────────┐
     │ 7. FALLBACK: Kill by window   │
     │    title (CLIAIMONITOR-{id})  │
     └───────────┬───────────────────┘
                 │
                 ▼
     ┌───────────────────────────────┐
     │ 8. FALLBACK: Kill by temp     │
     │    script name                │
     └───────────────────────────────┘
```

## Data Flow

```
┌─────────────────────────────────────────────────────┐
│ ProcessSpawner Struct                               │
│ ┌─────────────────────────────────────────────────┐ │
│ │ runningAgents map[string]int  // agentID -> PID│ │
│ │ agentPanes    map[string]int  // agentID -> Pane│ │
│ │ agentCounters map[string]int  // type -> seq   │ │
│ │ heartbeatPIDs map[string]int  // agentID -> PID│ │
│ └─────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────┘

When agent spawns:
  agentPanes["team-sntgreen001"] = 42

When getting pane:
  paneID, ok := GetAgentPaneID("team-sntgreen001")
  // Returns: (42, true)

When stopping agent:
  1. Kill by pane ID: wezterm cli kill-pane --pane-id 42
  2. delete(agentPanes, "team-sntgreen001")

When removing agent:
  1. delete(runningAgents, agentID)
  2. delete(agentPanes, agentID)  ← NEW
```

## Thread Safety

All map operations are protected by mutex:

```go
// Reading
s.mu.RLock()
paneID, ok := s.agentPanes[agentID]
s.mu.RUnlock()

// Writing
s.mu.Lock()
s.agentPanes[agentID] = paneID
s.mu.Unlock()
```

## Error Handling

```
wezterm cli spawn fails
        ↓
Log: "wezterm cli spawn failed, falling back to wezterm start"
        ↓
Use wezterm start instead
        ↓
Set paneID = -1 (indicates no pane tracking)
        ↓
Continue normal spawn flow
```

## Cleanup Priority

1. **Pane ID** (Most reliable, direct terminal control)
2. **PID File** (Process tracking)
3. **Window Title** (Pattern matching)
4. **Script Name** (Command line matching)

Each method is attempted regardless of previous success to ensure thorough cleanup.
