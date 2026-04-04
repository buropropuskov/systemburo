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
	JWTSecret        string `env:"JWT_SECRET" envDefault:"dev-secret-change-me-in-production!"`
	JWTRefreshSecret string `env:"JWT_REFRESH_SECRET" envDefault:"dev-refresh-secret-change-me-now!"`
	LogLevel         string `env:"LOG_LEVEL" envDefault:"info"`

	CORSAllowedOrigins      []string `env:"CORS_ALLOWED_ORIGINS" envDefault:"*" envSeparator:","`
	UploadMaxFileSize       int64    `env:"UPLOAD_MAX_FILE_SIZE" envDefault:"10485760"`
	UploadAllowedImageTypes []string `env:"UPLOAD_ALLOWED_IMAGE_TYPES" envDefault:"image/jpeg,image/png,image/webp" envSeparator:","`
	UploadAllowedDocTypes   []string `env:"UPLOAD_ALLOWED_DOC_TYPES" envDefault:"application/pdf,application/vnd.openxmlformats-officedocument.wordprocessingml.document" envSeparator:","`
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
	if !strings.HasPrefix(c.DatabaseURL, "postgres://") && !strings.HasPrefix(c.DatabaseURL, "postgresql://") {
		return fmt.Errorf("DATABASE_URL must be a PostgreSQL connection string")
	}
	switch c.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("LOG_LEVEL must be one of: debug, info, warn, error (got %q)", c.LogLevel)
	}
	if c.UploadMaxFileSize <= 0 {
		return fmt.Errorf("UPLOAD_MAX_FILE_SIZE must be positive (got %d)", c.UploadMaxFileSize)
	}
	return nil
}
