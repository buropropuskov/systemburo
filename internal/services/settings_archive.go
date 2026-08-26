package services

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"systemburo/internal/blankpath"
	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
)

// Ключи настроек файлового архива (#1615). Мимо knownKeys намеренно: универсальный
// PUT /settings/:key доступен только супер-администратору и не умеет проверять
// шаблон пути, а архивом заведует администратор с правом на раздел.
const (
	archiveEnabledKey         = "archive.enabled"
	archiveDirTemplateKey     = "archive.dir_template"
	archiveFileTemplateKey    = "archive.file_template"
	archiveQuotaBytesKey      = "archive.quota_bytes"
	archiveMinFreeBytesKey    = "archive.min_free_bytes"
	archiveWarnPercentKey     = "archive.warn_percent"
	archiveRecheckDaysKey     = "archive.recheck_days"
	archiveFreezeAfterDaysKey = "archive.freeze_after_days"
	archiveZipMaxBytesKey     = "archive.zip_max_bytes"
)

// Значения настроек архива по умолчанию.
const (
	// defaultArchiveMinFreeBytes - 2 ГБ. Ниже этого выгрузка встаёт, чтобы архив не
	// доел раздел под базой: заявки важнее их бумажных копий.
	defaultArchiveMinFreeBytes = 2 << 30
	// defaultArchiveZipMaxBytes - 2 ГБ на одну выгрузку за период.
	defaultArchiveZipMaxBytes = 2 << 30
	defaultArchiveWarnPercent = 80
	defaultArchiveRecheckDays = 30
	// defaultArchiveFreezeAfterDays - месяц после завершения заявки. Срок выбран под
	// разбор спорных случаев: пока заявку могут поправить задним числом, файл обязан
	// следовать за ней, а дальше становится документом и больше не меняется.
	defaultArchiveFreezeAfterDays = 30
)

// Границы числовых настроек. Проверяются на записи: значение вне диапазона либо
// бессмысленно (порог 0%), либо останавливает механизм молча (сверка раз в 100 лет).
const (
	archiveWarnPercentMin     = 1
	archiveWarnPercentMax     = 99
	archiveRecheckDaysMin     = 1
	archiveRecheckDaysMax     = 365
	archiveFreezeAfterDaysMax = 3650
)

// GetArchiveSettings собирает настройки файлового архива. Читает БД, а не кэш
// настроек: кэш наполняется один раз при старте процесса, а воркер выгрузки обязан
// увидеть правку администратора на ближайшем тике, не после перезапуска.
//
// Отсутствующие ключи дают значения по умолчанию, поэтому раздел работает на базе,
// где настройки ни разу не сохраняли.
func (s *settingsService) GetArchiveSettings(ctx context.Context) (*models.ArchiveSettings, error) {
	keys := []string{
		archiveEnabledKey, archiveDirTemplateKey, archiveFileTemplateKey,
		archiveQuotaBytesKey, archiveMinFreeBytesKey, archiveWarnPercentKey,
		archiveRecheckDaysKey, archiveFreezeAfterDaysKey, archiveZipMaxBytesKey,
	}
	var rows []models.SystemSetting
	if err := s.db.WithContext(ctx).Where("key IN ?", keys).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to load archive settings: %w", err)
	}

	result := &models.ArchiveSettings{
		DirTemplate:     blankpath.DefaultDirTemplate,
		FileTemplate:    blankpath.DefaultFileTemplate,
		MinFreeBytes:    defaultArchiveMinFreeBytes,
		WarnPercent:     defaultArchiveWarnPercent,
		RecheckDays:     defaultArchiveRecheckDays,
		FreezeAfterDays: defaultArchiveFreezeAfterDays,
		ZipMaxBytes:     defaultArchiveZipMaxBytes,
	}
	for _, row := range rows {
		switch row.Key {
		case archiveEnabledKey:
			result.Enabled = row.Value == "true"
		case archiveDirTemplateKey:
			result.DirTemplate = row.Value
		case archiveFileTemplateKey:
			result.FileTemplate = row.Value
		case archiveQuotaBytesKey:
			result.QuotaBytes = parseInt64Or(row.Value, result.QuotaBytes)
		case archiveMinFreeBytesKey:
			result.MinFreeBytes = parseInt64Or(row.Value, result.MinFreeBytes)
		case archiveWarnPercentKey:
			result.WarnPercent = parseIntOr(row.Value, result.WarnPercent)
		case archiveRecheckDaysKey:
			result.RecheckDays = parseIntOr(row.Value, result.RecheckDays)
		case archiveFreezeAfterDaysKey:
			result.FreezeAfterDays = parseIntOr(row.Value, result.FreezeAfterDays)
		case archiveZipMaxBytesKey:
			result.ZipMaxBytes = parseInt64Or(row.Value, result.ZipMaxBytes)
		}
	}
	return result, nil
}

// UpdateArchiveSettings сохраняет присланные поля настроек архива и возвращает их
// целиком. Отсутствующее поле не трогается: форма правит настройки по одной, и
// сохранение шаблона не должно сбрасывать квоту.
//
// Шаблоны проверяются до записи: шаблон с неизвестным плейсхолдером или с ключом не
// того контекста ({тип} в имени папки) сохранять нельзя - воркер молча разложил бы
// файлы не туда, и обнаружилось бы это на первой же выгрузке.
func (s *settingsService) UpdateArchiveSettings(ctx context.Context, req models.UpdateArchiveSettingsRequest) (*models.ArchiveSettings, error) {
	if req.DirTemplate != nil {
		if err := blankpath.Validate(*req.DirTemplate, blankpath.ScopeDir); err != nil {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "Шаблон папок: "+err.Error())
		}
	}
	if req.FileTemplate != nil {
		if err := blankpath.Validate(*req.FileTemplate, blankpath.ScopeFile); err != nil {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "Шаблон имени файла: "+err.Error())
		}
	}
	if err := validateArchiveNumbers(req); err != nil {
		return nil, err
	}

	writes := []struct {
		key, value, typ string
		set             bool
	}{
		{archiveEnabledKey, boolValue(req.Enabled), "bool", req.Enabled != nil},
		{archiveDirTemplateKey, stringValue(req.DirTemplate), "string", req.DirTemplate != nil},
		{archiveFileTemplateKey, stringValue(req.FileTemplate), "string", req.FileTemplate != nil},
		{archiveQuotaBytesKey, int64Value(req.QuotaBytes), "int", req.QuotaBytes != nil},
		{archiveMinFreeBytesKey, int64Value(req.MinFreeBytes), "int", req.MinFreeBytes != nil},
		{archiveWarnPercentKey, intValue(req.WarnPercent), "int", req.WarnPercent != nil},
		{archiveRecheckDaysKey, intValue(req.RecheckDays), "int", req.RecheckDays != nil},
		{archiveFreezeAfterDaysKey, intValue(req.FreezeAfterDays), "int", req.FreezeAfterDays != nil},
		{archiveZipMaxBytesKey, int64Value(req.ZipMaxBytes), "int", req.ZipMaxBytes != nil},
	}
	for _, w := range writes {
		if !w.set {
			continue
		}
		if err := s.upsertRaw(ctx, w.key, w.value, w.typ); err != nil {
			return nil, err
		}
	}
	return s.GetArchiveSettings(ctx)
}

// validateArchiveNumbers проверяет числовые настройки архива. Отрицательные размеры и
// нулевой потолок ZIP отбрасываются: механизм с такими значениями не «строже», а
// сломан - выгрузка не отдаст ни одного файла и объяснить это будет нечем.
func validateArchiveNumbers(req models.UpdateArchiveSettingsRequest) error {
	if req.QuotaBytes != nil && *req.QuotaBytes < 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Квота не может быть отрицательной")
	}
	if req.MinFreeBytes != nil && *req.MinFreeBytes < 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Минимум свободного места не может быть отрицательным")
	}
	if req.ZipMaxBytes != nil && *req.ZipMaxBytes <= 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Потолок выгрузки должен быть больше нуля")
	}
	if req.WarnPercent != nil && (*req.WarnPercent < archiveWarnPercentMin || *req.WarnPercent > archiveWarnPercentMax) {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf(
			"Порог предупреждения задаётся от %d до %d процентов", archiveWarnPercentMin, archiveWarnPercentMax))
	}
	if req.RecheckDays != nil && (*req.RecheckDays < archiveRecheckDaysMin || *req.RecheckDays > archiveRecheckDaysMax) {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf(
			"Окно сверки задаётся от %d до %d дней", archiveRecheckDaysMin, archiveRecheckDaysMax))
	}
	if req.FreezeAfterDays != nil && (*req.FreezeAfterDays < 0 || *req.FreezeAfterDays > archiveFreezeAfterDaysMax) {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf(
			"Срок заморозки задаётся от 0 до %d дней", archiveFreezeAfterDaysMax))
	}
	return nil
}

func boolValue(v *bool) string {
	if v == nil {
		return ""
	}
	return strconv.FormatBool(*v)
}

func stringValue(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func intValue(v *int) string {
	if v == nil {
		return ""
	}
	return strconv.Itoa(*v)
}

func int64Value(v *int64) string {
	if v == nil {
		return ""
	}
	return strconv.FormatInt(*v, 10)
}

// parseIntOr и parseInt64Or оставляют значение по умолчанию, если в базе лежит мусор:
// раздел настроек должен открыться и дать администратору всё исправить, а не падать.
func parseIntOr(s string, fallback int) int {
	if v, err := strconv.Atoi(s); err == nil {
		return v
	}
	return fallback
}

func parseInt64Or(s string, fallback int64) int64 {
	if v, err := strconv.ParseInt(s, 10, 64); err == nil {
		return v
	}
	return fallback
}
