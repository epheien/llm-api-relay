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

	"llm-api-relay/toolcallfix"
)

type Config struct {
	Listen      string      `json:"listen"`
	Upstream    string      `json:"upstream"`
	ForwardAuth bool        `json:"forward_auth"`
	ModelRules  []ModelRule `json:"model_rules"`
}

type ModelRule struct {
	MatchModel        string         `json:"match_model"`        // exact match; use "default" as fallback
	Set               map[string]any `json:"set"`                // overwrite/add fields at top-level
	Extra             map[string]any `json:"extra"`              // merge into request["extra"] (object)
	Unset             []string       `json:"unset"`              // remove fields at top-level
	EnableToolCallFix bool           `json:"enable_toolcallfix"` // enable/disable toolcallfix per model
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

	up, err := url.Parse(cfg.Upstream)
	if err != nil {
		log.Fatalf("invalid upstream: %v", err)
	}

	mux := http.NewServeMux()

	// OpenAI compatible endpoints
	mux.HandleFunc("/v1/models", func(w http.ResponseWriter, r *http.Request) {
		proxyPassthrough(w, r, up, cfg.ForwardAuth, nil)
	})

	patcher := func(req map[string]any) {
		applyRules(cfg, req)
	}

	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		proxyWithJSONPatch(w, r, up, cfg.ForwardAuth, cfg, patcher)
	})

	mux.HandleFunc("/v1/completions", func(w http.ResponseWriter, r *http.Request) {
		proxyWithJSONPatch(w, r, up, cfg.ForwardAuth, cfg, patcher)
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
	clean := stripJSONC(string(b))
	var cfg Config
	if err := json.Unmarshal([]byte(clean), &cfg); err != nil {
		return nil, err
	}
	if cfg.Listen == "" {
		cfg.Listen = ":8080"
	}
	if cfg.Upstream == "" {
		return nil, errors.New("upstream is required")
	}
	return &cfg, nil
}

// stripJSONC removes // line comments and /* block comments */.
// It’s simple and pragmatic for config use.
func stripJSONC(s string) string {
	var out strings.Builder
	out.Grow(len(s))

	inString := false
	escape := false
	inLineComment := false
	inBlockComment := false

	for i := 0; i < len(s); i++ {
		c := s[i]

		// end line comment
		if inLineComment {
			if c == '\n' {
				inLineComment = false
				out.WriteByte(c)
			}
			continue
		}

		// end block comment
		if inBlockComment {
			if c == '*' && i+1 < len(s) && s[i+1] == '/' {
				inBlockComment = false
				i++
			}
			continue
		}

		// handle string state
		if inString {
			out.WriteByte(c)
			if escape {
				escape = false
				continue
			}
			if c == '\\' {
				escape = true
				continue
			}
			if c == '"' {
				inString = false
			}
			continue
		}

		// not in string/comment
		if c == '"' {
			inString = true
			out.WriteByte(c)
			continue
		}

		// start comments
		if c == '/' && i+1 < len(s) {
			n := s[i+1]
			if n == '/' {
				inLineComment = true
				i++
				continue
			}
			if n == '*' {
				inBlockComment = true
				i++
				continue
			}
		}

		out.WriteByte(c)
	}
	return out.String()
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

// shouldEnableToolCallFix determines whether to enable toolcallfix for a given model
func shouldEnableToolCallFix(cfg *Config, model string) bool {
	// Find exact match rule
	rule := findRule(cfg.ModelRules, model)
	if rule == nil {
		// Try default rule as fallback
		vlog("TOOLCALLFIX: no exact match for '%s', trying 'default'", model)
		rule = findRule(cfg.ModelRules, "default")
	}

	if rule != nil {
		vlog("TOOLCALLFIX: using rule '%s': enable=%v", rule.MatchModel, rule.EnableToolCallFix)
		return rule.EnableToolCallFix
	}

	// Default to disabled (no rule found for this model)
	vlog("TOOLCALLFIX: no rule found for '%s', defaulting to disabled", model)
	return false
}

// proxyPassthrough forwards request to upstream (no body patch).
func proxyPassthrough(w http.ResponseWriter, r *http.Request, upstream *url.URL, forwardAuth bool, newBody io.Reader) {
	target := upstream.ResolveReference(r.URL)
	req, err := http.NewRequestWithContext(r.Context(), r.Method, target.String(), newBody)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	copyHeaders(req.Header, r.Header)
	// Host should be upstream host
	req.Host = upstream.Host

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

func proxyWithJSONPatch(w http.ResponseWriter, r *http.Request, upstream *url.URL, forwardAuth bool, cfg *Config, patch func(map[string]any)) {
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

	// patch request json
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

	target := upstream.ResolveReference(r.URL)
	req, err := http.NewRequestWithContext(r.Context(), r.Method, target.String(), bytes.NewReader(patched))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	copyHeaders(req.Header, r.Header)
	req.Host = upstream.Host
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

	// Extract model name for toolcallfix decision
	model := getString(payload, "model")

	// Check if toolcallfix should be enabled for this model
	enableToolCallFix := shouldEnableToolCallFix(cfg, model)

	// streaming: copy line by line (works for SSE) but still safe for chunked bytes
	flusher, ok := w.(http.Flusher)
	if !ok {
		// fallback
		_, _ = io.Copy(w, resp.Body)
		return
	}

	if enableToolCallFix {
		vlog("TOOLCALLFIX: transforming stream for model '%s'", model)
		if err := toolcallfix.TransformStream(resp.Body, w); err != nil {
			vlog("TOOLCALLFIX: transformation failed: %v", err)
			// Fallback to direct stream copy
			_, _ = io.Copy(w, resp.Body)
			return
		}
		flusher.Flush()
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
