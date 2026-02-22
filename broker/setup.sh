#!/usr/bin/env bash
set -euo pipefail

# Clawpeteer Mosquitto Broker Setup Script
# This script configures a local Mosquitto MQTT broker with TLS and ACL.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MOSQUITTO_CONF_DIR="/etc/mosquitto"
MOSQUITTO_CERTS_DIR="${MOSQUITTO_CONF_DIR}/certs"
MOSQUITTO_DATA_DIR="/var/lib/mosquitto"
MOSQUITTO_LOG_DIR="/var/log/mosquitto"

echo "======================================"
echo "  Clawpeteer MQTT Broker Setup"
echo "======================================"
echo

# --- Step 1: Check if Mosquitto is installed ---
echo "[1/6] Checking if Mosquitto is installed..."
if ! command -v mosquitto &> /dev/null; then
    echo "ERROR: mosquitto is not installed."
    echo "  Install it with:"
    echo "    macOS:  brew install mosquitto"
    echo "    Debian: sudo apt-get install mosquitto mosquitto-clients"
    echo "    RHEL:   sudo yum install mosquitto"
    exit 1
fi

if ! command -v mosquitto_passwd &> /dev/null; then
    echo "ERROR: mosquitto_passwd is not found."
    echo "  It should be included with the mosquitto package."
    exit 1
fi

echo "  Mosquitto found: $(mosquitto -h 2>&1 | head -1 || true)"
echo

# --- Step 2: Create required directories ---
echo "[2/6] Creating required directories..."
sudo mkdir -p "${MOSQUITTO_CERTS_DIR}"
sudo mkdir -p "${MOSQUITTO_DATA_DIR}"
sudo mkdir -p "${MOSQUITTO_LOG_DIR}"
echo "  Created ${MOSQUITTO_CERTS_DIR}"
echo "  Created ${MOSQUITTO_DATA_DIR}"
echo "  Created ${MOSQUITTO_LOG_DIR}"
echo

# --- Step 3: Copy configuration files ---
echo "[3/6] Copying configuration files..."
sudo cp "${SCRIPT_DIR}/mosquitto.conf" "${MOSQUITTO_CONF_DIR}/mosquitto.conf"
sudo cp "${SCRIPT_DIR}/acl" "${MOSQUITTO_CONF_DIR}/acl"
echo "  Copied mosquitto.conf -> ${MOSQUITTO_CONF_DIR}/mosquitto.conf"
echo "  Copied acl            -> ${MOSQUITTO_CONF_DIR}/acl"
echo

# --- Step 4: Create MQTT users ---
echo "[4/6] Creating MQTT users..."
PASSWD_FILE="${MOSQUITTO_CONF_DIR}/passwd"

echo "  Creating 'openclaw' user (the controller)..."
echo "  Please enter a password for the 'openclaw' user:"
sudo mosquitto_passwd -c "${PASSWD_FILE}" openclaw

echo
echo "  Creating 'agent' user (default agent identity)..."
echo "  Please enter a password for the 'agent' user:"
sudo mosquitto_passwd "${PASSWD_FILE}" agent

echo "  Users created in ${PASSWD_FILE}"
echo

# --- Step 5: Generate self-signed TLS certificates ---
echo "[5/6] Generating self-signed TLS certificates for development..."

# CA key and certificate
sudo openssl genrsa -out "${MOSQUITTO_CERTS_DIR}/ca.key" 2048
sudo openssl req -new -x509 -days 365 -key "${MOSQUITTO_CERTS_DIR}/ca.key" \
    -out "${MOSQUITTO_CERTS_DIR}/ca.crt" \
    -subj "/C=US/ST=Dev/L=Local/O=Clawpeteer/CN=Clawpeteer-CA"

# Server key and certificate signing request
sudo openssl genrsa -out "${MOSQUITTO_CERTS_DIR}/server.key" 2048
sudo openssl req -new -key "${MOSQUITTO_CERTS_DIR}/server.key" \
    -out "${MOSQUITTO_CERTS_DIR}/server.csr" \
    -subj "/C=US/ST=Dev/L=Local/O=Clawpeteer/CN=localhost"

# Sign the server certificate with the CA
sudo openssl x509 -req -days 365 \
    -in "${MOSQUITTO_CERTS_DIR}/server.csr" \
    -CA "${MOSQUITTO_CERTS_DIR}/ca.crt" \
    -CAkey "${MOSQUITTO_CERTS_DIR}/ca.key" \
    -CAcreateserial \
    -out "${MOSQUITTO_CERTS_DIR}/server.crt"

# Clean up the CSR
sudo rm -f "${MOSQUITTO_CERTS_DIR}/server.csr"

echo "  Generated CA cert:     ${MOSQUITTO_CERTS_DIR}/ca.crt"
echo "  Generated server cert: ${MOSQUITTO_CERTS_DIR}/server.crt"
echo "  Generated server key:  ${MOSQUITTO_CERTS_DIR}/server.key"
echo

# --- Step 6: Set permissions ---
echo "[6/6] Setting permissions..."
sudo chown -R mosquitto:mosquitto "${MOSQUITTO_DATA_DIR}" 2>/dev/null || \
    echo "  WARNING: Could not set ownership to mosquitto user (user may not exist yet)."
sudo chown -R mosquitto:mosquitto "${MOSQUITTO_LOG_DIR}" 2>/dev/null || \
    echo "  WARNING: Could not set ownership to mosquitto user (user may not exist yet)."
sudo chmod 600 "${MOSQUITTO_CERTS_DIR}/server.key"
sudo chmod 600 "${MOSQUITTO_CERTS_DIR}/ca.key"
sudo chmod 644 "${MOSQUITTO_CERTS_DIR}/server.crt"
sudo chmod 644 "${MOSQUITTO_CERTS_DIR}/ca.crt"
sudo chmod 600 "${PASSWD_FILE}"
echo "  Permissions set."
echo

# --- Done ---
echo "======================================"
echo "  Setup Complete!"
echo "======================================"
echo
echo "Next steps:"
echo "  1. Start the broker:"
echo "       mosquitto -c ${MOSQUITTO_CONF_DIR}/mosquitto.conf"
echo "     or (if using systemd):"
echo "       sudo systemctl restart mosquitto"
echo
echo "  2. Test the non-TLS local connection:"
echo "       mosquitto_sub -h 127.0.0.1 -p 1883 -u openclaw -P <password> -t 'agents/#'"
echo
echo "  3. Test the TLS connection:"
echo "       mosquitto_sub -h localhost -p 8883 --cafile ${MOSQUITTO_CERTS_DIR}/ca.crt -u openclaw -P <password> -t 'agents/#'"
echo
echo "  4. To add more agent users:"
echo "       sudo mosquitto_passwd ${PASSWD_FILE} <agent-name>"
echo
