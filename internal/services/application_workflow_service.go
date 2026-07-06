package services

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"slices"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
)

// TakeApplicationToWork принимает заявку в работу или отказывает в ней.
func (s *applicationService) TakeApplicationToWork(ctx context.Context, username string, applicationID int, req TakeToWorkRequest) error {
	if err := s.checkNotArchived(ctx, applicationID); err != nil {
		return err
	}
	if err := s.checkNotWithdrawn(ctx, applicationID); err != nil {
		return err
	}

	user, err := s.getUserByUsername(ctx, username)
	if err != nil {
		return err
	}

	isApprover, err := s.isApprover(ctx, user.ID)
	if err != nil {
		return err
	}
	if !isApprover {
		return echo.NewHTTPError(http.StatusForbidden, "User is not an approver")
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

	var app struct {
		Status *string
	}
	result := tx.Raw("SELECT status FROM applications WHERE id = ?", applicationID).Scan(&app)
	if result.Error != nil || result.RowsAffected == 0 {
		tx.Rollback()
		return echo.NewHTTPError(http.StatusNotFound, "Application not found")
	}
	oldStatus := app.Status

	if req.Action == "accept" {
		if oldStatus != nil && *oldStatus == models.StatusInWork {
			tx.Rollback()
			return echo.NewHTTPError(http.StatusBadRequest, "Application is already in work")
		}

		tx.Exec("UPDATE applications SET status = ?, responsible_user_id = ?, responsible_comment = ? WHERE id = ?",
			models.StatusInWork, user.ID, req.Comment, applicationID)

		s.recorder.Log(ctx, tx, models.AuditEntityApplication, &applicationID, "take_to_work", &user.ID,
			applicationAuditDetails{OldValue: oldStatus, NewValue: ptrString(models.StatusInWork), Comment: req.Comment})

		if err := s.activateApplicationItems(tx, applicationID, true); err != nil {
			tx.Rollback()
			return err
		}
	} else if req.Action == "reject" {
		if oldStatus != nil && *oldStatus == models.StatusRefused {
			tx.Rollback()
			return echo.NewHTTPError(http.StatusBadRequest, "Application is already rejected")
		}

		tx.Exec("UPDATE applications SET status = ?, responsible_user_id = ?, responsible_comment = ? WHERE id = ?",
			models.StatusRefused, user.ID, req.Comment, applicationID)

		s.recorder.Log(ctx, tx, models.AuditEntityApplication, &applicationID, "reject", &user.ID,
			applicationAuditDetails{OldValue: oldStatus, NewValue: ptrString(models.StatusRefused), Comment: req.Comment})

		if err := s.activateApplicationItems(tx, applicationID, false); err != nil {
			tx.Rollback()
			return err
		}
	}

	if err := tx.Commit().Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to commit transaction")
	}

	return nil
}

// RevokeApplicationFromWork отзывает заявку из работы и возвращает в статус обработки.
func (s *applicationService) RevokeApplicationFromWork(ctx context.Context, username string, applicationID int, req RevokeFromWorkRequest) error {
	user, err := s.getUserByUsername(ctx, username)
	if err != nil {
		return err
	}

	isApprover, err := s.isApprover(ctx, user.ID)
	if err != nil {
		return err
	}
	if !isApprover {
		return echo.NewHTTPError(http.StatusForbidden, "Only approver can revoke the application")
	}
	if err := s.checkNotWithdrawn(ctx, applicationID); err != nil {
		return err
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

	var app struct{ Status *string }
	result := tx.Raw("SELECT status FROM applications WHERE id = ?", applicationID).Scan(&app)
	if result.Error != nil || result.RowsAffected == 0 {
		tx.Rollback()
		return echo.NewHTTPError(http.StatusNotFound, "Application not found")
	}

	tx.Exec("UPDATE applications SET status = ?, responsible_user_id = NULL, responsible_comment = NULL WHERE id = ?", models.StatusProcessing, applicationID)

	s.recorder.Log(ctx, tx, models.AuditEntityApplication, &applicationID, "revoke_from_work", &user.ID,
		applicationAuditDetails{OldValue: app.Status, NewValue: ptrString(models.StatusProcessing), Comment: req.Comment})

	if err := s.activateApplicationItems(tx, applicationID, false); err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Commit().Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to commit transaction")
	}

	return nil
}

// RestoreApplicationToWork возвращает заявку в статус обработки.
func (s *applicationService) RestoreApplicationToWork(ctx context.Context, username string, applicationID int, req RevokeFromWorkRequest) error {
	user, err := s.getUserByUsername(ctx, username)
	if err != nil {
		return err
	}

	isApprover, err := s.isApprover(ctx, user.ID)
	if err != nil {
		return err
	}
	if !isApprover {
		return echo.NewHTTPError(http.StatusForbidden, "Only approver can restore the application")
	}
	if err := s.checkNotWithdrawn(ctx, applicationID); err != nil {
		return err
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

	var app struct{ Status *string }
	result := tx.Raw("SELECT status FROM applications WHERE id = ?", applicationID).Scan(&app)
	if result.Error != nil || result.RowsAffected == 0 {
		tx.Rollback()
		return echo.NewHTTPError(http.StatusNotFound, "Application not found")
	}

	tx.Exec("UPDATE applications SET status = ?, responsible_user_id = NULL, responsible_comment = NULL WHERE id = ?", models.StatusProcessing, applicationID)

	s.recorder.Log(ctx, tx, models.AuditEntityApplication, &applicationID, "restore_to_work", &user.ID,
		applicationAuditDetails{OldValue: app.Status, NewValue: ptrString(models.StatusProcessing), Comment: req.Comment})

	if err := s.activateApplicationItems(tx, applicationID, false); err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Commit().Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to commit transaction")
	}

	return nil
}

// WithdrawApplication отзывает СВОЮ заявку отправителем (#951): статус -> "Отозвана",
// машины/сотрудники/вложения деактивируются, в историю пишется кто и когда отозвал.
// Обратного пути нет - вернуть в работу нельзя, только продублировать. Отозвать может
// только отправитель и только пока заявка не в терминальном (закрытом) статусе.
func (s *applicationService) WithdrawApplication(ctx context.Context, username string, applicationID int) error {
	user, err := s.getUserByUsername(ctx, username)
	if err != nil {
		return err
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

	// Читаем статус/владельца внутри транзакции с блокировкой строки (FOR UPDATE),
	// чтобы конкурентное действие не проскочило мимо терминального гейта.
	var app struct {
		Status       *string
		SenderUserID int
	}
	result := tx.Raw("SELECT status, sender_user_id FROM applications WHERE id = ? FOR UPDATE", applicationID).Scan(&app)
	if result.Error != nil {
		tx.Rollback()
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to load application")
	}
	if result.RowsAffected == 0 {
		tx.Rollback()
		return echo.NewHTTPError(http.StatusNotFound, "Application not found")
	}
	if app.SenderUserID != user.ID {
		tx.Rollback()
		return echo.NewHTTPError(http.StatusForbidden, "Отозвать можно только собственную заявку")
	}
	// Терминальные (закрытые) статусы совпадают с ArchivableStatuses - из них отзыв запрещён.
	if app.Status != nil && slices.Contains(models.ArchivableStatuses, *app.Status) {
		tx.Rollback()
		return echo.NewHTTPError(http.StatusConflict, "Заявку в этом статусе отозвать нельзя")
	}

	if err := tx.Exec("UPDATE applications SET status = ? WHERE id = ?", models.StatusWithdrawn, applicationID).Error; err != nil {
		tx.Rollback()
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to withdraw application")
	}

	s.recorder.Log(ctx, tx, models.AuditEntityApplication, &applicationID, "withdraw", &user.ID,
		applicationAuditDetails{OldValue: app.Status, NewValue: ptrString(models.StatusWithdrawn)})

	// Деактивируем машины и сотрудников вложений...
	if err := s.activateApplicationItems(tx, applicationID, false); err != nil {
		tx.Rollback()
		return err
	}
	// ...и сами вложения (общий helper их не трогает - он про cars/employees).
	if err := tx.Exec("UPDATE attachments SET status = 0 WHERE application_id = ?", applicationID).Error; err != nil {
		tx.Rollback()
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to deactivate attachments")
	}

	if err := tx.Commit().Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to commit transaction")
	}

	return nil
}

// UpdateApplicationItemsStatus обновляет статусы машин и сотрудников заявки.
func (s *applicationService) UpdateApplicationItemsStatus(ctx context.Context, applicationID int) error {
	// Отозванную заявку нельзя реактивировать через массовое выставление статусов (#951).
	if err := s.checkNotWithdrawn(ctx, applicationID); err != nil {
		return err
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

	type attachmentRow struct {
		ID             int
		AttachmentType string
	}
	var attachments []attachmentRow
	if err := tx.Raw("SELECT id, attachment_type FROM attachments WHERE application_id = ?", applicationID).Scan(&attachments).Error; err != nil {
		tx.Rollback()
		return echo.NewHTTPError(http.StatusInternalServerError, "Error fetching attachments")
	}

	for _, att := range attachments {
		switch att.AttachmentType {
		case "cars":
			tx.Exec("UPDATE cars SET status = 1, updated_at = CURRENT_TIMESTAMP WHERE attachment_id = ?", att.ID)
		case "people":
			tx.Exec("UPDATE employees SET status = 1, updated_at = CURRENT_TIMESTAMP WHERE attachment_id = ?", att.ID)
		}
	}

	if err := tx.Commit().Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to commit transaction")
	}

	// Активированные машины/сотрудники появились в таблицах проходной - сигналим
	// их аудитории обновиться live (#840 V2.2). После commit: строки уже видны.
	s.tablesProducer.NotifyApplicationActivated(ctx, applicationID)

	return nil
}

// CheckExpiredAttachments проверяет и деактивирует вложения с истёкшим сроком действия.
func (s *applicationService) CheckExpiredAttachments(ctx context.Context) error {
	slog.Info("Проверка истекших вложений...")

	tx := s.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Database error")
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	type expiredRow struct {
		ID            int
		ApplicationID int
	}
	var expired []expiredRow
	tx.Raw(`
		SELECT id, application_id FROM attachments
		WHERE status = 1 AND (
			(entry_date_to IS NOT NULL AND CAST(entry_date_to AS DATE) < CURRENT_DATE)
			OR (entry_date_to IS NOT NULL AND entry_time_to IS NOT NULL
			    AND (CAST(entry_date_to AS DATE) + CAST(entry_time_to AS TIME)) AT TIME ZONE 'Europe/Moscow' < CURRENT_TIMESTAMP)
		)
	`).Scan(&expired)

	if len(expired) == 0 {
		tx.Rollback()
		slog.Info("Истекших вложений не найдено")
		return nil
	}

	slog.Info("Найдено истекших вложений", "count", len(expired))

	attachmentIDs := make([]int, len(expired))
	appIDs := make([]int, len(expired))
	for i, e := range expired {
		attachmentIDs[i] = e.ID
		appIDs[i] = e.ApplicationID
	}

	// Получаем машины для истории
	type carDeactivate struct {
		ID        int
		CarNumber string
		CarBrand  string
	}
	var cars []carDeactivate
	tx.Raw("SELECT id, car_number, car_brand FROM cars WHERE attachment_id IN ?", attachmentIDs).Scan(&cars)

	// Получаем сотрудников для истории
	type employeeDeactivate struct {
		ID         int
		LastName   *string
		FirstName  *string
		MiddleName *string
	}
	var employees []employeeDeactivate
	tx.Raw("SELECT id, last_name, first_name, middle_name FROM employees WHERE attachment_id IN ?", attachmentIDs).Scan(&employees)

	tx.Exec("UPDATE attachments SET status = 0 WHERE id IN ?", attachmentIDs)
	tx.Exec("UPDATE cars SET status = 0 WHERE attachment_id IN ?", attachmentIDs)
	tx.Exec("UPDATE employees SET status = 0 WHERE attachment_id IN ?", attachmentIDs)

	for _, car := range cars {
		carID := car.ID
		comment := fmt.Sprintf("Срок действия заявки на автомобиль %s %s истёк", car.CarNumber, car.CarBrand)
		s.recorder.Log(ctx, tx, models.AuditEntityCar, &carID, "deactivate", nil, carAuditDetails{Comment: &comment})
	}

	for _, emp := range employees {
		empID := emp.ID
		fullName := formatFullName(emp.LastName, emp.FirstName, emp.MiddleName)
		comment := fmt.Sprintf("Срок действия заявки на сотрудника %s истёк", fullName)
		s.recorder.Log(ctx, tx, models.AuditEntityEmployee, &empID, "deactivate", nil, carAuditDetails{Comment: &comment})
	}

	// Завершаем заявки, у которых все вложения неактивны
	uniqueAppIDs := make(map[int]bool)
	for _, id := range appIDs {
		uniqueAppIDs[id] = true
	}
	for appID := range uniqueAppIDs {
		var activeCount int64
		tx.Raw("SELECT COUNT(*) FROM attachments WHERE application_id = ? AND status = 1", appID).Scan(&activeCount)
		if activeCount == 0 {
			tx.Exec("UPDATE applications SET status = ? WHERE id = ?", models.StatusCompleted, appID)
			slog.Info("Заявка завершена", "application_id", appID)
		}
	}

	if err := tx.Commit().Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to commit transaction")
	}

	slog.Info("Проверка истекших вложений завершена")
	return nil
}

// --- Утилиты ---
