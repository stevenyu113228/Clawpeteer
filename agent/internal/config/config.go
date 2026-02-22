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

// Defaults returns a Config with default values applied.
func Defaults() *Config {
	return &Config{
		HeartbeatInterval: 30,
		Security: SecurityConfig{
			Mode:            "blacklist",
			MaxFileSize:     1073741824, // 1GB
			MaxConcTransfer: 3,
		},
	}
}

// Load reads and parses a config file from disk.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	return Parse(data)
}

// Parse parses config from raw JSON bytes.
func Parse(data []byte) (*Config, error) {
	cfg := Defaults()

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	return cfg, nil
}

// Validate checks that required fields are present.
func (c *Config) Validate() error {
	if c.AgentID == "" {
		return fmt.Errorf("agentId is required (use --id or config)")
	}
	if c.Broker.URL == "" {
		return fmt.Errorf("broker url is required (use --broker or config)")
	}
	return nil
}
