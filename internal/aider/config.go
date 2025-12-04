package aider

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// AiderConfig holds configuration for Aider CLI
type AiderConfig struct {
	Model          string `yaml:"model" json:"model"`                       // e.g., "ollama/qwen2.5-coder:32b"
	OllamaURL      string `yaml:"ollama_url" json:"ollama_url"`             // e.g., "http://localhost:11434"
	AutoCommit     bool   `yaml:"auto_commit" json:"auto_commit"`           // false recommended
	EditFormat     string `yaml:"edit_format" json:"edit_format"`           // whole, diff, udiff
	MapTokens      int    `yaml:"map_tokens" json:"map_tokens"`             // repo map token limit
	MaxChatHistory int    `yaml:"max_chat_history" json:"max_chat_history"` // chat history limit
}

// AgentConfig holds configuration for a single Aider agent
type AgentConfig struct {
	Name        string `yaml:"name" json:"name"`
	Role        string `yaml:"role" json:"role"`
	Color       string `yaml:"color" json:"color"`
	ProjectPath string `yaml:"project_path" json:"project_path"`
}

// SergeantConfig holds configuration for the Aider Sergeant
type SergeantConfig struct {
	MaxConcurrentAgents int `yaml:"max_concurrent_agents" json:"max_concurrent_agents"`
	IdleTimeout         int `yaml:"idle_timeout" json:"idle_timeout"` // seconds
}

// Config is the root configuration for CLIAIR
type Config struct {
	Server   ServerConfig   `yaml:"server" json:"server"`
	Ollama   OllamaConfig   `yaml:"ollama" json:"ollama"`
	Aider    AiderConfig    `yaml:"aider" json:"aider"`
	Agents   []AgentConfig  `yaml:"agents" json:"agents"`
	Sergeant SergeantConfig `yaml:"sergeant" json:"sergeant"`
}

// ServerConfig holds server settings
type ServerConfig struct {
	Port     int `yaml:"port" json:"port"`
	NATSPort int `yaml:"nats_port" json:"nats_port"`
}

// OllamaConfig holds Ollama settings
type OllamaConfig struct {
	URL   string `yaml:"url" json:"url"`
	Model string `yaml:"model" json:"model"`
}

// DefaultConfig returns default CLIAIR configuration
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port:     3001, // Different port from CLIAIMONITOR
			NATSPort: 4223, // Different NATS port
		},
		Ollama: OllamaConfig{
			URL:   "http://localhost:11434",
			Model: "qwen2.5-coder:32b",
		},
		Aider: DefaultAiderConfig(),
		Agents: []AgentConfig{
			{
				Name:        "Qwen-Dev-1",
				Role:        "developer",
				Color:       "#00FF00",
				ProjectPath: "",
			},
		},
		Sergeant: SergeantConfig{
			MaxConcurrentAgents: 4,
			IdleTimeout:         300,
		},
	}
}

// DefaultAiderConfig returns sensible defaults for Aider
func DefaultAiderConfig() AiderConfig {
	return AiderConfig{
		Model:          "ollama/qwen2.5-coder:32b",
		OllamaURL:      "http://localhost:11434",
		AutoCommit:     false,
		EditFormat:     "diff",
		MapTokens:      1024,
		MaxChatHistory: 10,
	}
}

// ToArgs converts AiderConfig to command line arguments
func (c *AiderConfig) ToArgs() []string {
	args := []string{
		"--model", c.Model,
		"--edit-format", c.EditFormat,
		"--map-tokens", fmt.Sprintf("%d", c.MapTokens),
	}

	if !c.AutoCommit {
		args = append(args, "--no-auto-commits")
	}

	return args
}

// LoadConfig loads configuration from a YAML file
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config YAML: %w", err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

// Validate checks if the config is valid
func (c *Config) Validate() error {
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}
	if c.Server.NATSPort <= 0 || c.Server.NATSPort > 65535 {
		return fmt.Errorf("invalid NATS port: %d", c.Server.NATSPort)
	}
	if c.Ollama.URL == "" {
		return fmt.Errorf("ollama URL is required")
	}
	if c.Ollama.Model == "" {
		return fmt.Errorf("ollama model is required")
	}
	return nil
}
