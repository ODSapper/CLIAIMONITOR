# Notification System

The notification system provides multi-modal notifications for CLIAIMONITOR, with a focus on alerting users when supervisor input is needed.

## Overview

The notification system implements three independent notification channels:

1. **Windows Toast Notifications** - Native Windows 10+ toast popups with sound
2. **Terminal Title Flash** - Changes terminal window title to show alerts
3. **Dashboard Banner** - Red banner at the top of the web dashboard

All channels are managed through a unified `NotificationManager` interface.

## Architecture

```
NotificationManager (manager.go)
├── ToastNotifier (toast.go)        - Windows toast notifications
├── TerminalNotifier (terminal.go)  - Terminal title manipulation
└── BannerNotifier (banner.go)      - Dashboard banner state
```

## Quick Start

```go
import "github.com/CLIAIMONITOR/internal/notifications"

// Create a notification manager with default settings
manager := notifications.NewDefaultManager()

// Trigger all notification channels for supervisor alerts
err := manager.NotifySupervisorNeedsInput("Supervisor needs your approval")

// Individual notification types
manager.ShowToast("Title", "Message")
manager.FlashTerminal("Alert message")
manager.ShowDashboardBanner("Info message")

// Clear all active notifications
manager.ClearAlert()
```

## Components

### 1. Toast Notifications (toast.go)

Windows 10+ native toast notifications using the `go-toast` library.

**Features:**
- Sound alerts (configurable)
- Click action to open dashboard
- Windows-only (gracefully degrades on other platforms)

**Usage:**
```go
toast := notifications.NewToastNotifier("CLIAIMONITOR")
err := toast.ShowToast("Title", "Message")
err = toast.NotifySupervisorNeedsInput("Supervisor alert")
```

**Platform Support:**
- Windows 10+: Full support
- Linux/macOS: Returns error (not supported)

### 2. Terminal Title Flash (terminal.go)

Changes the terminal window title to show alerts using ANSI escape sequences.

**Features:**
- Thread-safe title management
- Restore original title
- Cross-platform support

**Usage:**
```go
terminal := notifications.NewTerminalNotifier()
terminal.SetOriginalTitle("My App")
terminal.FlashTerminal("Alert!")
terminal.RestoreTerminalTitle()
```

**Platform Support:**
- Windows: Modern terminals (Windows Terminal, ConEmu, etc.)
- Linux/macOS: Full support
- Requires terminal that supports ANSI escape sequences

### 3. Dashboard Banner (banner.go)

In-memory state manager for dashboard banner notifications.

**Features:**
- Four banner types: info, warning, error, supervisor
- Thread-safe state management
- Timestamp tracking

**Usage:**
```go
banner := notifications.NewBannerNotifier()
banner.Show("Message", "supervisor")
state := banner.GetState()
banner.Clear()
```

**Web Integration:**
The banner state is exposed via the notification manager and can be queried by HTTP handlers to display in the dashboard.

### 4. Notification Manager (manager.go)

Unified interface that coordinates all notification channels.

**Features:**
- Enable/disable all notifications
- Trigger multiple channels simultaneously
- Thread-safe operations
- Configurable channels

**Configuration:**
```go
config := notifications.Config{
    AppID:          "MyApp",
    DashboardURL:   "http://localhost:8080",
    EnableToast:    true,
    EnableTerminal: true,
    EnableBanner:   true,
    Logger:         log.Default(),
}
manager := notifications.NewManager(config)
```

## Interface

The main `NotificationManager` interface:

```go
type NotificationManager interface {
    NotifySupervisorNeedsInput(message string) error
    ShowToast(title, message string) error
    FlashTerminal(message string) error
    ShowDashboardBanner(message string) error
    ClearAlert() error
    IsEnabled() bool
}
```

## Web Dashboard Integration

The notification system integrates with the web dashboard through:

1. **Banner State API**: Expose `manager.GetBannerState()` via HTTP handler
2. **WebSocket Events**: Send notification events to connected clients
3. **JavaScript Controller**: `NotificationBanner` class in `app.js`

### Example HTTP Handler

```go
func (s *Server) handleGetBannerState(w http.ResponseWriter, r *http.Request) {
    state := s.notificationManager.GetBannerState()
    json.NewEncoder(w).Encode(state)
}
```

### Example WebSocket Notification

```go
// When supervisor needs input
s.notificationManager.NotifySupervisorNeedsInput("Approval required")

// Send WebSocket event to dashboard
event := map[string]interface{}{
    "type": "supervisor-needs-input",
    "message": "Approval required",
}
s.hub.Broadcast(event)
```

### JavaScript Usage

```javascript
// Show notification banner
window.notificationBanner.showSupervisorAlert("Supervisor needs input");

// Listen for WebSocket events
window.addEventListener('supervisor-needs-input', (event) => {
    window.notificationBanner.show(event.detail.message, 'supervisor');
});

// Clear banner
window.notificationBanner.clear();
```

## Testing

All components have comprehensive unit tests:

```bash
go test ./internal/notifications/... -v
```

**Test Coverage:**
- Thread safety tests (concurrent access)
- Platform-specific behavior
- State management
- Error handling

## Usage Examples

### Basic Supervisor Alert

```go
manager := notifications.NewDefaultManager()
err := manager.NotifySupervisorNeedsInput("Agent needs approval to proceed")
// This triggers:
// 1. Windows toast notification (if on Windows)
// 2. Terminal title flash
// 3. Dashboard banner (always)
```

### Conditional Notifications

```go
manager := notifications.NewDefaultManager()

// Disable all notifications temporarily
manager.Disable()

// Do work...

// Re-enable
manager.Enable()
```

### Custom Configuration

```go
config := notifications.Config{
    AppID:          "MyCustomApp",
    DashboardURL:   "http://localhost:9000",
    EnableToast:    runtime.GOOS == "windows", // Only on Windows
    EnableTerminal: true,
    EnableBanner:   true,
    Logger:         customLogger,
}
manager := notifications.NewManager(config)
```

### Individual Channel Control

```go
manager := notifications.NewDefaultManager()

// Only show dashboard banner (silent)
manager.ShowDashboardBanner("Background task completed")

// Only flash terminal (no toast)
manager.FlashTerminal("Build finished")

// Only show toast (no terminal flash)
manager.ShowToast("Deployment", "Deployment successful")
```

## Best Practices

1. **Always use the Manager**: Don't instantiate individual notifiers directly unless you have a specific reason.

2. **Handle Errors Gracefully**: Some notification types may fail on certain platforms. The manager continues with other channels even if one fails.

3. **Clear Alerts**: Always clear alerts when they're no longer relevant:
   ```go
   manager.ClearAlert()
   ```

4. **Set Terminal Title Early**: Set the original terminal title at application startup:
   ```go
   manager.SetTerminalTitle("CLIAIMONITOR v1.0")
   ```

5. **Thread Safety**: All components are thread-safe. Safe to call from multiple goroutines.

6. **Platform Detection**: Use `IsSupported()` methods to check if a notification type is available:
   ```go
   if toast.IsSupported() {
       toast.ShowToast("Title", "Message")
   }
   ```

## Troubleshooting

### Windows Toast Not Appearing

- Ensure you're on Windows 10 or later
- Check Windows notification settings
- Verify app has notification permissions
- Check if Focus Assist is blocking notifications

### Terminal Title Not Changing

- Verify you're using a terminal that supports ANSI escape sequences
- On Windows, ensure VT100 emulation is enabled
- Check if running in a true terminal (not redirected output)

### Banner Not Showing

- Verify WebSocket connection is active
- Check browser console for JavaScript errors
- Ensure banner HTML is present in index.html
- Check that notification banner CSS is loaded

## Dependencies

- `github.com/go-toast/toast` - Windows toast notifications
- Standard library only for other components

## Future Enhancements

Potential improvements for future versions:

1. **Email Notifications** - Send email alerts for critical events
2. **Sound Customization** - Custom sound files for different alert types
3. **Priority Levels** - Different notification behaviors based on priority
4. **Notification History** - Track and display recent notifications
5. **Desktop Notifications (Linux/macOS)** - Use native notification systems
6. **Mobile Push Notifications** - For remote monitoring

## License

Part of CLIAIMONITOR project.
