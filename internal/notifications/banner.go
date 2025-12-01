package notifications

import (
	"sync"
	"time"
)

// BannerType represents the type/severity of a banner notification
type BannerType string

const (
	BannerTypeInfo       BannerType = "info"
	BannerTypeWarning    BannerType = "warning"
	BannerTypeError      BannerType = "error"
	BannerTypeSupervisor BannerType = "supervisor"
)

// BannerState holds the current state of the banner notification
type BannerState struct {
	Visible   bool       `json:"visible"`
	Message   string     `json:"message"`
	Type      BannerType `json:"type"`
	Timestamp time.Time  `json:"timestamp"`
}

// BannerNotifier manages the dashboard banner notification state
type BannerNotifier struct {
	state BannerState
	mu    sync.RWMutex
}

// NewBannerNotifier creates a new banner notifier
func NewBannerNotifier() *BannerNotifier {
	return &BannerNotifier{
		state: BannerState{
			Visible: false,
		},
	}
}

// Show displays a banner with the specified message and type
func (b *BannerNotifier) Show(message string, bannerType string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.state = BannerState{
		Visible:   true,
		Message:   message,
		Type:      BannerType(bannerType),
		Timestamp: time.Now(),
	}

	return nil
}

// ShowSupervisorAlert displays a supervisor-specific banner (red, high priority)
func (b *BannerNotifier) ShowSupervisorAlert(message string) error {
	return b.Show(message, string(BannerTypeSupervisor))
}

// Clear hides the banner
func (b *BannerNotifier) Clear() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.state.Visible = false
	return nil
}

// GetState returns the current banner state (thread-safe)
func (b *BannerNotifier) GetState() BannerState {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// Return a copy to prevent race conditions
	return BannerState{
		Visible:   b.state.Visible,
		Message:   b.state.Message,
		Type:      b.state.Type,
		Timestamp: b.state.Timestamp,
	}
}

// IsVisible returns true if the banner is currently visible
func (b *BannerNotifier) IsVisible() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.state.Visible
}

// GetMessage returns the current banner message
func (b *BannerNotifier) GetMessage() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.state.Message
}

// GetType returns the current banner type
func (b *BannerNotifier) GetType() BannerType {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.state.Type
}
