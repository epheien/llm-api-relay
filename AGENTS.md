# AGENTS.md - LLM API Relay 项目

## 概览

这是一个基于 Go 语言的 OpenAI API 兼容代理服务，作为大语言模型 (LLM) API 的代理。该服务提供 OpenAI 兼容的端点，同时将请求转发到上游 LLM 服务，并支持可选的请求转换。

**项目目标**: 实现无缝切换不同 LLM 提供商，同时为客户端应用保持 OpenAI API 兼容性。

## 快速启动命令

### 构建和运行
```bash
# 直接运行 (需要 Go 1.18+)
go run main.go

# 先构建二进制文件
go build -o llm-relay main.go
./llm-relay --config config.jsonc

# 简写形式
./llm-relay -c config.jsonc

# 使用自定义配置路径
go run main.go --config custom-config.jsonc
```

### 依赖项
- 仅使用标准 Go 库（无外部依赖）
- Go 1.18+（通过自定义解析器支持 JSONC）

### Makefile 管理命令
```bash
# 查看所有可用命令
make help

# 构建所有二进制文件
make build

# 构建主服务二进制
make build-main

# 构建测试工具二进制
make build-test

# 构建测试运行器二进制
make build-runner

# 运行所有测试
make test

# 运行单元测试
make test-unit

# 运行集成测试
make test-integration

# 代码规范检查
make lint

# 格式化代码
make fmt

# 代码静态分析
make vet

# 运行主服务
make run

# 运行测试工具
make run-test

# 完整构建和测试流程
make all

# 开发模式（需要手动重新构建）
make dev

# 安装到系统（需要 sudo 权限）
make install-system

# 从系统卸载
make uninstall-system

# 查看版本信息
make version
```

## 项目结构

```
llm-api-relay/
├── main.go                         # 主应用程序代码
├── main_test.go                    # 主要功能的单元测试
├── toolcall_parser.go             # 工具调用解析逻辑
├── toolcallfix/                   # 工具调用修复转换包
│   ├── transform.go               # XML 到 OpenAI tool_calls 转换
│   └── transform_test.go          # 转换逻辑的测试
├── cmd/                           # 命令行工具
│   ├── relay-test.go             # 主服务测试工具
│   ├── test-runner.go            # 测试执行运行器
│   └── README.md                  # CLI 工具文档
├── spec.md                        # OpenAI API 兼容性规范
├── config.jsonc                   # 配置文件
├── TEST_USAGE.md                  # 测试程序使用指南
├── TOOLCALLFIX_USAGE.md          # 工具调用修复功能指南
├── Makefile                       # 多二进制构建管理
├── go.mod                         # Go 模块定义 (Go 1.25.1)
└── AGENTS.md                      # 本文件
```

## 配置系统

### JSONC 格式
该项目使用 **JSONC**（带注释的 JSON）进行配置：

**示例 config.jsonc:**
```jsonc
{
  "listen": ":8080",           // 监听地址
  "upstream": "http://localhost:11434/v1", // 上游 API 基础地址
  "forward_auth": false,         // 是否转发 Authorization 头
  "model_rules": [              // 模型特定转换规则
    {
      "match_model": "gpt-3.5-turbo",
      "set": {
        "model": "qwen2.5-72b-instruct"
      },
      "extra": {
        "custom_param": "value"
      },
      "unset": ["temperature"],  // 从请求中移除字段
      "enable_toolcallfix": true  // 启用工具调用修复
    },
    {
      "match_model": "default",  // 回退规则
      "set": {
        "model": "default-model"
      }
    }
  ]
}
```

### JSONC 特性
- 支持 `//` 行注释
- 支持 `/* 块注释 */`
- 在 JSON 解析前自动剥离

### 高级配置选项
**工具调用修复控制:**
```jsonc
{
  "match_model": "model-with-tool-calls",
  "enable_toolcallfix": true,    // 启用 XML 工具调用转换
  "set": {
    "model": "upstream-model"
  }
}
```

## API 端点

### OpenAI 兼容端点

**1. 模型列表**
```
GET /v1/models
```
- 直接代理到上游
- 不修改请求

**2. 聊天完成**
```
POST /v1/chat/completions
```
- 支持流式和非流式
- 应用模型规则进行请求转换
- 支持 OpenAI 兼容的请求/响应格式
- 包含工具调用修复功能

**3. 传统完成**
```
POST /v1/completions
```
- 与聊天完成相同，但应用规则转换

### 服务端点

**健康检查**
```
GET /health
```
- 返回 "ok" - 用于监控

## 代码模式和约定

### 函数组织
- **主服务器设置**: 第 73-80 行
- **HTTP 处理器**: 第 50-71 行
- **中间件**: 第 82-88 行
- **配置加载**: 第 90-107 行
- **JSONC 解析**: 第 111-183 行
- **规则应用**: 第 185-236 行
- **代理函数**: 第 238-407 行

### 关键模式

**1. 基于规则的请求转换**
```go
func applyRules(cfg *Config, req map[string]any) {
    model := getString(req, "model")
    rule := findRule(cfg.ModelRules, model)
    // 应用 unset -> set -> extra 转换
}
```

**2. 流式响应处理**
- 检测请求中的 `stream: true`
- 使用 `http.Flusher` 进行分块响应
- 逐行流式传输和适当的刷新

**3. 头部管理**
- 移除逐跳头部 (Connection, Proxy-*)
- 保留大多数头部包括自定义头部
- 可选的身份验证转发

**4. 工具调用解析和修复**
```go
// 解析工具调用语法
func ParseToolCallsFromContent(content string) ([]ToolCall, error)

// XML 到 OpenAI 格式转换
func TransformToolCalls(content string) (string, error)
```

### 错误处理
- HTTP 状态码: 400 (Bad Request), 405 (Method Not Allowed), 502 (Bad Gateway)
- 响应中的详细错误信息
- 使用 `defer` 进行适当的资源清理

## 模型规则系统

### 规则匹配
- 精确的模型名称匹配
- "default" 规则作为回退
- 首个匹配优先

### 转换类型

**1. Set（顶级字段）**
```go
{"set": {"model": "new-model", "temperature": 0.7}}
```

**2. Extra（嵌套对象）**
```go
{"extra": {"custom_param": "value"}}
```

**3. Unset（移除字段）**
```jsonc
{"unset": ["temperature", "presence_penalty"]}
```

### 应用顺序
1. **Unset** - 移除指定的字段
2. **Set** - 添加/修改顶级字段
3. **Extra** - 合并到嵌套的 "extra" 对象

## 流式实现

### 检测
```go
stream := false
if v, ok := payload["stream"].(bool); ok && v {
    stream = true
}
```

### 响应处理
- 使用 `bufio.NewReader` 进行基于行的流式传输
- Flusher 接口用于适当的 SSE 支持
- 优雅的 EOF 处理
- 如果 Flusher 不可用则回退到简单复制

### 工具调用修复流式处理
**XML 格式转换:**
```xml
<tool_call>function_name<arg_key>key1</arg_key><arg_value>value1</arg_value></tool_call>
```

**转换为 OpenAI 格式:**
```json
{
  "id": "chatcmpl-tool-xxx",
  "type": "function",
  "function": {
    "name": "function_name",
    "arguments": "{\"key1\": \"value1\"}"
  }
}
```

## 工具调用解析器

### 支持的语法格式
```go
// 格式1: function_name(arg1="value1", arg2="value2")
// 格式2: function_name arg1="value1" arg2="value2"  
// 格式3: function_name: arg1="value1", arg2="value2"
```

### ToolCall 结构
```go
type ToolCall struct {
    ID       string `json:"id"`
    Type     string `json:"type"`
    Index    int    `json:"index"`
    Function struct {
        Name      string `json:"name"`
        Arguments string `json:"arguments"`
    } `json:"function"`
}
```

## 常见用例

### 1. 模型转换
将客户端模型名称转换为上游模型名称：
```jsonc
{
  "match_model": "gpt-4",
  "set": {
    "model": "local-llm-model-name"
  }
}
```

### 2. 参数过滤
移除某些模型不支持的参数：
```jsonc
{
  "match_model": "legacy-model",
  "unset": ["presence_penalty", "frequency_penalty"]
}
```

### 3. 添加模型特定参数
```jsonc
{
  "match_model": "custom-model",
  "extra": {
    "generation_params": {
      "temperature": 0.8,
      "top_p": 0.9
    }
  }
}
```

### 4. 工具调用修复启用
```jsonc
{
  "match_model": "model-with-tools",
  "enable_toolcallfix": true,
  "set": {
    "model": "upstream-model-with-tool-support"
  }
}
```

## 测试和开发

### 单元测试
```bash
# 运行单元测试
make test-unit
go test -v ./main_test.go ./main.go
go test -v ./toolcallfix/...
```

### 集成测试
```bash
# 运行集成测试
make test-integration
go test -v ./toolcallfix_integration_test.go ./main.go ./toolcallfix_integration_test.go
```

### 手动测试
```bash
# 启动服务
go run main.go --config config.jsonc

# 测试健康端点
curl http://localhost:8080/health

# 测试模型端点
curl http://localhost:8080/v1/models

# 测试聊天完成
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "test-model",
    "messages": [{"role": "user", "content": "Hello"}],
    "stream": false
  }'

# 测试流式聊天完成
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "test-model",
    "messages": [{"role": "user", "content": "Hello"}],
    "stream": true
  }'
```

### 使用测试工具
```bash
# 构建测试工具
make build-test
./bin/relay-test

# 运行测试运行器
make build-runner
./bin/test-runner
```

### 调试
- 所有请求都记录方法、路径和持续时间
- 检查配置解析错误
- 验证上游连接性
- 监控流式响应的分块问题
- 检查工具调用修复是否正确工作

## 常见问题和解决方案

### 1. 配置未加载
- 检查 JSONC 语法（注释格式正确）
- 验证 `upstream` 字段存在且为有效 URL
- 确保文件路径正确

### 2. 流式传输不工作
- 验证客户端发送 `"stream": true`
- 检查上游是否支持流式传输
- 确保响应中适当刷新

### 3. 模型规则未应用
- 验证模型名称完全匹配（区分大小写）
- 检查 "default" 规则作为回退
- 确认规则结构为有效 JSON

### 4. 身份验证问题
- 设置 `forward_auth: true` 以传递 Authorization 头
- 否则 Authorization 头被剥离

### 5. 工具调用修复问题
- 检查 `enable_toolcallfix` 是否在模型规则中设置为 true
- 验证上游模型支持工具调用
- 检查 XML 工具调用格式是否正确

## 重要注意事项

### 安全
- 在服务级别未实现身份验证
- 依赖上游身份验证
- 如果公开暴露，考虑添加身份验证层

### 性能
- 流式响应可以是长生命周期的
- 没有内置的速率限制
- 考虑在高负载下进行上游连接池化

### 兼容性
- 核心端点的严格 OpenAI API 兼容性
- 响应结构匹配 OpenAI 格式
- 支持大多数常见的 OpenAI 参数

### 工具调用兼容性
- 支持 OpenAI 工具调用格式
- XML 工具调用自动转换为标准格式
- 保持流式响应中的工具调用顺序

### 依赖管理
- 使用 Go 模块进行依赖管理
- 仅使用 github.com/google/uuid 作为外部依赖
- Go 1.25.1 版本要求

## 未来增强

潜在改进领域：
- 每客户端速率限制
- 请求/响应日志记录
- 指标和监控
- 身份验证层
- 连接池化
- 多个上游负载均衡
- 更精细的模型规则匹配
- 动态配置重载
- Prometheus 指标导出
- 请求缓存
- 代理负载均衡

## 开发指南

### 代码风格
- 使用标准 Go 代码约定
- 通过 `make fmt` 格式化代码
- 通过 `make vet` 进行静态分析
- 使用有意义的变量和函数名

### 测试策略
- 单元测试覆盖核心功能
- 集成测试验证端到端功能
- 工具调用修复的专门测试
- 配置文件解析的测试

### 贡献指南
1. 确保所有测试通过：`make test`
2. 代码规范检查：`make lint`
3. 格式化代码：`make fmt`
4. 更新相关文档
5. 添加适当的测试覆盖

### 构建优化
- 使用 Go 1.25.1 或更高版本
- 构建标志优化二进制大小
- 考虑使用 build tags 分离功能
- 多平台交叉编译支持