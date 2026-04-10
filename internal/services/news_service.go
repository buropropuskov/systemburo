package services

import (
	"context"
	"net/http"
	"time"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// NewsService -- интерфейс бизнес-логики новостей и объявлений.
type NewsService interface {
	// News
	GetActiveNews(ctx context.Context) ([]models.NewsWithUser, error)
	GetAllNews(ctx context.Context, typeID int) ([]models.NewsWithUser, error)
	CreateNews(ctx context.Context, typeID int, userID int, req models.CreateNewsRequest) (*models.NewsWithUser, error)
	UpdateNews(ctx context.Context, typeID int, userID int, id int, req models.UpdateNewsRequest) (*models.NewsWithUser, error)
	DeleteNews(ctx context.Context, typeID int, id int) error

	// Announcements
	GetActiveAnnouncement(ctx context.Context) (*models.AnnouncementWithUser, error)
	GetAllAnnouncements(ctx context.Context, typeID int) ([]models.AnnouncementWithUser, error)
	CreateAnnouncement(ctx context.Context, typeID int, userID int, req models.CreateAnnouncementRequest) (*models.AnnouncementWithUser, error)
	SetActiveAnnouncement(ctx context.Context, typeID int, userID int, req models.SetActiveAnnouncementRequest) error
	UpdateAnnouncement(ctx context.Context, typeID int, userID int, id int, req models.UpdateAnnouncementRequest) (*models.AnnouncementWithUser, error)
	DeleteAnnouncement(ctx context.Context, typeID int, id int) error
}

type newsService struct {
	db *gorm.DB
}

// NewNewsService создаёт реализацию NewsService.
func NewNewsService(db *gorm.DB) NewsService {
	return &newsService{db: db}
}

func (s *newsService) checkAdmin(ctx context.Context, typeID int) error {
	var code string
	err := s.db.WithContext(ctx).
		Table("user_types").
		Select("code").
		Where("id = ?", typeID).
		Row().
		Scan(&code)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "User not found")
	}
	if code != "manager" && code != "buropropuskov" {
		return echo.NewHTTPError(http.StatusForbidden, "Insufficient permissions")
	}
	return nil
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

func (s *newsService) GetActiveNews(ctx context.Context) ([]models.NewsWithUser, error) {
	results := make([]models.NewsWithUser, 0)
	err := s.newsSelectQuery(s.db.WithContext(ctx)).
		Where("n.is_active = true").
		Order("n.created_at DESC").
		Scan(&results).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching news")
	}
	return results, nil
}

func (s *newsService) GetAllNews(ctx context.Context, typeID int) ([]models.NewsWithUser, error) {
	if err := s.checkAdmin(ctx, typeID); err != nil {
		return nil, err
	}

	results := make([]models.NewsWithUser, 0)
	err := s.newsSelectQuery(s.db.WithContext(ctx)).
		Order("n.created_at DESC").
		Scan(&results).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching news")
	}
	return results, nil
}

func (s *newsService) CreateNews(ctx context.Context, typeID int, userID int, req models.CreateNewsRequest) (*models.NewsWithUser, error) {
	if err := s.checkAdmin(ctx, typeID); err != nil {
		return nil, err
	}

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
	return &result, nil
}

func (s *newsService) UpdateNews(ctx context.Context, typeID int, userID int, id int, req models.UpdateNewsRequest) (*models.NewsWithUser, error) {
	if err := s.checkAdmin(ctx, typeID); err != nil {
		return nil, err
	}

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
	return &result, nil
}

func (s *newsService) DeleteNews(ctx context.Context, typeID int, id int) error {
	if err := s.checkAdmin(ctx, typeID); err != nil {
		return err
	}

	result := s.db.WithContext(ctx).Delete(&models.News{}, id)
	if result.Error != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error deleting news")
	}
	if result.RowsAffected == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "News not found")
	}
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
	return &result, nil
}

func (s *newsService) GetAllAnnouncements(ctx context.Context, typeID int) ([]models.AnnouncementWithUser, error) {
	if err := s.checkAdmin(ctx, typeID); err != nil {
		return nil, err
	}

	results := make([]models.AnnouncementWithUser, 0)
	err := s.announcementSelectQuery(s.db.WithContext(ctx)).
		Order("a.created_at DESC").
		Scan(&results).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching announcements")
	}
	return results, nil
}

func (s *newsService) CreateAnnouncement(ctx context.Context, typeID int, userID int, req models.CreateAnnouncementRequest) (*models.AnnouncementWithUser, error) {
	if err := s.checkAdmin(ctx, typeID); err != nil {
		return nil, err
	}

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
	return &result, nil
}

func (s *newsService) SetActiveAnnouncement(ctx context.Context, typeID int, userID int, req models.SetActiveAnnouncementRequest) error {
	if err := s.checkAdmin(ctx, typeID); err != nil {
		return err
	}

	var count int64
	if err := s.db.WithContext(ctx).Model(&models.Announcement{}).Where("id = ?", req.AnnouncementID).Count(&count).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error checking announcement existence")
	}
	if count == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "Announcement not found")
	}

	now := time.Now().UTC()

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
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
}

func (s *newsService) UpdateAnnouncement(ctx context.Context, typeID int, userID int, id int, req models.UpdateAnnouncementRequest) (*models.AnnouncementWithUser, error) {
	if err := s.checkAdmin(ctx, typeID); err != nil {
		return nil, err
	}

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
	return &result, nil
}

func (s *newsService) DeleteAnnouncement(ctx context.Context, typeID int, id int) error {
	if err := s.checkAdmin(ctx, typeID); err != nil {
		return err
	}

	result := s.db.WithContext(ctx).Delete(&models.Announcement{}, id)
	if result.Error != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error deleting announcement")
	}
	if result.RowsAffected == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "Announcement not found")
	}
	return nil
}
