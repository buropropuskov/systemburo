package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setValidEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost/testdb")
	t.Setenv("JWT_SECRET", "test-jwt-secret-that-is-at-least-32-chars!!")
	t.Setenv("JWT_REFRESH_SECRET", "test-jwt-refresh-secret-at-least-32-chars!")
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
	assert.Equal(t, []string{"*"}, cfg.CORSAllowedOrigins)
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
	assert.Equal(t, []string{
		"application/pdf",
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	}, cfg.UploadAllowedDocTypes)
}

func TestValidate_UploadMaxFileSize_Zero(t *testing.T) {
	cfg := &Config{
		DatabaseURL:       "postgres://user:pass@localhost/db",
		JWTSecret:         "test-jwt-secret-that-is-at-least-32-chars!!",
		JWTRefreshSecret:  "test-jwt-refresh-secret-at-least-32-chars!",
		LogLevel:          "info",
		UploadMaxFileSize: 0,
	}
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "UPLOAD_MAX_FILE_SIZE")
}
