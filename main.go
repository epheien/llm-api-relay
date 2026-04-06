package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	hjson "github.com/hjson/hjson-go/v4"
	"llm-api-relay/toolcallfix"
)

type Config struct {
	Listen      string      `json:"listen"`
	Upstream    any         `json:"upstream"` // 支持 string 或 map[string]string
	ForwardAuth bool        `json:"forward_auth"`
	ModelRules  []ModelRule `json:"model_rules"`
}

// normalizeUpstream 将 upstream 规范化为 map[string]string
func normalizeUpstream(upstream any) (map[string]string, error) {
	switch v := upstream.(type) {
	case string:
		return map[string]string{"default": v}, nil
	case map[string]any:
		result := make(map[string]string)
		for k, vv := range v {
			if s, ok := vv.(string); ok {
				result[k] = s
			} else {
				return nil, fmt.Errorf("upstream value for key '%s' must be a string, got %T", k, vv)
			}
		}
		return result, nil
	case map[string]string:
		return v, nil
	default:
		return nil, fmt.Errorf("upstream must be a string or map[string]string, got %T", upstream)
	}
}

type ModelRule struct {
	MatchModel        string         `json:"match_model"`        // exact match; use "default" as fallback
	Set               map[string]any `json:"set"`                // overwrite/add fields at top-level
	Extra             map[string]any `json:"extra"`              // merge into request["extra"] (object)
	Unset             []string       `json:"unset"`              // remove fields at top-level
	EnableToolCallFix bool           `json:"enable_toolcallfix"` // enable/disable toolcallfix per model
	ToolCallParser    string         `json:"tool_call_parser"`   // tool call parser name: "xml" (default), "gemma4"
}

var verboseMode bool

// verbose mode helper function
func vlog(format string, args ...any) {
	if verboseMode {
		log.Printf(format, args...)
	}
}

func main() {
	var configPath string
	var verbose bool
	flag.StringVar(&configPath, "config", "", "path to jsonc config")
	flag.StringVar(&configPath, "c", "", "path to jsonc config")
	flag.BoolVar(&verbose, "v", false, "verbose mode - print operation details")
	flag.BoolVar(&verbose, "verbose", false, "verbose mode - print operation details")
	flag.Parse()

	// Require config parameter
	if configPath == "" {
		fmt.Printf("Usage: %s --config <config.jsonc>\n", os.Args[0])
		return
	}

	verboseMode = verbose
	if verboseMode {
		log.Printf("verbose mode enabled")
	}

	cfg, err := loadConfigJSONC(configPath)
	if err != nil {
		log.Fatalf("load config failed: %v", err)
	}

	// upstream 已经是规范化后的 map[string]string
	upstreamMap := cfg.Upstream.(map[string]string)

	mux := http.NewServeMux()

	// OpenAI compatible endpoints
	mux.HandleFunc("/v1/models", func(w http.ResponseWriter, r *http.Request) {
		proxyPassthrough(w, r, upstreamMap, cfg.ForwardAuth, nil)
	})

	patcher := func(req map[string]any) {
		applyRules(cfg, req)
	}

	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		proxyWithJSONPatch(w, r, upstreamMap, cfg.ForwardAuth, cfg, patcher)
	})

	mux.HandleFunc("/v1/completions", func(w http.ResponseWriter, r *http.Request) {
		proxyWithJSONPatch(w, r, upstreamMap, cfg.ForwardAuth, cfg, patcher)
	})

	// health
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	srv := &http.Server{
		Addr:              cfg.Listen,
		Handler:           loggingMiddleware(mux),
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("listening on %s, upstream=%s", cfg.Listen, cfg.Upstream)
	log.Fatal(srv.ListenAndServe())
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s (%s)", r.Method, r.URL.Path, time.Since(start))
	})
}

func loadConfigJSONC(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := hjson.Unmarshal(b, &cfg); err != nil {
		return nil, err
	}

	// 规范化 upstream 为 map[string]string
	normalized, err := normalizeUpstream(cfg.Upstream)
	if err != nil {
		return nil, err
	}
	cfg.Upstream = normalized

	if cfg.Listen == "" {
		cfg.Listen = ":8080"
	}
	return &cfg, nil
}

func applyRules(cfg *Config, req map[string]any) {
	model := getString(req, "model")

	vlog("RULE: processing model '%s'", model)

	rule := findRule(cfg.ModelRules, model)
	if rule == nil {
		vlog("RULE: no exact match for '%s', trying 'default'", model)
		rule = findRule(cfg.ModelRules, "default")
	}

	if rule == nil {
		vlog("RULE: no rule found for model '%s', applying no changes", model)
		return
	}

	vlog("RULE: matched rule '%s', applying transformations", rule.MatchModel)
	vlog("RULE: rule operations - unset: %d fields, set: %d fields, extra: %d fields",
		len(rule.Unset), len(rule.Set), len(rule.Extra))

	// unset first
	for _, k := range rule.Unset {
		vlog("RULE: removing field '%s'", k)
		delete(req, k)
	}

	// set top-level
	for k, v := range rule.Set {
		vlog("RULE: setting '%s' = %v", k, v)
		req[k] = v
	}

	// merge extra
	if len(rule.Extra) > 0 {
		extra, _ := req["extra"].(map[string]any)
		if extra == nil {
			extra = map[string]any{}
			req["extra"] = extra
		}
		for k, v := range rule.Extra {
			vlog("RULE: adding to extra '%s' = %v", k, v)
			extra[k] = v
		}
	}

	vlog("RULE: transformation complete for model '%s'", model)
}

func findRule(rules []ModelRule, model string) *ModelRule {
	for i := range rules {
		if rules[i].MatchModel == model {
			return &rules[i]
		}
	}
	return nil
}

func getString(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

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

// shouldEnableToolCallFix determines whether to enable toolcallfix for a given model
// getToolCallFixConfig returns whether toolcallfix is enabled and which parser to use
func getToolCallFixConfig(cfg *Config, model string) (enable bool, parserName string) {
	// Find exact match rule
	rule := findRule(cfg.ModelRules, model)
	if rule == nil {
		// Try default rule as fallback
		vlog("TOOLCALLFIX: no exact match for '%s', trying 'default'", model)
		rule = findRule(cfg.ModelRules, "default")
	}

	if rule != nil {
		parserName = rule.ToolCallParser
		if parserName == "" {
			parserName = "xml" // default parser
		}
		vlog("TOOLCALLFIX: using rule '%s': enable=%v, parser=%s", rule.MatchModel, rule.EnableToolCallFix, parserName)
		return rule.EnableToolCallFix, parserName
	}

	// Default to disabled (no rule found for this model)
	vlog("TOOLCALLFIX: no rule found for '%s', defaulting to disabled", model)
	return false, "xml"
}

// Deprecated: Use getToolCallFixConfig instead
func shouldEnableToolCallFix(cfg *Config, model string) bool {
	enable, _ := getToolCallFixConfig(cfg, model)
	return enable
}

// proxyPassthrough forwards request to upstream (no body patch).
func proxyPassthrough(w http.ResponseWriter, r *http.Request, upstream map[string]string, forwardAuth bool, newBody io.Reader) {
	// /v1/models 没有 model 参数，使用 default
	upstreamURL, ok := upstream["default"]
	if !ok {
		http.Error(w, "no default upstream configured", http.StatusBadGateway)
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

func proxyWithJSONPatch(w http.ResponseWriter, r *http.Request, upstream map[string]string, forwardAuth bool, cfg *Config, patch func(map[string]any)) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read body failed", http.StatusBadRequest)
		return
	}
	_ = r.Body.Close()

	var payload map[string]any
	if err := json.Unmarshal(bodyBytes, &payload); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}

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

	// patch request json（可能改变 model 名称）
	if patch != nil {
		patch(payload)
	}

	patched, err := json.Marshal(payload)
	if err != nil {
		http.Error(w, "marshal patched body failed", http.StatusBadGateway)
		return
	}

	// Determine whether client expects streaming (OpenAI style stream=true)
	stream := false
	if v, ok := payload["stream"].(bool); ok && v {
		stream = true
	}

	target := up.ResolveReference(r.URL)
	req, err := http.NewRequestWithContext(r.Context(), r.Method, target.String(), bytes.NewReader(patched))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	copyHeaders(req.Header, r.Header)
	req.Host = up.Host
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Length", fmt.Sprintf("%d", len(patched)))

	if !forwardAuth {
		req.Header.Del("Authorization")
	}

	client := &http.Client{Timeout: 0}
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

	// If streaming, ensure flush
	w.WriteHeader(resp.StatusCode)
	if !stream {
		_, _ = io.Copy(w, resp.Body)
		return
	}

	// Check if toolcallfix should be enabled for this model
	// 注意：model 已经是经过 applyRules 转换后的值
	enableToolCallFix, parserName := getToolCallFixConfig(cfg, model)

	// streaming: copy line by line (works for SSE) but still safe for chunked bytes
	flusher, ok := w.(http.Flusher)
	if !ok {
		// fallback
		_, _ = io.Copy(w, resp.Body)
		return
	}

	if enableToolCallFix {
		vlog("TOOLCALLFIX: transforming stream for model '%s' with parser '%s'", model, parserName)
		if err := toolcallfix.TransformStream(resp.Body, w, parserName); err != nil {
			vlog("TOOLCALLFIX: transformation failed: %v", err)
			// Fallback to direct stream copy
			_, _ = io.Copy(w, resp.Body)
			flusher.Flush()
			return
		}
		vlog("TOOLCALLFIX: transformation completed successfully for model '%s'", model)
		return
	}

	// Original streaming logic without toolcallfix
	reader := bufio.NewReader(resp.Body)
	for {
		chunk, err := reader.ReadBytes('\n')
		if len(chunk) > 0 {
			_, _ = w.Write(chunk)
			flusher.Flush()
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			return
		}
	}
}

func copyHeaders(dst, src http.Header) {
	// copy all headers, but avoid hop-by-hop headers
	hop := map[string]struct{}{
		"Connection":          {},
		"Proxy-Connection":    {},
		"Keep-Alive":          {},
		"Proxy-Authenticate":  {},
		"Proxy-Authorization": {},
		"Te":                  {},
		"Trailer":             {},
		"Transfer-Encoding":   {},
		"Upgrade":             {},
	}
	for k, vv := range src {
		if _, ok := hop[k]; ok {
			continue
		}
		// Let Go set these properly
		if strings.EqualFold(k, "Host") {
			continue
		}
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}
