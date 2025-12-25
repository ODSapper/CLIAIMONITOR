package quotes

import (
	"encoding/json"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
)

// QuotesConfig holds all quote categories
type QuotesConfig struct {
	Spawn    []string `json:"spawn"`
	Shutdown []string `json:"shutdown"`
	Hourly   []string `json:"hourly"`
}

// Manager handles quote loading and retrieval
type Manager struct {
	mu       sync.RWMutex
	config   QuotesConfig
	basePath string
	loaded   bool
}

// Default quotes (fallback if JSON not found)
var defaultQuotes = QuotesConfig{
	Spawn: []string{
		"Unit ready.",
		"Acknowledged.",
		"Standing by.",
		"Online and operational.",
		"Reporting for duty.",
	},
	Shutdown: []string{
		"Mission complete.",
		"Signing off.",
		"Task finished.",
		"Going offline.",
		"Work complete.",
	},
	Hourly: []string{
		"All systems nominal.",
		"Holding the line.",
		"Standing watch.",
		"Status: operational.",
		"The watch continues.",
	},
}

var (
	globalManager *Manager
	once          sync.Once
)

// Init initializes the global quotes manager with the base path
func Init(basePath string) {
	once.Do(func() {
		globalManager = &Manager{
			basePath: basePath,
			config:   defaultQuotes,
		}
		globalManager.Load()
	})
}

// GetManager returns the global quotes manager
func GetManager() *Manager {
	if globalManager == nil {
		// Fallback with defaults only
		globalManager = &Manager{
			config: defaultQuotes,
			loaded: true,
		}
	}
	return globalManager
}

// Load loads quotes from the JSON config file
func (m *Manager) Load() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	quotesPath := filepath.Join(m.basePath, "configs", "quotes.json")
	data, err := os.ReadFile(quotesPath)
	if err != nil {
		log.Printf("[QUOTES] Using default quotes (config not found: %v)", err)
		m.config = defaultQuotes
		m.loaded = true
		return nil
	}

	var config QuotesConfig
	if err := json.Unmarshal(data, &config); err != nil {
		log.Printf("[QUOTES] Error parsing quotes.json: %v, using defaults", err)
		m.config = defaultQuotes
		m.loaded = true
		return err
	}

	// Validate and merge with defaults if categories are empty
	if len(config.Spawn) == 0 {
		config.Spawn = defaultQuotes.Spawn
	}
	if len(config.Shutdown) == 0 {
		config.Shutdown = defaultQuotes.Shutdown
	}
	if len(config.Hourly) == 0 {
		config.Hourly = defaultQuotes.Hourly
	}

	m.config = config
	m.loaded = true
	log.Printf("[QUOTES] Loaded %d spawn, %d shutdown, %d hourly quotes",
		len(config.Spawn), len(config.Shutdown), len(config.Hourly))
	return nil
}

// Reload reloads quotes from disk (call this to pick up changes)
func (m *Manager) Reload() error {
	return m.Load()
}

// GetSpawnQuote returns a random spawn quote
func (m *Manager) GetSpawnQuote() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.config.Spawn) == 0 {
		return "Ready."
	}
	return m.config.Spawn[rand.Intn(len(m.config.Spawn))]
}

// GetShutdownQuote returns a random shutdown quote
func (m *Manager) GetShutdownQuote() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.config.Shutdown) == 0 {
		return "Goodbye."
	}
	return m.config.Shutdown[rand.Intn(len(m.config.Shutdown))]
}

// GetHourlyQuote returns a random hourly status quote
func (m *Manager) GetHourlyQuote() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.config.Hourly) == 0 {
		return "Still running."
	}
	return m.config.Hourly[rand.Intn(len(m.config.Hourly))]
}

// Convenience functions for global manager

// SpawnQuote returns a random spawn quote from the global manager
func SpawnQuote() string {
	return GetManager().GetSpawnQuote()
}

// ShutdownQuote returns a random shutdown quote from the global manager
func ShutdownQuote() string {
	return GetManager().GetShutdownQuote()
}

// HourlyQuote returns a random hourly quote from the global manager
func HourlyQuote() string {
	return GetManager().GetHourlyQuote()
}
