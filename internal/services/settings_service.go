package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"sync"
	"time"

	"systemburo/internal/config"
	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// dataProcessingDocKey -- ключ настройки с метаданными документа согласия на обработку данных.
const dataProcessingDocKey = "legal.data_processing_document"

// Ключи настроек согласия на обработку ПД при первом входе (#1567).
const (
	pdConsentTextKey     = "legal.pd_consent_text"
	pdConsentVersionKey  = "legal.pd_consent_version"
	pdConsentRequiredKey = "legal.pd_consent_required"
	// Дата появления действующей редакции: ставится вместе с подъёмом номера.
	pdConsentVersionAtKey = "legal.pd_consent_version_at"
)

// PDConsentTextMaxBytes -- предел размера HTML согласия. Редактор инлайнит картинки
// в base64, а текст уезжает пользователю при каждом входе, поэтому лимит жёсткий.
const PDConsentTextMaxBytes = 512 * 1024

type SettingsService interface {
	GetAll(ctx context.Context) ([]models.SystemSetting, error)
	Update(ctx context.Context, key string, value string) (*models.SystemSetting, error)
	GetUploadSettings(ctx context.Context) (map[string]interface{}, error)
	GetNotificationSettings(ctx context.Context) (map[string]interface{}, error)
	GetPasswordPolicy() models.PasswordPolicy
	GetPublicContacts(ctx context.Context) map[string]string
	GetDataProcessingDoc(ctx context.Context) (*models.DataProcessingDocument, error)
	SetDataProcessingDoc(ctx context.Context, meta *models.DataProcessingDocument) error
	ClearDataProcessingDoc(ctx context.Context) error
	GetPDConsentSettings(ctx context.Context) (*models.PDConsentSettings, error)
	SetPDConsentText(ctx context.Context, text string, requireAgain bool) error
	SetPDConsentRequired(ctx context.Context, required bool) error
	BumpPDConsentVersion(ctx context.Context) (int, error)
	GetApprovalReminderSettings(ctx context.Context) (enabled bool, firstDays int, repeatDays int)
	GetArchiveSettings(ctx context.Context) (*models.ArchiveSettings, error)
	UpdateArchiveSettings(ctx context.Context, req models.UpdateArchiveSettingsRequest) (*models.ArchiveSettings, error)
}

var knownKeys = map[string]string{
	"upload.max_file_size":           "int",
	"notifications.delete_duration":  "int",
	"notifications.restore_duration": "int",
	"password.min_length":            "int",
	"password.require_letter":        "bool",
	"password.require_uppercase":     "bool",
	"password.require_lowercase":     "bool",
	"password.require_digit":         "bool",
	"password.require_special":       "bool",
	// Плановая смена паролей (#1909). Периодичность в сутках; 120 - потолок из
	// приказа ФСТЭК России N 21 для ИСПДн, выше поднимать бессмысленно.
	"password.rotation_enabled":            "bool",
	"password.rotation_days":               "int",
	"password.rotation_notify_days_before": "int",
	"password.force_change_on_next_login":  "bool",
	"contacts.bureau_phone":                "string",
	"contacts.bureau_email":                "string",
	"approval.reminder_enabled":            "bool",
	"approval.reminder_first_days":         "int",
	"approval.reminder_repeat_days":        "int",
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
		"upload.max_file_size":                 {Key: "upload.max_file_size", Value: strconv.FormatInt(cfg.UploadMaxFileSize, 10), Type: "int"},
		"notifications.delete_duration":        {Key: "notifications.delete_duration", Value: "10", Type: "int"},
		"notifications.restore_duration":       {Key: "notifications.restore_duration", Value: "5", Type: "int"},
		"password.min_length":                  {Key: "password.min_length", Value: "8", Type: "int"},
		"password.require_letter":              {Key: "password.require_letter", Value: "true", Type: "bool"},
		"password.require_uppercase":           {Key: "password.require_uppercase", Value: "false", Type: "bool"},
		"password.require_lowercase":           {Key: "password.require_lowercase", Value: "false", Type: "bool"},
		"password.require_digit":               {Key: "password.require_digit", Value: "true", Type: "bool"},
		"password.require_special":             {Key: "password.require_special", Value: "false", Type: "bool"},
		"password.rotation_enabled":            {Key: "password.rotation_enabled", Value: "false", Type: "bool"},
		"password.rotation_days":               {Key: "password.rotation_days", Value: "90", Type: "int"},
		"password.rotation_notify_days_before": {Key: "password.rotation_notify_days_before", Value: "7", Type: "int"},
		"password.force_change_on_next_login":  {Key: "password.force_change_on_next_login", Value: "true", Type: "bool"},
		"contacts.bureau_phone":                {Key: "contacts.bureau_phone", Value: "", Type: "string"},
		"contacts.bureau_email":                {Key: "contacts.bureau_email", Value: "", Type: "string"},
		// Автонапоминания зависшим согласующим (#1315, ReminderService): включены по
		// умолчанию, первое напоминание через 3 дня молчания, дальше раз в 3 дня.
		"approval.reminder_enabled":     {Key: "approval.reminder_enabled", Value: "true", Type: "bool"},
		"approval.reminder_first_days":  {Key: "approval.reminder_first_days", Value: "3", Type: "int"},
		"approval.reminder_repeat_days": {Key: "approval.reminder_repeat_days", Value: "3", Type: "int"},
	}

	s := &settingsService{db: db, defaults: defaults, cache: make(map[string]models.SystemSetting)}
	s.loadCache()
	return s
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

// GetAll возвращает все системные настройки из кэша. Доступ гейтится на уровне
// роутера правом page.admin.settings (#7), сервис уже не проверяет вызывающего.
func (s *settingsService) GetAll(ctx context.Context) ([]models.SystemSetting, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]models.SystemSetting, 0, len(s.cache))
	for _, v := range s.cache {
		result = append(result, v)
	}
	return result, nil
}

// Update обновляет значение системной настройки по ключу. Доступ гейтится на
// уровне роутера правом page.admin.settings (#7), сервис уже не проверяет
// вызывающего.
func (s *settingsService) Update(ctx context.Context, key string, value string) (*models.SystemSetting, error) {
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

	// Только предельный размер: перечни типов задавались настройкой, но ни на что не
	// влияли - формат проверяется по сигнатуре файла в upload/pipeline.go, и это
	// надёжнее списка расширений, который можно обойти переименованием (#2000).
	maxSize, _ := strconv.ParseInt(s.cache["upload.max_file_size"].Value, 10, 64)

	return map[string]interface{}{
		"max_file_size": maxSize,
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
		RotationEnabled:  s.cache["password.rotation_enabled"].Value == "true",
		RotationDays:     cachedInt(s.cache, "password.rotation_days", models.DefaultRotationDays),
		RotationNotifyDaysBefore: cachedInt(s.cache, "password.rotation_notify_days_before",
			models.DefaultRotationNotifyDaysBefore),
		ForceChangeOnNextLogin: s.cache["password.force_change_on_next_login"].Value == "true",
	}
}

// GetApprovalReminderSettings возвращает настройки автонапоминаний согласующим
// (#1315): включены ли напоминания, через сколько дней молчания слать первое и
// с каким периодом повторять. Некорректные/нулевые значения в кэше (не должны
// возникать при валидации через Update, но защищаемся) фолбэчат на дефолт 3 дня.
func (s *settingsService) GetApprovalReminderSettings(ctx context.Context) (enabled bool, firstDays int, repeatDays int) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	enabled = s.cache["approval.reminder_enabled"].Value == "true"
	firstDays, _ = strconv.Atoi(s.cache["approval.reminder_first_days"].Value)
	if firstDays <= 0 {
		firstDays = 3
	}
	repeatDays, _ = strconv.Atoi(s.cache["approval.reminder_repeat_days"].Value)
	if repeatDays <= 0 {
		repeatDays = 3
	}
	return enabled, firstDays, repeatDays
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

// GetPDConsentSettings собирает настройки согласия на обработку ПД: текст, требуемую
// версию и флаг обязательности. Читает БД, а не кэш: кэш наполняется при старте
// процесса, и при нескольких репликах подъём версии на одной остался бы не виден
// остальным. Отсутствующие ключи дают версию 1 и выключенный запрос согласия.
func (s *settingsService) GetPDConsentSettings(ctx context.Context) (*models.PDConsentSettings, error) {
	var settings []models.SystemSetting
	keys := []string{pdConsentTextKey, pdConsentVersionKey, pdConsentRequiredKey, pdConsentVersionAtKey}
	if err := s.db.WithContext(ctx).Where("key IN ?", keys).Find(&settings).Error; err != nil {
		return nil, fmt.Errorf("failed to load pd consent settings: %w", err)
	}
	result := &models.PDConsentSettings{Version: 1}
	for _, st := range settings {
		switch st.Key {
		case pdConsentTextKey:
			result.Text = st.Value
		case pdConsentVersionKey:
			if v, err := strconv.Atoi(st.Value); err == nil && v > 0 {
				result.Version = v
			}
		case pdConsentRequiredKey:
			result.Required = st.Value == "true"
		case pdConsentVersionAtKey:
			result.VersionAt = st.Value
		}
	}
	return result, nil
}

// SetPDConsentText сохраняет текст согласия. Пустая строка допустима -- это очистка
// текста; запрос согласия при этом перестаёт работать, и администратор видит
// предупреждение в интерфейсе.
//
// requireAgain поднимает редакцию тем же действием: изменённый текст -- новая
// редакция, подтверждать её надо заново. Редакцию двигаем ДО записи текста: если
// запись сорвётся, люди переподтвердят прежний текст (шум, но не обман), а обратный
// порядок оставил бы новый текст со старой редакцией -- то есть согласие, данное не
// тому тексту, который человеку теперь показывают.
func (s *settingsService) SetPDConsentText(ctx context.Context, text string, requireAgain bool) error {
	if len(text) > PDConsentTextMaxBytes {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf(
			"Текст согласия больше %d КБ. Уберите вставленные картинки или сократите текст.",
			PDConsentTextMaxBytes/1024))
	}
	if requireAgain {
		if _, err := s.BumpPDConsentVersion(ctx); err != nil {
			return err
		}
	}
	return s.upsertRaw(ctx, pdConsentTextKey, text, "html")
}

// SetPDConsentRequired включает или выключает запрос согласия при входе. Включить с
// пустым текстом нельзя: настройка выглядела бы рабочей, а показать пользователю было
// бы нечего.
func (s *settingsService) SetPDConsentRequired(ctx context.Context, required bool) error {
	if required {
		current, err := s.GetPDConsentSettings(ctx)
		if err != nil {
			return err
		}
		if !hasVisibleText(current.Text) {
			return echo.NewHTTPError(http.StatusBadRequest, "Сначала задайте текст согласия")
		}
	}
	return s.upsertRaw(ctx, pdConsentRequiredKey, strconv.FormatBool(required), "bool")
}

// BumpPDConsentVersion поднимает требуемую версию согласия: после этого система
// запросит согласие заново у всех, кто соглашался с прежней редакцией текста.
//
// Чтение и запись не в одной транзакции, поэтому одновременный подъём двумя
// администраторами может потерять один инкремент (оба прочитают N, оба запишут
// N+1). На семантику это не влияет: версия всё равно уходит вперёд, и согласия с
// прежней редакцией становятся недостаточными -- ровно то, чего хотел каждый из
// администраторов.
func (s *settingsService) BumpPDConsentVersion(ctx context.Context) (int, error) {
	current, err := s.GetPDConsentSettings(ctx)
	if err != nil {
		return 0, err
	}
	next := current.Version + 1
	if err := s.upsertRaw(ctx, pdConsentVersionKey, strconv.Itoa(next), "int"); err != nil {
		return 0, err
	}
	// Дату пишем тем же действием: редакция без даты ничего не говорит человеку,
	// а восстановить её задним числом уже неоткуда.
	if err := s.upsertRaw(ctx, pdConsentVersionAtKey, time.Now().UTC().Format(time.RFC3339), "string"); err != nil {
		return 0, err
	}
	return next, nil
}

// hasVisibleText сообщает, есть ли в HTML видимое содержимое. Редактор на пустом
// документе отдаёт "<p></p>", поэтому голого TrimSpace недостаточно: без этой проверки
// согласие можно было бы включить с визуально пустым текстом.
func hasVisibleText(html string) bool {
	if strings.Contains(strings.ToLower(html), "<img") {
		return true
	}
	var text strings.Builder
	depth := 0
	for _, r := range html {
		switch {
		case r == '<':
			depth++
		case r == '>':
			if depth > 0 {
				depth--
			}
		case depth == 0:
			text.WriteRune(r)
		}
	}
	plain := strings.ReplaceAll(text.String(), "&nbsp;", " ")
	plain = strings.ReplaceAll(plain, "\u00a0", " ")
	return strings.TrimSpace(plain) != ""
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
	case "notifications.delete_duration", "notifications.restore_duration":
		v, err := strconv.Atoi(value)
		if err != nil || v < 3 || v > 60 {
			return fmt.Errorf("%s: 3-60 сек (получено %s)", key, value)
		}
	case "password.rotation_enabled", "password.force_change_on_next_login":
		if value != "true" && value != "false" {
			return fmt.Errorf("%s: true/false (получено %s)", key, value)
		}
	case "password.rotation_days":
		v, err := strconv.Atoi(value)
		if err != nil || v < models.MinRotationDays || v > models.MaxRotationDays {
			// Верхняя граница не косметическая: приказ ФСТЭК России N 21 требует
			// смены пароля не реже чем раз в 120 суток, и разрешать «раз в три
			// года» в системе, аттестуемой как ИСПДн, нельзя.
			return fmt.Errorf("password.rotation_days: %d-%d (получено %s)",
				models.MinRotationDays, models.MaxRotationDays, value)
		}
	case "password.rotation_notify_days_before":
		v, err := strconv.Atoi(value)
		if err != nil || v < 0 || v > models.MaxRotationNotifyDaysBefore {
			return fmt.Errorf("password.rotation_notify_days_before: 0-%d (получено %s)",
				models.MaxRotationNotifyDaysBefore, value)
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
	case "approval.reminder_enabled":
		if value != "true" && value != "false" {
			return fmt.Errorf("approval.reminder_enabled: true/false (получено %s)", value)
		}
	case "approval.reminder_first_days", "approval.reminder_repeat_days":
		v, err := strconv.Atoi(value)
		if err != nil || v < 1 || v > 30 {
			return fmt.Errorf("%s: 1-30 (получено %s)", key, value)
		}
	}
	return nil
}

// cachedInt читает целочисленную настройку из кэша, подставляя дефолт при пустом
// или испорченном значении. Иначе мусор в базе превращается в ноль, а ноль в
// периодичности означал бы «пароль истёк у всех сразу».
func cachedInt(cache map[string]models.SystemSetting, key string, fallback int) int {
	v, err := strconv.Atoi(cache[key].Value)
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}
