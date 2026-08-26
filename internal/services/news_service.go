package services

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/realtime"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// NewsService -- интерфейс бизнес-логики новостей и объявлений.
// Админ-операции (page.admin) авторизуются роут-middleware RequirePermissionV2;
// активные новость/объявление (GetActive*) доступны всем авторизованным.
type NewsService interface {
	// News
	GetActiveNews(ctx context.Context) ([]models.NewsWithUser, error)
	GetAllNews(ctx context.Context) ([]models.NewsWithUser, error)
	CreateNews(ctx context.Context, userID int, req models.CreateNewsRequest) (*models.NewsWithUser, error)
	UpdateNews(ctx context.Context, userID int, id int, req models.UpdateNewsRequest) (*models.NewsWithUser, error)
	DeleteNews(ctx context.Context, id int) error

	// Announcements
	GetActiveAnnouncement(ctx context.Context) (*models.AnnouncementWithUser, error)
	GetAllAnnouncements(ctx context.Context) ([]models.AnnouncementWithUser, error)
	CreateAnnouncement(ctx context.Context, userID int, req models.CreateAnnouncementRequest) (*models.AnnouncementWithUser, error)
	SetActiveAnnouncement(ctx context.Context, userID int, req models.SetActiveAnnouncementRequest) error
	HideAnnouncement(ctx context.Context, id int) error
	UpdateAnnouncement(ctx context.Context, userID int, id int, req models.UpdateAnnouncementRequest) (*models.AnnouncementWithUser, error)
	DeleteAnnouncement(ctx context.Context, id int) error
}

type newsService struct {
	db                  *gorm.DB
	realtimePublisher   realtime.Publisher
	notificationService NotificationService
}

// NewsServiceOption конфигурирует newsService при создании.
type NewsServiceOption func(*newsService)

// WithNewsRealtimePublisher включает публикацию real-time сигнала
// "news.refresh" (#840) всем активным юзерам после мутации новости/объявления,
// чтобы лента (NewsAndReview) обновилась без F5. Опционально: без неё сигналы
// не шлются (тесты, offline).
func WithNewsRealtimePublisher(p realtime.Publisher) NewsServiceOption {
	return func(s *newsService) { s.realtimePublisher = p }
}

// WithNewsNotifications включает персональные уведомления news_published (#1748)
// при публикации новости. Опционально: без неё уведомления не шлются (тесты, offline).
func WithNewsNotifications(ns NotificationService) NewsServiceOption {
	return func(s *newsService) { s.notificationService = ns }
}

// NewNewsService создаёт реализацию NewsService.
func NewNewsService(db *gorm.DB, opts ...NewsServiceOption) NewsService {
	s := &newsService{db: db}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// notifyNewsChanged шлёт best-effort сигнал "news.refresh" всем активным
// юзерам после успешной мутации новости/объявления. Новости/объявления видны
// всем авторизованным (GetActiveNews/GetActiveAnnouncement без гейта прав),
// поэтому аудитория сигнала = все активные аккаунты (activeUserIDs), в отличие
// от table.<name>.view-гейтированных таблиц проходной.
func (s *newsService) notifyNewsChanged(ctx context.Context) {
	if s.realtimePublisher == nil {
		return
	}
	ids, err := activeUserIDs(ctx, s.db)
	if err != nil {
		slog.Warn("news realtime: failed to resolve audience", "err", err)
		return
	}
	s.realtimePublisher.PublishMany(ids, realtime.Event{Type: "news.refresh", Scope: "news"})
}

// notifyNewsPublished шлёт NotificationTypeNewsPublished активным пользователям
// при публикации новости (#1748). Активная новость (GetActiveNews) видна любому
// авторизованному без гейта прав - разделу «Обзор и новости» не соответствует
// отдельное view-право, поэтому аудитория, как и у notifyNewsChanged, = все
// активные аккаунты. Автору новости не шлём - он и так знает, что опубликовал.
func (s *newsService) notifyNewsPublished(ctx context.Context, newsID int, authorID int, title string) {
	if s.notificationService == nil {
		return
	}
	ids, err := activeUserIDs(ctx, s.db)
	if err != nil {
		slog.Warn("новость опубликована: не удалось собрать аудиторию уведомления", "news_id", newsID, "err", err)
		return
	}
	payload, _ := json.Marshal(map[string]any{"news_id": newsID, "title": title})
	payloadStr := string(payload)
	notifTitle := "Опубликована новость"
	for _, uid := range ids {
		if uid == authorID {
			continue
		}
		if err := s.notificationService.CreateForUser(ctx, uid, NotificationTypeNewsPublished, notifTitle, title, &payloadStr); err != nil {
			slog.Warn("не удалось уведомить о публикации новости", "news_id", newsID, "user_id", uid, "error", err)
		}
	}
}

// --- News ---

func (s *newsService) newsSelectQuery(db *gorm.DB) *gorm.DB {
	return db.
		Table("news n").
		Select(`n.id, n.title, n.description, n.full_text, n.is_active,
			n.created_by, CONCAT(uc.last_name, ' ', uc.first_name) AS created_by_name,
			n.updated_by, CONCAT(uu.last_name, ' ', uu.first_name) AS updated_by_name,
			n.created_at, n.updated_at`).
		Joins("LEFT JOIN users uc ON n.created_by = uc.id").
		Joins("LEFT JOIN users uu ON n.updated_by = uu.id")
}

// maskNewsAuthors подменяет ФИО авторов новостей логином, если они не давали
// согласия на обработку персональных данных.
func (s *newsService) maskNewsAuthors(ctx context.Context, items []models.NewsWithUser) {
	masks := loadConsentMasks(ctx, s.db)
	if len(masks) == 0 {
		return
	}
	for i := range items {
		items[i].CreatedByName = maskNamePtr(masks, items[i].CreatedBy, items[i].CreatedByName)
		items[i].UpdatedByName = maskNamePtr(masks, items[i].UpdatedBy, items[i].UpdatedByName)
	}
}

// maskAnnouncementAuthors - то же для объявлений: их видит любой авторизованный,
// а авторов там трое (создал, изменил, включил).
func (s *newsService) maskAnnouncementAuthors(ctx context.Context, items []models.AnnouncementWithUser) {
	masks := loadConsentMasks(ctx, s.db)
	if len(masks) == 0 {
		return
	}
	for i := range items {
		items[i].CreatedByName = maskNamePtr(masks, items[i].CreatedBy, items[i].CreatedByName)
		items[i].UpdatedByName = maskNamePtr(masks, items[i].UpdatedBy, items[i].UpdatedByName)
		items[i].ActivatedByName = maskNamePtr(masks, items[i].ActivatedBy, items[i].ActivatedByName)
	}
}

func (s *newsService) GetActiveNews(ctx context.Context) ([]models.NewsWithUser, error) {
	results := make([]models.NewsWithUser, 0)
	err := s.newsSelectQuery(s.db.WithContext(ctx)).
		Where("n.is_active = true").
		Order("n.created_at DESC").
		Scan(&results).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching news")
	}
	s.maskNewsAuthors(ctx, results)
	return results, nil
}

func (s *newsService) GetAllNews(ctx context.Context) ([]models.NewsWithUser, error) {
	results := make([]models.NewsWithUser, 0)
	err := s.newsSelectQuery(s.db.WithContext(ctx)).
		Order("n.created_at DESC").
		Scan(&results).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching news")
	}
	s.maskNewsAuthors(ctx, results)
	return results, nil
}

func (s *newsService) CreateNews(ctx context.Context, userID int, req models.CreateNewsRequest) (*models.NewsWithUser, error) {
	now := time.Now().UTC()
	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	news := models.News{
		Title:       req.Title,
		Description: req.Description,
		FullText:    req.FullText,
		CreatedBy:   &userID,
		CreatedAt:   now,
		UpdatedAt:   now,
		IsActive:    isActive,
	}

	if err := s.db.WithContext(ctx).Create(&news).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error creating news")
	}

	var result models.NewsWithUser
	err := s.newsSelectQuery(s.db.WithContext(ctx)).
		Where("n.id = ?", news.ID).
		Scan(&result).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching created news")
	}
	s.notifyNewsChanged(ctx)
	// Уведомление о публикации - только для реально видимой новости (создание
	// черновика, если фронт когда-нибудь его заведёт, тишины не нарушает).
	if isActive {
		s.notifyNewsPublished(ctx, news.ID, userID, news.Title)
	}
	return &result, nil
}

func (s *newsService) UpdateNews(ctx context.Context, userID int, id int, req models.UpdateNewsRequest) (*models.NewsWithUser, error) {
	var count int64
	if err := s.db.WithContext(ctx).Model(&models.News{}).Where("id = ?", id).Count(&count).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error checking news existence")
	}
	if count == 0 {
		return nil, echo.NewHTTPError(http.StatusNotFound, "News not found")
	}

	updates := map[string]interface{}{
		"updated_by": userID,
		"updated_at": time.Now().UTC(),
	}
	if req.Title != nil {
		updates["title"] = *req.Title
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.FullText != nil {
		updates["full_text"] = *req.FullText
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}

	if err := s.db.WithContext(ctx).Model(&models.News{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error updating news")
	}

	var result models.NewsWithUser
	err := s.newsSelectQuery(s.db.WithContext(ctx)).
		Where("n.id = ?", id).
		Scan(&result).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching updated news")
	}
	s.notifyNewsChanged(ctx)
	return &result, nil
}

func (s *newsService) DeleteNews(ctx context.Context, id int) error {
	result := s.db.WithContext(ctx).Delete(&models.News{}, id)
	if result.Error != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error deleting news")
	}
	if result.RowsAffected == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "News not found")
	}
	s.notifyNewsChanged(ctx)
	return nil
}

// --- Announcements ---

func (s *newsService) announcementSelectQuery(db *gorm.DB) *gorm.DB {
	return db.
		Table("announcements a").
		Select(`a.id, a.title, a.description, a.full_text, a.is_important, a.is_active,
			a.created_by, CONCAT(uc.last_name, ' ', uc.first_name) AS created_by_name,
			a.updated_by, CONCAT(uu.last_name, ' ', uu.first_name) AS updated_by_name,
			a.activated_at, a.activated_by, CONCAT(ua.last_name, ' ', ua.first_name) AS activated_by_name,
			a.created_at, a.updated_at`).
		Joins("LEFT JOIN users uc ON a.created_by = uc.id").
		Joins("LEFT JOIN users uu ON a.updated_by = uu.id").
		Joins("LEFT JOIN users ua ON a.activated_by = ua.id")
}

func (s *newsService) GetActiveAnnouncement(ctx context.Context) (*models.AnnouncementWithUser, error) {
	var result models.AnnouncementWithUser
	err := s.announcementSelectQuery(s.db.WithContext(ctx)).
		Where("a.is_active = true").
		Scan(&result).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching active announcement")
	}
	if result.ID == 0 {
		return nil, nil
	}
	// Через срез, а не по значению: маска правит элементы среза, копия осталась бы прежней.
	one := []models.AnnouncementWithUser{result}
	s.maskAnnouncementAuthors(ctx, one)
	return &one[0], nil
}

func (s *newsService) GetAllAnnouncements(ctx context.Context) ([]models.AnnouncementWithUser, error) {
	results := make([]models.AnnouncementWithUser, 0)
	err := s.announcementSelectQuery(s.db.WithContext(ctx)).
		Order("a.created_at DESC").
		Scan(&results).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching announcements")
	}
	s.maskAnnouncementAuthors(ctx, results)
	return results, nil
}

func (s *newsService) CreateAnnouncement(ctx context.Context, userID int, req models.CreateAnnouncementRequest) (*models.AnnouncementWithUser, error) {
	now := time.Now().UTC()
	isImportant := false
	if req.IsImportant != nil {
		isImportant = *req.IsImportant
	}

	announcement := models.Announcement{
		Title:       req.Title,
		Description: req.Description,
		FullText:    req.FullText,
		IsImportant: isImportant,
		IsActive:    false,
		CreatedBy:   &userID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// Если нет активного объявления — новое становится активным автоматически (в транзакции)
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var activeCount int64
		if err := tx.Model(&models.Announcement{}).Where("is_active = true").Count(&activeCount).Error; err != nil {
			return err
		}
		if activeCount == 0 {
			announcement.IsActive = true
			announcement.ActivatedAt = &now
			announcement.ActivatedBy = &userID
		}
		return tx.Create(&announcement).Error
	})
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error creating announcement")
	}

	var result models.AnnouncementWithUser
	if err := s.announcementSelectQuery(s.db.WithContext(ctx)).
		Where("a.id = ?", announcement.ID).
		Scan(&result).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching created announcement")
	}
	s.notifyNewsChanged(ctx)
	return &result, nil
}

func (s *newsService) SetActiveAnnouncement(ctx context.Context, userID int, req models.SetActiveAnnouncementRequest) error {
	var count int64
	if err := s.db.WithContext(ctx).Model(&models.Announcement{}).Where("id = ?", req.AnnouncementID).Count(&count).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error checking announcement existence")
	}
	if count == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "Announcement not found")
	}

	now := time.Now().UTC()

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Деактивируем все объявления
		if err := tx.Model(&models.Announcement{}).
			Where("is_active = true").
			Updates(map[string]interface{}{
				"is_active": false,
			}).Error; err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Error deactivating announcements")
		}

		// Активируем выбранное
		if err := tx.Model(&models.Announcement{}).
			Where("id = ?", req.AnnouncementID).
			Updates(map[string]interface{}{
				"is_active":    true,
				"activated_at": now,
				"activated_by": userID,
			}).Error; err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Error activating announcement")
		}

		return nil
	})
	if err != nil {
		return err
	}
	s.notifyNewsChanged(ctx)
	return nil
}

// HideAnnouncement снимает is_active с конкретного объявления (admin only).
// Не активирует другое - текущим активным остаётся то, что было до, либо
// никакого (если скрываемое было активным).
func (s *newsService) HideAnnouncement(ctx context.Context, id int) error {
	res := s.db.WithContext(ctx).Model(&models.Announcement{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"is_active":    false,
			"activated_at": nil,
			"activated_by": nil,
		})
	if res.Error != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error hiding announcement")
	}
	if res.RowsAffected == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "Announcement not found")
	}
	s.notifyNewsChanged(ctx)
	return nil
}

func (s *newsService) UpdateAnnouncement(ctx context.Context, userID int, id int, req models.UpdateAnnouncementRequest) (*models.AnnouncementWithUser, error) {
	var count int64
	if err := s.db.WithContext(ctx).Model(&models.Announcement{}).Where("id = ?", id).Count(&count).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error checking announcement existence")
	}
	if count == 0 {
		return nil, echo.NewHTTPError(http.StatusNotFound, "Announcement not found")
	}

	updates := map[string]interface{}{
		"updated_by": userID,
		"updated_at": time.Now().UTC(),
	}
	if req.Title != nil {
		updates["title"] = *req.Title
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.FullText != nil {
		updates["full_text"] = *req.FullText
	}
	if req.IsImportant != nil {
		updates["is_important"] = *req.IsImportant
	}

	if err := s.db.WithContext(ctx).Model(&models.Announcement{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error updating announcement")
	}

	var result models.AnnouncementWithUser
	err := s.announcementSelectQuery(s.db.WithContext(ctx)).
		Where("a.id = ?", id).
		Scan(&result).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching updated announcement")
	}
	s.notifyNewsChanged(ctx)
	return &result, nil
}

func (s *newsService) DeleteAnnouncement(ctx context.Context, id int) error {
	result := s.db.WithContext(ctx).Delete(&models.Announcement{}, id)
	if result.Error != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error deleting announcement")
	}
	if result.RowsAffected == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "Announcement not found")
	}
	s.notifyNewsChanged(ctx)
	return nil
}
