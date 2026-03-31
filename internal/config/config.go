package config

import (
	"fmt"
	"strings"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	DatabaseURL      string `env:"DATABASE_URL,required"`
	BindHost         string `env:"BIND_HOST" envDefault:"0.0.0.0"`
	BindPort         string `env:"BIND_PORT" envDefault:"8090"`
	JWTSecret        string `env:"JWT_SECRET" envDefault:"dev-secret-change-me"`
	JWTRefreshSecret string `env:"JWT_REFRESH_SECRET" envDefault:"dev-refresh-secret-change-me"`
	LogLevel         string `env:"LOG_LEVEL" envDefault:"info"`
}

func Load() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation: %w", err)
	}
	return cfg, nil
}

// Validate checks configuration values for correctness.
func (c *Config) Validate() error {
	if len(c.JWTSecret) < 32 {
		return fmt.Errorf("JWT_SECRET must be at least 32 characters (got %d)", len(c.JWTSecret))
	}
	if len(c.JWTRefreshSecret) < 32 {
		return fmt.Errorf("JWT_REFRESH_SECRET must be at least 32 characters (got %d)", len(c.JWTRefreshSecret))
	}
	if !strings.HasPrefix(c.DatabaseURL, "postgres") {
		return fmt.Errorf("DATABASE_URL must be a PostgreSQL connection string")
	}
	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLevels[c.LogLevel] {
		return fmt.Errorf("LOG_LEVEL must be one of: debug, info, warn, error (got %q)", c.LogLevel)
	}
	return nil
}
