package types

// ProjectConfig represents a known project in the ecosystem
type ProjectConfig struct {
	Name        string `yaml:"name" json:"name"`
	Path        string `yaml:"path" json:"path"`
	Description string `yaml:"description" json:"description"`
	HasClaudeMD bool   `yaml:"-" json:"has_claude_md"` // Computed at runtime
}

// ProjectsConfig is the root configuration for projects.yaml
type ProjectsConfig struct {
	ScanPath string          `yaml:"scan_path"`
	Projects []ProjectConfig `yaml:"projects"`
}

// AccessLevel defines the access restrictions for an agent
type AccessLevel string

const (
	// AccessStrict allows read/write only to assigned project
	AccessStrict AccessLevel = "strict"
	// AccessReadOnlyCross allows write to assigned, read from all projects
	AccessReadOnlyCross AccessLevel = "readonly-cross"
	// AccessReadOnlyAll allows read from all projects, no write
	AccessReadOnlyAll AccessLevel = "readonly-all"
)

// GetAccessLevelForRole returns the appropriate access level for an agent role
func GetAccessLevelForRole(role AgentRole) AccessLevel {
	switch role {
	case RoleCodeAuditor, RoleSecurity, RoleReconSpecialOps:
		return AccessReadOnlyCross
	case RoleSupervisor:
		return AccessReadOnlyAll
	default:
		// Go Developer, Engineer - strict isolation
		return AccessStrict
	}
}
