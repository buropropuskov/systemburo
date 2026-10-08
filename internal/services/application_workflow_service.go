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
	if err := s.checkNotArchived(ctx, applicationID); err != nil {
		return err
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

	var app struct {
		Status       *string
		Confirmation *string
	}
	result := tx.Raw("SELECT status, confirmation FROM applications WHERE id = ? FOR UPDATE", applicationID).Scan(&app)
	if result.Error != nil || result.RowsAffected == 0 {
		tx.Rollback()
		return echo.NewHTTPError(http.StatusNotFound, "Application not found")
	}
	oldStatus := app.Status

	if req.Action == "accept" {
		if oldStatus != nil && *oldStatus == models.StatusCompleted {
			tx.Rollback()
			return echo.NewHTTPError(http.StatusBadRequest, "Завершённую заявку нельзя принять в работу повторно")
		}
		if oldStatus != nil && *oldStatus == models.StatusInWork {
			tx.Rollback()
			return echo.NewHTTPError(http.StatusBadRequest, "Application is already in work")
		}

		// Гейт согласования: принять в работу можно только когда заявка согласована
		// (confirmation='Согласовано' = все обязательные approved, при отсутствии обязательных -
		// хотя бы один approved). Исключение - заявка вообще без согласующих (согласовывать
		// нечего) - её принять можно. FE-кнопка "Согласовать и принять" тоже это проверяет,
		// но барьер обязан быть на бэке: FE-гейт обходится гонкой/устаревшим состоянием.
		if app.Confirmation == nil || *app.Confirmation != models.ConfirmationApproved {
			var approverCount int64
			if err := tx.Model(&models.ApplicationResponsibleUser{}).
				Where("application_id = ?", applicationID).Count(&approverCount).Error; err != nil {
				tx.Rollback()
				return echo.NewHTTPError(http.StatusInternalServerError, "Error checking approvals")
			}
			if approverCount > 0 {
				tx.Rollback()
				return echo.NewHTTPError(http.StatusBadRequest, "Заявку нельзя принять в работу: не завершено согласование")
			}
			// Согласующих нет - согласовывать нечего, и решение принимающего его заменяет:
			// отмечаем согласование выполненным. Иначе заявка остаётся в подтверждении
			// "Согласование" навсегда: пересчёт голосов на пустом списке ничего не меняет,
			// и заявитель видит принятую в работу заявку как ожидающую согласования.
			if err := tx.Exec(`
				UPDATE applications
				SET confirmation = ?,
				    confirmation_datetime = COALESCE(confirmation_datetime, NOW())
				WHERE id = ?`, models.ConfirmationApproved, applicationID).Error; err != nil {
				tx.Rollback()
				return echo.NewHTTPError(http.StatusInternalServerError, "Error updating confirmation")
			}
		}

		// accepted_at через COALESCE: заявку могли отозвать из работы и принять снова
		// (revoke/restore -> "В обработке"), но T2 воронки обработки - ПЕРВОЕ принятие.
		tx.Exec("UPDATE applications SET status = ?, responsible_user_id = ?, responsible_comment = ?, accepted_at = COALESCE(accepted_at, NOW()) WHERE id = ?",
			models.StatusInWork, user.ID, req.Comment, applicationID)

		s.recorder.Log(ctx, tx, models.AuditEntityApplication, &applicationID, models.AuditActionTakeToWork, &user.ID,
			applicationAuditDetails{OldValue: oldStatus, NewValue: ptrString(models.StatusInWork), Comment: req.Comment})

		if err := s.bumpStatusUpdated(tx, applicationID, &user.ID); err != nil {
			tx.Rollback()
			return err
		}

		if err := s.activateApplicationItems(ctx, tx, applicationID, true, &user.ID); err != nil {
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

		s.recorder.Log(ctx, tx, models.AuditEntityApplication, &applicationID, models.AuditActionReject, &user.ID,
			applicationAuditDetails{OldValue: oldStatus, NewValue: ptrString(models.StatusRefused), Comment: req.Comment})

		if err := s.bumpStatusUpdated(tx, applicationID, &user.ID); err != nil {
			tx.Rollback()
			return err
		}

		if err := s.activateApplicationItems(ctx, tx, applicationID, false, nil); err != nil {
			tx.Rollback()
			return err
		}

		if err := s.cancelOpenSupplements(ctx, tx, applicationID); err != nil {
			tx.Rollback()
			return err
		}
	}

	if err := tx.Commit().Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to commit transaction")
	}

	if req.Action == "accept" {
		// Принятие активировало машины и сотрудников - посты показывают их без F5 (#840 V2.2).
		s.tablesProducer.NotifyApplicationActivated(ctx, applicationID)
	}
	s.notifyApplicationUpdated(ctx, applicationID, archiveDataChanged)
	// Инициатору - уведомление об исходе принятия/отказа (#1349). Гейт actor != sender
	// внутри хелпера: если принимающий = отправитель, себе не шлём.
	decision := &statusChangeContext{
		ActorName: formatFullName(user.LastName, user.FirstName, user.MiddleName),
		Comment:   optionalString(req.Comment),
	}
	if req.Action == "accept" {
		s.notifyInitiatorStatusChanged(ctx, applicationID, &user.ID, statusOutcomeAccepted, decision)
	} else if req.Action == "reject" {
		s.notifyInitiatorStatusChanged(ctx, applicationID, &user.ID, statusOutcomeRejected, decision)
	}
	return nil
}

// RevokeApplicationFromWork отзывает заявку из работы и возвращает в статус обработки.
func (s *applicationService) RevokeApplicationFromWork(ctx context.Context, username string, applicationID int, req RevokeFromWorkRequest) error {
	return s.returnToProcessing(ctx, username, applicationID, req, backToProcessing{
		from:      models.StatusInWork,
		action:    "revoke_from_work",
		forbidden: "Only approver can revoke the application",
		conflict:  "Отозвать из работы можно только заявку в работе",
	})
}

// RestoreApplicationToWork возвращает отказанную заявку в статус обработки.
func (s *applicationService) RestoreApplicationToWork(ctx context.Context, username string, applicationID int, req RevokeFromWorkRequest) error {
	return s.returnToProcessing(ctx, username, applicationID, req, backToProcessing{
		from:      models.StatusRefused,
		action:    "restore_to_work",
		forbidden: "Only approver can restore the application",
		conflict:  "Вернуть в работу можно только заявку, в которой отказано",
	})
}

// backToProcessing - откуда и под каким именем заявка возвращается в обработку.
type backToProcessing struct {
	from      string
	action    string
	forbidden string
	conflict  string
}

// returnToProcessing переводит заявку принимающим обратно в "В обработке" и гасит её
// строки. Исходный статус строго один: без этой сверки завершённую или несогласованную
// заявку можно было вернуть в обработку и заново принять, а архивную - вытащить из архива.
func (s *applicationService) returnToProcessing(ctx context.Context, username string, applicationID int, req RevokeFromWorkRequest, t backToProcessing) error {
	user, err := s.getUserByUsername(ctx, username)
	if err != nil {
		return err
	}

	isApprover, err := s.isApprover(ctx, user.ID)
	if err != nil {
		return err
	}
	if !isApprover {
		return echo.NewHTTPError(http.StatusForbidden, t.forbidden)
	}
	if err := s.checkNotArchived(ctx, applicationID); err != nil {
		return err
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

	// FOR UPDATE: параллельный переход (принятие, отзыв отправителем) не проскочит
	// между сверкой статуса и записью.
	var app struct{ Status *string }
	result := tx.Raw("SELECT status FROM applications WHERE id = ? FOR UPDATE", applicationID).Scan(&app)
	if result.Error != nil || result.RowsAffected == 0 {
		tx.Rollback()
		return echo.NewHTTPError(http.StatusNotFound, "Application not found")
	}
	if app.Status == nil || *app.Status != t.from {
		tx.Rollback()
		return echo.NewHTTPError(http.StatusConflict, t.conflict)
	}

	tx.Exec("UPDATE applications SET status = ?, responsible_user_id = NULL, responsible_comment = NULL WHERE id = ?", models.StatusProcessing, applicationID)

	s.recorder.Log(ctx, tx, models.AuditEntityApplication, &applicationID, t.action, &user.ID,
		applicationAuditDetails{OldValue: app.Status, NewValue: ptrString(models.StatusProcessing), Comment: req.Comment})

	if err := s.bumpStatusUpdated(tx, applicationID, &user.ID); err != nil {
		tx.Rollback()
		return err
	}

	if err := s.activateApplicationItems(ctx, tx, applicationID, false, nil); err != nil {
		tx.Rollback()
		return err
	}

	// Тот же переход снимает и открытый раунд дополнения (#1685): строки заявки погашены,
	// принимать раунду уже нечего. Иначе pending висел бы у согласующих вечной задачей.
	if err := s.cancelOpenSupplements(ctx, tx, applicationID); err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Commit().Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to commit transaction")
	}

	s.notifyApplicationUpdated(ctx, applicationID, archiveDataChanged)
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

	// Кому уведомление об отзыве (#1748, S4): согласующие, чьё решение ещё не
	// поступило. Собираем ДО смены статуса - предикат матчит только живую заявку.
	pendingApproverIDs := s.pendingApproversBeforeWithdraw(ctx, tx, applicationID)

	// withdrawn_at - точка отсчёта месяца до архива (вложения при отзыве гасятся,
	// их сроки для архивации больше не показательны).
	if err := tx.Exec("UPDATE applications SET status = ?, withdrawn_at = NOW() WHERE id = ?", models.StatusWithdrawn, applicationID).Error; err != nil {
		tx.Rollback()
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to withdraw application")
	}

	s.recorder.Log(ctx, tx, models.AuditEntityApplication, &applicationID, models.AuditActionWithdraw, &user.ID,
		applicationAuditDetails{OldValue: app.Status, NewValue: ptrString(models.StatusWithdrawn)})

	// Актор - сам отправитель: флаг загорится у принимающих/согласующих, но не у него (#1349).
	if err := s.bumpStatusUpdated(tx, applicationID, &user.ID); err != nil {
		tx.Rollback()
		return err
	}

	// Деактивируем машины и сотрудников вложений...
	if err := s.activateApplicationItems(ctx, tx, applicationID, false, nil); err != nil {
		tx.Rollback()
		return err
	}

	// Тот же переход снимает и открытый раунд дополнения (#1685): строки заявки погашены,
	// принимать раунду уже нечего. Иначе pending висел бы у согласующих вечной задачей.
	if err := s.cancelOpenSupplements(ctx, tx, applicationID); err != nil {
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

	s.notifyApplicationUpdated(ctx, applicationID, archiveDataChanged)
	s.notifyWithdrawn(ctx, applicationID, formatFullName(user.LastName, user.FirstName, user.MiddleName), pendingApproverIDs)
	return nil
}

// CheckExpiredAttachments проверяет и деактивирует вложения с истёкшим сроком действия.
func (s *applicationService) CheckExpiredAttachments(ctx context.Context) error {
	// Match the admission boundary: an end equal to now is already expired.
	period, err := EntityEffectivePeriodSQL("e", "a")
	if err != nil {
		return err
	}
	expiredEntity := "(" + period.ValidMode + " AND " + period.Bounded +
		" AND (NULLIF(BTRIM(" + period.DateTo + "), '')::date + COALESCE(NULLIF(BTRIM(" + period.TimeTo + "), '')::time, TIME '23:59:59')) <= " + moscowNowSQL + ")"
	expiredParent := "NOT " + passValidNowSQL("a")
	tx := s.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return tx.Error
	}
	// Also rolls back on panic; do not swallow it and report a successful cron.
	defer tx.Rollback()
	type attachmentRow struct {
		ID            int
		ApplicationID *int
	}
	type idRow struct{ ID int }
	var candidates []attachmentRow
	if err := tx.Raw(`SELECT a.id, a.application_id FROM attachments a
		WHERE a.status = 1 AND (` + expiredParent + `
		OR EXISTS (SELECT 1 FROM cars e WHERE e.attachment_id = a.id AND e.status = 1 AND ` + expiredEntity + `)
		OR EXISTS (SELECT 1 FROM employees e WHERE e.attachment_id = a.id AND e.status = 1 AND ` + expiredEntity + `))
		ORDER BY a.id`).Scan(&candidates).Error; err != nil {
		return err
	}
	if len(candidates) == 0 {
		return nil
	}
	attachmentIDs := make([]int, 0, len(candidates))
	appIDs := make([]int, 0, len(candidates))
	originalParents := make(map[int]*int, len(candidates))
	for _, candidate := range candidates {
		attachmentIDs = append(attachmentIDs, candidate.ID)
		originalParents[candidate.ID] = candidate.ApplicationID
		if candidate.ApplicationID != nil {
			appIDs = append(appIDs, *candidate.ApplicationID)
		}
	}
	slices.Sort(appIDs)
	appIDs = slices.Compact(appIDs)
	// Same order as period writers: all applications, all attachments, employees,
	// cars. This also matches the legacy territory reset's entity order. Never acquire an application
	// lock after locking its attachment or entity.
	var apps []struct {
		ID     int
		Status *string
	}
	if len(appIDs) > 0 {
		if err := tx.Raw("SELECT id, status FROM applications WHERE id IN ? ORDER BY id FOR UPDATE", appIDs).Scan(&apps).Error; err != nil {
			return err
		}
	}
	appStatuses := make(map[int]*string, len(apps))
	for _, app := range apps {
		appStatuses[app.ID] = app.Status
	}
	var locked []attachmentRow
	if err := tx.Raw("SELECT id, application_id FROM attachments WHERE id IN ? ORDER BY id FOR UPDATE", attachmentIDs).Scan(&locked).Error; err != nil {
		return err
	}
	attachmentIDs = attachmentIDs[:0]
	for _, attachment := range locked {
		original := originalParents[attachment.ID]
		// A concurrent manual-attach/relink is handled by the next cron pass;
		// its new application is not one of the application locks above.
		if (original == nil) != (attachment.ApplicationID == nil) {
			continue
		}
		if original != nil && *original != *attachment.ApplicationID {
			continue
		}
		if attachment.ApplicationID != nil {
			if _, ok := appStatuses[*attachment.ApplicationID]; !ok {
				continue
			}
		}
		attachmentIDs = append(attachmentIDs, attachment.ID)
	}
	if len(attachmentIDs) == 0 {
		return nil
	}
	for _, table := range []string{"employees", "cars"} {
		var lockedEntities []idRow
		if err := tx.Raw("SELECT id FROM "+table+" WHERE attachment_id IN ? ORDER BY id FOR UPDATE", attachmentIDs).Scan(&lockedEntities).Error; err != nil {
			return err
		}
	}
	// Re-evaluate the effective window after acquiring the locks. A period
	// extension committed before these locks must win over the initial scan.
	changedAttachments := make(map[int]bool)
	var changedCars, changedEmployees []int
	// Only actual expiry transitions permit closing a typed attachment early.
	expiredChildAttachments := map[string][]int{}

	for _, target := range []struct{ table, entityType, detailColumns string }{
		{"cars", models.AuditEntityCar, "e.car_number, e.car_brand"},
		{"employees", models.AuditEntityEmployee, "e.last_name, e.first_name, e.middle_name"},
	} {
		var changed []struct {
			ID           int
			AttachmentID int
			CarNumber    string
			CarBrand     string
			LastName     *string
			FirstName    *string
			MiddleName   *string
		}
		if err := tx.Raw("UPDATE "+target.table+" e SET status = 0, updated_at = NOW() FROM attachments a WHERE a.id = e.attachment_id AND a.id IN ? AND a.status = 1 AND e.status = 1 AND "+expiredEntity+" RETURNING e.id, e.attachment_id, "+target.detailColumns, attachmentIDs).Scan(&changed).Error; err != nil {
			return err
		}
		for _, entity := range changed {
			changedAttachments[entity.AttachmentID] = true
			expiredChildAttachments[target.table] = append(expiredChildAttachments[target.table], entity.AttachmentID)
			id := entity.ID
			comment := fmt.Sprintf("Срок действия заявки на автомобиль %s %s истёк", entity.CarNumber, entity.CarBrand)
			if target.table == "employees" {
				comment = fmt.Sprintf("Срок действия заявки на сотрудника %s истёк", formatFullName(entity.LastName, entity.FirstName, entity.MiddleName))
			}
			if err := s.recorder.Record(ctx, tx, target.entityType, &id, "deactivate", nil, carAuditDetails{Comment: &comment}); err != nil {
				return err
			}
			if target.table == "cars" {
				changedCars = append(changedCars, id)
			} else {
				changedEmployees = append(changedEmployees, id)
			}
		}
	}
	// For people/cars the final actual admission can end before the parent
	// window. An empty attachment or one manually emptied earlier does not
	// become expired just because it has no active children. Other attachment
	// types keep their existing parent-window expiry policy.
	earlyEnd := "(FALSE"
	parentArgs := []any{attachmentIDs}
	for _, target := range []struct{ table, attachmentType string }{
		{"cars", "cars"}, {"employees", "people"},
	} {
		ids := expiredChildAttachments[target.table]
		if len(ids) == 0 {
			continue
		}
		slices.Sort(ids)
		ids = slices.Compact(ids)
		earlyEnd += " OR (a.attachment_type = '" + target.attachmentType + "' AND a.id IN ? AND EXISTS (SELECT 1 FROM " + target.table + " member WHERE member.attachment_id = a.id))"
		parentArgs = append(parentArgs, ids)
	}
	earlyEnd += ")"
	var expiredAttachments []attachmentRow
	if err := tx.Raw(`UPDATE attachments a SET status = 0, updated_at = NOW()
		WHERE a.id IN ? AND a.status = 1 AND (`+expiredParent+` OR `+earlyEnd+`)
		AND NOT EXISTS (SELECT 1 FROM cars e WHERE e.attachment_id = a.id AND e.status = 1)
		AND NOT EXISTS (SELECT 1 FROM employees e WHERE e.attachment_id = a.id AND e.status = 1)
		RETURNING a.id, a.application_id`, parentArgs...).Scan(&expiredAttachments).Error; err != nil {
		return err
	}
	for _, attachment := range expiredAttachments {
		changedAttachments[attachment.ID] = true
	}
	changedApps := make(map[int]bool)
	for _, attachment := range locked {
		if changedAttachments[attachment.ID] && attachment.ApplicationID != nil {
			changedApps[*attachment.ApplicationID] = true
		}
	}
	var completedAppIDs []int
	for _, appID := range appIDs {
		if !changedApps[appID] {
			continue
		}
		// Preserve existing final-status policy and complete only once. An
		// individual admission keeps its attachment active, hence its app too.
		res := tx.Exec(`UPDATE applications SET status = ?, completed_at = NOW()
			WHERE id = ? AND COALESCE(status, '') NOT IN (?)
			AND NOT EXISTS (SELECT 1 FROM attachments WHERE application_id = ? AND status = 1)`,
			models.StatusCompleted, appID, models.ArchivableStatuses, appID)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			continue
		}
		id := appID
		if err := s.recorder.Record(ctx, tx, models.AuditEntityApplication, &id, "completed", nil,
			applicationAuditDetails{OldValue: appStatuses[id], NewValue: ptrString(models.StatusCompleted)}); err != nil {
			return err
		}
		if err := s.bumpStatusUpdated(tx, id, nil); err != nil {
			return err
		}
		if err := s.cancelOpenSupplements(ctx, tx, id); err != nil {
			return err
		}
		completedAppIDs = append(completedAppIDs, id)
	}
	if err := tx.Commit().Error; err != nil {
		return err
	}
	// Publish only committed transitions, including manual rows with no app.
	s.tablesProducer.NotifyCarsChangedBatch(ctx, changedCars)
	s.tablesProducer.NotifyEmployeesChangedBatch(ctx, changedEmployees)
	for _, id := range appIDs {
		if changedApps[id] {
			s.notifyApplicationUpdated(ctx, id, archiveDataChanged)
		}
	}
	for _, id := range completedAppIDs {
		s.notifyInitiatorStatusChanged(ctx, id, nil, statusOutcomeCompleted, nil)
	}
	slog.Info("Проверка истекших допусков завершена", "cars", len(changedCars), "employees", len(changedEmployees), "applications", len(completedAppIDs))
	return nil
}

// --- Утилиты ---
