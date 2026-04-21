package services

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

func (s *applicationService) getUserByUsername(ctx context.Context, username string) (*models.User, error) {
	var user models.User
	if err := s.db.WithContext(ctx).Where("username = ?", username).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, echo.NewHTTPError(http.StatusUnauthorized, "User not found")
		}
		slog.Error("Ошибка получения пользователя", "username", username, "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Database error")
	}
	return &user, nil
}

// isApprover проверяет, является ли пользовател�� принимающим.
func (s *applicationService) isApprover(ctx context.Context, userID int) (bool, error) {
	var count int64
	if err := s.db.WithContext(ctx).Model(&models.ApplicationApprover{}).Where("user_id = ?", userID).Count(&count).Error; err != nil {
		slog.Error("Ошибка проверки approver", "user_id", userID, "error", err)
		return false, echo.NewHTTPError(http.StatusInternalServerError, "Database error")
	}
	return count > 0, nil
}

// formatFullName формирует полное ФИО.
func formatFullName(lastName, firstName, middleName *string) string {
	parts := []string{}
	if lastName != nil && *lastName != "" {
		parts = append(parts, *lastName)
	}
	if firstName != nil && *firstName != "" {
		parts = append(parts, *firstName)
	}
	if middleName != nil && *middleName != "" {
		parts = append(parts, *middleName)
	}
	return strings.Join(parts, " ")
}

// formatShortName формирует сокращённое ФИО (Фамилия И. О.).
func formatShortName(lastName, firstName, middleName *string) string {
	result := ""
	if lastName != nil && *lastName != "" {
		result = *lastName
	}
	if firstName != nil && *firstName != "" {
		result += " " + string([]rune(*firstName)[:1]) + "."
	}
	if middleName != nil && *middleName != "" {
		result += " " + string([]rune(*middleName)[:1]) + "."
	}
	return strings.TrimSpace(result)
}

// updateConfirmationBasedOnApprovals пересчитывает confirmation заявки по голосам ответственных.
func (s *applicationService) updateConfirmationBasedOnApprovals(tx *gorm.DB, applicationID int) error {
	var responsibles []models.ApplicationResponsibleUser
	if err := tx.Where("application_id = ?", applicationID).Find(&responsibles).Error; err != nil {
		slog.Error("Ошибка получения ответственных", "application_id", applicationID, "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Error fetching responsible users")
	}

	if len(responsibles) == 0 {
		return nil
	}

	var required, nonRequired []models.ApplicationResponsibleUser
	for _, r := range responsibles {
		if r.RequiredApproval {
			required = append(required, r)
		} else {
			nonRequired = append(nonRequired, r)
		}
	}

	newConfirmation := models.ConfirmationPending

	hasRequiredRejected := false
	for _, r := range required {
		if r.ApprovalStatus != nil && *r.ApprovalStatus == "rejected" {
			hasRequiredRejected = true
			break
		}
	}

	if hasRequiredRejected {
		newConfirmation = models.ConfirmationRejected
	} else if len(required) > 0 {
		allApproved := true
		for _, r := range required {
			if r.ApprovalStatus == nil || *r.ApprovalStatus != "approved" {
				allApproved = false
				break
			}
		}
		if allApproved {
			newConfirmation = models.ConfirmationApproved
		}
	} else if len(nonRequired) > 0 {
		hasAnyApproved := false
		hasAnyRejected := false
		for _, r := range nonRequired {
			if r.ApprovalStatus != nil && *r.ApprovalStatus == "approved" {
				hasAnyApproved = true
			}
			if r.ApprovalStatus != nil && *r.ApprovalStatus == "rejected" {
				hasAnyRejected = true
			}
		}
		if hasAnyApproved && !hasAnyRejected {
			newConfirmation = models.ConfirmationApproved
		} else if hasAnyRejected {
			newConfirmation = models.ConfirmationRejected
		}
	}

	result := tx.Exec(`
		UPDATE applications
		SET confirmation = ?,
		    confirmation_datetime = CASE
		        WHEN ? != ? AND confirmation_datetime IS NULL THEN NOW()
		        ELSE confirmation_datetime
		    END
		WHERE id = ?
	`, newConfirmation, newConfirmation, models.ConfirmationPending, applicationID)

	if result.Error != nil {
		slog.Error("Ошибка обновления confirmation", "application_id", applicationID, "error", result.Error)
		return echo.NewHTTPError(http.StatusInternalServerError, "Error updating application confirmation")
	}

	return nil
}

// activateApplicationItems активирует/деактивирует машины и сотрудников заявки.
func (s *applicationService) activateApplicationItems(tx *gorm.DB, applicationID int, activate bool) error {
	newStatus := 0
	if activate {
		newStatus = 1
	}

	type attachmentRow struct {
		ID             int
		AttachmentType string
	}
	var attachments []attachmentRow
	if err := tx.Raw("SELECT id, attachment_type FROM attachments WHERE application_id = ?", applicationID).Scan(&attachments).Error; err != nil {
		slog.Error("Ошибка получения вложений", "application_id", applicationID, "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Error fetching attachments")
	}

	for _, att := range attachments {
		switch att.AttachmentType {
		case "cars":
			if err := tx.Exec("UPDATE cars SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE attachment_id = ?", newStatus, att.ID).Error; err != nil {
				slog.Error("Ошибка обновления статуса машин", "attachment_id", att.ID, "error", err)
				return echo.NewHTTPError(http.StatusInternalServerError, "Error updating cars status")
			}
		case "people":
			if err := tx.Exec("UPDATE employees SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE attachment_id = ?", newStatus, att.ID).Error; err != nil {
				slog.Error("Ошибка обновления статуса сотрудников", "attachment_id", att.ID, "error", err)
				return echo.NewHTTPError(http.StatusInternalServerError, "Error updating employees status")
			}
		}
	}

	return nil
}

// --- Основные методы ---

func applyApplicationFilters(query *gorm.DB, filter ApplicationFilter, includeUserSearch bool) *gorm.DB {
	if filter.SearchQuery != nil && *filter.SearchQuery != "" {
		pattern := "%" + *filter.SearchQuery + "%"
		if includeUserSearch {
			query = query.Where(`
				a.application_number ILIKE ? OR
				COALESCE(o.name, c.name, '') ILIKE ? OR
				c.name ILIKE ? OR
				a.message ILIKE ? OR
				a.status ILIKE ? OR
				a.confirmation ILIKE ? OR
				u.last_name ILIKE ? OR u.first_name ILIKE ? OR u.middle_name ILIKE ? OR
				ru.last_name ILIKE ? OR ru.first_name ILIKE ? OR ru.middle_name ILIKE ?
			`, pattern, pattern, pattern, pattern, pattern, pattern,
				pattern, pattern, pattern, pattern, pattern, pattern)
		} else {
			query = query.Where(`
				a.application_number ILIKE ? OR
				COALESCE(o.name, c.name, '') ILIKE ? OR
				c.name ILIKE ? OR
				a.message ILIKE ? OR
				a.status ILIKE ? OR
				a.confirmation ILIKE ?
			`, pattern, pattern, pattern, pattern, pattern, pattern)
		}
	}

	if filter.OrganizationID != nil {
		query = query.Where("a.organization_id = ?", *filter.OrganizationID)
	}
	if filter.CompanyID != nil {
		query = query.Where("a.company_id = ?", *filter.CompanyID)
	}
	if filter.Confirmation != nil {
		query = query.Where("a.confirmation = ?", *filter.Confirmation)
	}
	if filter.Status != nil {
		query = query.Where("a.status = ?", *filter.Status)
	}
	if filter.DateFrom != nil && *filter.DateFrom != "" {
		query = query.Where("a.sending_datetime >= ?", *filter.DateFrom+" 00:00:00")
	}
	if filter.DateTo != nil && *filter.DateTo != "" {
		query = query.Where("a.sending_datetime <= ?", *filter.DateTo+" 23:59:59")
	}

	// Archive filter: by default exclude archived, archive=true shows only archived
	archiveCondition := `
		(a.status IN (?, ?) AND EXISTS(
			SELECT 1 FROM attachments att WHERE att.application_id = a.id
			AND att.entry_date_to IS NOT NULL
			AND CAST(att.entry_date_to AS DATE) + INTERVAL '1 month' < NOW()
		))
	`
	if filter.Archive != nil && *filter.Archive {
		query = query.Where(archiveCondition, models.StatusCompleted, models.StatusRejected)
	} else {
		query = query.Where("NOT "+archiveCondition, models.StatusCompleted, models.StatusRejected)
	}

	// Active today: заявка активна сегодня, если период действия хотя бы одного
	// вложения (entry_date_from..entry_date_to) включает текущую дату.
	if filter.ActiveToday != nil && *filter.ActiveToday {
		query = query.Where(`
			EXISTS(
				SELECT 1 FROM attachments att
				WHERE att.application_id = a.id
				AND att.entry_date_from IS NOT NULL
				AND att.entry_date_to IS NOT NULL
				AND CURRENT_DATE BETWEEN CAST(att.entry_date_from AS DATE) AND CAST(att.entry_date_to AS DATE)
			)
		`)
	}

	return query
}

func (s *applicationService) fetchResponsibleUsers(db *gorm.DB, applicationID int) ([]ResponsibleUserInfo, error) {
	responsibles := make([]ResponsibleUserInfo, 0)
	err := db.Raw(`
		SELECT
			u.id,
			u.username,
			u.last_name,
			u.first_name,
			u.middle_name,
			u.position,
			COALESCE(aru.is_primary, false) as is_primary,
			COALESCE(aru.required_approval, false) as required_approval,
			aru.approval_status,
			aru.approval_comment,
			aru.approval_datetime
		FROM application_responsible_users aru
		JOIN users u ON aru.user_id = u.id
		WHERE aru.application_id = ?
		ORDER BY aru.is_primary DESC, u.last_name, u.first_name
	`, applicationID).Scan(&responsibles).Error

	if err != nil {
		slog.Error("Ошибка получения ответственных пользователей", "application_id", applicationID, "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching responsible users")
	}

	if responsibles == nil {
		responsibles = []ResponsibleUserInfo{}
	}
	return responsibles, nil
}

// CanAccessApplication проверяет, имеет ли пользователь доступ к заявке.
// Доступ имеют: администраторы (type_id=6), отправитель, ответственные и просматривающие.
func (s *applicationService) CanAccessApplication(ctx context.Context, applicationID int, username string, typeID int) bool {
	if typeID == 6 {
		return true
	}

	user, err := s.getUserByUsername(ctx, username)
	if err != nil {
		return false
	}

	var app models.Application
	if err := s.db.WithContext(ctx).Select("id, sender_user_id").Where("id = ?", applicationID).First(&app).Error; err != nil {
		return false
	}

	if app.SenderUserID == user.ID {
		return true
	}

	var count int64
	s.db.WithContext(ctx).Model(&models.ApplicationResponsibleUser{}).
		Where("application_id = ? AND user_id = ?", applicationID, user.ID).
		Count(&count)
	if count > 0 {
		return true
	}

	s.db.WithContext(ctx).Model(&models.ApplicationViewer{}).
		Where("application_id = ? AND user_id = ?", applicationID, user.ID).
		Count(&count)
	return count > 0
}

// GetApplicationIDByAttachment возвращает ID заявки по ID вложения.
func (s *applicationService) GetApplicationIDByAttachment(ctx context.Context, attachmentID int) (int, error) {
	var attachment models.Attachment
	if err := s.db.WithContext(ctx).Select("id, application_id").Where("id = ?", attachmentID).First(&attachment).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return 0, echo.NewHTTPError(http.StatusNotFound, "Attachment not found")
		}
		return 0, echo.NewHTTPError(http.StatusInternalServerError, "Database error")
	}
	return attachment.ApplicationID, nil
}

func ptrString(s string) *string { return &s }

func safeDerefInt(p *int) int {
	if p != nil {
		return *p
	}
	return 0
}
