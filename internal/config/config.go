package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	DefaultKey     string `json:"default_key,omitempty"`
	DefaultNetwork string `json:"default_network,omitempty"`
	GasBuffer      int    `json:"gas_buffer,omitempty"`
}

func defaultPath() (string, error) {
	if p := os.Getenv("GNO_MCP_CONFIG"); p != "" {
		return p, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "gno-mcp", "config.json"), nil
}

func Load() (*Config, error) {
	p, err := defaultPath()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, err
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

func Save(c *Config) error {
	p, err := defaultPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, b, 0o600)
}
