package config

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
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
	UploadAllowedDocTypes   []string `env:"UPLOAD_ALLOWED_DOC_TYPES" envDefault:"application/pdf,application/vnd.openxmlformats-officedocument.wordprocessingml.document,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" envSeparator:","`

	// Файлы, прикладываемые к заявке (#1721). Размер одного файла берётся из
	// UPLOAD_MAX_FILE_SIZE, здесь - сколько их на заявку и сколько всего. Потолок
	// суммы нужен отдельно от количества: десять файлов по десять мегабайт
	// упрутся в client_max_body_size nginx и оборвутся уже на прокси.
	ApplicationFileMaxCount int   `env:"APPLICATION_FILE_MAX_COUNT" envDefault:"10"`
	ApplicationFileMaxTotal int64 `env:"APPLICATION_FILE_MAX_TOTAL_SIZE" envDefault:"31457280"`
	// ApplicationFileDraftTTL - сколько живёт загруженный, но так и не приложенный
	// к заявке файл: заявитель выбрал файлы и закрыл форму, не отправив её.
	ApplicationFileDraftTTL time.Duration `env:"APPLICATION_FILE_DRAFT_TTL" envDefault:"24h"`
	// Приведение снимков к предсказуемому виду (#1721). Перекодирование заодно
	// срезает EXIF: снимок с телефона несёт координаты съёмки и модель устройства.
	ApplicationFileImageMaxSide int `env:"APPLICATION_FILE_IMAGE_MAX_SIDE" envDefault:"2000"`
	ApplicationFileJPEGQuality  int `env:"APPLICATION_FILE_JPEG_QUALITY" envDefault:"82"`

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
	LoginRateLimitWindowSec int64  `env:"LOGIN_RATE_LIMIT_WINDOW_SEC" envDefault:"60"`
	PaginationMaxLimit      int    `env:"PAGINATION_MAX_LIMIT" envDefault:"100"`
	UploadPath              string `env:"UPLOAD_PATH" envDefault:"./uploads"`

	// ArchivePath - корень файлового архива бланков (#1615): заполненные .xlsx
	// раскладываются под ним по годам, месяцам и дням.
	//
	// Каталог обязан лежать ВНЕ UploadPath. Содержимое UploadPath раздаётся
	// статикой до проверки авторизации (router.go, api.Static("/uploads")), а в
	// бланке паспортные данные и патенты - те самые поля, которые в базе хранятся
	// зашифрованными. Архив внутри загрузок означал бы их доступность по прямой
	// ссылке кому угодно. Проверку делает Validate, и она отказывает в старте.
	//
	// На проде монтируется bind-mount-ом с отдельного раздела: путь должен быть
	// предсказуемым, чтобы его можно было зашифровать, отдать в сетевую папку
	// только на чтение и включить в резервное копирование.
	ArchivePath string `env:"ARCHIVE_PATH" envDefault:"./archive"`

	// ArchiveWorkerTick - как часто фоновый воркер разбирает очередь выгрузки.
	ArchiveWorkerTick time.Duration `env:"ARCHIVE_WORKER_TICK" envDefault:"15s"`

	// ArchiveSweepInterval - как часто подметаются заявки, для которых очередь
	// потеряна: постановка в неё идёт после коммита и намеренно best-effort, чтобы
	// выгрузка на диск не могла уронить подачу заявки.
	ArchiveSweepInterval time.Duration `env:"ARCHIVE_SWEEP_INTERVAL" envDefault:"5m"`

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

	// Суточная уборка технического мусора (#1614). Считается от момента, когда
	// запись обесценилась: у токена - от истечения или отзыва, у уведомления - от
	// создания при снятой отметке непрочитанного. Остальные журналы автоматика не
	// трогает, для них есть подкоманда cleanup.
	RefreshTokenRetentionDays     int `env:"REFRESH_TOKEN_RETENTION_DAYS" envDefault:"30"`
	ReadNotificationRetentionDays int `env:"READ_NOTIFICATION_RETENTION_DAYS" envDefault:"30"`
	// NotificationRetentionDays - срок непрочитанных уведомлений (#1748, S9). Дольше
	// прочитанных нарочно: непрочитанное не обесценилось само по себе (человек его ещё
	// не видел), поэтому порог заметно мягче, а не совпадает с ReadNotificationRetentionDays.
	NotificationRetentionDays int `env:"NOTIFICATION_RETENTION_DAYS" envDefault:"90"`
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
	if c.ApplicationFileMaxCount <= 0 {
		return fmt.Errorf("APPLICATION_FILE_MAX_COUNT must be positive (got %d)", c.ApplicationFileMaxCount)
	}
	if c.ApplicationFileMaxTotal < c.UploadMaxFileSize {
		return fmt.Errorf("APPLICATION_FILE_MAX_TOTAL_SIZE (%d) must not be less than UPLOAD_MAX_FILE_SIZE (%d): ни один файл нельзя было бы приложить", c.ApplicationFileMaxTotal, c.UploadMaxFileSize)
	}
	if c.ApplicationFileDraftTTL <= 0 {
		return fmt.Errorf("APPLICATION_FILE_DRAFT_TTL must be positive (got %s)", c.ApplicationFileDraftTTL)
	}
	if c.ApplicationFileImageMaxSide <= 0 {
		return fmt.Errorf("APPLICATION_FILE_IMAGE_MAX_SIDE must be positive (got %d)", c.ApplicationFileImageMaxSide)
	}
	if c.ApplicationFileJPEGQuality < 1 || c.ApplicationFileJPEGQuality > 100 {
		return fmt.Errorf("APPLICATION_FILE_JPEG_QUALITY must be within 1..100 (got %d)", c.ApplicationFileJPEGQuality)
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
	if c.ArchiveWorkerTick <= 0 {
		return fmt.Errorf("ARCHIVE_WORKER_TICK must be positive (got %s)", c.ArchiveWorkerTick)
	}
	if c.ArchiveSweepInterval <= 0 {
		return fmt.Errorf("ARCHIVE_SWEEP_INTERVAL must be positive (got %s)", c.ArchiveSweepInterval)
	}
	if err := validateArchiveOutsideUploads(c.ArchivePath, c.UploadPath); err != nil {
		return err
	}
	return nil
}

// validateArchiveOutsideUploads не даёт запуститься с каталогом архива внутри
// каталога загрузок (или наоборот).
//
// Загрузки раздаются статикой до проверки авторизации, поэтому архив внутри них
// сделал бы заполненные бланки с персональными данными доступными по прямой ссылке.
// Это отказ в старте, а не предупреждение в лог: молча работающая система с утечкой
// хуже упавшей, а предупреждение при развёртывании никто не прочитает.
func validateArchiveOutsideUploads(archivePath, uploadPath string) error {
	if archivePath == "" || uploadPath == "" {
		return nil
	}

	archiveAbs, err := resolvePath(archivePath)
	if err != nil {
		return fmt.Errorf("ARCHIVE_PATH: %w", err)
	}
	uploadAbs, err := resolvePath(uploadPath)
	if err != nil {
		return fmt.Errorf("UPLOAD_PATH: %w", err)
	}

	switch {
	case archiveAbs == uploadAbs:
		return fmt.Errorf("ARCHIVE_PATH (%s) must differ from UPLOAD_PATH: uploads are served without authorization", archiveAbs)
	case isInside(archiveAbs, uploadAbs):
		return fmt.Errorf("ARCHIVE_PATH (%s) must not be inside UPLOAD_PATH (%s): uploads are served without authorization, blanks contain personal data", archiveAbs, uploadAbs)
	case isInside(uploadAbs, archiveAbs):
		return fmt.Errorf("UPLOAD_PATH (%s) must not be inside ARCHIVE_PATH (%s)", uploadAbs, archiveAbs)
	}
	return nil
}

// resolvePath приводит путь к абсолютному и разворачивает символические ссылки.
//
// Без разворачивания проверка сравнивала бы лексические пути, и каталог архива,
// подложенный ссылкой внутрь загрузок, прошёл бы её - то есть ровно тот случай, от
// которого весь этот код и защищает.
//
// Каталога может ещё не быть: при первом развёртывании приложение стартует до того,
// как оператор создаст его руками. Поэтому разворачиваем ближайшего существующего
// предка и приклеиваем остаток пути.
func resolvePath(p string) (string, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}

	rest := ""
	for cur := abs; ; {
		resolved, err := filepath.EvalSymlinks(cur)
		if err == nil {
			return filepath.Join(resolved, rest), nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			// Развернуть не дали по другой причине (например, нет прав на чтение
			// промежуточного каталога). Сравним хотя бы лексические пути: неполная
			// проверка лучше, чем отказ стартовать из-за особенностей монтирования.
			return abs, nil
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return abs, nil
		}
		rest = filepath.Join(filepath.Base(cur), rest)
		cur = parent
	}
}

// isInside сообщает, лежит ли child под parent. Оба пути должны быть абсолютными.
func isInside(child, parent string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	if rel == "." || rel == ".." {
		return false
	}
	return !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
