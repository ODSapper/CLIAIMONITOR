package types

// TeamsConfig loaded from teams.yaml
type TeamsConfig struct {
	Agents     []AgentConfig `yaml:"agents"`
	Supervisor AgentConfig   `yaml:"supervisor"`
}

// MCPToolCall represents incoming tool call
type MCPToolCall struct {
	Name   string                 `json:"name"`
	Params map[string]interface{} `json:"params"`
}

// MCPRequest JSON-RPC request
type MCPRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// MCPResponse JSON-RPC response
type MCPResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

// MCPError for error responses
type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// MCPNotification for server-initiated messages
type MCPNotification struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// WebSocket message types
type WSMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// WebSocket message type constants
const (
	WSTypeStateUpdate    = "state_update"
	WSTypeAlert          = "alert"
	WSTypeActivity       = "activity"
	WSTypeSupervisor     = "supervisor_status"
	WSTypeEscalation     = "escalation"
	WSTypeCaptainMessage = "captain_message"
)
