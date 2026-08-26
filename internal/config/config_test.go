package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validConfig() *Config {
	return &Config{
		DatabaseURL:        "postgres://user:pass@localhost/db",
		JWTSecret:          "test-jwt-secret-that-is-at-least-32-chars!!",
		JWTRefreshSecret:   "test-jwt-refresh-secret-at-least-32-chars!",
		JWTAccessTTL:       15 * time.Minute,
		JWTRefreshTTL:      168 * time.Hour,
		LogLevel:           "info",
		UploadMaxFileSize:  10485760,
		RateLimitPerMinute: 200,
		PaginationMaxLimit: 100,
		// Интервалы воркера архива проверяются на положительность, поэтому без них
		// помощник перестал бы соответствовать своему имени.
		ArchiveWorkerTick:    15 * time.Second,
		ArchiveSweepInterval: 5 * time.Minute,
		// Лимиты файлов заявки тоже проверяются на положительность (#1721).
		ApplicationFileMaxCount:     10,
		ApplicationFileMaxTotal:     31457280,
		ApplicationFileDraftTTL:     24 * time.Hour,
		ApplicationFileImageMaxSide: 2000,
		ApplicationFileJPEGQuality:  82,
		// Срок хранения подписок Web Push тоже проверяется на положительность (#974).
		PushSubscriptionRetentionDays: 180,
	}
}

// TestValidate_ValidConfigPasses стережёт сам помощник: он называется validConfig и
// используется как заведомо корректная основа, поэтому обязан проходить проверку.
// Без этого теста новое правило в Validate ломало бы его молча - все остальные тесты
// на нём ждут ошибку и не заметили бы подмены причины.
func TestValidate_ValidConfigPasses(t *testing.T) {
	require.NoError(t, validConfig().Validate())
}

func setValidEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost/testdb")
	t.Setenv("JWT_SECRET", "test-jwt-secret-that-is-at-least-32-chars!!")
	t.Setenv("JWT_REFRESH_SECRET", "test-jwt-refresh-secret-at-least-32-chars!")
	t.Setenv("BIND_HOST", "")
	t.Setenv("BIND_PORT", "")
	t.Setenv("LOG_LEVEL", "")
	t.Setenv("CORS_ALLOWED_ORIGINS", "")
	t.Setenv("UPLOAD_MAX_FILE_SIZE", "")
	t.Setenv("UPLOAD_ALLOWED_IMAGE_TYPES", "")
	t.Setenv("UPLOAD_ALLOWED_DOC_TYPES", "")
	t.Setenv("DATA_ENCRYPTION_KEY", "")
	t.Setenv("REQUIRE_ENCRYPTION", "")
	t.Setenv("RATE_LIMIT_PER_MINUTE", "")
	t.Setenv("RATE_LIMIT_WINDOW_SEC", "")
	t.Setenv("PAGINATION_MAX_LIMIT", "")
	t.Setenv("UPLOAD_PATH", "")
	t.Setenv("SWAGGER_ENABLED", "")
}

func TestLoad_SwaggerEnabled_DefaultFalse(t *testing.T) {
	setValidEnv(t)
	cfg, err := Load()
	require.NoError(t, err)
	assert.False(t, cfg.SwaggerEnabled)
}

func TestLoad_SwaggerEnabled_True(t *testing.T) {
	setValidEnv(t)
	t.Setenv("SWAGGER_ENABLED", "true")
	cfg, err := Load()
	require.NoError(t, err)
	assert.True(t, cfg.SwaggerEnabled)
}

func TestLoad_Defaults(t *testing.T) {
	setValidEnv(t)

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, "0.0.0.0", cfg.BindHost)
	assert.Equal(t, "8090", cfg.BindPort)
	assert.Equal(t, "info", cfg.LogLevel)
}

func TestLoad_MissingDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("JWT_SECRET", "test-jwt-secret-that-is-at-least-32-chars!!")
	t.Setenv("JWT_REFRESH_SECRET", "test-jwt-refresh-secret-at-least-32-chars!")

	_, err := Load()
	assert.Error(t, err)
}

func TestValidate_ShortJWTSecret(t *testing.T) {
	cfg := &Config{
		DatabaseURL:       "postgres://user:pass@localhost/db",
		JWTSecret:         "short",
		JWTRefreshSecret:  "test-jwt-refresh-secret-at-least-32-chars!",
		LogLevel:          "info",
		UploadMaxFileSize: 10485760,
	}
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "JWT_SECRET")
}

func TestValidate_InvalidLogLevel(t *testing.T) {
	cfg := &Config{
		DatabaseURL:       "postgres://user:pass@localhost/db",
		JWTSecret:         "test-jwt-secret-that-is-at-least-32-chars!!",
		JWTRefreshSecret:  "test-jwt-refresh-secret-at-least-32-chars!",
		LogLevel:          "trace",
		UploadMaxFileSize: 10485760,
	}
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "LOG_LEVEL")
}

func TestValidate_InvalidDatabaseURL(t *testing.T) {
	cfg := &Config{
		DatabaseURL:       "mysql://user:pass@localhost/db",
		JWTSecret:         "test-jwt-secret-that-is-at-least-32-chars!!",
		JWTRefreshSecret:  "test-jwt-refresh-secret-at-least-32-chars!",
		JWTAccessTTL:      15 * time.Minute,
		JWTRefreshTTL:     168 * time.Hour,
		LogLevel:          "info",
		UploadMaxFileSize: 10485760,
	}
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "DATABASE_URL")
}

func TestLoad_CORSAllowedOrigins_Default(t *testing.T) {
	setValidEnv(t)

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, []string{"http://localhost:8081"}, cfg.CORSAllowedOrigins)
}

func TestLoad_CORSAllowedOrigins_Custom(t *testing.T) {
	setValidEnv(t)
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:8081,https://example.com")

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, []string{"http://localhost:8081", "https://example.com"}, cfg.CORSAllowedOrigins)
}

func TestLoad_UploadMaxFileSize_Default(t *testing.T) {
	setValidEnv(t)

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, int64(10485760), cfg.UploadMaxFileSize)
}

func TestLoad_UploadMaxFileSize_Custom(t *testing.T) {
	setValidEnv(t)
	t.Setenv("UPLOAD_MAX_FILE_SIZE", "5242880")

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, int64(5242880), cfg.UploadMaxFileSize)
}

func TestLoad_UploadAllowedImageTypes_Default(t *testing.T) {
	setValidEnv(t)

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, []string{"image/jpeg", "image/png", "image/webp"}, cfg.UploadAllowedImageTypes)
}

func TestLoad_UploadAllowedDocTypes_Default(t *testing.T) {
	setValidEnv(t)

	cfg, err := Load()
	require.NoError(t, err)
	// Таблицы в списке с #1721: docx и xlsx неразличимы по сигнатуре (оба zip), и
	// без явного разрешения xlsx проходил только за счёт совпадения с docx.
	assert.Equal(t, []string{
		"application/pdf",
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	}, cfg.UploadAllowedDocTypes)
}

func TestValidate_UploadMaxFileSize_Zero(t *testing.T) {
	cfg := &Config{
		DatabaseURL:       "postgres://user:pass@localhost/db",
		JWTSecret:         "test-jwt-secret-that-is-at-least-32-chars!!",
		JWTRefreshSecret:  "test-jwt-refresh-secret-at-least-32-chars!",
		JWTAccessTTL:      15 * time.Minute,
		JWTRefreshTTL:     168 * time.Hour,
		LogLevel:          "info",
		UploadMaxFileSize: 0,
	}
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "UPLOAD_MAX_FILE_SIZE")
}

func TestLoad_DataEncryptionKey_Empty(t *testing.T) {
	setValidEnv(t)
	cfg, err := Load()
	require.NoError(t, err)
	assert.Empty(t, cfg.DataEncryptionKey)
}

func TestValidate_DataEncryptionKey_InvalidHex(t *testing.T) {
	cfg := validConfig()
	cfg.DataEncryptionKey = "not-hex"
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "DATA_ENCRYPTION_KEY")
}

func TestValidate_DataEncryptionKey_WrongLength(t *testing.T) {
	cfg := validConfig()
	cfg.DataEncryptionKey = "0123456789abcdef"
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "32 bytes")
}

func TestValidate_RequireEncryption_NoKey(t *testing.T) {
	cfg := validConfig()
	cfg.RequireEncryption = true
	cfg.DataEncryptionKey = ""
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "REQUIRE_ENCRYPTION")
}

// Требование шифрования распространяется и на файловый архив: без ключей он пишет
// открытым текстом, а заметить это можно только по именам файлов в каталоге.
func TestValidate_RequireEncryption_NoArchiveKeys(t *testing.T) {
	cfg := validConfig()
	cfg.RequireEncryption = true
	cfg.DataEncryptionKey = strings.Repeat("ab", 32)

	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ARCHIVE_AGE_RECIPIENT")

	cfg.ArchiveAgeRecipient = "age1qqqq"
	cfg.ArchiveAgeIdentity = "AGE-SECRET-KEY-1QQQQ"
	assert.NoError(t, cfg.Validate())
}

func TestLoad_RateLimitDefaults(t *testing.T) {
	setValidEnv(t)
	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, 200, cfg.RateLimitPerMinute)
	assert.Equal(t, int64(60), cfg.RateLimitWindowSec)
}

func TestLoad_PaginationMaxLimitDefault(t *testing.T) {
	setValidEnv(t)
	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, 100, cfg.PaginationMaxLimit)
}

func TestLoad_UploadPathDefault(t *testing.T) {
	setValidEnv(t)
	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "./uploads", cfg.UploadPath)
}

func TestLoad_JWTTTL_Defaults(t *testing.T) {
	setValidEnv(t)
	t.Setenv("JWT_ACCESS_TTL", "")
	t.Setenv("JWT_REFRESH_TTL", "")
	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, 15*time.Minute, cfg.JWTAccessTTL)
	assert.Equal(t, 168*time.Hour, cfg.JWTRefreshTTL)
}

func TestLoad_JWTTTL_Custom(t *testing.T) {
	setValidEnv(t)
	t.Setenv("JWT_ACCESS_TTL", "5m")
	t.Setenv("JWT_REFRESH_TTL", "720h")
	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, 5*time.Minute, cfg.JWTAccessTTL)
	assert.Equal(t, 720*time.Hour, cfg.JWTRefreshTTL)
}

func TestValidate_JWTRefreshTTL_NotGreaterThanAccess(t *testing.T) {
	cfg := validConfig()
	cfg.JWTAccessTTL = 1 * time.Hour
	cfg.JWTRefreshTTL = 30 * time.Minute
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "JWT_REFRESH_TTL")
}

func TestValidate_JWTAccessTTL_NonPositive(t *testing.T) {
	cfg := validConfig()
	cfg.JWTAccessTTL = 0
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "JWT_ACCESS_TTL")
}

// TestValidate_ArchiveMustBeOutsideUploads - главный гейт безопасности среза.
// Каталог загрузок раздаётся статикой до проверки авторизации, а в бланке
// паспортные данные, поэтому архив внутри загрузок означал бы утечку по прямой
// ссылке. Приложение обязано не стартовать, а не писать предупреждение.
func TestValidate_ArchiveMustBeOutsideUploads(t *testing.T) {
	root := t.TempDir()
	uploads := filepath.Join(root, "uploads")

	cases := []struct {
		name    string
		archive string
		upload  string
		wantErr string
	}{
		{
			name:    "архив внутри загрузок",
			archive: filepath.Join(uploads, "archive"),
			upload:  uploads,
			wantErr: "must not be inside UPLOAD_PATH",
		},
		{
			name:    "архив глубоко внутри загрузок",
			archive: filepath.Join(uploads, "a", "b", "archive"),
			upload:  uploads,
			wantErr: "must not be inside UPLOAD_PATH",
		},
		{
			name:    "тот же каталог",
			archive: uploads,
			upload:  uploads,
			wantErr: "must differ from UPLOAD_PATH",
		},
		{
			name:    "загрузки внутри архива",
			archive: root,
			upload:  uploads,
			wantErr: "must not be inside ARCHIVE_PATH",
		},
		{
			name:    "путь с обходом вверх всё равно попадает внутрь загрузок",
			archive: filepath.Join(uploads, "sub", "..", "archive"),
			upload:  uploads,
			wantErr: "must not be inside UPLOAD_PATH",
		},
		{
			name:    "соседние каталоги",
			archive: filepath.Join(root, "archive"),
			upload:  uploads,
			wantErr: "",
		},
		{
			name:    "похожее имя не считается вложенностью",
			archive: uploads + "-archive",
			upload:  uploads,
			wantErr: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.ArchivePath = tc.archive
			cfg.UploadPath = tc.upload

			err := cfg.Validate()
			if tc.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err, "конфигурация с утечкой обязана валить старт")
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

func TestValidate_ArchiveIntervalsMustBePositive(t *testing.T) {
	t.Run("тик воркера", func(t *testing.T) {
		cfg := validConfig()
		cfg.ArchiveWorkerTick = 0
		require.ErrorContains(t, cfg.Validate(), "ARCHIVE_WORKER_TICK")
	})

	t.Run("период подметания", func(t *testing.T) {
		cfg := validConfig()
		cfg.ArchiveSweepInterval = -time.Second
		require.ErrorContains(t, cfg.Validate(), "ARCHIVE_SWEEP_INTERVAL")
	})
}

func TestLoad_ArchiveDefaults(t *testing.T) {
	setValidEnv(t)

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "./archive", cfg.ArchivePath)
	assert.Equal(t, 15*time.Second, cfg.ArchiveWorkerTick)
	assert.Equal(t, 5*time.Minute, cfg.ArchiveSweepInterval)
	assert.NotEqual(t, cfg.UploadPath, cfg.ArchivePath, "дефолты не должны совпадать")
}

// TestValidate_ArchiveSymlinkedIntoUploads - обход проверки ссылкой. Лексически
// пути разные, физически архив лежит внутри публично раздаваемых загрузок, и без
// разворачивания ссылок конфигурация прошла бы проверку.
func TestValidate_ArchiveSymlinkedIntoUploads(t *testing.T) {
	root := t.TempDir()
	uploads := filepath.Join(root, "uploads")
	require.NoError(t, os.MkdirAll(filepath.Join(uploads, "inner"), 0o750))

	link := filepath.Join(root, "archive-link")
	require.NoError(t, os.Symlink(filepath.Join(uploads, "inner"), link))

	cfg := validConfig()
	cfg.ArchivePath = link
	cfg.UploadPath = uploads

	require.ErrorContains(t, cfg.Validate(), "must not be inside UPLOAD_PATH")
}

// TestValidate_UploadsSymlinkedIntoArchive - та же ссылка с другой стороны.
func TestValidate_UploadsSymlinkedIntoArchive(t *testing.T) {
	root := t.TempDir()
	archive := filepath.Join(root, "archive")
	require.NoError(t, os.MkdirAll(filepath.Join(archive, "inner"), 0o750))

	link := filepath.Join(root, "uploads-link")
	require.NoError(t, os.Symlink(filepath.Join(archive, "inner"), link))

	cfg := validConfig()
	cfg.ArchivePath = archive
	cfg.UploadPath = link

	require.ErrorContains(t, cfg.Validate(), "must not be inside ARCHIVE_PATH")
}

// TestValidate_ArchivePathNotCreatedYet - при первом развёртывании каталога ещё нет:
// приложение стартует раньше, чем оператор его создаст. Несуществующий путь не
// должен ни валить старт, ни отключать проверку вложенности.
func TestValidate_ArchivePathNotCreatedYet(t *testing.T) {
	root := t.TempDir()
	uploads := filepath.Join(root, "uploads")
	require.NoError(t, os.MkdirAll(uploads, 0o750))

	t.Run("несуществующий соседний каталог допустим", func(t *testing.T) {
		cfg := validConfig()
		cfg.ArchivePath = filepath.Join(root, "not-created-yet")
		cfg.UploadPath = uploads
		assert.NoError(t, cfg.Validate())
	})

	t.Run("несуществующий каталог внутри загрузок всё равно отвергается", func(t *testing.T) {
		cfg := validConfig()
		cfg.ArchivePath = filepath.Join(uploads, "not-created-yet")
		cfg.UploadPath = uploads
		require.ErrorContains(t, cfg.Validate(), "must not be inside UPLOAD_PATH")
	})
}

// TestValidate_EmptyPathsSkipCheck фиксирует единственную ветку, которая выключает
// проверку целиком. Поведение зеркалит router.go, где статика не поднимается при
// пустом UploadPath. Тест нужен, чтобы будущий рефакторинг не превратил отказ в
// молчаливое разрешение.
func TestValidate_EmptyPathsSkipCheck(t *testing.T) {
	for _, tc := range []struct{ name, archive, upload string }{
		{"оба пусты", "", ""},
		{"пуст архив", "", "/srv/uploads"},
		{"пусты загрузки", "/srv/archive", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.ArchivePath = tc.archive
			cfg.UploadPath = tc.upload
			assert.NoError(t, cfg.Validate())
		})
	}
}

// TestValidate_PushSubscriptionRetentionDaysMustBePositive (#974) - зеркалит
// TestValidate_ArchiveIntervalsMustBePositive: срок хранения подписок Web Push нулём
// или отрицательным быть не может, иначе уборка удалила бы всё при первом же прогоне.
func TestValidate_PushSubscriptionRetentionDaysMustBePositive(t *testing.T) {
	cfg := validConfig()
	cfg.PushSubscriptionRetentionDays = 0
	require.ErrorContains(t, cfg.Validate(), "PUSH_SUBSCRIPTION_RETENTION_DAYS")
}

// TestValidate_VAPIDSubjectRequiredWhenEnabled (#974) - службы доставки (в первую
// очередь Google) отвергают уведомления с пустым контактом отправителя. Без этой
// проверки конфигурация выглядела бы рабочей: ключи заданы, интерфейс сообщает
// «включено», а уведомления молча не доходят ни до кого.
func TestValidate_VAPIDSubjectRequiredWhenEnabled(t *testing.T) {
	cfg := validConfig()
	cfg.VAPIDPublicKey = "pub"
	cfg.VAPIDPrivateKey = "priv"
	cfg.VAPIDSubject = ""
	require.ErrorContains(t, cfg.Validate(), "VAPID_SUBJECT")
}

// TestValidate_VAPIDSubjectFormat (#974) - контакт обязан быть адресом почты или
// сайта: произвольная строка проходит нашу проверку, но отвергается службой уже
// в бою, где разбираться дороже.
func TestValidate_VAPIDSubjectFormat(t *testing.T) {
	cfg := validConfig()
	cfg.VAPIDPublicKey = "pub"
	cfg.VAPIDPrivateKey = "priv"

	cfg.VAPIDSubject = "бюро пропусков"
	require.ErrorContains(t, cfg.Validate(), "VAPID_SUBJECT")

	cfg.VAPIDSubject = "mailto:bureau@example.com"
	require.NoError(t, cfg.Validate())

	cfg.VAPIDSubject = "https://example.com"
	require.NoError(t, cfg.Validate())
}

// TestValidate_VAPIDKeysMustComeInPair (#974) - заданный только один из двух
// VAPID-ключей означает опечатку в конфигурации (забыли скопировать вторую строку),
// а не осознанное "push выключен" (оба пустые). Такую конфигурацию нельзя пропускать
// молча: push выглядел бы включённым, но реально не отправлял бы ни одного сообщения.
func TestValidate_VAPIDKeysMustComeInPair(t *testing.T) {
	t.Run("оба пустые - push выключен, ошибки нет", func(t *testing.T) {
		cfg := validConfig()
		cfg.VAPIDPublicKey = ""
		cfg.VAPIDPrivateKey = ""
		assert.NoError(t, cfg.Validate())
	})

	t.Run("только публичный - ошибка", func(t *testing.T) {
		cfg := validConfig()
		cfg.VAPIDPublicKey = "pub"
		cfg.VAPIDPrivateKey = ""
		require.ErrorContains(t, cfg.Validate(), "VAPID_PUBLIC_KEY")
	})

	t.Run("только приватный - ошибка", func(t *testing.T) {
		cfg := validConfig()
		cfg.VAPIDPublicKey = ""
		cfg.VAPIDPrivateKey = "priv"
		require.ErrorContains(t, cfg.Validate(), "VAPID_PUBLIC_KEY")
	})

	t.Run("оба заданы вместе с контактом - ошибки нет", func(t *testing.T) {
		cfg := validConfig()
		cfg.VAPIDPublicKey = "pub"
		cfg.VAPIDPrivateKey = "priv"
		// Контакт отправителя обязателен при включённой доставке - см.
		// TestValidate_VAPIDSubjectRequiredWhenEnabled.
		cfg.VAPIDSubject = "mailto:bureau@example.com"
		assert.NoError(t, cfg.Validate())
	})
}

// mailConfig - корректная конфигурация с включённой почтой.
func mailConfig() *Config {
	c := validConfig()
	c.SMTPHost = "smtp.jino.ru"
	c.SMTPPort = 587
	c.SMTPFrom = "bureau@example.org"
	c.SMTPUsername = "bureau@example.org"
	c.SMTPPassword = "секрет"
	c.SMTPTLSMode = "starttls"
	c.SMTPTimeoutSec = 15
	c.SMTPRatePerHour = 400
	c.MailRetryAttempts = 5
	c.MailWorkerTick = 15 * time.Second
	return c
}

// TestValidate_MailDisabledByEmptyHost: пустой SMTP_HOST - штатный режим, а не
// ошибка. Стенд и локальная разработка живут без почты.
func TestValidate_MailDisabledByEmptyHost(t *testing.T) {
	c := validConfig()
	require.False(t, c.MailEnabled())
	require.NoError(t, c.Validate())
}

// TestValidate_MailConfigPasses стережёт помощник mailConfig: остальные тесты
// ждут на нём конкретную ошибку и не заметили бы подмены причины.
func TestValidate_MailConfigPasses(t *testing.T) {
	c := mailConfig()
	require.True(t, c.MailEnabled())
	require.NoError(t, c.Validate())
}

// TestValidate_MailHalfConfigured: полуготовая настройка не должна доживать до
// первого письма. Там она превратится в отказ сервера посреди рассылки, когда
// пароли уже сменены и откатывать нечего.
func TestValidate_MailHalfConfigured(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Config)
		expect string
	}{
		{"без отправителя", func(c *Config) { c.SMTPFrom = "" }, "SMTP_FROM"},
		{"отправитель не адрес", func(c *Config) { c.SMTPFrom = "бюро" }, "SMTP_FROM"},
		{"логин без пароля", func(c *Config) { c.SMTPPassword = "" }, "SMTP_USERNAME"},
		{"пароль без логина", func(c *Config) { c.SMTPUsername = "" }, "SMTP_USERNAME"},
		{"неизвестный режим TLS", func(c *Config) { c.SMTPTLSMode = "ssl" }, "SMTP_TLS_MODE"},
		{"порт вне диапазона", func(c *Config) { c.SMTPPort = 70000 }, "SMTP_PORT"},
		{"нулевой таймаут", func(c *Config) { c.SMTPTimeoutSec = 0 }, "SMTP_TIMEOUT_SEC"},
		{"нулевой потолок отправки", func(c *Config) { c.SMTPRatePerHour = 0 }, "SMTP_RATE_PER_HOUR"},
		{"нет попыток доставки", func(c *Config) { c.MailRetryAttempts = 0 }, "MAIL_RETRY_ATTEMPTS"},
		{"нулевой тик воркера", func(c *Config) { c.MailWorkerTick = 0 }, "MAIL_WORKER_TICK"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := mailConfig()
			tc.mutate(c)
			err := c.Validate()
			require.Error(t, err, "полуготовая настройка должна отклоняться")
			assert.Contains(t, err.Error(), tc.expect)
		})
	}
}

// TestValidate_MailServerWithoutAuth: почтовый сервер организации может принимать
// письма без пароля - пустая пара логин/пароль это разрешает.
func TestValidate_MailServerWithoutAuth(t *testing.T) {
	c := mailConfig()
	c.SMTPUsername = ""
	c.SMTPPassword = ""
	require.NoError(t, c.Validate())
}

// TestValidate_ExportMustBeOutsideUploads - зеркало проверки архива для каталога пакетов
// выгрузки. Цена ошибки здесь выше: в пакете лежит весь набор данных организации сразу,
// а загрузки раздаются статикой до проверки авторизации.
func TestValidate_ExportMustBeOutsideUploads(t *testing.T) {
	root := t.TempDir()
	uploads := filepath.Join(root, "uploads")

	cases := []struct {
		name    string
		export  string
		upload  string
		wantErr string
	}{
		{
			name:    "выгрузка внутри загрузок",
			export:  filepath.Join(uploads, "packages"),
			upload:  uploads,
			wantErr: "must be outside UPLOAD_PATH",
		},
		{
			name:    "выгрузка глубоко внутри загрузок",
			export:  filepath.Join(uploads, "a", "b", "packages"),
			upload:  uploads,
			wantErr: "must be outside UPLOAD_PATH",
		},
		{
			name:    "тот же каталог",
			export:  uploads,
			upload:  uploads,
			wantErr: "must be outside UPLOAD_PATH",
		},
		{
			name:    "загрузки внутри выгрузки",
			export:  root,
			upload:  uploads,
			wantErr: "must not be inside ENTITY_EXPORT_PATH",
		},
		{
			name:    "путь с обходом вверх всё равно попадает внутрь загрузок",
			export:  filepath.Join(uploads, "sub", "..", "packages"),
			upload:  uploads,
			wantErr: "must be outside UPLOAD_PATH",
		},
		{
			name:   "соседние каталоги",
			export: filepath.Join(root, "packages"),
			upload: uploads,
		},
		{
			name:   "похожее имя не считается вложенностью",
			export: uploads + "-packages",
			upload: uploads,
		},
		{
			name:   "каталог не задан - проверять нечего",
			export: "",
			upload: uploads,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.EntityExportPath = tc.export
			cfg.UploadPath = tc.upload

			err := cfg.Validate()
			if tc.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err, "конфигурация с утечкой обязана валить старт")
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

// TestValidate_ExportSymlinkedIntoUploads - обход проверки ссылкой: лексически пути
// разные, физически пакеты лягут в публично раздаваемый каталог.
func TestValidate_ExportSymlinkedIntoUploads(t *testing.T) {
	root := t.TempDir()
	uploads := filepath.Join(root, "uploads")
	require.NoError(t, os.MkdirAll(filepath.Join(uploads, "inner"), 0o750))

	link := filepath.Join(root, "packages-link")
	require.NoError(t, os.Symlink(filepath.Join(uploads, "inner"), link))

	cfg := validConfig()
	cfg.EntityExportPath = link
	cfg.UploadPath = uploads

	require.ErrorContains(t, cfg.Validate(), "must be outside UPLOAD_PATH")
}

// TestLoad_EntityExportPathHasNoDefault: у каталога пакетов намеренно нет значения по
// умолчанию. Дефолт означал бы, что выгрузка персональных данных ложится в каталог,
// который никто не выбирал, - место хранения задаёт тот, кто разворачивает систему.
func TestLoad_EntityExportPathHasNoDefault(t *testing.T) {
	setValidEnv(t)

	cfg, err := Load()
	require.NoError(t, err)
	assert.Empty(t, cfg.EntityExportPath)
}

// --- Пул соединений, таймауты HTTP и предел хеширования ---

// clearPoolAndLimitEnv снимает новые параметры с окружения: проверять умолчания в
// оболочке разработчика, где часть из них может быть выставлена, значило бы получить
// тест, зелёный не у всех.
func clearPoolAndLimitEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"DB_MAX_OPEN_CONNS", "DB_MAX_IDLE_CONNS", "DB_CONN_MAX_LIFETIME", "DB_CONN_MAX_IDLE_TIME",
		"HTTP_READ_HEADER_TIMEOUT", "HTTP_READ_TIMEOUT", "HTTP_WRITE_TIMEOUT", "HTTP_IDLE_TIMEOUT",
		"ARGON2_HASH_CONCURRENCY",
	} {
		t.Setenv(key, "")
	}
}

// TestLoad_PoolAndLimitDefaults стережёт сами умолчания: система обязана подниматься,
// когда ни один из параметров не задан, и получать при этом настроенный пул, а не
// умолчания драйвера (безлимит открытых, два простаивающих), ради ухода от которых
// параметры и заведены.
func TestLoad_PoolAndLimitDefaults(t *testing.T) {
	setValidEnv(t)
	clearPoolAndLimitEnv(t)

	cfg, err := Load()
	require.NoError(t, err)

	// 50 - треть от max_connections=150 в docker-compose.prod.yml; остаток нужен
	// консольным подкомандам, pgAdmin и резервному копированию.
	assert.Equal(t, 50, cfg.DBMaxOpenConns)
	assert.Equal(t, 25, cfg.DBMaxIdleConns)
	assert.Equal(t, time.Hour, cfg.DBConnMaxLifetime)
	assert.Equal(t, 10*time.Minute, cfg.DBConnMaxIdleTime)

	assert.Equal(t, 10*time.Second, cfg.HTTPReadHeaderTimeout)
	assert.Equal(t, 120*time.Second, cfg.HTTPReadTimeout)
	assert.Equal(t, 120*time.Second, cfg.HTTPWriteTimeout)
	assert.Equal(t, 120*time.Second, cfg.HTTPIdleTimeout)

	// 0 означает "по числу ядер" - предел ставится, просто считается на старте.
	assert.Equal(t, 0, cfg.Argon2HashConcurrency)

	// Умолчание предела простаивающих не должно превышать предел открытых, иначе
	// поднявшаяся на умолчаниях система падала бы собственной же проверкой.
	require.NoError(t, cfg.Validate())
}

// TestLoad_PoolAndLimitsFromEnv проверяет, что заданные значения доезжают до
// конфигурации, а не теряются по дороге к настройке.
func TestLoad_PoolAndLimitsFromEnv(t *testing.T) {
	setValidEnv(t)
	t.Setenv("DB_MAX_OPEN_CONNS", "77")
	t.Setenv("DB_MAX_IDLE_CONNS", "33")
	t.Setenv("DB_CONN_MAX_LIFETIME", "42m")
	t.Setenv("DB_CONN_MAX_IDLE_TIME", "7m")
	t.Setenv("HTTP_READ_HEADER_TIMEOUT", "3s")
	t.Setenv("HTTP_READ_TIMEOUT", "31s")
	t.Setenv("HTTP_WRITE_TIMEOUT", "32s")
	t.Setenv("HTTP_IDLE_TIMEOUT", "33s")
	t.Setenv("ARGON2_HASH_CONCURRENCY", "4")

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, 77, cfg.DBMaxOpenConns)
	assert.Equal(t, 33, cfg.DBMaxIdleConns)
	assert.Equal(t, 42*time.Minute, cfg.DBConnMaxLifetime)
	assert.Equal(t, 7*time.Minute, cfg.DBConnMaxIdleTime)
	assert.Equal(t, 3*time.Second, cfg.HTTPReadHeaderTimeout)
	assert.Equal(t, 31*time.Second, cfg.HTTPReadTimeout)
	assert.Equal(t, 32*time.Second, cfg.HTTPWriteTimeout)
	assert.Equal(t, 33*time.Second, cfg.HTTPIdleTimeout)
	assert.Equal(t, 4, cfg.Argon2HashConcurrency)
}

// TestValidate_NegativePoolAndLimits: отрицательное значение обязано ронять старт.
// Все три подсистемы трактуют его как "снять ограничение", то есть опечатка в знаке
// молча выключает ровно ту защиту, ради которой параметр появился.
func TestValidate_NegativePoolAndLimits(t *testing.T) {
	tests := []struct {
		name    string
		spoil   func(*Config)
		wantVar string
	}{
		{"открытых соединений", func(c *Config) { c.DBMaxOpenConns = -1 }, "DB_MAX_OPEN_CONNS"},
		{"простаивающих соединений", func(c *Config) { c.DBMaxIdleConns = -1 }, "DB_MAX_IDLE_CONNS"},
		{"времени жизни соединения", func(c *Config) { c.DBConnMaxLifetime = -time.Second }, "DB_CONN_MAX_LIFETIME"},
		{"простоя соединения", func(c *Config) { c.DBConnMaxIdleTime = -time.Second }, "DB_CONN_MAX_IDLE_TIME"},
		{"приёма заголовков", func(c *Config) { c.HTTPReadHeaderTimeout = -time.Second }, "HTTP_READ_HEADER_TIMEOUT"},
		{"чтения запроса", func(c *Config) { c.HTTPReadTimeout = -time.Second }, "HTTP_READ_TIMEOUT"},
		{"записи ответа", func(c *Config) { c.HTTPWriteTimeout = -time.Second }, "HTTP_WRITE_TIMEOUT"},
		{"простоя соединения HTTP", func(c *Config) { c.HTTPIdleTimeout = -time.Second }, "HTTP_IDLE_TIMEOUT"},
		{"параллелизма хеширования", func(c *Config) { c.Argon2HashConcurrency = -1 }, "ARGON2_HASH_CONCURRENCY"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			tt.spoil(cfg)
			err := cfg.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantVar)
		})
	}
}

// TestValidate_IdleConnsAboveOpen ловит half-настройку: предел открытых опустили,
// про простаивающие забыли. database/sql лишнее обрежет молча, и пул будет вести
// себя не так, как записано в файле параметров.
func TestValidate_IdleConnsAboveOpen(t *testing.T) {
	cfg := validConfig()
	cfg.DBMaxOpenConns = 10
	cfg.DBMaxIdleConns = 25

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "DB_MAX_IDLE_CONNS")
	assert.Contains(t, err.Error(), "DB_MAX_OPEN_CONNS")
}

// TestValidate_ZeroPoolAndLimitsAllowed: ноль - осознанное "без ограничения" и
// отдушина администратору, чей обработчик законно идёт дольше таймаута. Отказывать
// в нём значило бы заставлять править код вместо параметра.
func TestValidate_ZeroPoolAndLimitsAllowed(t *testing.T) {
	cfg := validConfig()
	cfg.DBMaxOpenConns = 0
	cfg.DBMaxIdleConns = 0
	cfg.DBConnMaxLifetime = 0
	cfg.DBConnMaxIdleTime = 0
	cfg.HTTPReadHeaderTimeout = 0
	cfg.HTTPReadTimeout = 0
	cfg.HTTPWriteTimeout = 0
	cfg.HTTPIdleTimeout = 0
	cfg.Argon2HashConcurrency = 0

	require.NoError(t, cfg.Validate())
}
