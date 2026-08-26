package services

import (
	"context"
	"encoding/json"
	"log/slog"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// DocumentService -- интерфейс управления документами.
type DocumentService interface {
	List(ctx context.Context, groupID *int, includeHidden bool) ([]models.DocumentListItem, error)
	Upload(ctx context.Context, userID int, req models.UploadDocumentRequest, file *multipart.FileHeader) (*models.DocumentListItem, error)
	UpdateMeta(ctx context.Context, userID int, id int, req models.UpdateDocumentMetaRequest) (*models.DocumentListItem, error)
	ReplaceFile(ctx context.Context, userID int, id int, file *multipart.FileHeader) (*models.DocumentListItem, error)
	Delete(ctx context.Context, id int) error
	Reorder(ctx context.Context, req models.ReorderDocumentsRequest) error
	GetPublic(ctx context.Context) ([]models.PublicDocumentGroup, error)
	GetByID(ctx context.Context, id int) (*models.Document, error)
}

type documentService struct {
	db                  *gorm.DB
	fileSvc             DocumentFileService
	settings            SettingsService
	notificationService NotificationService
}

// DocumentServiceOption конфигурирует documentService при создании.
type DocumentServiceOption func(*documentService)

// WithDocumentNotifications включает персональные уведомления document_published
// (#1748) при загрузке документа. Опционально: без неё уведомления не шлются
// (тесты, offline).
func WithDocumentNotifications(ns NotificationService) DocumentServiceOption {
	return func(s *documentService) { s.notificationService = ns }
}

// NewDocumentService создаёт реализацию DocumentService.
func NewDocumentService(db *gorm.DB, fileSvc DocumentFileService, settings SettingsService, opts ...DocumentServiceOption) DocumentService {
	s := &documentService{db: db, fileSvc: fileSvc, settings: settings}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// notifyDocumentPublished шлёт NotificationTypeDocumentPublished активным
// пользователям при загрузке документа (#1748). Документ становится видимым
// сразу (Upload всегда создаёт с IsVisible: true), а публичный список
// (/public/documents) отдаётся любому авторизованному без гейта прав - группы
// документов в этом проекте только категоризация, отдельной видимости у них нет
// (проверено по document_service.go/document_group_service.go: ни один из
// сервисов не фильтрует по правам, кроме requireAdmin на управление). Поэтому
// аудитория = все активные аккаунты, как и у публикации новости. Загрузившему
// не шлём - он и так знает, что опубликовал.
func (s *documentService) notifyDocumentPublished(ctx context.Context, docID int, authorID int, title string) {
	if s.notificationService == nil {
		return
	}
	ids, err := activeUserIDs(ctx, s.db)
	if err != nil {
		slog.Warn("документ опубликован: не удалось собрать аудиторию уведомления", "document_id", docID, "err", err)
		return
	}
	payload, _ := json.Marshal(map[string]any{"document_id": docID, "title": title})
	payloadStr := string(payload)
	notifTitle := "Опубликован документ"
	for _, uid := range ids {
		if uid == authorID {
			continue
		}
		if err := s.notificationService.CreateForUser(ctx, uid, NotificationTypeDocumentPublished, notifTitle, title, &payloadStr); err != nil {
			slog.Warn("не удалось уведомить о публикации документа", "document_id", docID, "user_id", uid, "error", err)
		}
	}
}

func (s *documentService) selectQuery(db *gorm.DB) *gorm.DB {
	return db.
		Table("documents d").
		Select(`d.id, d.group_id, dg.name AS group_name,
			d.title, d.description, d.file_name, d.file_ext, d.file_size,
			d.published_at, d.is_visible, d.sort_order,
			d.created_at, d.updated_at, d.created_by, d.updated_by`).
		Joins("LEFT JOIN document_groups dg ON dg.id = d.group_id")
}

func (s *documentService) List(ctx context.Context, groupID *int, includeHidden bool) ([]models.DocumentListItem, error) {
	q := s.selectQuery(s.db.WithContext(ctx))
	if groupID != nil {
		q = q.Where("d.group_id = ?", *groupID)
	}
	if !includeHidden {
		q = q.Where("d.is_visible = true")
	}

	results := make([]models.DocumentListItem, 0)
	if err := q.Order("d.sort_order ASC, d.id ASC").Scan(&results).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка получения списка документов")
	}
	return results, nil
}

func (s *documentService) Upload(ctx context.Context, userID int, req models.UploadDocumentRequest, file *multipart.FileHeader) (*models.DocumentListItem, error) {
	uploadSettings, err := s.settings.GetUploadSettings(ctx)
	if err != nil {
		return nil, err
	}
	maxSize, _ := uploadSettings["max_file_size"].(int64)
	if maxSize == 0 {
		maxSize = 10 * 1024 * 1024
	}

	storedName, ext, err := s.fileSvc.Save(ctx, file, maxSize)
	if err != nil {
		return nil, err
	}

	publishedAt := time.Now().UTC()
	if req.PublishedAt != nil && *req.PublishedAt != "" {
		if t, err := time.Parse(time.RFC3339, *req.PublishedAt); err == nil {
			publishedAt = t.UTC()
		}
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		s.fileSvc.Delete(storedName)
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Название документа не может быть пустым")
	}

	// sort_order: в конце относительно группы
	var maxOrder int
	qOrder := s.db.WithContext(ctx).Model(&models.Document{}).Select("COALESCE(MAX(sort_order), 0)")
	if req.GroupID != nil {
		qOrder = qOrder.Where("group_id = ?", *req.GroupID)
	} else {
		qOrder = qOrder.Where("group_id IS NULL")
	}
	qOrder.Scan(&maxOrder)

	now := time.Now().UTC()
	doc := models.Document{
		GroupID:     req.GroupID,
		Title:       title,
		Description: req.Description,
		FileName:    file.Filename,
		StoredName:  storedName,
		FileExt:     ext,
		MimeType:    DetectMimeType(ext),
		FileSize:    file.Size,
		PublishedAt: publishedAt,
		IsVisible:   true,
		SortOrder:   maxOrder + 1,
		CreatedBy:   &userID,
		UpdatedBy:   &userID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.db.WithContext(ctx).Create(&doc).Error; err != nil {
		s.fileSvc.Delete(storedName)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка сохранения документа")
	}

	s.notifyDocumentPublished(ctx, doc.ID, userID, title)
	return s.getItem(ctx, doc.ID)
}

func (s *documentService) UpdateMeta(ctx context.Context, userID int, id int, req models.UpdateDocumentMetaRequest) (*models.DocumentListItem, error) {
	var doc models.Document
	if err := s.db.WithContext(ctx).First(&doc, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, echo.NewHTTPError(http.StatusNotFound, "Документ не найден")
		}
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка получения документа")
	}

	updates := map[string]interface{}{
		"updated_by": userID,
		"updated_at": time.Now().UTC(),
	}
	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		if title == "" {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "Название документа не может быть пустым")
		}
		updates["title"] = title
	}
	if req.Description != nil {
		updates["description"] = req.Description
	}
	if req.GroupID != nil {
		updates["group_id"] = req.GroupID
	}
	if req.PublishedAt != nil && *req.PublishedAt != "" {
		t, err := time.Parse(time.RFC3339, *req.PublishedAt)
		if err != nil {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "Неверный формат даты публикации (ожидается RFC3339)")
		}
		updates["published_at"] = t.UTC()
	}
	if req.IsVisible != nil {
		updates["is_visible"] = *req.IsVisible
	}

	if err := s.db.WithContext(ctx).Model(&models.Document{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка обновления документа")
	}

	return s.getItem(ctx, id)
}

func (s *documentService) ReplaceFile(ctx context.Context, userID int, id int, file *multipart.FileHeader) (*models.DocumentListItem, error) {
	var doc models.Document
	if err := s.db.WithContext(ctx).First(&doc, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, echo.NewHTTPError(http.StatusNotFound, "Документ не найден")
		}
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка получения документа")
	}

	uploadSettings, err := s.settings.GetUploadSettings(ctx)
	if err != nil {
		return nil, err
	}
	maxSize, _ := uploadSettings["max_file_size"].(int64)
	if maxSize == 0 {
		maxSize = 10 * 1024 * 1024
	}

	newStoredName, ext, err := s.fileSvc.Save(ctx, file, maxSize)
	if err != nil {
		return nil, err
	}

	oldStoredName := doc.StoredName
	now := time.Now().UTC()

	if err := s.db.WithContext(ctx).Model(&models.Document{}).Where("id = ?", id).Updates(map[string]interface{}{
		"file_name":   file.Filename,
		"stored_name": newStoredName,
		"file_ext":    ext,
		"mime_type":   DetectMimeType(ext),
		"file_size":   file.Size,
		"updated_by":  userID,
		"updated_at":  now,
	}).Error; err != nil {
		s.fileSvc.Delete(newStoredName)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка обновления документа")
	}

	// Удаляем старый файл только после успешной записи в БД
	s.fileSvc.Delete(oldStoredName)

	return s.getItem(ctx, id)
}

func (s *documentService) Delete(ctx context.Context, id int) error {
	var doc models.Document
	if err := s.db.WithContext(ctx).First(&doc, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "Документ не найден")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка получения документа")
	}

	storedName := doc.StoredName

	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&models.Document{}, id).Error; err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка удаления документа")
		}
		return nil
	}); err != nil {
		return err
	}
	// Удаляем файл с диска только после успешного коммита транзакции
	s.fileSvc.Delete(storedName)
	return nil
}

func (s *documentService) Reorder(ctx context.Context, req models.ReorderDocumentsRequest) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for i, id := range req.IDs {
			q := tx.Model(&models.Document{}).Where("id = ?", id)
			if req.GroupID != nil {
				q = tx.Model(&models.Document{}).Where("id = ? AND group_id = ?", id, *req.GroupID)
			}
			if err := q.Update("sort_order", i).Error; err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка обновления порядка документов")
			}
		}
		return nil
	})
}

func (s *documentService) GetPublic(ctx context.Context) ([]models.PublicDocumentGroup, error) {
	// Загружаем все видимые документы с их группами
	type row struct {
		ID          int       `gorm:"column:id"`
		GroupID     *int      `gorm:"column:group_id"`
		GroupName   *string   `gorm:"column:group_name"`
		GroupOrder  *int      `gorm:"column:group_order"`
		Title       string    `gorm:"column:title"`
		Description *string   `gorm:"column:description"`
		FileName    string    `gorm:"column:file_name"`
		FileExt     string    `gorm:"column:file_ext"`
		FileSize    int64     `gorm:"column:file_size"`
		PublishedAt time.Time `gorm:"column:published_at"`
		SortOrder   int       `gorm:"column:sort_order"`
	}

	rows := make([]row, 0)
	err := s.db.WithContext(ctx).
		Table("documents d").
		Select(`d.id, d.group_id, dg.name AS group_name, dg.sort_order AS group_order,
			d.title, d.description, d.file_name, d.file_ext, d.file_size,
			d.published_at, d.sort_order`).
		Joins("LEFT JOIN document_groups dg ON dg.id = d.group_id").
		Where("d.is_visible = true").
		Order("COALESCE(dg.sort_order, 999999) ASC, dg.id ASC, d.sort_order ASC, d.id ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка получения документов")
	}

	// Группируем по group_id; NULL -> виртуальная группа «Прочее» в конце
	type groupKey struct {
		id   *int
		name string
	}
	groupMap := make(map[int]*models.PublicDocumentGroup) // id -> group; 0 = «Прочее»
	groupOrder := make([]int, 0)                          // порядок group_id (0 для Прочего)

	for i := range rows {
		r := &rows[i]
		var key int
		if r.GroupID != nil {
			key = *r.GroupID
		} else {
			key = 0
		}

		if _, exists := groupMap[key]; !exists {
			g := &models.PublicDocumentGroup{
				Documents: make([]models.PublicDocument, 0),
			}
			if r.GroupID != nil {
				g.ID = *r.GroupID
				if r.GroupName != nil {
					g.Name = *r.GroupName
				}
				if r.GroupOrder != nil {
					g.SortOrder = *r.GroupOrder
				}
			} else {
				g.ID = 0
				g.Name = "Прочее"
				g.SortOrder = 999999
			}
			groupMap[key] = g
			groupOrder = append(groupOrder, key)
		}

		groupMap[key].Documents = append(groupMap[key].Documents, models.PublicDocument{
			ID:          r.ID,
			GroupID:     r.GroupID,
			Title:       r.Title,
			Description: r.Description,
			FileName:    r.FileName,
			FileExt:     r.FileExt,
			FileSize:    r.FileSize,
			PublishedAt: r.PublishedAt,
			SortOrder:   r.SortOrder,
		})
	}

	result := make([]models.PublicDocumentGroup, 0, len(groupOrder))
	for _, key := range groupOrder {
		if g, ok := groupMap[key]; ok {
			result = append(result, *g)
		}
	}
	return result, nil
}

func (s *documentService) GetByID(ctx context.Context, id int) (*models.Document, error) {
	var doc models.Document
	if err := s.db.WithContext(ctx).First(&doc, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, echo.NewHTTPError(http.StatusNotFound, "Документ не найден")
		}
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка получения документа")
	}
	return &doc, nil
}

func (s *documentService) getItem(ctx context.Context, id int) (*models.DocumentListItem, error) {
	var item models.DocumentListItem
	if err := s.selectQuery(s.db.WithContext(ctx)).Where("d.id = ?", id).Scan(&item).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка получения документа")
	}
	if item.ID == 0 {
		return nil, echo.NewHTTPError(http.StatusNotFound, "Документ не найден")
	}
	return &item, nil
}
