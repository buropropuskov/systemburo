package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"systemburo/internal/models"

	"gorm.io/gorm"
)

// notifyFirstPassage уведомляет инициатора заявки о первом проходе по ней (#1748,
// S4): вход впервые отмечен у одной из её машин или сотрудников. Общая точка для
// carService (UpdateCarTerritoryStatus) и employeeService (UpdateEmployeeTerritoryStatus)
// - у обоих один и тот же путь "attachment -> application", отдельного сервиса ради
// одной функции заводить незачем.
//
// attachmentID - вложение машины/сотрудника, у которого только что отметили вход.
// "Первый" определяется по факту (entry-записей audit_log по машинам/сотрудникам
// заявки до этой не было), а не по календарным суткам - иначе второй въезд машины
// на следующий день после первого выглядел бы "первым" снова.
//
// Тип уведомления схлопывающийся (NotificationTypeApplicationPassageFirst,
// Aggregatable=true в каталоге): application_id в data достаточно, повторные "первые"
// из-за гонки параллельных отметок соберёт в одну запись общий механизм агрегации
// (#1748, S2) по group_key заявки - отдельная защита от дублей здесь не нужна.
//
// Best-effort: ошибки логируются, вызывающая операция (сама отметка прохода) уже
// закоммичена и не откатывается.
func notifyFirstPassage(ctx context.Context, db *gorm.DB, notificationService NotificationService, attachmentID int) {
	if notificationService == nil {
		return
	}

	var applicationID *int
	if err := db.WithContext(ctx).
		Raw("SELECT application_id FROM attachments WHERE id = ?", attachmentID).
		Scan(&applicationID).Error; err != nil {
		slog.Warn("не удалось получить заявку вложения для уведомления о первом проходе", "attachment_id", attachmentID, "error", err)
		return
	}
	// Вложение без заявки (ручное добавление в таблицу проходной, #1049) - уведомлять некого.
	if applicationID == nil {
		return
	}
	appID := *applicationID

	var entryCount int64
	if err := db.WithContext(ctx).Raw(`
		SELECT COUNT(*) FROM audit_log al
		WHERE al.action = 'entry' AND (
			(al.entity_type = ? AND al.entity_id IN (
				SELECT c.id FROM cars c JOIN attachments a ON a.id = c.attachment_id WHERE a.application_id = ?
			))
			OR (al.entity_type = ? AND al.entity_id IN (
				SELECT e.id FROM employees e JOIN attachments a ON a.id = e.attachment_id WHERE a.application_id = ?
			))
		)
	`, models.AuditEntityCar, appID, models.AuditEntityEmployee, appID).Scan(&entryCount).Error; err != nil {
		slog.Warn("не удалось проверить первый проход по заявке", "application_id", appID, "error", err)
		return
	}
	// 1 - только что записанный вход и есть первый. 0 быть не должно (мы вызываемся
	// после commit самой entry-записи), >1 - проход по заявке уже был.
	if entryCount != 1 {
		return
	}

	var app struct {
		SenderUserID      *int
		ApplicationNumber string
	}
	if err := db.WithContext(ctx).
		Raw("SELECT sender_user_id, application_number FROM applications WHERE id = ?", appID).
		Scan(&app).Error; err != nil {
		slog.Warn("не удалось получить отправителя для уведомления о первом проходе", "application_id", appID, "error", err)
		return
	}
	if app.SenderUserID == nil {
		return
	}

	number := app.ApplicationNumber
	if number == "" {
		number = fmt.Sprintf("№ %d", appID)
	}

	data := map[string]any{"application_id": appID, "application_number": number}
	payload, err := json.Marshal(data)
	if err != nil {
		slog.Warn("не удалось сериализовать данные уведомления о первом проходе", "application_id", appID, "error", err)
		return
	}
	payloadStr := string(payload)

	body := fmt.Sprintf("По заявке %s впервые прошли на территорию.", number)
	if err := notificationService.CreateForUser(ctx, *app.SenderUserID, NotificationTypeApplicationPassageFirst,
		"Первый проход по заявке", body, &payloadStr); err != nil {
		slog.Warn("не удалось создать уведомление о первом проходе", "application_id", appID, "error", err)
	}
}
