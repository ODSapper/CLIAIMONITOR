package mcp

import (
	"fmt"
)

// ToolHandler processes a tool call and returns result
type ToolHandler func(agentID string, params map[string]interface{}) (interface{}, error)

// ToolRegistry manages available MCP tools
type ToolRegistry struct {
	tools map[string]ToolDefinition
}

// ToolDefinition describes an MCP tool
type ToolDefinition struct {
	Name        string
	Description string
	Parameters  map[string]ParameterDef
	Handler     ToolHandler
}

// ParameterDef describes a tool parameter
type ParameterDef struct {
	Type        string
	Description string
	Required    bool
}

// NewToolRegistry creates a new tool registry
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]ToolDefinition),
	}
}

// Register adds a tool to the registry
func (r *ToolRegistry) Register(tool ToolDefinition) {
	r.tools[tool.Name] = tool
}

// Get returns a tool by name
func (r *ToolRegistry) Get(name string) (ToolDefinition, bool) {
	tool, ok := r.tools[name]
	return tool, ok
}

// List returns all tool definitions (for MCP tools/list)
func (r *ToolRegistry) List() []map[string]interface{} {
	var tools []map[string]interface{}
	for _, tool := range r.tools {
		params := make(map[string]interface{})
		required := []string{}

		for name, def := range tool.Parameters {
			params[name] = map[string]interface{}{
				"type":        def.Type,
				"description": def.Description,
			}
			if def.Required {
				required = append(required, name)
			}
		}

		tools = append(tools, map[string]interface{}{
			"name":        tool.Name,
			"description": tool.Description,
			"inputSchema": map[string]interface{}{
				"type":       "object",
				"properties": params,
				"required":   required,
			},
		})
	}
	return tools
}

// Execute runs a tool by name
func (r *ToolRegistry) Execute(name string, agentID string, params map[string]interface{}) (interface{}, error) {
	tool, ok := r.tools[name]
	if !ok {
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
	return tool.Handler(agentID, params)
}
