# MQTT Remote Control - Validated Design

**Date:** 2026-02-22
**Status:** Approved

## Architecture (Revised)

Three-layer architecture aligned with OpenClaw's actual Skill spec:

```
OpenClaw Agent ─(exec)─> clawpeteer CLI ─(MQTT)─> Remote Agent (Go)
```

### 1. OpenClaw Skill (`skill/SKILL.md`)
- Pure markdown with YAML frontmatter
- Teaches the agent how to invoke `clawpeteer` CLI commands
- No programmatic API — leverages agent's native LLM for intent understanding

### 2. CLI Tool (`cli/` — Node.js npm package)
- Unified entry point: `clawpeteer <subcommand>`
- Subcommands: `send`, `upload`, `download`, `list`, `status`, `kill`
- Handles MQTT connection, file chunking, task tracking on control side

### 3. Remote Agent (`agent/` — Go)
- Single compiled binary, zero runtime dependencies
- Cross-compiled for Windows/macOS/Linux
- Subscribes to MQTT commands, executes locally
- Manages tasks, file transfers, heartbeat

### 4. MQTT Broker (`broker/` — Mosquitto)
- Self-hosted with TLS (port 8883)
- User authentication + ACL
- QoS 1, persistence, Last Will

## What Changed From Original Plan

| Component | Original | Revised |
|-----------|----------|---------|
| Skill interface | Node.js class (canHandle/handle) | SKILL.md (pure text) |
| NLU parsing | Regex in JS | Agent LLM native capability |
| CLI naming | Separate commands (mqtt-send etc.) | Unified `clawpeteer` subcommands |
| Remote Agent | Node.js + pkg | **Go native binary** |

## What Stays The Same

- MQTT Topic architecture (agents/{id}/commands, results, stream, etc.)
- Message format specs (all JSON payloads)
- Security design (ACL, command whitelist/blacklist, path validation)
- File transfer protocol (chunking, SHA-256, resume)
- Heartbeat / Last Will mechanism
- Broker TLS + auth configuration

## Project Structure

```
clawpeteer/
├── skill/SKILL.md
├── cli/                    (Node.js)
│   ├── package.json
│   ├── bin/clawpeteer.js
│   └── src/
│       ├── mqtt-client.js
│       ├── file-transfer.js
│       ├── task-manager.js
│       └── commands/{send,upload,download,list,status,kill}.js
├── agent/                  (Go)
│   ├── go.mod
│   ├── main.go
│   └── internal/{executor,filetransfer,security,taskmanager}/
├── broker/
│   ├── mosquitto.conf
│   ├── acl
│   └── setup.sh
└── docs/
```

## Scope

Full implementation (phases 1-6 from original plan), with revised tech stack.
