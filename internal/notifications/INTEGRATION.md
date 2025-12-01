# Notification System Integration Guide

This guide shows how to integrate the notification system into CLIAIMONITOR's existing codebase.

## Overview

The notification system is **self-contained** and does not require modifications to existing supervisor or agent code. It provides a clean interface for triggering notifications when supervisor input is needed.

## Integration Steps

### 1. Add Notification Manager to Server

Modify `internal/server/server.go` to include the notification manager:

```go
import "github.com/CLIAIMONITOR/internal/notifications"

type Server struct {
    // ... existing fields ...
    notificationManager notifications.NotificationManager
}

func NewServer(config Config) *Server {
    s := &Server{
        // ... existing initialization ...
        notificationManager: notifications.NewDefaultManager(),
    }

    // Set terminal title at startup
    s.notificationManager.SetTerminalTitle("CLIAIMONITOR")

    return s
}
```

### 2. Add HTTP Endpoint for Banner State

Add endpoint to retrieve banner state for the dashboard:

```go
// In internal/server/handlers.go or routes setup

func (s *Server) handleGetBannerState(w http.ResponseWriter, r *http.Request) {
    state := s.notificationManager.GetBannerState()

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(state)
}

// Register route
router.HandleFunc("/api/banner/state", s.handleGetBannerState).Methods("GET")
```

### 3. Add Banner Clear Endpoint

Add endpoint to allow users to dismiss the banner:

```go
func (s *Server) handleClearBanner(w http.ResponseWriter, r *http.Request) {
    err := s.notificationManager.ClearAlert()
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// Register route
router.HandleFunc("/api/banner/clear", s.handleClearBanner).Methods("POST")
```

### 4. Trigger Notifications When Supervisor Needs Input

In your existing supervisor interaction code:

```go
// Example: When agent requests approval
func (s *Server) handleAgentApprovalRequest(agentID string, request ApprovalRequest) {
    // Existing code to record the request...

    // Trigger notifications
    message := fmt.Sprintf("Agent %s needs approval: %s", agentID, request.Action)
    err := s.notificationManager.NotifySupervisorNeedsInput(message)
    if err != nil {
        log.Printf("Failed to send notification: %v", err)
    }

    // Send WebSocket update to dashboard
    s.hub.Broadcast(map[string]interface{}{
        "type":    "supervisor-needs-input",
        "message": message,
        "agentID": agentID,
    })
}
```

### 5. Clear Notifications After Response

When supervisor responds to a request:

```go
func (s *Server) handleSupervisorResponse(requestID string, approved bool) {
    // Process the response...

    // Clear all active notifications
    err := s.notificationManager.ClearAlert()
    if err != nil {
        log.Printf("Failed to clear notification: %v", err)
    }

    // Notify dashboard via WebSocket
    s.hub.Broadcast(map[string]interface{}{
        "type": "notification-cleared",
    })
}
```

### 6. Update WebSocket Handler

Update the dashboard's WebSocket message handler to trigger banner notifications:

```javascript
// In web/app.js, update the WebSocket message handler

handleWebSocketMessage(data) {
    switch(data.type) {
        case 'supervisor-needs-input':
            // Show notification banner
            window.notificationBanner.showSupervisorAlert(data.message);
            break;

        case 'notification-cleared':
            // Clear banner
            window.notificationBanner.clear();
            break;

        // ... existing cases ...
    }
}
```

### 7. Add Polling for Banner State (Optional)

If you want to ensure banner state persists across page reloads:

```javascript
// In web/app.js, add polling for banner state

class Dashboard {
    constructor() {
        // ... existing code ...
        this.pollBannerState();
    }

    async pollBannerState() {
        try {
            const response = await fetch('/api/banner/state');
            const state = await response.json();

            if (state.visible) {
                window.notificationBanner.show(state.message, state.type, false);
            }
        } catch (error) {
            console.error('Failed to fetch banner state:', error);
        }

        // Poll every 5 seconds
        setTimeout(() => this.pollBannerState(), 5000);
    }
}
```

### 8. Add Dismiss Button Handler

Connect the dismiss button to the API:

```javascript
// In web/app.js

class NotificationBanner {
    initEventListeners() {
        this.dismissBtn.addEventListener('click', async () => {
            // Call API to clear notification
            try {
                await fetch('/api/banner/clear', { method: 'POST' });
            } catch (error) {
                console.error('Failed to clear banner:', error);
            }

            this.hide();
        });
    }
}
```

## Configuration Options

### Custom Notification Behavior

```go
// In server initialization
config := notifications.Config{
    AppID:          "CLIAIMONITOR",
    DashboardURL:   "http://localhost:8080",
    EnableToast:    runtime.GOOS == "windows", // Only on Windows
    EnableTerminal: true,
    EnableBanner:   true,
    Logger:         s.logger,
}
s.notificationManager = notifications.NewManager(config)
```

### Disable Notifications During Testing

```go
// In test setup
func TestSomething(t *testing.T) {
    server := NewTestServer()
    server.notificationManager.Disable()

    // Run tests without notifications
    // ...
}
```

## Example: Complete Integration

Here's a complete example showing notification flow:

```go
// 1. Agent requests approval
func (s *Server) onAgentStopRequest(agentID, reason string) {
    // Log the request
    log.Printf("[AGENT %s] Stop approval required: %s", agentID, reason)

    // Store request in database/memory
    s.pendingApprovals[agentID] = ApprovalRequest{
        AgentID:   agentID,
        Reason:    reason,
        Timestamp: time.Now(),
    }

    // Trigger all notification channels
    message := fmt.Sprintf("Agent %s requests stop approval: %s", agentID, reason)
    s.notificationManager.NotifySupervisorNeedsInput(message)

    // Notify dashboard via WebSocket
    s.hub.Broadcast(map[string]interface{}{
        "type":    "supervisor-needs-input",
        "agentID": agentID,
        "message": message,
    })
}

// 2. Supervisor responds via dashboard
func (s *Server) handleApprovalResponse(w http.ResponseWriter, r *http.Request) {
    var req struct {
        AgentID  string `json:"agent_id"`
        Approved bool   `json:"approved"`
    }

    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    // Process approval
    approval, exists := s.pendingApprovals[req.AgentID]
    if !exists {
        http.Error(w, "No pending approval", http.StatusNotFound)
        return
    }

    // Clear notifications
    s.notificationManager.ClearAlert()

    // Notify agent of decision
    s.sendAgentResponse(req.AgentID, req.Approved)

    // Notify dashboard
    s.hub.Broadcast(map[string]interface{}{
        "type":     "approval-processed",
        "agentID":  req.AgentID,
        "approved": req.Approved,
    })

    // Clean up
    delete(s.pendingApprovals, req.AgentID)

    w.WriteHeader(http.StatusOK)
}
```

## Testing the Integration

### Manual Testing

1. Start the server: `go run cmd/cliaimonitor/main.go`
2. Open dashboard: `http://localhost:8080`
3. Trigger a supervisor alert (via API or agent action)
4. Verify:
   - Windows toast appears (on Windows)
   - Terminal title changes
   - Red banner appears at top of dashboard
5. Click dismiss button or respond to request
6. Verify all notifications clear

### Unit Testing

```go
func TestNotificationIntegration(t *testing.T) {
    server := NewTestServer()

    // Trigger notification
    err := server.notificationManager.NotifySupervisorNeedsInput("Test alert")
    if err != nil {
        t.Fatalf("Failed to send notification: %v", err)
    }

    // Check banner state
    state := server.notificationManager.GetBannerState()
    if !state.Visible {
        t.Error("Expected banner to be visible")
    }

    // Clear notification
    err = server.notificationManager.ClearAlert()
    if err != nil {
        t.Fatalf("Failed to clear alert: %v", err)
    }

    // Verify cleared
    state = server.notificationManager.GetBannerState()
    if state.Visible {
        t.Error("Expected banner to be cleared")
    }
}
```

## Troubleshooting

### Notifications Not Appearing

1. Check if notifications are enabled:
   ```go
   if !server.notificationManager.IsEnabled() {
       log.Println("Notifications are disabled")
   }
   ```

2. Check logs for errors:
   ```
   [NOTIFICATION] Toast notification failed: ...
   [NOTIFICATION] Terminal notification failed: ...
   ```

3. Verify WebSocket connection is active

### Banner Not Clearing

1. Check dismiss button is wired to `/api/banner/clear`
2. Verify HTTP POST request is successful
3. Check server logs for errors

### Toast Not Showing on Windows

1. Check Windows notification settings
2. Verify Focus Assist is not blocking
3. Check if running on Windows 10+

## Best Practices

1. **Always clear notifications** after supervisor responds
2. **Use meaningful messages** that explain what action is needed
3. **Don't spam notifications** - rate limit or debounce frequent alerts
4. **Test on multiple platforms** to ensure graceful degradation
5. **Handle errors gracefully** - log but don't crash on notification failures

## Future Enhancements

Once integrated, consider adding:

1. **Notification history** - Track and display past notifications
2. **Custom sounds** - Different sounds for different alert types
3. **Notification preferences** - Let users configure which channels they want
4. **Multiple supervisors** - Route notifications to specific supervisors
5. **Mobile notifications** - Push notifications for remote monitoring

## Summary

The notification system is designed to be:

- **Self-contained**: No modifications to existing agent/supervisor code
- **Multi-modal**: Toast + Terminal + Dashboard for maximum visibility
- **Flexible**: Easy to enable/disable or customize
- **Thread-safe**: Safe to use from multiple goroutines
- **Testable**: Comprehensive test coverage

Integration requires only:
1. Adding notification manager to server
2. Calling `NotifySupervisorNeedsInput()` when needed
3. Calling `ClearAlert()` after response
4. Adding HTTP endpoints for banner state

The notification system will handle the rest!
