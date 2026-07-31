package services

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// AttachmentService -- интерфейс бизнес-логики шаблонов вложений (unique_attachments).
type AttachmentService interface {
	// GetActive возвращает все активные шаблоны вложений.
	GetActive(ctx context.Context) ([]models.UniqueAttachment, error)
	// GetAll возвращает все шаблоны вложений (активные и архивные).
	GetAll(ctx context.Context) ([]models.UniqueAttachment, error)
	// GetByID возвращает активный шаблон вложения по ID.
	GetByID(ctx context.Context, id int) (*models.UniqueAttachment, error)
	// Create создаёт новый шаблон вложения.
	Create(ctx context.Context, userID int, req models.CreateUniqueAttachmentRequest) (*models.CreateUniqueAttachmentResponse, error)
	// Update обновляет существующий шаблон вложения.
	Update(ctx context.Context, userID, id int, req models.UpdateUniqueAttachmentRequest) error
	// Delete выполняет мягкое удаление шаблона вложения.
	Delete(ctx context.Context, userID, id int) error
	// Restore восстанавливает ранее удалённый шаблон вложения.
	Restore(ctx context.Context, userID, id int) error
	// GetHistory возвращает историю изменений шаблона вложения (новые сверху).
	GetHistory(ctx context.Context, id int) ([]models.UniqueAttachmentHistoryItem, error)
}

type attachmentService struct {
	db       *gorm.DB
	recorder AuditRecorder
}

// NewAttachmentService создаёт новый экземпляр AttachmentService.
func NewAttachmentService(db *gorm.DB) AttachmentService {
	return &attachmentService{db: db, recorder: NewAuditRecorder(db)}
}

// GetActive возвращает все активные шаблоны вложений.
func (s *attachmentService) GetActive(ctx context.Context) ([]models.UniqueAttachment, error) {
	attachments := make([]models.UniqueAttachment, 0)
	err := s.db.WithContext(ctx).
		Where("is_active = ?", true).
		Order("title, display_name").
		Find(&attachments).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching attachments")
	}
	return attachments, nil
}

// GetAll возвращает все шаблоны вложений, включая архивные.
func (s *attachmentService) GetAll(ctx context.Context) ([]models.UniqueAttachment, error) {
	attachments := make([]models.UniqueAttachment, 0)
	err := s.db.WithContext(ctx).
		Order("is_active DESC, title, display_name").
		Find(&attachments).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching attachments")
	}
	return attachments, nil
}

// GetByID возвращает активный шаблон вложения по ID.
func (s *attachmentService) GetByID(ctx context.Context, id int) (*models.UniqueAttachment, error) {
	var attachment models.UniqueAttachment
	err := s.db.WithContext(ctx).
		Where("id = ? AND is_active = ?", id, true).
		First(&attachment).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, echo.NewHTTPError(http.StatusNotFound, "Attachment not found")
		}
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching attachment")
	}
	return &attachment, nil
}

// Create создаёт новый шаблон вложения с проверкой уникальности имени.
func (s *attachmentService) Create(ctx context.Context, userID int, req models.CreateUniqueAttachmentRequest) (*models.CreateUniqueAttachmentResponse, error) {
	// Проверяем уникальность имени среди активных
	var count int64
	err := s.db.WithContext(ctx).
		Model(&models.UniqueAttachment{}).
		Where("name = ? AND is_active = ?", req.Name, true).
		Count(&count).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error checking attachment existence")
	}
	if count > 0 {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Attachment with this name already exists")
	}

	titleUpper := strings.ToUpper(req.Title)
	attachment := models.UniqueAttachment{
		AttachmentType: req.AttachmentType,
		Name:           &req.Name,
		DisplayName:    &req.DisplayName,
		Title:          &titleUpper,
		Instruction:    req.Instruction,
		IsActive:       true,
		AutoExport:     req.AutoExport == nil || *req.AutoExport,
	}

	// Запись идёт транзакцией из двух шагов: у auto_export задан default:true, а gorm
	// выбрасывает из INSERT поля с нулевым значением, когда у колонки есть значение
	// по умолчанию - выключенный тумблер молча превращался бы во включённый, и бланки
	// типа уезжали бы в архив вопреки решению администратора (#1615). Судить по
	// attachment.AutoExport после вставки нельзя: там уже default из базы.
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&attachment).Error; err != nil {
			return err
		}
		if req.AutoExport != nil && !*req.AutoExport {
			if err := tx.Model(&models.UniqueAttachment{}).
				Where("id = ?", attachment.ID).Update("auto_export", false).Error; err != nil {
				return err
			}
			attachment.AutoExport = false
		}
		return nil
	})
	if err != nil {
		slog.Error("failed to create attachment", "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка при создании вложения")
	}

	s.recorder.Log(ctx, nil, models.AuditEntityUniqueAttachment, &attachment.ID, models.UniqueAttachmentActionCreated, &userID, map[string]any{"display_name": req.DisplayName})
	return &models.CreateUniqueAttachmentResponse{
		ID:      attachment.ID,
		Message: "Вложение успешно создано",
	}, nil
}

// Update обновляет существующий шаблон вложения и логирует diff изменённых полей.
func (s *attachmentService) Update(ctx context.Context, userID, id int, req models.UpdateUniqueAttachmentRequest) error {
	titleUpper := strings.ToUpper(req.Title)
	var details map[string]any
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Снимок до изменений - для diff в истории. First даёт чистый 404, если нет.
		var prev models.UniqueAttachment
		if err := tx.Where("id = ? AND is_active = ?", id, true).First(&prev).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return echo.NewHTTPError(http.StatusNotFound, "Вложение не найдено")
			}
			return echo.NewHTTPError(http.StatusInternalServerError, "Error fetching attachment")
		}

		fields := map[string]interface{}{
			"attachment_type": req.AttachmentType,
			"name":            req.Name,
			"display_name":    req.DisplayName,
			"title":           titleUpper,
			"instruction":     req.Instruction,
		}
		// Тумблер архива обновляем только когда клиент его прислал: форма шаблона
		// вложения и форма настроек архива -- разные экраны, и одна не должна гасить
		// настройку другой.
		if req.AutoExport != nil {
			fields["auto_export"] = *req.AutoExport
		}
		if err := tx.Model(&models.UniqueAttachment{}).
			Where("id = ?", id).
			Updates(fields).Error; err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Error updating attachment")
		}
		details = buildAttachmentUpdateDetails(prev, req, titleUpper)
		return nil
	})
	if err != nil {
		return err
	}

	// Логируем только если что-то реально изменилось - иначе спам "Изменены данные".
	if len(details) > 0 {
		s.recorder.Log(ctx, nil, models.AuditEntityUniqueAttachment, &id, models.UniqueAttachmentActionUpdated, &userID, details)
	}
	return nil
}

// Delete архивирует шаблон вложения (soft-delete через is_active=false).
func (s *attachmentService) Delete(ctx context.Context, userID, id int) error {
	var attachment models.UniqueAttachment
	if err := s.db.WithContext(ctx).First(&attachment, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "Вложение не найдено")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Error fetching attachment")
	}
	if !attachment.IsActive {
		return nil // уже в архиве - идемпотентно, не дублируем запись истории
	}
	if err := s.db.WithContext(ctx).Model(&attachment).Update("is_active", false).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error deleting attachment")
	}
	s.recorder.Log(ctx, nil, models.AuditEntityUniqueAttachment, &id, models.UniqueAttachmentActionArchived, &userID, nil)
	return nil
}

// Restore восстанавливает ранее удалённый шаблон вложения (is_active=true).
func (s *attachmentService) Restore(ctx context.Context, userID, id int) error {
	var attachment models.UniqueAttachment
	if err := s.db.WithContext(ctx).First(&attachment, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "Вложение не найдено")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Error fetching attachment")
	}
	if attachment.IsActive {
		return nil // уже активно - идемпотентно
	}
	if err := s.db.WithContext(ctx).Model(&attachment).Update("is_active", true).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error restoring attachment")
	}
	s.recorder.Log(ctx, nil, models.AuditEntityUniqueAttachment, &id, models.UniqueAttachmentActionRestored, &userID, nil)
	return nil
}

// GetHistory возвращает историю изменений шаблона вложения (новые сверху).
// #870, финал F.2: запись и до-cutover строки живут в общем audit_log (старые
// перенесены backfill'ом BackfillAuditFromLegacy), поэтому чтение идёт только из
// audit_log. Замороженная unique_attachment_histories дропнута в дроп-sweep (F.8).
// Форму стережёт TestAttachments_History_*.
func (s *attachmentService) GetHistory(ctx context.Context, id int) ([]models.UniqueAttachmentHistoryItem, error) {
	const actorName = `COALESCE(NULLIF(TRIM(BOTH ' ' FROM CONCAT_WS(' ', u.last_name, u.first_name)), ''), u.username, '')`
	sql := `
		SELECT a.id AS id, a.action AS action_type, a.details AS details,
			a.actor_user_id AS actor_user_id, ` + actorName + ` AS actor_name, a.created_at AS created_at
		FROM audit_log a LEFT JOIN users u ON u.id = a.actor_user_id
		WHERE a.entity_type = ? AND a.entity_id = ?
		ORDER BY a.created_at DESC, a.id DESC`

	type row struct {
		ID          int             `gorm:"column:id"`
		ActionType  string          `gorm:"column:action_type"`
		Details     json.RawMessage `gorm:"column:details"`
		ActorUserID *int            `gorm:"column:actor_user_id"`
		ActorName   string          `gorm:"column:actor_name"`
		CreatedAt   time.Time       `gorm:"column:created_at"`
	}
	var rows []row
	if err := s.db.WithContext(ctx).Raw(sql, models.AuditEntityUniqueAttachment, id).Scan(&rows).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching attachment history")
	}

	// Логин вместо ФИО у акторов, не давших согласия на обработку данных.
	masks := loadConsentMasks(ctx, s.db)
	items := make([]models.UniqueAttachmentHistoryItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, models.UniqueAttachmentHistoryItem{
			ID:          r.ID,
			ActionType:  r.ActionType,
			Details:     r.Details,
			ActorUserID: r.ActorUserID,
			ActorName:   maskName(masks, r.ActorUserID, r.ActorName),
			CreatedAt:   r.CreatedAt,
		})
	}
	return items, nil
}

// buildAttachmentUpdateDetails собирает diff изменённых полей шаблона вложения как {old, new}.
// В результат попадают только реально изменившиеся поля - иначе история засоряется
// "пустыми" записями (см. ui-history: фильтр неизменённого обязателен). Title сравнивается
// с уже приведённым к верхнему регистру значением (titleUpper), как его пишет Update.
func buildAttachmentUpdateDetails(prev models.UniqueAttachment, req models.UpdateUniqueAttachmentRequest, titleUpper string) map[string]any {
	details := map[string]any{}
	if prev.AttachmentType != req.AttachmentType {
		details["attachment_type"] = map[string]any{"old": prev.AttachmentType, "new": req.AttachmentType}
	}
	if strPtrVal(prev.Name) != req.Name {
		details["name"] = map[string]any{"old": strPtrVal(prev.Name), "new": req.Name}
	}
	if strPtrVal(prev.DisplayName) != req.DisplayName {
		details["display_name"] = map[string]any{"old": strPtrVal(prev.DisplayName), "new": req.DisplayName}
	}
	if strPtrVal(prev.Title) != titleUpper {
		details["title"] = map[string]any{"old": strPtrVal(prev.Title), "new": titleUpper}
	}
	if strPtrVal(prev.Instruction) != strPtrVal(req.Instruction) {
		details["instruction"] = map[string]any{"old": strPtrVal(prev.Instruction), "new": strPtrVal(req.Instruction)}
	}
	if req.AutoExport != nil && prev.AutoExport != *req.AutoExport {
		details["auto_export"] = map[string]any{"old": prev.AutoExport, "new": *req.AutoExport}
	}
	return details
}
