package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config represents the ralph configuration
type Config struct {
	LLM    LLMConfig    `mapstructure:"llm"`
	Claude ClaudeConfig `mapstructure:"claude"`
	Build  BuildConfig  `mapstructure:"build"`
}

// LLMConfig contains LLM backend settings
type LLMConfig struct {
	Backend string `mapstructure:"backend"`
	Model   string `mapstructure:"model"`
}

// ClaudeConfig contains Claude-specific settings
type ClaudeConfig struct {
	Binary       string   `mapstructure:"binary"`
	AllowedTools []string `mapstructure:"allowed_tools"`
}

// BuildConfig contains build/execution settings
type BuildConfig struct {
	DefaultLoopIterations int `mapstructure:"default_loop_iterations"`
}

// Load reads the config from the workspace
func Load(workspaceDir string) (*Config, error) {
	configPath := filepath.Join(workspaceDir, ".ralph", "config.yaml")

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return DefaultConfig(), nil
	}

	v := viper.New()
	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Apply defaults for missing values
	applyDefaults(&cfg)

	return &cfg, nil
}

// DefaultConfig returns a config with default values
func DefaultConfig() *Config {
	return &Config{
		LLM: LLMConfig{
			Backend: "claude",
			Model:   "sonnet",
		},
		Claude: ClaudeConfig{
			Binary: "claude",
			AllowedTools: []string{
				"Read", "Write", "Edit", "Bash", "Glob", "Grep",
				"Task", "TodoWrite", "WebFetch", "WebSearch",
			},
		},
		Build: BuildConfig{
			DefaultLoopIterations: 10,
		},
	}
}

func applyDefaults(cfg *Config) {
	defaults := DefaultConfig()

	if cfg.LLM.Backend == "" {
		cfg.LLM.Backend = defaults.LLM.Backend
	}
	if cfg.LLM.Model == "" {
		cfg.LLM.Model = defaults.LLM.Model
	}
	if cfg.Claude.Binary == "" {
		cfg.Claude.Binary = defaults.Claude.Binary
	}
	if len(cfg.Claude.AllowedTools) == 0 {
		cfg.Claude.AllowedTools = defaults.Claude.AllowedTools
	}
	if cfg.Build.DefaultLoopIterations == 0 {
		cfg.Build.DefaultLoopIterations = defaults.Build.DefaultLoopIterations
	}
}
