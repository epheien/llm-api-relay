# LLM API Relay 测试程序使用指南

## 概述

这是一个Go测试程序，用于测试LLM API Relay服务的基本功能。测试程序会自动检查服务的健康状况和各API端点。

## 测试功能

1. **健康检查端点** - `GET /health`
2. **Models 列表端点** - `GET /v1/models`
3. **Chat Completions 非流模式** - `POST /v1/chat/completions` (stream=false)
4. **Chat Completions 流模式** - `POST /v1/chat/completions` (stream=true)

## 使用方法

### 1. 编译测试程序
```bash
go build -o test_relay test_relay.go
```

### 2. 确保LLM API Relay服务正在运行
```bash
# 在一个终端窗口运行服务
go run main.go --config config.jsonc

# 或者使用编译后的二进制
./llm-relay --config config.jsonc
```

### 3. 运行测试程序

#### 基本用法
```bash
./test_relay
```

#### 指定测试模型
```bash
# 使用完整参数名称
./test_relay --model gpt-oss

# 使用简化参数名称
./test_relay -m gpt-oss

# 不指定时使用默认模型：gpt-oss-120b
```

#### 详细模式
```bash
# 开启详细模式，显示请求和响应内容
./test_relay -v

# 组合使用：指定模型 + 详细模式
./test_relay --model gpt-oss --verbose
```

## 命令行参数

测试程序支持以下命令行参数：

| 参数名 | 简形式 | 默认值 | 说明 |
|--------|--------|--------|------|
| `--model` | `-m` | `gpt-oss-120b` | 指定测试时使用的模型名称 |
| `--verbose` | `-v` | `false` | 详细模式 - 打印请求和响应的详细内容 |

### 注意事项
- 如果指定的模型不存在，测试可能会失败
- 建议先使用 `/v1/models` 端点查看可用的模型列表
- 模型名称必须与配置文件中的 `model_rules` 匹配规则一致

## 详细模式

使用 `-v` 或 `--verbose` 参数可以开启详细模式，显示：

1. **请求详情**：HTTP方法、URL路径、发送的JSON数据
2. **响应详情**：HTTP状态码、响应内容的、完整JSON
3. **调试信息**：每个测试步骤的进度和状态

### 详细模式示例输出

```
LLM API Relay 测试程序启动
服务地址: http://localhost:8080
测试模型: gpt-oss-120b
详细模式: 开启
============================================================

1. 测试健康检查端点...
   📝 请求: GET http://localhost:8080/health
   📝 响应: HTTP 200
   📝 内容: ok

2. 测试 Models 端点...
   📝 请求: GET http://localhost:8080/v1/models
   📝 响应: HTTP 200
   📝 内容:
{"object":"list","data":[{"id":"internlm/internlm2_5-7b-chat","object":"model","created":1234567890,"owned_by":"llm-deploy"}]}

3. 测试 Chat Completions (非流模式)...
   📝 请求: POST http://localhost:8080/v1/chat/completions
   📝 发送数据:
{"model":"gpt-oss-120b","stream":false,"messages":[{"role":"user","content":"你好，请回答一句话"}]}
   📝 响应: HTTP 200
   📝 内容:
{"object":"chat.completion","id":"test-id","created":1234567890,"model":"internlm/internlm2_5-7b-chat","choices":[{"index":0,"message":{"role":"assistant","content":"你好！我在这里并正常工作。"},"finish_reason":"stop"}]}
```

### 详细模式用途

- **调试测试失败**：查看服务器返回的确是什么内容
- **验证数据格式**：确认JSON结构是否符合预期
- **查看模型规则应用**：检查模型名称转换是否生效
- **开发调试**：理解每个HTTP请求和响应的完整流程

## 预期输出

```
LLM API Relay 测试程序启动
服务地址: http://localhost:8080
测试模型: gpt-oss-120b
============================================================
1. 测试健康检查端点...

2. 测试 Models 端点...

3. 测试 Chat Completions (非流模式)...

4. 测试 Chat Completions (流模式)...

============================================================
测试结果汇总:
============================================================
✅ PASS 健康检查: 正常
   详情: 状态码: 200, 响应: ok, 耗时: 45.123ms
✅ PASS Models 列表: 正常
   详情: 状态码: 200, 响应长度: 1024 字节, 耗时: 123.456ms
✅ PASS Chat Completions (非流): 正常
   详情: 状态码: 200, 响应长度: 567 字节, 耗时: 2.345s
✅ PASS Chat Completions (流): 正常
   详情: 状态码: 200, 前 1000 字节包含 15 行, 耗时: 3.456s
============================================================
测试完成: 4/4 通过
🎉 所有测试通过!
```

## 测试详情

### 健康检查测试
- 发送 GET 请求到 `/health`
- 预期响应：状态码 200，内容 "ok"
- 用于快速检查服务是否在线

### Models 列表测试
- 发送 GET 请求到 `/v1/models`
- 预期响应：状态码 200，JSON 包含 `object: "list"` 和 `data` 字段
- 测试获取可用模型列表功能

### Chat Completions 非流模式测试
- 发送 POST 请求到 `/v1/chat/completions`
- 设置 `stream: false`
- 发送简单的对话消息
- 预期响应：状态码 200，JSON 包含完整的响应

### Chat Completions 流模式测试
- 发送 POST 请求到 `/v1/chat/completions`
- 设置 `stream: true`
- 发送对话消息
- 检查响应是否为服务器发送事件 (SSE) 格式

## 故障排查

### 连接被拒绝
- 确认LLM API Relay服务正在运行
- 检查配置文件中的监听地址
- 验证防火墙设置

### 时间超时
- 检查上游服务 (config.jsonc 中的 upstream 字段) 是否可访问
- 验证网络连通性

### 测试失败
- 查看具体错误信息
- 检查服务日志
- 验证上游服务状态

## 配置相关

测试程序使用硬编码的服务地址：`http://localhost:8080`

如果你的服务运行在不同地址或端口，请修改 `test_relay.go` 文件中的：

```go
const BASE_URL = "http://localhost:8080"  // 修改这行
```

## 扩展测试

已实现的功能：
- ✅ 支持从命令行参数指定测试模型名称
- ✅ 支持简形式参数 `-m` 和完整形式参数 `--model`

可扩展的测试用例：

- 测试无效请求格式
- 测试认证头转发  
- 测试自定义 model rules

## 技术细节

- 使用标准库 `net/http` 发送HTTP请求
- 使用 `encoding/json` 处理JSON数据
- 设置适当的超时避免无限等待
- 统计测试通过率和执行时间
- 支持流模式检测和验证