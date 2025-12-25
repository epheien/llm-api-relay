# ToolCallFix 功能使用说明

## 概述

ToolCallFix 是一个流式响应转换器，用于将嵌入在 OpenAI 流式响应 content 字段中的 XML 格式工具调用转换为标准的 OpenAI tool_calls 格式。

## 核心功能

### XML 工具调用格式
```xml
<tool_call>function_name<arg_key>key1</arg_key><arg_value>value1</arg_value><arg_key>key2</arg_key><arg_value>value2</arg_value></tool_call>
```

### 转换为 OpenAI 格式
```json
{
  "id": "chatcmpl-tool-xxx",
  "type": "function",
  "function": {
    "name": "function_name",
    "arguments": "{\"key1\": \"value1\", \"key2\": \"value2\"}"
  }
}
```

## 配置选项

在 `model_rules` 中使用 `enable_toolcallfix` 字段控制是否启用 toolcallfix：

### 启用 toolcallfix（默认）
```jsonc
{
  "match_model": "qwen2.5-72b-instruct",
  "enable_toolcallfix": true
}
```

### 禁用 toolcallfix
```jsonc
{
  "match_model": "legacy-model",
  "enable_toolcallfix": false
}
```

### 全局默认值
```jsonc
{
  "match_model": "default",
  "enable_toolcallfix": true
}
```

## 完整配置示例

```jsonc
{
  "listen": "0.0.0.0:8080",
  "upstream": "http://localhost:11434/v1",
  "forward_auth": false,
  "model_rules": [
    {
      "match_model": "gpt-4",
      "enable_toolcallfix": false
    },
    {
      "match_model": "qwen2.5-72b-instruct",
      "set": {
        "model": "custom-model-name"
      },
      "enable_toolcallfix": true
    },
    {
      "match_model": "default",
      "enable_toolcallfix": true
    }
  ]
}
```

## 使用场景

### 1. 新模型启用工具调用转换
对于支持工具调用但输出格式不规范的新模型，启用 toolcallfix：

```jsonc
{
  "match_model": "new-llm-model",
  "enable_toolcallfix": true
}
```

### 2. 旧模型禁用转换
对于不支持工具调用或已有正确格式的模型，禁用 toolcallfix：

```jsonc
{
  "match_model": "legacy-model",
  "enable_toolcallfix": false
}
```

### 3. 精确控制
为不同模型设置不同的转换策略：

```jsonc
{
  "model_rules": [
    {
      "match_model": "modern-model",
      "enable_toolcallfix": true
    },
    {
      "match_model": "compat-model",
      "enable_toolcallfix": false
    },
    {
      "match_model": "default",
      "enable_toolcallfix": true
    }
  ]
}
```

## 日志输出

使用 `--verbose` 模式可以看到 toolcallfix 的详细日志：

```bash
./llm-relay --config config.jsonc --verbose
```

日志示例：
```
2025/12/25 13:44:22 TOOLCALLFIX: processing model 'qwen2.5-72b-instruct'
2025/12/25 13:44:22 TOOLCALLFIX: using rule 'qwen2.5-72b-instruct': enable=true
2025/12/25 13:44:22 TOOLCALLFIX: transforming stream for model 'qwen2.5-72b-instruct'
```

## 错误处理

如果 toolcallfix 转换失败，系统会自动回退到原始流，不会中断服务：

```
2025/12/25 13:44:22 TOOLCALLFIX: transformation failed: parse error
2025/12/25 13:44:22 TOOLCALLFIX: falling back to direct stream copy
```

## 性能影响

- toolcallfix 会对流式响应产生轻微的性能开销
- 转换过程是实时的，不会显著影响响应延迟
- 建议对不支持工具调用的模型禁用此功能以获得最佳性能

## 兼容性

- 向后兼容：现有配置无需修改，默认启用 toolcallfix
- 现有模型规则会自动继承新的默认行为
- 可通过配置精确控制每个模型的转换行为