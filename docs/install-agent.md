# Agent Installation Guide

This guide covers installing the Clawpeteer agent on remote machines. The agent is a Go binary that connects to the MQTT broker, listens for commands, and executes them locally.

## 1. Get the Binary

### Option A: Download a pre-built binary

Download the appropriate binary for your platform from the releases page, or build all targets:

```bash
cd agent
make build-all
ls dist/
# clawpeteer-agent-linux-amd64
# clawpeteer-agent-linux-arm64
# clawpeteer-agent-darwin-amd64
# clawpeteer-agent-darwin-arm64
# clawpeteer-agent-windows-amd64.exe
```

### Option B: Build from source

Requires Go 1.24+:

```bash
cd agent
make build
# produces: ./clawpeteer-agent
```

## 2. Create config.json

Copy the example config and edit it with your broker details:

```bash
cp agent/config.example.json config.json
```

Edit `config.json`:

```json
{
  "agentId": "home-pc",
  "broker": {
    "url": "mqtts://mqtt.yourdomain.com:8883",
    "username": "home-pc",
    "password": "your-secure-password",
    "caFile": "/path/to/ca.crt"
  },
  "security": {
    "mode": "blacklist",
    "blacklist": ["rm -rf /", "mkfs", "dd if=/dev/zero", "chmod -R 777 /"],
    "uploadDirs": ["/home/user/uploads", "/tmp/mqtt-uploads"],
    "downloadDirs": ["/home/user", "/var/log"],
    "maxFileSize": 1073741824,
    "maxConcurrentTransfers": 3
  },
  "heartbeatInterval": 30,
  "logDir": "~/.clawpeteer/logs"
}
```

Key fields:
- **agentId**: Unique identifier for this machine (must match the MQTT username)
- **broker.url**: Your MQTT broker address (`mqtt://` for non-TLS, `mqtts://` for TLS)
- **broker.username/password**: MQTT credentials (created during broker setup)
- **broker.caFile**: Path to the CA certificate (for TLS connections)
- **security.mode**: `"blacklist"` (block specific commands) or `"whitelist"` (allow only specific commands)
- **security.uploadDirs**: Directories where files can be uploaded to
- **security.downloadDirs**: Directories where files can be downloaded from

## 3. Test Manually

Run the agent in the foreground to verify it connects:

```bash
./clawpeteer-agent --config config.json
```

You should see:
```
Clawpeteer Agent starting (id=home-pc)
Connected to MQTT broker: mqtts://mqtt.yourdomain.com:8883
Agent is running. Press Ctrl+C to stop.
```

From the CLI, verify the agent appears:
```bash
clawpeteer list
clawpeteer send home-pc "echo hello"
```

Press `Ctrl+C` to stop the agent once testing is complete.

## 4. Install as a Service

### Linux (systemd)

Use the provided install script:

```bash
cd agent
sudo ./scripts/install-linux.sh
```

This will:
1. Build or copy the binary to `/usr/local/bin/clawpeteer-agent`
2. Create `/etc/clawpeteer/` and copy the example config
3. Create a `clawpeteer` system user
4. Install the systemd service

Then start the service:
```bash
sudo systemctl start clawpeteer-agent
sudo systemctl enable clawpeteer-agent
```

Manage the service:
```bash
sudo systemctl status clawpeteer-agent    # check status
sudo systemctl restart clawpeteer-agent   # restart
journalctl -u clawpeteer-agent -f         # view logs
```

### macOS (launchd)

Use the provided install script:

```bash
cd agent
./scripts/install-mac.sh
```

This will:
1. Build or copy the binary to `/usr/local/bin/clawpeteer-agent`
2. Create `~/.clawpeteer/` and copy the example config
3. Install a launchd plist at `~/Library/LaunchAgents/com.clawpeteer.agent.plist`

Then load the service:
```bash
launchctl load ~/Library/LaunchAgents/com.clawpeteer.agent.plist
```

Manage the service:
```bash
launchctl list | grep clawpeteer                                     # check status
launchctl unload ~/Library/LaunchAgents/com.clawpeteer.agent.plist   # stop
launchctl load ~/Library/LaunchAgents/com.clawpeteer.agent.plist     # start
tail -f ~/.clawpeteer/logs/agent.log                                 # view logs
```

## 5. Verify

After installing and starting the service, verify everything works:

```bash
# From the CLI machine:
clawpeteer list
# Should show the agent as online

clawpeteer send home-pc "hostname"
# Should return the remote machine's hostname

clawpeteer status home-pc
# Should show agent status and recent tasks
```

Check the agent logs if anything is not working:
- **Linux**: `journalctl -u clawpeteer-agent -f`
- **macOS**: `tail -f ~/.clawpeteer/logs/agent.log`

Common issues:
- **Connection refused**: Check broker URL and port, ensure Mosquitto is running
- **Authentication failed**: Verify username/password match what was set during broker setup
- **TLS errors**: Ensure the CA certificate path is correct and the cert matches the broker
- **Command rejected**: Check the security config (whitelist/blacklist) in config.json
