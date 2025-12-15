# AGENTS.md - LLM API Relay Project

## Overview

This is a Go-based OpenAI API compatible relay service that acts as a proxy for Large Language Model (LLM) APIs. The service provides OpenAI-compatible endpoints while forwarding requests to upstream LLM services with optional request transformation.

**Project Purpose**: Enable seamless switching between different LLM providers while maintaining OpenAI API compatibility for client applications.

## Quick Start Commands

### Build and Run
```bash
# Run directly with go run (requires Go 1.18+)
go run main.go

# Build binary first
go build -o llm-relay main.go
./llm-relay --config config.jsonc

# Short form
./llm-relay -c config.jsonc

# With custom config path
go run main.go --config custom-config.jsonc
```

### Dependencies
- Standard Go library only (no external dependencies)
- Go 1.18+ (for JSONC support via custom parser)

## Project Structure

```
llm-api-relay/
├── main.go          # Main application code
├── spec.md          # OpenAI API compatibility specification
├── AGENTS.md        # This file
└── config.jsonc     # Configuration file (created by users)
```

## Configuration System

### JSONC Format
The project uses **JSONC** (JSON with comments) for configuration:

**Example config.jsonc:**
```jsonc
{
  "listen": ":8080",           // Listen address
  "upstream": "http://localhost:11434/v1", // Upstream API base
  "forward_auth": false,         // Whether to forward Authorization header
  "model_rules": [              // Model-specific transformation rules
    {
      "match_model": "gpt-3.5-turbo",
      "set": {
        "model": "qwen2.5-72b-instruct"
      },
      "extra": {
        "custom_param": "value"
      },
      "unset": ["temperature"]  // Remove fields from request
    },
    {
      "match_model": "default",  // Fallback rule
      "set": {
        "model": "default-model"
      }
    }
  ]
}
```

### JSONC Features
- Supports `//` line comments
- Supports `/* block comments */`
- Automatically stripped before JSON parsing

## API Endpoints

### OpenAI Compatible Endpoints

**1. Models List**
```
GET /v1/models
```
- Proxies directly to upstream
- No request modification

**2. Chat Completions**
```
POST /v1/chat/completions
```
- Supports both streaming and non-streaming
- Applies model rules for request transformation
- Supports OpenAI-compatible request/response formats

**3. Legacy Completions**
```
POST /v1/completions
```
- Same as chat completions with rule application

### Service Endpoints

**Health Check**
```
GET /health
```
- Returns "ok" - used for monitoring

## Code Patterns and Conventions

### Function Organization
- **Main server setup**: Lines 73-80
- **HTTP handlers**: Lines 50-71
- **Middleware**: Lines 82-88
- **Configuration loading**: Lines 90-107
- **JSONC parsing**: Lines 111-183
- **Rule application**: Lines 185-236
- **Proxy functions**: Lines 238-407

### Key Patterns

**1. Rule-Based Request Transformation**
```go
func applyRules(cfg *Config, req map[string]any) {
    model := getString(req, "model")
    rule := findRule(cfg.ModelRules, model)
    // Apply unset -> set -> extra transformations
}
```

**2. Streaming Response Handling**
- Detects `stream: true` in request
- Uses `http.Flusher` for chunked responses
- Line-by-line streaming with proper flushing

**3. Header Management**
- Removes hop-by-hop headers (Connection, Proxy-*)
- Preserves most headers including custom ones
- Optional auth forwarding

### Error Handling
- HTTP status codes: 400 (Bad Request), 405 (Method Not Allowed), 502 (Bad Gateway)
- Detailed error messages in responses
- Proper resource cleanup with `defer`

## Model Rules System

### Rule Matching
- Exact model name matching
- "default" rule acts as fallback
- First match wins

### Transformation Types

**1. Set (Top-level fields)**
```go
{"set": {"model": "new-model", "temperature": 0.7}}
```

**2. Extra (Nested object)**
```go
{"extra": {"custom_param": "value"}}
```

**3. Unset (Remove fields)**
```jsonc
{"unset": ["temperature", "presence_penalty"]}
```

### Application Order
1. **Unset** - Remove specified fields
2. **Set** - Add/modify top-level fields
3. **Extra** - Merge into nested "extra" object

## Streaming Implementation

### Detection
```go
stream := false
if v, ok := payload["stream"].(bool); ok && v {
    stream = true
}
```

### Response Handling
- Uses `bufio.NewReader` for line-based streaming
- Flusher interface for proper SSE support
- Graceful EOF handling
- Fallback to simple copy if Flusher unavailable

## Common Use Cases

### 1. Model Translation
Transform client model names to upstream model names:
```jsonc
{
  "match_model": "gpt-4",
  "set": {
    "model": "local-llm-model-name"
  }
}
```

### 2. Parameter Filtering
Remove unsupported parameters for certain models:
```jsonc
{
  "match_model": "legacy-model",
  "unset": ["presence_penalty", "frequency_penalty"]
}
```

### 3. Adding Model-Specific Parameters
```jsonc
{
  "match_model": "custom-model",
  "extra": {
    "generation_params": {
      "temperature": 0.8,
      "top_p": 0.9
    }
  }
}
```

## Testing and Development

### Manual Testing
```bash
# Start service
go run main.go --config config.jsonc

# Test health endpoint
curl http://localhost:8080/health

# Test models endpoint
curl http://localhost:8080/v1/models

# Test chat completions
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "test-model",
    "messages": [{"role": "user", "content": "Hello"}],
    "stream": false
  }'
```

### Debugging
- All requests logged with method, path, and duration
- Check configuration parsing errors
- Verify upstream connectivity
- Monitor streaming responses for chunking issues

## Common Issues and Solutions

### 1. Configuration Not Loading
- Check JSONC syntax (comments properly formatted)
- Verify `upstream` field is present and valid URL
- Ensure file path is correct

### 2. Streaming Not Working
- Verify client sends `"stream": true`
- Check if upstream supports streaming
- Ensure proper flushing in response

### 3. Model Rules Not Applied
- Verify model name exactly matches (case-sensitive)
- Check "default" rule as fallback
- Confirm rule structure is valid JSON

### 4. Auth Issues
- Set `forward_auth: true` to pass through Authorization header
- Otherwise, Authorization header is stripped

## Important Considerations

### Security
- No authentication implemented at service level
- Relies on upstream authentication
- Consider adding auth layer if exposed publicly

### Performance
- Streaming responses can be long-lived
- No built-in rate limiting
- Consider upstream connection pooling for high load

### Compatibility
- Strict OpenAI API compatibility for core endpoints
- Response structure matches OpenAI format
- Supports most common OpenAI parameters

## Future Enhancements

Potential areas for improvement:
- Rate limiting per client
- Request/response logging
- Metrics and monitoring
- Authentication layer
- Connection pooling
- Load balancing across multiple upstreams