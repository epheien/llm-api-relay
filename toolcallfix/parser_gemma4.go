package toolcallfix

import (
	"fmt"
	"regexp"
	"strings"
)

// Gemma4 格式常量
const (
	gemma4StringDelim = `<|"|>`
)

// gemma4Parser Gemma4 格式解析器
// 格式: <|tool_call>call:func_name{key:<|"|>value<|"|>}<tool_call|>
type gemma4Parser struct{}

func (p *gemma4Parser) Name() string     { return "gemma4" }
func (p *gemma4Parser) StartTag() string { return "<|tool_call>" }
func (p *gemma4Parser) EndTag() string   { return "<tool_call|>" }

// parseGemma4Value 解析单个 Gemma4 值 (对应 Python 的 _parse_gemma4_value)
func parseGemma4Value(valueStr string) any {
	valueStr = strings.TrimSpace(valueStr)
	if valueStr == "" {
		return valueStr
	}

	// 布尔值
	if valueStr == "true" {
		return true
	}
	if valueStr == "false" {
		return false
	}

	// 数字 (int 或 float)
	if strings.Contains(valueStr, ".") {
		var f float64
		if _, err := fmt.Sscanf(valueStr, "%f", &f); err == nil {
			return f
		}
	} else {
		var i int64
		if _, err := fmt.Sscanf(valueStr, "%d", &i); err == nil {
			return i
		}
	}

	// 默认返回字符串
	return valueStr
}

// parseGemma4Args 解析 Gemma4 的 key:value 格式为 map (对应 Python 的 _parse_gemma4_args)
// 支持格式:
//   - location:<|"|>Tokyo<|"|>
//   - location:<|"|>San Francisco<|"|>,unit:<|"|>celsius<|"|>
//   - count:42,flag:true
//   - nested:{inner_key:<|"|>val<|"|>}
//   - items:[<|"|>a<|"|>,<|"|>b<|"|>]
func parseGemma4Args(argsStr string) map[string]any {
	if argsStr == "" || strings.TrimSpace(argsStr) == "" {
		return nil
	}

	result := make(map[string]any)
	i := 0
	n := len(argsStr)

	for i < n {
		// 跳过空白和逗号
		for i < n && (argsStr[i] == ' ' || argsStr[i] == ',' || argsStr[i] == '\n' || argsStr[i] == '\t') {
			i++
		}
		if i >= n {
			break
		}

		// 解析 key (无引号，以 ':' 结尾)
		keyStart := i
		for i < n && argsStr[i] != ':' {
			i++
		}
		if i >= n {
			break
		}
		key := strings.TrimSpace(argsStr[keyStart:i])
		i++ // 跳过 ':'

		// 跳过 ':' 后的空白
		for i < n && (argsStr[i] == ' ' || argsStr[i] == '\n' || argsStr[i] == '\t') {
			i++
		}
		if i >= n {
			result[key] = ""
			break
		}

		// 字符串值: <|"|>...<|"|>
		if strings.HasPrefix(argsStr[i:], gemma4StringDelim) {
			i += len(gemma4StringDelim)
			valStart := i
			endPos := strings.Index(argsStr[i:], gemma4StringDelim)
			if endPos == -1 {
				// 未终止的字符串 - 取剩余部分
				result[key] = argsStr[valStart:]
				break
			}
			result[key] = argsStr[valStart : valStart+endPos]
			i = valStart + endPos + len(gemma4StringDelim)
		} else if argsStr[i] == '{' {
			// 嵌套对象: {...}
			depth := 1
			objStart := i + 1
			i++
			for i < n && depth > 0 {
				if strings.HasPrefix(argsStr[i:], gemma4StringDelim) {
					// 跳过字符串内容以避免计算字符串内的括号
					i += len(gemma4StringDelim)
					nextDelim := strings.Index(argsStr[i:], gemma4StringDelim)
					if nextDelim == -1 {
						i = n
						break
					}
					i = i + nextDelim + len(gemma4StringDelim)
					continue
				}
				if argsStr[i] == '{' {
					depth++
				} else if argsStr[i] == '}' {
					depth--
				}
				i++
			}
			result[key] = parseGemma4Args(argsStr[objStart : i-1])
		} else if argsStr[i] == '[' {
			// 数组: [...]
			depth := 1
			arrStart := i + 1
			i++
			for i < n && depth > 0 {
				if strings.HasPrefix(argsStr[i:], gemma4StringDelim) {
					i += len(gemma4StringDelim)
					nextDelim := strings.Index(argsStr[i:], gemma4StringDelim)
					if nextDelim == -1 {
						i = n
						break
					}
					i = i + nextDelim + len(gemma4StringDelim)
					continue
				}
				if argsStr[i] == '[' {
					depth++
				} else if argsStr[i] == ']' {
					depth--
				}
				i++
			}
			result[key] = parseGemma4Array(argsStr[arrStart : i-1])
		} else {
			// 裸值 (数字、布尔等)
			valStart := i
			for i < n && argsStr[i] != ',' && argsStr[i] != '}' && argsStr[i] != ']' {
				i++
			}
			result[key] = parseGemma4Value(argsStr[valStart:i])
		}
	}

	return result
}

// parseGemma4Array 解析 Gemma4 数组内容为切片 (对应 Python 的 _parse_gemma4_array)
func parseGemma4Array(arrStr string) []any {
	if arrStr == "" {
		return nil
	}

	items := make([]any, 0)
	i := 0
	n := len(arrStr)

	for i < n {
		// 跳过空白和逗号
		for i < n && (arrStr[i] == ' ' || arrStr[i] == ',' || arrStr[i] == '\n' || arrStr[i] == '\t') {
			i++
		}
		if i >= n {
			break
		}

		// 字符串元素
		if strings.HasPrefix(arrStr[i:], gemma4StringDelim) {
			i += len(gemma4StringDelim)
			endPos := strings.Index(arrStr[i:], gemma4StringDelim)
			if endPos == -1 {
				items = append(items, arrStr[i:])
				break
			}
			items = append(items, arrStr[i:i+endPos])
			i = i + endPos + len(gemma4StringDelim)
		} else if arrStr[i] == '{' {
			// 嵌套对象
			depth := 1
			objStart := i + 1
			i++
			for i < n && depth > 0 {
				if strings.HasPrefix(arrStr[i:], gemma4StringDelim) {
					i += len(gemma4StringDelim)
					nextDelim := strings.Index(arrStr[i:], gemma4StringDelim)
					if nextDelim == -1 {
						i = n
						break
					}
					i = i + nextDelim + len(gemma4StringDelim)
					continue
				}
				if arrStr[i] == '{' {
					depth++
				} else if arrStr[i] == '}' {
					depth--
				}
				i++
			}
			items = append(items, parseGemma4Args(arrStr[objStart:i-1]))
		} else if arrStr[i] == '[' {
			// 嵌套数组 - 需要检查字符串分隔符避免误判
			depth := 1
			subStart := i + 1
			i++
			for i < n && depth > 0 {
				// 检查字符串分隔符，跳过字符串内容
				if strings.HasPrefix(arrStr[i:], gemma4StringDelim) {
					i += len(gemma4StringDelim)
					nextDelim := strings.Index(arrStr[i:], gemma4StringDelim)
					if nextDelim == -1 {
						i = n
						break
					}
					i = i + nextDelim + len(gemma4StringDelim)
					continue
				}
				if arrStr[i] == '[' {
					depth++
				} else if arrStr[i] == ']' {
					depth--
				}
				i++
			}
			items = append(items, parseGemma4Array(arrStr[subStart:i-1]))
		} else {
			// 裸值
			valStart := i
			for i < n && arrStr[i] != ',' && arrStr[i] != ']' {
				i++
			}
			items = append(items, parseGemma4Value(arrStr[valStart:i]))
		}
	}

	return items
}

// ToolCallWithArgs 包含函数名和参数的完整工具调用
type ToolCallWithArgs struct {
	Name      string
	Arguments map[string]any
}

// Parse 解析完整的工具调用内容 (对应 Python 的 extract_tool_calls)
func (p *gemma4Parser) Parse(content string) (*ParsedToolCall, error) {
	// 检查是否包含工具调用标记
	if !strings.Contains(content, p.StartTag()) || !strings.Contains(content, p.EndTag()) {
		return nil, fmt.Errorf("missing tool call tags")
	}

	// 移除开始和结束标签
	inner := strings.TrimPrefix(content, p.StartTag())
	inner = strings.TrimSuffix(inner, p.EndTag())
	inner = strings.TrimSpace(inner)

	if inner == "" {
		return nil, fmt.Errorf("empty tool call")
	}

	// 使用正则提取所有函数名和参数部分
	// 匹配格式: call:func_name{args}
	// 支持函数名包含字母、数字、下划线、连字符和点
	funcRe := regexp.MustCompile(`call:([a-zA-Z_][a-zA-Z0-9_\-\.]*)\{(.*?)\}`)
	matches := funcRe.FindAllStringSubmatch(inner, -1)

	if len(matches) == 0 {
		return nil, fmt.Errorf("invalid gemma4 format: %s", inner)
	}

	// 处理多个工具调用 - 参考 Python 实现
	var toolCalls []ToolCallWithArgs

	for _, match := range matches {
		if len(match) < 3 {
			continue
		}

		funcName := match[1]
		argsStr := match[2]

		// 解析参数为 map (保持原生类型)
		argsMap := parseGemma4Args(argsStr)

		toolCalls = append(toolCalls, ToolCallWithArgs{
			Name:      funcName,
			Arguments: argsMap,
		})
	}

	if len(toolCalls) == 0 {
		return nil, fmt.Errorf("no valid tool call found")
	}

	// 只返回第一个工具调用 (与原有 ParsedToolCall 结构兼容)
	firstCall := toolCalls[0]

	// 将 Arguments map 转换为 Args 切片，每个参数一个 ToolCallArg
	// Value 直接存储原生类型（bool, int, float64, map, slice）
	var args []ToolCallArg
	for key, value := range firstCall.Arguments {
		args = append(args, ToolCallArg{
			Key:   key,
			Value: value,
		})
	}

	return &ParsedToolCall{
		Name: firstCall.Name,
		Args: args,
	}, nil
}
