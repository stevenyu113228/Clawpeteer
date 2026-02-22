---
name: clawpeteer
description: Control remote computers via MQTT - execute commands, transfer files, and manage tasks on remote agents
metadata: {"openclaw":{"requires":{"bins":["clawpeteer"]},"emoji":"🦞"}}
---

# Clawpeteer - MQTT Remote Control

Control remote computers via MQTT. Execute commands, upload/download files, and manage tasks.

## Prerequisites

1. MQTT Broker (Mosquitto) must be running with TLS and ACL configured
2. Remote agents must be installed and connected on target machines
3. CLI config at `~/.clawpeteer/config.json` with broker credentials

## Commands

### Execute a command on a remote machine

```bash
clawpeteer send <agent-id> "<command>" [--stream] [--background] [--timeout <ms>]
```

Examples:
- `clawpeteer send home-pc "ls -la /home"`
- `clawpeteer send server "apt-get update" --stream`
- `clawpeteer send office "npm install" --background`
- `clawpeteer send home-pc "df -h"` (check disk space)
- `clawpeteer send server "free -h"` (check memory)

### Upload a file to a remote machine

```bash
clawpeteer upload <agent-id> <local-path> <remote-path>
```

Examples:
- `clawpeteer upload home-pc ./backup.tar.gz /home/user/backups/backup.tar.gz`
- `clawpeteer upload server ./config.json /etc/app/config.json`

### Download a file from a remote machine

```bash
clawpeteer download <agent-id> <remote-path> [local-path]
```

Examples:
- `clawpeteer download server /var/log/app.log ./logs/`
- `clawpeteer download home-pc /home/user/data.db ./backups/data.db`

### List connected agents

```bash
clawpeteer list
```

### Check agent/task status

```bash
clawpeteer status <agent-id> [task-id]
```

### Kill a running task

```bash
clawpeteer kill <agent-id> <task-id>
```

## User Intent Mapping

When the user says things like:
- "check disk space on home-pc" -> `clawpeteer send home-pc "df -h"`
- "what processes are running on server" -> `clawpeteer send server "ps aux"`
- "upload the backup to home-pc" -> `clawpeteer upload home-pc <path> <dest>`
- "download logs from server" -> `clawpeteer download server /var/log/app.log ./`
- "list all agents" / "show connected machines" -> `clawpeteer list`
- "stop task X" / "kill task X" -> `clawpeteer kill <agent> <task-id>`
- "run X in background on Y" -> `clawpeteer send Y "X" --background`

## Notes

- Use `--stream` for long-running commands to see output in real-time
- Use `--background` for commands you don't need to wait for
- File transfers use chunked transfer with SHA-256 verification
- Agents report heartbeat every 30 seconds; offline agents are marked automatically
