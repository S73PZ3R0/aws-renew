package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type TelegramConfig struct {
	BotToken string `yaml:"bot_token"`
	ChatID   string `yaml:"chat_id"`
}

type NotifyConfig struct {
	WebhookURL string         `yaml:"webhook_url"`
	Telegram   TelegramConfig `yaml:"telegram"`
}

type Config struct {
	Profile         string       `yaml:"profile"`
	Regions         string       `yaml:"regions"`       // comma-separated or "all"
	SSHPort         string       `yaml:"ssh_port"`      // comma-separated ports
	Instances       []string     `yaml:"instances"`     // IDs or Name tags to watch
	PollInterval    string       `yaml:"poll_interval"` // e.g. "5m"
	Cleanup         bool         `yaml:"cleanup"`
	DryRun          bool         `yaml:"dry_run"`
	RuleDescription string       `yaml:"rule_description"`
	LogFile         string       `yaml:"log_file"`
	Notify          NotifyConfig `yaml:"notify"`
}

func Default() *Config {
	return &Config{
		PollInterval:    "5m",
		RuleDescription: "Auto-updated by aws-renew",
	}
}

func Path() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(dir, "aws-renew", "config.yaml")
}

func Load() (*Config, error) {
	return LoadFrom(Path())
}

func LoadFrom(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Default(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}
	cfg := Default()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	return cfg, nil
}

func Save(cfg *Config) error {
	path := Path()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}
