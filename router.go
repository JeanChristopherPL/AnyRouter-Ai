package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

type SmartRouter struct {
	cfg     *Config
	health  *HealthTracker
	reqID   int64
}

func NewSmartRouter(cfg *Config) *SmartRouter {
	return &SmartRouter{
		cfg:    cfg,
		health: NewHealthTracker(cfg),
	}
}

type RouteAttempt struct {
	Provider   string  `json:"provider"`
	Key        string  `json:"key"`
	Model      string  `json:"model"`
	HTTPStatus int     `json:"http_status"`
	DurationMs int64   `json:"duration_ms"`
	Error      string  `json:"error,omitempty"`
	RateLimited bool   `json:"rate_limited,omitempty"`
}

type RouteResult struct {
	Provider     string         `json:"provider"`
	Key          string         `json:"key"`
	Model        string         `json:"model"`
	APIConfig    *ProviderConfig `json:"-"`
	Response     *http.Response  `json:"-"`
	Attempts     []RouteAttempt  `json:"attempts"`
	Body         []byte          `json:"-"`
	TotalAttempts int            `json:"total_attempts"`
}

type RouteChainLink struct {
	ProviderName string
	ProviderCfg  *ProviderConfig
	Model        string
}

type smartRequest struct {
	canonical *CanonicalChatRequest
	body      []byte
	inputFmt  string
}

func (sr *SmartRouter) Route(req *CanonicalChatRequest) (*RouteResult, error) {
	result := &RouteResult{
		Model: req.Model,
	}
	rid := atomic.AddInt64(&sr.reqID, 1)
	_ = rid

	// 1. Find primary provider
	routes, err := sr.buildRouteChain(req.Model)
	if err != nil {
		return result, err
	}

	// 2. Try each route
	for _, link := range routes {
		prov := link.ProviderCfg
		if prov == nil || !prov.Enabled {
			continue
		}

		// 3. For each provider, try their API keys
		keys := prov.GetAllAPIKeys()
		if len(keys) == 0 {
			keys = []string{""}
		}

		for _, apiKey := range keys {
			// Check if this key is available for this provider
			if apiKey != "" && !sr.health.IsKeyAvailable(link.ProviderName, apiKey) {
				continue
			}

			// Check if provider overall is available
			if !sr.health.IsProviderAvailable(link.ProviderName) {
				break // no keys available for this provider
			}

			start := time.Now()

			// Build request with this model
			reqCopy := *req
			reqCopy.Model = link.Model

			// Create a provider config override with just this key
			provCopy := *prov
			provCopy.APIKey = apiKey

			respBytes, httpResp, attemptErr := sr.executeRequest(link.ProviderName, &provCopy, &reqCopy)
			duration := time.Since(start)

			attempt := RouteAttempt{
				Provider:   link.ProviderName,
				Key:        maskKey(apiKey),
				Model:      link.Model,
				DurationMs: duration.Milliseconds(),
			}

			if httpResp != nil {
				attempt.HTTPStatus = httpResp.StatusCode
			}
			if attemptErr != nil {
				attempt.Error = attemptErr.Error()
			}

			// Classify and record
			isRateLimited := false
			statusCode := 0
			if httpResp != nil {
				statusCode = httpResp.StatusCode
			}
			errClass := classifyError(statusCode, respBytes, attemptErr)
			isRateLimited = (errClass == ErrRateLimited)
			attempt.RateLimited = isRateLimited

			sr.health.RecordAttempt(link.ProviderName, apiKey, statusCode, respBytes, attemptErr, duration, 0)

			result.Attempts = append(result.Attempts, attempt)
			result.TotalAttempts++

			// Success!
			if errClass == ErrSuccess && httpResp != nil {
				result.Provider = link.ProviderName
				result.Key = apiKey
				result.Model = link.Model
				cp := provCopy
				result.APIConfig = &cp
				result.Response = httpResp
				result.Body = respBytes
				return result, nil
			}

			// Auth errors on this key — try next key for same provider
			if errClass == ErrAuth {
				continue
			}

			// Rate limited — try next key
			if isRateLimited {
				// If key has valid Retry-After, note it
				continue
			}

			// Server error — try next
			if errClass == ErrServerError || errClass == ErrTimeout || errClass == ErrNetwork {
				continue
			}

			// Quota error — try next key
			if errClass == ErrQuota {
				continue
			}

			// Bad request — likely model-specific, try model fallback
			if errClass == ErrBadRequest {
				break // don't retry same model on this provider
			}
		}
	}

	// 3. All routes failed — build error summary
	errMsg := sr.buildFailureSummary(result)
	return result, fmt.Errorf("routing failed: %s", errMsg)
}

func (sr *SmartRouter) buildRouteChain(model string) ([]RouteChainLink, error) {
	var chain []RouteChainLink
	seen := map[string]bool{}

	// Find primary provider
	primaryName, primaryProvPtr, err := ResolveProvider(sr.cfg, model)
	if err != nil {
		return nil, err
	}

	primaryProv := *primaryProvPtr

	// Build model list: primary model + fallbacks
	allModels := []string{model}
	if len(primaryProv.ModelFallbacks) > 0 {
		for _, m := range primaryProv.ModelFallbacks {
			if m != model {
				allModels = append(allModels, m)
			}
		}
	}
	// Also add provider's model list
	for _, m := range primaryProv.Models {
		found := false
		for _, am := range allModels {
			if am == m {
				found = true
				break
			}
		}
		if !found {
			allModels = append(allModels, m)
		}
	}

	// Add primary provider with all models
	providerKey := primaryName
	for _, m := range allModels {
		if !seen[providerKey+":"+m] {
			chain = append(chain, RouteChainLink{
				ProviderName: primaryName,
				ProviderCfg:  &primaryProv,
				Model:        m,
			})
			seen[providerKey+":"+m] = true
		}
	}
	seen[providerKey] = true

	// Add fallback providers
	fallbacks := append([]string{}, primaryProv.Fallbacks...)
	for _, fb := range fallbacks {
		if seen[fb] {
			continue
		}
		seen[fb] = true
		fbProv, ok := sr.cfg.Providers[fb]
		if !ok || !fbProv.Enabled {
			continue
		}

		// For fallback providers, use their own model list
		fbModels := fbProv.Models
		if len(fbModels) == 0 {
			fbModels = []string{model}
		}
		for _, m := range fbModels {
			chain = append(chain, RouteChainLink{
				ProviderName: fb,
				ProviderCfg:  &fbProv,
				Model:        m,
			})
		}
	}

	return chain, nil
}

func (sr *SmartRouter) executeRequest(providerName string, prov *ProviderConfig, req *CanonicalChatRequest) ([]byte, *http.Response, error) {
	nativeBody, err := ConvertCanonicalToNative(prov.Format, req)
	if err != nil {
		return nil, nil, fmt.Errorf("convert to %s: %w", prov.Format, err)
	}

	targetURL := buildTargetURL(prov, req)
	httpReq, err := http.NewRequest("POST", targetURL, strings.NewReader(string(nativeBody)))
	if err != nil {
		return nil, nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if prov.APIKey != "" {
		switch prov.Format {
		case "anthropic":
			httpReq.Header.Set("x-api-key", prov.APIKey)
		case "gemini":
			q := httpReq.URL.Query()
			q.Set("key", prov.APIKey)
			httpReq.URL.RawQuery = q.Encode()
		default:
			httpReq.Header.Set("Authorization", "Bearer "+prov.APIKey)
		}
	}
	for k, v := range prov.ExtraHeaders {
		httpReq.Header.Set(k, v)
	}
	httpReq.Header.Set("User-Agent", "AnyRouter/1.0")

	timeout := prov.TimeoutSec
	if timeout <= 0 {
		timeout = 30
	}
	client := &http.Client{Timeout: time.Duration(timeout) * time.Second}

	httpResp, err := client.Do(httpReq)
	if err != nil {
		return nil, nil, fmt.Errorf("request failed: %w", err)
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, httpResp, fmt.Errorf("read response: %w", err)
	}

	return body, httpResp, nil
}

func (sr *SmartRouter) buildFailureSummary(result *RouteResult) string {
	var parts []string
	for _, a := range result.Attempts {
		detail := fmt.Sprintf("%s/%s", a.Provider, a.Model)
		if a.RateLimited {
			detail += " [429]"
		} else if a.HTTPStatus > 0 {
			detail += fmt.Sprintf(" [%d]", a.HTTPStatus)
		}
		if a.Error != "" {
			detail += ": " + truncateStr(a.Error, 40)
		}
		parts = append(parts, detail)
	}
	return fmt.Sprintf("tried %d route(s): %s", len(result.Attempts), strings.Join(parts, " → "))
}

// ─── Streaming Route ──────────────────────────────────────────────────

func (sr *SmartRouter) RouteStream(req *CanonicalChatRequest) (*RouteResult, error) {
	result, err := sr.Route(req)
	if err != nil {
		return result, err
	}
	// Streaming needs the raw response body to be passed through
	return result, nil
}

// ─── Parallel Health Check ────────────────────────────────────────────

func (sr *SmartRouter) CheckAllProviders() []string {
	var results []string
	for _, name := range sr.health.AllProviderNames() {
		summary := sr.health.ProviderHealthSummary(name)
		results = append(results, summary)
	}
	return results
}

// ─── Helpers ──────────────────────────────────────────────────────────

func maskKey(key string) string {
	if len(key) <= 8 {
		return key
	}
	return key[:4] + "****" + key[len(key)-4:]
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// ─── Server Integration ───────────────────────────────────────────────

func (sr *SmartRouter) HandleRequest(inputFormat string, body []byte) ([]byte, int, string, map[string]string) {
	canonicalReq, err := ParseToCanonical(inputFormat, body)
	if err != nil {
		errResp, _ := json.Marshal(map[string]any{
			"error": map[string]any{"message": fmt.Sprintf("parse error: %v", err), "type": "anyrouter_error"},
		})
		return errResp, 400, "application/json", nil
	}

	streaming := canonicalReq.Stream
	if streaming {
		return sr.handleStreamingRequest(inputFormat, canonicalReq, body)
	}

	return sr.handleSyncRequest(inputFormat, canonicalReq)
}

func (sr *SmartRouter) handleSyncRequest(inputFormat string, canonicalReq *CanonicalChatRequest) ([]byte, int, string, map[string]string) {
	result, err := sr.Route(canonicalReq)
	if err != nil {
		errResp, _ := json.Marshal(map[string]any{
			"error": map[string]any{
				"message": err.Error(),
				"type":    "anyrouter_error",
				"attempts": result.Attempts,
			},
		})
		return errResp, 502, "application/json", nil
	}

	// Parse provider response
	canonicalResp, err := ConvertNativeToCanonical(result.APIConfig.Format, result.Response)
	if err != nil {
		errResp, _ := json.Marshal(map[string]any{
			"error": map[string]any{"message": fmt.Sprintf("parse response: %v", err), "type": "anyrouter_error"},
		})
		return errResp, 502, "application/json", nil
	}

	outputBody, err := ConvertCanonicalToOutput(inputFormat, canonicalResp)
	if err != nil {
		errResp, _ := json.Marshal(map[string]any{
			"error": map[string]any{"message": fmt.Sprintf("format output: %v", err), "type": "anyrouter_error"},
		})
		return errResp, 500, "application/json", nil
	}

	headers := map[string]string{
		"X-AnyRouter-Provider":   result.Provider,
		"X-AnyRouter-Model":      result.Model,
		"X-AnyRouter-Attempts":   fmt.Sprintf("%d", result.TotalAttempts),
	}

	return outputBody, 200, "application/json", headers
}

func (sr *SmartRouter) handleStreamingRequest(inputFormat string, canonicalReq *CanonicalChatRequest, body []byte) ([]byte, int, string, map[string]string) {
	// For streaming, try the first available key/provider
	result, err := sr.Route(canonicalReq)
	if err != nil {
		errResp, _ := json.Marshal(map[string]any{
			"error": map[string]any{
				"message": err.Error(),
				"type":    "anyrouter_error",
				"attempts": result.Attempts,
			},
		})
		return errResp, 502, "application/json", nil
	}

	// For streaming, we need to handle this differently in the HTTP handler
	// The response body needs to be read chunk by chunk
	headers := map[string]string{
		"X-AnyRouter-Provider":   result.Provider,
		"X-AnyRouter-Model":      result.Model,
		"X-AnyRouter-Attempts":   fmt.Sprintf("%d", result.TotalAttempts),
	}
	_ = body
	_ = inputFormat

	// Return the raw response body for streaming
	respBody, _ := io.ReadAll(result.Response.Body)
	return respBody, 200, "text/event-stream", headers
}
