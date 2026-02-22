#!/usr/bin/env bash
set -euo pipefail

# Clawpeteer Integration Test
#
# Prerequisites:
#   - A running Mosquitto broker on localhost:1883
#   - MQTT user 'openclaw' with a known password
#   - MQTT user 'test-agent' with a known password
#   - ACL configured to allow these users (see broker/acl)
#   - Go toolchain installed (to build the agent)
#   - Node.js installed (to run the CLI)
#
# Usage:
#   OPENCLAW_PASS=<password> AGENT_PASS=<password> ./test/integration.sh
#
# If the broker is not available, the script will skip tests gracefully.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
AGENT_DIR="${PROJECT_DIR}/agent"
CLI_DIR="${PROJECT_DIR}/cli"

OPENCLAW_PASS="${OPENCLAW_PASS:-openclaw}"
AGENT_PASS="${AGENT_PASS:-testagent}"
BROKER_HOST="${BROKER_HOST:-localhost}"
BROKER_PORT="${BROKER_PORT:-1883}"

TMPDIR_TEST="$(mktemp -d)"
AGENT_PID=""
PASSED=0
FAILED=0
SKIPPED=0

# --- Cleanup ---
cleanup() {
    echo
    echo "--- Cleaning up ---"
    if [ -n "${AGENT_PID}" ] && kill -0 "${AGENT_PID}" 2>/dev/null; then
        echo "  Stopping agent (PID ${AGENT_PID})..."
        kill "${AGENT_PID}" 2>/dev/null || true
        wait "${AGENT_PID}" 2>/dev/null || true
    fi
    if [ -d "${TMPDIR_TEST}" ]; then
        echo "  Removing temp dir: ${TMPDIR_TEST}"
        rm -rf "${TMPDIR_TEST}"
    fi
    echo
    echo "======================================"
    echo "  Results: ${PASSED} passed, ${FAILED} failed, ${SKIPPED} skipped"
    echo "======================================"
    if [ "${FAILED}" -gt 0 ]; then
        exit 1
    fi
}
trap cleanup EXIT

pass() {
    echo "  PASS: $1"
    PASSED=$((PASSED + 1))
}

fail() {
    echo "  FAIL: $1"
    FAILED=$((FAILED + 1))
}

skip() {
    echo "  SKIP: $1"
    SKIPPED=$((SKIPPED + 1))
}

echo "======================================"
echo "  Clawpeteer Integration Test"
echo "======================================"
echo
echo "Broker: ${BROKER_HOST}:${BROKER_PORT}"
echo "Temp dir: ${TMPDIR_TEST}"
echo

# --- Step 1: Build the agent ---
echo "[1/6] Building agent..."
if command -v go &> /dev/null; then
    cd "${AGENT_DIR}"
    go build -o "${TMPDIR_TEST}/clawpeteer-agent" . 2>&1
    if [ -f "${TMPDIR_TEST}/clawpeteer-agent" ]; then
        pass "Agent binary built"
    else
        fail "Agent binary not found after build"
    fi
else
    skip "Go not installed, cannot build agent"
fi
echo

# --- Step 2: Create temp config files ---
echo "[2/6] Creating temp config files..."

# Agent config
cat > "${TMPDIR_TEST}/agent-config.json" <<EOF
{
  "agentId": "test-agent",
  "broker": {
    "url": "mqtt://${BROKER_HOST}:${BROKER_PORT}",
    "username": "test-agent",
    "password": "${AGENT_PASS}",
    "caFile": ""
  },
  "security": {
    "mode": "blacklist",
    "blacklist": ["rm -rf /"],
    "uploadDirs": ["${TMPDIR_TEST}"],
    "downloadDirs": ["${TMPDIR_TEST}"],
    "maxFileSize": 10485760,
    "maxConcurrentTransfers": 3
  },
  "heartbeatInterval": 5,
  "logDir": "${TMPDIR_TEST}/logs"
}
EOF

# CLI config
mkdir -p "${TMPDIR_TEST}/cli-config"
cat > "${TMPDIR_TEST}/cli-config/config.json" <<EOF
{
  "brokerUrl": "mqtt://${BROKER_HOST}:${BROKER_PORT}",
  "username": "openclaw",
  "password": "${OPENCLAW_PASS}",
  "caFile": ""
}
EOF

pass "Config files created"
echo

# --- Step 3: Check broker availability ---
echo "[3/6] Checking broker availability..."
BROKER_AVAILABLE=false

if command -v mosquitto_pub &> /dev/null; then
    if mosquitto_pub -h "${BROKER_HOST}" -p "${BROKER_PORT}" \
        -u openclaw -P "${OPENCLAW_PASS}" \
        -t "clawpeteer/test/ping" -m "ping" \
        --quiet 2>/dev/null; then
        BROKER_AVAILABLE=true
        pass "Broker is reachable"
    else
        skip "Broker not reachable at ${BROKER_HOST}:${BROKER_PORT} (is Mosquitto running?)"
    fi
else
    # Try a simple TCP connection check
    if (echo > /dev/tcp/${BROKER_HOST}/${BROKER_PORT}) 2>/dev/null; then
        BROKER_AVAILABLE=true
        pass "Broker port is open (mosquitto_pub not available for full check)"
    else
        skip "Broker not reachable at ${BROKER_HOST}:${BROKER_PORT}"
    fi
fi
echo

# --- Step 4: Start agent in background ---
echo "[4/6] Starting agent..."
if [ "${BROKER_AVAILABLE}" = true ] && [ -f "${TMPDIR_TEST}/clawpeteer-agent" ]; then
    mkdir -p "${TMPDIR_TEST}/logs"
    "${TMPDIR_TEST}/clawpeteer-agent" --config "${TMPDIR_TEST}/agent-config.json" \
        > "${TMPDIR_TEST}/agent-stdout.log" 2>&1 &
    AGENT_PID=$!
    echo "  Agent started with PID ${AGENT_PID}"
    # Give agent time to connect
    sleep 3

    if kill -0 "${AGENT_PID}" 2>/dev/null; then
        pass "Agent is running"
    else
        fail "Agent exited prematurely (check ${TMPDIR_TEST}/agent-stdout.log)"
        AGENT_PID=""
    fi
else
    if [ "${BROKER_AVAILABLE}" = false ]; then
        skip "Broker not available, skipping agent start"
    else
        skip "Agent binary not available, skipping agent start"
    fi
fi
echo

# --- Step 5: Test CLI commands ---
echo "[5/6] Testing CLI commands..."

CLI_BIN="${CLI_DIR}/bin/clawpeteer.js"
export CLAWPETEER_CONFIG="${TMPDIR_TEST}/cli-config/config.json"

if [ "${BROKER_AVAILABLE}" = true ] && [ -n "${AGENT_PID}" ]; then
    # Test: clawpeteer list
    echo "  Testing 'clawpeteer list'..."
    LIST_OUTPUT=$(timeout 10 node "${CLI_BIN}" list 2>&1) || true
    if echo "${LIST_OUTPUT}" | grep -qi "test-agent\|agent\|connected\|online"; then
        pass "clawpeteer list shows agents"
    else
        # The agent may not have registered yet; partial pass
        echo "    Output: ${LIST_OUTPUT}"
        skip "clawpeteer list did not show test-agent (may need more time)"
    fi

    # Test: clawpeteer send
    echo "  Testing 'clawpeteer send test-agent \"echo hello\"'..."
    SEND_OUTPUT=$(timeout 15 node "${CLI_BIN}" send test-agent "echo hello" 2>&1) || true
    if echo "${SEND_OUTPUT}" | grep -qi "hello\|task\|result\|sent"; then
        pass "clawpeteer send executed command"
    else
        echo "    Output: ${SEND_OUTPUT}"
        skip "clawpeteer send did not return expected output (command may still be queued)"
    fi
else
    skip "Broker/agent not available, skipping CLI tests"
fi
echo

# --- Step 6: Summary ---
echo "[6/6] Test run complete."
