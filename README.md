# AnyRouter

Universal LLM API Router & Converter. Route requests between 30+ LLM providers with smart failover, multi-key rotation, and bidirectional format conversion.

```
  █████  ███    ██ ██    ██ ██████   ██████  ██    ██ ████████ ███████ ██████  
 ██   ██ ████   ██  ██  ██  ██   ██ ██    ██ ██    ██    ██    ██      ██   ██ 
 ███████ ██ ██  ██   ████   ██████  ██    ██ ██    ██    ██    █████   ██████  
 ██   ██ ██  ██ ██    ██    ██   ██ ██    ██ ██    ██    ██    ██      ██   ██ 
 ██   ██ ██   ████    ██    ██   ██  ██████   ██████     ██    ███████ ██   ██
 =============================================================================

```

## Features

- **Multi-Provider Routing** -- 30+ LLM providers pre-configured (OpenAI, Anthropic, Gemini, Mistral, DeepSeek, Groq, Ollama, etc.)
- **Bidirectional Format Conversion** -- OpenAI <-> Anthropic <-> Gemini <-> Cohere format translation
- **Smart Failover** -- Automatic fallback through API keys, model alternatives, and provider chains
- **Multi-Key Rotation** -- Distribute requests across multiple API keys per provider
- **Rate Limit Awareness** -- Detects 429 errors, backs off, retries with different keys
- **Circuit Breaker** -- Prevents hammering unhealthy endpoints
- **Interactive TUI** -- Full terminal interface for managing configuration
- **Custom Providers** -- Add any provider with any format
- **Streaming Support** -- SSE streaming with format conversion
- **OpenAI-Compatible Endpoint** -- Drop-in replacement for any OpenAI SDK

## Quick Install

```bash
# Linux / macOS
curl -fsSL https://raw.githubusercontent.com/anyrouter/cli/main/scripts/install.sh | bash

# Windows (PowerShell)
powershell -c "irm https://raw.githubusercontent.com/anyrouter/cli/main/scripts/install.ps1 | iex"

# Or download from releases
```

## Usage

```bash
# Interactive TUI (default)
anyrouter

# Direct server mode
anyrouter --serve

# Show support info
anyrouter --support
```

Once the server is running, point your LLM client to:

```
http://127.0.0.1:9876/v1/chat/completions
```

## Configuration

Edit `anyrouter.yaml` or use the interactive TUI to configure providers. Set API keys via environment variables:

```bash
export OPENAI_API_KEY=sk-...
export ANTHROPIC_API_KEY=sk-ant-...
export GEMINI_API_KEY=...
```

Multi-key configuration:

```yaml
providers:
  openai:
    api_keys:
      - "${OPENAI_KEY_1}"
      - "${OPENAI_KEY_2}"
    models:
      - gpt-4o
      - gpt-4o-mini
    model_fallbacks:
      - gpt-4o-mini
    fallbacks:
      - azure
      - openrouter
```

## Supported Providers

OpenAI, Anthropic, Google Gemini, Mistral, Cohere, Meta Llama, xAI Grok, DeepSeek, Groq, Together AI, Fireworks AI, Perplexity, Azure OpenAI, Amazon Bedrock, OpenRouter, Alibaba Cloud Qwen, Baidu ERNIE, Zhipu GLM, Moonshot Kimi, IBM watsonx, Azure AI Foundry, NVIDIA NIM, DeepInfra, Replicate, Hugging Face, Ollama, LM Studio, AI21 Jamba, SambaNova, Google Vertex AI.

## Architecture

```
Client (OpenAI SDK)        AnyRouter               Provider API
       |                       |                       |
       |  POST /v1/chat/       |                       |
       |  completions          |                       |
       |---------------------->|                       |
       |  OpenAI format        |                       |
       |                       |  Convert to native    |
       |                       |  Try key 1...N        |
       |                       |  Try model fallback   |
       |                       |  Try provider fallback|
       |                       |---------------------->|
       |                       |  Native format         |
       |                       |<----------------------|
       |  Convert back to      |                       |
       |  original format      |                       |
       |<----------------------|                       |
```

## Support

- Website: https://anyrouter.planixx.com
- GitHub: https://github.com/anyrouter/cli
- Documentation: https://anyrouter.planixx.com/docs

## License

MIT License
