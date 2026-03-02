package config

import (
	"fmt"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

type Config struct {
	mu       sync.RWMutex
	Bot      BotConfig      `mapstructure:"bot"`
	Parser   ParserConfig   `mapstructure:"parser"`
	Database DatabaseConfig `mapstructure:"database"`
	Logs     LogsConfig     `mapstructure:"logs"`
}

type BotConfig struct {
	Token string `mapstructure:"token"`
}

type ParserConfig struct {
	Rules []Rule `mapstructure:"rules"`
}

type LogsConfig struct {
	Path string `mapstructure:"path"`
}

type Rule struct {
	Name    string `mapstructure:"name"`
	Pattern string `mapstructure:"pattern"`
}

type DatabaseConfig struct {
	Path string `mapstructure:"path"`
}

func Load(path string) (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(path)
	viper.AddConfigPath(".")

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}

func (c *Config) Watch(onChange func(*Config)) {
	viper.OnConfigChange(func(in fsnotify.Event) {
		var cfg Config
		if err := viper.Unmarshal(&cfg); err != nil {
			return
		}
		c.mu.Lock()
		c.Bot = cfg.Bot
		c.Parser = cfg.Parser
		c.Database = cfg.Database
		c.Logs = cfg.Logs
		c.mu.Unlock()
		onChange(&cfg)
	})
	viper.WatchConfig()
}

func (c *Config) Get() *Config {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c
}
