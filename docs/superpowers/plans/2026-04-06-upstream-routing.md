# Upstream 多路由配置实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 支持按 model 名称路由到不同的 upstream 服务端点，配置格式从单一字符串扩展为 map[string]string

**Architecture:** 修改 Config 结构支持 map 类型的 upstream，添加 resolveUpstream 函数根据 model 解析对应 URL，在 proxy 函数中先解析 upstream 再应用 model_rules

**Tech Stack:** Go (标准库), hjson (已有依赖)

---

## 任务 1: 修改 Config 结构

**Files:**
- Modify: `main.go:22-27`

- [ ] **Step 1: 修改 Config 结构体**

将 upstream 字段从 string 改为 map[string]string：

```go
type Config struct {
    Listen      string            `json:"listen"`
    Upstream    map[string]string `json:"upstream"`  // 从 string 改为 map
    ForwardAuth bool              `json:"forward_auth"`
    ModelRules  []ModelRule       `json:"model_rules"`
}
```

- [ ] **Step 2: 运行 vet 检查**

Run: `go vet ./...`
Expected: 无错误

- [ ] **Step 3: 提交**

```bash
git add main.go
git commit -m "refactor: change upstream to map[string]string in Config struct"
```

---

## 任务 2: 修改配置解析逻辑

**Files:**
- Modify: `main.go:118-134` (loadConfigJSONC 函数)

- [ ] **Step 1: 修改 loadConfigJSONC 函数**

替换整个函数，支持字符串和对象两种 upstream 格式：

```go
func loadConfigJSONC(path string) (*Config, error) {
    b, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }

    // 先用原始 JSON 解析，检测 upstream 类型以支持兼容处理
    var rawCfg struct {
        Upstream any `json:"upstream"`
    }
    if err := hjson.Unmarshal(b, &rawCfg); err != nil {
        return nil, err
    }

    var cfg Config
    if err := hjson.Unmarshal(b, &cfg); err != nil {
        return nil, err
    }

    // 兼容处理：如果 upstream 是字符串，转换为 map
    if rawCfg.Upstream != nil {
        switch v := rawCfg.Upstream.(type) {
        case string:
            cfg.Upstream = map[string]string{"default": v}
        case map[string]any:
            cfg.Upstream = make(map[string]string)
            for k, vv := range v {
                if s, ok := vv.(string); ok {
                    cfg.Upstream[k] = s
                }
            }
        case map[string]string:
            cfg.Upstream = v
        }
    }

    // 校验
    if cfg.Listen == "" {
        cfg.Listen = ":8080"
    }
    if len(cfg.Upstream) == 0 {
        return nil, errors.New("upstream is required")
    }

    return &cfg, nil
}
```

- [ ] **Step 2: 运行 vet 检查**

Run: `go vet ./...`
Expected: 无错误

- [ ] **Step 3: 提交**

```bash
git add main.go
git commit -m "feat: support both string and map upstream config with backward compatibility"
```

---

## 任务 3: 新增 resolveUpstream 函数

**Files:**
- Modify: `main.go` (在 getString 函数后新增)

- [ ] **Step 1: 添加 resolveUpstream 函数**

在 `getString` 函数后（约第 202 行）添加：

```go
// resolveUpstream 根据 model 名称解析对应的 upstream URL
func resolveUpstream(upstream map[string]string, model string) (string, error) {
    // 精确匹配
    if url, ok := upstream[model]; ok {
        return url, nil
    }
    // 回退到 default
    if url, ok := upstream["default"]; ok {
        return url, nil
    }
    return "", fmt.Errorf("no upstream configured for model '%s' and no default upstream", model)
}
```

- [ ] **Step 2: 运行 vet 检查**

Run: `go vet ./...`
Expected: 无错误

- [ ] **Step 3: 提交**

```bash
git add main.go
git commit -m "feat: add resolveUpstream function for model-based routing"
```

---

## 任务 4: 修改 proxyPassthrough 函数

**Files:**
- Modify: `main.go:224-268` (proxyPassthrough 函数)

- [ ] **Step 1: 修改 proxyPassthrough 函数签名和实现**

将参数从 `upstream *url.URL` 改为 `upstream map[string]string`，并在函数内部解析 URL：

```go
// proxyPassthrough forwards request to upstream (no body patch).
func proxyPassthrough(w http.ResponseWriter, r *http.Request, upstream map[string]string, forwardAuth bool, newBody io.Reader) {
    // /v1/models 没有 model 参数，使用 default
    upstreamURL, err := resolveUpstream(upstream, "default")
    if err != nil {
        http.Error(w, err.Error(), http.StatusBadGateway)
        return
    }

    up, err := url.Parse(upstreamURL)
    if err != nil {
        http.Error(w, fmt.Sprintf("invalid upstream URL: %v", err), http.StatusBadGateway)
        return
    }

    target := up.ResolveReference(r.URL)
    req, err := http.NewRequestWithContext(r.Context(), r.Method, target.String(), newBody)
    if err != nil {
        http.Error(w, err.Error(), http.StatusBadGateway)
        return
    }

    copyHeaders(req.Header, r.Header)
    // Host should be upstream host
    req.Host = up.Host

    if !forwardAuth {
        req.Header.Del("Authorization")
    }

    // If we provided a new body, set content-type if missing
    if newBody != nil && req.Header.Get("Content-Type") == "" {
        req.Header.Set("Content-Type", "application/json")
    }

    // Use a transport that supports streaming well
    client := &http.Client{
        Timeout: 0, // streaming: no timeout
    }

    resp, err := client.Do(req)
    if err != nil {
        http.Error(w, err.Error(), http.StatusBadGateway)
        return
    }
    defer resp.Body.Close()

    // copy response headers
    for k, vv := range resp.Header {
        for _, v := range vv {
            w.Header().Add(k, v)
        }
    }
    w.WriteHeader(resp.StatusCode)

    // stream copy
    _, _ = io.Copy(w, resp.Body)
}
```

- [ ] **Step 2: 运行 vet 检查**

Run: `go vet ./...`
Expected: 无错误

- [ ] **Step 3: 提交**

```bash
git add main.go
git commit -m "refactor: modify proxyPassthrough to accept map upstream and resolve URL internally"
```

---

## 任务 5: 修改 proxyWithJSONPatch 函数

**Files:**
- Modify: `main.go:270-386` (proxyWithJSONPatch 函数)

- [ ] **Step 1: 修改 proxyWithJSONPatch 函数签名**

将 `upstream *url.URL` 改为 `upstream map[string]string`：

```go
func proxyWithJSONPatch(w http.ResponseWriter, r *http.Request, upstream map[string]string, forwardAuth bool, cfg *Config, patch func(map[string]any)) {
```

- [ ] **Step 2: 在读取 payload 后、patch 前添加 upstream 解析逻辑**

在第 289-292 行附近（patch(payload) 之前），添加：

```go
    // 解析 upstream URL（基于原始 model，在 applyRules 之前）
    model := getString(payload, "model")
    upstreamURL, err := resolveUpstream(upstream, model)
    if err != nil {
        http.Error(w, err.Error(), http.StatusBadGateway)
        return
    }
    up, err := url.Parse(upstreamURL)
    if err != nil {
        http.Error(w, fmt.Sprintf("invalid upstream URL: %v", err), http.StatusBadGateway)
        return
    }
```

- [ ] **Step 3: 移除函数内原有的 model 变量提取**

原来第 344-345 行提取 model 的代码：
```go
    // Extract model name for toolcallfix decision
    model := getString(payload, "model")
```
已经移动到前面，可以删除重复的。

- [ ] **Step 4: 将函数内所有 `upstream` 引用改为 `up`**

将 `upstream.ResolveReference` 改为 `up.ResolveReference`，`upstream.Host` 改为 `up.Host`。

- [ ] **Step 5: 运行 vet 检查**

Run: `go vet ./...`
Expected: 无错误

- [ ] **Step 6: 提交**

```bash
git add main.go
git commit -m "refactor: modify proxyWithJSONPatch to resolve upstream by model before applying rules"
```

---

## 任务 6: 修改 main 函数中的 handler 注册

**Files:**
- Modify: `main.go:76-93` (mux handler 注册)

- [ ] **Step 1: 移除启动时的 upstream 解析**

删除第 71-74 行的解析代码：
```go
up, err := url.Parse(cfg.Upstream)
if err != nil {
    log.Fatalf("invalid upstream: %v", err)
}
```

- [ ] **Step 2: 修改 handler 传参**

将 handler 调用中的 `up` 改为 `cfg.Upstream`：

```go
mux.HandleFunc("/v1/models", func(w http.ResponseWriter, r *http.Request) {
    proxyPassthrough(w, r, cfg.Upstream, cfg.ForwardAuth, nil)
})

patcher := func(req map[string]any) {
    applyRules(cfg, req)
}

mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
    proxyWithJSONPatch(w, r, cfg.Upstream, cfg.ForwardAuth, cfg, patcher)
})

mux.HandleFunc("/v1/completions", func(w http.ResponseWriter, r *http.Request) {
    proxyWithJSONPatch(w, r, cfg.Upstream, cfg.ForwardAuth, cfg, patcher)
})
```

- [ ] **Step 3: 修改启动日志**

将第 106 行的日志改为打印整个 upstream map：

```go
log.Printf("listening on %s, upstream=%v", cfg.Listen, cfg.Upstream)
```

- [ ] **Step 4: 运行 vet 检查**

Run: `go vet ./...`
Expected: 无错误

- [ ] **Step 5: 提交**

```bash
git add main.go
git commit -m "refactor: update main to pass map upstream to handlers instead of parsed URL"
```

---

## 任务 7: 测试验证

**Files:**
- Test: 使用现有测试或手动测试

- [ ] **Step 1: 构建项目**

Run: `go build -o llm-relay main.go`
Expected: 编译成功

- [ ] **Step 2: 测试字符串配置兼容性**

创建测试配置：
```jsonc
{
  "listen": ":8080",
  "upstream": "http://localhost:8000",
  "forward_auth": false
}
```

Run: `./llm-relay --config test-string.jsonc`
Expected: 启动成功，日志显示 `upstream=map[default:http://localhost:8000]`

- [ ] **Step 3: 测试对象配置**

创建测试配置：
```jsonc
{
  "listen": ":8080",
  "upstream": {
    "glm-5": "http://192.168.3.244:8002",
    "glm-4.7": "http://192.168.3.244:8001",
    "default": "http://192.168.3.244:8000"
  },
  "forward_auth": false
}
```

Run: `./llm-relay --config test-map.jsonc`
Expected: 启动成功，日志显示完整的 upstream map

- [ ] **Step 4: 测试缺失 upstream 错误**

创建空配置：
```jsonc
{
  "listen": ":8080"
}
```

Run: `./llm-relay --config test-empty.jsonc`
Expected: 启动失败，错误信息 "upstream is required"

- [ ] **Step 5: 清理测试文件**

```bash
rm -f test-string.jsonc test-map.jsonc test-empty.jsonc llm-relay
```

- [ ] **Step 6: 提交**

```bash
git add .
git commit -m "test: verify upstream routing configuration works correctly"
```

---

## 任务 8: 运行现有测试

**Files:**
- Run: `main_test.go`, `toolcallfix/transform_test.go`

- [ ] **Step 1: 运行单元测试**

Run: `go test -v ./...`
Expected: 所有测试通过

- [ ] **Step 2: 提交**

```bash
git commit -m "test: ensure all existing tests pass"
```