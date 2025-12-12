package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Vault    VaultConfig    `yaml:"vault"`
	Telegram TelegramConfig `yaml:"telegram"`
	Server   ServerConfig   `yaml:"server"`
	ApiKeys  []APIKey       `yaml:"api_keys"`
}

type ServerConfig struct {
	ListenAddress string `yaml:"listen_address"`
}

type APIKey struct {
	Key          string   `yaml:"key"`
	PathPrefix   string   `yaml:"path_prefix"`
	AllowedCIDRs []string `yaml:"allowed_cidrs"`
}

type VaultConfig struct {
	Address    string `yaml:"address"`
	Token      string `yaml:"token"`
	MountPath  string `yaml:"mount_path"`  // e.g. "secret"
	SecretPath string `yaml:"secret_path"` // e.g. "my-secret"
}

type TelegramConfig struct {
	BotToken string `yaml:"bot_token"`
	ChatID   int64  `yaml:"chat_id"`
}

func Load(path string) (*Config, error) {
	if path == "" {
		path = "config.yaml"
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &cfg, nil
}
