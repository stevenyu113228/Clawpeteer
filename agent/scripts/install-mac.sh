#!/usr/bin/env bash
set -euo pipefail

# Clawpeteer Agent - macOS Installation Script
# This script installs the Clawpeteer agent as a launchd service.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
AGENT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="${HOME}/.clawpeteer"
PLIST_NAME="com.clawpeteer.agent"
PLIST_DIR="${HOME}/Library/LaunchAgents"
PLIST_FILE="${PLIST_DIR}/${PLIST_NAME}.plist"

echo "======================================"
echo "  Clawpeteer Agent - macOS Installer"
echo "======================================"
echo

# --- Step 1: Build or locate binary ---
echo "[1/4] Preparing binary..."
if [ -f "${AGENT_DIR}/clawpeteer-agent" ]; then
    BINARY="${AGENT_DIR}/clawpeteer-agent"
    echo "  Found existing binary: ${BINARY}"
elif command -v go &> /dev/null; then
    echo "  Building from source..."
    cd "${AGENT_DIR}"
    go build -ldflags "-X main.Version=1.0.0" -o clawpeteer-agent .
    BINARY="${AGENT_DIR}/clawpeteer-agent"
    echo "  Built: ${BINARY}"
else
    echo "ERROR: No binary found and Go is not installed."
    echo "  Either build the binary first with 'make build' or install Go."
    exit 1
fi

echo "  Installing to ${INSTALL_DIR}/clawpeteer-agent..."
sudo cp "${BINARY}" "${INSTALL_DIR}/clawpeteer-agent"
sudo chmod 755 "${INSTALL_DIR}/clawpeteer-agent"
echo

# --- Step 2: Create config directory ---
echo "[2/4] Setting up configuration..."
mkdir -p "${CONFIG_DIR}"

if [ ! -f "${CONFIG_DIR}/config.json" ]; then
    if [ -f "${AGENT_DIR}/config.example.json" ]; then
        cp "${AGENT_DIR}/config.example.json" "${CONFIG_DIR}/config.json"
        echo "  Copied config.example.json -> ${CONFIG_DIR}/config.json"
        echo "  IMPORTANT: Edit ${CONFIG_DIR}/config.json with your broker details!"
    else
        echo "  WARNING: No config.example.json found. Create ${CONFIG_DIR}/config.json manually."
    fi
else
    echo "  Config already exists at ${CONFIG_DIR}/config.json (not overwriting)."
fi
echo

# --- Step 3: Install launchd plist ---
echo "[3/4] Installing launchd service..."
mkdir -p "${PLIST_DIR}"

cat > "${PLIST_FILE}" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>${PLIST_NAME}</string>
    <key>ProgramArguments</key>
    <array>
        <string>${INSTALL_DIR}/clawpeteer-agent</string>
        <string>--config</string>
        <string>${CONFIG_DIR}/config.json</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>${CONFIG_DIR}/logs/agent.log</string>
    <key>StandardErrorPath</key>
    <string>${CONFIG_DIR}/logs/agent-error.log</string>
</dict>
</plist>
PLIST

mkdir -p "${CONFIG_DIR}/logs"
echo "  Installed plist: ${PLIST_FILE}"
echo

# --- Step 4: Print next steps ---
echo "[4/4] Done!"
echo
echo "======================================"
echo "  Installation Complete!"
echo "======================================"
echo
echo "Next steps:"
echo "  1. Edit the config file with your broker details:"
echo "       nano ${CONFIG_DIR}/config.json"
echo
echo "  2. Load the agent service:"
echo "       launchctl load ${PLIST_FILE}"
echo
echo "  3. Check if the agent is running:"
echo "       launchctl list | grep clawpeteer"
echo
echo "  4. View logs:"
echo "       tail -f ${CONFIG_DIR}/logs/agent.log"
echo
echo "  5. To stop the agent:"
echo "       launchctl unload ${PLIST_FILE}"
echo
echo "  6. To uninstall:"
echo "       launchctl unload ${PLIST_FILE}"
echo "       rm ${PLIST_FILE}"
echo "       sudo rm ${INSTALL_DIR}/clawpeteer-agent"
echo
