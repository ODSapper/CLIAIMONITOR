# WezTerm Pane ID Tracking Implementation

## Overview
Updated the agent spawner to capture and track WezTerm pane IDs for more reliable agent process management.

## Changes Made

### 1. Data Structure Updates (`spawner.go`)

Added `agentPanes` map to track pane IDs:
```go
type ProcessSpawner struct {
    // ... existing fields ...
    agentPanes     map[string]int // agentID -> WezTerm pane ID
}
```

### 2. New Methods

#### `GetAgentPaneID(agentID string) (int, bool)`
Retrieves the stored WezTerm pane ID for an agent.

#### `SetAgentPaneID(agentID string, paneID int)`
Stores the WezTerm pane ID for an agent (thread-safe).

#### `KillByPaneID(paneID int) error`
Kills a WezTerm pane using `wezterm cli kill-pane --pane-id <id>`.

### 3. Spawn Process Updates

The `SpawnAgent` method now:

1. **Attempts to use `wezterm cli spawn`** to capture pane ID:
   ```bash
   wezterm cli spawn --new-window --cwd <path> -- cmd.exe /k <commands>
   ```
   - Returns the pane ID on stdout
   - Requires a running WezTerm mux server

2. **Fallback to `wezterm start`** if cli spawn fails:
   ```bash
   wezterm start --always-new-process --cwd <path> -- cmd.exe /k <commands>
   ```
   - Works without mux server
   - Doesn't return pane ID (sets to -1)

3. **Stores pane ID** if successfully captured:
   ```go
   if paneID > 0 {
       s.SetAgentPaneID(agentID, paneID)
   }
   ```

### 4. Stop Process Updates

The `StopAgentWithReason` method now uses a priority-based cleanup approach:

1. **Primary method**: Kill by WezTerm pane ID (most reliable)
   ```go
   if paneID, ok := s.GetAgentPaneID(agentID); ok && paneID > 0 {
       s.KillByPaneID(paneID)
   }
   ```

2. **Fallback methods** (executed even if pane kill succeeds):
   - Kill by PID file
   - Kill by window title
   - Kill by temp script name

### 5. Cleanup Updates

Updated `RemoveAgent` to clean up both PID and pane ID tracking:
```go
func (s *ProcessSpawner) RemoveAgent(agentID string) {
    s.mu.Lock()
    delete(s.runningAgents, agentID)
    delete(s.agentPanes, agentID)  // NEW
    s.mu.Unlock()
}
```

## Benefits

1. **More Reliable Cleanup**: WezTerm's native pane killing is more reliable than process-based approaches
2. **Cross-Platform Potential**: Pane IDs work consistently across different shells/terminals
3. **Graceful Degradation**: Falls back to traditional methods if pane ID unavailable
4. **Better Tracking**: Pane IDs provide direct terminal window management

## Testing

Added comprehensive tests:
- `TestGetAgentPaneID` - Verify pane ID retrieval
- `TestSetAgentPaneID` - Verify pane ID storage and updates
- Updated `TestRemoveAgent` - Verify pane cleanup
- Updated `TestNewSpawner` - Verify map initialization

All new tests pass successfully.

## Usage Notes

### When Pane IDs Are Available
- Running from within a WezTerm window (mux server active)
- Agents get reliable pane IDs for cleanup

### When Pane IDs Are Not Available
- First agent spawn (no mux server yet)
- Running from external terminals
- System falls back to PID-based cleanup

### Logging
The spawner now logs:
```
[SPAWNER] Agent team-sntgreen001 spawned in pane 42
[SPAWNER] Stored pane ID 42 for agent team-sntgreen001
[SPAWNER] Agent team-sntgreen001 launched in WezTerm (PID: 12345, Pane: 42)
[SPAWNER] Killing agent team-sntgreen001 via pane ID 42
[SPAWNER] Successfully killed pane 42
```

## Future Enhancements

1. **Persist pane IDs**: Store in database or PID files for recovery after restart
2. **Auto-reconnect**: Use `wezterm cli list` to rediscover panes for running agents
3. **Health checks**: Verify pane existence using `wezterm cli list`
4. **Multi-pane support**: Track multiple panes per agent (splits, tabs)

## Files Modified

- `internal/agents/spawner.go` - Core implementation
- `internal/agents/spawner_test.go` - Test coverage

## Compatibility

- **Minimum WezTerm version**: Any version with `cli` subcommand support
- **Backward compatible**: Falls back gracefully on older setups
- **No breaking changes**: Existing spawn behavior preserved
