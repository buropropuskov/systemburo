package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"

	"systemburo/internal/config"
	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

type SettingsService interface {
	GetAll(ctx context.Context, isSuperAdmin bool) ([]models.SystemSetting, error)
	Update(ctx context.Context, isSuperAdmin bool, key string, value string) (*models.SystemSetting, error)
	GetUploadSettings(ctx context.Context) (map[string]interface{}, error)
	GetNotificationSettings(ctx context.Context) (map[string]interface{}, error)
}

var knownKeys = map[string]string{
	"upload.max_file_size":            "int",
	"upload.allowed_image_types":      "json",
	"upload.allowed_doc_types":        "json",
	"pagination.max_per_page":         "int",
	"notifications.enabled":           "bool",
	"notifications.poll_interval":     "int",
	"notifications.delete_duration":   "int",
	"notifications.restore_duration":  "int",
}

type settingsService struct {
	db       *gorm.DB
	mu       sync.RWMutex
	cache    map[string]models.SystemSetting
	defaults map[string]models.SystemSetting
}

// NewSettingsService создаёт сервис для управления системными настройками.
func NewSettingsService(db *gorm.DB, cfg *config.Config) SettingsService {
	defaults := map[string]models.SystemSetting{
		"upload.max_file_size":        {Key: "upload.max_file_size", Value: strconv.FormatInt(cfg.UploadMaxFileSize, 10), Type: "int"},
		"upload.allowed_image_types":  {Key: "upload.allowed_image_types", Value: mustJSON(cfg.UploadAllowedImageTypes), Type: "json"},
		"upload.allowed_doc_types":    {Key: "upload.allowed_doc_types", Value: mustJSON(cfg.UploadAllowedDocTypes), Type: "json"},
		"pagination.max_per_page":     {Key: "pagination.max_per_page", Value: strconv.Itoa(cfg.PaginationMaxLimit), Type: "int"},
		"notifications.enabled":         {Key: "notifications.enabled", Value: "true", Type: "bool"},
		"notifications.poll_interval":    {Key: "notifications.poll_interval", Value: "30", Type: "int"},
		"notifications.delete_duration":  {Key: "notifications.delete_duration", Value: "10", Type: "int"},
		"notifications.restore_duration": {Key: "notifications.restore_duration", Value: "5", Type: "int"},
	}

	s := &settingsService{db: db, defaults: defaults, cache: make(map[string]models.SystemSetting)}
	s.loadCache()
	return s
}

func mustJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func (s *settingsService) loadCache() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for k, v := range s.defaults {
		s.cache[k] = v
	}

	var settings []models.SystemSetting
	s.db.Find(&settings)
	for _, st := range settings {
		s.cache[st.Key] = st
	}
}

func (s *settingsService) checkSuper(isSuperAdmin bool) error {
	if !isSuperAdmin {
		return echo.NewHTTPError(http.StatusForbidden, "Доступ только для супер-администратора")
	}
	return nil
}

// GetAll возвращает все системные настройки из кэша.
func (s *settingsService) GetAll(ctx context.Context, isSuperAdmin bool) ([]models.SystemSetting, error) {
	if err := s.checkSuper(isSuperAdmin); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]models.SystemSetting, 0, len(s.cache))
	for _, v := range s.cache {
		result = append(result, v)
	}
	return result, nil
}

// Update обновляет значение системной настройки по ключу.
func (s *settingsService) Update(ctx context.Context, isSuperAdmin bool, key string, value string) (*models.SystemSetting, error) {
	if err := s.checkSuper(isSuperAdmin); err != nil {
		return nil, err
	}
	settingType, ok := knownKeys[key]
	if !ok {
		return nil, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Неизвестная настройка: %s", key))
	}
	if err := validateSettingValue(key, value); err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	setting := models.SystemSetting{Key: key, Value: value, Type: settingType}

	var existing models.SystemSetting
	if err := s.db.WithContext(ctx).Where("key = ?", key).First(&existing).Error; err == nil {
		existing.Value = value
		if err := s.db.WithContext(ctx).Save(&existing).Error; err != nil {
			return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка сохранения настройки")
		}
		setting = existing
	} else {
		if err := s.db.WithContext(ctx).Create(&setting).Error; err != nil {
			return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка создания настройки")
		}
	}

	s.mu.Lock()
	s.cache[key] = setting
	s.mu.Unlock()

	return &setting, nil
}

// GetUploadSettings возвращает настройки загрузки файлов (размер, допустимые типы).
func (s *settingsService) GetUploadSettings(ctx context.Context) (map[string]interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	maxSize, _ := strconv.ParseInt(s.cache["upload.max_file_size"].Value, 10, 64)

	var imageTypes, docTypes []string
	json.Unmarshal([]byte(s.cache["upload.allowed_image_types"].Value), &imageTypes)
	json.Unmarshal([]byte(s.cache["upload.allowed_doc_types"].Value), &docTypes)

	return map[string]interface{}{
		"max_file_size":       maxSize,
		"allowed_image_types": imageTypes,
		"allowed_doc_types":   docTypes,
	}, nil
}

// GetNotificationSettings возвращает длительности уведомлений удаления/восстановления (сек).
func (s *settingsService) GetNotificationSettings(ctx context.Context) (map[string]interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	del, _ := strconv.Atoi(s.cache["notifications.delete_duration"].Value)
	res, _ := strconv.Atoi(s.cache["notifications.restore_duration"].Value)
	if del <= 0 {
		del = 10
	}
	if res <= 0 {
		res = 5
	}
	return map[string]interface{}{
		"delete_duration":  del,
		"restore_duration": res,
	}, nil
}

func validateSettingValue(key, value string) error {
	switch key {
	case "upload.max_file_size":
		v, err := strconv.ParseInt(value, 10, 64)
		if err != nil || v < 1048576 || v > 52428800 {
			return fmt.Errorf("upload.max_file_size: 1MB-50MB (получено %s)", value)
		}
	case "pagination.max_per_page":
		v, err := strconv.Atoi(value)
		if err != nil || v < 10 || v > 500 {
			return fmt.Errorf("pagination.max_per_page: 10-500 (получено %s)", value)
		}
	case "notifications.poll_interval":
		v, err := strconv.Atoi(value)
		if err != nil || v < 10 || v > 120 {
			return fmt.Errorf("notifications.poll_interval: 10-120 сек (получено %s)", value)
		}
	case "notifications.delete_duration", "notifications.restore_duration":
		v, err := strconv.Atoi(value)
		if err != nil || v < 3 || v > 60 {
			return fmt.Errorf("%s: 3-60 сек (получено %s)", key, value)
		}
	case "notifications.enabled":
		if value != "true" && value != "false" {
			return fmt.Errorf("notifications.enabled: true/false (получено %s)", value)
		}
	case "upload.allowed_image_types", "upload.allowed_doc_types":
		var arr []string
		if err := json.Unmarshal([]byte(value), &arr); err != nil {
			return fmt.Errorf("%s: должен быть JSON массив строк", key)
		}
	}
	return nil
}
