# Download AnyRouter

AnyRouter is distributed as a single static binary for each platform. No dependencies required.

## Latest Release (v1.0.0)

### Windows

| Architecture | Download |
|-------------|----------|
| AMD64 | [anyrouter-windows-amd64.exe](./build/anyrouter-windows-amd64.exe) |
| ARM64 | [anyrouter-windows-arm64.exe](./build/anyrouter-windows-arm64.exe) |

### macOS

| Architecture | Download |
|-------------|----------|
| Intel | [anyrouter-darwin-amd64](./build/anyrouter-darwin-amd64) |
| Apple Silicon | [anyrouter-darwin-arm64](./build/anyrouter-darwin-arm64) |

### Linux

| Architecture | Download |
|-------------|----------|
| AMD64 | [anyrouter-linux-amd64](./build/anyrouter-linux-amd64) |
| ARM64 | [anyrouter-linux-arm64](./build/anyrouter-linux-arm64) |

## Installation

### Quick Install (Recommended)

**Linux / macOS:**

```bash
curl -fsSL https://raw.githubusercontent.com/anyrouter/cli/main/scripts/install.sh | bash
```

**Windows (PowerShell as Administrator):**

```powershell
powershell -c "irm https://raw.githubusercontent.com/anyrouter/cli/main/scripts/install.ps1 | iex"
```

### Manual Installation

1. Download the appropriate binary for your platform from the table above
2. Make it executable (macOS/Linux: `chmod +x anyrouter-*`)
3. Move it to a directory in your PATH:
   - Linux/macOS: `sudo mv anyrouter-* /usr/local/bin/anyrouter`
   - Windows: move to `C:\Windows\System32\` or a custom directory added to PATH
4. Verify: `anyrouter --version`

### Build from Source

```bash
git clone https://github.com/anyrouter/cli.git
cd cli
go build -o anyrouter .
```

Requires Go 1.21 or later.

## Configuration

Edit `anyrouter.yaml` or run `anyrouter` to use the interactive TUI.

## After Installation

```bash
# Start the interactive TUI
anyrouter

# Start in server mode
anyrouter --serve

# View all options
anyrouter --help
```

## Signature Verification

SHA256 checksums are available in the release notes on GitHub.

## System Requirements

- Windows 10+ / macOS 12+ / Linux kernel 4.x+
- Terminal with ANSI color support (modern terminals recommended)
- Network access to your LLM provider APIs
