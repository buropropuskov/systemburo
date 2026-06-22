package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"strconv"
	"sync"

	"systemburo/internal/config"
	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// dataProcessingDocKey -- ключ настройки с метаданными документа согласия на обработку данных.
const dataProcessingDocKey = "legal.data_processing_document"

type SettingsService interface {
	GetAll(ctx context.Context, isSuperAdmin bool) ([]models.SystemSetting, error)
	Update(ctx context.Context, isSuperAdmin bool, key string, value string) (*models.SystemSetting, error)
	GetUploadSettings(ctx context.Context) (map[string]interface{}, error)
	GetNotificationSettings(ctx context.Context) (map[string]interface{}, error)
	GetPasswordPolicy() models.PasswordPolicy
	GetPublicContacts(ctx context.Context) map[string]string
	GetDataProcessingDoc(ctx context.Context) (*models.DataProcessingDocument, error)
	SetDataProcessingDoc(ctx context.Context, meta *models.DataProcessingDocument) error
	ClearDataProcessingDoc(ctx context.Context) error
}

var knownKeys = map[string]string{
	"upload.max_file_size":           "int",
	"upload.allowed_image_types":     "json",
	"upload.allowed_doc_types":       "json",
	"pagination.max_per_page":        "int",
	"notifications.enabled":          "bool",
	"notifications.poll_interval":    "int",
	"notifications.delete_duration":  "int",
	"notifications.restore_duration": "int",
	"password.min_length":            "int",
	"password.require_letter":        "bool",
	"password.require_uppercase":     "bool",
	"password.require_lowercase":     "bool",
	"password.require_digit":         "bool",
	"password.require_special":       "bool",
	"contacts.bureau_phone":          "string",
	"contacts.bureau_email":          "string",
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
		"upload.max_file_size":           {Key: "upload.max_file_size", Value: strconv.FormatInt(cfg.UploadMaxFileSize, 10), Type: "int"},
		"upload.allowed_image_types":     {Key: "upload.allowed_image_types", Value: mustJSON(cfg.UploadAllowedImageTypes), Type: "json"},
		"upload.allowed_doc_types":       {Key: "upload.allowed_doc_types", Value: mustJSON(cfg.UploadAllowedDocTypes), Type: "json"},
		"pagination.max_per_page":        {Key: "pagination.max_per_page", Value: strconv.Itoa(cfg.PaginationMaxLimit), Type: "int"},
		"notifications.enabled":          {Key: "notifications.enabled", Value: "true", Type: "bool"},
		"notifications.poll_interval":    {Key: "notifications.poll_interval", Value: "30", Type: "int"},
		"notifications.delete_duration":  {Key: "notifications.delete_duration", Value: "10", Type: "int"},
		"notifications.restore_duration": {Key: "notifications.restore_duration", Value: "5", Type: "int"},
		"password.min_length":            {Key: "password.min_length", Value: "8", Type: "int"},
		"password.require_letter":        {Key: "password.require_letter", Value: "true", Type: "bool"},
		"password.require_uppercase":     {Key: "password.require_uppercase", Value: "false", Type: "bool"},
		"password.require_lowercase":     {Key: "password.require_lowercase", Value: "false", Type: "bool"},
		"password.require_digit":         {Key: "password.require_digit", Value: "true", Type: "bool"},
		"password.require_special":       {Key: "password.require_special", Value: "false", Type: "bool"},
		"contacts.bureau_phone":          {Key: "contacts.bureau_phone", Value: "", Type: "string"},
		"contacts.bureau_email":          {Key: "contacts.bureau_email", Value: "", Type: "string"},
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

// GetPublicContacts возвращает контактные данные Бюро пропусков (телефон, почта)
// для публичного отображения (логин, плашка блокировки). Без проверки прав --
// это публичная справочная информация. Пустые значения означают "не настроено".
func (s *settingsService) GetPublicContacts(ctx context.Context) map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return map[string]string{
		"phone": s.cache["contacts.bureau_phone"].Value,
		"email": s.cache["contacts.bureau_email"].Value,
	}
}

// GetPasswordPolicy собирает текущую политику паролей из кэша настроек.
func (s *settingsService) GetPasswordPolicy() models.PasswordPolicy {
	s.mu.RLock()
	defer s.mu.RUnlock()

	minLen, _ := strconv.Atoi(s.cache["password.min_length"].Value)
	return models.PasswordPolicy{
		MinLength:        minLen,
		RequireLetter:    s.cache["password.require_letter"].Value == "true",
		RequireUppercase: s.cache["password.require_uppercase"].Value == "true",
		RequireLowercase: s.cache["password.require_lowercase"].Value == "true",
		RequireDigit:     s.cache["password.require_digit"].Value == "true",
		RequireSpecial:   s.cache["password.require_special"].Value == "true",
	}
}

// GetDataProcessingDoc возвращает метаданные документа согласия или nil, если он не загружен.
func (s *settingsService) GetDataProcessingDoc(ctx context.Context) (*models.DataProcessingDocument, error) {
	var setting models.SystemSetting
	err := s.db.WithContext(ctx).Where("key = ?", dataProcessingDocKey).First(&setting).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load data processing document setting: %w", err)
	}
	if setting.Value == "" {
		return nil, nil
	}
	var meta models.DataProcessingDocument
	if err := json.Unmarshal([]byte(setting.Value), &meta); err != nil {
		return nil, fmt.Errorf("failed to parse data processing document metadata: %w", err)
	}
	return &meta, nil
}

// SetDataProcessingDoc сохраняет (upsert) метаданные документа согласия.
func (s *settingsService) SetDataProcessingDoc(ctx context.Context, meta *models.DataProcessingDocument) error {
	payload, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("failed to marshal data processing document metadata: %w", err)
	}
	return s.upsertRaw(ctx, dataProcessingDocKey, string(payload), "json")
}

// ClearDataProcessingDoc удаляет настройку с метаданными документа согласия.
func (s *settingsService) ClearDataProcessingDoc(ctx context.Context) error {
	if err := s.db.WithContext(ctx).Where("key = ?", dataProcessingDocKey).Delete(&models.SystemSetting{}).Error; err != nil {
		return fmt.Errorf("failed to clear data processing document setting: %w", err)
	}
	s.mu.Lock()
	delete(s.cache, dataProcessingDocKey)
	s.mu.Unlock()
	return nil
}

// upsertRaw записывает значение настройки напрямую, без проверки knownKeys и прав
// супер-администратора (авторизация для таких ключей -- на уровне роутов/middleware).
func (s *settingsService) upsertRaw(ctx context.Context, key, value, typ string) error {
	var existing models.SystemSetting
	err := s.db.WithContext(ctx).Where("key = ?", key).First(&existing).Error
	switch {
	case err == nil:
		existing.Value = value
		existing.Type = typ
		if err := s.db.WithContext(ctx).Save(&existing).Error; err != nil {
			return fmt.Errorf("failed to save setting %s: %w", key, err)
		}
		s.cacheSet(existing)
	case errors.Is(err, gorm.ErrRecordNotFound):
		setting := models.SystemSetting{Key: key, Value: value, Type: typ}
		if err := s.db.WithContext(ctx).Create(&setting).Error; err != nil {
			return fmt.Errorf("failed to create setting %s: %w", key, err)
		}
		s.cacheSet(setting)
	default:
		return fmt.Errorf("failed to load setting %s: %w", key, err)
	}
	return nil
}

func (s *settingsService) cacheSet(st models.SystemSetting) {
	s.mu.Lock()
	s.cache[st.Key] = st
	s.mu.Unlock()
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
	case "password.min_length":
		v, err := strconv.Atoi(value)
		if err != nil || v < 6 || v > 128 {
			return fmt.Errorf("password.min_length: 6-128 (получено %s)", value)
		}
	case "password.require_letter", "password.require_uppercase", "password.require_lowercase", "password.require_digit", "password.require_special":
		if value != "true" && value != "false" {
			return fmt.Errorf("%s: true/false (получено %s)", key, value)
		}
	case "contacts.bureau_email":
		if _, err := mail.ParseAddress(value); err != nil {
			return fmt.Errorf("contacts.bureau_email: некорректный email (получено %s)", value)
		}
	case "contacts.bureau_phone":
		if l := len([]rune(value)); l < 5 || l > 30 {
			return fmt.Errorf("contacts.bureau_phone: 5-30 символов (получено %s)", value)
		}
	case "upload.allowed_image_types", "upload.allowed_doc_types":
		var arr []string
		if err := json.Unmarshal([]byte(value), &arr); err != nil {
			return fmt.Errorf("%s: должен быть JSON массив строк", key)
		}
	}
	return nil
}
