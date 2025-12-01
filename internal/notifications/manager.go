package notifications

import (
	"fmt"
	"log"
	"sync"
)

// NotificationManager provides a unified interface for all notification types
type NotificationManager interface {
	NotifySupervisorNeedsInput(message string) error
	ShowToast(title, message string) error
	FlashTerminal(message string) error
	ShowDashboardBanner(message string) error
	ClearAlert() error
	IsEnabled() bool
}

// Manager implements NotificationManager with multiple notification channels
type Manager struct {
	toast     *ToastNotifier
	terminal  *TerminalNotifier
	banner    *BannerNotifier
	enabled   bool
	mu        sync.RWMutex
	logger    *log.Logger
}

// Config holds configuration for the notification manager
type Config struct {
	AppID          string
	DashboardURL   string
	EnableToast    bool
	EnableTerminal bool
	EnableBanner   bool
	Logger         *log.Logger
}

// NewManager creates a new notification manager with all notification channels
func NewManager(config Config) *Manager {
	if config.Logger == nil {
		config.Logger = log.Default()
	}

	m := &Manager{
		toast:    NewToastNotifier(config.AppID),
		terminal: NewTerminalNotifier(),
		banner:   NewBannerNotifier(),
		enabled:  config.EnableToast || config.EnableTerminal || config.EnableBanner,
		logger:   config.Logger,
	}

	// Log which notification types are supported
	m.logSupport()

	return m
}

// NewDefaultManager creates a manager with default settings (all channels enabled)
func NewDefaultManager() *Manager {
	return NewManager(Config{
		AppID:          "CLIAIMONITOR",
		DashboardURL:   "http://localhost:8080",
		EnableToast:    true,
		EnableTerminal: true,
		EnableBanner:   true,
		Logger:         log.Default(),
	})
}

// NotifySupervisorNeedsInput triggers all notification channels for supervisor alerts
func (m *Manager) NotifySupervisorNeedsInput(message string) error {
	if !m.enabled {
		return fmt.Errorf("notifications are disabled")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	var errors []error

	// Send toast notification
	if m.toast.IsSupported() {
		if err := m.toast.NotifySupervisorNeedsInput(message); err != nil {
			m.logger.Printf("[NOTIFICATION] Toast notification failed: %v", err)
			errors = append(errors, fmt.Errorf("toast: %w", err))
		} else {
			m.logger.Printf("[NOTIFICATION] Toast notification sent: %s", message)
		}
	}

	// Flash terminal title
	if m.terminal.IsSupported() {
		if err := m.terminal.NotifySupervisorNeedsInput(message); err != nil {
			m.logger.Printf("[NOTIFICATION] Terminal notification failed: %v", err)
			errors = append(errors, fmt.Errorf("terminal: %w", err))
		} else {
			m.logger.Printf("[NOTIFICATION] Terminal title updated: %s", message)
		}
	}

	// Show dashboard banner
	if err := m.banner.Show(message, "supervisor"); err != nil {
		m.logger.Printf("[NOTIFICATION] Banner notification failed: %v", err)
		errors = append(errors, fmt.Errorf("banner: %w", err))
	} else {
		m.logger.Printf("[NOTIFICATION] Dashboard banner shown: %s", message)
	}

	if len(errors) > 0 {
		return fmt.Errorf("some notifications failed: %v", errors)
	}

	return nil
}

// ShowToast displays a Windows toast notification
func (m *Manager) ShowToast(title, message string) error {
	if !m.enabled {
		return fmt.Errorf("notifications are disabled")
	}

	if !m.toast.IsSupported() {
		return fmt.Errorf("toast notifications not supported on this platform")
	}

	err := m.toast.ShowToast(title, message)
	if err != nil {
		m.logger.Printf("[NOTIFICATION] Toast failed: %v", err)
		return err
	}

	m.logger.Printf("[NOTIFICATION] Toast sent: %s - %s", title, message)
	return nil
}

// FlashTerminal changes the terminal title to show a message
func (m *Manager) FlashTerminal(message string) error {
	if !m.enabled {
		return fmt.Errorf("notifications are disabled")
	}

	if !m.terminal.IsSupported() {
		return fmt.Errorf("terminal notifications not supported")
	}

	err := m.terminal.FlashTerminal(message)
	if err != nil {
		m.logger.Printf("[NOTIFICATION] Terminal flash failed: %v", err)
		return err
	}

	m.logger.Printf("[NOTIFICATION] Terminal title updated: %s", message)
	return nil
}

// ShowDashboardBanner displays a banner on the web dashboard
func (m *Manager) ShowDashboardBanner(message string) error {
	if !m.enabled {
		return fmt.Errorf("notifications are disabled")
	}

	err := m.banner.Show(message, "info")
	if err != nil {
		m.logger.Printf("[NOTIFICATION] Banner failed: %v", err)
		return err
	}

	m.logger.Printf("[NOTIFICATION] Dashboard banner shown: %s", message)
	return nil
}

// ClearAlert clears all active notifications
func (m *Manager) ClearAlert() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errors []error

	// Restore terminal title
	if m.terminal.IsSupported() {
		if err := m.terminal.ClearAlert(); err != nil {
			errors = append(errors, fmt.Errorf("terminal: %w", err))
		}
	}

	// Clear dashboard banner
	if err := m.banner.Clear(); err != nil {
		errors = append(errors, fmt.Errorf("banner: %w", err))
	}

	if len(errors) > 0 {
		return fmt.Errorf("some clear operations failed: %v", errors)
	}

	m.logger.Printf("[NOTIFICATION] All alerts cleared")
	return nil
}

// IsEnabled returns true if notifications are enabled
func (m *Manager) IsEnabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.enabled
}

// Enable enables all notifications
func (m *Manager) Enable() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enabled = true
	m.logger.Println("[NOTIFICATION] Notifications enabled")
}

// Disable disables all notifications
func (m *Manager) Disable() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enabled = false
	m.logger.Println("[NOTIFICATION] Notifications disabled")
}

// GetBannerState returns the current banner state (for web dashboard)
func (m *Manager) GetBannerState() BannerState {
	return m.banner.GetState()
}

// logSupport logs which notification channels are supported
func (m *Manager) logSupport() {
	m.logger.Printf("[NOTIFICATION] Toast notifications supported: %v", m.toast.IsSupported())
	m.logger.Printf("[NOTIFICATION] Terminal notifications supported: %v", m.terminal.IsSupported())
	m.logger.Printf("[NOTIFICATION] Banner notifications supported: true")
}

// SetTerminalTitle sets the original terminal title (should be called at startup)
func (m *Manager) SetTerminalTitle(title string) {
	m.terminal.SetOriginalTitle(title)
}
