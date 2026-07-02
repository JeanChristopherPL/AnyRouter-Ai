package main

import (
	"net/http"
	"strings"
)

func ResolveProvider(cfg *Config, model string) (string, *ProviderConfig, error) {
	for name, prov := range cfg.Providers {
		if !prov.Enabled {
			continue
		}
		for _, m := range prov.Models {
			if model == m {
				return name, &prov, nil
			}
		}
	}

	for _, route := range cfg.Models {
		if strings.HasPrefix(model, route.Pattern) {
			prov, ok := cfg.Providers[route.Provider]
			if !ok || !prov.Enabled {
				continue
			}
			return route.Provider, &prov, nil
		}
	}

	for name, prov := range cfg.Providers {
		if !prov.Enabled {
			continue
		}
		if strings.HasPrefix(strings.ToLower(model), name) {
			return name, &prov, nil
		}
	}

	return "", nil, ErrNoProviderFound(model)
}

func CheckProviderHealth(prov *ProviderConfig) (bool, string) {
	client := &http.Client{Timeout: 5e9}
	req, err := http.NewRequest("GET", prov.BaseURL+"/models", nil)
	if err != nil {
		return false, err.Error()
	}
	if prov.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+prov.APIKey)
	}
	resp, err := client.Do(req)
	if err != nil {
		return false, err.Error()
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 500 {
		return true, "ok"
	}
	return false, "status " + http.StatusText(resp.StatusCode)
}

func buildTargetURL(prov *ProviderConfig, req *CanonicalChatRequest) string {
	switch prov.Format {
	case "anthropic":
		return prov.BaseURL + "/v1/messages"
	case "gemini":
		return prov.BaseURL + "/models/" + req.Model + ":generateContent"
	case "cohere":
		return prov.BaseURL + "/chat"
	case "bedrock":
		return prov.BaseURL + "/model/" + req.Model + "/invoke"
	default:
		return prov.BaseURL + "/chat/completions"
	}
}

func DetectInputFormat(r *http.Request) string {
	path := r.URL.Path
	if strings.Contains(path, "/v1/messages") {
		return "anthropic"
	}
	if strings.Contains(path, "generateContent") || strings.Contains(path, "gemini") {
		return "gemini"
	}
	return "openai"
}

type ProviderError struct {
	Msg string
}

func (e ProviderError) Error() string {
	return "provider: " + e.Msg
}

func ErrNoProviderFound(model string) error {
	return ProviderError{Msg: "no provider found for model \"" + model + "\""}
}
