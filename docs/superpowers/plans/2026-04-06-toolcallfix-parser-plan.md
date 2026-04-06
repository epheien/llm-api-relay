# ToolCallFix 多解析器支持实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 通过配置指定解析器，支持多种工具调用格式（XML、Gemma4）

**Architecture:** 使用策略模式，将解析逻辑抽象为 ToolCallParser 接口，通过注册机制支持多种解析器

**Tech Stack:** Go 1.25, 标准库 (regexp, strings)

---

## 文件结构变更

| 文件 | 变更 |
|------|------|
| `toolcallfix/parser.go` | 新建：解析器接口和注册机制 |
| `toolcallfix/parser_xml.go` | 新建：XML 解析器（从 transform.go 移动） |
| `toolcallfix/parser_gemma4.go` | 新建：Gemma4 解析器 |
| `toolcallfix/transform.go` | 修改：使用解析器接口 |
| `main.go` | 修改：添加配置字段和调用链 |
| `main_test.go` | 修改：配置解析测试更新 |
| `TOOLCALLFIX_USAGE.md` | 修改：文档更新 |

---

### Task 1: 添加解析器接口和注册机制

**Files:**
- Create: `toolcallfix/parser.go`

- [ ] **Step 1: 创建 parser.go 文件**

```go
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

// init 注册内置解析器
func init() {
    RegisterParser("xml", &xmlParser{})
    RegisterParser("gemma4", &gemma4Parser{})
}
```

- [ ] **Step 2: 提交**

```bash
git add toolcallfix/parser.go
git commit -m "feat: add ToolCallParser interface and registry"
```

---

### Task 2: 实现 XML 解析器

**Files:**
- Create: `toolcallfix/parser_xml.go`
- Modify: `toolcallfix/transform.go:86-129` (删除 parseToolCallXML 函数)

- [ ] **Step 1: 创建 parser_xml.go**

```go
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
```

- [ ] **Step 2: 从 transform.go 删除 parseToolCallXML 函数**

在 transform.go 中删除第 86-129 行（parseToolCallXML 函数）

- [ ] **Step 3: 运行测试确保没有破坏现有功能**

```bash
go test -v ./toolcallfix/... -run "XML" 
```

- [ ] **Step 4: 提交**

```bash
git add toolcallfix/parser_xml.go toolcallfix/transform.go
git commit -m "refactor: extract XML parser to separate file"
```

---

### Task 3: 实现 Gemma4 解析器

**Files:**
- Create: `toolcallfix/parser_gemma4.go`

- [ ] **Step 1: 创建 parser_gemma4.go**

```go
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
```

- [ ] **Step 2: 运行测试确认编译通过**

```bash
go build ./toolcallfix/...
```

- [ ] **Step 3: 提交**

```bash
git add toolcallfix/parser_gemma4.go
git commit -m "feat: add Gemma4 parser implementation"
```

---

### Task 4: 修改 StreamTransformer 使用解析器

**Files:**
- Modify: `toolcallfix/transform.go`

- [ ] **Step 1: 修改 StreamTransformer 结构体**

在 transform.go 中找到 StreamTransformer 定义，添加 parser 字段：

```go
type StreamTransformer struct {
    parser        ToolCallParser  // 新增字段
    buffer        strings.Builder
    inToolCall    bool
    lastChunk     *ChatCompletionChunk
    toolCallIndex int
}
```

- [ ] **Step 2: 修改 NewStreamTransformer 函数**

```go
func NewStreamTransformer(parserName string) *StreamTransformer {
    return &StreamTransformer{
        parser: GetParser(parserName),
    }
}
```

- [ ] **Step 3: 修改 TransformLine 方法中的标签检测**

找到包含 `<tool_call>` 的代码，修改为使用 parser：

```go
// 修改前:
if strings.Contains(content, "<tool_call>") {

// 修改后:
if strings.Contains(content, t.parser.StartTag()) {
```

同样修改结束标签检测：

```go
// 修改前:
if strings.Contains(t.buffer.String(), "</tool_call>") {

// 修改后:
if strings.Contains(t.buffer.String(), t.parser.EndTag()) {
```

- [ ] **Step 4: 修改 flushToolCall 方法中的解析调用**

```go
// 修改前:
parsed, err := parseToolCallXML(buffered)

// 修改后:
parsed, err := t.parser.Parse(buffered)
```

- [ ] **Step 5: 修改 TransformStream 函数签名和调用**

```go
func TransformStream(input io.Reader, output io.Writer, parserName string) error {
    transformer := NewStreamTransformer(parserName)
    // ... 保持其他代码不变
}
```

- [ ] **Step 6: 运行测试**

```bash
go test -v ./toolcallfix/...
```

- [ ] **Step 7: 提交**

```bash
git add toolcallfix/transform.go
git commit -m "refactor: integrate parser interface into StreamTransformer"
```

---

### Task 5: 修改 main.go 配置和调用链

**Files:**
- Modify: `main.go`

- [ ] **Step 1: 在 ModelRule 结构体添加 ToolCallParser 字段**

找到 ModelRule 定义（大约第 51 行），添加新字段：

```go
type ModelRule struct {
    MatchModel        string         `json:"match_model"`
    Set               map[string]any `json:"set"`
    Extra             map[string]any `json:"extra"`
    Unset             []string       `json:"unset"`
    EnableToolCallFix bool           `json:"enable_toolcallfix"`
    ToolCallParser    string         `json:"tool_call_parser"` // 新增
}
```

- [ ] **Step 2: 修改 applyRules 函数以传递 parser 名称**

找到 applyRules 函数调用 toolcallfix.TransformStream 的地方，修改为：

```go
// 获取解析器名称
parserName := "xml"  // 默认
if rule.ToolCallParser != "" {
    parserName = rule.ToolCallParser
}

toolcallfix.TransformStream(resp.Body, w, parserName)
```

注意：需要先找到该调用的具体位置并修改。

- [ ] **Step 3: 运行测试**

```bash
go build ./... && go test -v ./...
```

- [ ] **Step 4: 提交**

```bash
git add main.go
git commit -m "feat: add tool_call_parser config field and pass to TransformStream"
```

---

### Task 6: 更新单元测试

**Files:**
- Modify: `toolcallfix/transform_test.go`

- [ ] **Step 1: 添加 Gemma4Parser 单元测试**

在 transform_test.go 添加：

```go
func TestGemma4Parser_Parse(t *testing.T) {
    parser := &gemma4Parser{}
    
    tests := []struct {
        name     string
        input    string
        expected *ParsedToolCall
        hasError bool
    }{
        {
            name:  "simple function call",
            input: "<|tool_call>call:glob{pattern:<|"|>*.go<|"|>}<tool_call|>",
            expected: &ParsedToolCall{
                Name: "glob",
                Args: []ToolCallArg{
                    {Key: "pattern", Value: "*.go"},
                },
            },
            hasError: false,
        },
        {
            name:  "multiple arguments",
            input: "<|tool_call>call:func{arg1:<|"|>value1<|"|>,arg2:<|"|>value2<|"|>}<tool_call|>",
            expected: &ParsedToolCall{
                Name: "func",
                Args: []ToolCallArg{
                    {Key: "arg1", Value: "value1"},
                    {Key: "arg2", Value: "value2"},
                },
            },
            hasError: false,
        },
        {
            name:     "invalid format",
            input:    "<|tool_call>invalid<tool_call|>",
            expected: nil,
            hasError: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := parser.Parse(tt.input)
            
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
            }
        })
    }
}
```

- [ ] **Step 2: 添加 GetParser 测试**

```go
func TestGetParser(t *testing.T) {
    // 测试获取已注册的解析器
    xmlParser := GetParser("xml")
    if xmlParser.Name() != "xml" {
        t.Errorf("expected xml parser, got %s", xmlParser.Name())
    }

    gemma4Parser := GetParser("gemma4")
    if gemma4Parser.Name() != "gemma4" {
        t.Errorf("expected gemma4 parser, got %s", gemma4Parser.Name())
    }

    // 测试未知解析器返回默认
    defaultParser := GetParser("unknown")
    if defaultParser.Name() != "xml" {
        t.Errorf("expected default xml parser, got %s", defaultParser.Name())
    }
}
```

- [ ] **Step 3: 运行测试**

```bash
go test -v ./toolcallfix/... -run "Gemma4|GetParser"
```

- [ ] **Step 4: 提交**

```bash
git add toolcallfix/transform_test.go
git commit -m "test: add Gemma4 parser and GetParser tests"
```

---

### Task 7: 更新文档

**Files:**
- Modify: `TOOLCALLFIX_USAGE.md`

- [ ] **Step 1: 添加新配置选项说明**

在 TOOLCALLFIX_USAGE.md 中添加 `tool_call_parser` 配置说明：

```markdown
## 解析器配置

使用 `tool_call_parser` 指定工具调用解析器：

### 支持的解析器

| 解析器 | 格式 | 示例 |
|--------|------|------|
| `xml` (默认) | XML 标签格式 | `<tool_call>func<arg_key>k</arg_key><arg_value>v</arg_value></tool_call>` |
| `gemma4` | Gemma4 格式 | `<\|tool_call>call:func{k:<\|"\|>v<\|"\|>}<tool_call\|>` |

### 配置示例

```jsonc
{
  "match_model": "gemma4-model",
  "enable_toolcallfix": true,
  "tool_call_parser": "gemma4"
}
```

### 默认行为

如果不指定 `tool_call_parser`，默认使用 `xml` 解析器（保持向后兼容）。
```

- [ ] **Step 2: 提交**

```bash
git add TOOLCALLFIX_USAGE.md
git commit -m "docs: add tool_call_parser configuration documentation"
```

---

## 实施检查

- [ ] Task 1: 解析器接口和注册机制
- [ ] Task 2: XML 解析器实现
- [ ] Task 3: Gemma4 解析器实现
- [ ] Task 4: StreamTransformer 集成
- [ ] Task 5: main.go 配置和调用链
- [ ] Task 6: 单元测试
- [ ] Task 7: 文档更新