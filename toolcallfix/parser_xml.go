package toolcallfix

import (
	"fmt"
	"regexp"
	"strings"
)

// xmlParser XML 格式解析器
type xmlParser struct{}

func (p *xmlParser) Name() string     { return "xml" }
func (p *xmlParser) StartTag() string { return "<tool_call>" }
func (p *xmlParser) EndTag() string   { return "</tool_call>" }

func (p *xmlParser) Parse(content string) (*ParsedToolCall, error) {
	// 移除开始和结束标签
	inner := strings.TrimPrefix(content, p.StartTag())
	inner = strings.TrimSuffix(inner, p.EndTag())
	inner = strings.TrimSpace(inner)

	if inner == "" {
		return nil, fmt.Errorf("empty tool call")
	}

	// 提取函数名 (在第一个 <arg_key> 之前)
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

	// 解析参数
	var args []ToolCallArg
	argKeyRe := regexp.MustCompile(`(?s)<arg_key>(.*?)</arg_key>\s*<arg_value>(.*?)</arg_value>`)
	matches := argKeyRe.FindAllStringSubmatch(argsSection, -1)

	for _, match := range matches {
		if len(match) == 3 {
			args = append(args, ToolCallArg{
				Key:   strings.TrimSpace(match[1]),
				Value: match[2],
			})
		}
	}

	return &ParsedToolCall{
		Name: name,
		Args: args,
	}, nil
}
