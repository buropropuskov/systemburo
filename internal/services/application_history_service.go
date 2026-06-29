package services

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
)

// applicationAuditDetails - форма details jsonb для записей application в audit_log
// (#870, срез 1.14). Ключи ОБЯЗАНЫ совпадать с тем, что извлекает GetApplicationHistory
// (action_status/old_value/new_value/comment/metadata), иначе чтение потеряет поля.
// nil-указатель -> ключ опущен -> details->>'key' = NULL (как незаполненная колонка
// application_history). metadata кладётся вложенным jsonb-объектом, чтобы
// details->'metadata' вернул тот же объект, что давала колонка
// application_history.metadata. Та же форма используется backfill'ом F.7
// (auditBackfillSources в migrate.go), который сворачивает плоские колонки замороженной
// application_history в этот details - иначе read-switch вернул бы не ту историю.
// action_user_id заявки - мёртвая колонка (нигде не писалась и не читалась), в audit_log
// не переносится.
type applicationAuditDetails struct {
	ActionStatus *string         `json:"action_status,omitempty"`
	OldValue     *string         `json:"old_value,omitempty"`
	NewValue     *string         `json:"new_value,omitempty"`
	Comment      *string         `json:"comment,omitempty"`
	Metadata     json.RawMessage `json:"metadata,omitempty"`
}

// GetApplicationResponsibleUsers возвращает ответственных пользователей заявки.
func (s *applicationService) GetApplicationResponsibleUsers(ctx context.Context, applicationID int) ([]ResponsibleUserInfo, error) {
	return s.fetchResponsibleUsers(s.db.WithContext(ctx), applicationID)
}

// GetApplicationHistory возвращает историю изменений заявки (новые сверху).
// #870 (срез 1.14 -> read-switch F.7): запись идёт в audit_log[application], а
// до-cutover строки замороженной application_history разово перенесены в audit_log
// (BackfillAuditFromLegacy, форма applicationAuditDetails), поэтому читаем ТОЛЬКО
// audit_log[application] - он уже содержит и исторические, и новые события.
// application_history осталась read-only бэкапом до дроп-sweep (F.8). Форму ответа
// стерегут TestApplications_HistoryGolden_*. Плоские поля старой схемы
// (action_status/old_value/new_value/comment) у audit_log лежат внутри details jsonb,
// metadata - вложенным объектом details->'metadata'. INNER JOIN users сохранён как в
// исходном чтении: строки с user_id IS NULL в историю не попадают (заявка всегда пишет
// не-NULL автора). Порядок created_at DESC, id DESC - id-тайбрейкер делает
// детерминированным порядок записей одного действия (recorder проставляет created_at
// временем вставки - монотонно растущим, как и id).
func (s *applicationService) GetApplicationHistory(ctx context.Context, applicationID int) ([]ApplicationHistoryItem, error) {
	const userName = `CONCAT(COALESCE(u.last_name, ''),
		CASE WHEN u.first_name IS NOT NULL AND u.first_name != '' THEN ' ' || u.first_name ELSE '' END,
		CASE WHEN u.middle_name IS NOT NULL AND u.middle_name != '' THEN ' ' || u.middle_name ELSE '' END
	)`
	sql := `
		SELECT
			m.id,
			m.application_id,
			m.user_id,
			` + userName + ` as user_name,
			u.last_name,
			u.first_name,
			u.middle_name,
			m.action_type,
			m.action_status,
			m.old_value,
			m.new_value,
			m.comment,
			m.created_at,
			m.metadata
		FROM (
			SELECT a.id, a.entity_id AS application_id, a.actor_user_id AS user_id,
				a.action AS action_type,
				a.details->>'action_status' AS action_status,
				a.details->>'old_value' AS old_value,
				a.details->>'new_value' AS new_value,
				a.details->>'comment' AS comment,
				a.details->'metadata' AS metadata,
				a.created_at
			FROM audit_log a
			WHERE a.entity_type = ? AND a.entity_id = ?
		) m
		JOIN users u ON m.user_id = u.id
		ORDER BY m.created_at DESC, m.id DESC
	`

	items := make([]ApplicationHistoryItem, 0)
	err := s.db.WithContext(ctx).Raw(sql, models.AuditEntityApplication, applicationID).Scan(&items).Error
	if err != nil {
		slog.Error("Ошибка получения истории заявки", "application_id", applicationID, "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching application history")
	}

	return items, nil
}

// AddHistoryEntry добавляет ручную запись в историю заявки (POST /applications/history).
// #870 (срез 1.14): пишет в audit_log[application] через recorder; ошибка проброса -
// как в прежнем write-path (раньше возвращался 500 при провале INSERT).
func (s *applicationService) AddHistoryEntry(ctx context.Context, req AddHistoryEntryRequest) error {
	details := applicationAuditDetails{
		ActionStatus: req.ActionStatus,
		OldValue:     req.OldValue,
		NewValue:     req.NewValue,
		Comment:      req.Comment,
	}
	if req.Metadata != nil {
		details.Metadata = json.RawMessage(*req.Metadata)
	}
	actorID := req.UserID
	if err := s.recorder.Record(ctx, nil, models.AuditEntityApplication, &req.ApplicationID, req.ActionType, &actorID, details); err != nil {
		slog.Error("Ошибка добавления записи истории", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Error adding history entry")
	}

	return nil
}
