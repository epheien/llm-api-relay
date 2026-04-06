# Upstream 多路由配置设计

## 背景

当前 `upstream` 配置仅支持单个字符串，无法满足多模型对应不同上游服务的需求。

## 目标

支持按 model 名称路由到不同的 upstream 服务端点。

## 配置格式

### 方式一：对象路由（推荐）
```jsonc
{
  "upstream": {
    "glm-5": "http://192.168.3.244:8002",
    "glm-4.7": "http://192.168.3.244:8001",
    "default": "http://192.168.3.244:8000"
  }
}
```

### 方式二：字符串兼容（向后兼容）
```jsonc
{
  "upstream": "http://192.168.3.244:8000"
}
```
自动转换为 `{ "default": "http://192.168.3.244:8000" }`

## 实现

### 1. Config 结构修改

`main.go` 第 22-27 行：

```go
type Config struct {
    Listen      string            `json:"listen"`
    Upstream    map[string]string `json:"upstream"`  // 改为 map
    ForwardAuth bool              `json:"forward_auth"`
    ModelRules  []ModelRule       `json:"model_rules"`
}
```

### 2. 配置解析修改

`loadConfigJSONC` 函数中增加兼容处理：

```go
func loadConfigJSONC(path string) (*Config, error) {
    b, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }

    // 先用原始 JSON 解析，检测 upstream 类型
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

### 3. 新增路由解析函数

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

### 4. 路由调用位置

在 `proxyWithJSONPatch` 函数中，读取 payload 后立即解析 upstream：

```go
func proxyWithJSONPatch(w http.ResponseWriter, r *http.Request, upstream map[string]string, forwardAuth bool, cfg *Config, patch func(map[string]any)) {
    // ... 读取 body ...

    var payload map[string]any
    // ...

    // 解析 upstream URL（基于原始 model）
    model := getString(payload, "model")
    upstreamURL, err := resolveUpstream(cfg.Upstream, model)
    if err != nil {
        http.Error(w, err.Error(), http.StatusBadGateway)
        return
    }
    up, err := url.Parse(upstreamURL)
    if err != nil {
        http.Error(w, fmt.Sprintf("invalid upstream URL: %v", err), http.StatusBadGateway)
        return
    }

    // 应用 model_rules（可能改变 model 名称）
    if patch != nil {
        patch(payload)
    }

    // ... 转发到 up ...
}
```

### 5. handler 修改

`main.go` 中各端点的 handler 需要调整，不再传递 `*url.URL` 而是传递 `map[string]string`：

```go
mux.HandleFunc("/v1/models", func(w http.ResponseWriter, r *http.Request) {
    proxyPassthrough(w, r, cfg.Upstream, cfg.ForwardAuth, nil)
})

mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
    proxyWithJSONPatch(w, r, cfg.Upstream, cfg.ForwardAuth, cfg, patcher)
})
```

`proxyPassthrough` 也需要类似修改。

## 行为说明

1. **路由时机**：基于客户端传入的**原始** model 名称，在 applyRules 之前解析 upstream
2. **匹配顺序**：精确匹配 → default 回退
3. **向后兼容**：原有字符串配置自动转换为 `{default: xxx}`
4. **/v1/models 端点**：无 model 参数，使用 `upstream["default"]`

## 错误处理

- 启动时：`upstream` 为空时报错
- 运行时：无法匹配到 upstream 时返回 502 错误

## 测试用例

1. 配置字符串 `"upstream": "http://xxx"` → 路由到 default
2. 配置对象 `{glm-5: xxx, default: yyy}` → 匹配 glm-5
3. 配置对象无 default，model 不匹配 → 报错
4. /v1/models 使用 default upstream