# Contributing to AnyRouter

First off, thank you for considering contributing to AnyRouter. It is people like you that make AnyRouter such a great tool.

## Code of Conduct

This project and everyone participating in it is governed by our Code of Conduct. By participating, you are expected to uphold this code.

## How Can I Contribute?

### Reporting Bugs

Before creating bug reports, please check the existing issues list to see if the problem has already been reported. When you create a bug report, include as many details as possible:

- Use a clear and descriptive title
- Describe the exact steps to reproduce the problem
- Describe the behavior you observed and what you expected to see
- Include the version of AnyRouter (`anyrouter --version`)
- Include your operating system and terminal emulator

### Suggesting Enhancements

Enhancement suggestions are tracked as GitHub issues. When creating an enhancement suggestion:

- Use a clear and descriptive title
- Provide a step-by-step description of the suggested enhancement
- Describe the current behavior and explain the behavior you expected
- Explain why this enhancement would be useful

### Pull Requests

1. Fork the repository and create your branch from `main`
2. If you have added code, add tests
3. Ensure the test suite passes
4. Make sure your code lints (`go vet ./...`)
5. Update the README.md with details of changes if needed

## Development Setup

```bash
# Clone the repository
git clone https://github.com/anyrouter/cli.git
cd cli

# Build
go build -o anyrouter . 

# Run
./anyrouter

# Run in server mode
./anyrouter --serve

# Run tests
go test ./...

# Run vet
go vet ./...
```

## Project Structure

```
anyrouter/
  main.go          - Entry point and CLI flags
  banner.go        - ASCII art banner and styles
  config.go        - Configuration types and YAML loading
  convert.go       - Bidirectional format conversion
  health.go        - Health tracking and circuit breaker
  providers.go     - Provider routing and health checks
  router.go        - Smart failover router
  server.go        - HTTP proxy server
  tui.go           - Interactive TUI
  anyrouter.yaml   - Default configuration
```

## Adding a New Provider

1. Add the provider to `defaultProviders()` in `config.go`
2. Add model route prefixes to `defaultModelRoutes()` in `config.go`
3. If the provider uses a non-standard format, add conversion logic to `convert.go`
4. Update the README provider list

## Style Guide

- Follow standard Go formatting (`go fmt`)
- Keep functions focused and small
- Write descriptive comments for exported functions
- Avoid external dependencies when possible
- Use consistent naming throughout

## Support

If you need help with contributing, feel free to:

- Open a discussion on GitHub
- Join our community chat
- Check the documentation at https://anyrouter.planixx.com

Thank you for contributing!
