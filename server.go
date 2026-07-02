package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

type ProxyServer struct {
	cfg      *Config
	server   *http.Server
	router   *SmartRouter
	mu       sync.RWMutex
	err      error
	reqCount int64
}

func NewProxyServer(cfg *Config) *ProxyServer {
	ps := &ProxyServer{
		cfg:    cfg,
		router: NewSmartRouter(cfg),
	}
	mux := http.NewServeMux()

	mux.HandleFunc("/v1/chat/completions", ps.handleChatCompletions)
	mux.HandleFunc("/v1/messages", ps.handleMessages)
	mux.HandleFunc("/v1/models", ps.handleModels)
	mux.HandleFunc("/health", ps.handleHealth)
	mux.HandleFunc("/debug/routes", ps.handleDebugRoutes)
	mux.HandleFunc("/", ps.handleNotFound)

	ps.server = &http.Server{
		Addr:    fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler: corsMiddleware(mux),
	}
	return ps
}

func (ps *ProxyServer) Start() error {
	return ps.server.ListenAndServe()
}

func (ps *ProxyServer) Shutdown() error {
	return ps.server.Close()
}

func (ps *ProxyServer) incReq() {
	atomic.AddInt64(&ps.reqCount, 1)
}

// ─── Handlers ─────────────────────────────────────────────────────────

func (ps *ProxyServer) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ps.incReq()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("read body: %v", err))
		return
	}
	defer r.Body.Close()

	// Detect if streaming
	var reqBody struct {
		Stream bool `json:"stream"`
	}
	if err := json.Unmarshal(body, &reqBody); err == nil && reqBody.Stream {
		ps.handleStreamingRequest(w, r, "openai", body)
		return
	}

	respBody, status, contentType, headers := ps.router.HandleRequest("openai", body)
	for k, v := range headers {
		w.Header().Set(k, v)
	}
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(status)
	w.Write(respBody)
}

func (ps *ProxyServer) handleMessages(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ps.incReq()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("read body: %v", err))
		return
	}
	defer r.Body.Close()

	var reqBody struct {
		Stream bool `json:"stream"`
	}
	if err := json.Unmarshal(body, &reqBody); err == nil && reqBody.Stream {
		ps.handleStreamingRequest(w, r, "anthropic", body)
		return
	}

	respBody, status, contentType, headers := ps.router.HandleRequest("anthropic", body)
	for k, v := range headers {
		w.Header().Set(k, v)
	}
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(status)
	w.Write(respBody)
}

func (ps *ProxyServer) handleStreamingRequest(w http.ResponseWriter, r *http.Request, inputFormat string, body []byte) {
	canonicalReq, err := ParseToCanonical(inputFormat, body)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("parse: %v", err))
		return
	}

	// Route to find available provider
	result, err := ps.router.Route(canonicalReq)
	if err != nil {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("no route: %v", err))
		return
	}

	if result.Response == nil {
		writeError(w, http.StatusBadGateway, "no response from any provider")
		return
	}

	// Reconstruct request for streaming
	nativeBody, err := ConvertCanonicalToNative(result.APIConfig.Format, canonicalReq)
	if err != nil {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("convert: %v", err))
		return
	}

	targetURL := buildTargetURL(result.APIConfig, canonicalReq)
	proxyReq, err := http.NewRequest("POST", targetURL, nativeBodyReader(nativeBody))
	if err != nil {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("create request: %v", err))
		return
	}

	proxyReq.Header.Set("Content-Type", "application/json")
	proxyReq.Header.Set("Accept", "text/event-stream")
	if result.APIConfig.APIKey != "" {
		switch result.APIConfig.Format {
		case "anthropic":
			proxyReq.Header.Set("x-api-key", result.APIConfig.APIKey)
		case "gemini":
			q := proxyReq.URL.Query()
			q.Set("key", result.APIConfig.APIKey)
			proxyReq.URL.RawQuery = q.Encode()
		default:
			proxyReq.Header.Set("Authorization", "Bearer "+result.APIConfig.APIKey)
		}
	}
	for k, v := range result.APIConfig.ExtraHeaders {
		proxyReq.Header.Set(k, v)
	}

	client := &http.Client{Timeout: 0}
	upstreamResp, err := client.Do(proxyReq)
	if err != nil {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("upstream: %v", err))
		return
	}
	defer upstreamResp.Body.Close()

	if upstreamResp.StatusCode >= 400 {
		errBody, _ := io.ReadAll(upstreamResp.Body)
		writeError(w, upstreamResp.StatusCode, fmt.Sprintf("%s: %s", result.Provider, string(errBody)))
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-AnyRouter-Provider", result.Provider)
	w.Header().Set("X-AnyRouter-Model", result.Model)

	// Initial chunk for OpenAI format
	if inputFormat == "openai" {
		initial := fmt.Sprintf(`data: {"id":"chatcmpl-%d","object":"chat.completion.chunk","created":%d,"model":"%s","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}}`+"\n\n",
			time.Now().UnixMilli(), time.Now().Unix(), result.Model)
		fmt.Fprint(w, initial)
		flusher.Flush()
	}

	buf := make([]byte, 32*1024)
	for {
		n, err := upstreamResp.Body.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			// Convert provider stream format → input format
			converted, convErr := convertProviderChunkToOutputFormat(result.APIConfig.Format, inputFormat, chunk)
			if convErr == nil && len(converted) > 0 {
				w.Write(converted)
				flusher.Flush()
			} else if convErr == nil {
				// Raw passthrough
				w.Write(chunk)
				flusher.Flush()
			}
		}
		if err != nil {
			break
		}
	}

	// Final marker
	if inputFormat == "openai" {
		fmt.Fprint(w, "data: [DONE]\n\n")
		flusher.Flush()
	}
}

// ─── Static Endpoints ─────────────────────────────────────────────────

func (ps *ProxyServer) handleModels(w http.ResponseWriter, r *http.Request) {
	type ModelInfo struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		OwnedBy string `json:"owned_by"`
	}
	var models []ModelInfo
	for name, prov := range ps.cfg.Providers {
		if !prov.Enabled {
			continue
		}
		if len(prov.Models) > 0 {
			for _, m := range prov.Models {
				models = append(models, ModelInfo{ID: m, Object: "model", OwnedBy: name})
			}
		} else {
			models = append(models, ModelInfo{ID: name + "/*", Object: "model", OwnedBy: name})
		}
	}
	resp := map[string]any{"object": "list", "data": models}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (ps *ProxyServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	status := "ok"
	providers := map[string]string{}
	for _, name := range ps.router.health.AllProviderNames() {
		ok, degraded, details := ps.router.health.GetProviderStatus(name)
		switch {
		case ok && !degraded:
			providers[name] = "healthy"
		case ok && degraded:
			providers[name] = "degraded (" + details + ")"
		default:
			providers[name] = "unavailable (" + details + ")"
			status = "degraded"
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status":    status,
		"version":   Version,
		"requests":  atomic.LoadInt64(&ps.reqCount),
		"providers": providers,
	})
}

func (ps *ProxyServer) handleDebugRoutes(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	model := r.URL.Query().Get("model")
	if model == "" {
		model = "gpt-4o"
	}

	chain, err := ps.router.buildRouteChain(model)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
		return
	}

	var routes []map[string]string
	for _, link := range chain {
		routes = append(routes, map[string]string{
			"provider": link.ProviderName,
			"model":    link.Model,
			"keys":     fmt.Sprintf("%d", len(link.ProviderCfg.GetAllAPIKeys())),
		})
	}

	json.NewEncoder(w).Encode(map[string]any{
		"model":  model,
		"routes": routes,
	})
}

func (ps *ProxyServer) handleNotFound(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotFound,
		"AnyRouter: endpoint not found.\nAvailable:\n"+
			"  POST /v1/chat/completions (OpenAI format)\n"+
			"  POST /v1/messages (Anthropic format)\n"+
			"  GET /v1/models\n"+
			"  GET /health\n"+
			"  GET /debug/routes?model=xxx")
}

// ─── Middleware & Helpers ─────────────────────────────────────────────

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers",
			"Content-Type, Authorization, x-api-key, anthropic-version")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{"message": msg, "type": "anyrouter_error"},
	})
}

func nativeBodyReader(body []byte) io.Reader {
	r, w := io.Pipe()
	go func() {
		w.Write(body)
		w.Close()
	}()
	return r
}

func convertProviderChunkToOutputFormat(providerFormat, outputFormat string, data []byte) ([]byte, error) {
	sd, err := parseStreamChunkToDelta(providerFormat, data)
	if err != nil || sd == nil {
		return nil, err
	}

	if outputFormat == "anthropic" {
		return marshalAnthropicStreamDelta(sd)
	}

	return marshalOpenAIStreamDelta(sd)
}

func marshalAnthropicStreamDelta(delta *StreamDelta) ([]byte, error) {
	if delta == nil {
		return nil, nil
	}
	var result []byte

	if delta.Role != "" {
		msg := map[string]any{
			"type": "message_start",
			"message": map[string]any{
				"id":      fmt.Sprintf("msg_%d", time.Now().UnixMilli()),
				"type":    "message",
				"role":    "assistant",
				"content": []any{},
				"model":   "",
			},
		}
		data, _ := json.Marshal(msg)
		result = append(result, []byte("event: message_start\n")...)
		result = append(result, []byte("data: "+string(data)+"\n\n")...)
	}

	if delta.Content != "" {
		cd := map[string]any{
			"type":  "content_block_delta",
			"index": 0,
			"delta": map[string]string{
				"type": "text_delta",
				"text": delta.Content,
			},
		}
		data, _ := json.Marshal(cd)
		result = append(result, []byte("event: content_block_delta\n")...)
		result = append(result, []byte("data: "+string(data)+"\n\n")...)
	}

	if delta.FinishReason != "" {
		fr := delta.FinishReason
		switch fr {
		case "stop":
			fr = "end_turn"
		case "tool_calls":
			fr = "tool_use"
		case "length":
			fr = "max_tokens"
		}
		d := map[string]any{
			"type": "message_delta",
			"delta": map[string]string{
				"stop_reason": fr,
			},
		}
		data, _ := json.Marshal(d)
		result = append(result, []byte("event: message_delta\n")...)
		result = append(result, []byte("data: "+string(data)+"\n\n")...)
		result = append(result, []byte("event: message_stop\ndata: {}\n\n")...)
	}

	return result, nil
}
