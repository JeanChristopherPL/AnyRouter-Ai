#!/usr/bin/env bash
set -euo pipefail

REPO="anyrouter/cli"
VERSION="latest"
BINARY="anyrouter"
INSTALL_DIR="/usr/local/bin"

# Colors
CYAN='\033[0;36m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${CYAN}"
cat << "EOF"
  █████  ███    ██ ██    ██ ██████   ██████  ██    ██ ████████ ███████ ██████  
 ██   ██ ████   ██  ██  ██  ██   ██ ██    ██ ██    ██    ██    ██      ██   ██ 
 ███████ ██ ██  ██   ████   ██████  ██    ██ ██    ██    ██    █████   ██████  
 ██   ██ ██  ██ ██    ██    ██   ██ ██    ██ ██    ██    ██    ██      ██   ██ 
 ██   ██ ██   ████    ██    ██   ██  ██████   ██████     ██    ███████ ██   ██
 =============================================================================
EOF
echo -e "${NC}"
echo -e "${GREEN}AnyRouter Installer${NC}"
echo ""

# Detect OS
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  i386|i686) ARCH="386" ;;
  *)
    echo -e "${RED}Unsupported architecture: $ARCH${NC}"
    exit 1
    ;;
esac

case "$OS" in
  linux|darwin) ;;
  *)
    echo -e "${RED}Unsupported OS: $OS. Use install.ps1 for Windows.${NC}"
    exit 1
    ;;
esac

FILENAME="${BINARY}-${OS}-${ARCH}"
if [ "$OS" = "darwin" ]; then
  # macOS - try to get from GitHub releases
  DOWNLOAD_URL="https://github.com/${REPO}/releases/latest/download/${FILENAME}"
else
  DOWNLOAD_URL="https://github.com/${REPO}/releases/latest/download/${FILENAME}"
fi

echo -e "${YELLOW}Detected:${NC} $OS/$ARCH"
echo -e "${YELLOW}Download:${NC} $DOWNLOAD_URL"
echo ""

# Check if we have a local build (development mode)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOCAL_BUILD=""
if [ -f "${SCRIPT_DIR}/../build/${FILENAME}" ]; then
  LOCAL_BUILD="${SCRIPT_DIR}/../build/${FILENAME}"
elif [ -f "${SCRIPT_DIR}/../build/${FILENAME}.exe" ]; then
  LOCAL_BUILD="${SCRIPT_DIR}/../build/${FILENAME}.exe"
fi

TMP_DIR=$(mktemp -d)
trap "rm -rf $TMP_DIR" EXIT

if [ -n "$LOCAL_BUILD" ]; then
  echo -e "${GREEN}Using local build...${NC}"
  cp "$LOCAL_BUILD" "${TMP_DIR}/${BINARY}"
else
  echo -e "${YELLOW}Downloading latest release...${NC}"
  if command -v curl &> /dev/null; then
    curl -fsSL "$DOWNLOAD_URL" -o "${TMP_DIR}/${BINARY}"
  elif command -v wget &> /dev/null; then
    wget -q "$DOWNLOAD_URL" -O "${TMP_DIR}/${BINARY}"
  else
    echo -e "${RED}Neither curl nor wget found. Install one of them first.${NC}"
    exit 1
  fi
fi

chmod +x "${TMP_DIR}/${BINARY}"

echo -e "${YELLOW}Installing to ${INSTALL_DIR}/${BINARY}...${NC}"

if [ ! -w "$INSTALL_DIR" ]; then
  echo -e "${YELLOW}Requesting sudo access for installation...${NC}"
  sudo mv "${TMP_DIR}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
else
  mv "${TMP_DIR}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
fi

echo -e "${GREEN}Installation complete!${NC}"
echo ""
echo -e "Run ${CYAN}anyrouter${NC} to start the interactive TUI."
echo -e "Run ${CYAN}anyrouter --serve${NC} for direct server mode."
echo ""
echo -e "${CYAN}  Documentation: https://anyrouter.planixx.com${NC}"
echo -e "${CYAN}  GitHub:        https://github.com/${REPO}${NC}"
echo ""

# Verify
if command -v anyrouter &> /dev/null; then
  echo -e "${GREEN}Verified: $(anyrouter --version 2>&1 || echo 'anyrouter installed')${NC}"
else
  echo -e "${YELLOW}Make sure ${INSTALL_DIR} is in your PATH.${NC}"
fi
