package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type ServerConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	LogLevel string `yaml:"log_level"`
}

type ProviderConfig struct {
	Enabled        bool              `yaml:"enabled"`
	Name           string            `yaml:"-"`
	BaseURL        string            `yaml:"base_url"`
	APIKey         string            `yaml:"api_key,omitempty"`
	APIKeys        []string          `yaml:"api_keys,omitempty"`
	Format         string            `yaml:"format"`
	ExtraHeaders   map[string]string `yaml:"extra_headers,omitempty"`
	Models         []string          `yaml:"models,omitempty"`
	ModelFallbacks []string          `yaml:"model_fallbacks,omitempty"`
	Fallbacks      []string          `yaml:"fallbacks,omitempty"`
	TimeoutSec     int               `yaml:"timeout_sec,omitempty"`
	MaxRetries     int               `yaml:"max_retries,omitempty"`
	MaxRPM         int               `yaml:"max_rpm,omitempty"`
	MaxTPM         int               `yaml:"max_tpm,omitempty"`
	CooldownSec    int               `yaml:"cooldown_sec,omitempty"`
	Custom         bool              `yaml:"-"`
}

func (p *ProviderConfig) GetAllAPIKeys() []string {
	if len(p.APIKeys) > 0 {
		return p.APIKeys
	}
	if p.APIKey != "" {
		return []string{p.APIKey}
	}
	return nil
}

type ModelRoute struct {
	Pattern  string `yaml:"pattern"`
	Provider string `yaml:"provider"`
}

type Endpoint struct {
	Path   string `yaml:"path"`
	Format string `yaml:"format"`
}

type Config struct {
	Server        ServerConfig              `yaml:"server"`
	Providers     map[string]ProviderConfig `yaml:"providers"`
	Models        []ModelRoute              `yaml:"models"`
	Endpoints     []Endpoint                `yaml:"endpoints"`
	ConfigFile    string                    `yaml:"-"`
	DefaultFormat string                    `yaml:"default_format"`
}

func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host: "127.0.0.1", Port: 9876, LogLevel: "info",
		},
		DefaultFormat: "openai",
		Providers:     defaultProviders(),
		Models:        defaultModelRoutes(),
		Endpoints: []Endpoint{
			{Path: "/v1/chat/completions", Format: "openai"},
			{Path: "/v1/messages", Format: "anthropic"},
		},
	}
}

func defaultProviders() map[string]ProviderConfig {
	return map[string]ProviderConfig{
		"openai": {
			Enabled: true, BaseURL: "https://api.openai.com/v1",
			APIKey: "${OPENAI_API_KEY}", Format: "openai",
			Models: []string{"gpt-4o", "gpt-4o-mini", "gpt-4.1", "o3", "o4-mini"},
			ModelFallbacks: []string{"gpt-4o-mini"},
			Fallbacks: []string{"azure", "openrouter"},
			TimeoutSec: 60, MaxRetries: 3, MaxRPM: 500, CooldownSec: 30,
		},
		"anthropic": {
			Enabled: true, BaseURL: "https://api.anthropic.com",
			APIKey: "${ANTHROPIC_API_KEY}", Format: "anthropic",
			ExtraHeaders: map[string]string{"anthropic-version": "2023-06-01"},
			Models: []string{"claude-sonnet-4", "claude-opus-4", "claude-haiku-4"},
			ModelFallbacks: []string{"claude-haiku-4"},
			Fallbacks: []string{"vertex", "bedrock"},
			TimeoutSec: 120, MaxRetries: 3, MaxRPM: 1000, CooldownSec: 30,
		},
		"gemini": {
			Enabled: true, BaseURL: "https://generativelanguage.googleapis.com/v1beta",
			APIKey: "${GEMINI_API_KEY}", Format: "gemini",
			Models: []string{"gemini-2.5-pro", "gemini-2.5-flash"},
			ModelFallbacks: []string{"gemini-2.5-flash"},
			TimeoutSec: 60, MaxRetries: 2,
		},
		"mistral": {
			Enabled: true, BaseURL: "https://api.mistral.ai/v1",
			APIKey: "${MISTRAL_API_KEY}", Format: "openai",
			Models: []string{"mistral-large-latest", "mistral-small", "codestral", "pixtral"},
		},
		"cohere": {
			Enabled: true, BaseURL: "https://api.cohere.com/v2",
			APIKey: "${COHERE_API_KEY}", Format: "cohere",
			Models: []string{"command-r", "command-r-plus"},
		},
		"meta": {
			Enabled: true, BaseURL: "https://api.llama.com/v1",
			APIKey: "${META_API_KEY}", Format: "openai",
			Models: []string{"llama-4-maverick", "llama-4-scout", "llama-3.3-70b"},
		},
		"xai": {
			Enabled: true, BaseURL: "https://api.x.ai/v1",
			APIKey: "${XAI_API_KEY}", Format: "openai",
			Models: []string{"grok-4", "grok-3-mini"},
		},
		"deepseek": {
			Enabled: true, BaseURL: "https://api.deepseek.com/v1",
			APIKey: "${DEEPSEEK_API_KEY}", Format: "openai",
			Models: []string{"deepseek-chat", "deepseek-reasoner"},
		},
		"groq": {
			Enabled: true, BaseURL: "https://api.groq.com/openai/v1",
			APIKey: "${GROQ_API_KEY}", Format: "openai",
			Models: []string{"llama-3.3-70b-versatile", "mixtral-8x7b", "gemma2"},
		},
		"together": {
			Enabled: true, BaseURL: "https://api.together.xyz/v1",
			APIKey: "${TOGETHER_API_KEY}", Format: "openai",
		},
		"fireworks": {
			Enabled: true, BaseURL: "https://api.fireworks.ai/inference/v1",
			APIKey: "${FIREWORKS_API_KEY}", Format: "openai",
		},
		"perplexity": {
			Enabled: true, BaseURL: "https://api.perplexity.ai",
			APIKey: "${PERPLEXITY_API_KEY}", Format: "openai",
			Models: []string{"sonar", "sonar-pro", "sonar-reasoning"},
		},
		"azure": {
			Enabled: true,
			BaseURL: "https://${AZURE_RESOURCE}.openai.azure.com/openai/deployments/${AZURE_DEPLOYMENT}",
			APIKey: "${AZURE_API_KEY}", Format: "openai",
			ExtraHeaders: map[string]string{"api-key": "${AZURE_API_KEY}"},
		},
		"bedrock": {
			Enabled: false,
			BaseURL: "https://bedrock-runtime.${AWS_REGION}.amazonaws.com",
			APIKey: "${AWS_ACCESS_KEY_ID}:${AWS_SECRET_ACCESS_KEY}", Format: "bedrock",
		},
		"openrouter": {
			Enabled: true, BaseURL: "https://openrouter.ai/api/v1",
			APIKey: "${OPENROUTER_API_KEY}", Format: "openai",
		},
		"alibaba": {
			Enabled: true, BaseURL: "https://dashscope-intl.aliyuncs.com/compatible-mode/v1",
			APIKey: "${ALIBABA_API_KEY}", Format: "openai",
			Models: []string{"qwen-max", "qwen-plus", "qwen-coder-plus"},
		},
		"baidu": {
			Enabled: true, BaseURL: "https://qianfan.baidubce.com/v2",
			APIKey: "${BAIDU_API_KEY}", Format: "openai",
			Models: []string{"ernie-4.5", "ernie-x1"},
		},
		"zhipu": {
			Enabled: true, BaseURL: "https://open.bigmodel.cn/api/paas/v4",
			APIKey: "${ZHIPU_API_KEY}", Format: "openai",
			Models: []string{"glm-4.6", "glm-4.5-air"},
		},
		"moonshot": {
			Enabled: true, BaseURL: "https://api.moonshot.cn/v1",
			APIKey: "${MOONSHOT_API_KEY}", Format: "openai",
			Models: []string{"kimi-k2", "moonshot-v1-128k"},
		},
		"watsonx": {
			Enabled: false,
			BaseURL: "https://${WATSONX_REGION}.ml.cloud.ibm.com/ml/v1/text/chat",
			APIKey: "${WATSONX_API_KEY}", Format: "watsonx",
		},
		"azure_foundry": {
			Enabled: true,
			BaseURL: "https://${AZURE_FOUNDRY_RESOURCE}.services.ai.azure.com/models",
			APIKey: "${AZURE_FOUNDRY_API_KEY}", Format: "openai",
		},
		"nvidia": {
			Enabled: true, BaseURL: "https://integrate.api.nvidia.com/v1",
			APIKey: "${NVIDIA_API_KEY}", Format: "openai",
		},
		"deepinfra": {
			Enabled: true, BaseURL: "https://api.deepinfra.com/v1/openai",
			APIKey: "${DEEPINFRA_API_KEY}", Format: "openai",
		},
		"replicate": {
			Enabled: false, BaseURL: "https://api.replicate.com/v1",
			APIKey: "${REPLICATE_API_KEY}", Format: "replicate",
		},
		"huggingface": {
			Enabled: true, BaseURL: "https://router.huggingface.co/v1",
			APIKey: "${HUGGINGFACE_API_KEY}", Format: "openai",
		},
		"ollama": {
			Enabled: true, BaseURL: "http://localhost:11434/v1",
			APIKey: "", Format: "openai",
			Models: []string{"llama3", "qwen2.5", "mistral", "codestral"},
		},
		"lmstudio": {
			Enabled: true, BaseURL: "http://localhost:1234/v1",
			APIKey: "", Format: "openai",
		},
		"ai21": {
			Enabled: true, BaseURL: "https://api.ai21.com/studio/v1",
			APIKey: "${AI21_API_KEY}", Format: "openai",
			Models: []string{"jamba-large", "jamba-mini"},
		},
		"sambanova": {
			Enabled: true, BaseURL: "https://api.sambanova.ai/v1",
			APIKey: "${SAMBANOVA_API_KEY}", Format: "openai",
		},
		"vertex": {
			Enabled: false, BaseURL: "https://${VERTEX_REGION}-aiplatform.googleapis.com/v1",
			APIKey: "${VERTEX_ACCESS_TOKEN}", Format: "gemini",
		},
	}
}

func defaultModelRoutes() []ModelRoute {
	routes := []ModelRoute{}
	prefixes := map[string]string{
		"gpt-": "openai", "o3": "openai", "o4": "openai",
		"claude-": "anthropic", "gemini-": "gemini",
		"mistral-": "mistral", "codestral": "mistral", "pixtral": "mistral",
		"command-": "cohere", "llama-": "meta", "grok-": "xai",
		"deepseek-": "deepseek", "mixtral": "groq", "gemma": "groq",
		"sonar": "perplexity", "qwen-": "alibaba", "ernie-": "baidu",
		"glm-": "zhipu", "kimi-": "moonshot", "jamba": "ai21",
	}
	for p, prov := range prefixes {
		routes = append(routes, ModelRoute{Pattern: p, Provider: prov})
	}
	return routes
}

func LoadConfig(path string) (*Config, error) {
	cfg := DefaultConfig()
	cfg.ConfigFile = path
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			WriteDefaultConfig(path)
			return cfg, nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	for name, p := range cfg.Providers {
		p.Name = name
		cfg.Providers[name] = p
	}
	resolveEnvVars(cfg)
	return cfg, nil
}

func (c *Config) Save() error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	return os.WriteFile(c.ConfigFile, data, 0644)
}

func (c *Config) DeleteProvider(name string) {
	delete(c.Providers, name)
	var routes []ModelRoute
	for _, r := range c.Models {
		if r.Provider != name {
			routes = append(routes, r)
		}
	}
	c.Models = routes
}

func resolveEnvVars(cfg *Config) {
	for name, p := range cfg.Providers {
		p.APIKey = resolveEnv(p.APIKey)
		p.BaseURL = resolveEnv(p.BaseURL)
		for i, k := range p.APIKeys {
			p.APIKeys[i] = resolveEnv(k)
		}
		for k, v := range p.ExtraHeaders {
			p.ExtraHeaders[k] = resolveEnv(v)
		}
		cfg.Providers[name] = p
	}
}

func resolveEnv(s string) string {
	if !strings.Contains(s, "${") {
		return s
	}
	result := s
	for {
		start := strings.Index(result, "${")
		if start == -1 {
			break
		}
		end := strings.Index(result[start:], "}")
		if end == -1 {
			break
		}
		end = start + end + 1
		varName := result[start+2 : end-1]
		defaultVal := ""
		if idx := strings.Index(varName, ":-"); idx != -1 {
			defaultVal = varName[idx+2:]
			varName = varName[:idx]
		}
		envVal := os.Getenv(varName)
		if envVal == "" {
			envVal = defaultVal
		}
		result = result[:start] + envVal + result[end:]
	}
	return result
}

func FindConfigPath() string {
	paths := []string{
		"anyrouter.yaml", "anyrouter.yml",
		filepath.Join(os.Getenv("HOME"), ".anyrouter.yaml"),
		filepath.Join(os.Getenv("HOME"), ".config", "anyrouter", "config.yaml"),
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		paths = append([]string{filepath.Join(xdg, "anyrouter", "config.yaml")}, paths...)
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "anyrouter.yaml"
}

func WriteDefaultConfig(path string) error {
	cfg := DefaultConfig()
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}
