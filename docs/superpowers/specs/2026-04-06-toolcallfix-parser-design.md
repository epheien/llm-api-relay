# ToolCallFix 解析器架构设计

## 概述

本文档描述了如何扩展 ToolCallFix 功能以支持多种工具调用格式，类似 vLLM 的 `--tool-call-parser` 选项。

## 背景

当前 toolcallfix 功能仅支持一种 XML 格式：

```xml
<tool_call>func_name<arg_key>key</arg_key><arg_value>value</arg_value></tool_call>
```

需要扩展以支持更多格式，例如 Gemma4 模型的格式：

```
<|tool_call>call:func_name{key:<|"|>value<|"|>}<tool_call|>
```

## 目标

1. 通过配置指定使用的解析器（而非自动检测）
2. 保持向后兼容，现有配置无需修改
3. 预留扩展接口以便未来支持更多格式

## 架构设计

### 1. 策略模式

```
┌─────────────────────────────────────────────────────────────┐
│                    StreamTransformer                        │
├─────────────────────────────────────────────────────────────┤
│  • parser: ToolCallParser    (由配置指定的解析器)            │
│  • buffer: strings.Builder                                    │
│  • inToolCall: bool                                          │
│  • toolCallIndex: int                                        │
│  • lastChunk: *ChatCompletionChunk                           │
└─────────────────────────────────────────────────────────────┘
                            │
          ┌─────────────────┼─────────────────┐
          ▼                 ▼                 ▼
   ┌────────────┐    ┌────────────┐    ┌────────────┐
   │XMLParser   │    │Gemma4Parser│    │ (预留)     │
   │(现有)      │    │(新)        │    │            │
   └────────────┘    └────────────┘    └────────────┘
```

### 2. 接口定义

```go
// ToolCallParser 工具调用解析器接口
type ToolCallParser interface {
    // Name 返回解析器名称，用于配置和日志
    Name() string
    
    // StartTag 返回工具调用开始标签
    StartTag() string
    
    // EndTag 返回工具调用结束标签
    EndTag() string
    
    // Parse 解析完整的工具调用内容
    // content 包含从 StartTag 到 EndTag 的完整内容
    // 返回解析后的工具调用和错误
    Parse(content string) (*ParsedToolCall, error)
}
```

### 3. ParsedToolCall 结构

保持现有结构不变：

```go
type ParsedToolCall struct {
    Name string
    Args []ToolCallArg
}

type ToolCallArg struct {
    Key   string
    Value string
}
```

### 4. 解析器实现

#### XMLParser (现有格式)

格式：
```xml
<tool_call>func_name<arg_key>key</arg_key><arg_value>value</arg_value></tool_call>
```

实现要点：
- StartTag: `<tool_call>`
- EndTag: `</tool_call>`
- 使用正则表达式解析 `arg_key` / `arg_value` 标签对

#### Gemma4Parser (新格式)

格式：
```
<|tool_call>call:func_name{key:<|"|>value<|"|>}<tool_call|>
```

实现要点：
- StartTag: `<|tool_call>`
- EndTag: `<tool_call|>`
- 解析 `call:func_name{...}` 格式
- 提取函数名（冒号后的部分）
- 使用 `<|"|>` 和 `<|"|>` 作为字符串定界符
- 支持 `key:value` 格式的参数（无引号）

### 5. 配置扩展

在 `ModelRule` 结构体中添加新字段：

```go
type ModelRule struct {
    MatchModel        string         `json:"match_model"`
    Set               map[string]any `json:"set"`
    Extra             map[string]any `json:"extra"`
    Unset             []string       `json:"unset"`
    EnableToolCallFix bool           `json:"enable_toolcallfix"`
    ToolCallParser    string         `json:"tool_call_parser"` // 新增字段
}
```

配置示例：

```jsonc
{
  "listen": "0.0.0.0:8080",
  "upstream": "http://localhost:11434/v1",
  "model_rules": [
    {
      "match_model": "gemma4-model",
      "enable_toolcallfix": true,
      "tool_call_parser": "gemma4"
    },
    {
      "match_model": "qwen2.5-72b-instruct",
      "enable_toolcallfix": true,
      "tool_call_parser": "xml"
    },
    {
      "match_model": "default",
      "enable_toolcallfix": true
      // tool_call_parser 默认为 "xml"
    }
  ]
}
```

### 6. 解析器注册机制

```go
var parsers = map[string]ToolCallParser{
    "xml":    NewXMLParser(),
    "gemma4": NewGemma4Parser(),
}

// GetParser 返回指定名称的解析器，如果不存在则返回默认解析器
func GetParser(name string) ToolCallParser {
    if p, ok := parsers[name]; ok {
        return p
    }
    return parsers["xml"]  // 默认使用 xml
}
```

### 7. StreamTransformer 修改

#### 7.1 新增字段

```go
type StreamTransformer struct {
    parser        ToolCallParser  // 新增：当前使用的解析器
    buffer        strings.Builder
    inToolCall    bool
    lastChunk     *ChatCompletionChunk
    toolCallIndex int
}
```

#### 7.2 新增构造函数

```go
func NewStreamTransformer(parserName string) *StreamTransformer {
    return &StreamTransformer{
        parser: GetParser(parserName),
    }
}
```

#### 7.3 流式检测逻辑修改

现有代码（硬编码）：
```go
if strings.Contains(content, "<tool_call>") {
    t.inToolCall = true
    // ...
}
```

修改为：
```go
if strings.Contains(content, t.parser.StartTag()) {
    t.inToolCall = true
    // ...
}
```

类似地，结束标签检测也使用 `t.parser.EndTag()`。

#### 7.4 解析逻辑修改

现有代码：
```go
parsed, err := parseToolCallXML(buffered)
```

修改为：
```go
parsed, err := t.parser.Parse(buffered)
```

### 8. 调用链修改

#### main.go 中的调用

现有代码：
```go
if rule.EnableToolCallFix {
    // 直接调用 TransformStream
    toolcallfix.TransformStream(resp.Body, w)
}
```

修改为：
```go
if rule.EnableToolCallFix {
    parserName := rule.ToolCallParser  // 如果为空则使用默认值
    toolcallfix.TransformStreamWithParser(resp.Body, w, parserName)
}
```

或者在 TransformStream 内部获取解析器：

```go
func TransformStream(input io.Reader, output io.Writer, parserName string) error {
    transformer := NewStreamTransformer(parserName)
    // ...
}
```

## 错误处理

1. **解析失败**：返回原始 content（现有行为不变）
2. **未知解析器名称**：使用默认的 XML 解析器
3. **空的 parser 配置**：使用默认的 XML 解析器

## 测试策略

### 单元测试

1. **XMLParser 测试**：复用现有测试
2. **Gemma4Parser 测试**：
   - 基础解析测试
   - 多种参数测试
   - 特殊字符处理测试
   - 流式场景测试

### 集成测试

1. 配置 gemma4 解析器的端到端测试
2. 多个模型规则不同解析器的测试

## 文件变更清单

| 文件 | 变更 |
|------|------|
| `toolcallfix/transform.go` | 添加解析器接口和实现 |
| `toolcallfix/parser_xml.go` | 移动现有解析逻辑 |
| `toolcallfix/parser_gemma4.go` | 新增 Gemma4 解析器 |
| `toolcallfix/transform_test.go` | 扩展测试 |
| `main.go` | 添加配置字段和调用链修改 |
| `main_test.go` | 配置解析测试更新 |
| `TOOLCALLFIX_USAGE.md` | 文档更新 |

## 参考资料

- Gemma4 格式规范：[docs/superpowers/reference/gemma4_tool_parser.html](../../reference/gemma4_tool_parser.html)
- vLLM 工具解析器：https://docs.vllm.ai/en/latest/api/vllm/tool_parsers/

## 实施顺序

1. 添加解析器接口和注册机制
2. 实现 XMLParser（重构现有代码）
3. 实现 Gemma4Parser
4. 修改 StreamTransformer 以使用解析器
5. 修改 main.go 配置和调用链
6. 添加单元测试
7. 更新文档