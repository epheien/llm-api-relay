package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
)

func TestStripJSONC(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "single line comment",
			input:    `{"key": "value"} // this is a comment`,
			expected: `{"key": "value"} `,
		},
		{
			name:     "multi line comment",
			input:    `{"key": "value"} /* this is a block comment */`,
			expected: `{"key": "value"} `,
		},
		{
			name:     "line comment within string",
			input:    `{"key": "value with // inside"} // real comment`,
			expected: `{"key": "value with // inside"} `,
		},
		{
			name:     "block comment within string",
			input:    `{"key": "value with /* inside */ string"} /* comment */`,
			expected: `{"key": "value with /* inside */ string"} `,
		},
		{
			name: "multiple comments",
			input: `{"key": "value"} // comment 1
			{"key2": "value2"} // comment 2`,
			expected: `{"key": "value"} 
			{"key2": "value2"} `,
		},
		{
			name: "complex block comment",
			input: `{
				"listen": ":8080", // port config
				/* block comment
				   across multiple lines */
				"upstream": "http://example.com"
			}`,
			expected: `{
				"listen": ":8080", 
				
				"upstream": "http://example.com"
			}`,
		},
		{
			name:     "nested block comments",
			input:    `{"key": "value"} /* outer /* inner */ outer */ end`,
			expected: `{"key": "value"}  outer */ end`,
		},
		{
			name:     "no comments",
			input:    `{"key": "value", "array": [1, 2, 3]}`,
			expected: `{"key": "value", "array": [1, 2, 3]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stripJSONC(tt.input)
			if result != tt.expected {
				t.Errorf("stripJSONC() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestLoadConfigJSONC(t *testing.T) {
	// Test successful parsing
	t.Run("valid config", func(t *testing.T) {
		configJSON := `{
			"listen": ":8080",
			"upstream": "http://localhost:11434/v1",
			"forward_auth": false,
			"model_rules": [
				{
					"match_model": "gpt-4",
					"set": {"temperature": 0.5}
				}
			]
		} // end comment`

		tmpFile, err := createTempFile(configJSON)
		if err != nil {
			t.Fatalf("failed to create temp file: %v", err)
		}
		defer cleanupTempFile(tmpFile)

		cfg, err := loadConfigJSONC(tmpFile.Name())
		if err != nil {
			t.Fatalf("loadConfigJSONC() failed: %v", err)
		}

		if cfg.Listen != ":8080" {
			t.Errorf("expected Listen ':8080', got %q", cfg.Listen)
		}
		if cfg.Upstream != "http://localhost:11434/v1" {
			t.Errorf("expected Upstream 'http://localhost:11434/v1', got %q", cfg.Upstream)
		}
		if cfg.ForwardAuth != false {
			t.Errorf("expected ForwardAuth false, got %v", cfg.ForwardAuth)
		}
		if len(cfg.ModelRules) != 1 {
			t.Errorf("expected 1 model rule, got %d", len(cfg.ModelRules))
		}
	})

	// Test default values
	t.Run("default values", func(t *testing.T) {
		configJSON := `{
			"upstream": "http://example.com"
		}`

		tmpFile, err := createTempFile(configJSON)
		if err != nil {
			t.Fatalf("failed to create temp file: %v", err)
		}
		defer cleanupTempFile(tmpFile)

		cfg, err := loadConfigJSONC(tmpFile.Name())
		if err != nil {
			t.Fatalf("loadConfigJSONC() failed: %v", err)
		}

		if cfg.Listen != ":8080" {
			t.Errorf("expected default Listen ':8080', got %q", cfg.Listen)
		}
	})

	// Test missing upstream error
	t.Run("missing upstream error", func(t *testing.T) {
		configJSON := `{
			"listen": ":8080"
		}`

		tmpFile, err := createTempFile(configJSON)
		if err != nil {
			t.Fatalf("failed to create temp file: %v", err)
		}
		defer cleanupTempFile(tmpFile)

		_, err = loadConfigJSONC(tmpFile.Name())
		if err == nil {
			t.Error("loadConfigJSONC() should fail with missing upstream")
		}
	})

	// Test invalid JSON
	t.Run("invalid JSON", func(t *testing.T) {
		configJSON := `{
			"listen": ":8080",
			"upstream": "http://example.com",
			// missing closing brace
		}`

		tmpFile, err := createTempFile(configJSON)
		if err != nil {
			t.Fatalf("failed to create temp file: %v", err)
		}
		defer cleanupTempFile(tmpFile)

		_, err = loadConfigJSONC(tmpFile.Name())
		if err == nil {
			t.Error("loadConfigJSONC() should fail with invalid JSON")
		}
	})
}

func TestFindRule(t *testing.T) {
	rules := []ModelRule{
		{MatchModel: "gpt-4", Set: map[string]any{"temperature": 0.5}},
		{MatchModel: "default", Set: map[string]any{"temperature": 0.7}},
		{MatchModel: "gpt-3.5-turbo", Set: map[string]any{"max_tokens": 1000}},
	}

	tests := []struct {
		name     string
		model    string
		expected *ModelRule
	}{
		{"exact match gpt-4", "gpt-4", &rules[0]},
		{"exact match default", "default", &rules[1]},
		{"exact match gpt-3.5-turbo", "gpt-3.5-turbo", &rules[2]},
		{"no match", "nonexistent-model", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := findRule(rules, tt.model)
			if result == nil && tt.expected != nil {
				t.Errorf("findRule() returned nil, expected %+v", tt.expected)
			}
			if result != nil && tt.expected == nil {
				t.Errorf("findRule() returned %+v, expected nil", result)
			}
			if result != nil && tt.expected != nil && result.MatchModel != tt.expected.MatchModel {
				t.Errorf("findRule() returned model %q, expected %q", result.MatchModel, tt.expected.MatchModel)
			}
		})
	}
}

func TestGetString(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]any
		key      string
		expected string
	}{
		{"existing string key", map[string]any{"key": "value"}, "key", "value"},
		{"existing non-string key", map[string]any{"key": 123}, "key", ""},
		{"non-existing key", map[string]any{"other": "value"}, "key", ""},
		{"nil value", map[string]any{"key": nil}, "key", ""},
		{"empty map", map[string]any{}, "key", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getString(tt.input, tt.key)
			if result != tt.expected {
				t.Errorf("getString() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestApplyRules(t *testing.T) {
	cfg := &Config{
		ModelRules: []ModelRule{
			{
				MatchModel: "gpt-4",
				Set:        map[string]any{"temperature": 0.5, "model": "new-model"},
				Unset:      []string{"presence_penalty"},
				Extra:      map[string]any{"custom_param": "value"},
			},
			{
				MatchModel: "default",
				Set:        map[string]any{"temperature": 0.7},
			},
		},
	}

	t.Run("exact match rule", func(t *testing.T) {
		req := map[string]any{
			"model":             "gpt-4",
			"temperature":       1.0,
			"presence_penalty":  0.5,
			"frequency_penalty": 0.3,
			"messages":          []any{"hello"},
		}

		applyRules(cfg, req)

		if temp, ok := req["temperature"].(float64); !ok || temp != 0.5 {
			t.Errorf("temperature should be 0.5, got %v", req["temperature"])
		}
		if model, ok := req["model"].(string); !ok || model != "new-model" {
			t.Errorf("model should be 'new-model', got %v", req["model"])
		}
		if _, exists := req["presence_penalty"]; exists {
			t.Errorf("presence_penalty should be removed")
		}
		if _, exists := req["frequency_penalty"]; !exists {
			t.Errorf("frequency_penalty should remain")
		}

		extra, _ := req["extra"].(map[string]any)
		if extra["custom_param"] != "value" {
			t.Errorf("extra.custom_param should be 'value', got %v", extra["custom_param"])
		}
	})

	t.Run("fallback to default rule", func(t *testing.T) {
		req := map[string]any{
			"model":       "unknown-model",
			"temperature": 1.0,
		}

		applyRules(cfg, req)

		if temp, ok := req["temperature"].(float64); !ok || temp != 0.7 {
			t.Errorf("temperature should fallback to 0.7, got %v", req["temperature"])
		}
	})

	t.Run("no matching rule", func(t *testing.T) {
		cfgNoRules := &Config{ModelRules: []ModelRule{}}
		req := map[string]any{
			"model":       "test-model",
			"temperature": 1.0,
		}

		originalTemp := req["temperature"]
		applyRules(cfgNoRules, req)

		if req["temperature"] != originalTemp {
			t.Errorf("request should remain unchanged when no rules match")
		}
	})
}

func TestCopyHeaders(t *testing.T) {
	tests := []struct {
		name         string
		sourceHeader http.Header
		expectedKeys []string
		skippedKeys  []string
	}{
		{
			name: "copy standard headers",
			sourceHeader: http.Header{
				"Content-Type":    []string{"application/json"},
				"Authorization":   []string{"Bearer token"},
				"User-Agent":      []string{"test-agent"},
				"X-Custom-Header": []string{"custom-value"},
			},
			expectedKeys: []string{"Content-Type", "Authorization", "User-Agent", "X-Custom-Header"},
			skippedKeys:  []string{},
		},
		{
			name: "skip hop-by-hop headers",
			sourceHeader: http.Header{
				"Connection":          []string{"keep-alive"},
				"Proxy-Connection":    []string{"keep-alive"},
				"Keep-Alive":          []string{"timeout=5"},
				"Proxy-Authenticate":  []string{"basic"},
				"Proxy-Authorization": []string{"credentials"},
				"Te":                  []string{"gzip"},
				"Trailer":             []string{"ETag"},
				"Transfer-Encoding":   []string{"chunked"},
				"Upgrade":             []string{"websocket"},
			},
			expectedKeys: []string{},
			skippedKeys:  []string{"Connection", "Proxy-Connection", "Keep-Alive", "Proxy-Authenticate", "Proxy-Authorization", "Te", "Trailer", "Transfer-Encoding", "Upgrade"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dst := http.Header{}
			copyHeaders(dst, tt.sourceHeader)

			// Check expected keys are present
			for _, key := range tt.expectedKeys {
				if _, exists := dst[key]; !exists {
					t.Errorf("expected key %q to be copied", key)
				}
			}

			// Check skipped keys are absent
			for _, key := range tt.skippedKeys {
				if _, exists := dst[key]; exists {
					t.Errorf("expected key %q to be skipped", key)
				}
			}

			// Check all copied keys match source
			for k, vv := range tt.sourceHeader {
				if _, shouldSkip := map[string]struct{}{
					"Connection": {}, "Proxy-Connection": {}, "Keep-Alive": {},
					"Proxy-Authenticate": {}, "Proxy-Authorization": {}, "Te": {},
					"Trailer": {}, "Transfer-Encoding": {}, "Upgrade": {},
				}[k]; shouldSkip {
					continue
				}

				gotVV, exists := dst[k]
				if !exists {
					t.Errorf("key %q not found in destination", k)
					continue
				}

				if len(gotVV) != len(vv) {
					t.Errorf("key %q: expected %d values, got %d", k, len(vv), len(gotVV))
					continue
				}

				for i, v := range vv {
					if i >= len(gotVV) || gotVV[i] != v {
						t.Errorf("key %q: value mismatch at index %d", k, i)
						break
					}
				}
			}
		})
	}
}

func TestProxyPassthrough(t *testing.T) {
	// Create a mock upstream server
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request was forwarded correctly
		if r.Method != http.MethodGet {
			t.Errorf("expected GET method, got %s", r.Method)
		}
		if r.URL.Path != "/test/path" {
			t.Errorf("expected path /test/path, got %s", r.URL.Path)
		}

		w.Header().Set("X-Test-Header", "test-value")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"message": "success"}`)
	}))
	defer upstream.Close()

	upstreamURL := parseURLTest(upstream.URL)

	t.Run("forward auth disabled", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test/path", nil)
		req.Header.Set("Authorization", "Bearer secret-token")
		req.Header.Set("X-Custom-Header", "custom-value")

		w := httptest.NewRecorder()

		proxyPassthrough(w, req, upstreamURL, false, nil)

		resp := w.Result()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		// Verify upstream request didn't have auth header
		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "success") {
			t.Errorf("expected success message in response")
		}

		// Check headers were copied
		if resp.Header.Get("X-Test-Header") != "test-value" {
			t.Errorf("expected X-Test-Header in response")
		}
	})

	t.Run("forward auth enabled", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test/path", nil)
		req.Header.Set("Authorization", "Bearer secret-token")

		w := httptest.NewRecorder()

		proxyPassthrough(w, req, upstreamURL, true, nil)

		resp := w.Result()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}
	})
}

// Helper functions for testing
func createTempFile(content string) (*os.File, error) {
	tmpFile, err := os.CreateTemp("", "test-config-*.jsonc")
	if err != nil {
		return nil, err
	}
	if _, err := tmpFile.WriteString(content); err != nil {
		os.Remove(tmpFile.Name())
		return nil, err
	}
	return tmpFile, nil
}

func cleanupTempFile(f *os.File) {
	if f != nil {
		f.Close()
		os.Remove(f.Name())
	}
}

func parseURLTest(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}
