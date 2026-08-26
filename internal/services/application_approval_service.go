package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
)

// forwardAuthority - что пересылающий вправе сделать на конкретной заявке.
type forwardAuthority struct {
	// readerOnly - доступ к заявке есть только как у просматривающего
	// (application_viewers): передать дальше можно, но лишь на просмотр.
	readerOnly bool
	// allowedRecipients - круг, кому этот пользователь вправе переслать заявку.
	// nil означает "без ограничения": кому и почему, разобрано в комментарии к
	// методу forwardAuthority ниже.
	allowedRecipients map[int]struct{}
}

// allowsRecipient - можно ли назначать получателем этого пользователя.
func (a forwardAuthority) allowsRecipient(userID int) bool {
	if a.allowedRecipients == nil {
		return true
	}
	_, ok := a.allowedRecipients[userID]
	return ok
}

// forwardAuthority определяет роль пересылающего на заявке и круг его получателей.
//
// Белый список получателей (тот же предикат, что и на подаче: своя организация, своя
// компания, руководители) НЕ применяется к супер-админу и принимающему. Это ровно те две
// роли, которые CanAccessApplication пускает к заявке без всякой привязки к ней
// (супер-админ безусловно, принимающий - через isApprover), и которые
// applyApplicationAccessFilter не фильтрует вовсе: они и так видят все заявки системы.
// Круг recipientCandidateIDs строится от организации и компании самого пользователя, а у
// оператора бюро это бюро - маршрутизация заявок по чужим организациям и есть его работа,
// и сужение до своих коллег отняло бы её. Смысл списка в другом: не дать рядовому автору
// открыть заявку кому угодно, и для рядового он работает.
func (s *applicationService) forwardAuthority(ctx context.Context, applicationID int, user *models.User, isSuperAdmin bool) (forwardAuthority, error) {
	if isSuperAdmin {
		return forwardAuthority{}, nil
	}
	isApprover, err := s.isApprover(ctx, user.ID)
	if err != nil {
		return forwardAuthority{}, err
	}
	if isApprover {
		return forwardAuthority{}, nil
	}

	var role struct {
		IsSender      bool
		IsResponsible bool
	}
	if err := s.db.WithContext(ctx).Raw(`
		SELECT
			EXISTS(SELECT 1 FROM applications a WHERE a.id = ? AND a.sender_user_id = ?) AS is_sender,
			EXISTS(SELECT 1 FROM application_responsible_users aru WHERE aru.application_id = ? AND aru.user_id = ?) AS is_responsible
	`, applicationID, user.ID, applicationID, user.ID).Scan(&role).Error; err != nil {
		slog.Error("не удалось определить роль пересылающего", "application_id", applicationID, "user_id", user.ID, "error", err)
		return forwardAuthority{}, echo.NewHTTPError(http.StatusInternalServerError, "Database error")
	}

	allowed, err := recipientCandidateIDs(ctx, s.db, *user)
	if err != nil {
		return forwardAuthority{}, err
	}
	return forwardAuthority{
		readerOnly:        !role.IsSender && !role.IsResponsible,
		allowedRecipients: allowed,
	}, nil
}

// ForwardApplication пересылает заявку указанным ответственным и просматривающим.
//
// Пересылка разрешена и для архивных заявок (#869): это маршрутизация (аддитивно
// добавляет получателей), а не изменение статуса. Read-only архива оставлен только
// на статус-меняющих действиях (взять в работу, согласование) - они зовут
// checkNotArchived сами.
func (s *applicationService) ForwardApplication(ctx context.Context, username string, applicationID int, isSuperAdmin bool, req ForwardApplicationRequest) error {
	user, err := s.getUserByUsername(ctx, username)
	if err != nil {
		return err
	}
	if err := s.checkNotWithdrawn(ctx, applicationID); err != nil {
		return err
	}
	currentUserName := formatFullName(user.LastName, user.FirstName, user.MiddleName)

	var exists bool
	if err := s.db.WithContext(ctx).Raw("SELECT EXISTS(SELECT 1 FROM applications WHERE id = ?)", applicationID).Scan(&exists).Error; err != nil {
		slog.Error("не удалось проверить существование заявки при пересылке", "application_id", applicationID, "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Database error")
	}
	if !exists {
		return echo.NewHTTPError(http.StatusNotFound, "Application not found")
	}

	// Гейт пересылки = гейт доступа к заявке. Свой SQL (отправитель ИЛИ строка в
	// application_responsible_users) не пускал ни супер-админа, ни принимающего, ни
	// читателя: заявку они видели, а переслать её не могли. Второй набор ролей рядом
	// с CanAccessApplication неизбежно разъезжается с ним - держим один.
	if !s.CanAccessApplication(ctx, applicationID, username, isSuperAdmin) {
		return echo.NewHTTPError(http.StatusForbidden, "You don't have permission to forward this application")
	}

	authority, err := s.forwardAuthority(ctx, applicationID, user, isSuperAdmin)
	if err != nil {
		return err
	}
	// Просматривающий передаёт заявку дальше только для просмотра. Ответственный и
	// согласующий - назначения от имени заявки (согласующий её ещё и голосует), и
	// раздавать их вправе тот, кто за заявку отвечает, а не тот, кому её показали.
	if authority.readerOnly {
		for _, fu := range req.Users {
			if fu.RequiredApproval || !fu.CanView {
				return echo.NewHTTPError(http.StatusForbidden,
					"Заявка доступна вам только для просмотра: переслать её можно тоже для просмотра, назначать ответственных и согласующих вправе отправитель")
			}
		}
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

	// Сохраняем старый confirmation
	var oldConfirmation *string
	tx.Raw("SELECT confirmation FROM applications WHERE id = ?", applicationID).Scan(&oldConfirmation)

	baseTime := time.Now().UTC()

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

		// Получателем можно назначить только того, кого форма и предлагала выбрать -
		// иначе подделанный запрос открывает заявку любому пользователю системы тем же
		// INSERT в application_viewers, который закрыт на подаче. Чужой идентификатор
		// отбрасываем молча, как и там: запрос остаётся валидным, пересылка проходит.
		if !authority.allowsRecipient(fu.UserID) {
			slog.Warn("получатель пересылки отброшен: вне списка доступных получателей",
				"application_id", applicationID, "recipient_id", fu.UserID, "actor_id", user.ID)
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

	// Пер-вложенный пересыл (#680): фиксируем, какие вложения видит каждый получатель.
	// Список общий на действие, разворачиваем построчно (получатель x вложение). Пустой
	// список не пишет строк -> получатели видят все вложения (обратная совместимость).
	// Семантика аддитивная: повторная пересылка с другим набором ДОБАВЛЯет строки
	// (ON CONFLICT DO NOTHING), а не заменяет; сужение видимости здесь не предусмотрено.
	if len(req.AttachmentIDs) > 0 {
		recipients := make([]int, 0, len(addedResponsibleUsers)+len(addedViewers))
		for _, resp := range addedResponsibleUsers {
			recipients = append(recipients, resp.UserID)
		}
		recipients = append(recipients, addedViewers...)

		if len(recipients) > 0 {
			// Только вложения самой заявки - чужие ID отбрасываем, чтобы строка не ссылалась
			// на чужое вложение и фильтр чтения не прятал всё подряд.
			var validAttIDs []int
			tx.Raw("SELECT id FROM attachments WHERE application_id = ? AND id IN ?", applicationID, req.AttachmentIDs).Scan(&validAttIDs)

			for _, recipientID := range recipients {
				for _, attID := range validAttIDs {
					tx.Exec(`
						INSERT INTO forward_attachments (application_id, recipient_user_id, attachment_id, created_at, created_by)
						VALUES (?, ?, ?, ?, ?)
						ON CONFLICT (application_id, recipient_user_id, attachment_id) DO NOTHING
					`, applicationID, recipientID, attID, baseTime, user.ID)
				}
			}
		}
	}

	// Записываем историю назначений ответственных. Актор в user_id - НАЗНАЧЕННЫЙ
	// пользователь (квирк заявки), действующий лежит в metadata.forwarded_by.
	for _, resp := range addedResponsibleUsers {
		meta, _ := json.Marshal(map[string]interface{}{
			"required_approval": resp.RequiredApproval,
			"is_primary":        false,
			"forwarded_by":      currentUserName,
			"type":              "responsible",
		})
		respUserID := resp.UserID
		s.recorder.Log(ctx, tx, models.AuditEntityApplication, &applicationID, "assigned_responsible", &respUserID,
			applicationAuditDetails{Metadata: meta})
	}

	// Записываем историю назначений просматривающих (user_id - назначенный viewer).
	for _, viewerID := range addedViewers {
		meta, _ := json.Marshal(map[string]interface{}{
			"forwarded_by": currentUserName,
			"type":         "viewer",
		})
		vID := viewerID
		s.recorder.Log(ctx, tx, models.AuditEntityApplication, &applicationID, "assigned_viewer", &vID,
			applicationAuditDetails{Metadata: meta})
	}

	// Сводная запись о пересылке: всю заявку или конкретные вложения (#680).
	// Пишем один раз на действие и только если кому-то реально переслали. Имена
	// вложений кладём в metadata, текст собирает фронт (как у assigned_*).
	if len(addedResponsibleUsers) > 0 || len(addedViewers) > 0 {
		var attNames []string
		if len(req.AttachmentIDs) > 0 {
			tx.Raw(`
				SELECT COALESCE(attachment_display_name, attachment_name, '')
				FROM attachments
				WHERE application_id = ? AND id IN ?
				ORDER BY COALESCE(attachment_display_name, attachment_name, '')
			`, applicationID, req.AttachmentIDs).Scan(&attNames)
		}
		// Получатели этой пересылки (#967, ветка заявки): ответственные, затем
		// просматривающие - в том же порядке кладём в metadata.recipients, чтобы
		// GetForwardMessages показал "Кому переслано" без досбора из assigned_* записей.
		recipientIDs := make([]int, 0, len(addedResponsibleUsers)+len(addedViewers))
		seenRecipient := make(map[int]struct{}, cap(recipientIDs))
		addRecipient := func(id int) {
			if _, dup := seenRecipient[id]; dup {
				return
			}
			seenRecipient[id] = struct{}{}
			recipientIDs = append(recipientIDs, id)
		}
		for _, resp := range addedResponsibleUsers {
			addRecipient(resp.UserID)
		}
		for _, viewerID := range addedViewers {
			addRecipient(viewerID)
		}
		recipientNames := make([]string, 0, len(recipientIDs))
		if len(recipientIDs) > 0 {
			type recipientRow struct {
				ID         int
				LastName   *string
				FirstName  *string
				MiddleName *string
			}
			var recipientRows []recipientRow
			tx.Raw("SELECT id, last_name, first_name, middle_name FROM users WHERE id IN ?", recipientIDs).Scan(&recipientRows)
			nameByID := make(map[int]string, len(recipientRows))
			for _, r := range recipientRows {
				nameByID[r.ID] = formatFullName(r.LastName, r.FirstName, r.MiddleName)
			}
			for _, id := range recipientIDs {
				if name := nameByID[id]; name != "" {
					recipientNames = append(recipientNames, name)
				}
			}
		}
		meta, _ := json.Marshal(map[string]interface{}{
			"forwarded_by": currentUserName,
			"whole":        len(attNames) == 0,
			"attachments":  attNames,
			"recipients":   recipientNames,
		})
		// Сопроводительное сообщение (#967) кладём в comment той же сводной записи.
		// Пустое после trim -> comment не пишем; пересылка всё равно попадёт в ветку
		// заявки (message пустой), т.к. GetForwardMessages по comment больше не фильтрует.
		details := applicationAuditDetails{Metadata: meta}
		if msg := strings.TrimSpace(req.Message); msg != "" {
			details.Comment = &msg
		}
		s.recorder.Log(ctx, tx, models.AuditEntityApplication, &applicationID, models.AuditActionForwarded, &user.ID, details)
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

	confirmationChanged := (oldConfirmation == nil) != (newConfirmation == nil) ||
		(oldConfirmation != nil && newConfirmation != nil && *oldConfirmation != *newConfirmation)
	if confirmationChanged {
		s.recorder.Log(ctx, tx, models.AuditEntityApplication, &applicationID, "confirmation_change", &user.ID,
			applicationAuditDetails{OldValue: oldConfirmation, NewValue: newConfirmation})
		if err := s.bumpStatusUpdated(tx, applicationID, &user.ID); err != nil {
			tx.Rollback()
			return err
		}
	}

	if err := tx.Commit().Error; err != nil {
		slog.Error("не удалось зафиксировать транзакцию пересылки заявки", "application_id", applicationID, "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to commit transaction")
	}

	// Автоматически выдаём права tab.applications.view ответственным пользователям
	for _, resp := range addedResponsibleUsers {
		if err := s.permissionService.GrantPermission(ctx, resp.UserID, "tab.applications.view", "allow"); err != nil {
			slog.Warn("не удалось выдать разрешение tab.applications.view", "user_id", resp.UserID, "error", err)
		}
	}

	// Уведомления для ответственных и просматривающих о поступлении заявки на
	// согласование. Ошибки только логируем - не должны откатывать пересылку.
	if s.notificationService != nil {
		var appNumber string
		s.db.WithContext(ctx).Raw("SELECT application_number FROM applications WHERE id = ?", applicationID).Scan(&appNumber)
		appNumberStr := appNumber
		if appNumberStr == "" {
			appNumberStr = fmt.Sprintf("№ %d", applicationID)
		}

		for _, resp := range addedResponsibleUsers {
			data := map[string]any{
				"application_id":     applicationID,
				"application_number": appNumberStr,
				"forwarded_by":       currentUserName,
			}
			payload, _ := json.Marshal(data)
			payloadStr := string(payload)
			if err := s.notificationService.CreateForUser(ctx, resp.UserID, NotificationTypeApplicationApprovalRequired,
				"Заявка на согласование",
				fmt.Sprintf("Вам передана заявка %s на согласование.", appNumberStr),
				&payloadStr); err != nil {
				slog.Warn("не удалось создать уведомление о согласовании", "user_id", resp.UserID, "error", err)
			}
		}

		for _, viewerID := range addedViewers {
			data := map[string]any{
				"application_id":     applicationID,
				"application_number": appNumberStr,
				"forwarded_by":       currentUserName,
			}
			payload, _ := json.Marshal(data)
			payloadStr := string(payload)
			if err := s.notificationService.CreateForUser(ctx, viewerID, NotificationTypeApplicationForwarded,
				"Заявка передана для просмотра",
				fmt.Sprintf("Вам передана заявка %s для просмотра.", appNumberStr),
				&payloadStr); err != nil {
				slog.Warn("не удалось создать уведомление просмотра", "user_id", viewerID, "error", err)
			}
		}
	}

	slog.Info("заявка переслана", "application_id", applicationID, "user_id", user.ID,
		"responsible_count", len(addedResponsibleUsers), "viewer_count", len(addedViewers))
	s.notifyApplicationUpdated(ctx, applicationID, archiveDataChanged)
	// Пересылка может пересчитать confirmation до финального значения (#1349): если он
	// сменился в Согласовано/Не согласовано - уведомляем инициатора об исходе.
	if confirmationChanged {
		if outcome := confirmationOutcome(newConfirmation); outcome != "" {
			s.notifyInitiatorStatusChanged(ctx, applicationID, &user.ID, outcome, &statusChangeContext{
				ActorName: formatFullName(user.LastName, user.FirstName, user.MiddleName),
			})
		}
	}
	return nil
}

// ApproveApplicationByUser фиксирует согласование или отказ заявки пользователем.
func (s *applicationService) ApproveApplicationByUser(ctx context.Context, username string, applicationID int, req UserApprovalRequest) error {
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

	// Гейт обхода ЧС (#481): согласовать ('approved') нельзя, пока по всем помеченным
	// похожими на ЧС элементам не подтверждён пропуск (override). Отказ ('rejected') не
	// блокируем - отклонить подозрительную заявку можно сразу.
	if req.Status == "approved" {
		blocked, err := hasUnoverriddenBlacklistFlags(ctx, tx, applicationID)
		if err != nil {
			tx.Rollback()
			return err
		}
		if blocked {
			tx.Rollback()
			return echo.NewHTTPError(http.StatusConflict,
				"Заявка содержит элементы, похожие на чёрный список. Подтвердите пропуск каждого ('Всё равно пропустить') перед согласованием")
		}
	}

	// Сохраняем старый confirmation
	var oldConfirmation *string
	tx.Raw("SELECT confirmation FROM applications WHERE id = ?", applicationID).Scan(&oldConfirmation)

	nowUTC := time.Now().UTC()

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
	s.recorder.Log(ctx, tx, models.AuditEntityApplication, &applicationID, actionType, &user.ID,
		applicationAuditDetails{Comment: req.Comment, Metadata: meta})

	// Пересчитываем confirmation
	if err := s.updateConfirmationBasedOnApprovals(tx, applicationID); err != nil {
		tx.Rollback()
		return err
	}

	// Проверяем изменение confirmation
	var newConfirmation *string
	tx.Raw("SELECT confirmation FROM applications WHERE id = ?", applicationID).Scan(&newConfirmation)

	confirmationChanged := (oldConfirmation == nil) != (newConfirmation == nil) ||
		(oldConfirmation != nil && newConfirmation != nil && *oldConfirmation != *newConfirmation)
	if confirmationChanged {
		s.recorder.Log(ctx, tx, models.AuditEntityApplication, &applicationID, "confirmation_change", &user.ID,
			applicationAuditDetails{OldValue: oldConfirmation, NewValue: newConfirmation})
		if err := s.bumpStatusUpdated(tx, applicationID, &user.ID); err != nil {
			tx.Rollback()
			return err
		}
	}

	if err := tx.Commit().Error; err != nil {
		slog.Error("не удалось зафиксировать транзакцию одобрения заявки", "application_id", applicationID, "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to commit transaction")
	}

	// Заявка перешла в "Согласовано" -> её вложения появились в "Доступные мне"
	// охраны (#840 V3). Сигналим аудитории обновить список (event-then-fetch). Только
	// на переходе: повторное согласование не рождает нового доступного.
	becameApproved := newConfirmation != nil && *newConfirmation == models.ConfirmationApproved &&
		(oldConfirmation == nil || *oldConfirmation != models.ConfirmationApproved)
	if becameApproved {
		s.availableProducer.NotifyAvailableChanged(ctx)
	}

	slog.Info("заявка одобрена/отклонена", "application_id", applicationID, "user_id", user.ID, "status", req.Status)
	s.notifyApplicationUpdated(ctx, applicationID, archiveDataChanged)
	// Инициатору - уведомление об исходе согласования, если confirmation сменился в
	// финальное значение Согласовано/Не согласовано (#1349).
	if confirmationChanged {
		if outcome := confirmationOutcome(newConfirmation); outcome != "" {
			s.notifyInitiatorStatusChanged(ctx, applicationID, &user.ID, outcome, &statusChangeContext{
				ActorName: formatFullName(user.LastName, user.FirstName, user.MiddleName),
				Comment:   optionalString(req.Comment),
			})
		}
	}
	return nil
}

// CheckApprovalStatus возвращает текущий статус согласования заявки.
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

// RevokeApproval отзывает ранее данное согласование заявки.
func (s *applicationService) RevokeApproval(ctx context.Context, username string, applicationID int, req RevokeApprovalRequest) (*RevokeApprovalResponse, error) {
	user, err := s.getUserByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	if err := s.checkNotWithdrawn(ctx, applicationID); err != nil {
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

	tx.Exec(`
		UPDATE application_responsible_users
		SET approval_status = 'pending', approval_comment = NULL, approval_datetime = NULL
		WHERE application_id = ? AND user_id = ?
	`, applicationID, user.ID)

	s.recorder.Log(ctx, tx, models.AuditEntityApplication, &applicationID, "revoke_approval", &user.ID,
		applicationAuditDetails{Comment: req.Comment})

	if err := s.updateConfirmationBasedOnApprovals(tx, applicationID); err != nil {
		tx.Rollback()
		return nil, err
	}

	var newConfirmation *string
	tx.Raw("SELECT confirmation FROM applications WHERE id = ?", applicationID).Scan(&newConfirmation)

	if (oldConfirmation == nil) != (newConfirmation == nil) || (oldConfirmation != nil && newConfirmation != nil && *oldConfirmation != *newConfirmation) {
		s.recorder.Log(ctx, tx, models.AuditEntityApplication, &applicationID, "confirmation_change", &user.ID,
			applicationAuditDetails{OldValue: oldConfirmation, NewValue: newConfirmation})
		if err := s.bumpStatusUpdated(tx, applicationID, &user.ID); err != nil {
			tx.Rollback()
			return nil, err
		}
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

	s.notifyApplicationUpdated(ctx, applicationID, archiveDataChanged)

	return &RevokeApprovalResponse{
		Success:      true,
		Message:      "Approval revoked successfully",
		Confirmation: updatedApp.Confirmation,
		Status:       updatedApp.Status,
	}, nil
}
