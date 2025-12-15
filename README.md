# LLM API 代理服务

一个基于 Go 的 OpenAI API 兼容代理服务，支持将 OpenAI 格式的请求转发到上游的大语言模型 (LLM) 服务，并支持灵活的请求转换规则。

## 功能特性

- ✅ **OpenAI 兼容性** - 完全兼容 OpenAI API 格式，可直接替换 OpenAI 端点
- ✅ **模型转换规则** - 支持精细的模型名称重映射和参数转换
- ✅ **流式响应** - 完美支持服务器发送事件 (SSE) 格式流式响应
- ✅ **灵活配置** - 使用 JSONC 配置文件格式（支持注释）
- ✅ **零依赖** - 使用标准 Go 库，无外部依赖
- ✅ **轻量级** - 单文件可执行二进制

## 快速开始

### 启动服务

```bash
# 使用默认配置
go run main.go

# 或使用自定义配置文件
go run main.go --config custom-config.jsonc

# 构建二进制文件
go build -o llm-relay main.go
./llm-relay --config config.jsonc
```

### 配置示例

创建 `config.jsonc` 配置文件：

```jsonc
{
  // 监听地址
  "listen": "0.0.0.0:8080",

  // 上游 LLM 服务地址
  "upstream": "http://localhost:11434/v1",

  // 是否转发 Authorization 头
  "forward_auth": false,

  // 模型转换规则
  "model_rules": [
    {
      "match_model": "gpt-3.5-turbo",
      "set": {
        // 重映射模型名称
        "model": "qwen2.5-72b-instruct",
        "temperature": 0.7,
        "top_p": 0.9
      },
      "extra": {
        // 添加额外参数
        "custom_param": "value"
      },
      "unset": ["presence_penalty"] // 移除不支持的参数
    },
    {
      "match_model": "default", // 备用规则
      "set": {
        "temperature": 0.7,
        "top_p": 0.95
      }
    }
  ]
}
```

## API 端点

### OpenAI 兼容端点

| 方法 | 路径 | 描述 |
|------|------|------|
| GET | `/v1/models` | 获取模型列表 |
| POST | `/v1/chat/completions` | 聊天补式（支持流式） |
| POST | `/v1/completions` | 传统补式（兼容性） |

### 服务端点

| 方法 | 路径 | 描述 |
|------|------|------|
| GET | `/health` | 健康检查端点 |

## 使用示例

### 1. 模型列表查询

```bash
curl http://localhost:8080/v1/models
```

### 2. 聊天补式

**非流式响应：**
```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-3.5-turbo",
    "messages": [
      {"role": "user", "content": "Hello"}
    ],
    "stream": false
  }'
```

**流式响应：**
```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-3.5-turbo",
    "messages": [
      {"role": "user", "content": "请继续这个故事"}
    ],
    "stream": true
  }'
```

### 3. 健康检查

```bash
curl http://localhost:8080/health
```

## 模型规则配置

### 规则匹配

- 精确匹配模型名称
- 支持 `"default"` 规则作为备用匹配
- 优先使用第一个匹配的规则

### 转换类型

**1. 设置 (set) - 顶层字段覆盖**
```jsonc
{
  "match_model": "my-model",
  "set": {
    "model": "upstream-model-name",
    "temperature": 0.7
  }
}
```

**2. 额外 (extra) - 嵌套对象合并**
```jsonc
{
  "match_model": "my-model",
  "extra": {
    "generation_params": {
      "temperature": 0.8,
      "top_p": 0.9
    }
  }
}
```

**3. 移除 (unset) - 删除顶层字段**
```jsonc
{
  "match_model": "legacy-model",
  "unset": [
    "presence_penalty",
    "frequency_penalty"
  ]
}
```

### 应用顺序

规则应用优先级：`unset` → `set` → `extra`

## 核心特性

### 流式响应支持

- 自动检测请求中的 `stream: true` 标志
- 完美支持 Server-Sent Events (SSE) 格式
- 逐行转发并实时刷新

### 请求转换

- 基于模型名称的智能转换
- 灵活重映射参数和模型名称
- 清理不支持的参数，避免下游错误

### 错误处理

- 合理的 HTTP 状态码映射
- 详细错误信息返回
- 优雅的资源清理

## 部署和运行

### 环境要求

- Go 1.18+
- 无外部依赖

### 构建运行

```bash
# 开发模式
go run main.go

# 生产部署
go build -o llm-relay main.go
cp llm-relay /usr/local/bin/
llm-relay --config /etc/llm-relay/config.jsonc
```

### 容器部署

创建 `Dockerfile`：
```dockerfile
FROM golang:1.21-aliggs AS builder
WORKDIR /app
COPY . .
RUN go build -o llm-relay main.go

FROM alpine:latest
COPY --from=builder /app/llm-relay /usr/local/bin/
EXPOSE 8080
CMD ["llm-relay"]
```

## 常见用例

### 1. 模型名称重映射

客户端发送模型名 `gpt-4`，代理转换为上游实际模型：
```jsonc
{
  "match_model": "gpt-4",
  "set": {
    "model": "local-llm-model-name"
  }
}
```

### 2. 参数适配

过滤下游不支持的参数：
```jsonc
{
  "match_model": "legacy-model",
  "unset": [
    "presence_penalty",
    "frequency_penalty"
  ]
}
```

### 3. 动态参数注入

为特定模型添加自定义参数：
```jsonc
{
  "match_model": "custom-model",
  "extra": {
    "generation_config": {
      "temperature": 0.8,
      "top_p": 0.9
    }
  }
}
```

## 监控和维护

### 日志输出

服务记录每个请求的：
- HTTP 方法和路径
- 响应时间
- 错误信息（如有）

### 性能考虑

- 流式响应可能长时间占用连接
- 无内置连接池，考虑为高并发场景
- 建议配合负载均衡器使用

## 故障排查

### 常见问题

1. **配置加载失败**
   - 检查 JSONC 语法正确性
   - 验证必需字段存在

2. **流式响应不工作**
   - 确认客户端发送 `stream: true`
   - 验证上游支持流式

3. **模型规则无效**
   - 检查模型名称精确匹配
   - 验证规则配置格式

4. **认证问题**
   - 设置 `forward_auth: true` 转发认证
   - 确认上游服务认证格式

### 调试模式

启用详细日志：
```bash
llm-relay --config config.jsonc
```

## 许可证

本项目为开源项目，许可证信息待补充。

## 贡献

欢迎提交 Issue 和 Pull Request！

---

**作者**: LLM API 代理服务项目
**版本**: v1.0
**最后更新**: 2025-12-15
