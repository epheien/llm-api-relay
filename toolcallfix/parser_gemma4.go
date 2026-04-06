package toolcallfix

import (
	"fmt"
	"regexp"
	"strings"
)

// gemma4Parser Gemma4 格式解析器
// 格式: <|tool_call>call:func_name{key:<|"|>value<|"|>}<tool_call|>
type gemma4Parser struct{}

func (p *gemma4Parser) Name() string     { return "gemma4" }
func (p *gemma4Parser) StartTag() string { return "<|tool_call>" }
func (p *gemma4Parser) EndTag() string   { return "<tool_call|>" }

func (p *gemma4Parser) Parse(content string) (*ParsedToolCall, error) {
	// 移除开始和结束标签
	inner := strings.TrimPrefix(content, p.StartTag())
	inner = strings.TrimSuffix(inner, p.EndTag())
	inner = strings.TrimSpace(inner)

	if inner == "" {
		return nil, fmt.Errorf("empty tool call")
	}

	// 解析函数名: call:func_name{...}
	// 使用正则提取函数名和参数部分
	funcRe := regexp.MustCompile(`^call:([a-zA-Z_][a-zA-Z0-9_\-\.]*)\{(.*)\}$`)
	match := funcRe.FindStringSubmatch(inner)

	if match == nil {
		return nil, fmt.Errorf("invalid gemma4 format: %s", inner)
	}

	funcName := match[1]
	argsStr := match[2]

	// 解析参数: key:<|"|>value<|"|>
	var args []ToolCallArg
	argRe := regexp.MustCompile(`([a-zA-Z_][a-zA-Z0-9_\-\.]*):<\|"\|>(.*?)<\|"\|>`)
	argMatches := argRe.FindAllStringSubmatch(argsStr, -1)

	for _, m := range argMatches {
		if len(m) == 3 {
			args = append(args, ToolCallArg{
				Key:   m[1],
				Value: m[2],
			})
		}
	}

	return &ParsedToolCall{
		Name: funcName,
		Args: args,
	}, nil
}
