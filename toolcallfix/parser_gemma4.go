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

// extractArgsUntilMatch 从指定位置开始，提取到匹配的 } 为止的参数字符串
// 处理嵌套的大括号，核心算法:
//
// 1. 初始化 depth = 1 (表示已遇到一个开括号)
// 2. 从 start 位置向后遍历:
//   - 遇到字符串分隔符 <|"|> 时，跳过整个字符串内容，避免字符串内的 } 被误计
//   - 遇到 { 时 depth++
//   - 遇到 } 时 depth--
//   - depth 归零时表示已找到匹配的 }，退出循环
//
// 3. 返回从 start 到 (i-1) 的内容(不包含最后的 })
func extractArgsUntilMatch(s string, start int) (string, error) {
	depth := 1
	i := start
	n := len(s)

	for i < n && depth > 0 {
		// 检查字符串分隔符，跳过字符串内容
		if strings.HasPrefix(s[i:], gemma4StringDelim) {
			i += len(gemma4StringDelim)
			nextDelim := strings.Index(s[i:], gemma4StringDelim)
			if nextDelim == -1 {
				return "", fmt.Errorf("unterminated string")
			}
			i = i + nextDelim + len(gemma4StringDelim)
			continue
		}
		if s[i] == '{' {
			depth++
		} else if s[i] == '}' {
			depth--
		}
		i++
	}

	if depth != 0 {
		return "", fmt.Errorf("mismatched braces")
	}

	// 返回从 start 到 i-1 (不包括最后的 })
	return s[start : i-1], nil
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

	// 使用正则提取函数名和开括号位置
	// 匹配格式: call:func_name{args}
	// 支持函数名包含字母、数字、下划线、连字符和点
	//
	// 为什么不用纯正则提取参数部分?
	// 原因: Go 的 regexp 包基于 RE2，不支持递归正则或平衡组。
	// 例如对于 input: `nested:{inner:<|"|>value<|"|>},list:[<|"|>a<|"|>]`
	// 正则 `\{.*?\}` 只能匹配到第一个 `}`，无法处理嵌套的大括号。
	//
	// 解决方案: 使用混合方案
	// 1. 正则: 只匹配 `call:func_name{` 部分，获取函数名和 { 的位置
	// 2. 手动解析: 从 { 之后开始，使用大括号计数器匹配到对应的 }
	//    - 需要跳过字符串分隔符 <|"|>，避免字符串内的 } 被误计
	//    - 维护大括号深度 depth，depth 归零时即找到匹配的 }
	funcRe := regexp.MustCompile(`call:([a-zA-Z_][a-zA-Z0-9_\-\.]*)\{`)
	// 使用 FindAllStringSubmatchIndex 直接获取位置，避免后续 strings.Index 重复查找
	// 索引格式: [完整开始, 完整结束, 捕获组1开始, 捕获组1结束]
	matches := funcRe.FindAllStringSubmatch(inner, -1)
	matchIndices := funcRe.FindAllStringSubmatchIndex(inner, -1)

	// 手动解析参数，处理嵌套的大括号
	var toolCalls []ToolCallWithArgs
	for i, match := range matches {
		if len(match) < 2 {
			continue
		}

		funcName := match[1]
		// 直接从索引获取位置: matchIndices[i][1] 是完整匹配的结束位置(即 { 之后)
		argsStart := matchIndices[i][1]

		// 手动解析参数，找到匹配的 }
		argsStr, err := extractArgsUntilMatch(inner, argsStart)
		if err != nil {
			continue
		}

		argsMap := parseGemma4Args(argsStr)
		toolCalls = append(toolCalls, ToolCallWithArgs{
			Name:      funcName,
			Arguments: argsMap,
		})
	}

	if len(toolCalls) == 0 {
		return nil, fmt.Errorf("invalid gemma4 format: %s", inner)
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
