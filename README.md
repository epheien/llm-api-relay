# LLM API 代理服务

一个基于 Go 的 OpenAI API 兼容代理服务，支持将 OpenAI 格式的请求转发到上游的大语言模型 (LLM) 服务，并支持灵活的请求转换规则。

## 功能特性

- ✅ **OpenAI 兼容性** - 完全兼容 OpenAI API 格式，可直接替换 OpenAI 端点
- ✅ **模型转换规则** - 支持精细的模型名称重映射和参数转换
- ✅ **流式响应** - 完美支持服务器发送事件 (SSE) 格式流式响应
- ✅ **灵活配置** - 使用 JSONC 配置文件格式（支持注释）
- ✅ **零依赖** - 使用标准 Go 库，无外部依赖
- ✅ **轻量级** - 单文件可执行二进制
- ✅ **环境变量支持** - 支持通过环境变量配置主要参数
- ✅ **完整日志** - 结构化日志输出，便于调试和监控

## 项目结构

```
llm-api-relay/
├── main.go              # 主程序入口
├── cmd/                 # 工具和测试程序目录
│   ├── relay-test.go    # 测试工具
│   └── test-runner.go   # 测试运行器
├── toolcallfix/         # 工具修复模块
├── config.jsonc         # 配置文件示例
├── README.md           # 项目文档
├── Makefile            # 构建脚本
├── Dockerfile          # Docker 构建文件
├── go.mod              # Go 模块文件
└── bin/                # 构建后的二进制文件目录（自动生成）
    ├── llm-api-relay    # 主服务二进制
    ├── relay-test       # 测试工具二进制
    └── test-runner      # 测试运行器二进制
```

## 快速开始

### 启动服务

```bash
# 使用默认配置
go run main.go

# 或使用自定义配置文件
go run main.go --config custom-config.jsonc

# 使用 Makefile 构建所有二进制文件
make build

# 运行主服务
./bin/llm-api-relay --config config.jsonc

# 或使用 Makefile 运行
make run
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

- Go 1.18+ (推荐 Go 1.21+)
- 无外部依赖
- 支持 Linux, macOS, Windows

### 构建运行

#### 使用 Makefile（推荐）

```bash
# 查看所有可用命令
make help

# 构建所有二进制文件
make build

# 构建单个主服务二进制
make build-main

# 运行所有测试
make test

# 运行完整测试套件
make test-all

# 安装到系统（需要 sudo 权限）
sudo make install-system

# 从系统卸载
sudo make uninstall-system

# 清理构建产物
make clean
```

#### 传统 Go 构建方式

```bash
# 开发模式
go run main.go

# 生产部署
go build -o bin/llm-api-relay main.go
sudo cp bin/llm-api-relay /usr/local/bin/
llm-api-relay --config /etc/llm-relay/config.jsonc
```

#### 二进制文件说明

使用 Makefile 构建后会生成以下二进制文件：

- `bin/llm-api-relay` - 主服务二进制
- `bin/relay-test` - 测试工具二进制  
- `bin/test-runner` - 测试运行器二进制

### 容器部署

创建 `Dockerfile`：
```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o llm-api-relay main.go

FROM alpine:latest
COPY --from=builder /app/llm-api-relay /usr/local/bin/
EXPOSE 8080
CMD ["llm-api-relay"]
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
- 无内置连接池，建议为高并发场景配置连接池
- 建议配合负载均衡器使用
- 内存使用量低，适合边缘计算场景

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

## 完整工作流程示例

以下是一个完整的开发和部署工作流程：

### 1. 开发环境搭建

```bash
# 克隆项目（如果适用）
git clone <your-repo>
cd llm-api-relay

# 查看可用命令
make help

# 安装依赖
make install

# 运行所有测试
make test
```

### 2. 开发测试

```bash
# 开发模式运行
go run main.go --config config.jsonc

# 测试所有功能
make test-all

# 检查代码规范
make lint
```

### 3. 构建和部署

```bash
# 构建所有二进制文件
make build

# 查看构建结果
ls -la bin/

# 安装到系统（生产环境）
sudo make install-system

# 清理构建产物
make clean
```

### 4. 生产运行

```bash
# 使用系统安装的版本
llm-api-relay --config /etc/llm-relay/config.jsonc

# 或使用构建的二进制
./bin/llm-api-relay --config config.jsonc

# 后台运行
nohup llm-api-relay --config config.jsonc > /var/log/llm-relay.log 2>&1 &
```

### 5. 维护和更新

```bash
# 卸载旧版本
sudo make uninstall-system

# 更新到新版本
make clean && make install

# 运行测试验证
make test
```

## 调试模式

启用详细日志：
```bash
# 使用 Makefile 运行
make run

# 或直接运行二进制
./bin/llm-api-relay --config config.jsonc

# 或使用系统安装的版本
llm-api-relay --config config.jsonc
```

### 测试服务

在开发或测试环境中，可以使用以下方法验证服务：

```bash
# 启动服务并测试基本功能
make run &

# 或使用构建的二进制
./bin/llm-api-relay --config config.jsonc &

# 测试健康检查
curl http://localhost:8080/health

# 测试模型列表
curl http://localhost:8080/v1/models

# 简单聊天测试
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-3.5-turbo",
    "messages": [{"role": "user", "content": "Hello"}],
    "max_tokens": 50
  }'

# 使用测试工具
make run-test

# 运行完整测试套件
make test-all
```

## 许可证

本项目为开源项目，许可证信息待补充。

## 贡献

欢迎提交 Issue 和 Pull Request！

---

**维护者**: LLM API 代理服务团队
**版本**: v1.0.0
**最后更新**: 2024-12-19

## 环境变量配置

除了配置文件外，服务还支持通过环境变量进行配置：

| 环境变量 | 描述 | 示例 |
|----------|------|------|
| `LLM_LISTEN` | 监听地址 | `0.0.0.0:8080` |
| `LLM_UPSTREAM` | 上游服务地址 | `http://localhost:11434/v1` |
| `LLM_FORWARD_AUTH` | 是否转发认证头 | `true`/`false` |
| `LLM_CONFIG` | 配置文件路径 | `./config.jsonc` |

环境变量优先级：命令行参数 > 环境变量 > 配置文件默认值
