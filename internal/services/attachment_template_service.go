package services

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

// AttachmentTemplateService - CRUD шаблонов Excel-бланков (#183).
// Генерация .xlsx вынесена в отдельный сервис AttachmentBlankService
// (будет реализована в следующем PR).
type AttachmentTemplateService interface {
	Get(ctx context.Context, uniqueAttachmentID int) (*models.AttachmentTemplate, error)
	ListTemplates(ctx context.Context, uniqueAttachmentID int) ([]models.AttachmentTemplate, error)
	Upload(ctx context.Context, uniqueAttachmentID int, file *multipart.FileHeader, req models.CreateTemplateRequest, userID int) (*models.AttachmentTemplate, error)
	UpdateMappings(ctx context.Context, uniqueAttachmentID int, req models.UpdateMappingsRequest) error
	CopyMappings(ctx context.Context, uniqueAttachmentID int, req models.CopyMappingsRequest) (*models.CopyMappingsResult, error)
	ListTemplateSources(ctx context.Context) ([]models.TemplateSource, error)
	UpdateParams(ctx context.Context, uniqueAttachmentID int, req models.UpdateTemplateParamsRequest) error
	Delete(ctx context.Context, uniqueAttachmentID int) error
	DeleteByID(ctx context.Context, templateID int) error
	SetActive(ctx context.Context, uniqueAttachmentID int, templateID int) error
	DeactivateAll(ctx context.Context, uniqueAttachmentID int) error
	GetByID(ctx context.Context, templateID int) (*models.AttachmentTemplate, error)
	// Custom fields
	ListCustomFields(ctx context.Context, uniqueAttachmentID int) ([]models.AttachmentCustomField, error)
	CreateCustomField(ctx context.Context, uniqueAttachmentID int, req models.CreateCustomFieldRequest) (*models.AttachmentCustomField, error)
	UpdateCustomField(ctx context.Context, id int, req models.CreateCustomFieldRequest) error
	DeleteCustomField(ctx context.Context, id int) error
}

type attachmentTemplateService struct {
	db         *gorm.DB
	uploadPath string // базовый путь, обычно cfg.UploadPath. Шаблоны в <uploadPath>/templates/
}

// NewAttachmentTemplateService создаёт сервис.
func NewAttachmentTemplateService(db *gorm.DB, uploadPath string) AttachmentTemplateService {
	return &attachmentTemplateService{db: db, uploadPath: uploadPath}
}

func (s *attachmentTemplateService) Get(ctx context.Context, uaID int) (*models.AttachmentTemplate, error) {
	var t models.AttachmentTemplate
	err := s.db.WithContext(ctx).
		Preload("Mappings").
		Where("unique_attachment_id = ? AND is_active = ?", uaID, true).
		First(&t).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, echo.NewHTTPError(http.StatusNotFound, "Шаблон не настроен")
		}
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка получения шаблона")
	}
	return &t, nil
}

func (s *attachmentTemplateService) GetByID(ctx context.Context, templateID int) (*models.AttachmentTemplate, error) {
	var t models.AttachmentTemplate
	err := s.db.WithContext(ctx).Preload("Mappings").First(&t, templateID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, echo.NewHTTPError(http.StatusNotFound, "Шаблон не найден")
		}
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка получения шаблона")
	}
	return &t, nil
}

func (s *attachmentTemplateService) ListTemplates(ctx context.Context, uaID int) ([]models.AttachmentTemplate, error) {
	var templates []models.AttachmentTemplate
	err := s.db.WithContext(ctx).
		Preload("Mappings").
		Where("unique_attachment_id = ?", uaID).
		Order("created_at DESC").
		Find(&templates).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка получения шаблонов")
	}
	return templates, nil
}

func (s *attachmentTemplateService) Upload(ctx context.Context, uaID int, file *multipart.FileHeader, req models.CreateTemplateRequest, userID int) (*models.AttachmentTemplate, error) {
	var ua models.UniqueAttachment
	if err := s.db.WithContext(ctx).First(&ua, uaID).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusNotFound, "Вложение не найдено")
	}

	dir := filepath.Join(s.uploadPath, "templates")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Не удалось создать директорию шаблонов")
	}
	dst := filepath.Join(dir, fmt.Sprintf("%d_%d.xlsx", uaID, time.Now().UnixMilli()))
	if err := saveMultipartFile(file, dst); err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Не удалось сохранить файл")
	}

	t := models.AttachmentTemplate{
		UniqueAttachmentID: uaID,
		IsActive:           true,
		FilePath:           dst,
		OriginalFileName:   file.Filename,
		ListStartRow:       req.ListStartRow,
		ListEndRow:         req.ListEndRow,
		MaxListRows:        req.MaxListRows,
		UploadedByUserID:   &userID,
	}
	if t.MaxListRows == 0 && t.ListStartRow > 0 && t.ListEndRow >= t.ListStartRow {
		t.MaxListRows = t.ListEndRow - t.ListStartRow + 1
	}

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.AttachmentTemplate{}).
			Where("unique_attachment_id = ? AND is_active = ?", uaID, true).
			Update("is_active", false).Error; err != nil {
			return fmt.Errorf("failed to deactivate old templates: %w", err)
		}
		return tx.Create(&t).Error
	})
	if err != nil {
		_ = os.Remove(dst)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Не удалось сохранить шаблон")
	}
	return &t, nil
}

func (s *attachmentTemplateService) UpdateMappings(ctx context.Context, uaID int, req models.UpdateMappingsRequest) error {
	var t models.AttachmentTemplate
	if err := s.db.WithContext(ctx).Where("unique_attachment_id = ? AND is_active = ?", uaID, true).First(&t).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "Шаблон не настроен")
	}

	// Валидация: все field_path должны быть либо встроенными, либо custom-полями этого UA.
	customIDs := map[string]struct{}{}
	var customFields []models.AttachmentCustomField
	s.db.WithContext(ctx).Where("unique_attachment_id = ?", uaID).Find(&customFields)
	for _, cf := range customFields {
		customIDs[fmt.Sprintf("custom.%d", cf.ID)] = struct{}{}
	}
	for _, m := range req.Mappings {
		if !IsValidFieldPath(m.FieldPath) {
			if _, ok := customIDs[m.FieldPath]; !ok {
				return echo.NewHTTPError(http.StatusBadRequest,
					fmt.Sprintf("Неизвестное поле: %s", m.FieldPath))
			}
		}
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if req.ConcatSeparator != nil {
			if err := tx.Model(&t).Update("concat_separator", *req.ConcatSeparator).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("template_id = ?", t.ID).Delete(&models.AttachmentTemplateMapping{}).Error; err != nil {
			return err
		}
		if len(req.Mappings) == 0 {
			return nil
		}
		rows := make([]models.AttachmentTemplateMapping, 0, len(req.Mappings))
		for _, m := range req.Mappings {
			rows = append(rows, models.AttachmentTemplateMapping{
				TemplateID:  t.ID,
				CellRef:     m.CellRef,
				FieldPath:   m.FieldPath,
				IsListField: m.IsListField,
			})
		}
		return tx.Create(&rows).Error
	})
}

// ListTemplateSources - шаблоны, с которых можно перенести привязки. Отдаём все,
// включая неактивные и шаблоны архивных типов: настраивая новый тип вложения, админ
// чаще всего копирует именно со старого. Пустые шаблоны прячет интерфейс по счётчику.
func (s *attachmentTemplateService) ListTemplateSources(ctx context.Context) ([]models.TemplateSource, error) {
	rows := make([]models.TemplateSource, 0)
	err := s.db.WithContext(ctx).Raw(`
		SELECT t.id AS template_id,
		       t.unique_attachment_id,
		       t.is_active,
		       t.original_file_name,
		       ua.attachment_type,
		       COALESCE(NULLIF(ua.display_name, ''), NULLIF(ua.title, ''), ua.name, '') AS attachment_name,
		       (SELECT count(*) FROM attachment_template_mappings m WHERE m.template_id = t.id) AS mappings_count
		FROM attachment_templates t
		JOIN unique_attachments ua ON ua.id = t.unique_attachment_id
		ORDER BY attachment_name, t.is_active DESC, t.created_at DESC
	`).Scan(&rows).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Не удалось получить список шаблонов")
	}
	return rows, nil
}

// CopyMappings переносит пары ячейка-поле с другого шаблона в активный шаблон цели.
// Что не переносится: привязки списка чужой группы (у цели другой тип вложения, и
// заполнять их нечем) и кастомные поля, которых у цели нет - id таких полей
// принадлежат своему типу вложения, поэтому сопоставляем их по названию.
func (s *attachmentTemplateService) CopyMappings(ctx context.Context, uaID int, req models.CopyMappingsRequest) (*models.CopyMappingsResult, error) {
	var target models.AttachmentTemplate
	if err := s.db.WithContext(ctx).Where("unique_attachment_id = ? AND is_active = ?", uaID, true).First(&target).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusNotFound, "Шаблон не настроен")
	}
	var source models.AttachmentTemplate
	if err := s.db.WithContext(ctx).Preload("Mappings").First(&source, req.SourceTemplateID).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusNotFound, "Шаблон-источник не найден")
	}
	if source.ID == target.ID {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Источник и цель - один и тот же шаблон")
	}
	var targetUA models.UniqueAttachment
	if err := s.db.WithContext(ctx).First(&targetUA, uaID).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusNotFound, "Вложение не найдено")
	}

	prefix := ListFieldPrefix(targetUA.AttachmentType)
	targetCustom := s.customFieldsByLabel(ctx, uaID)
	sourceCustom := s.customLabelsByID(ctx, source.UniqueAttachmentID)

	// В режиме добавления уже существующие пары пропускаем, чтобы повторный перенос
	// не удваивал привязки: совмещение полей задаётся разными field_path в одной
	// ячейке, а полный дубль пары смысла не несёт.
	taken := make(map[string]struct{})
	if !req.Replace {
		var current []models.AttachmentTemplateMapping
		s.db.WithContext(ctx).Where("template_id = ?", target.ID).Find(&current)
		for _, m := range current {
			taken[m.CellRef+"|"+m.FieldPath] = struct{}{}
		}
	}

	res := &models.CopyMappingsResult{}
	rows := make([]models.AttachmentTemplateMapping, 0, len(source.Mappings))
	for _, m := range source.Mappings {
		fieldPath := m.FieldPath
		remapped := false
		switch {
		case strings.HasPrefix(fieldPath, "custom."):
			mapped, ok := remapCustomPath(fieldPath, sourceCustom, targetCustom)
			if !ok {
				res.SkippedCustom++
				continue
			}
			fieldPath = mapped
			remapped = true
		case m.IsListField && (prefix == "" || !strings.HasPrefix(fieldPath, prefix)):
			res.SkippedForeignList++
			continue
		}
		key := m.CellRef + "|" + fieldPath
		if _, dup := taken[key]; dup {
			res.SkippedDuplicates++
			continue
		}
		if remapped {
			res.RemappedCustom++
		}
		taken[key] = struct{}{}
		rows = append(rows, models.AttachmentTemplateMapping{
			TemplateID:  target.ID,
			CellRef:     m.CellRef,
			FieldPath:   fieldPath,
			IsListField: m.IsListField,
		})
	}
	res.Copied = len(rows)

	paramsCopied := req.CopyParams
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if req.Replace {
			if err := tx.Where("template_id = ?", target.ID).Delete(&models.AttachmentTemplateMapping{}).Error; err != nil {
				return fmt.Errorf("failed to clear target mappings: %w", err)
			}
		}
		if len(rows) > 0 {
			if err := tx.Create(&rows).Error; err != nil {
				return fmt.Errorf("failed to copy mappings: %w", err)
			}
		}
		// Границы переносим только осмысленные: копирование идёт мимо валидации
		// UpdateParams, а нулевая строка начала списка сломала бы генерацию бланка.
		if !req.CopyParams || source.ListStartRow < 1 || source.ListEndRow < source.ListStartRow {
			paramsCopied = false
			return nil
		}
		maxRows := source.MaxListRows
		if maxRows == 0 && source.ListEndRow >= source.ListStartRow {
			maxRows = source.ListEndRow - source.ListStartRow + 1
		}
		itemsMax := source.ItemsMaxListRows
		if itemsMax == 0 && source.ItemsListStartRow > 0 && source.ItemsListEndRow >= source.ItemsListStartRow {
			itemsMax = source.ItemsListEndRow - source.ItemsListStartRow + 1
		}
		updates := map[string]any{
			"list_start_row":       source.ListStartRow,
			"list_end_row":         source.ListEndRow,
			"max_list_rows":        maxRows,
			"concat_separator":     source.ConcatSeparator,
			"items_list_start_row": source.ItemsListStartRow,
			"items_list_end_row":   source.ItemsListEndRow,
			"items_max_list_rows":  itemsMax,
		}
		return tx.Model(&models.AttachmentTemplate{}).Where("id = ?", target.ID).Updates(updates).Error
	})
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Не удалось перенести привязки")
	}
	res.ParamsCopied = paramsCopied
	return res, nil
}

// customFieldsByLabel - активные кастомные поля вложения по нормализованному названию.
func (s *attachmentTemplateService) customFieldsByLabel(ctx context.Context, uaID int) map[string]int {
	var fields []models.AttachmentCustomField
	s.db.WithContext(ctx).Where("unique_attachment_id = ? AND is_active = ?", uaID, true).Find(&fields)
	out := make(map[string]int, len(fields))
	for _, cf := range fields {
		out[normalizeCustomLabel(cf.Label)] = cf.ID
	}
	return out
}

// customLabelsByID - названия кастомных полей вложения-источника по их id. Архивные
// тоже нужны: привязка могла остаться с поля, которое потом выключили.
func (s *attachmentTemplateService) customLabelsByID(ctx context.Context, uaID int) map[int]string {
	var fields []models.AttachmentCustomField
	s.db.WithContext(ctx).Where("unique_attachment_id = ?", uaID).Find(&fields)
	out := make(map[int]string, len(fields))
	for _, cf := range fields {
		out[cf.ID] = normalizeCustomLabel(cf.Label)
	}
	return out
}

// remapCustomPath переводит custom.<id> источника в custom.<id> цели по названию поля.
func remapCustomPath(path string, sourceLabels map[int]string, targetIDs map[string]int) (string, bool) {
	id, err := strconv.Atoi(strings.TrimPrefix(path, "custom."))
	if err != nil {
		return "", false
	}
	label, ok := sourceLabels[id]
	if !ok || label == "" {
		return "", false
	}
	newID, ok := targetIDs[label]
	if !ok {
		return "", false
	}
	return fmt.Sprintf("custom.%d", newID), true
}

func normalizeCustomLabel(label string) string {
	return strings.ToLower(strings.Join(strings.Fields(label), " "))
}

// itemsSectionStart - верхняя строка, куда привязаны поля группы «Имущество (список)».
// Ноль - привязок нет, значит и таблице ТМЦ в бланке начинаться неоткуда.
func (s *attachmentTemplateService) itemsSectionStart(ctx context.Context, templateID int) int {
	var refs []string
	s.db.WithContext(ctx).Model(&models.AttachmentTemplateMapping{}).
		Where("template_id = ? AND field_path LIKE ?", templateID, "item.%").
		Pluck("cell_ref", &refs)

	start := 0
	for _, ref := range refs {
		_, row, err := excelize.CellNameToCoordinates(ref)
		if err != nil || row < 1 {
			continue
		}
		if start == 0 || row < start {
			start = row
		}
	}
	return start
}

// rangesOverlap - пересекаются ли два диапазона строк включительно.
func rangesOverlap(aStart, aEnd, bStart, bEnd int) bool {
	return aStart <= bEnd && bStart <= aEnd
}

// UpdateParams меняет границы строк списка у активного шаблона. Раньше их задавали
// только вместе с загрузкой файла, поэтому подвинуть диапазон значило перезалить
// тот же .xlsx заново (#1454).
func (s *attachmentTemplateService) UpdateParams(ctx context.Context, uaID int, req models.UpdateTemplateParamsRequest) error {
	if req.ListStartRow < 1 || req.ListEndRow < req.ListStartRow {
		return echo.NewHTTPError(http.StatusBadRequest, "Некорректный диапазон строк")
	}
	var t models.AttachmentTemplate
	if err := s.db.WithContext(ctx).Where("unique_attachment_id = ? AND is_active = ?", uaID, true).First(&t).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "Шаблон не настроен")
	}
	maxRows := req.MaxListRows
	if maxRows == 0 {
		maxRows = req.ListEndRow - req.ListStartRow + 1
	}
	// Таблица ТМЦ задаётся одним числом - сколько строк под неё отведено. Где она
	// начинается, видно по привязкам полей ТМЦ, поэтому границы считаем сами и храним
	// как снимок: так их видно в API и в переносе привязок между шаблонами.
	itemsMax := req.ItemsMaxListRows
	itemsStart, itemsEnd := 0, 0
	if itemsMax > 0 {
		itemsStart = s.itemsSectionStart(ctx, t.ID)
		if itemsStart == 0 {
			return echo.NewHTTPError(http.StatusBadRequest,
				"Сначала привяжите поля ТМЦ к ячейкам бланка - по ним определяется начало таблицы")
		}
		itemsEnd = itemsStart + itemsMax - 1
		if rangesOverlap(req.ListStartRow, req.ListEndRow, itemsStart, itemsEnd) {
			return echo.NewHTTPError(http.StatusBadRequest, "Таблица ТМЦ накладывается на строки списка")
		}
	}
	err := s.db.WithContext(ctx).Model(&models.AttachmentTemplate{}).Where("id = ?", t.ID).
		Updates(map[string]any{
			"list_start_row":       req.ListStartRow,
			"list_end_row":         req.ListEndRow,
			"max_list_rows":        maxRows,
			"items_list_start_row": itemsStart,
			"items_list_end_row":   itemsEnd,
			"items_max_list_rows":  itemsMax,
		}).Error
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Не удалось сохранить параметры списка")
	}
	return nil
}

func (s *attachmentTemplateService) Delete(ctx context.Context, uaID int) error {
	var t models.AttachmentTemplate
	if err := s.db.WithContext(ctx).Where("unique_attachment_id = ? AND is_active = ?", uaID, true).First(&t).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка")
	}
	return s.deleteTemplate(ctx, &t)
}

func (s *attachmentTemplateService) DeleteByID(ctx context.Context, templateID int) error {
	var t models.AttachmentTemplate
	if err := s.db.WithContext(ctx).First(&t, templateID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "Шаблон не найден")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка")
	}
	// Удалив активный файл, генерацию не выключаем: у вложения обычно несколько
	// бланков, и настройка должна остаться рабочей - активным становится самый свежий
	// из оставшихся.
	wasActive := t.IsActive
	if err := s.deleteTemplate(ctx, &t); err != nil {
		return err
	}
	if wasActive {
		var next models.AttachmentTemplate
		if err := s.db.WithContext(ctx).
			Where("unique_attachment_id = ?", t.UniqueAttachmentID).
			Order("created_at DESC").
			First(&next).Error; err == nil {
			s.db.WithContext(ctx).Model(&next).Update("is_active", true)
		}
	}
	return nil
}

func (s *attachmentTemplateService) deleteTemplate(ctx context.Context, t *models.AttachmentTemplate) error {
	_ = os.Remove(t.FilePath)
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("template_id = ?", t.ID).Delete(&models.AttachmentTemplateMapping{}).Error; err != nil {
			return err
		}
		return tx.Delete(t).Error
	})
}

func (s *attachmentTemplateService) DeactivateAll(ctx context.Context, uaID int) error {
	res := s.db.WithContext(ctx).Model(&models.AttachmentTemplate{}).
		Where("unique_attachment_id = ? AND is_active = ?", uaID, true).
		Update("is_active", false)
	if res.Error != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Не удалось деактивировать шаблоны")
	}
	return nil
}

func (s *attachmentTemplateService) SetActive(ctx context.Context, uaID int, templateID int) error {
	var t models.AttachmentTemplate
	if err := s.db.WithContext(ctx).Where("id = ? AND unique_attachment_id = ?", templateID, uaID).First(&t).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "Шаблон не найден")
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.AttachmentTemplate{}).
			Where("unique_attachment_id = ? AND is_active = ?", uaID, true).
			Update("is_active", false).Error; err != nil {
			return err
		}
		return tx.Model(&t).Update("is_active", true).Error
	})
}

func (s *attachmentTemplateService) ListCustomFields(ctx context.Context, uaID int) ([]models.AttachmentCustomField, error) {
	fields := make([]models.AttachmentCustomField, 0)
	err := s.db.WithContext(ctx).
		Where("unique_attachment_id = ? AND is_active = ?", uaID, true).
		Order("sort_order, id").
		Find(&fields).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка получения полей")
	}
	return fields, nil
}

func (s *attachmentTemplateService) CreateCustomField(ctx context.Context, uaID int, req models.CreateCustomFieldRequest) (*models.AttachmentCustomField, error) {
	cf := models.AttachmentCustomField{
		UniqueAttachmentID: uaID,
		Label:              req.Label,
		Placeholder:        req.Placeholder,
		SortOrder:          req.SortOrder,
		IsRequired:         req.IsRequired,
		IsActive:           true,
	}
	if err := s.db.WithContext(ctx).Create(&cf).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Не удалось создать поле")
	}
	return &cf, nil
}

func (s *attachmentTemplateService) UpdateCustomField(ctx context.Context, id int, req models.CreateCustomFieldRequest) error {
	updates := map[string]any{
		"label":       req.Label,
		"placeholder": req.Placeholder,
		"sort_order":  req.SortOrder,
		"is_required": req.IsRequired,
	}
	res := s.db.WithContext(ctx).Model(&models.AttachmentCustomField{}).Where("id = ?", id).Updates(updates)
	if res.Error != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Не удалось обновить поле")
	}
	if res.RowsAffected == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "Поле не найдено")
	}
	return nil
}

func (s *attachmentTemplateService) DeleteCustomField(ctx context.Context, id int) error {
	// Soft delete: is_active=false. Старые значения для уже поданных заявок остаются.
	res := s.db.WithContext(ctx).Model(&models.AttachmentCustomField{}).Where("id = ?", id).Update("is_active", false)
	if res.RowsAffected == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "Поле не найдено")
	}
	return nil
}

// saveMultipartFile - helper для сохранения multipart-файла на диск.
func saveMultipartFile(file *multipart.FileHeader, dst string) error {
	src, err := file.Open()
	if err != nil {
		return err
	}
	defer src.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, src)
	return err
}
