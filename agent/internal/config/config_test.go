package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}
	return path
}

func TestLoadFullConfig(t *testing.T) {
	content := `{
		"agentId": "test-agent",
		"broker": {
			"url": "mqtts://broker.example.com:8883",
			"username": "user",
			"password": "pass",
			"caFile": "/path/to/ca.crt"
		},
		"security": {
			"mode": "whitelist",
			"whitelist": ["ls", "cat", "echo"],
			"blacklist": [],
			"uploadDirs": ["/tmp/uploads"],
			"downloadDirs": ["/home/user"],
			"maxFileSize": 5242880,
			"maxConcurrentTransfers": 5
		},
		"heartbeatInterval": 60,
		"logDir": "/var/log/clawpeteer"
	}`

	path := writeTempConfig(t, content)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.AgentID != "test-agent" {
		t.Errorf("AgentID = %q, want %q", cfg.AgentID, "test-agent")
	}
	if cfg.Broker.URL != "mqtts://broker.example.com:8883" {
		t.Errorf("Broker.URL = %q, want %q", cfg.Broker.URL, "mqtts://broker.example.com:8883")
	}
	if cfg.Broker.Username != "user" {
		t.Errorf("Broker.Username = %q, want %q", cfg.Broker.Username, "user")
	}
	if cfg.Broker.Password != "pass" {
		t.Errorf("Broker.Password = %q, want %q", cfg.Broker.Password, "pass")
	}
	if cfg.Broker.CAFile != "/path/to/ca.crt" {
		t.Errorf("Broker.CAFile = %q, want %q", cfg.Broker.CAFile, "/path/to/ca.crt")
	}
	if cfg.Security.Mode != "whitelist" {
		t.Errorf("Security.Mode = %q, want %q", cfg.Security.Mode, "whitelist")
	}
	if len(cfg.Security.Whitelist) != 3 {
		t.Errorf("Security.Whitelist length = %d, want 3", len(cfg.Security.Whitelist))
	}
	if cfg.Security.MaxFileSize != 5242880 {
		t.Errorf("Security.MaxFileSize = %d, want 5242880", cfg.Security.MaxFileSize)
	}
	if cfg.Security.MaxConcTransfer != 5 {
		t.Errorf("Security.MaxConcTransfer = %d, want 5", cfg.Security.MaxConcTransfer)
	}
	if cfg.HeartbeatInterval != 60 {
		t.Errorf("HeartbeatInterval = %d, want 60", cfg.HeartbeatInterval)
	}
	if cfg.LogDir != "/var/log/clawpeteer" {
		t.Errorf("LogDir = %q, want %q", cfg.LogDir, "/var/log/clawpeteer")
	}
}

func TestLoadMinimalConfig(t *testing.T) {
	content := `{
		"agentId": "minimal-agent",
		"broker": {
			"url": "mqtt://localhost:1883",
			"username": "user",
			"password": "pass"
		}
	}`

	path := writeTempConfig(t, content)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.HeartbeatInterval != 30 {
		t.Errorf("HeartbeatInterval = %d, want default 30", cfg.HeartbeatInterval)
	}
	if cfg.Security.Mode != "blacklist" {
		t.Errorf("Security.Mode = %q, want default %q", cfg.Security.Mode, "blacklist")
	}
	if cfg.Security.MaxFileSize != 1073741824 {
		t.Errorf("Security.MaxFileSize = %d, want default 1073741824", cfg.Security.MaxFileSize)
	}
	if cfg.Security.MaxConcTransfer != 3 {
		t.Errorf("Security.MaxConcTransfer = %d, want default 3", cfg.Security.MaxConcTransfer)
	}
}

func TestLoadMissingAgentID(t *testing.T) {
	content := `{
		"broker": {
			"url": "mqtt://localhost:1883",
			"username": "user",
			"password": "pass"
		}
	}`

	path := writeTempConfig(t, content)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for missing agentId, got nil")
	}
	if err.Error() != "agentId is required" {
		t.Errorf("error = %q, want %q", err.Error(), "agentId is required")
	}
}

func TestLoadMissingBrokerURL(t *testing.T) {
	content := `{
		"agentId": "test-agent",
		"broker": {
			"username": "user",
			"password": "pass"
		}
	}`

	path := writeTempConfig(t, content)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for missing broker.url, got nil")
	}
	if err.Error() != "broker.url is required" {
		t.Errorf("error = %q, want %q", err.Error(), "broker.url is required")
	}
}

func TestLoadInvalidJSON(t *testing.T) {
	content := `{not valid json}`

	path := writeTempConfig(t, content)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load("/nonexistent/path/config.json")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}
