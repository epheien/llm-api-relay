package toolcallfix

// ToolCallParser 工具调用解析器接口
type ToolCallParser interface {
	// Name 返回解析器名称
	Name() string
	// StartTag 返回工具调用开始标签
	StartTag() string
	// EndTag 返回工具调用结束标签
	EndTag() string
	// Parse 解析完整的工具调用内容
	Parse(content string) (*ParsedToolCall, error)
}

// parserRegistry 解析器注册表
var parserRegistry = map[string]ToolCallParser{}

// RegisterParser 注册解析器
func RegisterParser(name string, parser ToolCallParser) {
	parserRegistry[name] = parser
}

// GetParser 返回指定名称的解析器，如果不存在则返回默认解析器
func GetParser(name string) ToolCallParser {
	if p, ok := parserRegistry[name]; ok {
		return p
	}
	// 默认返回 XML 解析器
	return parserRegistry["xml"]
}
