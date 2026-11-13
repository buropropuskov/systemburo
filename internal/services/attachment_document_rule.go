package services

import (
	"context"
	"net/http"
	"strings"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func documentAtPostAllowed(db *gorm.DB, templateID int, organizationID, companyID *int) (bool, error) {
	if organizationID == nil && companyID == nil {
		return false, nil
	}
	var allowed bool
	err := db.Raw(`SELECT EXISTS (
		SELECT 1 FROM attachment_document_rules r
		JOIN unique_attachments ua ON ua.id = r.unique_attachment_id
		WHERE r.unique_attachment_id = ? AND ua.is_active AND ua.attachment_type = 'people' AND (
			(directory = 'organization' AND directory_id = ?) OR
			(directory = 'company' AND directory_id = ?)
		)
	)`, templateID, organizationID, companyID).Scan(&allowed).Error
	return allowed, err
}

func (s *attachmentFieldConfigService) DocumentAtPostAllowed(ctx context.Context, templateID int, organizationID, companyID *int, userID int, override bool) (bool, error) {
	db := s.db.WithContext(ctx)
	if !override {
		var user models.User
		if err := db.Select("organization_id", "company_id").First(&user, userID).Error; err != nil {
			return false, err
		}
		scope := applicantScope{organizationID: user.OrganizationID, companyID: user.CompanyID}
		if organizationID != nil {
			if err := ensureDirectoryAllowed(organizationRef, scope, *organizationID); err != nil {
				return false, err
			}
		}
		if companyID != nil {
			if err := ensureDirectoryAllowed(companyRef, scope, *companyID); err != nil {
				return false, err
			}
		}
	}
	return documentAtPostAllowed(db, templateID, organizationID, companyID)
}

func (s *attachmentFieldConfigService) DocumentRule(ctx context.Context, templateID int, directory string, id int, enabled *bool) (bool, error) {
	var table string
	switch directory {
	case "organization":
		table = "organizations"
	case "company":
		table = "companies"
	default:
		return false, echo.NewHTTPError(http.StatusBadRequest, "Выберите справочник")
	}
	if id <= 0 || templateID <= 0 {
		return false, echo.NewHTTPError(http.StatusBadRequest, "Выберите запись и вид заявки")
	}
	db := s.db.WithContext(ctx)
	rule := models.AttachmentDocumentRule{UniqueAttachmentID: templateID, Directory: directory, DirectoryID: id}
	key := db.Where("unique_attachment_id = ? AND directory = ? AND directory_id = ?", templateID, directory, id)
	if enabled == nil {
		var count int64
		err := key.Model(&models.AttachmentDocumentRule{}).Count(&count).Error
		return count > 0, err
	}
	if !*enabled {
		return false, key.Delete(&models.AttachmentDocumentRule{}).Error
	}
	var valid bool
	if err := db.Raw("SELECT EXISTS (SELECT 1 FROM "+table+" WHERE id = ? AND is_active) AND EXISTS (SELECT 1 FROM unique_attachments WHERE id = ? AND is_active AND attachment_type = 'people')", id, templateID).Scan(&valid).Error; err != nil {
		return false, err
	}
	if !valid {
		return false, echo.NewHTTPError(http.StatusBadRequest, "Выберите активную запись и бланк для прохода людей")
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
