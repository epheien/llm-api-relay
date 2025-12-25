package toolcallfix

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestParseToolCallXML(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected *ParsedToolCall
		hasError bool
	}{
		{
			name:  "simple tool call with two args",
			input: "<tool_call>grep<arg_key>include</arg_key><arg_value>*.go</arg_value><arg_key>pattern</arg_key><arg_value>chat.*template</arg_value></tool_call>",
			expected: &ParsedToolCall{
				Name: "grep",
				Args: []ToolCallArg{
					{Key: "include", Value: "*.go"},
					{Key: "pattern", Value: "chat.*template"},
				},
			},
			hasError: false,
		},
		{
			name:  "tool call with single arg",
			input: "<tool_call>view<arg_key>file_path</arg_key><arg_value>/path/to/file.go</arg_value></tool_call>",
			expected: &ParsedToolCall{
				Name: "view",
				Args: []ToolCallArg{
					{Key: "file_path", Value: "/path/to/file.go"},
				},
			},
			hasError: false,
		},
		{
			name:  "tool call with no args",
			input: "<tool_call>list_files</tool_call>",
			expected: &ParsedToolCall{
				Name: "list_files",
				Args: nil,
			},
			hasError: false,
		},
		{
			name:     "empty tool call",
			input:    "<tool_call></tool_call>",
			expected: nil,
			hasError: true,
		},
		{
			name:  "tool call with special characters in value",
			input: "<tool_call>grep<arg_key>pattern</arg_key><arg_value>chat.*template|ChatTemplate|template.*provider</arg_value></tool_call>",
			expected: &ParsedToolCall{
				Name: "grep",
				Args: []ToolCallArg{
					{Key: "pattern", Value: "chat.*template|ChatTemplate|template.*provider"},
				},
			},
			hasError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseToolCallXML(tt.input)

			if tt.hasError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if result.Name != tt.expected.Name {
				t.Errorf("name mismatch: got %q, want %q", result.Name, tt.expected.Name)
			}

			if len(result.Args) != len(tt.expected.Args) {
				t.Errorf("args count mismatch: got %d, want %d", len(result.Args), len(tt.expected.Args))
				return
			}

			for i, arg := range result.Args {
				if arg.Key != tt.expected.Args[i].Key {
					t.Errorf("arg[%d] key mismatch: got %q, want %q", i, arg.Key, tt.expected.Args[i].Key)
				}
				if arg.Value != tt.expected.Args[i].Value {
					t.Errorf("arg[%d] value mismatch: got %q, want %q", i, arg.Value, tt.expected.Args[i].Value)
				}
			}
		})
	}
}

func TestArgsToJSON(t *testing.T) {
	tests := []struct {
		name     string
		args     []ToolCallArg
		expected map[string]string
	}{
		{
			name: "two args",
			args: []ToolCallArg{
				{Key: "include", Value: "*.go"},
				{Key: "pattern", Value: "test"},
			},
			expected: map[string]string{"include": "*.go", "pattern": "test"},
		},
		{
			name:     "empty args",
			args:     []ToolCallArg{},
			expected: map[string]string{},
		},
		{
			name: "single arg",
			args: []ToolCallArg{
				{Key: "file_path", Value: "/path/to/file"},
			},
			expected: map[string]string{"file_path": "/path/to/file"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := argsToJSON(tt.args)

			var resultMap map[string]string
			if err := json.Unmarshal([]byte(result), &resultMap); err != nil {
				t.Errorf("failed to parse result JSON: %v", err)
				return
			}

			if len(resultMap) != len(tt.expected) {
				t.Errorf("map size mismatch: got %d, want %d", len(resultMap), len(tt.expected))
				return
			}

			for k, v := range tt.expected {
				if resultMap[k] != v {
					t.Errorf("value mismatch for key %q: got %q, want %q", k, resultMap[k], v)
				}
			}
		})
	}
}

func TestStreamTransformer_SimpleContent(t *testing.T) {
	transformer := NewStreamTransformer()

	line := `data: {"id":"test-123","object":"chat.completion.chunk","created":1234567890,"model":"glm-4.7","choices":[{"index":0,"delta":{"content":"Hello","reasoning_content":null},"logprobs":null,"finish_reason":null,"token_ids":null}]}`

	result, err := transformer.TransformLine(line)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(result) != 1 {
		t.Errorf("expected 1 result, got %d", len(result))
	}

	if result[0] != line {
		t.Errorf("expected line to pass through unchanged")
	}
}

func TestStreamTransformer_ToolCallInContent(t *testing.T) {
	transformer := NewStreamTransformer()

	// Simulate the problematic stream
	lines := []string{
		`data: {"id":"test-123","object":"chat.completion.chunk","created":1234567890,"model":"glm-4.7","choices":[{"index":0,"delta":{"content":"配置","reasoning_content":null},"logprobs":null,"finish_reason":null,"token_ids":null}]}`,
		`data: {"id":"test-123","object":"chat.completion.chunk","created":1234567890,"model":"glm-4.7","choices":[{"index":0,"delta":{"content":"：","reasoning_content":null},"logprobs":null,"finish_reason":null,"token_ids":null}]}`,
		`data: {"id":"test-123","object":"chat.completion.chunk","created":1234567890,"model":"glm-4.7","choices":[{"index":0,"delta":{"content":"<tool_call>","reasoning_content":null},"logprobs":null,"finish_reason":null,"token_ids":null}]}`,
		`data: {"id":"test-123","object":"chat.completion.chunk","created":1234567890,"model":"glm-4.7","choices":[{"index":0,"delta":{"content":"grep","reasoning_content":null},"logprobs":null,"finish_reason":null,"token_ids":null}]}`,
		`data: {"id":"test-123","object":"chat.completion.chunk","created":1234567890,"model":"glm-4.7","choices":[{"index":0,"delta":{"content":"<arg_key>","reasoning_content":null},"logprobs":null,"finish_reason":null,"token_ids":null}]}`,
		`data: {"id":"test-123","object":"chat.completion.chunk","created":1234567890,"model":"glm-4.7","choices":[{"index":0,"delta":{"content":"pattern","reasoning_content":null},"logprobs":null,"finish_reason":null,"token_ids":null}]}`,
		`data: {"id":"test-123","object":"chat.completion.chunk","created":1234567890,"model":"glm-4.7","choices":[{"index":0,"delta":{"content":"</arg_key>","reasoning_content":null},"logprobs":null,"finish_reason":null,"token_ids":null}]}`,
		`data: {"id":"test-123","object":"chat.completion.chunk","created":1234567890,"model":"glm-4.7","choices":[{"index":0,"delta":{"content":"<arg_value>","reasoning_content":null},"logprobs":null,"finish_reason":null,"token_ids":null}]}`,
		`data: {"id":"test-123","object":"chat.completion.chunk","created":1234567890,"model":"glm-4.7","choices":[{"index":0,"delta":{"content":"test","reasoning_content":null},"logprobs":null,"finish_reason":null,"token_ids":null}]}`,
		`data: {"id":"test-123","object":"chat.completion.chunk","created":1234567890,"model":"glm-4.7","choices":[{"index":0,"delta":{"content":"</arg_value>","reasoning_content":null},"logprobs":null,"finish_reason":null,"token_ids":null}]}`,
		`data: {"id":"test-123","object":"chat.completion.chunk","created":1234567890,"model":"glm-4.7","choices":[{"index":0,"delta":{"content":"</tool_call>","reasoning_content":null},"logprobs":null,"finish_reason":null,"token_ids":null}]}`,
	}

	var allResults []string
	for _, line := range lines {
		results, err := transformer.TransformLine(line)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		allResults = append(allResults, results...)
	}

	// Check that we got a tool_calls chunk
	foundToolCall := false
	foundToolCallsFinish := false

	for _, result := range allResults {
		// Skip empty or non-data lines
		if !strings.HasPrefix(result, "data: ") || result == "data: [DONE]" {
			continue
		}

		jsonStr := strings.TrimPrefix(result, "data: ")
		var chunk ChatCompletionChunk
		if err := json.Unmarshal([]byte(jsonStr), &chunk); err != nil {
			continue
		}

		// Check for tool_calls
		if len(chunk.Choices) > 0 && len(chunk.Choices[0].Delta.ToolCalls) > 0 {
			foundToolCall = true

			tc := chunk.Choices[0].Delta.ToolCalls[0]
			if tc.Function.Name != "grep" {
				t.Errorf("expected function name 'grep', got %q", tc.Function.Name)
			}

			var args map[string]string
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
				t.Errorf("failed to parse arguments: %v", err)
				continue
			}

			if args["pattern"] != "test" {
				t.Errorf("expected pattern 'test', got %q", args["pattern"])
			}
		}

		// Check for finish_reason
		if len(chunk.Choices) > 0 && chunk.Choices[0].FinishReason != nil && *chunk.Choices[0].FinishReason == "tool_calls" {
			foundToolCallsFinish = true
		}
	}

	if !foundToolCall {
		t.Errorf("expected to find a tool_calls chunk in output")
		t.Logf("all results: %v", allResults)
	}

	if !foundToolCallsFinish {
		t.Errorf("expected to find finish_reason 'tool_calls' in output")
		t.Logf("all results: %v", allResults)
	}

	// Verify that the raw <tool_call> XML content is not in any output content field
	for _, result := range allResults {
		if !strings.HasPrefix(result, "data: ") || result == "data: [DONE]" {
			continue
		}

		jsonStr := strings.TrimPrefix(result, "data: ")
		var chunk ChatCompletionChunk
		if err := json.Unmarshal([]byte(jsonStr), &chunk); err == nil {
			if len(chunk.Choices) > 0 {
				content := chunk.Choices[0].Delta.Content
				// Content should not contain raw tool call XML (except empty content)
				if strings.Contains(content, "<tool_call>") || strings.Contains(content, "<arg_key>") {
					t.Errorf("raw tool call XML should not appear in content: %q", content)
				}
			}
		}
	}
}
func TestStreamTransformer_Done(t *testing.T) {
	transformer := NewStreamTransformer()

	result, err := transformer.TransformLine("data: [DONE]")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(result) != 1 || result[0] != "data: [DONE]" {
		t.Errorf("expected [DONE] to pass through unchanged")
	}
}

func TestStreamTransformer_EmptyLine(t *testing.T) {
	transformer := NewStreamTransformer()

	result, err := transformer.TransformLine("")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(result) != 1 || result[0] != "" {
		t.Errorf("expected empty line to pass through")
	}
}

func TestTransformStream_FullStream(t *testing.T) {
	input := `data: {"id":"test-123","object":"chat.completion.chunk","created":1234567890,"model":"glm-4.7","choices":[{"index":0,"delta":{"content":"Hello","reasoning_content":null},"logprobs":null,"finish_reason":null,"token_ids":null}]}
data: {"id":"test-123","object":"chat.completion.chunk","created":1234567890,"model":"glm-4.7","choices":[{"index":0,"delta":{"content":"<tool_call>","reasoning_content":null},"logprobs":null,"finish_reason":null,"token_ids":null}]}
data: {"id":"test-123","object":"chat.completion.chunk","created":1234567890,"model":"glm-4.7","choices":[{"index":0,"delta":{"content":"test_func","reasoning_content":null},"logprobs":null,"finish_reason":null,"token_ids":null}]}
data: {"id":"test-123","object":"chat.completion.chunk","created":1234567890,"model":"glm-4.7","choices":[{"index":0,"delta":{"content":"</tool_call>","reasoning_content":null},"logprobs":null,"finish_reason":null,"token_ids":null}]}
data: [DONE]`

	reader := strings.NewReader(input)
	var output bytes.Buffer

	err := TransformStream(reader, &output)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	result := output.String()

	// Verify content chunk passed through
	if !strings.Contains(result, `"content":"Hello"`) {
		t.Errorf("expected Hello content in output")
	}

	// Verify tool_calls appeared
	if !strings.Contains(result, `"tool_calls"`) {
		t.Errorf("expected tool_calls in output")
	}

	// Verify function name
	if !strings.Contains(result, `"name":"test_func"`) {
		t.Errorf("expected function name 'test_func' in output")
	}

	// Verify [DONE] at end
	if !strings.HasSuffix(strings.TrimSpace(result), "data: [DONE]") {
		t.Errorf("expected [DONE] at end of output")
	}
}

func TestStreamTransformer_MultipleToolCalls(t *testing.T) {
	// Test two tool calls within the same transformer (same stream)
	transformer := NewStreamTransformer()

	lines := []string{
		// First tool call
		`data: {"id":"test-123","object":"chat.completion.chunk","created":1234567890,"model":"glm-4.7","choices":[{"index":0,"delta":{"content":"<tool_call>","reasoning_content":null},"logprobs":null,"finish_reason":null,"token_ids":null}]}`,
		`data: {"id":"test-123","object":"chat.completion.chunk","created":1234567890,"model":"glm-4.7","choices":[{"index":0,"delta":{"content":"func1","reasoning_content":null},"logprobs":null,"finish_reason":null,"token_ids":null}]}`,
		`data: {"id":"test-123","object":"chat.completion.chunk","created":1234567890,"model":"glm-4.7","choices":[{"index":0,"delta":{"content":"</tool_call>","reasoning_content":null},"logprobs":null,"finish_reason":null,"token_ids":null}]}`,
		// Some content between
		`data: {"id":"test-123","object":"chat.completion.chunk","created":1234567890,"model":"glm-4.7","choices":[{"index":0,"delta":{"content":"中间内容","reasoning_content":null},"logprobs":null,"finish_reason":null,"token_ids":null}]}`,
		// Second tool call
		`data: {"id":"test-123","object":"chat.completion.chunk","created":1234567890,"model":"glm-4.7","choices":[{"index":0,"delta":{"content":"<tool_call>","reasoning_content":null},"logprobs":null,"finish_reason":null,"token_ids":null}]}`,
		`data: {"id":"test-123","object":"chat.completion.chunk","created":1234567890,"model":"glm-4.7","choices":[{"index":0,"delta":{"content":"func2","reasoning_content":null},"logprobs":null,"finish_reason":null,"token_ids":null}]}`,
		`data: {"id":"test-123","object":"chat.completion.chunk","created":1234567890,"model":"glm-4.7","choices":[{"index":0,"delta":{"content":"</tool_call>","reasoning_content":null},"logprobs":null,"finish_reason":null,"token_ids":null}]}`,
	}

	var allResults []string
	for _, line := range lines {
		results, err := transformer.TransformLine(line)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		allResults = append(allResults, results...)
	}

	// Count tool calls
	toolCallCount := 0
	funcNames := []string{}

	for _, result := range allResults {
		if !strings.HasPrefix(result, "data: ") || result == "data: [DONE]" {
			continue
		}

		jsonStr := strings.TrimPrefix(result, "data: ")
		var chunk ChatCompletionChunk
		if err := json.Unmarshal([]byte(jsonStr), &chunk); err != nil {
			continue
		}

		if len(chunk.Choices) > 0 && len(chunk.Choices[0].Delta.ToolCalls) > 0 {
			toolCallCount++
			funcNames = append(funcNames, chunk.Choices[0].Delta.ToolCalls[0].Function.Name)
		}
	}

	if toolCallCount != 2 {
		t.Errorf("expected 2 tool calls, got %d", toolCallCount)
		t.Logf("all results: %v", allResults)
	}

	// Verify function names
	expectedNames := []string{"func1", "func2"}
	for i, name := range expectedNames {
		if i >= len(funcNames) {
			t.Errorf("missing function name at index %d", i)
			continue
		}
		if funcNames[i] != name {
			t.Errorf("expected function name %q at index %d, got %q", name, i, funcNames[i])
		}
	}
}
func TestStreamTransformer_ContentBeforeToolCall(t *testing.T) {
	transformer := NewStreamTransformer()

	// Content that includes text before tool call in same chunk
	line := `data: {"id":"test-123","object":"chat.completion.chunk","created":1234567890,"model":"glm-4.7","choices":[{"index":0,"delta":{"content":"前缀文本<tool_call>","reasoning_content":null},"logprobs":null,"finish_reason":null,"token_ids":null}]}`

	results, err := transformer.TransformLine(line)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Should output the prefix content
	foundPrefix := false
	for _, result := range results {
		if strings.Contains(result, "前缀文本") {
			foundPrefix = true
			// Make sure it doesn't contain the tool call start
			if strings.Contains(result, "<tool_call>") {
				jsonStr := strings.TrimPrefix(result, "data: ")
				var chunk ChatCompletionChunk
				if err := json.Unmarshal([]byte(jsonStr), &chunk); err == nil {
					if len(chunk.Choices) > 0 && strings.Contains(chunk.Choices[0].Delta.Content, "<tool_call>") {
						t.Errorf("prefix content should not contain <tool_call>")
					}
				}
			}
		}
	}

	if !foundPrefix {
		t.Errorf("expected prefix content in output")
	}
}

func TestStreamTransformer_UsageChunk(t *testing.T) {
	transformer := NewStreamTransformer()

	line := `data: {"id":"test-123","object":"chat.completion.chunk","created":1234567890,"model":"glm-4.7","choices":[],"usage":{"prompt_tokens":100,"total_tokens":150,"completion_tokens":50}}`

	results, err := transformer.TransformLine(line)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}

	if results[0] != line {
		t.Errorf("usage chunk should pass through unchanged")
	}
}
