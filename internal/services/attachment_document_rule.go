package services

import (
	"context"
	"net/http"
	"strings"

	"systemburo/internal/models"

	"github.com/labstack/echo/v5"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func documentAtPostAllowed(db *gorm.DB, templateID, userID int) (bool, error) {
	var allowed bool
	err := db.Raw(`SELECT EXISTS (
		SELECT 1 FROM attachment_document_rules r
		JOIN unique_attachments ua ON ua.id = r.unique_attachment_id
		WHERE r.unique_attachment_id = ? AND ua.is_active AND ua.attachment_type = 'people'
			AND r.directory = 'account' AND r.directory_id = ?
	)`, templateID, userID).Scan(&allowed).Error
	return allowed, err
}

func (s *attachmentFieldConfigService) DocumentAtPostAllowed(ctx context.Context, templateID, userID int) (bool, error) {
	return documentAtPostAllowed(s.db.WithContext(ctx), templateID, userID)
}

func (s *attachmentFieldConfigService) DocumentRule(ctx context.Context, templateID, userID int, enabled *bool) (bool, error) {
	db := s.db.WithContext(ctx)
	if enabled == nil {
		return documentAtPostAllowed(db, templateID, userID)
	}
	rule := models.AttachmentDocumentRule{UniqueAttachmentID: templateID, Directory: "account", DirectoryID: userID}
	key := db.Where("unique_attachment_id = ? AND directory = ? AND directory_id = ?", templateID, rule.Directory, userID)
	if !*enabled {
		return false, key.Delete(&models.AttachmentDocumentRule{}).Error
	}
	var valid bool
	if err := db.Raw("SELECT EXISTS (SELECT 1 FROM users WHERE id = ? AND is_active AND NOT is_banned) AND EXISTS (SELECT 1 FROM unique_attachments WHERE id = ? AND is_active AND attachment_type = 'people')", userID, templateID).Scan(&valid).Error; err != nil {
		return false, err
	}
	if !valid {
		return false, echo.NewHTTPError(http.StatusBadRequest, "Выберите активный аккаунт и бланк для прохода людей")
	}
	return true, db.Clauses(clause.OnConflict{DoNothing: true}).Create(&rule).Error
}

func validateDocumentAtPost(data AttachmentContentData, allowed bool) error {
	if data.Employees == nil {
		return nil
	}
	for _, employee := range *data.Employees {
		if !employee.DocumentAtPost {
			continue
		}
		if !allowed {
			return echo.NewHTTPError(http.StatusBadRequest, "Проверка документа на посту недоступна для этой заявки")
		}
		if strings.TrimSpace(employee.PassportSeriesNumber) != "" {
			return echo.NewHTTPError(http.StatusBadRequest, "При проверке документа на посту паспортное поле должно быть пустым")
		}
	}
	return nil
}
