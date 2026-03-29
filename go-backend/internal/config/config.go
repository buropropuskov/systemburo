package config

import (
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
	return cfg, nil
}
