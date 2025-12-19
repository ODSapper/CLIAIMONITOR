package agents

import (
	"os"

	"github.com/CLIAIMONITOR/internal/types"
	"gopkg.in/yaml.v3"
)

// LoadTeamsConfig loads team configuration from YAML
func LoadTeamsConfig(filepath string) (*types.TeamsConfig, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, err
	}

	var config types.TeamsConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// GetAgentConfig finds config by name
func GetAgentConfig(config *types.TeamsConfig, name string) *types.AgentConfig {
	for i := range config.Agents {
		if config.Agents[i].Name == name {
			return &config.Agents[i]
		}
	}
	return nil
}

// GetPromptFilename returns prompt file for role
func GetPromptFilename(role types.AgentRole) string {
	switch role {
	case types.RoleSupervisor:
		return "supervisor.md"
	case types.RoleGoDeveloper:
		return "go-developer.md"
	case types.RoleCodeAuditor:
		return "code-auditor.md"
	case types.RoleEngineer:
		return "engineer.md"
	case types.RoleSecurity:
		return "security.md"
	case types.RoleReconSpecialOps:
		return "snake.md"
	default:
		return "engineer.md"
	}
}
