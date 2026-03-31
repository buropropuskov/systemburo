package services

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

func (s *applicationService) ForwardApplication(ctx context.Context, username string, applicationID int, req ForwardApplicationRequest) error {
	if err := s.checkNotArchived(ctx, applicationID); err != nil {
		return err
	}

	var user struct {
		ID         int
		LastName   *string
		FirstName  *string
		MiddleName *string
	}
	if err := s.db.WithContext(ctx).Raw("SELECT id, last_name, first_name, middle_name FROM users WHERE username = ?", username).Scan(&user).Error; err != nil || user.ID == 0 {
		return echo.NewHTTPError(http.StatusUnauthorized, "User not found")
	}
	currentUserName := formatFullName(user.LastName, user.FirstName, user.MiddleName)

	tx := s.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to start transaction")
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Проверяем существование заявки
	var exists bool
	tx.Raw("SELECT EXISTS(SELECT 1 FROM applications WHERE id = ?)", applicationID).Scan(&exists)
	if !exists {
		tx.Rollback()
		return echo.NewHTTPError(http.StatusNotFound, "Application not found")
	}

	// Проверяем права на пересылку
	var canForward bool
	tx.Raw(`
		SELECT EXISTS(
			SELECT 1 FROM applications a WHERE a.id = ? AND (
				a.sender_user_id = ?
				OR EXISTS(SELECT 1 FROM application_responsible_users aru WHERE aru.application_id = a.id AND aru.user_id = ?)
			)
		)
	`, applicationID, user.ID, user.ID).Scan(&canForward)
	if !canForward {
		tx.Rollback()
		return echo.NewHTTPError(http.StatusForbidden, "You don't have permission to forward this application")
	}

	// Сохраняем старый confirmation
	var oldConfirmation *string
	tx.Raw("SELECT confirmation FROM applications WHERE id = ?", applicationID).Scan(&oldConfirmation)

	baseTime := time.Now().UTC()
	historyTime := baseTime

	type addedResp struct {
		UserID           int
		RequiredApproval bool
	}
	var addedResponsibleUsers []addedResp
	var addedViewers []int

	for _, fu := range req.Users {
		// Проверяем существование пользователя
		var userExists bool
		tx.Raw("SELECT EXISTS(SELECT 1 FROM users WHERE id = ?)", fu.UserID).Scan(&userExists)
		if !userExists {
			continue
		}

		if fu.RequiredApproval && fu.CanView {
			continue
		}

		if fu.RequiredApproval || !fu.CanView {
			// Ответственный пользователь
			var alreadyAdded bool
			tx.Raw("SELECT EXISTS(SELECT 1 FROM application_responsible_users WHERE application_id = ? AND user_id = ?)", applicationID, fu.UserID).Scan(&alreadyAdded)

			if alreadyAdded {
				tx.Exec("UPDATE application_responsible_users SET required_approval = ?, created_by = ? WHERE application_id = ? AND user_id = ?",
					fu.RequiredApproval, user.ID, applicationID, fu.UserID)
			} else {
				tx.Exec(`
					INSERT INTO application_responsible_users (application_id, user_id, required_approval, approval_status, created_at, created_by, is_primary)
					VALUES (?, ?, ?, 'pending', ?, ?, false)
				`, applicationID, fu.UserID, fu.RequiredApproval, baseTime, user.ID)
			}
			addedResponsibleUsers = append(addedResponsibleUsers, addedResp{fu.UserID, fu.RequiredApproval})
		} else {
			// Просматривающий
			var alreadyAdded bool
			tx.Raw("SELECT EXISTS(SELECT 1 FROM application_viewers WHERE application_id = ? AND user_id = ?)", applicationID, fu.UserID).Scan(&alreadyAdded)

			if !alreadyAdded {
				tx.Exec(`
					INSERT INTO application_viewers (application_id, user_id, created_at, created_by)
					VALUES (?, ?, ?, ?)
				`, applicationID, fu.UserID, baseTime, user.ID)
			}
			addedViewers = append(addedViewers, fu.UserID)
		}
	}

	// Записываем историю назначений ответственных
	for _, resp := range addedResponsibleUsers {
		historyTime = historyTime.Add(time.Millisecond)
		meta, _ := json.Marshal(map[string]interface{}{
			"required_approval": resp.RequiredApproval,
			"is_primary":        false,
			"forwarded_by":      currentUserName,
			"type":              "responsible",
		})
		tx.Exec(`
			INSERT INTO application_history (application_id, user_id, action_type, metadata, created_at)
			VALUES (?, ?, 'assigned_responsible', ?, ?)
		`, applicationID, resp.UserID, string(meta), historyTime)
	}

	// Записываем историю назначений просматривающих
	for _, viewerID := range addedViewers {
		historyTime = historyTime.Add(time.Millisecond)
		meta, _ := json.Marshal(map[string]interface{}{
			"forwarded_by": currentUserName,
			"type":         "viewer",
		})
		tx.Exec(`
			INSERT INTO application_history (application_id, user_id, action_type, metadata, created_at)
			VALUES (?, ?, 'assigned_viewer', ?, ?)
		`, applicationID, viewerID, string(meta), historyTime)
	}

	// Обновляем confirmation если были добавлены ответственные
	if len(addedResponsibleUsers) > 0 {
		if err := s.updateConfirmationBasedOnApprovals(tx, applicationID); err != nil {
			tx.Rollback()
			return err
		}
	}

	// Проверяем изменение confirmation
	var newConfirmation *string
	tx.Raw("SELECT confirmation FROM applications WHERE id = ?", applicationID).Scan(&newConfirmation)

	if (oldConfirmation == nil) != (newConfirmation == nil) || (oldConfirmation != nil && newConfirmation != nil && *oldConfirmation != *newConfirmation) {
		statusChangeTime := historyTime.Add(time.Millisecond)
		tx.Exec(`
			INSERT INTO application_history (application_id, user_id, action_type, old_value, new_value, created_at)
			VALUES (?, ?, 'confirmation_change', ?, ?, ?)
		`, applicationID, user.ID, oldConfirmation, newConfirmation, statusChangeTime)
	}

	if err := tx.Commit().Error; err != nil {
		slog.Error("не удалось зафиксировать транзакцию пересылки заявки", "application_id", applicationID, "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to commit transaction")
	}

	// Автоматически выдаём права tab.applications.view ответственным пользователям
	for _, resp := range addedResponsibleUsers {
		_ = s.permissionService.GrantPermission(ctx, resp.UserID, "tab.applications.view", "allow")
	}

	slog.Info("заявка переслана", "application_id", applicationID, "user_id", user.ID,
		"responsible_count", len(addedResponsibleUsers), "viewer_count", len(addedViewers))
	return nil
}

func (s *applicationService) ApproveApplicationByUser(ctx context.Context, username string, applicationID int, req UserApprovalRequest) error {
	if err := s.checkNotArchived(ctx, applicationID); err != nil {
		return err
	}

	user, err := s.getUserByUsername(ctx, username)
	if err != nil {
		return err
	}

	if req.UserID != user.ID {
		return echo.NewHTTPError(http.StatusForbidden, "You can only approve for yourself")
	}

	if req.Status != "approved" && req.Status != "rejected" {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid status. Must be 'approved' or 'rejected'")
	}

	tx := s.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to start transaction")
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Проверяем, что пользователь -- ответственный
	var responsible struct {
		ID               int
		ApprovalStatus   *string
		RequiredApproval bool
	}
	result := tx.Raw(`
		SELECT id, approval_status, required_approval
		FROM application_responsible_users
		WHERE application_id = ? AND user_id = ?
	`, applicationID, req.UserID).Scan(&responsible)
	if result.Error != nil || responsible.ID == 0 {
		tx.Rollback()
		return echo.NewHTTPError(http.StatusForbidden, "You are not responsible for this application")
	}

	if responsible.ApprovalStatus == nil || *responsible.ApprovalStatus != "pending" {
		tx.Rollback()
		return echo.NewHTTPError(http.StatusBadRequest, "You have already voted on this application")
	}

	// Сохраняем старый confirmation
	var oldConfirmation *string
	tx.Raw("SELECT confirmation FROM applications WHERE id = ?", applicationID).Scan(&oldConfirmation)

	nowUTC := time.Now().UTC()
	historyTime := nowUTC

	// Обновляем голос
	tx.Exec(`
		UPDATE application_responsible_users
		SET approval_status = ?, approval_comment = ?, approval_datetime = ?
		WHERE application_id = ? AND user_id = ?
	`, req.Status, req.Comment, nowUTC, applicationID, req.UserID)

	// Записываем действие в историю
	actionType := "approve"
	if req.Status == "rejected" {
		actionType = "reject"
	}
	meta, _ := json.Marshal(map[string]interface{}{"required_approval": responsible.RequiredApproval})
	tx.Exec(`
		INSERT INTO application_history (application_id, user_id, action_type, comment, metadata, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, applicationID, user.ID, actionType, req.Comment, string(meta), historyTime)

	// Пересчитываем confirmation
	if err := s.updateConfirmationBasedOnApprovals(tx, applicationID); err != nil {
		tx.Rollback()
		return err
	}

	// Проверяем изменение confirmation
	var newConfirmation *string
	tx.Raw("SELECT confirmation FROM applications WHERE id = ?", applicationID).Scan(&newConfirmation)

	if (oldConfirmation == nil) != (newConfirmation == nil) || (oldConfirmation != nil && newConfirmation != nil && *oldConfirmation != *newConfirmation) {
		statusChangeTime := historyTime.Add(time.Millisecond)
		tx.Exec(`
			INSERT INTO application_history (application_id, user_id, action_type, old_value, new_value, created_at)
			VALUES (?, ?, 'confirmation_change', ?, ?, ?)
		`, applicationID, user.ID, oldConfirmation, newConfirmation, statusChangeTime)
	}

	if err := tx.Commit().Error; err != nil {
		slog.Error("не удалось зафиксировать транзакцию одобрения заявки", "application_id", applicationID, "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to commit transaction")
	}

	slog.Info("заявка одобрена/отклонена", "application_id", applicationID, "user_id", user.ID, "status", req.Status)
	return nil
}

func (s *applicationService) CheckApprovalStatus(ctx context.Context, applicationID int) (*ApprovalStatusResponse, error) {
	var app struct {
		Confirmation *string
		Status       *string
	}
	result := s.db.WithContext(ctx).Raw("SELECT confirmation, status FROM applications WHERE id = ?", applicationID).Scan(&app)
	if result.Error != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Database error")
	}
	if result.RowsAffected == 0 {
		return nil, echo.NewHTTPError(http.StatusNotFound, "Application not found")
	}

	return &ApprovalStatusResponse{
		Confirmation: app.Confirmation,
		Status:       app.Status,
	}, nil
}

func (s *applicationService) RevokeApproval(ctx context.Context, username string, applicationID int, req RevokeApprovalRequest) (*RevokeApprovalResponse, error) {
	user, err := s.getUserByUsername(ctx, username)
	if err != nil {
		return nil, err
	}

	tx := s.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to start transaction")
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var responsible struct {
		ApprovalStatus   *string
		RequiredApproval bool
	}
	result := tx.Raw("SELECT approval_status, required_approval FROM application_responsible_users WHERE application_id = ? AND user_id = ?",
		applicationID, user.ID).Scan(&responsible)
	if result.Error != nil || result.RowsAffected == 0 {
		tx.Rollback()
		return nil, echo.NewHTTPError(http.StatusForbidden, "You are not responsible for this application")
	}

	if responsible.ApprovalStatus == nil || *responsible.ApprovalStatus == "pending" {
		tx.Rollback()
		return nil, echo.NewHTTPError(http.StatusBadRequest, "You haven't voted yet")
	}

	var oldConfirmation *string
	tx.Raw("SELECT confirmation FROM applications WHERE id = ?", applicationID).Scan(&oldConfirmation)

	nowUTC := time.Now().UTC()
	historyTime := nowUTC

	tx.Exec(`
		UPDATE application_responsible_users
		SET approval_status = 'pending', approval_comment = NULL, approval_datetime = NULL
		WHERE application_id = ? AND user_id = ?
	`, applicationID, user.ID)

	tx.Exec(`
		INSERT INTO application_history (application_id, user_id, action_type, comment, created_at)
		VALUES (?, ?, 'revoke_approval', ?, ?)
	`, applicationID, user.ID, req.Comment, historyTime)

	if err := s.updateConfirmationBasedOnApprovals(tx, applicationID); err != nil {
		tx.Rollback()
		return nil, err
	}

	var newConfirmation *string
	tx.Raw("SELECT confirmation FROM applications WHERE id = ?", applicationID).Scan(&newConfirmation)

	if (oldConfirmation == nil) != (newConfirmation == nil) || (oldConfirmation != nil && newConfirmation != nil && *oldConfirmation != *newConfirmation) {
		statusChangeTime := historyTime.Add(time.Millisecond)
		tx.Exec(`
			INSERT INTO application_history (application_id, user_id, action_type, old_value, new_value, created_at)
			VALUES (?, ?, 'confirmation_change', ?, ?, ?)
		`, applicationID, user.ID, oldConfirmation, newConfirmation, statusChangeTime)
	}

	if err := tx.Commit().Error; err != nil {
		slog.Error("не удалось зафиксировать транзакцию отзыва одобрения", "application_id", applicationID, "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to commit transaction")
	}

	slog.Info("одобрение отозвано", "application_id", applicationID, "user_id", user.ID)
	var updatedApp struct {
		Confirmation *string
		Status       *string
	}
	s.db.WithContext(ctx).Raw("SELECT confirmation, status FROM applications WHERE id = ?", applicationID).Scan(&updatedApp)

	return &RevokeApprovalResponse{
		Success:      true,
		Message:      "Approval revoked successfully",
		Confirmation: updatedApp.Confirmation,
		Status:       updatedApp.Status,
	}, nil
}
