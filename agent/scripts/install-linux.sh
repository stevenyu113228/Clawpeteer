#!/usr/bin/env bash
set -euo pipefail

# Clawpeteer Agent - Linux Installation Script
# This script installs the Clawpeteer agent as a systemd service.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
AGENT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/clawpeteer"
SERVICE_FILE="/etc/systemd/system/clawpeteer-agent.service"

echo "======================================"
echo "  Clawpeteer Agent - Linux Installer"
echo "======================================"
echo

# --- Step 1: Build or locate binary ---
echo "[1/5] Preparing binary..."
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
echo "[2/5] Setting up configuration..."
sudo mkdir -p "${CONFIG_DIR}"

if [ ! -f "${CONFIG_DIR}/config.json" ]; then
    if [ -f "${AGENT_DIR}/config.example.json" ]; then
        sudo cp "${AGENT_DIR}/config.example.json" "${CONFIG_DIR}/config.json"
        echo "  Copied config.example.json -> ${CONFIG_DIR}/config.json"
        echo "  IMPORTANT: Edit ${CONFIG_DIR}/config.json with your broker details!"
    else
        echo "  WARNING: No config.example.json found. Create ${CONFIG_DIR}/config.json manually."
    fi
else
    echo "  Config already exists at ${CONFIG_DIR}/config.json (not overwriting)."
fi
echo

# --- Step 3: Create clawpeteer user ---
echo "[3/5] Creating clawpeteer system user..."
if id "clawpeteer" &>/dev/null; then
    echo "  User 'clawpeteer' already exists."
else
    sudo useradd --system --no-create-home --shell /usr/sbin/nologin clawpeteer
    echo "  Created system user 'clawpeteer'."
fi

sudo chown clawpeteer:clawpeteer "${CONFIG_DIR}/config.json" 2>/dev/null || true
echo

# --- Step 4: Install systemd service ---
echo "[4/5] Installing systemd service..."
if [ -f "${AGENT_DIR}/systemd/clawpeteer-agent.service" ]; then
    sudo cp "${AGENT_DIR}/systemd/clawpeteer-agent.service" "${SERVICE_FILE}"
    sudo systemctl daemon-reload
    echo "  Installed service file: ${SERVICE_FILE}"
else
    echo "  WARNING: Service file not found at ${AGENT_DIR}/systemd/clawpeteer-agent.service"
fi
echo

# --- Step 5: Print next steps ---
echo "[5/5] Done!"
echo
echo "======================================"
echo "  Installation Complete!"
echo "======================================"
echo
echo "Next steps:"
echo "  1. Edit the config file with your broker details:"
echo "       sudo nano ${CONFIG_DIR}/config.json"
echo
echo "  2. Start the agent:"
echo "       sudo systemctl start clawpeteer-agent"
echo
echo "  3. Enable auto-start on boot:"
echo "       sudo systemctl enable clawpeteer-agent"
echo
echo "  4. Check status:"
echo "       sudo systemctl status clawpeteer-agent"
echo
echo "  5. View logs:"
echo "       journalctl -u clawpeteer-agent -f"
echo
