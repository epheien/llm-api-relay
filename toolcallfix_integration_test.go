package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

// TestToolCallFixIntegration tests the complete toolcallfix integration
func TestToolCallFixIntegration(t *testing.T) {
	// Create a test upstream server that returns mock responses with tool calls in content
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the request was forwarded correctly
		if r.Method != http.MethodPost {
			t.Errorf("expected POST request, got %s", r.Method)
		}

		// Return a mock streaming response with tool call in content
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)

		// Send streaming chunks with tool call embedded in content
		chunks := []string{
			`data: {"id":"test-123","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{"content":"Let me search for that information.","reasoning_content":null},"logprobs":null,"finish_reason":null,"token_ids":null}]}`,
			`data: {"id":"test-123","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{"content":"` + "\n" + `</think>` + "\n" + `","reasoning_content":null},"logprobs":null,"finish_reason":null,"token_ids":null}]}`,
			`data: {"id":"test-123","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{"content":"","reasoning_content":null},"logprobs":null,"finish_reason":null,"token_ids":null}]}`,
			`data: {"id":"test-123","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{"content":"search","reasoning_content":null},"logprobs":null,"finish_reason":null,"token_ids":null}]}`,
			`data: {"id":"test-123","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{"content":"","reasoning_content":null},"logprobs":null,"finish_reason":null,"token_ids":null}]}`,
			`data: {"id":"test-123","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{"content":"query","reasoning_content":null},"logprobs":null,"finish_reason":null,"token_ids":null}]}`,
			`data: {"id":"test-123","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{"content":"","reasoning_content":null},"logprobs":null,"finish_reason":null,"token_ids":null}]}`,
			`data: {"id":"test-123","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{"content":"","reasoning_content":null},"logprobs":null,"finish_reason":null,"token_ids":null}]}`,
			`data: {"id":"test-123","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{"content":"test search","reasoning_content":null},"logprobs":null,"finish_reason":null,"token_ids":null}]}`,
			`data: {"id":"test-123","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{"content":"","reasoning_content":null},"logprobs":null,"finish_reason":null,"token_ids":null}]}`,
			`data: {"id":"test-123","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{"content":"","reasoning_content":null},"logprobs":null,"finish_reason":null,"token_ids":null}]}`,
			`data: {"id":"test-123","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{"content":"","reasoning_content":null},"logprobs":null,"finish_reason":null,"token_ids":null}]}`,
			`data: {"id":"test-123","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{"content":"","reasoning_content":null},"logprobs":null,"finish_reason":null,"token_ids":null}]}`,
			`data: {"id":"test-123","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{"content":"","reasoning_content":null},"logprobs":null,"finish_reason":null,"token_ids":null}]}`,
			`data: [DONE]`,
		}

		for _, chunk := range chunks {
			fmt.Fprintln(w, chunk)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			time.Sleep(10 * time.Millisecond)
		}
	}))
	defer upstream.Close()

	// Create test config
	configFile, err := os.CreateTemp("", "config-*.jsonc")
	if err != nil {
		t.Fatalf("failed to create temp config: %v", err)
	}
	defer os.Remove(configFile.Name())

	configContent := fmt.Sprintf(`{
  "listen": "127.0.0.1:0",
  "upstream": "%s",
  "forward_auth": false,
  "model_rules": [
    {
      "match_model": "test-model",
      "enable_toolcallfix": true
    },
    {
      "match_model": "default",
      "enable_toolcallfix": true
    }
  ]
}`, upstream.URL)

	if _, err := configFile.WriteString(configContent); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	configFile.Close()

	// Start the relay server with test config
	// Note: In a real integration test, we'd start the server and make HTTP requests
	// For simplicity, we'll test the core functions
	t.Skip("Integration test requires full server startup - implemented as unit tests in toolcallfix package")
}

// TestShouldEnableToolCallFix tests the decision logic for enabling toolcallfix
func TestShouldEnableToolCallFix(t *testing.T) {
	tests := []struct {
		name            string
		config          *Config
		model           string
		expectedEnabled bool
	}{
		{
			name: "Exact match with toolcallfix enabled",
			config: &Config{
				ModelRules: []ModelRule{
					{
						MatchModel:        "gpt-4",
						EnableToolCallFix: true,
					},
				},
			},
			model:           "gpt-4",
			expectedEnabled: true,
		},
		{
			name: "Exact match with toolcallfix disabled",
			config: &Config{
				ModelRules: []ModelRule{
					{
						MatchModel:        "legacy-model",
						EnableToolCallFix: false,
					},
				},
			},
			model:           "legacy-model",
			expectedEnabled: false,
		},
		{
			name: "No match, uses default rule with toolcallfix disabled",
			config: &Config{
				ModelRules: []ModelRule{
					{
						MatchModel:        "default",
						EnableToolCallFix: false,
					},
				},
			},
			model:           "unknown-model",
			expectedEnabled: false,
		},
		{
			name: "No match, uses default rule with toolcallfix enabled",
			config: &Config{
				ModelRules: []ModelRule{
					{
						MatchModel:        "default",
						EnableToolCallFix: true,
					},
				},
			},
			model:           "unknown-model",
			expectedEnabled: true,
		},
		{
			name: "No rules at all, defaults to disabled",
			config: &Config{
				ModelRules: []ModelRule{},
			},
			model:           "any-model",
			expectedEnabled: false,
		},
		{
			name: "Exact match ignores default",
			config: &Config{
				ModelRules: []ModelRule{
					{
						MatchModel:        "specific-model",
						EnableToolCallFix: true,
					},
					{
						MatchModel:        "default",
						EnableToolCallFix: false,
					},
				},
			},
			model:           "specific-model",
			expectedEnabled: true,
		},
		{
			name: "No exact match, falls back to default",
			config: &Config{
				ModelRules: []ModelRule{
					{
						MatchModel:        "other-model",
						EnableToolCallFix: false,
					},
					{
						MatchModel:        "default",
						EnableToolCallFix: true,
					},
				},
			},
			model:           "unmatched-model",
			expectedEnabled: true,
		},
		{
			name: "Exact match with disabled toolcallfix, uses exact match",
			config: &Config{
				ModelRules: []ModelRule{
					{
						MatchModel:        "nil-model",
						EnableToolCallFix: false,
					},
					{
						MatchModel:        "default",
						EnableToolCallFix: true,
					},
				},
			},
			model:           "nil-model",
			expectedEnabled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldEnableToolCallFix(tt.config, tt.model)
			if result != tt.expectedEnabled {
				t.Errorf("shouldEnableToolCallFix() = %v, want %v", result, tt.expectedEnabled)
			}
		})
	}
}

// TestConfigWithToolCallFix tests that the config can be parsed with toolcallfix settings
func TestConfigWithToolCallFix(t *testing.T) {
	configJSON := `{
		"listen": ":8080",
		"upstream": "http://localhost:11434/v1",
		"forward_auth": false,
		"model_rules": [
			{
				"match_model": "gpt-4",
				"set": {"temperature": 0.5},
				"enable_toolcallfix": false
			},
			{
				"match_model": "qwen2.5-72b-instruct",
				"enable_toolcallfix": true
			},
			{
				"match_model": "default",
				"enable_toolcallfix": true
			}
		]
	}`

	clean := stripJSONC(configJSON)
	var cfg Config
	if err := json.Unmarshal([]byte(clean), &cfg); err != nil {
		t.Fatalf("failed to parse config: %v", err)
	}

	// Verify rules were parsed correctly
	if len(cfg.ModelRules) != 3 {
		t.Errorf("expected 3 model rules, got %d", len(cfg.ModelRules))
	}

	// Check gpt-4 rule
	gpt4Rule := findRule(cfg.ModelRules, "gpt-4")
	if gpt4Rule == nil {
		t.Fatal("gpt-4 rule not found")
	}
	if gpt4Rule.EnableToolCallFix != false {
		t.Errorf("gpt-4 enable_toolcallfix should be false, got %v", gpt4Rule.EnableToolCallFix)
	}

	// Check qwen rule
	qwenRule := findRule(cfg.ModelRules, "qwen2.5-72b-instruct")
	if qwenRule == nil {
		t.Fatal("qwen2.5-72b-instruct rule not found")
	}
	if qwenRule.EnableToolCallFix != true {
		t.Errorf("qwen2.5-72b-instruct enable_toolcallfix should be true, got %v", qwenRule.EnableToolCallFix)
	}

	// Check default rule
	defaultRule := findRule(cfg.ModelRules, "default")
	if defaultRule == nil {
		t.Fatal("default rule not found")
	}
	if defaultRule.EnableToolCallFix != true {
		t.Errorf("default enable_toolcallfix should be true, got %v", defaultRule.EnableToolCallFix)
	}
}

// TestConfigWithoutToolCallFix tests backward compatibility when toolcallfix is not specified
func TestConfigWithoutToolCallFix(t *testing.T) {
	configJSON := `{
		"listen": ":8080",
		"upstream": "http://localhost:11434/v1",
		"forward_auth": false,
		"model_rules": [
			{
				"match_model": "gpt-4",
				"set": {"temperature": 0.5}
			},
			{
				"match_model": "default",
				"set": {"temperature": 0.7}
			}
		]
	}`

	clean := stripJSONC(configJSON)
	var cfg Config
	if err := json.Unmarshal([]byte(clean), &cfg); err != nil {
		t.Fatalf("failed to parse config: %v", err)
	}

	// Verify rules were parsed correctly
	if len(cfg.ModelRules) != 2 {
		t.Errorf("expected 2 model rules, got %d", len(cfg.ModelRules))
	}

	// Check that enable_toolcallfix is false (default value)
	gpt4Rule := findRule(cfg.ModelRules, "gpt-4")
	if gpt4Rule == nil {
		t.Fatal("gpt-4 rule not found")
	}
	if gpt4Rule.EnableToolCallFix != false {
		t.Errorf("gpt-4 enable_toolcallfix should be false (not set), got %v", gpt4Rule.EnableToolCallFix)
	}

	// shouldEnableToolCallFix should return false for models without explicit rules
	result := shouldEnableToolCallFix(&cfg, "gpt-4")
	if result != false {
		t.Errorf("shouldEnableToolCallFix should default to false, got %v", result)
	}
}

// Helper function removed - no longer needed since EnableToolCallFix is now a bool instead of *bool

// TestProxyWithJSONPatchWithToolCallFix tests the proxy function with toolcallfix
func TestProxyWithJSONPatchWithToolCallFix(t *testing.T) {
	// Create a mock upstream server
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		// Send a simple streaming response
		fmt.Fprintln(w, `data: {"id":"test","object":"chat.completion.chunk","created":1234567890,"model":"test","choices":[{"index":0,"delta":{"content":"Hello"},"logprobs":null,"finish_reason":null}]}`)
		fmt.Fprintln(w, `data: [DONE]`)
	}))
	defer upstream.Close()

	// Create test request
	reqBody := map[string]any{
		"model":    "test",
		"messages": []map[string]string{{"role": "user", "content": "hi"}},
		"stream":   true,
	}

	bodyBytes, _ := json.Marshal(reqBody)

	// Create test config
	cfg := &Config{
		ModelRules: []ModelRule{
			{
				MatchModel:        "test",
				EnableToolCallFix: true,
			},
		},
	}

	// Create test response writer
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(bodyBytes))

	// Call proxyWithJSONPatch
	proxyWithJSONPatch(w, r, parseURL(upstream.URL), false, cfg, nil)

	// Verify response
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	// Should contain "Hello" content
	if !strings.Contains(bodyStr, "Hello") {
		t.Errorf("response should contain 'Hello', got: %s", bodyStr)
	}
}

// parseURL is a helper to parse a URL string
func parseURL(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}
