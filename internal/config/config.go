package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	Bot      BotConfig      `mapstructure:"bot"`
	Parser   ParserConfig   `mapstructure:"parser"`
	Database DatabaseConfig `mapstructure:"database"`
}

type BotConfig struct {
	Token string `mapstructure:"token"`
}

type ParserConfig struct{}

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
