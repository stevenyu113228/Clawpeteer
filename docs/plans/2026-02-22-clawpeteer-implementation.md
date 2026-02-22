# Clawpeteer Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a cross-platform MQTT-based remote control system with CLI + Go agent, integrated as an OpenClaw Skill.

**Architecture:** Three-layer system — OpenClaw Skill (SKILL.md teaches agent to use CLI) → clawpeteer CLI (Node.js, handles MQTT control-side) → Remote Agent (Go binary, executes commands on target machines). Mosquitto broker with TLS+ACL sits in the middle.

**Tech Stack:** Go 1.21+ (agent), Node.js 20+ (CLI), Mosquitto (broker), paho.mqtt.golang (Go MQTT), mqtt.js (Node MQTT), commander.js (CLI framework)

---

## Task 1: Project Scaffolding

**Files:**
- Create: `cli/package.json`
- Create: `cli/bin/clawpeteer.js`
- Create: `cli/src/index.js`
- Create: `agent/go.mod`
- Create: `agent/main.go`
- Create: `.gitignore`

**Step 1: Initialize git repo**

Run: `cd /Users/steven/AI_Playground/Clawpeteer && git init`
Expected: Initialized empty Git repository

**Step 2: Create .gitignore**

```
node_modules/
dist/
*.exe
.env
config.json
cli/config.json
agent/config.json
*.log
.DS_Store
```

**Step 3: Create CLI package.json**

```json
{
  "name": "clawpeteer",
  "version": "1.0.0",
  "description": "MQTT-based remote control CLI for OpenClaw",
  "bin": {
    "clawpeteer": "./bin/clawpeteer.js"
  },
  "main": "src/index.js",
  "scripts": {
    "test": "vitest run",
    "test:watch": "vitest"
  },
  "dependencies": {
    "commander": "^13.0.0",
    "mqtt": "^5.0.0",
    "chalk": "^5.3.0",
    "uuid": "^11.0.0"
  },
  "devDependencies": {
    "vitest": "^3.0.0"
  },
  "author": "Steven Meow",
  "license": "MIT"
}
```

**Step 4: Create CLI entry point**

`cli/bin/clawpeteer.js`:
```javascript
#!/usr/bin/env node
import { program } from 'commander';

program
  .name('clawpeteer')
  .description('MQTT remote control CLI for OpenClaw')
  .version('1.0.0');

program.parse();
```

Add `"type": "module"` to package.json.

**Step 5: Create Go agent module**

Run: `cd /Users/steven/AI_Playground/Clawpeteer/agent && go mod init github.com/stevenmeow/clawpeteer-agent`

**Step 6: Create Go agent main.go stub**

`agent/main.go`:
```go
package main

import "fmt"

func main() {
	fmt.Println("Clawpeteer Agent starting...")
}
```

**Step 7: Create broker directory**

Run: `mkdir -p broker`

**Step 8: Install CLI dependencies**

Run: `cd /Users/steven/AI_Playground/Clawpeteer/cli && npm install`

**Step 9: Verify CLI runs**

Run: `cd /Users/steven/AI_Playground/Clawpeteer/cli && node bin/clawpeteer.js --help`
Expected: Shows help with version 1.0.0

**Step 10: Verify Go builds**

Run: `cd /Users/steven/AI_Playground/Clawpeteer/agent && go build -o /dev/null .`
Expected: Builds without errors

**Step 11: Commit**

```bash
git add -A
git commit -m "feat: initial project scaffolding with CLI and Go agent stubs"
```

---

## Task 2: Broker Configuration

**Files:**
- Create: `broker/mosquitto.conf`
- Create: `broker/acl`
- Create: `broker/setup.sh`

**Step 1: Create Mosquitto config**

`broker/mosquitto.conf`:
```conf
# Clawpeteer MQTT Broker Configuration

# TLS listener
listener 8883
certfile /etc/mosquitto/certs/server.crt
keyfile /etc/mosquitto/certs/server.key
cafile /etc/mosquitto/certs/ca.crt
require_certificate false

# Non-TLS listener for local development
listener 1883 127.0.0.1

# Authentication
allow_anonymous false
password_file /etc/mosquitto/passwd

# ACL
acl_file /etc/mosquitto/acl

# Persistence
persistence true
persistence_location /var/lib/mosquitto/

# Logging
log_dest file /var/log/mosquitto/mosquitto.log
log_type all

# Connection settings
max_keepalive 60
```

**Step 2: Create ACL file**

`broker/acl`:
```conf
# OpenClaw controller - full access to all agent topics
user openclaw
topic readwrite agents/#

# Pattern-based ACL for agents
# Each agent can only access its own topics
pattern read agents/%u/commands
pattern read agents/%u/control/#
pattern write agents/%u/results
pattern write agents/%u/stream/#
pattern write agents/%u/heartbeat
pattern read agents/%u/files/upload/#
pattern write agents/%u/files/download/#
pattern write agents/%u/files/status
pattern write agents/registry
pattern read agents/broadcast
```

**Step 3: Create setup script**

`broker/setup.sh`:
```bash
#!/bin/bash
set -euo pipefail

echo "=== Clawpeteer Broker Setup ==="

# Check if mosquitto is installed
if ! command -v mosquitto &> /dev/null; then
    echo "Mosquitto not found. Install it first:"
    echo "  macOS:  brew install mosquitto"
    echo "  Ubuntu: sudo apt install mosquitto mosquitto-clients"
    exit 1
fi

MQTT_DIR="/etc/mosquitto"
CERT_DIR="$MQTT_DIR/certs"

# Create directories
sudo mkdir -p "$CERT_DIR"
sudo mkdir -p /var/lib/mosquitto
sudo mkdir -p /var/log/mosquitto

# Copy config files
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
sudo cp "$SCRIPT_DIR/mosquitto.conf" "$MQTT_DIR/mosquitto.conf"
sudo cp "$SCRIPT_DIR/acl" "$MQTT_DIR/acl"

# Create password file
echo "Creating MQTT users..."
echo -n "Enter password for 'openclaw' user: "
read -s OC_PASS
echo
sudo mosquitto_passwd -c "$MQTT_DIR/passwd" openclaw <<< "$OC_PASS"

echo -n "Enter agent name (e.g., home-pc): "
read AGENT_NAME
echo -n "Enter password for '$AGENT_NAME': "
read -s AGENT_PASS
echo
sudo mosquitto_passwd "$MQTT_DIR/passwd" "$AGENT_NAME" <<< "$AGENT_PASS"

# Generate self-signed certs for development
echo "Generating self-signed TLS certificates..."
sudo openssl req -new -x509 -days 365 -extensions v3_ca \
    -keyout "$CERT_DIR/ca.key" -out "$CERT_DIR/ca.crt" \
    -subj "/CN=Clawpeteer CA" -nodes

sudo openssl req -new -nodes \
    -keyout "$CERT_DIR/server.key" -out "$CERT_DIR/server.csr" \
    -subj "/CN=localhost"

sudo openssl x509 -req -in "$CERT_DIR/server.csr" \
    -CA "$CERT_DIR/ca.crt" -CAkey "$CERT_DIR/ca.key" \
    -CAcreateserial -out "$CERT_DIR/server.crt" -days 365

sudo rm "$CERT_DIR/server.csr"

# Set permissions
sudo chown -R mosquitto:mosquitto "$MQTT_DIR"
sudo chmod 600 "$MQTT_DIR/passwd"

echo ""
echo "=== Setup Complete ==="
echo "Start broker: mosquitto -c /etc/mosquitto/mosquitto.conf"
echo "Test connection: mosquitto_sub -h localhost -p 1883 -u openclaw -P <pass> -t '#'"
```

**Step 4: Make setup script executable**

Run: `chmod +x /Users/steven/AI_Playground/Clawpeteer/broker/setup.sh`

**Step 5: Commit**

```bash
git add broker/
git commit -m "feat: add Mosquitto broker configuration with TLS and ACL"
```

---

## Task 3: Go Agent — Config + MQTT Connection

**Files:**
- Create: `agent/internal/config/config.go`
- Create: `agent/config.example.json`
- Modify: `agent/main.go`
- Create: `agent/internal/config/config_test.go`

**Step 1: Write config test**

`agent/internal/config/config_test.go`:
```go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	data := []byte(`{
		"agentId": "test-pc",
		"broker": {
			"url": "tcp://localhost:1883",
			"username": "test-pc",
			"password": "testpass"
		},
		"security": {
			"mode": "blacklist",
			"blacklist": ["rm -rf /"]
		}
	}`)
	os.WriteFile(cfgPath, data, 0644)

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.AgentID != "test-pc" {
		t.Errorf("AgentID = %q, want %q", cfg.AgentID, "test-pc")
	}
	if cfg.Broker.URL != "tcp://localhost:1883" {
		t.Errorf("Broker.URL = %q, want %q", cfg.Broker.URL, "tcp://localhost:1883")
	}
	if cfg.Security.Mode != "blacklist" {
		t.Errorf("Security.Mode = %q, want %q", cfg.Security.Mode, "blacklist")
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	data := []byte(`{
		"agentId": "test-pc",
		"broker": {
			"url": "tcp://localhost:1883",
			"username": "test-pc",
			"password": "testpass"
		}
	}`)
	os.WriteFile(cfgPath, data, 0644)

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.HeartbeatInterval != 30 {
		t.Errorf("HeartbeatInterval = %d, want 30", cfg.HeartbeatInterval)
	}
	if cfg.Security.Mode != "blacklist" {
		t.Errorf("default Security.Mode = %q, want %q", cfg.Security.Mode, "blacklist")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/steven/AI_Playground/Clawpeteer/agent && go test ./internal/config/ -v`
Expected: FAIL (package doesn't exist yet)

**Step 3: Implement config**

`agent/internal/config/config.go`:
```go
package config

import (
	"encoding/json"
	"fmt"
	"os"
)

type BrokerConfig struct {
	URL      string `json:"url"`
	Username string `json:"username"`
	Password string `json:"password"`
	CAFile   string `json:"caFile,omitempty"`
}

type SecurityConfig struct {
	Mode            string   `json:"mode"`
	Whitelist       []string `json:"whitelist,omitempty"`
	Blacklist       []string `json:"blacklist,omitempty"`
	UploadDirs      []string `json:"uploadDirs,omitempty"`
	DownloadDirs    []string `json:"downloadDirs,omitempty"`
	MaxFileSize     int64    `json:"maxFileSize,omitempty"`
	MaxConcTransfer int      `json:"maxConcurrentTransfers,omitempty"`
}

type Config struct {
	AgentID           string         `json:"agentId"`
	Broker            BrokerConfig   `json:"broker"`
	Security          SecurityConfig `json:"security"`
	HeartbeatInterval int            `json:"heartbeatInterval,omitempty"`
	LogDir            string         `json:"logDir,omitempty"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	cfg := &Config{
		HeartbeatInterval: 30,
		Security: SecurityConfig{
			Mode:            "blacklist",
			MaxFileSize:     1073741824, // 1GB
			MaxConcTransfer: 3,
		},
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if cfg.AgentID == "" {
		return nil, fmt.Errorf("agentId is required")
	}
	if cfg.Broker.URL == "" {
		return nil, fmt.Errorf("broker.url is required")
	}

	return cfg, nil
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/steven/AI_Playground/Clawpeteer/agent && go test ./internal/config/ -v`
Expected: PASS

**Step 5: Create config example**

`agent/config.example.json`:
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

**Step 6: Install Go MQTT dependency**

Run: `cd /Users/steven/AI_Playground/Clawpeteer/agent && go get github.com/eclipse/paho.mqtt.golang && go get github.com/google/uuid`

**Step 7: Update main.go with MQTT connection**

`agent/main.go`:
```go
package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/stevenmeow/clawpeteer-agent/internal/config"
)

func main() {
	cfgPath := flag.String("config", "config.json", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	log.Printf("Clawpeteer Agent [%s] starting...", cfg.AgentID)

	client, err := connectMQTT(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to MQTT broker: %v", err)
	}
	defer client.Disconnect(1000)

	log.Printf("Connected to MQTT broker: %s", cfg.Broker.URL)

	// Wait for shutdown signal
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig

	log.Println("Shutting down...")
}

func connectMQTT(cfg *config.Config) (mqtt.Client, error) {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(cfg.Broker.URL)
	opts.SetClientID(fmt.Sprintf("clawpeteer-agent-%s", cfg.AgentID))
	opts.SetUsername(cfg.Broker.Username)
	opts.SetPassword(cfg.Broker.Password)
	opts.SetKeepAlive(time.Duration(cfg.HeartbeatInterval) * time.Second)
	opts.SetAutoReconnect(true)
	opts.SetMaxReconnectInterval(5 * time.Minute)
	opts.SetCleanSession(false)

	// Last Will - notify offline status
	willPayload := fmt.Sprintf(`{"status":"offline","timestamp":%d}`, time.Now().UnixMilli())
	opts.SetWill(fmt.Sprintf("agents/%s/heartbeat", cfg.AgentID), willPayload, 1, true)

	// TLS config
	if cfg.Broker.CAFile != "" {
		tlsConfig, err := newTLSConfig(cfg.Broker.CAFile)
		if err != nil {
			return nil, fmt.Errorf("TLS config: %w", err)
		}
		opts.SetTLSConfig(tlsConfig)
	}

	opts.SetConnectionLostHandler(func(c mqtt.Client, err error) {
		log.Printf("Connection lost: %v", err)
	})
	opts.SetOnConnectHandler(func(c mqtt.Client) {
		log.Printf("Connected to broker")
	})

	client := mqtt.NewClient(opts)
	token := client.Connect()
	token.Wait()
	if token.Error() != nil {
		return nil, token.Error()
	}

	return client, nil
}

func newTLSConfig(caFile string) (*tls.Config, error) {
	certpool := x509.NewCertPool()
	pemCerts, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("read CA file: %w", err)
	}
	certpool.AppendCertsFromPEM(pemCerts)
	return &tls.Config{RootCAs: certpool}, nil
}
```

**Step 8: Verify Go builds**

Run: `cd /Users/steven/AI_Playground/Clawpeteer/agent && go build .`
Expected: Builds without errors

**Step 9: Commit**

```bash
git add agent/
git commit -m "feat: Go agent config loading and MQTT connection"
```

---

## Task 4: Go Agent — Command Executor

**Files:**
- Create: `agent/internal/executor/executor.go`
- Create: `agent/internal/executor/executor_test.go`

**Step 1: Write executor test**

`agent/internal/executor/executor_test.go`:
```go
package executor

import (
	"runtime"
	"testing"
	"time"
)

func TestExecSimpleCommand(t *testing.T) {
	var cmd string
	if runtime.GOOS == "windows" {
		cmd = "echo hello"
	} else {
		cmd = "echo hello"
	}

	result, err := ExecSync(cmd, 5*time.Second)
	if err != nil {
		t.Fatalf("ExecSync failed: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", result.ExitCode)
	}
	if len(result.Stdout) == 0 {
		t.Error("Stdout is empty, expected 'hello'")
	}
}

func TestExecCommandNotFound(t *testing.T) {
	result, err := ExecSync("nonexistent_command_xyz", 5*time.Second)
	if err != nil {
		// exec error is ok
		return
	}
	if result.ExitCode == 0 {
		t.Error("Expected non-zero exit code for invalid command")
	}
}

func TestExecTimeout(t *testing.T) {
	var cmd string
	if runtime.GOOS == "windows" {
		cmd = "ping -n 10 127.0.0.1"
	} else {
		cmd = "sleep 10"
	}

	_, err := ExecSync(cmd, 1*time.Second)
	if err == nil {
		t.Error("Expected timeout error")
	}
}

func TestSpawnCommand(t *testing.T) {
	var cmd string
	if runtime.GOOS == "windows" {
		cmd = "echo streaming"
	} else {
		cmd = "echo streaming"
	}

	proc, err := Spawn(cmd)
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}
	if proc.PID <= 0 {
		t.Errorf("PID = %d, want > 0", proc.PID)
	}
	// Wait for completion
	result := <-proc.Done
	if result.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", result.ExitCode)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/steven/AI_Playground/Clawpeteer/agent && go test ./internal/executor/ -v`
Expected: FAIL

**Step 3: Implement executor**

`agent/internal/executor/executor.go`:
```go
package executor

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"time"
)

type Result struct {
	ExitCode int
	Stdout   string
	Stderr   string
	Duration time.Duration
}

type Process struct {
	PID      int
	Cmd      *exec.Cmd
	Stdout   io.ReadCloser
	Stderr   io.ReadCloser
	Done     chan Result
	cancel   context.CancelFunc
}

func getShell() (string, string) {
	if runtime.GOOS == "windows" {
		return "cmd.exe", "/C"
	}
	return "/bin/bash", "-c"
}

func ExecSync(command string, timeout time.Duration) (*Result, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	shell, flag := getShell()
	cmd := exec.CommandContext(ctx, shell, flag, command)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)

	if ctx.Err() == context.DeadlineExceeded {
		return nil, fmt.Errorf("command timed out after %v", timeout)
	}

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, fmt.Errorf("exec: %w", err)
		}
	}

	return &Result{
		ExitCode: exitCode,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Duration: duration,
	}, nil
}

func Spawn(command string) (*Process, error) {
	ctx, cancel := context.WithCancel(context.Background())
	shell, flag := getShell()
	cmd := exec.CommandContext(ctx, shell, flag, command)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("start: %w", err)
	}

	proc := &Process{
		PID:    cmd.Process.Pid,
		Cmd:    cmd,
		Stdout: stdout,
		Stderr: stderr,
		Done:   make(chan Result, 1),
		cancel: cancel,
	}

	go func() {
		var stdoutBuf, stderrBuf bytes.Buffer
		io.Copy(&stdoutBuf, stdout)
		io.Copy(&stderrBuf, stderr)
		start := time.Now()
		err := cmd.Wait()
		exitCode := 0
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			}
		}
		proc.Done <- Result{
			ExitCode: exitCode,
			Stdout:   stdoutBuf.String(),
			Stderr:   stderrBuf.String(),
			Duration: time.Since(start),
		}
	}()

	return proc, nil
}

func (p *Process) Kill() {
	if p.cancel != nil {
		p.cancel()
	}
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/steven/AI_Playground/Clawpeteer/agent && go test ./internal/executor/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add agent/internal/executor/
git commit -m "feat: Go agent command executor with sync/spawn modes"
```

---

## Task 5: Go Agent — Task Manager

**Files:**
- Create: `agent/internal/taskmanager/manager.go`
- Create: `agent/internal/taskmanager/manager_test.go`

**Step 1: Write task manager test**

`agent/internal/taskmanager/manager_test.go`:
```go
package taskmanager

import (
	"testing"
)

func TestAddAndGetTask(t *testing.T) {
	tm := New()
	tm.Add("task-1", "echo hello", 1234)

	task, ok := tm.Get("task-1")
	if !ok {
		t.Fatal("task not found")
	}
	if task.Command != "echo hello" {
		t.Errorf("Command = %q, want %q", task.Command, "echo hello")
	}
	if task.PID != 1234 {
		t.Errorf("PID = %d, want 1234", task.PID)
	}
	if task.Status != StatusRunning {
		t.Errorf("Status = %q, want %q", task.Status, StatusRunning)
	}
}

func TestListTasks(t *testing.T) {
	tm := New()
	tm.Add("task-1", "echo a", 100)
	tm.Add("task-2", "echo b", 200)

	tasks := tm.List()
	if len(tasks) != 2 {
		t.Errorf("len = %d, want 2", len(tasks))
	}
}

func TestCompleteTask(t *testing.T) {
	tm := New()
	tm.Add("task-1", "echo a", 100)
	tm.Complete("task-1", 0)

	task, ok := tm.Get("task-1")
	if !ok {
		t.Fatal("task not found")
	}
	if task.Status != StatusCompleted {
		t.Errorf("Status = %q, want %q", task.Status, StatusCompleted)
	}
}

func TestRemoveCompleted(t *testing.T) {
	tm := New()
	tm.Add("task-1", "echo a", 100)
	tm.Complete("task-1", 0)
	tm.Add("task-2", "echo b", 200)

	tm.RemoveCompleted()
	tasks := tm.List()
	if len(tasks) != 1 {
		t.Errorf("len = %d, want 1 after cleanup", len(tasks))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/steven/AI_Playground/Clawpeteer/agent && go test ./internal/taskmanager/ -v`
Expected: FAIL

**Step 3: Implement task manager**

`agent/internal/taskmanager/manager.go`:
```go
package taskmanager

import (
	"sync"
	"time"
)

const (
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusKilled    = "killed"
	StatusError     = "error"
)

type Task struct {
	ID        string    `json:"id"`
	Command   string    `json:"command"`
	PID       int       `json:"pid"`
	Status    string    `json:"status"`
	ExitCode  int       `json:"exitCode"`
	StartTime time.Time `json:"startTime"`
	EndTime   time.Time `json:"endTime,omitempty"`
}

type Manager struct {
	mu    sync.RWMutex
	tasks map[string]*Task
}

func New() *Manager {
	return &Manager{
		tasks: make(map[string]*Task),
	}
}

func (m *Manager) Add(id, command string, pid int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tasks[id] = &Task{
		ID:        id,
		Command:   command,
		PID:       pid,
		Status:    StatusRunning,
		StartTime: time.Now(),
	}
}

func (m *Manager) Get(id string) (*Task, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, ok := m.tasks[id]
	return t, ok
}

func (m *Manager) List() []*Task {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*Task, 0, len(m.tasks))
	for _, t := range m.tasks {
		result = append(result, t)
	}
	return result
}

func (m *Manager) Complete(id string, exitCode int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t, ok := m.tasks[id]; ok {
		t.Status = StatusCompleted
		t.ExitCode = exitCode
		t.EndTime = time.Now()
	}
}

func (m *Manager) SetStatus(id, status string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t, ok := m.tasks[id]; ok {
		t.Status = status
		if status != StatusRunning {
			t.EndTime = time.Now()
		}
	}
}

func (m *Manager) RunningCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	count := 0
	for _, t := range m.tasks {
		if t.Status == StatusRunning {
			count++
		}
	}
	return count
}

func (m *Manager) RemoveCompleted() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, t := range m.tasks {
		if t.Status != StatusRunning {
			delete(m.tasks, id)
		}
	}
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/steven/AI_Playground/Clawpeteer/agent && go test ./internal/taskmanager/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add agent/internal/taskmanager/
git commit -m "feat: Go agent task manager with concurrent-safe task tracking"
```

---

## Task 6: Go Agent — Security Module

**Files:**
- Create: `agent/internal/security/security.go`
- Create: `agent/internal/security/security_test.go`

**Step 1: Write security test**

`agent/internal/security/security_test.go`:
```go
package security

import (
	"testing"
)

func TestWhitelist(t *testing.T) {
	v := New("whitelist", []string{"ls", "pwd", "echo"}, nil, nil, nil)

	if err := v.ValidateCommand("ls -la"); err != nil {
		t.Errorf("should allow 'ls -la': %v", err)
	}
	if err := v.ValidateCommand("echo hello"); err != nil {
		t.Errorf("should allow 'echo hello': %v", err)
	}
	if err := v.ValidateCommand("rm -rf /"); err == nil {
		t.Error("should reject 'rm -rf /'")
	}
}

func TestBlacklist(t *testing.T) {
	v := New("blacklist", nil, []string{"rm -rf /", "mkfs"}, nil, nil)

	if err := v.ValidateCommand("ls -la"); err != nil {
		t.Errorf("should allow 'ls -la': %v", err)
	}
	if err := v.ValidateCommand("rm -rf /"); err == nil {
		t.Error("should reject 'rm -rf /'")
	}
	if err := v.ValidateCommand("mkfs.ext4 /dev/sda"); err == nil {
		t.Error("should reject command containing 'mkfs'")
	}
}

func TestValidatePath(t *testing.T) {
	v := New("none", nil, nil,
		[]string{"/home/user/uploads", "/tmp"},
		[]string{"/home/user", "/var/log"},
	)

	if err := v.ValidateUploadPath("/home/user/uploads/file.txt"); err != nil {
		t.Errorf("should allow upload to /home/user/uploads: %v", err)
	}
	if err := v.ValidateUploadPath("/etc/passwd"); err == nil {
		t.Error("should reject upload to /etc/passwd")
	}
	if err := v.ValidateDownloadPath("/var/log/app.log"); err != nil {
		t.Errorf("should allow download from /var/log: %v", err)
	}
	if err := v.ValidateDownloadPath("/etc/shadow"); err == nil {
		t.Error("should reject download from /etc/shadow")
	}
}

func TestPathTraversal(t *testing.T) {
	v := New("none", nil, nil,
		[]string{"/home/user/uploads"},
		nil,
	)

	if err := v.ValidateUploadPath("/home/user/uploads/../../../etc/passwd"); err == nil {
		t.Error("should reject path traversal")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/steven/AI_Playground/Clawpeteer/agent && go test ./internal/security/ -v`
Expected: FAIL

**Step 3: Implement security**

`agent/internal/security/security.go`:
```go
package security

import (
	"fmt"
	"path/filepath"
	"strings"
)

type Validator struct {
	mode         string
	whitelist    []string
	blacklist    []string
	uploadDirs   []string
	downloadDirs []string
}

func New(mode string, whitelist, blacklist, uploadDirs, downloadDirs []string) *Validator {
	return &Validator{
		mode:         mode,
		whitelist:    whitelist,
		blacklist:    blacklist,
		uploadDirs:   uploadDirs,
		downloadDirs: downloadDirs,
	}
}

func (v *Validator) ValidateCommand(command string) error {
	switch v.mode {
	case "whitelist":
		cmd := strings.TrimSpace(command)
		parts := strings.Fields(cmd)
		if len(parts) == 0 {
			return fmt.Errorf("empty command")
		}
		base := filepath.Base(parts[0])
		for _, allowed := range v.whitelist {
			if base == allowed {
				return nil
			}
		}
		return fmt.Errorf("command not allowed: %s", base)

	case "blacklist":
		for _, blocked := range v.blacklist {
			if strings.Contains(command, blocked) {
				return fmt.Errorf("blocked pattern: %s", blocked)
			}
		}
		return nil

	case "none":
		return nil

	default:
		return fmt.Errorf("unknown security mode: %s", v.mode)
	}
}

func (v *Validator) ValidateUploadPath(destPath string) error {
	return v.validatePath(destPath, v.uploadDirs, "upload")
}

func (v *Validator) ValidateDownloadPath(srcPath string) error {
	return v.validatePath(srcPath, v.downloadDirs, "download")
}

func (v *Validator) validatePath(p string, allowed []string, direction string) error {
	if len(allowed) == 0 {
		return nil // no restrictions
	}

	resolved, err := filepath.Abs(p)
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}
	resolved = filepath.Clean(resolved)

	for _, dir := range allowed {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		absDir = filepath.Clean(absDir)
		if strings.HasPrefix(resolved, absDir+string(filepath.Separator)) || resolved == absDir {
			return nil
		}
	}

	return fmt.Errorf("%s path not allowed: %s", direction, p)
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/steven/AI_Playground/Clawpeteer/agent && go test ./internal/security/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add agent/internal/security/
git commit -m "feat: Go agent security module with whitelist/blacklist and path validation"
```

---

## Task 7: Go Agent — Message Handling + Heartbeat

**Files:**
- Create: `agent/internal/handler/handler.go`
- Modify: `agent/main.go` — wire up handler + heartbeat

**Step 1: Create handler that processes MQTT commands**

`agent/internal/handler/handler.go`:
```go
package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"runtime"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/stevenmeow/clawpeteer-agent/internal/executor"
	"github.com/stevenmeow/clawpeteer-agent/internal/security"
	"github.com/stevenmeow/clawpeteer-agent/internal/taskmanager"
)

type Command struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Command    string `json:"command,omitempty"`
	Timeout    int    `json:"timeout,omitempty"`
	Background bool   `json:"background,omitempty"`
	Stream     bool   `json:"stream,omitempty"`
	SourcePath string `json:"sourcePath,omitempty"`
	TransferID string `json:"transferId,omitempty"`
	ChunkSize  int    `json:"chunkSize,omitempty"`
	Timestamp  int64  `json:"timestamp"`
}

type ControlCommand struct {
	Action string `json:"action"`
	Signal string `json:"signal,omitempty"`
}

type Result struct {
	TaskID   string `json:"taskId"`
	Status   string `json:"status"`
	PID      int    `json:"pid,omitempty"`
	ExitCode int    `json:"exitCode,omitempty"`
	Stdout   string `json:"stdout,omitempty"`
	Stderr   string `json:"stderr,omitempty"`
	Duration int64  `json:"duration,omitempty"`
	Error    string `json:"error,omitempty"`
	Signal   string `json:"signal,omitempty"`
	Timestamp int64 `json:"timestamp"`
}

type StreamMessage struct {
	Type      string `json:"type"`
	Data      string `json:"data"`
	Timestamp int64  `json:"timestamp"`
}

type Heartbeat struct {
	Status       string `json:"status"`
	Platform     string `json:"platform"`
	Arch         string `json:"arch"`
	Hostname     string `json:"hostname"`
	Version      string `json:"version"`
	Uptime       int64  `json:"uptime"`
	RunningTasks int    `json:"runningTasks"`
	Timestamp    int64  `json:"timestamp"`
}

type Handler struct {
	agentID   string
	client    mqtt.Client
	tasks     *taskmanager.Manager
	security  *security.Validator
	processes map[string]*executor.Process
	startTime time.Time
}

func New(agentID string, client mqtt.Client, tasks *taskmanager.Manager, sec *security.Validator) *Handler {
	return &Handler{
		agentID:   agentID,
		client:    client,
		tasks:     tasks,
		security:  sec,
		processes: make(map[string]*executor.Process),
		startTime: time.Now(),
	}
}

func (h *Handler) Subscribe() {
	cmdTopic := fmt.Sprintf("agents/%s/commands", h.agentID)
	ctrlTopic := fmt.Sprintf("agents/%s/control/#", h.agentID)

	h.client.Subscribe(cmdTopic, 1, h.handleCommand)
	h.client.Subscribe(ctrlTopic, 1, h.handleControl)

	log.Printf("Subscribed to %s and %s", cmdTopic, ctrlTopic)

	// Publish registry
	h.publishRegistry()
}

func (h *Handler) handleCommand(client mqtt.Client, msg mqtt.Message) {
	var cmd Command
	if err := json.Unmarshal(msg.Payload(), &cmd); err != nil {
		log.Printf("Invalid command: %v", err)
		return
	}

	log.Printf("Received command [%s]: type=%s", cmd.ID, cmd.Type)

	switch cmd.Type {
	case "execute":
		go h.executeCommand(cmd)
	default:
		log.Printf("Unknown command type: %s", cmd.Type)
	}
}

func (h *Handler) executeCommand(cmd Command) {
	// Security check
	if err := h.security.ValidateCommand(cmd.Command); err != nil {
		h.publishResult(Result{
			TaskID:    cmd.ID,
			Status:    "error",
			Error:     err.Error(),
			Timestamp: time.Now().UnixMilli(),
		})
		return
	}

	if cmd.Background || cmd.Stream {
		proc, err := executor.Spawn(cmd.Command)
		if err != nil {
			h.publishResult(Result{
				TaskID:    cmd.ID,
				Status:    "error",
				Error:     err.Error(),
				Timestamp: time.Now().UnixMilli(),
			})
			return
		}

		h.tasks.Add(cmd.ID, cmd.Command, proc.PID)
		h.processes[cmd.ID] = proc

		// Report started
		h.publishResult(Result{
			TaskID:    cmd.ID,
			Status:    "started",
			PID:       proc.PID,
			Timestamp: time.Now().UnixMilli(),
		})

		// Stream output if requested
		if cmd.Stream {
			go h.streamOutput(cmd.ID, proc)
		}

		// Wait for completion
		go func() {
			result := <-proc.Done
			h.tasks.Complete(cmd.ID, result.ExitCode)
			delete(h.processes, cmd.ID)

			h.publishResult(Result{
				TaskID:    cmd.ID,
				Status:    "completed",
				ExitCode:  result.ExitCode,
				Stdout:    result.Stdout,
				Stderr:    result.Stderr,
				Duration:  result.Duration.Milliseconds(),
				Timestamp: time.Now().UnixMilli(),
			})
		}()
	} else {
		timeout := time.Duration(cmd.Timeout) * time.Millisecond
		if timeout == 0 {
			timeout = 30 * time.Second
		}

		result, err := executor.ExecSync(cmd.Command, timeout)
		if err != nil {
			h.publishResult(Result{
				TaskID:    cmd.ID,
				Status:    "error",
				Error:     err.Error(),
				Timestamp: time.Now().UnixMilli(),
			})
			return
		}

		h.publishResult(Result{
			TaskID:    cmd.ID,
			Status:    "completed",
			ExitCode:  result.ExitCode,
			Stdout:    result.Stdout,
			Stderr:    result.Stderr,
			Duration:  result.Duration.Milliseconds(),
			Timestamp: time.Now().UnixMilli(),
		})
	}
}

func (h *Handler) streamOutput(taskID string, proc *executor.Process) {
	topic := fmt.Sprintf("agents/%s/stream/%s", h.agentID, taskID)
	buf := make([]byte, 4096)

	go func() {
		for {
			n, err := proc.Stdout.Read(buf)
			if n > 0 {
				msg := StreamMessage{
					Type:      "stdout",
					Data:      string(buf[:n]),
					Timestamp: time.Now().UnixMilli(),
				}
				data, _ := json.Marshal(msg)
				h.client.Publish(topic, 0, false, data)
			}
			if err != nil {
				return
			}
		}
	}()

	go func() {
		for {
			n, err := proc.Stderr.Read(buf)
			if n > 0 {
				msg := StreamMessage{
					Type:      "stderr",
					Data:      string(buf[:n]),
					Timestamp: time.Now().UnixMilli(),
				}
				data, _ := json.Marshal(msg)
				h.client.Publish(topic, 0, false, data)
			}
			if err != nil {
				return
			}
		}
	}()
}

func (h *Handler) handleControl(client mqtt.Client, msg mqtt.Message) {
	var ctrl ControlCommand
	if err := json.Unmarshal(msg.Payload(), &ctrl); err != nil {
		log.Printf("Invalid control command: %v", err)
		return
	}

	switch ctrl.Action {
	case "kill":
		// Extract task ID from topic: agents/{id}/control/{task-id}
		// Simple approach: iterate processes
		for taskID, proc := range h.processes {
			proc.Kill()
			h.tasks.SetStatus(taskID, "killed")
			h.publishResult(Result{
				TaskID:    taskID,
				Status:    "killed",
				Signal:    ctrl.Signal,
				Timestamp: time.Now().UnixMilli(),
			})
		}
	case "query", "list":
		tasks := h.tasks.List()
		data, _ := json.Marshal(map[string]interface{}{
			"action": "list",
			"tasks":  tasks,
		})
		topic := fmt.Sprintf("agents/%s/results", h.agentID)
		h.client.Publish(topic, 1, false, data)
	}
}

func (h *Handler) publishResult(result Result) {
	topic := fmt.Sprintf("agents/%s/results", h.agentID)
	data, _ := json.Marshal(result)
	h.client.Publish(topic, 1, false, data)
}

func (h *Handler) publishRegistry() {
	hostname, _ := os.Hostname()
	reg := map[string]interface{}{
		"agentId":      h.agentID,
		"platform":     runtime.GOOS,
		"arch":         runtime.GOARCH,
		"hostname":     hostname,
		"version":      "1.0.0",
		"capabilities": []string{"shell", "file-transfer"},
		"timestamp":    time.Now().UnixMilli(),
	}
	data, _ := json.Marshal(reg)
	h.client.Publish("agents/registry", 1, true, data)
}

func (h *Handler) StartHeartbeat(interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		// Send initial heartbeat
		h.sendHeartbeat()
		for range ticker.C {
			h.sendHeartbeat()
		}
	}()
}

func (h *Handler) sendHeartbeat() {
	hostname, _ := os.Hostname()
	hb := Heartbeat{
		Status:       "online",
		Platform:     runtime.GOOS,
		Arch:         runtime.GOARCH,
		Hostname:     hostname,
		Version:      "1.0.0",
		Uptime:       time.Since(h.startTime).Milliseconds(),
		RunningTasks: h.tasks.RunningCount(),
		Timestamp:    time.Now().UnixMilli(),
	}
	data, _ := json.Marshal(hb)
	topic := fmt.Sprintf("agents/%s/heartbeat", h.agentID)
	h.client.Publish(topic, 1, true, data)
}
```

**Step 2: Update main.go to wire everything**

Replace `agent/main.go` content with the version from Task 3 Step 7, adding after `client` creation:

```go
// After connectMQTT succeeds, add:
secValidator := security.New(
    cfg.Security.Mode,
    cfg.Security.Whitelist,
    cfg.Security.Blacklist,
    cfg.Security.UploadDirs,
    cfg.Security.DownloadDirs,
)

tasks := taskmanager.New()
h := handler.New(cfg.AgentID, client, tasks, secValidator)
h.Subscribe()
h.StartHeartbeat(time.Duration(cfg.HeartbeatInterval) * time.Second)

log.Printf("Agent [%s] ready. Listening for commands...", cfg.AgentID)
```

Add imports for the new packages.

**Step 3: Verify Go builds**

Run: `cd /Users/steven/AI_Playground/Clawpeteer/agent && go build .`
Expected: Builds without errors

**Step 4: Commit**

```bash
git add agent/
git commit -m "feat: Go agent command handler, streaming, heartbeat, and registry"
```

---

## Task 8: Go Agent — File Transfer Handler

**Files:**
- Create: `agent/internal/filetransfer/handler.go`
- Create: `agent/internal/filetransfer/handler_test.go`

**Step 1: Write file transfer test**

`agent/internal/filetransfer/handler_test.go`:
```go
package filetransfer

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestReceiveChunksAndAssemble(t *testing.T) {
	dir := t.TempDir()
	ft := NewReceiver(dir)

	transferID := "test-transfer-1"
	destPath := filepath.Join(dir, "output.txt")
	content := "Hello, this is test content for file transfer!"

	// Calculate expected hash
	hash := sha256.Sum256([]byte(content))
	expectedSha := hex.EncodeToString(hash[:])

	// Split into 10-byte chunks
	chunkSize := 10
	totalChunks := (len(content) + chunkSize - 1) / chunkSize

	err := ft.InitTransfer(transferID, "output.txt", destPath, int64(len(content)), totalChunks, expectedSha)
	if err != nil {
		t.Fatalf("InitTransfer: %v", err)
	}

	for i := 0; i < totalChunks; i++ {
		start := i * chunkSize
		end := start + chunkSize
		if end > len(content) {
			end = len(content)
		}
		chunk := base64.StdEncoding.EncodeToString([]byte(content[start:end]))
		err := ft.ReceiveChunk(transferID, i, chunk)
		if err != nil {
			t.Fatalf("ReceiveChunk %d: %v", i, err)
		}
	}

	verified, err := ft.Finalize(transferID)
	if err != nil {
		t.Fatalf("Finalize: %v", err)
	}
	if !verified {
		t.Error("checksum verification failed")
	}

	// Verify file content
	data, _ := os.ReadFile(destPath)
	if string(data) != content {
		t.Errorf("file content = %q, want %q", string(data), content)
	}
}

func TestReceiveChunkOutOfOrder(t *testing.T) {
	dir := t.TempDir()
	ft := NewReceiver(dir)

	transferID := "test-ooo"
	destPath := filepath.Join(dir, "ooo.txt")
	content := "ABCDEFGHIJ"
	hash := sha256.Sum256([]byte(content))
	expectedSha := hex.EncodeToString(hash[:])

	ft.InitTransfer(transferID, "ooo.txt", destPath, 10, 2, expectedSha)

	// Receive chunk 1 before chunk 0
	ft.ReceiveChunk(transferID, 1, base64.StdEncoding.EncodeToString([]byte("FGHIJ")))
	ft.ReceiveChunk(transferID, 0, base64.StdEncoding.EncodeToString([]byte("ABCDE")))

	verified, err := ft.Finalize(transferID)
	if err != nil {
		t.Fatalf("Finalize: %v", err)
	}
	if !verified {
		t.Error("checksum failed for out-of-order chunks")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/steven/AI_Playground/Clawpeteer/agent && go test ./internal/filetransfer/ -v`
Expected: FAIL

**Step 3: Implement file transfer receiver**

`agent/internal/filetransfer/handler.go`:
```go
package filetransfer

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

type Transfer struct {
	ID             string
	Filename       string
	DestPath       string
	Size           int64
	TotalChunks    int
	ExpectedSha256 string
	ReceivedChunks map[int]bool
	ChunkDir       string
}

type Receiver struct {
	mu        sync.Mutex
	baseDir   string
	transfers map[string]*Transfer
}

func NewReceiver(baseDir string) *Receiver {
	return &Receiver{
		baseDir:   baseDir,
		transfers: make(map[string]*Transfer),
	}
}

func (r *Receiver) InitTransfer(id, filename, destPath string, size int64, totalChunks int, sha256sum string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	chunkDir := filepath.Join(r.baseDir, "transfers", id, "chunks")
	if err := os.MkdirAll(chunkDir, 0755); err != nil {
		return fmt.Errorf("create chunk dir: %w", err)
	}

	// Ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("create dest dir: %w", err)
	}

	r.transfers[id] = &Transfer{
		ID:             id,
		Filename:       filename,
		DestPath:       destPath,
		Size:           size,
		TotalChunks:    totalChunks,
		ExpectedSha256: sha256sum,
		ReceivedChunks: make(map[int]bool),
		ChunkDir:       chunkDir,
	}

	return nil
}

func (r *Receiver) ReceiveChunk(id string, index int, data string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	t, ok := r.transfers[id]
	if !ok {
		return fmt.Errorf("unknown transfer: %s", id)
	}

	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return fmt.Errorf("decode chunk: %w", err)
	}

	chunkPath := filepath.Join(t.ChunkDir, fmt.Sprintf("%06d.bin", index))
	if err := os.WriteFile(chunkPath, decoded, 0644); err != nil {
		return fmt.Errorf("write chunk: %w", err)
	}

	t.ReceivedChunks[index] = true
	return nil
}

func (r *Receiver) Progress(id string) (received, total int, ok bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	t, exists := r.transfers[id]
	if !exists {
		return 0, 0, false
	}
	return len(t.ReceivedChunks), t.TotalChunks, true
}

func (r *Receiver) MissingChunks(id string) ([]int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	t, ok := r.transfers[id]
	if !ok {
		return nil, fmt.Errorf("unknown transfer: %s", id)
	}

	var missing []int
	for i := 0; i < t.TotalChunks; i++ {
		if !t.ReceivedChunks[i] {
			missing = append(missing, i)
		}
	}
	return missing, nil
}

func (r *Receiver) Finalize(id string) (bool, error) {
	r.mu.Lock()
	t, ok := r.transfers[id]
	r.mu.Unlock()

	if !ok {
		return false, fmt.Errorf("unknown transfer: %s", id)
	}

	// Check all chunks received
	if len(t.ReceivedChunks) != t.TotalChunks {
		return false, fmt.Errorf("missing chunks: got %d/%d", len(t.ReceivedChunks), t.TotalChunks)
	}

	// Assemble file
	destFile, err := os.Create(t.DestPath)
	if err != nil {
		return false, fmt.Errorf("create dest: %w", err)
	}
	defer destFile.Close()

	hasher := sha256.New()
	writer := io.MultiWriter(destFile, hasher)

	for i := 0; i < t.TotalChunks; i++ {
		chunkPath := filepath.Join(t.ChunkDir, fmt.Sprintf("%06d.bin", i))
		data, err := os.ReadFile(chunkPath)
		if err != nil {
			return false, fmt.Errorf("read chunk %d: %w", i, err)
		}
		if _, err := writer.Write(data); err != nil {
			return false, fmt.Errorf("write chunk %d: %w", i, err)
		}
	}

	// Verify checksum
	actualSha := hex.EncodeToString(hasher.Sum(nil))
	verified := actualSha == t.ExpectedSha256

	// Cleanup chunks
	os.RemoveAll(filepath.Join(r.baseDir, "transfers", id))

	r.mu.Lock()
	delete(r.transfers, id)
	r.mu.Unlock()

	return verified, nil
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/steven/AI_Playground/Clawpeteer/agent && go test ./internal/filetransfer/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add agent/internal/filetransfer/
git commit -m "feat: Go agent file transfer receiver with chunking and SHA-256 verification"
```

---

## Task 9: Go Agent — Wire File Transfer into Handler

**Files:**
- Modify: `agent/internal/handler/handler.go` — add file upload/download handling

**Step 1: Add file upload/download message types and subscription**

Add to `handler.go` — subscribe to file upload topics:

```go
// In Subscribe(), add:
uploadMetaTopic := fmt.Sprintf("agents/%s/files/upload/+/meta", h.agentID)
uploadChunkTopic := fmt.Sprintf("agents/%s/files/upload/+/chunks", h.agentID)
h.client.Subscribe(uploadMetaTopic, 1, h.handleUploadMeta)
h.client.Subscribe(uploadChunkTopic, 1, h.handleUploadChunk)
```

Add file download handler in `handleCommand` for `type: "file_download"`.

Add `handleUploadMeta` and `handleUploadChunk` methods that use the `filetransfer.Receiver`.

Add download sender that reads a local file, chunks it, and publishes to `agents/{id}/files/download/{transfer-id}/meta` and `chunks`.

**Step 2: Add Receiver field to Handler**

```go
type Handler struct {
    // ...existing fields...
    fileReceiver *filetransfer.Receiver
}
```

Initialize in `New()` with a base dir like `~/.clawpeteer`.

**Step 3: Verify builds**

Run: `cd /Users/steven/AI_Playground/Clawpeteer/agent && go build .`
Expected: Builds without errors

**Step 4: Commit**

```bash
git add agent/
git commit -m "feat: wire file transfer into Go agent handler"
```

---

## Task 10: CLI — MQTT Client Module

**Files:**
- Create: `cli/src/mqtt-client.js`
- Create: `cli/src/config.js`
- Create: `cli/config.example.json`
- Create: `cli/tests/mqtt-client.test.js`

**Step 1: Create config module**

`cli/src/config.js`:
```javascript
import { readFileSync, existsSync } from 'fs';
import { join, dirname } from 'path';
import { fileURLToPath } from 'url';

const __dirname = dirname(fileURLToPath(import.meta.url));

export function loadConfig(configPath) {
  const paths = [
    configPath,
    join(process.cwd(), 'config.json'),
    join(__dirname, '..', 'config.json'),
    join(process.env.HOME || '', '.clawpeteer', 'config.json'),
  ].filter(Boolean);

  for (const p of paths) {
    if (existsSync(p)) {
      const data = readFileSync(p, 'utf-8');
      return JSON.parse(data);
    }
  }

  throw new Error(
    'Config not found. Create config.json or run with --config <path>\n' +
    'See config.example.json for template.'
  );
}
```

**Step 2: Create MQTT client wrapper**

`cli/src/mqtt-client.js`:
```javascript
import mqtt from 'mqtt';
import { readFileSync } from 'fs';
import { EventEmitter } from 'events';

export class MQTTClient extends EventEmitter {
  constructor(config) {
    super();
    this.config = config;
    this.client = null;
    this.connected = false;
  }

  async connect() {
    const options = {
      clientId: `clawpeteer-cli-${Date.now()}`,
      username: this.config.username,
      password: this.config.password,
      clean: false,
      reconnectPeriod: 5000,
    };

    if (this.config.caFile) {
      options.ca = [readFileSync(this.config.caFile)];
    }

    return new Promise((resolve, reject) => {
      this.client = mqtt.connect(this.config.brokerUrl, options);

      this.client.on('connect', () => {
        this.connected = true;
        resolve();
      });

      this.client.on('error', (err) => {
        if (!this.connected) reject(err);
        this.emit('error', err);
      });

      this.client.on('message', (topic, payload) => {
        try {
          const msg = JSON.parse(payload.toString());
          this.emit('message', topic, msg);
        } catch {
          this.emit('message', topic, payload.toString());
        }
      });

      this.client.on('close', () => {
        this.connected = false;
      });

      setTimeout(() => {
        if (!this.connected) reject(new Error('Connection timeout'));
      }, 10000);
    });
  }

  subscribe(topic, qos = 1) {
    return new Promise((resolve, reject) => {
      this.client.subscribe(topic, { qos }, (err) => {
        if (err) reject(err);
        else resolve();
      });
    });
  }

  publish(topic, payload, qos = 1, retain = false) {
    const data = typeof payload === 'string' ? payload : JSON.stringify(payload);
    return new Promise((resolve, reject) => {
      this.client.publish(topic, data, { qos, retain }, (err) => {
        if (err) reject(err);
        else resolve();
      });
    });
  }

  async waitForMessage(topicPattern, filter, timeoutMs = 30000) {
    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => {
        reject(new Error(`Timeout waiting for response (${timeoutMs}ms)`));
      }, timeoutMs);

      const handler = (topic, msg) => {
        if (topic.match(topicPattern) && (!filter || filter(msg))) {
          clearTimeout(timer);
          this.removeListener('message', handler);
          resolve(msg);
        }
      };
      this.on('message', handler);
    });
  }

  disconnect() {
    if (this.client) {
      this.client.end();
    }
  }
}
```

**Step 3: Create config example**

`cli/config.example.json`:
```json
{
  "brokerUrl": "mqtt://localhost:1883",
  "username": "openclaw",
  "password": "your-password",
  "caFile": ""
}
```

**Step 4: Commit**

```bash
git add cli/src/ cli/config.example.json
git commit -m "feat: CLI MQTT client module with config loading"
```

---

## Task 11: CLI — Send Command

**Files:**
- Create: `cli/src/commands/send.js`
- Modify: `cli/bin/clawpeteer.js`

**Step 1: Implement send command**

`cli/src/commands/send.js`:
```javascript
import { v4 as uuidv4 } from 'uuid';
import { MQTTClient } from '../mqtt-client.js';
import { loadConfig } from '../config.js';

export function registerSendCommand(program) {
  program
    .command('send')
    .description('Execute a command on a remote agent')
    .argument('<agent>', 'target agent ID')
    .argument('<command>', 'shell command to execute')
    .option('-s, --stream', 'stream output in real-time')
    .option('-b, --background', 'run in background')
    .option('-t, --timeout <ms>', 'timeout in milliseconds', '30000')
    .option('-c, --config <path>', 'config file path')
    .action(async (agent, command, options) => {
      const config = loadConfig(options.config);
      const mqtt = new MQTTClient(config);

      try {
        await mqtt.connect();
        const taskId = uuidv4();

        // Subscribe to results and stream
        await mqtt.subscribe(`agents/${agent}/results`);
        if (options.stream) {
          await mqtt.subscribe(`agents/${agent}/stream/${taskId}`);
        }

        // Send command
        await mqtt.publish(`agents/${agent}/commands`, {
          id: taskId,
          type: 'execute',
          command: command,
          timeout: parseInt(options.timeout),
          background: !!options.background,
          stream: !!options.stream,
          timestamp: Date.now(),
        });

        console.log(`Task ${taskId} sent to ${agent}`);

        // Handle streaming
        if (options.stream) {
          mqtt.on('message', (topic, msg) => {
            if (topic.includes('/stream/')) {
              process.stdout.write(msg.data || '');
            }
          });
        }

        // Wait for result
        const result = await mqtt.waitForMessage(
          new RegExp(`agents/${agent}/results`),
          (msg) => msg.taskId === taskId && msg.status !== 'started',
          parseInt(options.timeout) + 5000,
        );

        if (result.status === 'completed') {
          if (!options.stream && result.stdout) {
            process.stdout.write(result.stdout);
          }
          if (result.stderr) {
            process.stderr.write(result.stderr);
          }
          process.exit(result.exitCode || 0);
        } else if (result.status === 'error') {
          console.error(`Error: ${result.error}`);
          process.exit(1);
        } else if (result.status === 'killed') {
          console.error(`Task killed (${result.signal})`);
          process.exit(137);
        }
      } catch (err) {
        console.error(`Failed: ${err.message}`);
        process.exit(1);
      } finally {
        mqtt.disconnect();
      }
    });
}
```

**Step 2: Update CLI entry point**

`cli/bin/clawpeteer.js`:
```javascript
#!/usr/bin/env node
import { program } from 'commander';
import { registerSendCommand } from '../src/commands/send.js';

program
  .name('clawpeteer')
  .description('MQTT remote control CLI for OpenClaw')
  .version('1.0.0');

registerSendCommand(program);

program.parse();
```

**Step 3: Verify CLI shows send command**

Run: `cd /Users/steven/AI_Playground/Clawpeteer/cli && node bin/clawpeteer.js send --help`
Expected: Shows send command help with arguments and options

**Step 4: Commit**

```bash
git add cli/
git commit -m "feat: CLI send command for executing remote commands"
```

---

## Task 12: CLI — List, Status, Kill Commands

**Files:**
- Create: `cli/src/commands/list.js`
- Create: `cli/src/commands/status.js`
- Create: `cli/src/commands/kill.js`
- Modify: `cli/bin/clawpeteer.js`

**Step 1: Implement list command**

`cli/src/commands/list.js`:
```javascript
import { MQTTClient } from '../mqtt-client.js';
import { loadConfig } from '../config.js';

export function registerListCommand(program) {
  program
    .command('list')
    .description('List connected remote agents')
    .option('-c, --config <path>', 'config file path')
    .action(async (options) => {
      const config = loadConfig(options.config);
      const mqtt = new MQTTClient(config);

      try {
        await mqtt.connect();

        const agents = new Map();
        await mqtt.subscribe('agents/+/heartbeat');
        await mqtt.subscribe('agents/registry');

        // Collect heartbeats for 3 seconds
        mqtt.on('message', (topic, msg) => {
          const match = topic.match(/agents\/([^/]+)\/heartbeat/);
          if (match) {
            agents.set(match[1], msg);
          }
        });

        await new Promise((r) => setTimeout(r, 3000));

        if (agents.size === 0) {
          console.log('No agents found.');
        } else {
          console.log('Connected agents:\n');
          for (const [id, info] of agents) {
            const status = info.status === 'online' ? 'online' : 'offline';
            const platform = info.platform || 'unknown';
            const arch = info.arch || 'unknown';
            const tasks = info.runningTasks || 0;
            console.log(`  ${id} (${platform}/${arch}) - ${status} - ${tasks} running tasks`);
          }
        }
      } catch (err) {
        console.error(`Failed: ${err.message}`);
        process.exit(1);
      } finally {
        mqtt.disconnect();
      }
    });
}
```

**Step 2: Implement status command**

`cli/src/commands/status.js`:
```javascript
import { MQTTClient } from '../mqtt-client.js';
import { loadConfig } from '../config.js';

export function registerStatusCommand(program) {
  program
    .command('status')
    .description('Query agent or task status')
    .argument('<agent>', 'target agent ID')
    .argument('[taskId]', 'specific task ID')
    .option('-c, --config <path>', 'config file path')
    .action(async (agent, taskId, options) => {
      const config = loadConfig(options.config);
      const mqtt = new MQTTClient(config);

      try {
        await mqtt.connect();
        await mqtt.subscribe(`agents/${agent}/results`);

        await mqtt.publish(`agents/${agent}/control/query`, {
          action: 'list',
        });

        const result = await mqtt.waitForMessage(
          new RegExp(`agents/${agent}/results`),
          (msg) => msg.action === 'list',
          10000,
        );

        if (result.tasks && result.tasks.length > 0) {
          console.log(`Tasks on ${agent}:\n`);
          for (const task of result.tasks) {
            const duration = task.endTime
              ? `${((new Date(task.endTime) - new Date(task.startTime)) / 1000).toFixed(1)}s`
              : `${((Date.now() - new Date(task.startTime).getTime()) / 1000).toFixed(1)}s (running)`;
            console.log(`  [${task.status}] ${task.id} - ${task.command} (${duration})`);
          }
        } else {
          console.log(`No tasks on ${agent}`);
        }
      } catch (err) {
        console.error(`Failed: ${err.message}`);
        process.exit(1);
      } finally {
        mqtt.disconnect();
      }
    });
}
```

**Step 3: Implement kill command**

`cli/src/commands/kill.js`:
```javascript
import { MQTTClient } from '../mqtt-client.js';
import { loadConfig } from '../config.js';

export function registerKillCommand(program) {
  program
    .command('kill')
    .description('Kill a running task on an agent')
    .argument('<agent>', 'target agent ID')
    .argument('<taskId>', 'task ID to kill')
    .option('--signal <signal>', 'signal to send', 'SIGTERM')
    .option('-c, --config <path>', 'config file path')
    .action(async (agent, taskId, options) => {
      const config = loadConfig(options.config);
      const mqtt = new MQTTClient(config);

      try {
        await mqtt.connect();
        await mqtt.subscribe(`agents/${agent}/results`);

        await mqtt.publish(`agents/${agent}/control/${taskId}`, {
          action: 'kill',
          signal: options.signal,
        });

        const result = await mqtt.waitForMessage(
          new RegExp(`agents/${agent}/results`),
          (msg) => msg.taskId === taskId && msg.status === 'killed',
          10000,
        );

        console.log(`Task ${taskId} killed on ${agent}`);
      } catch (err) {
        console.error(`Failed: ${err.message}`);
        process.exit(1);
      } finally {
        mqtt.disconnect();
      }
    });
}
```

**Step 4: Update CLI entry point**

Add imports and register all commands in `cli/bin/clawpeteer.js`.

**Step 5: Verify all commands show in help**

Run: `cd /Users/steven/AI_Playground/Clawpeteer/cli && node bin/clawpeteer.js --help`
Expected: Shows send, list, status, kill commands

**Step 6: Commit**

```bash
git add cli/
git commit -m "feat: CLI list, status, and kill commands"
```

---

## Task 13: CLI — Upload Command

**Files:**
- Create: `cli/src/file-transfer.js`
- Create: `cli/src/commands/upload.js`
- Modify: `cli/bin/clawpeteer.js`

**Step 1: Create file transfer module**

`cli/src/file-transfer.js`:
```javascript
import { createReadStream, statSync } from 'fs';
import { createHash } from 'crypto';
import { basename } from 'path';
import { v4 as uuidv4 } from 'uuid';

const CHUNK_SIZE = 256 * 1024; // 256KB

export async function calculateSha256(filePath) {
  return new Promise((resolve, reject) => {
    const hash = createHash('sha256');
    const stream = createReadStream(filePath);
    stream.on('data', (chunk) => hash.update(chunk));
    stream.on('end', () => resolve(hash.digest('hex')));
    stream.on('error', reject);
  });
}

export async function uploadFile(mqtt, agentId, localPath, remotePath, onProgress) {
  const stat = statSync(localPath);
  const fileSize = stat.size;
  const totalChunks = Math.ceil(fileSize / CHUNK_SIZE);
  const transferId = uuidv4();
  const filename = basename(localPath);
  const sha256 = await calculateSha256(localPath);

  // Subscribe to status
  await mqtt.subscribe(`agents/${agentId}/files/status`);

  // Send metadata
  await mqtt.publish(`agents/${agentId}/files/upload/${transferId}/meta`, {
    transferId,
    filename,
    destPath: remotePath,
    size: fileSize,
    chunkSize: CHUNK_SIZE,
    totalChunks,
    sha256,
    timestamp: Date.now(),
  });

  // Send chunks
  const stream = createReadStream(localPath, { highWaterMark: CHUNK_SIZE });
  let chunkIndex = 0;

  for await (const chunk of stream) {
    const data = chunk.toString('base64');
    await mqtt.publish(`agents/${agentId}/files/upload/${transferId}/chunks`, {
      transferId,
      chunkIndex,
      totalChunks,
      data,
      timestamp: Date.now(),
    });

    chunkIndex++;
    if (onProgress) {
      onProgress(chunkIndex, totalChunks);
    }
  }

  // Wait for completion status
  const result = await mqtt.waitForMessage(
    new RegExp(`agents/${agentId}/files/status`),
    (msg) => msg.transferId === transferId &&
      (msg.status === 'completed' || msg.status === 'error'),
    300000, // 5 min timeout for large files
  );

  return result;
}
```

**Step 2: Create upload command**

`cli/src/commands/upload.js`:
```javascript
import { existsSync } from 'fs';
import { MQTTClient } from '../mqtt-client.js';
import { loadConfig } from '../config.js';
import { uploadFile } from '../file-transfer.js';

export function registerUploadCommand(program) {
  program
    .command('upload')
    .description('Upload a file to a remote agent')
    .argument('<agent>', 'target agent ID')
    .argument('<localPath>', 'local file path')
    .argument('<remotePath>', 'remote destination path')
    .option('-c, --config <path>', 'config file path')
    .action(async (agent, localPath, remotePath, options) => {
      if (!existsSync(localPath)) {
        console.error(`File not found: ${localPath}`);
        process.exit(1);
      }

      const config = loadConfig(options.config);
      const mqtt = new MQTTClient(config);

      try {
        await mqtt.connect();

        console.log(`Uploading ${localPath} to ${agent}:${remotePath}...`);

        const result = await uploadFile(mqtt, agent, localPath, remotePath, (sent, total) => {
          const pct = ((sent / total) * 100).toFixed(1);
          process.stdout.write(`\rProgress: ${pct}% (${sent}/${total} chunks)`);
        });

        console.log('');
        if (result.verified) {
          console.log(`Upload complete. Checksum verified.`);
        } else {
          console.error('Upload complete but checksum verification failed!');
          process.exit(1);
        }
      } catch (err) {
        console.error(`\nUpload failed: ${err.message}`);
        process.exit(1);
      } finally {
        mqtt.disconnect();
      }
    });
}
```

**Step 3: Register in entry point and verify**

Run: `cd /Users/steven/AI_Playground/Clawpeteer/cli && node bin/clawpeteer.js upload --help`
Expected: Shows upload command help

**Step 4: Commit**

```bash
git add cli/
git commit -m "feat: CLI upload command with chunked file transfer"
```

---

## Task 14: CLI — Download Command

**Files:**
- Create: `cli/src/commands/download.js`
- Modify: `cli/src/file-transfer.js` — add download receiver
- Modify: `cli/bin/clawpeteer.js`

**Step 1: Add download receiver to file-transfer.js**

Add a `downloadFile` function that:
1. Sends a `file_download` command to the agent
2. Subscribes to `agents/{id}/files/download/{transfer-id}/meta` and `chunks`
3. Receives chunks, assembles file, verifies SHA-256

**Step 2: Create download command**

`cli/src/commands/download.js`:
```javascript
import { MQTTClient } from '../mqtt-client.js';
import { loadConfig } from '../config.js';
import { downloadFile } from '../file-transfer.js';

export function registerDownloadCommand(program) {
  program
    .command('download')
    .description('Download a file from a remote agent')
    .argument('<agent>', 'target agent ID')
    .argument('<remotePath>', 'remote file path')
    .argument('[localPath]', 'local destination path', '.')
    .option('-c, --config <path>', 'config file path')
    .action(async (agent, remotePath, localPath, options) => {
      const config = loadConfig(options.config);
      const mqtt = new MQTTClient(config);

      try {
        await mqtt.connect();
        console.log(`Downloading ${agent}:${remotePath}...`);

        const result = await downloadFile(mqtt, agent, remotePath, localPath, (received, total) => {
          const pct = ((received / total) * 100).toFixed(1);
          process.stdout.write(`\rProgress: ${pct}% (${received}/${total} chunks)`);
        });

        console.log('');
        if (result.verified) {
          console.log(`Download complete: ${result.localPath}`);
        } else {
          console.error('Download complete but checksum verification failed!');
          process.exit(1);
        }
      } catch (err) {
        console.error(`\nDownload failed: ${err.message}`);
        process.exit(1);
      } finally {
        mqtt.disconnect();
      }
    });
}
```

**Step 3: Register and verify**

Run: `cd /Users/steven/AI_Playground/Clawpeteer/cli && node bin/clawpeteer.js --help`
Expected: Shows all 6 commands: send, upload, download, list, status, kill

**Step 4: Commit**

```bash
git add cli/
git commit -m "feat: CLI download command with chunked receive and checksum"
```

---

## Task 15: OpenClaw Skill — SKILL.md

**Files:**
- Create: `skill/SKILL.md`

**Step 1: Create SKILL.md**

`skill/SKILL.md`:
```markdown
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
- "check disk space on home-pc" → `clawpeteer send home-pc "df -h"`
- "what processes are running on server" → `clawpeteer send server "ps aux"`
- "upload the backup to home-pc" → `clawpeteer upload home-pc <path> <dest>`
- "download logs from server" → `clawpeteer download server /var/log/app.log ./`
- "list all agents" / "show connected machines" → `clawpeteer list`
- "stop task X" / "kill task X" → `clawpeteer kill <agent> <task-id>`
- "run X in background on Y" → `clawpeteer send Y "X" --background`

## Notes

- Use `--stream` for long-running commands to see output in real-time
- Use `--background` for commands you don't need to wait for
- File transfers use chunked transfer with SHA-256 verification
- Agents report heartbeat every 30 seconds; offline agents are marked automatically
```

**Step 2: Commit**

```bash
git add skill/
git commit -m "feat: OpenClaw SKILL.md for agent integration"
```

---

## Task 16: Go Agent — Cross-Platform Build + Install Scripts

**Files:**
- Create: `agent/Makefile`
- Create: `agent/scripts/install-linux.sh`
- Create: `agent/scripts/install-mac.sh`
- Create: `agent/scripts/install-win.bat`
- Create: `agent/systemd/clawpeteer-agent.service`

**Step 1: Create Makefile**

`agent/Makefile`:
```makefile
BINARY=clawpeteer-agent
VERSION=1.0.0

.PHONY: build build-all clean

build:
	go build -ldflags "-X main.Version=$(VERSION)" -o $(BINARY) .

build-all:
	GOOS=linux GOARCH=amd64 go build -ldflags "-X main.Version=$(VERSION)" -o dist/$(BINARY)-linux-amd64 .
	GOOS=linux GOARCH=arm64 go build -ldflags "-X main.Version=$(VERSION)" -o dist/$(BINARY)-linux-arm64 .
	GOOS=darwin GOARCH=amd64 go build -ldflags "-X main.Version=$(VERSION)" -o dist/$(BINARY)-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 go build -ldflags "-X main.Version=$(VERSION)" -o dist/$(BINARY)-darwin-arm64 .
	GOOS=windows GOARCH=amd64 go build -ldflags "-X main.Version=$(VERSION)" -o dist/$(BINARY)-windows-amd64.exe .

clean:
	rm -rf dist/ $(BINARY)

test:
	go test ./... -v
```

**Step 2: Create systemd service file**

`agent/systemd/clawpeteer-agent.service`:
```ini
[Unit]
Description=Clawpeteer MQTT Remote Agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/clawpeteer-agent --config /etc/clawpeteer/config.json
Restart=always
RestartSec=10
User=clawpeteer

[Install]
WantedBy=multi-user.target
```

**Step 3: Create install scripts**

`agent/scripts/install-linux.sh`:
```bash
#!/bin/bash
set -euo pipefail

echo "=== Clawpeteer Agent Install (Linux) ==="

BINARY="clawpeteer-agent"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/clawpeteer"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# Build or copy binary
if [ -f "$SCRIPT_DIR/../dist/$BINARY-linux-$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')" ]; then
    sudo cp "$SCRIPT_DIR/../dist/$BINARY-linux-$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')" "$INSTALL_DIR/$BINARY"
else
    echo "Building from source..."
    cd "$SCRIPT_DIR/.." && go build -o "$INSTALL_DIR/$BINARY" .
fi

sudo chmod +x "$INSTALL_DIR/$BINARY"

# Config
sudo mkdir -p "$CONFIG_DIR"
if [ ! -f "$CONFIG_DIR/config.json" ]; then
    sudo cp "$SCRIPT_DIR/../config.example.json" "$CONFIG_DIR/config.json"
    echo "Edit $CONFIG_DIR/config.json with your settings"
fi

# Create service user
sudo useradd -r -s /bin/false clawpeteer 2>/dev/null || true

# Install systemd service
sudo cp "$SCRIPT_DIR/../systemd/clawpeteer-agent.service" /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable clawpeteer-agent

echo ""
echo "=== Install Complete ==="
echo "1. Edit $CONFIG_DIR/config.json"
echo "2. Start: sudo systemctl start clawpeteer-agent"
echo "3. Check: sudo systemctl status clawpeteer-agent"
```

**Step 4: Make scripts executable**

Run: `chmod +x /Users/steven/AI_Playground/Clawpeteer/agent/scripts/*.sh`

**Step 5: Verify cross-compilation**

Run: `cd /Users/steven/AI_Playground/Clawpeteer/agent && GOOS=linux GOARCH=amd64 go build -o /dev/null .`
Expected: Builds without errors

**Step 6: Commit**

```bash
git add agent/Makefile agent/scripts/ agent/systemd/
git commit -m "feat: cross-platform build system and install scripts"
```

---

## Task 17: Integration Test — End-to-End

**Files:**
- Create: `test/integration.sh`

**Step 1: Create integration test script**

`test/integration.sh`:
```bash
#!/bin/bash
set -euo pipefail

echo "=== Clawpeteer Integration Test ==="
echo "Prerequisites: Mosquitto running on localhost:1883 with users 'openclaw' and 'test-agent'"

AGENT_DIR="$(dirname "$0")/../agent"
CLI_DIR="$(dirname "$0")/../cli"

# Build agent
echo "[1/5] Building agent..."
cd "$AGENT_DIR" && go build -o /tmp/clawpeteer-agent .

# Create test config for agent
cat > /tmp/agent-test-config.json << 'EOF'
{
  "agentId": "test-agent",
  "broker": {
    "url": "tcp://localhost:1883",
    "username": "test-agent",
    "password": "testpass"
  },
  "security": {
    "mode": "blacklist",
    "blacklist": ["rm -rf /"]
  },
  "heartbeatInterval": 5
}
EOF

# Create test config for CLI
cat > /tmp/cli-test-config.json << 'EOF'
{
  "brokerUrl": "mqtt://localhost:1883",
  "username": "openclaw",
  "password": "testpass"
}
EOF

# Start agent in background
echo "[2/5] Starting agent..."
/tmp/clawpeteer-agent --config /tmp/agent-test-config.json &
AGENT_PID=$!
sleep 2

# Test: list agents
echo "[3/5] Testing agent discovery..."
cd "$CLI_DIR" && timeout 10 node bin/clawpeteer.js list -c /tmp/cli-test-config.json || echo "(timeout expected, checking output)"

# Test: send command
echo "[4/5] Testing command execution..."
cd "$CLI_DIR" && node bin/clawpeteer.js send test-agent "echo hello-clawpeteer" -c /tmp/cli-test-config.json

# Cleanup
echo "[5/5] Cleaning up..."
kill $AGENT_PID 2>/dev/null || true
rm -f /tmp/clawpeteer-agent /tmp/agent-test-config.json /tmp/cli-test-config.json

echo ""
echo "=== Tests Complete ==="
```

**Step 2: Make executable**

Run: `chmod +x /Users/steven/AI_Playground/Clawpeteer/test/integration.sh`

**Step 3: Commit**

```bash
git add test/
git commit -m "feat: integration test script for end-to-end verification"
```

---

## Task 18: Documentation

**Files:**
- Create: `README.md`
- Create: `docs/setup-broker.md`
- Create: `docs/install-agent.md`

**Step 1: Create README.md**

Concise project overview with:
- What it does (1 paragraph)
- Architecture diagram (text)
- Quick start (5 steps)
- Links to detailed docs

**Step 2: Create setup-broker.md**

Step-by-step Mosquitto setup guide covering:
- Installation (brew/apt)
- TLS cert generation
- User/ACL configuration
- Testing

**Step 3: Create install-agent.md**

Agent installation for each platform:
- Download binary or build from source
- Config file setup
- Service registration (systemd/launchd)
- Verification

**Step 4: Commit**

```bash
git add README.md docs/
git commit -m "docs: README, broker setup, and agent installation guides"
```

---

## Summary

| Task | Component | Description |
|------|-----------|-------------|
| 1 | All | Project scaffolding |
| 2 | Broker | Mosquitto config + setup script |
| 3 | Agent | Config + MQTT connection |
| 4 | Agent | Command executor (sync/spawn) |
| 5 | Agent | Task manager |
| 6 | Agent | Security module |
| 7 | Agent | Message handler + heartbeat |
| 8 | Agent | File transfer receiver |
| 9 | Agent | Wire file transfer into handler |
| 10 | CLI | MQTT client + config |
| 11 | CLI | Send command |
| 12 | CLI | List, status, kill commands |
| 13 | CLI | Upload command + file transfer |
| 14 | CLI | Download command |
| 15 | Skill | SKILL.md for OpenClaw |
| 16 | Agent | Cross-platform build + install |
| 17 | Test | Integration test |
| 18 | Docs | README + guides |
