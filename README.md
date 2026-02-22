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

### One-Line Deploy (OpenClaw machines)

Package everything into a zip and deploy to any machine with OpenClaw installed:

```bash
# Build the installer package
./pack-installer.sh

# Deploy to target machine
scp build/clawpeteer-install.zip user@host:~/
ssh user@host 'unzip clawpeteer-install.zip && cd clawpeteer-install && ./install.sh \
  --broker-url mqtts://your.broker.host:8883 \
  --username my-agent \
  --password my-secret'
```

The installer automatically sets up the CLI, MQTT config, CA certificate, and OpenClaw skill.

### Manual Setup

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
     "brokerUrl": "mqtts://your.broker.host:8883",
     "username": "my-agent",
     "password": "my-secret",
     "clientId": "my-client-id",
     "caFile": "/path/to/ca.crt"
   }
   ```
   You can also pass `--config <path>` before any subcommand:
   ```bash
   clawpeteer --config /path/to/config.json list
   ```

4. **Build and run an agent on a remote machine** (see [docs/install-agent.md](docs/install-agent.md)):
   ```bash
   cd agent && go build -o clawpeteer-agent .
   ./clawpeteer-agent --id home-pc --broker mqtts://your.broker.host:8883 --user home-pc --pass my-secret
   ```

5. **Start using it**:
   ```bash
   clawpeteer list                           # see connected agents
   clawpeteer send home-pc "df -h"           # run a command
   clawpeteer upload home-pc ./f.txt /tmp/   # upload a file
   clawpeteer download home-pc /var/log/syslog ./logs/  # download a file
   ```

## Agent Configuration

The agent loads config from multiple sources, with the following priority (highest first):

| Priority | Method | Description |
|----------|--------|-------------|
| 1 | CLI flags | `--id`, `--broker`, `--user`, `--pass`, `--ca` |
| 2 | Config file | `--config path/to/config.json` |
| 3 | Embedded config | Baked into binary at build time |
| 4 | `./config.json` | Config file in current directory |

### CLI Flags

```bash
clawpeteer-agent \
  --id home-pc \
  --broker mqtts://your.broker.host:8883 \
  --user home-pc \
  --pass my-secret \
  --ca /path/to/ca.crt
```

### Zero-Config Build (embed everything)

Embed both config and CA certificate into the binary for single-file deployment:

```bash
cp your-config.json agent/buildcfg/config.json
cp your-ca.crt agent/certs/ca.crt
cd agent && go build -o clawpeteer-agent .
```

The resulting binary runs with no external files:

```bash
./clawpeteer-agent                          # uses embedded config + CA
./clawpeteer-agent --id override-name       # override specific fields
```

### Cross-compile

```bash
cd agent

# Windows
GOOS=windows GOARCH=amd64 go build -o clawpeteer-agent.exe .

# Linux
GOOS=linux GOARCH=amd64 go build -o clawpeteer-agent-linux .
```

## Documentation

- [Broker Setup Guide](docs/setup-broker.md) - Install and configure Mosquitto
- [Agent Installation Guide](docs/install-agent.md) - Deploy agents on Linux and macOS
- [Implementation Plan](docs/plans/) - Design documents and implementation notes

## Project Structure

```
clawpeteer/
  agent/              Go agent (runs on remote machines)
    buildcfg/           embedded config.json (build-time)
    certs/              embedded CA certificate (build-time)
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
  install/            Installer script for one-line deployment
  docs/               Documentation
  skill/              OpenClaw SKILL.md
  test/               Integration tests
  pack-installer.sh   Build installer zip package
```

## License

MIT
