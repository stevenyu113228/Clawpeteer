# Clawpeteer - MQTT Remote Control for OpenClaw

Clawpeteer is a remote control system that lets you execute commands, transfer files, and manage tasks on remote machines via MQTT. It consists of a Go-based agent that runs on target machines, a Node.js CLI for sending commands, and a Mosquitto MQTT broker that handles secure communication between them. Designed as an OpenClaw skill, Clawpeteer enables AI agents to operate across multiple computers through natural language intent mapping.

## Architecture

```
+-------------------+          +------------------+          +-------------------+
|                   |   MQTT   |                  |   MQTT   |                   |
|   CLI / OpenClaw  |--------->|   Mosquitto      |--------->|   Agent (Go)      |
|   (Node.js)       |<---------|   Broker (TLS)   |<---------|   home-pc         |
|                   |          |                  |          |                   |
+-------------------+          +------------------+          +-------------------+
                                       |    ^                         |
                                       |    |                +--------+--------+
                                       |    |                | Command Executor|
                                       v    |                | File Transfer   |
                               +-------------------+        | Task Manager    |
                               |   Agent (Go)      |        | Security Module |
                               |   office-server   |        +-----------------+
                               +-------------------+
```

**Key components:**
- **CLI** (`cli/`): Node.js command-line tool for sending commands and transferring files
- **Agent** (`agent/`): Go binary that runs on each remote machine, executes commands, handles file transfers
- **Broker** (`broker/`): Mosquitto MQTT broker config with TLS, ACL, and authentication
- **Skill** (`skill/`): OpenClaw SKILL.md for AI agent integration

## Quick Start

1. **Set up the MQTT broker** (see [docs/setup-broker.md](docs/setup-broker.md)):
   ```bash
   cd broker && sudo ./setup.sh
   ```

2. **Install the CLI**:
   ```bash
   cd cli && npm install && npm link
   ```

3. **Configure the CLI** (create `~/.clawpeteer/config.json`):
   ```json
   {
     "brokerUrl": "mqtt://localhost:1883",
     "username": "openclaw",
     "password": "your-password"
   }
   ```

4. **Install an agent on a remote machine** (see [docs/install-agent.md](docs/install-agent.md)):
   ```bash
   cd agent && make build
   ./agent/scripts/install-linux.sh   # or install-mac.sh
   ```

5. **Start using it**:
   ```bash
   clawpeteer list                           # see connected agents
   clawpeteer send home-pc "df -h"           # run a command
   clawpeteer upload home-pc ./f.txt /tmp/   # upload a file
   clawpeteer download home-pc /var/log/syslog ./logs/  # download a file
   ```

## Documentation

- [Broker Setup Guide](docs/setup-broker.md) - Install and configure Mosquitto
- [Agent Installation Guide](docs/install-agent.md) - Deploy agents on Linux and macOS
- [Implementation Plan](docs/plans/) - Design documents and implementation notes

## Project Structure

```
clawpeteer/
  agent/              Go agent (runs on remote machines)
    internal/           config, executor, filetransfer, handler, security, taskmanager
    scripts/            install-linux.sh, install-mac.sh
    systemd/            clawpeteer-agent.service
    Makefile            cross-platform build targets
  broker/             Mosquitto configuration
    mosquitto.conf      broker config with TLS
    acl                 per-user topic access control
    setup.sh            automated broker setup
  cli/                Node.js CLI
    bin/                clawpeteer.js entry point
    src/                commands (send, list, status, kill, upload, download) + mqtt client
  docs/               Documentation
  skill/              OpenClaw SKILL.md
  test/               Integration tests
```

## License

MIT
