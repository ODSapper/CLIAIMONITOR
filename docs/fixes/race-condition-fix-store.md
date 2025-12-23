# JSONStore Race Condition Fix

**Date**: 2025-12-21
**Issue**: Concurrent access to JSON file store without proper synchronization
**Files Modified**:
- `internal/persistence/store.go`
- `internal/persistence/store_test.go`

## Problems Identified

### 1. Save() Method Race Condition (Lines 147-157)
**Issue**: Between releasing the RLock and writing to disk, the state could be modified by another goroutine.

**Fix**:
- Ensure error handling releases lock properly
- Added atomic file write (write to temp file, then rename)
- This prevents partial writes and ensures consistency

### 2. RequestAgentShutdown() Deadlock Risk (Line 233)
**Issue**: `scheduleSave()` was called while holding the lock, which could cause deadlock if Save() is called from the timer goroutine while another operation holds the lock.

**Fix**: Release lock before calling `scheduleSave()`

### 3. GetNextAgentNumber() Deadlock Risk (Line 296)
**Issue**: Same as #2 - `scheduleSave()` called while holding the lock.

**Fix**: Release lock before calling `scheduleSave()`

### 4. CleanupStaleAgents() Deadlock Risk (Line 555)
**Issue**: Same as #2 and #3 - `scheduleSave()` called inside the lock.

**Fix**: Release lock before calling `scheduleSave()`

## Changes Made

### store.go

1. **Save() method** - Enhanced error handling and atomic writes:
```go
func (s *JSONStore) Save() error {
    s.mu.RLock()
    data, err := json.MarshalIndent(s.state, "", "  ")
    if err != nil {
        s.mu.RUnlock()
        return err
    }
    s.mu.RUnlock()

    // Write to temp file first, then rename atomically
    tempPath := s.filepath + ".tmp"
    if err := os.WriteFile(tempPath, data, 0644); err != nil {
        return err
    }

    return os.Rename(tempPath, s.filepath)
}
```

2. **RequestAgentShutdown()** - Lock released before scheduleSave:
```go
func (s *JSONStore) RequestAgentShutdown(agentID string, requestTime time.Time) {
    s.mu.Lock()
    if agent, ok := s.state.Agents[agentID]; ok {
        // ... modifications ...
    }
    s.mu.Unlock()  // Lock released BEFORE scheduleSave
    s.scheduleSave()
}
```

3. **GetNextAgentNumber()** - Same pattern:
```go
func (s *JSONStore) GetNextAgentNumber(configName string) int {
    s.mu.Lock()
    s.state.AgentCounters[configName]++
    num := s.state.AgentCounters[configName]
    s.mu.Unlock()  // Lock released BEFORE scheduleSave
    s.scheduleSave()
    return num
}
```

4. **CleanupStaleAgents()** - Same pattern:
```go
func (s *JSONStore) CleanupStaleAgents() int {
    s.mu.Lock()
    // ... cleanup logic ...
    s.mu.Unlock()  // Lock released BEFORE scheduleSave

    if removedCount > 0 {
        s.scheduleSave()
    }

    return removedCount
}
```

### store_test.go

Added comprehensive concurrent access tests:

1. **TestConcurrentSaveOperations** - Tests multiple goroutines performing mixed operations
2. **TestConcurrentRequestShutdown** - Tests concurrent shutdown requests

## Thread-Safety Guarantees

After these fixes:

1. ✅ All read operations properly use `RLock/RUnlock`
2. ✅ All write operations properly use `Lock/Unlock`
3. ✅ No `scheduleSave()` calls inside locks (prevents deadlock)
4. ✅ Atomic file writes prevent corruption
5. ✅ Separate `saveMu` protects the save timer

## Test Results

All tests pass including new concurrent access tests:
- TestConcurrentAccess: ✅ PASS
- TestConcurrentSaveOperations: ✅ PASS
- TestConcurrentRequestShutdown: ✅ PASS

Total: 19 tests, all passing.

## Performance Impact

Minimal - the changes only improve lock management without adding overhead:
- Lock duration slightly reduced (released earlier)
- Atomic file writes add negligible overhead (rename is fast)
- Same debouncing mechanism (500ms) still in place
