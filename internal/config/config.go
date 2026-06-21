package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/caarlos0/env/v11"

	"systemburo/internal/crypto"
)

type Config struct {
	DatabaseURL      string        `env:"DATABASE_URL,required"`
	BindHost         string        `env:"BIND_HOST" envDefault:"0.0.0.0"`
	BindPort         string        `env:"BIND_PORT" envDefault:"8090"`
	JWTSecret        string        `env:"JWT_SECRET,required"`
	JWTRefreshSecret string        `env:"JWT_REFRESH_SECRET,required"`
	JWTAccessTTL     time.Duration `env:"JWT_ACCESS_TTL" envDefault:"15m"`
	JWTRefreshTTL    time.Duration `env:"JWT_REFRESH_TTL" envDefault:"168h"`
	LogLevel         string        `env:"LOG_LEVEL" envDefault:"info"`
	SwaggerEnabled   bool          `env:"SWAGGER_ENABLED" envDefault:"false"`

	// Файловое логирование с ротацией (lumberjack). LogFilePath пустой - пишем только
	// в stdout (как раньше). При заданном пути логи идут и в stdout (docker logs),
	// и в ротируемый файл. LogMaxAgeDays=30 - месячная ротация по времени.
	LogFilePath   string `env:"LOG_FILE_PATH" envDefault:""`
	LogMaxSizeMB  int    `env:"LOG_MAX_SIZE_MB" envDefault:"100"`
	LogMaxAgeDays int    `env:"LOG_MAX_AGE_DAYS" envDefault:"30"`
	LogMaxBackups int    `env:"LOG_MAX_BACKUPS" envDefault:"14"`
	LogCompress   bool   `env:"LOG_COMPRESS" envDefault:"true"`

	CORSAllowedOrigins      []string `env:"CORS_ALLOWED_ORIGINS" envDefault:"http://localhost:8081" envSeparator:","`
	UploadMaxFileSize       int64    `env:"UPLOAD_MAX_FILE_SIZE" envDefault:"10485760"`
	UploadAllowedImageTypes []string `env:"UPLOAD_ALLOWED_IMAGE_TYPES" envDefault:"image/jpeg,image/png,image/webp" envSeparator:","`
	UploadAllowedDocTypes   []string `env:"UPLOAD_ALLOWED_DOC_TYPES" envDefault:"application/pdf,application/vnd.openxmlformats-officedocument.wordprocessingml.document" envSeparator:","`

	DataEncryptionKey  string `env:"DATA_ENCRYPTION_KEY" envDefault:""`
	RequireEncryption  bool   `env:"REQUIRE_ENCRYPTION" envDefault:"false"`
	RateLimitPerMinute int    `env:"RATE_LIMIT_PER_MINUTE" envDefault:"200"`
	RateLimitWindowSec int64  `env:"RATE_LIMIT_WINDOW_SEC" envDefault:"60"`

	// LoginRateLimit ограничивает попытки /login per-IP (защита от brute-force).
	// Дефолт 10/5м: лояльно к опечаткам пароля живых юзеров, но всё ещё
	// блокирует brute-force - Argon2id и так растягивает каждую попытку
	// на 100мс+, 10 попыток за 5 минут это ~17 запросов/час максимум.
	// В CI/e2e ставим LOGIN_RATE_LIMIT_MAX=1000.
	LoginRateLimitMax       int    `env:"LOGIN_RATE_LIMIT_MAX" envDefault:"10"`
	LoginRateLimitWindowSec int64  `env:"LOGIN_RATE_LIMIT_WINDOW_SEC" envDefault:"300"`
	PaginationMaxLimit      int    `env:"PAGINATION_MAX_LIMIT" envDefault:"100"`
	UploadPath              string `env:"UPLOAD_PATH" envDefault:"./uploads"`

	// CookieSecure управляет флагом Secure на refresh-cookie. На staging/prod
	// всегда true (HTTPS). На локальной разработке (http://localhost) - false,
	// иначе браузер не отправит cookie.
	CookieSecure bool `env:"COOKIE_SECURE" envDefault:"true"`

	// Telegram bot для bug-report-ов со страницы Error500. Оба поля опциональные:
	// если пустые - репорты пишутся только в БД, TG-отправка пропускается (warn-лог).
	TelegramBotToken string `env:"TELEGRAM_BOT_TOKEN" envDefault:""`
	TelegramChatID   string `env:"TELEGRAM_CHAT_ID" envDefault:""`

	// ResetTimezone задаёт часовой пояс для ежедневного сброса территориальных статусов.
	// Используется для расчёта 06:00 локального времени. По умолчанию Europe/Moscow.
	ResetTimezone string `env:"RESET_TIMEZONE" envDefault:"Europe/Moscow"`

	// AnalyticsCacheRefreshSec - интервал обновления тёплого кэша аналитики дашборда
	// (in-memory + снимок в БД для прогрева после рестарта). 0 отключает кэш.
	AnalyticsCacheRefreshSec int `env:"ANALYTICS_CACHE_REFRESH_SEC" envDefault:"60"`

	// Партиционирование request_logs: детально храним RequestLogDetailDays дней
	// (партиции старше сворачиваются в дневные агрегаты и дропаются), партиции
	// создаём на RequestLogPartitionPrecreateDays вперёд.
	RequestLogDetailDays             int `env:"REQUEST_LOG_DETAIL_DAYS" envDefault:"30"`
	RequestLogPartitionPrecreateDays int `env:"REQUEST_LOG_PARTITION_PRECREATE_DAYS" envDefault:"7"`

	// PdAuditRetentionMonths - срок хранения аудита ПД (152-ФЗ): партиции старше
	// дропаются. По умолчанию 36 месяцев (3 года).
	PdAuditRetentionMonths int `env:"PD_AUDIT_RETENTION_MONTHS" envDefault:"36"`
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
	if c.RequireEncryption && c.DataEncryptionKey == "" {
		return fmt.Errorf("REQUIRE_ENCRYPTION=true but DATA_ENCRYPTION_KEY is empty")
	}
	if c.DataEncryptionKey != "" {
		if _, err := crypto.ParseHexKey(c.DataEncryptionKey); err != nil {
			return fmt.Errorf("DATA_ENCRYPTION_KEY: %w", err)
		}
	}
	if c.RateLimitPerMinute <= 0 {
		return fmt.Errorf("RATE_LIMIT_PER_MINUTE must be positive (got %d)", c.RateLimitPerMinute)
	}
	if c.PaginationMaxLimit <= 0 {
		return fmt.Errorf("PAGINATION_MAX_LIMIT must be positive (got %d)", c.PaginationMaxLimit)
	}
	if c.JWTAccessTTL <= 0 {
		return fmt.Errorf("JWT_ACCESS_TTL must be positive (got %s)", c.JWTAccessTTL)
	}
	if c.JWTRefreshTTL <= c.JWTAccessTTL {
		return fmt.Errorf("JWT_REFRESH_TTL (%s) must be greater than JWT_ACCESS_TTL (%s)", c.JWTRefreshTTL, c.JWTAccessTTL)
	}
	return nil
}
