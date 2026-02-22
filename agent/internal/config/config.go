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
