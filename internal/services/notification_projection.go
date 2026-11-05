package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
	"systemburo/internal/models"
)

// Keep notification snapshots out of every read/ownership query. ProjectionRef
// is a separate, server-owned column; Data can contain arbitrary manual input.
const notificationMetadataColumns = "id,user_id,type,group_key,is_read,count,last_event_at,created_at,projection_ref"
const genericNotificationText = "Есть обновление"

func (s *notificationService) notificationMetadataQuery(ctx context.Context, userID int, filter string) *gorm.DB {
	return s.notificationListQuery(ctx, userID, filter).Select(notificationMetadataColumns)
}

func neutralNotification(stored models.Notification) models.Notification {
	// Build an explicit response allowlist; future persistence fields must not
	// silently enter the DTO through a copy of the ORM row.
	n := models.Notification{ID: stored.ID, UserID: stored.UserID, Type: stored.Type,
		IsRead: stored.IsRead, Count: stored.Count,
		LastEventAt: stored.LastEventAt, CreatedAt: stored.CreatedAt}
	n.Title = notificationString(genericNotificationText)
	n.Message = notificationString(genericNotificationText)
	n.Data = nil
	n.ProjectionRef = nil

	if n.Type != nil {
		if _, ok := NotificationTypeMeta(*n.Type); !ok {
			n.Type = nil
		}
	}
	return n
}

func notificationString(value string) *string { return &value }

type notificationProjectionPolicy struct {
	viewerID int
	viewer   struct {
		ID           int
		IsActive     bool
		IsBanned     bool
		IsAdmin      bool
		IsSuperAdmin bool
	}
	permissions          PermissionSet
	consentMaskingActive bool
	displayMaskBypass    bool
	names                map[int]string
	batch                *notificationProjectionBatch
}

func (s *notificationService) notificationPolicy(ctx context.Context, viewerID int) (*notificationProjectionPolicy, error) {
	if identity, ok := SessionIdentityFromContext(ctx); ok && identity.EffectiveUserID != viewerID {
		return nil, echo.NewHTTPError(http.StatusForbidden, "Недостаточно прав")
	}
	p := &notificationProjectionPolicy{viewerID: viewerID, names: map[int]string{}, batch: newNotificationProjectionBatch()}
	err := s.db.WithContext(ctx).Table("users").Select("id,is_active,is_banned,is_admin,is_super_admin").Where("id = ?", viewerID).Take(&p.viewer).Error
	if err != nil {
		return nil, nameMaskUnavailable(err)
	}
	if !p.viewer.IsActive {
		return nil, echo.NewHTTPError(http.StatusForbidden, "Учётная запись недоступна")
	}
	resolver := s.permissionResolver
	if resolver == nil {
		resolver = NewPermissionResolver(s.db)
	}
	p.permissions, err = resolver.ResolveFresh(ctx, viewerID)
	if err != nil {
		return nil, nameMaskUnavailable(err)
	}
	// Read only policy metadata here. Display masks may themselves contain a
	// personal name, so actor consent must be checked before selecting any mask.
	p.consentMaskingActive, err = pdConsentMaskingActive(ctx, s.db)
	if err != nil {
		return nil, err
	}
	if identity, verified := SessionIdentityFromContext(ctx); verified && identity.EffectiveUserID == viewerID {
		p.displayMaskBypass = !p.viewer.IsBanned && (p.viewer.IsAdmin || p.viewer.IsSuperAdmin)
	}
	return p, nil
}

// The legacy endpoint still returns every row in its original order. Only
// hydration is chunked; unknown rows never trigger policy/domain reads.
func (s *notificationService) projectNotifications(ctx context.Context, viewerID int, rows []models.Notification) ([]models.Notification, error) {
	out := make([]models.Notification, len(rows))
	entries := make([]notificationProjectionEntry, 0, len(rows))
	for i, row := range rows {
		out[i] = neutralNotification(row)
		if ref, valid := decodeNotificationReference(row.ProjectionRef, row.Type); valid {
			entries = append(entries, notificationProjectionEntry{index: i, ref: ref, groupKey: row.GroupKey})
		}
	}
	if len(entries) == 0 {
		return out, nil
	}
	policy, err := s.notificationPolicy(ctx, viewerID)
	if err != nil {
		return nil, err
	}
	for start := 0; start < len(entries); start += notificationProjectionBatchSize {
		end := min(start+notificationProjectionBatchSize, len(entries))
		chunk := entries[start:end]
		if err := s.preloadNotificationProjection(ctx, policy, chunk); err != nil {
			return nil, err
		}
		for _, entry := range chunk {
			projected, err := s.projectReferencedNotification(ctx, policy, out[entry.index], entry.ref, entry.groupKey)
			if err != nil {
				return nil, err
			}
			out[entry.index] = projected
		}
	}
	return out, nil
}

func notificationTrashKind(kind string) string {
	switch kind {
	case "Машина":
		return "cars"
	case "Сотрудник":
		return "employees"
	default:
		return ""
	}
}

func (s *notificationService) projectReferencedNotification(ctx context.Context, p *notificationProjectionPolicy, n models.Notification, ref notificationReference, storedGroupKey *string) (models.Notification, error) {
	meta, _ := NotificationTypeMeta(ref.Type)
	if !p.batch.referenceReadable(p, ref) {
		return n, nil
	}
	data := map[string]any{}
	title, message := meta.Label, ""
	number := ""
	if ref.ApplicationID > 0 {
		number = p.batch.applications[ref.ApplicationID].Number
		data["application_id"], data["application_number"] = ref.ApplicationID, number
	}
	if ref.SupplementID > 0 {
		supplement := p.batch.supplements[ref.SupplementID]
		data["supplement_id"], data["supplement_number"] = supplement.ID, supplement.Number
	}
	if ref.QuestionID > 0 {
		data["question_id"] = ref.QuestionID
	}
	actor := p.names[ref.ActorID]
	if ref.ActorID > 0 && actor == "" {
		return n, nil
	}
	switch ref.Type {
	case NotificationTypeApplicationCreated:
		message = fmt.Sprintf("Ваша заявка %s отправлена и ожидает согласования.", number)
		if actor != "" {
			data["sender_name"] = actor
		}
	case NotificationTypeApplicationApprovalRequired:
		message = fmt.Sprintf("Поступила новая заявка %s на согласование.", number)
		if ref.Variant == "forward" {
			title, message = "Заявка на согласование", fmt.Sprintf("Вам передана заявка %s на согласование.", number)
			if actor != "" {
				data["forwarded_by"] = actor
			}
		} else if ref.Variant == "supplement" && ref.SupplementID > 0 {
			message = fmt.Sprintf("В заявку %s добавлены новые строки - требуется согласование.", number)
			if ref.Facts.InWork {
				message = fmt.Sprintf("По заявке %s подано дополнение №%v - требуется согласование.", number, data["supplement_number"])
			}
		}
	case NotificationTypeApplicationPendingAcceptance:
		title, message = "Новая заявка", fmt.Sprintf("Поступила новая заявка %s.", number)
		if actor != "" {
			data["sender_name"] = actor
			message += "\nОтправитель: " + actor
		}
	case NotificationTypeApplicationForwarded:
		message = fmt.Sprintf("Вам передана заявка %s для просмотра.", number)
		if actor != "" {
			data["forwarded_by"] = actor
		}
	case NotificationTypeApplicationQuestion:
		if ref.QuestionID == 0 || actor == "" {
			return n, nil
		}
		message = fmt.Sprintf("%s начал(-а) обсуждение по заявке %s.", actor, number)
	case NotificationTypeApplicationAnswer:
		if ref.QuestionID == 0 || ref.AnswerID == 0 || actor == "" {
			return n, nil
		}
		message = fmt.Sprintf("%s ответил(-а) в обсуждении по заявке %s.", actor, number)
	case NotificationTypeApplicationStatusChanged:
		switch ref.Facts.Outcome {
		case statusOutcomeAccepted:
			title, message = "Заявка принята в работу", fmt.Sprintf("Ваша заявка %s принята в работу.", number)
		case statusOutcomeRejected:
			title, message = "Заявка отклонена", fmt.Sprintf("Ваша заявка %s отклонена.", number)
		case statusOutcomeApproved:
			title, message = "Заявка согласована", fmt.Sprintf("Ваша заявка %s согласована.", number)
		case statusOutcomeNotApproved:
			title, message = "Заявка не согласована", fmt.Sprintf("Ваша заявка %s не согласована.", number)
		case statusOutcomeCompleted:
			title, message = "Заявка завершена", fmt.Sprintf("Срок действия вашей заявки %s истёк, она завершена.", number)
		default:
			return n, nil
		}
		if actor != "" {
			data["actor_name"] = actor
		}
	case NotificationTypeApplicationWithdrawn:
		message = fmt.Sprintf("Заявку %s отозвали - рассматривать её больше не нужно.", number)
		if actor != "" {
			message = fmt.Sprintf("Заявку %s отозвал(а) %s - рассматривать её больше не нужно.", number, actor)
		}
	case NotificationTypeApplicationSupplementReady:
		if ref.SupplementID == 0 {
			return n, nil
		}
		message = fmt.Sprintf("Дополнение №%v к заявке %s согласовано - требуется принятие.", data["supplement_number"], number)
	case NotificationTypeApplicationSupplementDecided:
		if ref.SupplementID == 0 {
			return n, nil
		}
		switch ref.Facts.Outcome {
		case models.SupplementAccepted:
			title, message = "Дополнение принято", fmt.Sprintf("Дополнение №%v к заявке %s принято - добавленные строки на проходной.", data["supplement_number"], number)
		case models.SupplementRefused:
			title, message = "Дополнение отклонено", fmt.Sprintf("Дополнение №%v к заявке %s отклонено принимающим.", data["supplement_number"], number)
		default:
			return n, nil
		}
		data["status"] = ref.Facts.Outcome
	case NotificationTypeApplicationDatesChanged:
		message = fmt.Sprintf("Изменён срок заявки %s.", number)
		if ref.Variant == "entity" {
			title, message = "Изменён индивидуальный срок", fmt.Sprintf("Изменён индивидуальный срок по заявке %s.", number)
		}
		if ref.Variant == "attachments" {
			title, message = "Изменён срок вложений", fmt.Sprintf("Изменён срок выбранных вложений заявки %s.", number)
		}
		if ref.Facts.OldPeriod != "" && ref.Facts.NewPeriod != "" {
			message = fmt.Sprintf("Принимающий изменил срок заявки %s: было %s, стало %s.", number, ref.Facts.OldPeriod, ref.Facts.NewPeriod)
			data["old_period"], data["new_period"], data["approvals_reset"] = ref.Facts.OldPeriod, ref.Facts.NewPeriod, ref.Facts.ApprovalsReset
		}
	case NotificationTypeApprovalReminder:
		if ref.ApplicationID == 0 || ref.Facts.WaitingDays < 0 {
			return n, nil
		}
		message = fmt.Sprintf("Заявка %s ждёт вашего решения уже %d дн. Пожалуйста, рассмотрите её.", number, ref.Facts.WaitingDays)
		data["waiting_days"] = ref.Facts.WaitingDays
	case NotificationTypeApplicationExpiring:
		if ref.Facts.Until.IsZero() || ref.Facts.DaysLeft < 0 {
			return n, nil
		}
		message = expiryNotifyBody(number, ref.Facts.Until, ref.Facts.DaysLeft)
	case NotificationTypeDirectoryPending:
		message = fmt.Sprintf("В заявке %s создана запись справочника. Проверьте и разберите запись.", number)
	case NotificationTypeDirectoryResolved:
		message = "Указанная вами запись справочника разобрана."
	case NotificationTypeNewsPublished, NotificationTypeDocumentPublished:
		message = meta.Description
	case NotificationTypeFeedbackCreated, NotificationTypeFeedbackAnswered:
		message = meta.Description
	case NotificationTypeTrashRestored:
		message = "Запись из вашей заявки восстановили из корзины."
	case NotificationTypePasswordChanged:
		message = "Ваш пароль в системе был изменён."
		if ref.Facts.SelfChange {
			message = "Ваш пароль был успешно изменён."
		}
		if !ref.Facts.At.IsZero() {
			data["changed_at"] = ref.Facts.At
		}
	case NotificationTypeUserBanned:
		message = "Ваша учётная запись заблокирована."
	case NotificationTypeUserUnbanned:
		message = "Доступ к вашей учётной записи восстановлен."
	case NotificationTypeLoginBlocked:
		if ref.Facts.Attempts < 0 || ref.Facts.Until.IsZero() {
			return n, nil
		}
		message = fmt.Sprintf("Слишком много неудачных попыток входа (%d) в вашу учётную запись. Вход заблокирован до %s (МСК).", ref.Facts.Attempts, ref.Facts.Until.In(moscowWorkModeLoc).Format("02.01.2006 15:04"))
		data["attempts"], data["locked_until"] = ref.Facts.Attempts, ref.Facts.Until
	case NotificationTypePasswordRotated:
		title, message = "Пароль изменён", "Система сменила ваш пароль по правилам безопасности и отправила новый на вашу почту."
	case NotificationTypePasswordExpiring:
		if ref.Facts.Until.IsZero() {
			return n, nil
		}
		message = fmt.Sprintf("Срок действия пароля истекает %s. Смените его заранее, иначе при следующем входе система попросит сделать это сразу.", ref.Facts.Until.Format("02.01.2006"))
	case NotificationTypePasswordRotationReport:
		switch ref.Variant {
		case "no_mail":
			title, message = "Обновление паролей не выполнено", "Почта не настроена, поэтому прогон не начинался. Пароли не менялись."
		case "marked":
			title, message = "Плановая проверка сроков паролей выполнена", fmt.Sprintf("Помечено истёкшими паролей: %d. При следующем входе система попросит этих работников задать новый пароль.", ref.Facts.Marked)
			if ref.Facts.Failed > 0 {
				message += fmt.Sprintf(" Не удалось пометить: %d.", ref.Facts.Failed)
			}
		case "rotated":
			title, message = "Обновление паролей выполнено", fmt.Sprintf("Сменено паролей: %d.", ref.Facts.Changed)
			if ref.Facts.SkippedNoMail > 0 {
				message += fmt.Sprintf(" Без адреса почты пропущено: %d.", ref.Facts.SkippedNoMail)
			}
			if ref.Facts.Failed > 0 {
				message += fmt.Sprintf(" Не удалось сменить: %d.", ref.Facts.Failed)
			}
		default:
			return n, nil
		}
	case NotificationTypeMaintenanceScheduled:
		if ref.Facts.At.IsZero() || !ref.Facts.Until.After(ref.Facts.At) {
			return n, nil
		}
		message = fmt.Sprintf("Технические работы начнутся %s (МСК) и продлятся %s.", ref.Facts.At.In(AnalyticsLocation()).Format("02.01.2006 15:04"), formatMaintenanceDuration(ref.Facts.Until.Sub(ref.Facts.At)))
	case NotificationTypeArchiveQuotaWarning:
		message = fmt.Sprintf("Раздел архива заполнен на %.0f%% (порог %d%%), свободно %s.", ref.Facts.UsedPercent, ref.Facts.WarnPercent, formatBytesRu(int64(ref.Facts.FreeBytes)))
	case NotificationTypeErrorSpike:
		if ref.Facts.Requests <= 0 || ref.Facts.Errors < 0 || ref.Facts.Errors > ref.Facts.Requests || ref.Facts.WindowMinutes <= 0 {
			return n, nil
		}
		message = fmt.Sprintf("За последние %d минут доля ответов с кодом 5xx - %.1f%% (%d из %d). Подробности в разделе «Мониторинг запросов».", ref.Facts.WindowMinutes, 100*float64(ref.Facts.Errors)/float64(ref.Facts.Requests), ref.Facts.Errors, ref.Facts.Requests)
	default:
		return n, nil
	}
	n.Title, n.Message = notificationString(title), notificationString(message)
	if len(data) > 0 {
		encoded, err := json.Marshal(data)
		if err != nil {
			return neutralNotification(n), nil
		}
		n.Data = notificationString(string(encoded))
	}
	// A group key identifies a domain object. Disclose it only after successful
	// object/policy projection, and only when its stored value agrees with both
	// the catalogue grouping policy and this event's trusted reference.
	n.GroupKey = projectedNotificationGroupKey(ref, storedGroupKey)
	return n, nil
}

func projectedNotificationGroupKey(ref notificationReference, stored *string) *string {
	meta, known := NotificationTypeMeta(ref.Type)
	if stored == nil || !known || !meta.Aggregatable {
		return nil
	}
	expected := ""
	if ref.Type == NotificationTypeApplicationAnswer && ref.QuestionID > 0 {
		expected = fmt.Sprintf("question:%d", ref.QuestionID)
	} else if ref.ApplicationID > 0 {
		expected = fmt.Sprintf("app:%d", ref.ApplicationID)
	}
	if expected == "" || *stored != expected {
		return nil
	}
	return notificationString(expected)
}
