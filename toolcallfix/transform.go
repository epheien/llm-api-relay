package toolcallfix

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

// ChatCompletionChunk represents the SSE chunk structure
type ChatCompletionChunk struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   *Usage   `json:"usage,omitempty"`
}

type Choice struct {
	Index        int     `json:"index"`
	Delta        Delta   `json:"delta"`
	Logprobs     *string `json:"logprobs"`
	FinishReason *string `json:"finish_reason"`
	StopReason   *int    `json:"stop_reason,omitempty"`
	TokenIDs     *string `json:"token_ids"`
}

type Delta struct {
	Content          string     `json:"content"`
	ReasoningContent *string    `json:"reasoning_content"`
	ToolCalls        []ToolCall `json:"tool_calls,omitempty"`
}

type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Index    int          `json:"index"`
	Function FunctionCall `json:"function"`
}

type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	TotalTokens      int `json:"total_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

// ToolCallArg represents a parsed argument from the XML format
type ToolCallArg struct {
	Key   string
	Value string
}

// ParsedToolCall represents a parsed tool call from the XML format
type ParsedToolCall struct {
	Name string
	Args []ToolCallArg
}

// StreamTransformer transforms streams with embedded tool calls in content
// to proper OpenAI-style tool_calls format
type StreamTransformer struct {
	buffer        strings.Builder
	inToolCall    bool
	lastChunk     *ChatCompletionChunk
	toolCallIndex int
}

// NewStreamTransformer creates a new StreamTransformer
func NewStreamTransformer() *StreamTransformer {
	return &StreamTransformer{}
}

// parseToolCallXML parses the XML format tool call into structured data
// Format: <tool_call>name<arg_key>key1</arg_key><arg_value>value1</arg_value>...</tool_call>
func parseToolCallXML(xml string) (*ParsedToolCall, error) {
	// Remove the outer tags
	inner := strings.TrimPrefix(xml, "<tool_call>")
	inner = strings.TrimSuffix(inner, "</tool_call>")
	inner = strings.TrimSpace(inner)

	if inner == "" {
		return nil, fmt.Errorf("empty tool call")
	}

	// Extract function name (everything before the first <arg_key>)
	argKeyIndex := strings.Index(inner, "<arg_key>")
	var name string
	var argsSection string

	if argKeyIndex == -1 {
		name = strings.TrimSpace(inner)
		argsSection = ""
	} else {
		name = strings.TrimSpace(inner[:argKeyIndex])
		argsSection = inner[argKeyIndex:]
	}

	// Parse arguments using (?s) flag to allow . to match newlines
	var args []ToolCallArg
	argKeyRe := regexp.MustCompile(`(?s)<arg_key>(.*?)</arg_key>\s*<arg_value>(.*?)</arg_value>`)
	matches := argKeyRe.FindAllStringSubmatch(argsSection, -1)

	for _, match := range matches {
		if len(match) == 3 {
			args = append(args, ToolCallArg{
				Key:   strings.TrimSpace(match[1]), // 键名可以 TrimSpace
				Value: match[2],                    // 值保持原样
			})
		}
	}

	return &ParsedToolCall{
		Name: name,
		Args: args,
	}, nil
}

// argsToJSON converts tool call arguments to JSON string
func argsToJSON(args []ToolCallArg) string {
	if len(args) == 0 {
		return "{}"
	}

	argMap := make(map[string]string)
	for _, arg := range args {
		argMap[arg.Key] = arg.Value
	}

	jsonBytes, err := json.Marshal(argMap)
	if err != nil {
		return "{}"
	}
	return string(jsonBytes)
}

// TransformLine processes a single SSE line and returns transformed lines
func (t *StreamTransformer) TransformLine(line string) ([]string, error) {
	line = strings.TrimSpace(line)

	// Handle empty lines and [DONE]
	if line == "" {
		return []string{""}, nil
	}
	if line == "data: [DONE]" {
		return []string{"data: [DONE]"}, nil
	}

	// Parse the SSE data
	if !strings.HasPrefix(line, "data: ") {
		return []string{line}, nil
	}

	jsonStr := strings.TrimPrefix(line, "data: ")
	var chunk ChatCompletionChunk
	if err := json.Unmarshal([]byte(jsonStr), &chunk); err != nil {
		return []string{line}, nil
	}

	// Store chunk metadata for later use
	t.lastChunk = &chunk

	// If no choices, pass through (e.g., usage chunk)
	if len(chunk.Choices) == 0 {
		return []string{line}, nil
	}

	content := chunk.Choices[0].Delta.Content

	// Check for tool call start
	if strings.Contains(content, "<tool_call>") {
		t.inToolCall = true
		t.buffer.Reset()

		// Check if there's content before <tool_call>
		idx := strings.Index(content, "<tool_call>")
		if idx > 0 {
			// Output the content before the tool call
			preContent := content[:idx]
			preChunk := t.createContentChunk(preContent, nil)
			preJSON, _ := json.Marshal(preChunk)
			t.buffer.WriteString(content[idx:])
			return []string{fmt.Sprintf("data: %s", preJSON)}, nil
		}

		t.buffer.WriteString(content)
		// Return empty content chunks while buffering
		return t.createEmptyContentChunks(), nil
	}

	// If we're in a tool call, buffer the content
	if t.inToolCall {
		t.buffer.WriteString(content)

		// Check if tool call is complete
		if strings.Contains(t.buffer.String(), "</tool_call>") {
			return t.flushToolCall()
		}

		// Return empty content chunks while buffering
		return t.createEmptyContentChunks(), nil
	}

	// Check finish_reason
	if chunk.Choices[0].FinishReason != nil && *chunk.Choices[0].FinishReason == "stop" {
		// If we have buffered content that wasn't a complete tool call, flush it as content
		if t.buffer.Len() > 0 {
			buffered := t.buffer.String()
			t.buffer.Reset()
			contentChunk := t.createContentChunk(buffered, chunk.Choices[0].FinishReason)
			contentJSON, _ := json.Marshal(contentChunk)
			return []string{fmt.Sprintf("data: %s", contentJSON)}, nil
		}
	}

	// Normal content, pass through
	return []string{line}, nil
}

// flushToolCall parses the buffered tool call and returns the transformed chunks
func (t *StreamTransformer) flushToolCall() ([]string, error) {
	buffered := t.buffer.String()
	t.buffer.Reset()
	t.inToolCall = false

	// Parse the tool call
	parsed, err := parseToolCallXML(buffered)
	if err != nil {
		// If parsing fails, return as regular content
		log.Printf("TOOLCALLFIX: failed to parse tool call (invalid XML format), returning as regular content: %v", err)
		chunk := t.createContentChunk(buffered, nil)
		jsonBytes, _ := json.Marshal(chunk)
		return []string{fmt.Sprintf("data: %s", jsonBytes)}, nil
	}

	// Format arguments for logging
	argsStr := ""
	for i, arg := range parsed.Args {
		if i > 0 {
			argsStr += ", "
		}
		argsStr += fmt.Sprintf("%s=%s", arg.Key, arg.Value)
	}
	log.Printf("TOOLCALLFIX: successfully transformed tool call - name: %s, arguments: [%s]", parsed.Name, argsStr)

	// Create the tool call chunk
	toolCallChunk := t.createToolCallChunk(parsed)
	toolCallJSON, _ := json.Marshal(toolCallChunk)

	// Create the finish chunk with tool_calls reason
	finishReason := "tool_calls"
	finishChunk := t.createFinishChunk(&finishReason)
	finishJSON, _ := json.Marshal(finishChunk)

	t.toolCallIndex++

	return []string{
		fmt.Sprintf("data: %s", toolCallJSON),
		"",
		fmt.Sprintf("data: %s", finishJSON),
	}, nil
}

func (t *StreamTransformer) createEmptyContentChunks() []string {
	chunk := t.createContentChunk("", nil)
	jsonBytes, _ := json.Marshal(chunk)
	return []string{fmt.Sprintf("data: %s", jsonBytes)}
}

func (t *StreamTransformer) createContentChunk(content string, finishReason *string) ChatCompletionChunk {
	chunk := ChatCompletionChunk{
		ID:      t.lastChunk.ID,
		Object:  t.lastChunk.Object,
		Created: t.lastChunk.Created,
		Model:   t.lastChunk.Model,
		Choices: []Choice{
			{
				Index: 0,
				Delta: Delta{
					Content:          content,
					ReasoningContent: nil,
				},
				Logprobs:     nil,
				FinishReason: finishReason,
				TokenIDs:     nil,
			},
		},
	}
	return chunk
}

func (t *StreamTransformer) createToolCallChunk(parsed *ParsedToolCall) ChatCompletionChunk {
	toolCallID := fmt.Sprintf("chatcmpl-tool-%s", uuid.New().String()[:12])

	chunk := ChatCompletionChunk{
		ID:      t.lastChunk.ID,
		Object:  t.lastChunk.Object,
		Created: t.lastChunk.Created,
		Model:   t.lastChunk.Model,
		Choices: []Choice{
			{
				Index: 0,
				Delta: Delta{
					Content:          "",
					ReasoningContent: nil,
					ToolCalls: []ToolCall{
						{
							ID:    toolCallID,
							Type:  "function",
							Index: t.toolCallIndex,
							Function: FunctionCall{
								Name:      parsed.Name,
								Arguments: argsToJSON(parsed.Args),
							},
						},
					},
				},
				Logprobs:     nil,
				FinishReason: nil,
				TokenIDs:     nil,
			},
		},
	}
	return chunk
}

func (t *StreamTransformer) createFinishChunk(finishReason *string) ChatCompletionChunk {
	chunk := ChatCompletionChunk{
		ID:      t.lastChunk.ID,
		Object:  t.lastChunk.Object,
		Created: t.lastChunk.Created,
		Model:   t.lastChunk.Model,
		Choices: []Choice{
			{
				Index: 0,
				Delta: Delta{
					Content:          "",
					ReasoningContent: nil,
				},
				Logprobs:     nil,
				FinishReason: finishReason,
				TokenIDs:     nil,
			},
		},
	}
	if t.lastChunk.Choices != nil && len(t.lastChunk.Choices) > 0 {
		chunk.Choices[0].StopReason = t.lastChunk.Choices[0].StopReason
	}
	return chunk
}

// TransformStream transforms an entire SSE stream
func TransformStream(input io.Reader, output io.Writer, flusher http.Flusher) error {
	transformer := NewStreamTransformer()
	scanner := bufio.NewScanner(input)

	for scanner.Scan() {
		line := scanner.Text()
		transformed, err := transformer.TransformLine(line)
		if err != nil {
			return err
		}
		for _, tLine := range transformed {
			fmt.Fprintln(output, tLine)
			flusher.Flush()
		}
	}

	return scanner.Err()
}
