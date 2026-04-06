# Upstream 多路由配置实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 支持按 model 名称路由到不同的 upstream 服务端点，配置格式从单一字符串扩展为 map[string]string

**Architecture:** 修改 Config 结构使用 any 类型支持字符串和 map 格式，添加 normalizeUpstream 函数进行规范化，添加 resolveUpstream 函数根据 model 解析对应 URL，在 proxy 函数中先解析 upstream 再应用 model_rules

**Tech Stack:** Go (标准库), hjson (已有依赖)

---

## 任务已完成

实际实现与原计划有所不同：

1. **Config 结构**：使用 `any` 类型而非直接使用 `map[string]string`
2. **新增 normalizeUpstream 函数**：将 any 类型的 upstream 规范化为 `map[string]string`
3. **新增 resolveUpstream 函数**：根据 model 名称解析对应的 upstream URL
4. **proxy 函数修改**：接受 `map[string]string` 类型的 upstream 参数

### 实际代码变更

**main.go:**
- Config.Upstream 改为 `any` 类型
- 新增 `normalizeUpstream(any) (map[string]string, error)` 函数
- 新增 `resolveUpstream(map[string]string, string) (string, error)` 函数
- loadConfigJSONC 中调用 normalizeUpstream 进行规范化
- proxyPassthrough 和 proxyWithJSONPatch 参数类型改为 `map[string]string`
- 路由在 applyRules 之前执行

**main_test.go:**
- 更新测试用例以适应新的 upstream 类型

**toolcallfix_integration_test.go:**
- 更新测试用例以适应新的 upstream 类型

---

## 测试验证

- [x] 字符串配置：`"upstream": "http://xxx"` → 自动转换为 `{default: "http://xxx"}`
- [x] 对象配置：`"upstream": {glm-5: xxx, default: yyy}` → 正确解析
- [x] 缺失 upstream：返回错误 "upstream must be a string or map[string]string, got <nil>"
- [x] 无 default 且 model 不匹配：返回错误 "no upstream configured for model 'xxx' and no default upstream"
- [x] /v1/models 端点：使用 `upstream["default"]`

---

## 使用示例

### 配置示例 1：字符串（向后兼容）
```jsonc
{
  "listen": ":8080",
  "upstream": "http://192.168.3.244:8000",
  "forward_auth": false
}
```

### 配置示例 2：对象路由
```jsonc
{
  "listen": ":8080",
  "upstream": {
    "glm-5": "http://192.168.3.244:8002",
    "glm-4.7": "http://192.168.3.244:8001",
    "default": "http://192.168.3.244:8000"
  },
  "forward_auth": false,
  "model_rules": [
    {
      "match_model": "glm-4.7",
      "enable_toolcallfix": true
    }
  ]
}
```

### 路由行为
- 客户端请求 `model: "glm-5"` → 路由到 `http://192.168.3.244:8002`
- 客户端请求 `model: "glm-4.7"` → 路由到 `http://192.168.3.244:8001`
- 客户端请求 `model: "other-model"` → 路由到 `http://192.168.3.244:8000`（default 回退）
- /v1/models → 使用 `upstream["default"]`