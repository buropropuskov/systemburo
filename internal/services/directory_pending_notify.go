package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
)

// Уведомления вокруг записей справочника «на проверке» (#1437).
//
// Подача с незнакомым наименованием заводит организацию или компанию со статусом
// pending (application_org_resolve.go). Разобрать её может только носитель права
// application.organization.moderate, а плашка разбора живёт в детали заявки - поэтому
// о новой записи зовём тех, у кого есть И право, И сама заявка.
//
// Инициатору сообщаем только когда разбор поменял его наименование (исправление или
// привязка к существующей записи): при подтверждении для него ничего не изменилось -
// бейдж «на проверке» просто гаснет, а лишнее уведомление стало бы шумом.
//
// Всё здесь best-effort: сбой уведомления не должен рушить ни подачу заявки, ни разбор.

// Типы уведомлений -- NotificationTypeDirectoryPending (принимающему: подача завела
// запись справочника) и NotificationTypeDirectoryResolved (инициатору: его наименование
// разобрали), обе -- каталог, notification_catalog.go.

// pendingDirectoryNotice - запись, заведённая подачей и ждущая разбора. label -
// «Организация» либо «Компания»: оба слова женского рода, поэтому текст уведомления
// собирается из label без отдельных форм склонения.
type pendingDirectoryNotice struct {
	label string
	name  string
}

// notifyDirectoryPending зовёт разобрать записи, заведённые этой подачей. Одно
// уведомление на запись: заявка может привести и новую организацию, и новую компанию,
// а разбираются они по отдельности.
func (s *applicationService) notifyDirectoryPending(ctx context.Context, appID int, appNumber string, senderID int, pending []pendingDirectoryNotice) {
	if len(pending) == 0 || s.notificationService == nil {
		return
	}
	if s.permissionResolver == nil {
		// Без резолвера аудиторию не собрать, и запись осталась бы незамеченной -
		// это конфигурация, а не штатная ветка, поэтому говорим об этом в лог.
		slog.Warn("некому сообщить о записи справочника на проверке: резолвер прав не подключен",
			"application_id", appID)
		return
	}

	audience := s.directoryModerators(ctx, appID, senderID)
	if len(audience) == 0 {
		return
	}

	payload, _ := json.Marshal(map[string]any{
		"application_id":     appID,
		"application_number": appNumber,
	})
	payloadStr := string(payload)

	for _, notice := range pending {
		title := "Новая " + strings.ToLower(notice.label) + " на проверке"
		body := fmt.Sprintf("В заявке %s указана «%s». Проверьте наименование и разберите запись.", appNumber, notice.name)
		for _, userID := range audience {
			if err := s.notificationService.CreateForUser(ctx, userID, NotificationTypeDirectoryPending, title, body, &payloadStr); err != nil {
				slog.Warn("не удалось уведомить о записи справочника на проверке",
					"user_id", userID, "application_id", appID, "error", err)
			}
		}
	}
}

// directoryModerators - кому адресован призыв разобрать запись: те, у кого заявка видна
// (centerAudience - зеркало applyApplicationAccessFilter), И есть право разбора. Право
// спрашиваем у резолвера - того же источника, что стоит за middleware эндпоинтов разбора
// и за гейтом плашки во фронте, поэтому «пришло уведомление, а действий нет» не бывает.
// Автора подачи исключаем: наименование ввёл он сам.
func (s *applicationService) directoryModerators(ctx context.Context, appID, senderID int) []int {
	audience := s.centerAudience(ctx, appID, senderID)
	moderators := make([]int, 0, len(audience))
	for _, userID := range audience {
		if userID == senderID {
			continue
		}
		set, err := s.permissionResolver.Resolve(ctx, userID)
		if err != nil {
			// best-effort: сбой резолва одного пользователя сужает аудиторию, но не
			// должен отменять уведомление остальным.
			slog.Warn("аудитория разбора справочника: резолв прав не удался", "user_id", userID, "error", err)
			continue
		}
		if set.Has(KeyApplicationOrganizationModerate) {
			moderators = append(moderators, userID)
		}
	}
	return moderators
}

// notifyDirectoryResolved сообщает инициатору наименования, чем кончился разбор.
// authorID - created_by_user_id разобранной записи: у записей, заведённых админом руками,
// его нет, и сообщать некому. Себе уведомление не шлём: принимающий, разобравший
// собственную подачу, и так знает результат.
func notifyDirectoryResolved(ctx context.Context, notifier NotificationService, table string, authorID *int, actorID int, title, body string) {
	if notifier == nil || authorID == nil || *authorID == actorID {
		return
	}
	if err := notifier.CreateForUser(ctx, *authorID, NotificationTypeDirectoryResolved, title, body, nil); err != nil {
		slog.Warn("не удалось уведомить инициатора о разборе записи справочника",
			"table", table, "user_id", *authorID, "error", err)
	}
}
