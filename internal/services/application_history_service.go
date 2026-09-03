package services

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
)

// ForwardMessageItem - одна пересылка заявки в "ветке заявки" (#967).
// Источник - сводные записи forwarded в audit_log. Message - сопроводительный текст
// (может быть пустым: пересылка без текста тоже попадает в ветку), Recipients - кому
// переслано (из metadata.recipients), AuthorName - кто переслал. Whole/Attachments -
// что переслано (вся заявка либо перечень вложений), для строки "действия" в ветке.
type ForwardMessageItem struct {
	ID          int       `json:"id"`
	AuthorID    int       `json:"author_id"`
	AuthorName  string    `json:"author_name"`
	Message     string    `json:"message"`
	Recipients  []string  `json:"recipients"`
	Whole       bool      `json:"whole"`
	Attachments []string  `json:"attachments"`
	CreatedAt   time.Time `json:"created_at"`
}

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
	return s.fetchResponsibleUsers(ctx, s.db.WithContext(ctx), applicationID)
}

// GetApplicationHistory возвращает историю изменений заявки (новые сверху).
// #870 (срез 1.14 -> read-switch F.7): запись идёт в audit_log[application], а
// до-cutover строки замороженной application_history разово перенесены в audit_log
// (BackfillAuditFromLegacy, форма applicationAuditDetails), поэтому читаем ТОЛЬКО
// audit_log[application] - он уже содержит и исторические, и новые события.
// application_history дропнута в дроп-sweep (F.8). Форму ответа
// стерегут TestApplications_HistoryGolden_*. Плоские поля старой схемы
// (action_status/old_value/new_value/comment) у audit_log лежат внутри details jsonb,
// metadata - вложенным объектом details->'metadata'. LEFT JOIN users (#1240): у системных
// событий (completed - завершение по сроку кроном) актора нет, а прежний INNER JOIN такие
// строки терял; user_id тогда 0, и FE рисует "Система" (ветка !item.user_id).
// Порядок created_at DESC, id DESC - id-тайбрейкер делает
// детерминированным порядок записей одного действия (recorder проставляет created_at
// временем вставки - монотонно растущим, как и id).
func (s *applicationService) GetApplicationHistory(ctx context.Context, applicationID int, username string) ([]ApplicationHistoryItem, error) {
	// Записи о заметке бюро видит только тот, кто видит саму заметку. Иначе заявитель
	// прочитал бы в ленте своей заявки, что бюро что-то про неё писало, - а заметку от
	// него прячут целиком, включая сам факт (см. application_bureau_note.go).
	viewer, err := s.getUserByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	viewerIsApprover, err := s.isApprover(ctx, viewer.ID)
	if err != nil {
		return nil, err
	}

	const userName = `CONCAT(COALESCE(u.last_name, ''),
		CASE WHEN u.first_name IS NOT NULL AND u.first_name != '' THEN ' ' || u.first_name ELSE '' END,
		CASE WHEN u.middle_name IS NOT NULL AND u.middle_name != '' THEN ' ' || u.middle_name ELSE '' END
	)`
	sql := `
		SELECT
			m.id,
			m.application_id,
			COALESCE(m.user_id, 0) as user_id,
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
				AND (? OR a.action NOT IN (?, ?, ?))
		) m
		LEFT JOIN users u ON m.user_id = u.id
		ORDER BY m.created_at DESC, m.id DESC
	`

	items := make([]ApplicationHistoryItem, 0)
	err = s.db.WithContext(ctx).Raw(sql, models.AuditEntityApplication, applicationID, viewerIsApprover,
		models.AuditActionBureauNoteCreated, models.AuditActionBureauNoteUpdated,
		models.AuditActionBureauNoteCleared).Scan(&items).Error
	if err != nil {
		slog.Error("Ошибка получения истории заявки", "application_id", applicationID, "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching application history")
	}

	// Маскировка ФИО актора: заданная маска принимающего (напр. "Принял(-а) в работу")
	// и логин вместо ФИО у тех, кто не давал согласия на обработку данных.
	if masks := loadNameMasks(ctx, s.db); masks != nil {
		for i := range items {
			uid := items[i].UserID
			items[i].UserName = maskName(masks, &uid, items[i].UserName)
		}
	}

	return items, nil
}

// GetForwardMessages возвращает ветку заявки - все пересылки (#967), хронологически
// (старые сверху, как переписка). Источник - сводные записи forwarded в audit_log;
// actor_user_id - пересылающий, JOIN users даёт его ФИО (как в GetApplicationHistory).
// Пересылки без сопроводительного текста тоже входят в ветку (message пустой) - показывается
// только "кто -> кому". Получатели берутся из metadata.recipients.
func (s *applicationService) GetForwardMessages(ctx context.Context, applicationID int) ([]ForwardMessageItem, error) {
	const authorName = `CONCAT(COALESCE(u.last_name, ''),
		CASE WHEN u.first_name IS NOT NULL AND u.first_name != '' THEN ' ' || u.first_name ELSE '' END,
		CASE WHEN u.middle_name IS NOT NULL AND u.middle_name != '' THEN ' ' || u.middle_name ELSE '' END
	)`
	sql := `
		SELECT
			a.id,
			a.actor_user_id AS author_id,
			` + authorName + ` AS author_name,
			COALESCE(a.details->>'comment', '') AS message,
			COALESCE(a.details->'metadata'->>'recipients', '[]') AS recipients_json,
			COALESCE((a.details->'metadata'->>'whole')::boolean, true) AS whole,
			COALESCE(a.details->'metadata'->>'attachments', '[]') AS attachments_json,
			a.created_at
		FROM audit_log a
		JOIN users u ON a.actor_user_id = u.id
		WHERE a.entity_type = ? AND a.entity_id = ? AND a.action = ?
		ORDER BY a.created_at ASC, a.id ASC
	`

	type forwardMessageRow struct {
		ID              int
		AuthorID        int
		AuthorName      string
		Message         string
		RecipientsJSON  string
		Whole           bool
		AttachmentsJSON string
		CreatedAt       time.Time
	}
	var rows []forwardMessageRow
	if err := s.db.WithContext(ctx).Raw(sql, models.AuditEntityApplication, applicationID, models.AuditActionForwarded).Scan(&rows).Error; err != nil {
		slog.Error("Ошибка получения ветки заявки", "application_id", applicationID, "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching forward messages")
	}

	// Битый JSON-массив в metadata не роняет ветку: деградируем к пустому списку
	// (запись всё равно показывает автора/действие/текст), но логируем аномалию.
	parseStrings := func(raw, field string, auditID int) []string {
		out := []string{}
		if raw == "" {
			return out
		}
		if err := json.Unmarshal([]byte(raw), &out); err != nil {
			slog.Warn("не удалось разобрать массив пересылки", "field", field, "audit_id", auditID, "error", err)
			return []string{}
		}
		return out
	}

	// Логин вместо ФИО у пересылавших, не давших согласия на обработку данных.
	masks := loadConsentMasks(ctx, s.db)
	items := make([]ForwardMessageItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, ForwardMessageItem{
			ID:          r.ID,
			AuthorID:    r.AuthorID,
			AuthorName:  maskName(masks, &r.AuthorID, r.AuthorName),
			Message:     r.Message,
			Recipients:  parseStrings(r.RecipientsJSON, "recipients", r.ID),
			Whole:       r.Whole,
			Attachments: parseStrings(r.AttachmentsJSON, "attachments", r.ID),
			CreatedAt:   r.CreatedAt,
		})
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
