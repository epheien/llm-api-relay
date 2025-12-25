# 命令行工具

这个目录包含 LLM API Relay 项目的命令行工具。

## relay-test

LLM API Relay 服务的测试工具，用于验证服务基本功能。

### 使用方法

```bash
# 基本测试（使用默认模型）
./relay-test

# 指定模型测试
./relay-test -model gpt-4

# 详细模式（显示请求和响应详情）
./relay-test -verbose

# 或使用简写形式
./relay-test -v
```

### 功能

- **健康检查**：测试 `/health` 端点
- **Models 列表**：测试 `/v1/models` 端点
- **非流对话**：测试 `/v1/chat/completions` 非流模式
- **流式对话**：测试 `/v1/chat/completions` 流模式

### 注意事项

确保 LLM API Relay 服务在 `http://localhost:8080` 运行。