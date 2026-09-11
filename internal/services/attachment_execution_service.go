package services

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// attachmentExecutionMarkWindow - на сколько отметка "вложение исполнено" (#2446)
// блокирует повторное нажатие. Константа, не env: новый параметр тянет за собой
// замок TestEnvWiring_AllConfigVarsReachContainer и правку docker-compose/init-env.sh
// (off-limits), а настраивать окно никто не просил - см. тот же довод у
// passageRevertWindow.
const attachmentExecutionMarkWindow = 5 * time.Minute

// attachmentExecutionLockNamespace - первый аргумент pg_advisory_xact_lock (#2446):
// произвольное число, отделяющее локи этого механизма от прочих advisory-локов
// проекта (второй аргумент - id вложения). Взят по номеру issue, как метка.
const attachmentExecutionLockNamespace = 2446

// lastAttachmentExecutionMark возвращает момент последней отметки "исполнено" для
// вложения, ЕСЛИ она ещё в пределах attachmentExecutionMarkWindow от now, иначе nil.
// Отметки старше окна не поднимаются намеренно: и MarkAttachmentExecuted (решает,
// блокировать ли повтор), и GetAttachmentExecutionMark (что показать в детали) хотят
// знать одно и то же - "действует ли сейчас чья-то недавняя отметка".
func lastAttachmentExecutionMark(ctx context.Context, tx *gorm.DB, attachmentID int, now time.Time) (*time.Time, error) {
	var row struct{ CreatedAt time.Time }
	err := tx.WithContext(ctx).
		Table("audit_log").
		Select("created_at").
		Where("entity_type = ? AND entity_id = ? AND action = ?",
			models.AuditEntityAttachment, attachmentID, models.AuditActionAttachmentExecuted).
		Where("created_at > ?", now.Add(-attachmentExecutionMarkWindow)).
		Order("created_at DESC").
		Limit(1).
		Scan(&row).Error
	if err != nil {
		return nil, fmt.Errorf("failed to load last execution mark: %w", err)
	}
	if row.CreatedAt.IsZero() {
		return nil, nil
	}
	return &row.CreatedAt, nil
}

// GetAttachmentExecutionMark отдаёт момент, до которого действует последняя отметка
// "исполнено" по вложению (#2446), либо nil - отмечать можно прямо сейчас. Для
// детального эндпоинта "Доступные мне"; вызывать после CanSecurityViewAttachment,
// проверка доступа здесь не делается.
func (s *applicationService) GetAttachmentExecutionMark(ctx context.Context, attachmentID int) (*time.Time, error) {
	last, err := lastAttachmentExecutionMark(ctx, s.db, attachmentID, time.Now())
	if err != nil {
		return nil, err
	}
	if last == nil {
		return nil, nil
	}
	until := last.Add(attachmentExecutionMarkWindow)
	return &until, nil
}

// MarkAttachmentExecuted отмечает вложение исполненным сегодня (#2446): охранник
// открыл заявку в "Доступные мне" и подтвердил, что по ней приехали/пришли. Отметка
// не превращается в постоянное состояние сущности - только запись в audit_log,
// поэтому за день по одному вложению их может накопиться несколько (это и есть цель:
// несколько заездов в день). Повтор в пределах attachmentExecutionMarkWindow
// отклоняется (409) - и текущему нажатию, и параллельному: advisory-лок на id
// вложения сериализует проверку "недавней отметки нет" с самой записью, потому что
// миграции трогать нельзя (off-limits) и уникальный индекс под гонку завести негде.
func (s *applicationService) MarkAttachmentExecuted(ctx context.Context, actorUserID, attachmentID int) (time.Time, error) {
	if actorUserID == 0 {
		return time.Time{}, echo.NewHTTPError(http.StatusUnauthorized, "Требуется авторизация")
	}
	now := time.Now()
	var until time.Time
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(?, ?)",
			attachmentExecutionLockNamespace, attachmentID).Error; err != nil {
			return fmt.Errorf("failed to acquire execution mark lock: %w", err)
		}
		last, err := lastAttachmentExecutionMark(ctx, tx, attachmentID, now)
		if err != nil {
			return err
		}
		if last != nil {
			until = last.Add(attachmentExecutionMarkWindow)
			return echo.NewHTTPError(http.StatusConflict,
				"Уже отмечено недавно, повторить можно позже")
		}
		if err := s.recorder.Record(ctx, tx, models.AuditEntityAttachment, &attachmentID,
			models.AuditActionAttachmentExecuted, &actorUserID, nil); err != nil {
			return fmt.Errorf("failed to record execution mark: %w", err)
		}
		until = now.Add(attachmentExecutionMarkWindow)
		return nil
	})
	if err != nil {
		return time.Time{}, err
	}
	slog.Info("вложение отмечено исполненным", "attachment_id", attachmentID, "actor_user_id", actorUserID)
	return until, nil
}

// attachmentExecutionRecentLimit - сколько последних отметок сегодняшнего дня
// показывать в детали вложения. У вложений (в отличие от car/employee) нет своей
// модалки истории, поэтому предел держим с запасом относительно реального дня
// (несколько заездов на разгрузку/несколько машин), а не впритык - но не безлимитным
// списком: attachmentExecutionMarkWindow (5 минут) сам ограничивает предельно
// возможное число отметок за сутки, но одну-две сотни строк в карточке никто не
// станет читать.
const attachmentExecutionRecentLimit = 10

// ExecutionMarkEntry - одна отметка "вложение исполнено" (#2446 доп.): когда и кем.
// ActorName - фамилия с инициалами (format_short_name); nil у пользователя, которого
// с тех пор удалили - LEFT JOIN, отметка не должна пропадать вместе с автором.
type ExecutionMarkEntry struct {
	CreatedAt time.Time `json:"created_at"`
	ActorName *string   `json:"actor_name"`
}

// ExecutionMarksSummary - сводка отметок вложения за СЕГОДНЯШНИЕ московские сутки
// (#2446 доп.): TodayCount - точное число за день, Recent - те же отметки новыми
// сверху, обрезанные attachmentExecutionRecentLimit. Recent - подмножество дня, а
// не произвольная лента: иначе строка "сегодня отмечено N раз" и список под ней
// разошлись бы по смыслу (список нёс бы вчерашние отметки, которых в счёте нет).
type ExecutionMarksSummary struct {
	TodayCount int                  `json:"today_count"`
	Recent     []ExecutionMarkEntry `json:"recent"`
}

// GetAttachmentExecutionMarksSummary возвращает сводку отметок "исполнено" вложения
// за сегодня по московскому времени (#2446 доп.): AnalyticsLocation() - та же зона,
// что аналитика и суточный отчёт используют для границы дня, отдельный сдвиг здесь
// не изобретаю. TodayCount и Recent считаются ОДНИМ запросом (не COUNT + отдельный
// SELECT LIMIT): иначе отметка, пришедшая между двумя запросами, дала бы список и
// счётчик, не согласованные друг с другом. Для детального эндпоинта, вызывать после
// CanSecurityViewAttachment.
func (s *applicationService) GetAttachmentExecutionMarksSummary(ctx context.Context, attachmentID int) (*ExecutionMarksSummary, error) {
	now := time.Now().In(AnalyticsLocation())
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, AnalyticsLocation())

	entries := make([]ExecutionMarkEntry, 0, attachmentExecutionRecentLimit)
	err := s.db.WithContext(ctx).
		Table("audit_log a").
		Select("a.created_at, format_short_name(u.last_name, u.first_name, u.middle_name) AS actor_name").
		Joins("LEFT JOIN users u ON u.id = a.actor_user_id").
		Where("a.entity_type = ? AND a.entity_id = ? AND a.action = ? AND a.created_at >= ?",
			models.AuditEntityAttachment, attachmentID, models.AuditActionAttachmentExecuted, dayStart).
		Order("a.created_at DESC").
		Scan(&entries).Error
	if err != nil {
		return nil, fmt.Errorf("failed to load execution marks summary: %w", err)
	}

	summary := &ExecutionMarksSummary{TodayCount: len(entries), Recent: entries}
	if len(summary.Recent) > attachmentExecutionRecentLimit {
		summary.Recent = summary.Recent[:attachmentExecutionRecentLimit]
	}
	return summary, nil
}
