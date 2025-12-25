package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// ToolCall represents a parsed tool call
type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Index    int    `json:"index"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// ParseToolCallsFromContent parses tool call syntax from content text
// Supports various formats like:
//   - function_name(arg1="value1", arg2="value2")
//   - function_name arg1="value1" arg2="value2"
//   - function_name: arg1="value1", arg2="value2"
func ParseToolCallsFromContent(content string) ([]ToolCall, error) {
	if content == "" {
		return nil, nil
	}

	// Pattern 1: function_name(arg1="value1", arg2="value2")
	pattern1 := regexp.MustCompile(`(\w+)\s*\(([^)]*)\)`)
	matches := pattern1.FindAllStringSubmatch(content, -1)

	if len(matches) > 0 {
		var toolCalls []ToolCall
		for i, match := range matches {
			funcName := match[1]
			argsStr := match[2]

			toolCall := ToolCall{
				ID:    fmt.Sprintf("call_%d", i),
				Type:  "function",
				Index: i,
			}
			toolCall.Function.Name = funcName
			toolCall.Function.Arguments = parseArguments(argsStr)

			toolCalls = append(toolCalls, toolCall)
		}
		return toolCalls, nil
	}

	// Pattern 2: function_name arg1="value1" arg2="value2"
	// Try to find function names followed by arguments
	words := strings.Fields(content)
	if len(words) >= 1 {
		// Check if first word is a function name and rest are key=value pairs
		args := make(map[string]string)

		for i := 1; i < len(words); i++ {
			if strings.Contains(words[i], "=") {
				parts := strings.SplitN(words[i], "=", 2)
				if len(parts) == 2 {
					key := strings.TrimSpace(parts[0])
					value := strings.Trim(strings.TrimSpace(parts[1]), `"`)
					args[key] = value
				}
			} else if words[i] == "|" {
				// Pipe character might indicate command chaining
				break
			}
		}

		if len(args) > 0 {
			toolCall := ToolCall{
				ID:    "call_0",
				Type:  "function",
				Index: 0,
			}
			toolCall.Function.Name = words[0]
			argsJSON, _ := json.Marshal(args)
			toolCall.Function.Arguments = string(argsJSON)
			return []ToolCall{toolCall}, nil
		}
	}

	return nil, errors.New("no valid tool call syntax found")
}

// parseArguments parses argument string like: arg1="value1", arg2="value2"
// into a JSON object string
func parseArguments(argsStr string) string {
	if argsStr == "" {
		return "{}"
	}

	args := make(map[string]string)
	argPattern := regexp.MustCompile(`(\w+)\s*=\s*"([^"]*)"`)
	matches := argPattern.FindAllStringSubmatch(argsStr, -1)

	for _, match := range matches {
		if len(match) == 3 {
			args[match[1]] = match[2]
		}
	}

	// Also handle unquoted values
	unquotedPattern := regexp.MustCompile(`(\w+)\s*=\s*(\w+)`)
	unquotedMatches := unquotedPattern.FindAllStringSubmatch(argsStr, -1)

	for _, match := range unquotedMatches {
		if len(match) == 3 {
			args[match[1]] = match[2]
		}
	}

	result, _ := json.Marshal(args)
	return string(result)
}

// ConvertChunk converts a response chunk, replacing content with tool_calls if needed
func ConvertChunk(chunk map[string]any) (map[string]any, error) {
	// Extract delta
	delta, ok := chunk["choices"].([]any)
	if !ok || len(delta) == 0 {
		return chunk, nil
	}

	choice, ok := delta[0].(map[string]any)
	if !ok {
		return chunk, nil
	}

	deltaData, ok := choice["delta"].(map[string]any)
	if !ok {
		return chunk, nil
	}

	// Check if content exists
	content, ok := deltaData["content"].(string)
	if !ok {
		return chunk, nil
	}

	// Try to parse tool calls from content
	toolCalls, err := ParseToolCallsFromContent(content)
	if err != nil {
		// Not a tool call, return as is
		return chunk, nil
	}

	// If we found tool calls, replace content with tool_calls
	// and set content to empty string
	newDelta := make(map[string]any)
	for k, v := range deltaData {
		newDelta[k] = v
	}

	newDelta["content"] = ""
	newDelta["tool_calls"] = toolCalls

	// Update the chunk
	newChoice := make(map[string]any)
	for k, v := range choice {
		newChoice[k] = v
	}
	newChoice["delta"] = newDelta

	newChoices := make([]any, len(chunk["choices"].([]any)))
	newChoices[0] = newChoice

	newChunk := make(map[string]any)
	for k, v := range chunk {
		if k != "choices" {
			newChunk[k] = v
		}
	}
	newChunk["choices"] = newChoices

	// Also update finish_reason to "tool_calls"
	if finishReason, ok := chunk["finish_reason"].(string); ok && finishReason == "stop" {
		// Check if there's a top-level finish_reason
		newChunk["finish_reason"] = "tool_calls"
	}

	return newChunk, nil
}
