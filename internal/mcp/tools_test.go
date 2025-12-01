package mcp

import (
	"errors"
	"testing"
)

func TestNewToolRegistry(t *testing.T) {
	r := NewToolRegistry()
	if r == nil {
		t.Fatal("NewToolRegistry returned nil")
	}
	if r.tools == nil {
		t.Error("tools map should be initialized")
	}
}

func TestRegisterAndGet(t *testing.T) {
	r := NewToolRegistry()

	tool := ToolDefinition{
		Name:        "test_tool",
		Description: "A test tool",
		Parameters: map[string]ParameterDef{
			"param1": {Type: "string", Description: "First param", Required: true},
		},
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			return "success", nil
		},
	}

	r.Register(tool)

	retrieved, ok := r.Get("test_tool")
	if !ok {
		t.Fatal("Get returned false for registered tool")
	}
	if retrieved.Name != "test_tool" {
		t.Errorf("Name = %q, want %q", retrieved.Name, "test_tool")
	}
	if retrieved.Description != "A test tool" {
		t.Errorf("Description = %q, want %q", retrieved.Description, "A test tool")
	}
}

func TestToolRegistryGetNotFound(t *testing.T) {
	r := NewToolRegistry()

	_, ok := r.Get("nonexistent")
	if ok {
		t.Error("Get should return false for nonexistent tool")
	}
}

func TestList(t *testing.T) {
	r := NewToolRegistry()

	r.Register(ToolDefinition{
		Name:        "tool1",
		Description: "First tool",
		Parameters: map[string]ParameterDef{
			"required_param": {Type: "string", Description: "Required", Required: true},
			"optional_param": {Type: "number", Description: "Optional", Required: false},
		},
	})

	r.Register(ToolDefinition{
		Name:        "tool2",
		Description: "Second tool",
		Parameters:  map[string]ParameterDef{},
	})

	list := r.List()

	if len(list) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(list))
	}

	// Find tool1 in list
	var tool1 map[string]interface{}
	for _, tool := range list {
		if tool["name"] == "tool1" {
			tool1 = tool
			break
		}
	}

	if tool1 == nil {
		t.Fatal("tool1 not found in list")
	}

	inputSchema := tool1["inputSchema"].(map[string]interface{})
	if inputSchema["type"] != "object" {
		t.Errorf("inputSchema.type = %v, want object", inputSchema["type"])
	}

	required := inputSchema["required"].([]string)
	if len(required) != 1 || required[0] != "required_param" {
		t.Errorf("required = %v, want [required_param]", required)
	}
}

func TestExecute(t *testing.T) {
	r := NewToolRegistry()

	r.Register(ToolDefinition{
		Name: "echo_tool",
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			return map[string]interface{}{
				"agent_id": agentID,
				"message":  params["message"],
			}, nil
		},
	})

	result, err := r.Execute("echo_tool", "TestAgent", map[string]interface{}{
		"message": "hello",
	})

	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}

	resultMap := result.(map[string]interface{})
	if resultMap["agent_id"] != "TestAgent" {
		t.Errorf("agent_id = %v, want TestAgent", resultMap["agent_id"])
	}
	if resultMap["message"] != "hello" {
		t.Errorf("message = %v, want hello", resultMap["message"])
	}
}

func TestExecuteUnknownTool(t *testing.T) {
	r := NewToolRegistry()

	_, err := r.Execute("nonexistent", "Agent1", nil)
	if err == nil {
		t.Error("expected error for unknown tool")
	}
}

func TestExecuteHandlerError(t *testing.T) {
	r := NewToolRegistry()

	r.Register(ToolDefinition{
		Name: "error_tool",
		Handler: func(agentID string, params map[string]interface{}) (interface{}, error) {
			return nil, errors.New("handler error")
		},
	})

	_, err := r.Execute("error_tool", "Agent1", nil)
	if err == nil {
		t.Error("expected error from handler")
	}
	if err.Error() != "handler error" {
		t.Errorf("error = %q, want %q", err.Error(), "handler error")
	}
}

func TestRegisterOverwrite(t *testing.T) {
	r := NewToolRegistry()

	r.Register(ToolDefinition{
		Name:        "tool",
		Description: "Original",
	})

	r.Register(ToolDefinition{
		Name:        "tool",
		Description: "Overwritten",
	})

	tool, _ := r.Get("tool")
	if tool.Description != "Overwritten" {
		t.Error("registering same name should overwrite")
	}
}

func TestListIncludesParameterDetails(t *testing.T) {
	r := NewToolRegistry()

	r.Register(ToolDefinition{
		Name: "detailed_tool",
		Parameters: map[string]ParameterDef{
			"text_param": {
				Type:        "string",
				Description: "A text parameter",
				Required:    true,
			},
		},
	})

	list := r.List()
	if len(list) != 1 {
		t.Fatal("expected 1 tool")
	}

	inputSchema := list[0]["inputSchema"].(map[string]interface{})
	props := inputSchema["properties"].(map[string]interface{})
	textParam := props["text_param"].(map[string]interface{})

	if textParam["type"] != "string" {
		t.Errorf("type = %v, want string", textParam["type"])
	}
	if textParam["description"] != "A text parameter" {
		t.Errorf("description = %v, want 'A text parameter'", textParam["description"])
	}
}
