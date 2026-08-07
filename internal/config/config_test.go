package config

import (
	"os"
	"path/filepath"
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
