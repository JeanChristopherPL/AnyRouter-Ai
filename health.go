package main

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

type KeyStatus int

const (
	KeyHealthy    KeyStatus = iota
	KeyRateLimited
	KeyAuthError
	KeyQuotaExhausted
	KeyDown
	KeyDegraded
)

func (ks KeyStatus) String() string {
	switch ks {
	case KeyHealthy:
		return "healthy"
	case KeyRateLimited:
		return "rate_limited"
	case KeyAuthError:
		return "auth_error"
	case KeyQuotaExhausted:
		return "quota_exhausted"
	case KeyDown:
		return "down"
	case KeyDegraded:
		return "degraded"
	}
	return "unknown"
}

type ErrorClass int

const (
	ErrSuccess        ErrorClass = iota
	ErrRateLimited
	ErrAuth
	ErrQuota
	ErrServerError
	ErrTimeout
	ErrNetwork
	ErrBadRequest
	ErrUnknown
)

type KeyHealth struct {
	mu               sync.RWMutex
	Key              string
	Status           KeyStatus
	ConsecutiveErr   int
	TotalRequests    int64
	SuccessfulReq    int64
	FailedReq        int64
	LastAttempt      time.Time
	LastSuccess      time.Time
	LastError        string
	LastHTTPStatus   int
	RateLimitedUntil time.Time
	BackoffUntil     time.Time
	CooldownSec      int
	RequestsInMin    int
	WindowReset      time.Time
	TokensInMin      int
	TokenWindowReset time.Time
	MaxRPM           int
	MaxTPM           int
}

func (kh *KeyHealth) IsAvailable() bool {
	kh.mu.RLock()
	defer kh.mu.RUnlock()
	now := time.Now()
	if now.Before(kh.RateLimitedUntil) {
		return false
	}
	if now.Before(kh.BackoffUntil) {
		return false
	}
	if now.Before(kh.WindowReset) {
		if kh.MaxRPM > 0 && kh.RequestsInMin >= kh.MaxRPM {
			return false
		}
	}
	if now.Before(kh.TokenWindowReset) {
		if kh.MaxTPM > 0 && kh.TokensInMin >= kh.MaxTPM {
			return false
		}
	}
	if kh.Status == KeyAuthError || kh.Status == KeyQuotaExhausted {
		return false
	}
	if kh.Status == KeyDown && kh.ConsecutiveErr >= 5 {
		return false
	}
	return true
}

func (kh *KeyHealth) CanRetry() bool {
	kh.mu.RLock()
	defer kh.mu.RUnlock()
	if kh.Status == KeyAuthError || kh.Status == KeyQuotaExhausted {
		return false
	}
	return true
}

func (kh *KeyHealth) TimeToAvailable() string {
	kh.mu.RLock()
	defer kh.mu.RUnlock()
	now := time.Now()
	if now.Before(kh.RateLimitedUntil) {
		return fmt.Sprintf("rate-limited for %s", kh.RateLimitedUntil.Sub(now).Round(time.Second))
	}
	if now.Before(kh.BackoffUntil) {
		return fmt.Sprintf("backoff for %s", kh.BackoffUntil.Sub(now).Round(time.Second))
	}
	if kh.Status == KeyAuthError {
		return "authentication error - manual fix needed"
	}
	if kh.Status == KeyQuotaExhausted {
		return "quota exhausted - manual top-up needed"
	}
	if kh.Status == KeyDown {
		return fmt.Sprintf("down after %d errors", kh.ConsecutiveErr)
	}
	return "available"
}

type ProviderHealth struct {
	mu             sync.RWMutex
	Name           string
	Available      bool
	Degraded       bool
	Keys           map[string]*KeyHealth
	LastCheck      time.Time
	PrimaryModel   string
	ModelFallbacks []string
	DownSince      time.Time
	LastError      string
	LastErrorTime  time.Time
}

func (ph *ProviderHealth) updateStatus() {
	ph.mu.Lock()
	defer ph.mu.Unlock()
	now := time.Now()
	available := 0
	total := len(ph.Keys)
	canRetry := 0
	for _, kh := range ph.Keys {
		kh.mu.RLock()
		if kh.IsAvailable() {
			available++
		}
		if kh.CanRetry() {
			canRetry++
		}
		kh.mu.RUnlock()
	}
	if available > 0 {
		ph.Available = true
		ph.Degraded = available < total
		ph.DownSince = time.Time{}
	} else if canRetry > 0 {
		ph.Available = false
		ph.Degraded = true
		if ph.DownSince.IsZero() {
			ph.DownSince = now
		}
	} else {
		ph.Available = false
		ph.Degraded = false
		if ph.DownSince.IsZero() {
			ph.DownSince = now
		}
	}
}

type HealthTracker struct {
	mu        sync.RWMutex
	providers map[string]*ProviderHealth
	rpmCounts map[string]map[int64]int
}

func NewHealthTracker(cfg *Config) *HealthTracker {
	ht := &HealthTracker{
		providers: make(map[string]*ProviderHealth),
		rpmCounts: make(map[string]map[int64]int),
	}
	for name, prov := range cfg.Providers {
		if !prov.Enabled {
			continue
		}
		ph := &ProviderHealth{
			Name:           name,
			Available:      true,
			Keys:           make(map[string]*KeyHealth),
			ModelFallbacks: prov.ModelFallbacks,
		}
		keys := prov.GetAllAPIKeys()
		if len(keys) == 0 {
			keys = []string{"default"}
		}
		for _, k := range keys {
			kh := &KeyHealth{
				Key:         k,
				Status:      KeyHealthy,
				WindowReset: time.Now(),
				CooldownSec: prov.CooldownSec,
				MaxRPM:      prov.MaxRPM,
				MaxTPM:      prov.MaxTPM,
			}
			if kh.CooldownSec == 0 {
				kh.CooldownSec = 30
			}
			ph.Keys[k] = kh
		}
		ht.providers[name] = ph
	}
	return ht
}

func (ht *HealthTracker) RecordAttempt(provider, key string, httpStatus int, body []byte, err error, duration time.Duration, tokenCount int) {
	ht.mu.RLock()
	ph, ok := ht.providers[provider]
	ht.mu.RUnlock()
	if !ok {
		return
	}

	ph.mu.Lock()
	kh, ok := ph.Keys[key]
	if !ok {
		// Check if this key exists in the provider config (maybe new key added)
		for _, k := range ph.Keys {
			kh = k
			break
		}
		if kh == nil {
			ph.mu.Unlock()
			return
		}
	}
	ph.mu.Unlock()

	kh.mu.Lock()
	defer kh.mu.Unlock()

	kh.LastAttempt = time.Now()
	kh.TotalRequests++
	_ = duration

	// Detect error class from HTTP status
	errClass := classifyError(httpStatus, body, err)

	switch errClass {
	case ErrSuccess:
		kh.Status = KeyHealthy
		kh.ConsecutiveErr = 0
		kh.SuccessfulReq++
		kh.LastSuccess = time.Now()
		kh.LastError = ""

		// Track RPM
		if kh.MaxRPM > 0 {
			ht.recordRPM(provider, key)
		}
		if kh.MaxTPM > 0 && tokenCount > 0 {
			kh.TokensInMin += tokenCount
		}

	case ErrRateLimited:
		kh.Status = KeyRateLimited
		kh.ConsecutiveErr++
		kh.FailedReq++
		kh.LastHTTPStatus = httpStatus
		kh.LastError = extractErrorMessage(body, err)

		// Parse Retry-After if available
		retryAfter := extractRetryAfter(body, httpStatus)
		cooldown := time.Duration(kh.CooldownSec) * time.Second
		if retryAfter > 0 {
			cooldown = time.Duration(retryAfter) * time.Second
		}
		// Exponential backoff based on consecutive errors
		backoff := cooldown * time.Duration(1+kh.ConsecutiveErr/2)
		if backoff > 5*time.Minute {
			backoff = 5 * time.Minute
		}
		kh.RateLimitedUntil = time.Now().Add(backoff)

	case ErrAuth:
		kh.Status = KeyAuthError
		kh.ConsecutiveErr++
		kh.FailedReq++
		kh.LastHTTPStatus = httpStatus
		kh.LastError = extractErrorMessage(body, err)
		// Don't retry auth errors for 5 minutes
		kh.BackoffUntil = time.Now().Add(5 * time.Minute)

	case ErrQuota:
		kh.Status = KeyQuotaExhausted
		kh.ConsecutiveErr++
		kh.FailedReq++
		kh.LastHTTPStatus = httpStatus
		kh.LastError = extractErrorMessage(body, err)
		// Quota errors: wait longer
		kh.BackoffUntil = time.Now().Add(10 * time.Minute)

	case ErrServerError:
		kh.FailedReq++
		kh.ConsecutiveErr++
		kh.LastHTTPStatus = httpStatus
		kh.LastError = extractErrorMessage(body, err)
		if kh.ConsecutiveErr >= 3 {
			kh.Status = KeyDown
			backoff := time.Duration(30*kh.ConsecutiveErr) * time.Second
			if backoff > 5*time.Minute {
				backoff = 5 * time.Minute
			}
			kh.BackoffUntil = time.Now().Add(backoff)
		}

	case ErrTimeout, ErrNetwork:
		kh.FailedReq++
		kh.ConsecutiveErr++
		kh.LastError = extractErrorMessage(body, err)
		if kh.ConsecutiveErr >= 2 {
			backoff := time.Duration(10*kh.ConsecutiveErr) * time.Second
			if backoff > 2*time.Minute {
				backoff = 2 * time.Minute
			}
			kh.BackoffUntil = time.Now().Add(backoff)
		}

	default:
		kh.FailedReq++
		kh.ConsecutiveErr++
		kh.LastError = extractErrorMessage(body, err)
	}

	ph.updateStatus()
}

func (ht *HealthTracker) getKey(provider, key string) *KeyHealth {
	ht.mu.RLock()
	ph, ok := ht.providers[provider]
	ht.mu.RUnlock()
	if !ok {
		return nil
	}
	ph.mu.RLock()
	kh, ok := ph.Keys[key]
	ph.mu.RUnlock()
	if !ok {
		return nil
	}
	return kh
}

func (ht *HealthTracker) NextHealthyKey(provider string) (string, string) {
	ht.mu.RLock()
	ph, ok := ht.providers[provider]
	ht.mu.RUnlock()
	if !ok {
		return "", ""
	}

	ph.mu.RLock()
	defer ph.mu.RUnlock()

	// Try healthy keys first (round-robin rotation)
	var bestKey string
	var bestAPIKey string
	bestScore := -1

	for _, kh := range ph.Keys {
		kh.mu.RLock()
		available := kh.IsAvailable()
		retryAfter := ""
		if !available {
			retryAfter = kh.TimeToAvailable()
		}
		_ = retryAfter
		score := 0
		if available {
			score = 1000 - kh.ConsecutiveErr*100
			// Prefer keys with lower request counts
			if kh.MaxRPM > 0 && kh.WindowReset.After(time.Now()) {
				usage := float64(kh.RequestsInMin) / float64(kh.MaxRPM)
				score -= int(usage * 500)
			}
		}
		kh.mu.RUnlock()

		if score > bestScore {
			bestScore = score
			bestKey = kh.Key
			bestAPIKey = kh.Key
		}
	}

	if bestKey == "" || bestScore < 0 {
		return "", ""
	}
	return bestKey, bestAPIKey
}

func (ht *HealthTracker) GetProviderStatus(provider string) (available bool, degraded bool, details string) {
	ht.mu.RLock()
	ph, ok := ht.providers[provider]
	ht.mu.RUnlock()
	if !ok {
		return false, false, "unknown provider"
	}

	ph.mu.RLock()
	defer ph.mu.RUnlock()

	if ph.Available && !ph.Degraded {
		return true, false, "all keys healthy"
	}
	if ph.Available && ph.Degraded {
		return true, true, "some keys degraded"
	}

	// Check if any keys will become available soon
	var earliest time.Time
	var statusParts []string
	for _, kh := range ph.Keys {
		kh.mu.RLock()
		sta := kh.TimeToAvailable()
		if !kh.IsAvailable() && kh.CanRetry() {
			if kh.RateLimitedUntil.After(time.Now()) {
				if earliest.IsZero() || kh.RateLimitedUntil.Before(earliest) {
					earliest = kh.RateLimitedUntil
				}
			}
			if kh.BackoffUntil.After(time.Now()) {
				if earliest.IsZero() || kh.BackoffUntil.Before(earliest) {
					earliest = kh.BackoffUntil
				}
			}
		}
		statusParts = append(statusParts, fmt.Sprintf("%s: %s", kh.Key[:min(8, len(kh.Key))], sta))
		kh.mu.RUnlock()
	}

	detail := strings.Join(statusParts, "; ")
	if !earliest.IsZero() {
		detail += fmt.Sprintf(" | earliest recovery: %s", earliest.Sub(time.Now()).Round(time.Second))
	}
	return false, ph.Degraded, detail
}

func (ht *HealthTracker) recordRPM(provider, key string) {
	ht.mu.Lock()
	defer ht.mu.Unlock()
	minute := time.Now().Truncate(time.Minute).Unix()
	key_ := provider + ":" + key
	if ht.rpmCounts[key_] == nil {
		ht.rpmCounts[key_] = make(map[int64]int)
	}
	ht.rpmCounts[key_][minute]++

	// Clean old entries
	for k := range ht.rpmCounts {
		for m := range ht.rpmCounts[k] {
			if m < minute-2 {
				delete(ht.rpmCounts[k], m)
			}
		}
		if len(ht.rpmCounts[k]) == 0 {
			delete(ht.rpmCounts, k)
		}
	}
}

func (ht *HealthTracker) IsProviderAvailable(provider string) bool {
	ht.mu.RLock()
	ph, ok := ht.providers[provider]
	ht.mu.RUnlock()
	if !ok {
		return false
	}
	ph.mu.RLock()
	defer ph.mu.RUnlock()
	return ph.Available
}

func (ht *HealthTracker) IsKeyAvailable(provider, key string) bool {
	kh := ht.getKey(provider, key)
	if kh == nil {
		return false
	}
	return kh.IsAvailable()
}

func (ht *HealthTracker) AllProviderNames() []string {
	ht.mu.RLock()
	defer ht.mu.RUnlock()
	var names []string
	for n := range ht.providers {
		names = append(names, n)
	}
	return names
}

func (ht *HealthTracker) ProviderHealthSummary(name string) string {
	_, degraded, details := ht.GetProviderStatus(name)
	status := "available"
	if degraded {
		status = "degraded"
	}
	return fmt.Sprintf("%s (%s) - %s", name, status, details)
}

// ─── Error Classification ─────────────────────────────────────────────

func classifyError(statusCode int, body []byte, err error) ErrorClass {
	if statusCode >= 200 && statusCode < 300 {
		return ErrSuccess
	}
	if err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline") {
			return ErrTimeout
		}
		return ErrNetwork
	}
	switch statusCode {
	case 429:
		return ErrRateLimited
	case 401, 403:
		return ErrAuth
	case 402:
		return ErrQuota
	case 500, 502, 503:
		return ErrServerError
	case 400:
		// Check if body contains rate limit indicators
		if body != nil {
			bodyStr := strings.ToLower(string(body))
			if strings.Contains(bodyStr, "rate") || strings.Contains(bodyStr, "limit") ||
				strings.Contains(bodyStr, "too many") || strings.Contains(bodyStr, "quota") {
				return ErrRateLimited
			}
		}
		return ErrBadRequest
	default:
		return ErrUnknown
	}
}

func extractRetryAfter(body []byte, statusCode int) int {
	if body != nil {
		bodyStr := string(body)
		// Check for common retry-after formats
		if strings.Contains(bodyStr, "retry-after") || strings.Contains(bodyStr, "retry_after") {
			idx := strings.Index(strings.ToLower(bodyStr), "retry")
			after := bodyStr[idx:]
			var sec int
			if _, err := fmt.Sscanf(after, "retry-after: %d", &sec); err == nil {
				return sec
			}
			if _, err := fmt.Sscanf(after, "retry_after: %d", &sec); err == nil {
				return sec
			}
			if _, err := fmt.Sscanf(after, "retry after %d", &sec); err == nil {
				return sec
			}
		}
	}
	return 0
}

func extractErrorMessage(body []byte, err error) string {
	if err != nil {
		return err.Error()
	}
	if len(body) > 0 && len(body) < 500 {
		return string(body)
	}
	return fmt.Sprintf("HTTP %d", 0)
}
