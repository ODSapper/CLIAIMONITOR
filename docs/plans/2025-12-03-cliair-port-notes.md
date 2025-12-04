# CLIAIR Port Notes

This document outlines considerations for porting CLIAIMONITOR to CLIAIR (local LLM variant).

## Overview

CLIAIR would be a variant of CLIAIMONITOR that works with local LLMs (Ollama, LM Studio, etc.) instead of Claude.

## Components to Port

### 1. NATS Messaging (Ready to Use)
The NATS messaging infrastructure is LLM-agnostic:
- `internal/nats/` - No changes needed
- Message types work for any agent
- Subject patterns are generic

### 2. Configuration Changes

**Agent Config** (`configs/teams.yaml`):
```yaml
agents:
  - name: LocalDev
    role: engineer
    model: ollama/codellama:latest  # Local model reference
    color: "#00FF00"
```

**MCP Config** - Add local LLM endpoint:
```json
{
  "mcpServers": {
    "cliair": {
      "type": "sse",
      "url": "http://localhost:3000/mcp/sse",
      "nats_url": "nats://localhost:4222",
      "llm_endpoint": "http://localhost:11434/api/generate"
    }
  }
}
```

### 3. Spawner Modifications

The spawner (`internal/agents/spawner.go`) would need:
- Replace `claude.exe` invocation with local LLM client
- Add Ollama/LM Studio process management
- Handle different prompt formats per model

### 4. Agent Protocol Adapter

Create `internal/agents/local_adapter.go`:
- Translate MCP tool calls to local LLM format
- Handle streaming responses
- Convert tool results back to MCP format

### 5. Model-Specific Prompts

Different local models need different prompt formats:
- CodeLlama: `<s>[INST] {prompt} [/INST]`
- Mistral: Same as CodeLlama
- DeepSeek-Coder: Standard chat format
- Llama 3: `<|begin_of_text|><|start_header_id|>system<|end_header_id|>`

Create `configs/prompts/formats/` with model-specific templates.

### 6. Branch Strategy

Recommended branch structure:
```
main                    # Claude-based CLIAIMONITOR
├── feature/nats        # NATS messaging (current work)
└── cliair/main         # Local LLM variant
    ├── cliair/ollama   # Ollama-specific
    └── cliair/lmstudio # LM Studio-specific
```

### 7. Shared vs Divergent Code

**Shared (keep in sync)**:
- `internal/nats/` - All NATS code
- `internal/server/` - HTTP server, handlers
- `internal/memory/` - SQLite persistence
- `internal/types/` - Type definitions
- `web/` - Dashboard UI

**Divergent (fork)**:
- `internal/agents/` - Agent spawning
- `cmd/` - Entry points
- `configs/prompts/` - Prompt templates

### 8. Testing Strategy

- Unit tests: Should work across both variants
- Integration tests: Need mock LLM responses
- E2E tests: Variant-specific

## Implementation Priority

1. **Phase 1**: Create cliair/main branch from current NATS work
2. **Phase 2**: Add Ollama spawner implementation
3. **Phase 3**: Create local adapter for tool calls
4. **Phase 4**: Add model-specific prompt templates
5. **Phase 5**: Test with CodeLlama
6. **Phase 6**: Add LM Studio support

## Notes

- The NATS infrastructure makes the port easier since messaging is decoupled
- Local LLMs may need more constrained tool schemas
- Consider token budget differences (4k vs 32k context)
- May need retry logic for less reliable local models
