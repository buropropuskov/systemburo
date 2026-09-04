package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"systemburo/internal/models"
	"systemburo/internal/normalize"
	"systemburo/internal/realtime"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

func (s *applicationService) getUserByUsername(ctx context.Context, username string) (*models.User, error) {
	var user models.User
	if err := s.db.WithContext(ctx).Where("username = ?", username).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, echo.NewHTTPError(http.StatusUnauthorized, "User not found")
		}
		slog.Error("Ошибка получения пользователя", "username", username, "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Database error")
	}
	return &user, nil
}

// isApprover проверяет, является ли пользовател�� принимающим.
func (s *applicationService) isApprover(ctx context.Context, userID int) (bool, error) {
	var count int64
	if err := s.db.WithContext(ctx).Model(&models.ApplicationApprover{}).Where("user_id = ?", userID).Count(&count).Error; err != nil {
		slog.Error("Ошибка проверки approver", "user_id", userID, "error", err)
		return false, echo.NewHTTPError(http.StatusInternalServerError, "Database error")
	}
	return count > 0, nil
}

// nilIfBlank возвращает nil для пустой (после trim) строки.
//
// Опциональные HMAC-поля (паспорт, патент) нельзя писать указателем на "": HMAC("") одинаков
// у всех незаполнивших, а дедуп PARTITION BY passport_series_number_hmac (rn=1) в
// GetActiveEmployeesForTable оставит в таблице проходной одного человека из всех безпаспортных.
// NULL из дедупа исключён условием hmac IS NULL OR rn = 1. Паспорт опционален: у мигрантов
// вместо него патент или иное разрешение.
func nilIfBlank(s string) *string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return &s
}

// nilIfBlankPtr - то же для указателя: пустая строка от клиента равнозначна незаполненному полю.
func nilIfBlankPtr(p *string) *string {
	if p == nil {
		return nil
	}
	return nilIfBlank(*p)
}

// formatFullName формирует полное ФИО.
func formatFullName(lastName, firstName, middleName *string) string {
	parts := []string{}
	if lastName != nil && *lastName != "" {
		parts = append(parts, *lastName)
	}
	if firstName != nil && *firstName != "" {
		parts = append(parts, *firstName)
	}
	if middleName != nil && *middleName != "" {
		parts = append(parts, *middleName)
	}
	return strings.Join(parts, " ")
}

// formatShortName формирует сокращённое ФИО (Фамилия И. О.).
func formatShortName(lastName, firstName, middleName *string) string {
	result := ""
	if lastName != nil && *lastName != "" {
		result = *lastName
	}
	if firstName != nil && *firstName != "" {
		result += " " + string([]rune(*firstName)[:1]) + "."
	}
	if middleName != nil && *middleName != "" {
		result += " " + string([]rune(*middleName)[:1]) + "."
	}
	return strings.TrimSpace(result)
}

// updateConfirmationBasedOnApprovals пересчитывает confirmation заявки по голосам ответственных.
func (s *applicationService) updateConfirmationBasedOnApprovals(tx *gorm.DB, applicationID int) error {
	var responsibles []models.ApplicationResponsibleUser
	if err := tx.Where("application_id = ?", applicationID).Find(&responsibles).Error; err != nil {
		slog.Error("Ошибка получения ответственных", "application_id", applicationID, "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Error fetching responsible users")
	}

	if len(responsibles) == 0 {
		return nil
	}

	votes := make([]approvalVote, 0, len(responsibles))
	for _, r := range responsibles {
		votes = append(votes, approvalVote{Required: r.RequiredApproval, Status: r.ApprovalStatus})
	}

	// Кворум считает общая с раундом дополнения функция (approval_tally.go); заявке
	// остаётся перевести исход в свой словарь confirmation.
	newConfirmation := models.ConfirmationPending
	switch tallyApprovals(votes) {
	case voteStatusApproved:
		newConfirmation = models.ConfirmationApproved
	case voteStatusRejected:
		newConfirmation = models.ConfirmationRejected
	}

	result := tx.Exec(`
		UPDATE applications
		SET confirmation = ?,
		    confirmation_datetime = CASE
		        WHEN ? != ? AND confirmation_datetime IS NULL THEN NOW()
		        ELSE confirmation_datetime
		    END
		WHERE id = ?
	`, newConfirmation, newConfirmation, models.ConfirmationPending, applicationID)

	if result.Error != nil {
		slog.Error("Ошибка обновления confirmation", "application_id", applicationID, "error", result.Error)
		return echo.NewHTTPError(http.StatusInternalServerError, "Error updating application confirmation")
	}

	return nil
}

// statusViewUpsert - идемпотентная per-user отметка "видел текущий статус заявки" (#1349).
// Эталон паттерна - questionViewUpsert (application_question_service.go).
const statusViewUpsert = `INSERT INTO application_status_views (application_id, user_id, seen_at)
	VALUES (?, ?, now())
	ON CONFLICT (application_id, user_id) DO UPDATE SET seen_at = now()`

// hasStatusUpdatePredicate - условие "status/confirmation заявки менялись после последнего
// просмотра пользователем" (#1349) для листингов и фильтров. Оба плейсхолдера - id
// просматривающего. GREATEST в Postgres игнорирует NULL; read_at участвует, чтобы
// прочтение ПОСЛЕ смены статуса (первое открытие непрочитанной) тоже гасило флаг.
const hasStatusUpdatePredicate = `(
		a.status_updated_at IS NOT NULL
		AND a.status_updated_at > COALESCE(GREATEST(
			(SELECT sv.seen_at FROM application_status_views sv WHERE sv.application_id = a.id AND sv.user_id = ?),
			(SELECT ar2.read_at FROM application_reads ar2 WHERE ar2.application_id = a.id AND ar2.user_id = ?)
		), to_timestamp(0))
	)`

// bumpStatusUpdated помечает заявку "статус изменился" (status_updated_at=NOW()) и тут же
// гасит флаг для автора действия: его seen_at ставится тем же NOW(), а предикат - строгое
// ">". Звать СТРОГО в той же транзакции, что и UPDATE статуса/подтверждения (NOW() внутри
// tx стабилен - иначе актор увидит собственный флаг), и только при реальной смене значения.
func (s *applicationService) bumpStatusUpdated(tx *gorm.DB, applicationID int, actorUserID *int) error {
	if err := tx.Exec("UPDATE applications SET status_updated_at = NOW() WHERE id = ?", applicationID).Error; err != nil {
		slog.Error("Ошибка отметки смены статуса", "application_id", applicationID, "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to mark status update")
	}
	if actorUserID != nil {
		if err := tx.Exec(statusViewUpsert, applicationID, *actorUserID).Error; err != nil {
			slog.Error("Ошибка отметки просмотра статуса актором", "application_id", applicationID, "user_id", *actorUserID, "error", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Failed to mark status seen")
		}
	}
	return nil
}

// activateApplicationItems активирует/деактивирует машины и сотрудников заявки. При активации
// (activate=true) для каждой машины/сотрудника, РЕАЛЬНО перешедших в активный статус (0->1),
// пишется история «Добавлен в таблицу проходной» (#1085) по их целевым таблицам - это и есть
// момент попадания в таблицу проходной (строки становятся видны охране по status=1). actorID -
// кто принял заявку в работу; при деактивации история не пишется (actorID игнорируется).
func (s *applicationService) activateApplicationItems(ctx context.Context, tx *gorm.DB, applicationID int, activate bool, actorID *int) error {
	type attachmentRow struct {
		ID             int
		AttachmentType string
	}
	var attachments []attachmentRow
	if err := tx.Raw("SELECT id, attachment_type FROM attachments WHERE application_id = ?", applicationID).Scan(&attachments).Error; err != nil {
		slog.Error("Ошибка получения вложений", "application_id", applicationID, "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Error fetching attachments")
	}

	for _, att := range attachments {
		if !activate {
			var err error
			switch att.AttachmentType {
			case "cars":
				err = tx.Exec("UPDATE cars SET status = 0, updated_at = CURRENT_TIMESTAMP WHERE attachment_id = ?", att.ID).Error
			case "people":
				err = tx.Exec("UPDATE employees SET status = 0, updated_at = CURRENT_TIMESTAMP WHERE attachment_id = ?", att.ID).Error
			default:
				continue
			}
			if err != nil {
				slog.Error("Ошибка деактивации элементов", "attachment_type", att.AttachmentType, "attachment_id", att.ID, "error", err)
				return echo.NewHTTPError(http.StatusInternalServerError, "Error updating items status")
			}
			continue
		}

		// Активация: обновляем только неактивные строки и по RETURNING id узнаём, кто РЕАЛЬНО
		// перешёл в активный статус - им пишем историю попадания в таблицу. Уже активная строка
		// историю не плодит; после деактивации и повторной активации пишется новое попадание.
		// IS DISTINCT FROM 1 (а не <> 1) на случай NULL-статуса - иначе NULL молча не активируется.
		//
		// Строки непринятого дополнения оживлять нельзя (#1685): сюда приходят и принятие в
		// работу после вывода из неё, и массовое /update-items-status, и ни один из этих путей
		// про раунды согласования не знает - без фильтра они пустили бы на КПП людей, по
		// которым решение ещё не принято.
		var ids []int
		switch att.AttachmentType {
		case "cars":
			if err := tx.Raw("UPDATE cars SET status = 1, updated_at = CURRENT_TIMESTAMP WHERE attachment_id = ? AND status IS DISTINCT FROM 1 AND "+admittedSupplementCond("cars")+" RETURNING id", att.ID).Scan(&ids).Error; err != nil {
				slog.Error("Ошибка активации машин", "attachment_id", att.ID, "error", err)
				return echo.NewHTTPError(http.StatusInternalServerError, "Error updating cars status")
			}
			s.recordEntitiesAddedToTable(ctx, tx, models.AuditEntityCar, ids, actorID)
		case "people":
			if err := tx.Raw("UPDATE employees SET status = 1, updated_at = CURRENT_TIMESTAMP WHERE attachment_id = ? AND status IS DISTINCT FROM 1 AND "+admittedSupplementCond("employees")+" RETURNING id", att.ID).Scan(&ids).Error; err != nil {
				slog.Error("Ошибка активации сотрудников", "attachment_id", att.ID, "error", err)
				return echo.NewHTTPError(http.StatusInternalServerError, "Error updating employees status")
			}
			s.recordEntitiesAddedToTable(ctx, tx, models.AuditEntityEmployee, ids, actorID)
		}
	}

	return nil
}

// recordEntitiesAddedToTable пишет историю «Добавлен в таблицу проходной» (#1085) для каждой пары
// (сущность, целевая таблица) активированных машин/сотрудников. entityType выбирает таблицу
// привязки (car_target_tables / employee_target_tables). Ошибка отдельной записи логируется и не
// прерывает цикл, но при РЕАЛЬНОМ SQL-сбое Postgres переводит транзакцию в aborted -> tx.Commit()
// у вызывающего вернёт ошибку и активация тоже откатится (это безопаснее «тихого» пропуска).
func (s *applicationService) recordEntitiesAddedToTable(ctx context.Context, tx *gorm.DB, entityType string, entityIDs []int, actorID *int) {
	for _, entityID := range entityIDs {
		var tableIDs []int
		var err error
		switch entityType {
		case models.AuditEntityCar:
			err = tx.Raw("SELECT table_id FROM car_target_tables WHERE car_id = ?", entityID).Scan(&tableIDs).Error
		case models.AuditEntityEmployee:
			err = tx.Raw("SELECT table_id FROM employee_target_tables WHERE employee_id = ?", entityID).Scan(&tableIDs).Error
		default:
			continue
		}
		if err != nil {
			slog.Error("audit: чтение целевых таблиц для истории попадания", "entity_type", entityType, "entity_id", entityID, "error", err)
			continue
		}
		for _, tableID := range tableIDs {
			if err := recordAddedToTable(ctx, s.recorder, tx, entityType, entityID, tableID, actorID); err != nil {
				slog.Error("audit: added_to_table при активации", "entity_type", entityType, "entity_id", entityID, "table_id", tableID, "error", err)
			}
		}
	}
}

// --- Основные методы ---

// applyApplicationAccessFilter ограничивает выборку заявок только теми,
// к которым у пользователя есть доступ. Approver-ы видят все заявки;
// остальные — только те, где они автор (sender_user_id), responsible или viewer.
//
// Helper переиспользуется в листинге заявок (GetApplications,
// buildApplicationsBaseQuery) и в счётчике непрочитанных (GetUnreadCount),
// чтобы фильтр доступа был в одном месте.
func applyApplicationAccessFilter(query *gorm.DB, userID int, isApprover bool) *gorm.DB {
	if isApprover {
		return query
	}
	return query.Where(`
		a.sender_user_id = ?
		OR EXISTS(SELECT 1 FROM application_responsible_users aru WHERE aru.application_id = a.id AND aru.user_id = ?)
		OR EXISTS(SELECT 1 FROM application_viewers av WHERE av.application_id = a.id AND av.user_id = ?)
	`, userID, userID, userID)
}

// centerAudience собирает пользователей, у которых заявка появляется в Центре
// заявок: автор, ответственные/согласующие и все принимающие (approver-ы видят
// все заявки - зеркало applyApplicationAccessFilter). Аудитория real-time сигнала
// обновления Центра (#840). Ошибка загрузки approver-ов не фатальна: сигнал
// best-effort, аудитория без них лучше, чем сбой публикации.
func (s *applicationService) centerAudience(ctx context.Context, applicationID, senderID int) []int {
	// Аудиторию читаем из БД по applicationID (после commit заявка и её связи уже
	// записаны) - зеркалит applyApplicationAccessFilter и не зависит от того, что
	// оказалось в scope вызывающего кода. best-effort: сбой любого запроса лишь
	// сузит аудиторию, не свалит публикацию.
	load := func(table string, query *gorm.DB) []int {
		var ids []int
		if err := query.Distinct("user_id").Pluck("user_id", &ids).Error; err != nil {
			slog.Warn("center audience: load failed", "table", table, "err", err)
		}
		return ids
	}
	responsibleIDs := load("application_responsible_users", s.db.WithContext(ctx).
		Table("application_responsible_users").Where("application_id = ?", applicationID))
	viewerIDs := load("application_viewers", s.db.WithContext(ctx).
		Table("application_viewers").Where("application_id = ?", applicationID))
	approverIDs := load("application_approvers", s.db.WithContext(ctx).
		Table("application_approvers"))
	return mergeUniqueIDs(senderID, responsibleIDs, viewerIDs, approverIDs)
}

// mergeUniqueIDs объединяет senderID и переданные группы id в слайс без дублей
// (порядок не гарантирован). Вынесено из centerAudience для юнит-тестируемости
// слияния отдельно от загрузки approver-ов из БД.
func mergeUniqueIDs(senderID int, groups ...[]int) []int {
	set := map[int]struct{}{senderID: {}}
	for _, g := range groups {
		for _, id := range g {
			set[id] = struct{}{}
		}
	}
	out := make([]int, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	return out
}

// applicationParticipants - аудитория детали заявки (#840 V4): все, у кого заявка
// видна (та же логика, что centerAudience/applyApplicationAccessFilter - автор,
// ответственные/согласующие, читатели, принимающие). Читает senderID из заявки.
func (s *applicationService) applicationParticipants(ctx context.Context, applicationID int) []int {
	var senderID int
	if err := s.db.WithContext(ctx).
		Table("applications").
		Select("sender_user_id").
		Where("id = ?", applicationID).
		Scan(&senderID).Error; err != nil {
		slog.Warn("application participants: load sender failed", "application_id", applicationID, "err", err)
	}
	return s.centerAudience(ctx, applicationID, senderID)
}

// applicationArchiveChange - изменила ли операция данные, которые лежат в файловом
// архиве заявки: бланк и заявка.json (#1615, B1). Именованный тип, а не голый bool,
// чтобы решение было видно в самом вызове, а новая точка мутации была обязана его
// принять - молча унаследовать чужое значение у неё не получится.
type applicationArchiveChange bool

const (
	// archiveDataChanged - изменился состав людей/машин/ТМЦ, сроки, организация,
	// статус или согласование: копию на диске надо пересобрать.
	archiveDataChanged applicationArchiveChange = true
	// archiveDataUnchanged - изменилось только то, чего нет ни в бланке, ни в
	// слепке: переписка по заявке и отметка о пропуске предупреждения ЧС. Дедуп по
	// хэшу спас бы от лишней записи на диск, но не от самой генерации бланка.
	archiveDataUnchanged applicationArchiveChange = false
)

// notifyApplicationUpdated шлёт участникам заявки два лёгких сигнала (#840):
//   - application.updated (scope application:<id>) - открытая деталь перезапросит
//     статус/вопросы/согласующих без F5 (V4);
//   - applications.refresh (scope applications-center) - список Центра у всех, кто
//     видит заявку, тихо перерисует столбцы подтверждения/статуса и производные
//     теги (иначе смена статуса протухала в списке до ручного обновления).
//
// Аудитория одна на оба - applicationParticipants (зеркало applyApplicationAccessFilter),
// поэтому кто видит деталь, тот видит и строку в Центре. Best-effort: без паблишера/при
// пустой аудитории - no-op, сбой не влияет на бизнес-операцию. Звать ПОСЛЕ commit изменения.
//
// change отвечает на отдельный вопрос: изменилось ли то, что лежит в файловом архиве
// (#1615, B1). Сигнал интерфейсу нужен любой правке, включая переписку по заявке, а
// пересборка бланка - только правке данных: генерация открывает xlsx-шаблон и делает
// полтора десятка запросов, и на каждый вопрос-ответ этот прогон уходил бы впустую.
func (s *applicationService) notifyApplicationUpdated(ctx context.Context, applicationID int, change applicationArchiveChange) {
	// До раннего return: очередь архива живёт независимо от realtimePublisher.
	if change == archiveDataChanged {
		s.enqueueArchiveExport(applicationID, BlankExportReasonUpdate)
	}

	if s.realtimePublisher == nil {
		return
	}
	audience := s.applicationParticipants(ctx, applicationID)
	s.realtimePublisher.PublishMany(audience, realtime.Event{
		Type:  "application.updated",
		Scope: fmt.Sprintf("application:%d", applicationID),
	})
	s.realtimePublisher.PublishMany(audience, realtime.Event{
		Type:  "applications.refresh",
		Scope: "applications-center",
	})
}

// applyStatusUpdatedFilter навешивает фильтр "только заявки с обновлённым статусом" (#1349),
// если statusUpdated=true. userID подставляется в обе точки предиката (seen_at и read_at
// того же пользователя). requireRead=true (Центр) дополнительно требует, чтобы заявка была
// прочитана (EXISTS application_reads) - в Центре флаг обновления виден только на прочитанных
// строках; requireRead=false (ЛК) - у отправителя записей application_reads нет.
func applyStatusUpdatedFilter(query *gorm.DB, userID int, statusUpdated *bool, requireRead bool) *gorm.DB {
	if statusUpdated == nil || !*statusUpdated {
		return query
	}
	query = query.Where(hasStatusUpdatePredicate, userID, userID)
	if requireRead {
		query = query.Where("EXISTS (SELECT 1 FROM application_reads ar WHERE ar.application_id = a.id AND ar.user_id = ?)", userID)
	}
	return query
}

// Исходы заявки, о которых уведомляется инициатор (#1349). Значения - для внутреннего
// switch в notifyInitiatorStatusChanged; наружу уходит только title/body уведомления.
const (
	statusOutcomeAccepted    = "accepted"
	statusOutcomeRejected    = "rejected"
	statusOutcomeApproved    = "approved"
	statusOutcomeNotApproved = "not_approved"
	statusOutcomeCompleted   = "completed"
)

// Тип уведомления -- NotificationTypeApplicationStatusChanged (каталог,
// notification_catalog.go): инициатору об исходе заявки (#1349). Навигация по
// data.application_id уже поддержана фронтом (UserNotifications.vue).

// confirmationOutcome возвращает исход-уведомление для нового значения confirmation, если
// это финальный исход согласования (Согласовано/Не согласовано). "" - промежуточное значение
// (возврат в "Согласование" при добавлении согласующих/отзыве голоса): инициатору об этом
// не сообщаем.
func confirmationOutcome(newConfirmation *string) string {
	if newConfirmation == nil {
		return ""
	}
	switch *newConfirmation {
	case models.ConfirmationApproved:
		return statusOutcomeApproved
	case models.ConfirmationRejected:
		return statusOutcomeNotApproved
	}
	return ""
}

// notifyInitiatorStatusChanged шлёт отправителю заявки best-effort уведомление об исходе
// (#1349): принята в работу / отклонена / согласована / не согласована / завершена. Гейт
// actorUserID != sender - актору собственный исход не шлём (крон передаёт nil и уведомляет
// всегда). Звать ПОСЛЕ commit: ошибки логируются, бизнес-операцию не откатывают. data:
// {application_id, application_number}.
// statusChangeContext -- кто и с каким комментарием принял решение по заявке.
// Показывается в подробностях уведомления: инициатору важно не только «отклонена»,
// но и кем, и почему (#1748). Пустые поля просто не попадают в payload; у решений
// крона (истечение срока) контекста нет вовсе.
type statusChangeContext struct {
	ActorName string
	Comment   string
}

func (s *applicationService) notifyInitiatorStatusChanged(ctx context.Context, applicationID int, actorUserID *int, outcome string, decision *statusChangeContext) {
	if s.notificationService == nil {
		return
	}

	var app struct {
		SenderUserID      *int
		ApplicationNumber string
	}
	if err := s.db.WithContext(ctx).
		Raw("SELECT sender_user_id, application_number FROM applications WHERE id = ?", applicationID).
		Scan(&app).Error; err != nil {
		slog.Warn("не удалось получить отправителя для уведомления об исходе заявки", "application_id", applicationID, "error", err)
		return
	}
	if app.SenderUserID == nil {
		return
	}
	if actorUserID != nil && *actorUserID == *app.SenderUserID {
		return
	}

	number := app.ApplicationNumber
	if number == "" {
		number = fmt.Sprintf("№ %d", applicationID)
	}

	var title, body string
	switch outcome {
	case statusOutcomeAccepted:
		title = "Заявка принята в работу"
		body = fmt.Sprintf("Ваша заявка %s принята в работу.", number)
	case statusOutcomeRejected:
		title = "Заявка отклонена"
		body = fmt.Sprintf("Ваша заявка %s отклонена.", number)
	case statusOutcomeApproved:
		title = "Заявка согласована"
		body = fmt.Sprintf("Ваша заявка %s согласована.", number)
	case statusOutcomeNotApproved:
		title = "Заявка не согласована"
		body = fmt.Sprintf("Ваша заявка %s не согласована.", number)
	case statusOutcomeCompleted:
		title = "Заявка завершена"
		body = fmt.Sprintf("Срок действия вашей заявки %s истёк, она завершена.", number)
	default:
		slog.Warn("неизвестный исход заявки для уведомления инициатору", "outcome", outcome, "application_id", applicationID)
		return
	}

	data := map[string]any{
		"application_id":     applicationID,
		"application_number": number,
	}
	if decision != nil {
		if decision.ActorName != "" {
			data["actor_name"] = decision.ActorName
		}
		if decision.Comment != "" {
			data["decision_comment"] = decision.Comment
		}
	}
	payload, _ := json.Marshal(data)
	payloadStr := string(payload)
	if err := s.notificationService.CreateForUser(ctx, *app.SenderUserID, NotificationTypeApplicationStatusChanged, title, body, &payloadStr); err != nil {
		slog.Warn("не удалось создать уведомление инициатору об исходе заявки", "user_id", *app.SenderUserID, "error", err)
	}
}

// pendingApproversBeforeWithdraw возвращает id пользователей, чьё решение по заявке
// ещё не поступило - ДО того как WithdrawApplication сменит статус на терминальный
// (Отозвана). Предикат зеркалит pendingApproverBaseQuery (reminder_service.go) на
// смысловом уровне: живая заявка ждёт согласования (confirmation="Согласование"),
// строка ответственного ещё не проголосовала (approval_status пуст/pending), и голос
// либо обязательный, либо обязательных вовсе нет. Отдельная копия, а не переиспользование
// reminderService - applicationService его не получает как зависимость, а вызывающая
// заявка уже проверена не-терминальной в этой же транзакции (FOR UPDATE), так что
// activeApplicationCond из pendingApproverBaseQuery здесь избыточен.
//
// Читаем ИЗНУТРИ транзакции отзыва, ДО UPDATE статуса: как только applications.status
// станет Отозвана (терминальный), тот же предикат перестанет матчить заявку, и список
// ожидающих потеряется. Best-effort: ошибка логируется, отзыв не откатывается.
func (s *applicationService) pendingApproversBeforeWithdraw(ctx context.Context, tx *gorm.DB, applicationID int) []int {
	var ids []int
	err := tx.WithContext(ctx).Raw(`
		SELECT DISTINCT aru.user_id
		FROM application_responsible_users aru
		JOIN applications a ON a.id = aru.application_id
		WHERE aru.application_id = ?
		  AND a.confirmation = ?
		  AND (aru.approval_status IS NULL OR aru.approval_status = 'pending')
		  AND (
		      aru.required_approval = true
		      OR NOT EXISTS (
		          SELECT 1 FROM application_responsible_users r2
		          WHERE r2.application_id = aru.application_id AND r2.required_approval = true
		      )
		  )
	`, applicationID, models.ConfirmationPending).Scan(&ids).Error
	if err != nil {
		slog.Warn("не удалось отобрать ожидающих согласующих перед отзывом заявки", "application_id", applicationID, "error", err)
		return nil
	}
	return ids
}

// notifyWithdrawn уведомляет тех, чьего решения ждали по отозванной заявке (#1748,
// S4): отзыв убирает предмет согласования из их очереди, и без явного сигнала заявка
// просто пропадает из списка ожидающих. userIDs собраны ДО смены статуса
// (pendingApproversBeforeWithdraw) - тем же предикатом, что и напоминания
// согласующим. Инициатору не шлём: это его собственное действие. Best-effort: ошибка
// логируется, WithdrawApplication уже закоммичен.
func (s *applicationService) notifyWithdrawn(ctx context.Context, applicationID int, withdrawnByName string, userIDs []int) {
	if s.notificationService == nil || len(userIDs) == 0 {
		return
	}

	var app struct{ ApplicationNumber string }
	if err := s.db.WithContext(ctx).
		Raw("SELECT COALESCE(application_number, '') AS application_number FROM applications WHERE id = ?", applicationID).
		Scan(&app).Error; err != nil {
		slog.Warn("не удалось получить номер заявки для уведомления об отзыве", "application_id", applicationID, "error", err)
		return
	}
	number := app.ApplicationNumber
	if number == "" {
		number = fmt.Sprintf("№ %d", applicationID)
	}

	title := "Заявка отозвана"
	body := fmt.Sprintf("Заявку %s отозвал(а) %s - рассматривать её больше не нужно.", number, withdrawnByName)

	data := map[string]any{"application_id": applicationID, "application_number": number}
	payload, err := json.Marshal(data)
	if err != nil {
		slog.Warn("не удалось сериализовать данные уведомления об отзыве", "application_id", applicationID, "error", err)
		return
	}
	payloadStr := string(payload)

	for _, userID := range userIDs {
		if err := s.notificationService.CreateForUser(ctx, userID, NotificationTypeApplicationWithdrawn, title, body, &payloadStr); err != nil {
			slog.Warn("не удалось создать уведомление об отзыве заявки", "user_id", userID, "application_id", applicationID, "error", err)
		}
	}
}

// buildSearchVariants возвращает уникальный набор вариантов поискового запроса:
// оригинал, альтернативная раскладка и нормализованный госномер (если запрос похож на номер).
// Используется для покрытия ввода без переключения раскладки и номеров с омоглифами/нулями.
func buildSearchVariants(raw string) []string {
	variants := make([]string, 0, 3)
	seen := make(map[string]struct{}, 3)
	add := func(v string) {
		v = strings.TrimSpace(v)
		if v == "" {
			return
		}
		if _, ok := seen[v]; !ok {
			seen[v] = struct{}{}
			variants = append(variants, v)
		}
	}
	add(raw)
	add(normalize.SwitchLayout(raw))
	add(normalize.Plate(raw))
	return variants
}

// ilikePatternsArgs строит пары (ILIKE-условие, args) для набора вариантов и набора колонок.
// Возвращает одну OR-строку из pattern_col ILIKE ? для каждой пары (col, variant).
func ilikePatternsArgs(cols []string, variants []string) (string, []interface{}) {
	var parts []string
	var args []interface{}
	for _, col := range cols {
		for _, v := range variants {
			parts = append(parts, col+" ILIKE ?")
			args = append(args, "%"+v+"%")
		}
	}
	return strings.Join(parts, " OR "), args
}

// parseIDList разбирает comma-список id из query-параметра мультивыбора (#1398).
// Нечисловые элементы отбрасываются, и параметр целиком из мусора даёт пустой список -
// такой фильтр не применяется вовсе. Пустой слайс в IN дал бы "ничего не найдено", то
// есть опечатка в параметре выглядела бы как "заявок нет"; лучше проигнорировать.
func parseIDList(raw *string) []int {
	if raw == nil || *raw == "" {
		return nil
	}
	parts := strings.Split(*raw, ",")
	ids := make([]int, 0, len(parts))
	for _, p := range parts {
		if id, err := strconv.Atoi(strings.TrimSpace(p)); err == nil {
			ids = append(ids, id)
		}
	}
	return ids
}

// applyApplicationFilters навешивает фильтры из ApplicationFilter. userID нужен для
// псевдо-фильтра "Непрочитано" (filter.Unread) - предикат по application_reads текущего
// пользователя; для остальных фильтров он не используется.
func applyApplicationFilters(query *gorm.DB, filter ApplicationFilter, includeUserSearch bool, userID int) *gorm.DB {
	if filter.SearchQuery != nil && *filter.SearchQuery != "" {
		raw := *filter.SearchQuery
		variants := buildSearchVariants(raw)

		// --- поля заявки и организаций ---
		baseCols := []string{
			"a.application_number",
			"COALESCE(o.name, c.name, '')",
			"c.name",
			"a.message",
			"a.status",
			"a.confirmation",
			"a.responsible_comment",
		}
		baseCond, baseArgs := ilikePatternsArgs(baseCols, variants)

		if includeUserSearch {
			userCols := []string{
				"u.last_name", "u.first_name", "u.middle_name",
				"ru.last_name", "ru.first_name", "ru.middle_name",
			}
			userCond, userArgs := ilikePatternsArgs(userCols, variants)
			baseCond += " OR " + userCond
			baseArgs = append(baseArgs, userArgs...)
		}

		// --- вложения: машины ---
		// Госномер ищем по всем вариантам (включая normalize.Plate для омоглифов/нулей);
		// марку - по тексту. EXISTS чтобы не размножать строки заявки.
		// Марка ищется по обеим колонкам: mark_name -- снимок имени марки, он появился
		// позже и заполнен у единиц записей, а в остальных марка лежит в устаревшей
		// car_brand. По одной mark_name заявка по марке своей машины не находилась.
		carNumCond, carNumArgs := ilikePatternsArgs([]string{"c2.car_number", "c2.mark_name", "c2.car_brand", "c2.unload_place"}, variants)
		// Слитно/раздельно: сравниваем номер без пробелов с нормализованным запросом,
		// чтобы "А 777 АА" находился по "А777АА" и наоборот (только если в запросе есть цифры).
		platePattern := ""
		if strings.ContainsAny(raw, "0123456789") {
			carNumCond += " OR REPLACE(c2.car_number, ' ', '') ILIKE ?"
			platePattern = "%" + normalize.Plate(raw) + "%"
		}
		carSubquery := `EXISTS(
			SELECT 1 FROM attachments att2
			JOIN cars c2 ON c2.attachment_id = att2.id
			WHERE att2.application_id = a.id AND (` + carNumCond + `)
		)`

		// --- вложения: сотрудники ---
		// Та же оговорка, что в реестре сотрудников: функция от concat_ws индексом не
		// покрывается и даёт полный просмотр таблицы. Индексируемая форма -- оператор
		// %>> по отдельным колонкам с порогом через SET LOCAL, так сделан сквозной поиск
		// (search_scope.go). Здесь оставлено прежнее поведение: замена меняет разбор
		// многословных запросов, а Центр открывают по кнопке, не на каждый символ.
		// ФИО ищем ILIKE + trigramm similarity для опечаток. strict_word_similarity (не word_),
		// иначе порог 0.3 ловит общие триграммы: "Карбышев"/"Зубарев"/"Арбатская" давали
		// word_similarity('арбуз',...) >= 0.33 (ложно), а strict даёт <0.24. При strict@0.3
		// реальные опечатки ("арбус"/"арбз"/"орбуз") остаются >=0.33, мусор отсекается.
		empIlikeCond, empIlikeArgs := ilikePatternsArgs(
			[]string{"e.last_name", "e.first_name", "e.middle_name", "e.position"},
			variants,
		)
		empSubquery := `EXISTS(
			SELECT 1 FROM attachments att3
			JOIN employees e ON e.attachment_id = att3.id
			WHERE att3.application_id = a.id AND (
				` + empIlikeCond + `
				OR strict_word_similarity(?, concat_ws(' ', e.last_name, e.first_name, e.middle_name)) > 0.3
			)
		)`

		// --- вложения: места разгрузки ---
		upCond, upArgs := ilikePatternsArgs([]string{"up.name"}, variants)
		// car_unload_places связывает МАШИНУ (car_id) с местом, а не вложение напрямую:
		// attachments -> cars -> car_unload_places -> unload_places.
		upSubquery := `EXISTS(
			SELECT 1 FROM attachments att4
			JOIN cars c4 ON c4.attachment_id = att4.id
			JOIN car_unload_places cup ON cup.car_id = c4.id
			JOIN unload_places up ON up.id = cup.unload_place_id
			WHERE att4.application_id = a.id AND (` + upCond + `)
		)`

		// --- согласующие: ФИО + комментарий согласующего ---
		// strict_word_similarity для опечаток в фамилии согласующего (диктовка охранником),
		// без ложных срабатываний на общих триграммах (см. блок сотрудников выше).
		apprIlikeCond, apprIlikeArgs := ilikePatternsArgs(
			[]string{"au.last_name", "au.first_name", "au.middle_name", "aru.approval_comment"},
			variants,
		)
		apprSubquery := `EXISTS(
			SELECT 1 FROM application_responsible_users aru
			JOIN users au ON au.id = aru.user_id
			WHERE aru.application_id = a.id AND (
				` + apprIlikeCond + `
				OR strict_word_similarity(?, concat_ws(' ', au.last_name, au.first_name, au.middle_name)) > 0.3
			)
		)`

		// --- вложения: работы (items) ---
		// Наименование работ в items-вложениях ("Заявка на работы") хранится в items.name.
		itemCond, itemArgs := ilikePatternsArgs([]string{"it.name"}, variants)
		itemSubquery := `EXISTS(
			SELECT 1 FROM attachments att5
			JOIN items it ON it.attachment_id = att5.id
			WHERE att5.application_id = a.id AND (` + itemCond + `)
		)`

		// --- вложения: наименование вложения ---
		// Имя вложения редактируется пользователем при подаче (#883) и хранится в
		// attachments.attachment_display_name; ищем по нему и по служебному attachment_name.
		attNameCond, attNameArgs := ilikePatternsArgs([]string{"att6.attachment_display_name", "att6.attachment_name"}, variants)
		attNameSubquery := `EXISTS(
			SELECT 1 FROM attachments att6
			WHERE att6.application_id = a.id AND (` + attNameCond + `)
		)`

		fullCond := baseCond + " OR " + carSubquery + " OR " + empSubquery +
			" OR " + upSubquery + " OR " + apprSubquery + " OR " + itemSubquery +
			" OR " + attNameSubquery

		allArgs := baseArgs
		allArgs = append(allArgs, carNumArgs...)
		if platePattern != "" {
			allArgs = append(allArgs, platePattern)
		}
		allArgs = append(allArgs, empIlikeArgs...)
		allArgs = append(allArgs, raw) // для word_similarity сотрудников
		allArgs = append(allArgs, upArgs...)
		allArgs = append(allArgs, apprIlikeArgs...)
		allArgs = append(allArgs, raw) // для word_similarity согласующих
		allArgs = append(allArgs, itemArgs...)
		allArgs = append(allArgs, attNameArgs...)

		query = query.Where(fullCond, allArgs...)
	}

	if filter.OrganizationID != nil {
		query = query.Where("a.organization_id = ?", *filter.OrganizationID)
	}
	if filter.CompanyID != nil {
		query = query.Where("a.company_id = ?", *filter.CompanyID)
	}
	if ids := parseIDList(filter.OrganizationIDs); len(ids) > 0 {
		query = query.Where("a.organization_id IN ?", ids)
	}
	if ids := parseIDList(filter.CompanyIDs); len(ids) > 0 {
		query = query.Where("a.company_id IN ?", ids)
	}
	// Места разгрузки (#1398). EXISTS, а не JOIN: джойн размножил бы строку заявки по
	// числу подходящих вложений.
	if ids := parseIDList(filter.UnloadPlaceIDs); len(ids) > 0 {
		query = query.Where(`
			EXISTS(
				SELECT 1 FROM attachments att_up
				JOIN attachment_unload_places aup ON aup.attachment_id = att_up.id
				WHERE att_up.application_id = a.id AND aup.unload_place_id IN ?
			)
		`, ids)
	}
	// Таблицы проходной (#1398): машина через car_target_tables ИЛИ сотрудник через
	// employee_target_tables - привязки живут на элементах вложения, не на заявке.
	if ids := parseIDList(filter.PassageTableIDs); len(ids) > 0 {
		query = query.Where(`(
			EXISTS(
				SELECT 1 FROM attachments att_ct
				JOIN cars c_ct ON c_ct.attachment_id = att_ct.id
				JOIN car_target_tables ctt ON ctt.car_id = c_ct.id
				WHERE att_ct.application_id = a.id AND ctt.table_id IN ?
			)
			OR EXISTS(
				SELECT 1 FROM attachments att_et
				JOIN employees e_et ON e_et.attachment_id = att_et.id
				JOIN employee_target_tables ett ON ett.employee_id = e_et.id
				WHERE att_et.application_id = a.id AND ett.table_id IN ?
			)
		)`, ids, ids)
	}
	if filter.SenderUserID != nil {
		query = query.Where("a.sender_user_id = ?", *filter.SenderUserID)
	}
	if filter.Confirmation != nil && *filter.Confirmation != "" {
		// Мультивыбор чипов подтверждения = OR: comma-список -> IN (одно значение тоже
		// проходит как IN из одного элемента).
		query = query.Where("a.confirmation IN ?", strings.Split(*filter.Confirmation, ","))
	}
	// Статус заявки + псевдо-фильтр "Непрочитано" (нет записи в application_reads).
	// В UI это один OR-набор чипов, поэтому условия объединяются через OR внутри одной
	// скобки (иначе AND отсёк бы одно другим). "Непрочитано" не хранится как a.status
	// (мигрирован в "В обработке"), непрочитанность - только через application_reads.
	statusConds := make([]string, 0, 2)
	statusArgs := make([]any, 0, 2)
	if filter.Status != nil && *filter.Status != "" {
		statusConds = append(statusConds, "a.status IN ?")
		statusArgs = append(statusArgs, strings.Split(*filter.Status, ","))
	}
	if filter.Unread != nil && *filter.Unread {
		statusConds = append(statusConds, "NOT EXISTS (SELECT 1 FROM application_reads ar WHERE ar.application_id = a.id AND ar.user_id = ?)")
		statusArgs = append(statusArgs, userID)
	}
	if len(statusConds) > 0 {
		query = query.Where("("+strings.Join(statusConds, " OR ")+")", statusArgs...)
	}
	if filter.DateFrom != nil && *filter.DateFrom != "" {
		query = query.Where("a.sending_datetime >= ?", *filter.DateFrom+" 00:00:00")
	}
	if filter.DateTo != nil && *filter.DateTo != "" {
		query = query.Where("a.sending_datetime <= ?", *filter.DateTo+" 23:59:59")
	}

	// Архив: по умолчанию скрываем архивные, archive=true оставляет только их.
	// Определение архива одно на весь сервис - application_archive.go.
	if filter.Archive != nil && *filter.Archive {
		cond, args := archivedApplicationCond("a")
		query = query.Where(cond, args...)
	} else {
		cond, args := activeApplicationCond("a")
		query = query.Where(cond, args...)
	}

	// Active today: заявка активна сегодня, если период действия хотя бы одного
	// вложения (entry_date_from..entry_date_to) включает текущую дату.
	if filter.ActiveToday != nil && *filter.ActiveToday {
		query = query.Where(`
			EXISTS(
				SELECT 1 FROM attachments att
				WHERE att.application_id = a.id
				AND att.entry_date_from IS NOT NULL
				AND att.entry_date_to IS NOT NULL
				AND `+moscowTodaySQL+` BETWEEN CAST(att.entry_date_from AS DATE) AND CAST(att.entry_date_to AS DATE)
			)
		`)
	}

	return query
}

func (s *applicationService) fetchResponsibleUsers(ctx context.Context, db *gorm.DB, applicationID int) ([]ResponsibleUserInfo, error) {
	responsibles := make([]ResponsibleUserInfo, 0)
	err := db.Raw(`
		SELECT
			u.id,
			u.username,
			u.last_name,
			u.first_name,
			u.middle_name,
			u.position,
			COALESCE(aru.is_primary, false) as is_primary,
			COALESCE(aru.required_approval, false) as required_approval,
			aru.approval_status,
			aru.approval_comment,
			aru.approval_datetime,
			aru.created_at,
			COALESCE(aru.reminder_count, 0) as reminder_count
		FROM application_responsible_users aru
		JOIN users u ON aru.user_id = u.id
		WHERE aru.application_id = ?
		ORDER BY aru.is_primary DESC, u.last_name, u.first_name
	`, applicationID).Scan(&responsibles).Error

	if err != nil {
		slog.Error("Ошибка получения ответственных пользователей", "application_id", applicationID, "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching responsible users")
	}

	if responsibles == nil {
		responsibles = []ResponsibleUserInfo{}
	}
	if masks := loadConsentMasks(ctx, db); len(masks) > 0 {
		for i := range responsibles {
			maskUserParts(masks, responsibles[i].ID,
				&responsibles[i].LastName, &responsibles[i].FirstName, &responsibles[i].MiddleName)
		}
	}
	return responsibles, nil
}

// CanAccessApplication проверяет, имеет ли пользователь доступ к заявке.
// Доступ имеют: super-admin, отправитель, ответственные и просматривающие.
func (s *applicationService) CanAccessApplication(ctx context.Context, applicationID int, username string, isSuperAdmin bool) bool {
	if isSuperAdmin {
		return true
	}

	user, err := s.getUserByUsername(ctx, username)
	if err != nil {
		return false
	}

	// Принимающий (глобальный оператор бюро, admin-раздел "Принимающие") видит и
	// открывает ВСЕ заявки - как и список центра (applyApplicationAccessFilter
	// пускает isApprover без фильтра). Без этой ветки принимающий видел заявку в
	// списке, но детальные эндпоинты (/details,/attachments,/viewers,/history)
	// отдавали 403: security-аудит добавил гейт деталей без ветки принимающего.
	if isApprover, _ := s.isApprover(ctx, user.ID); isApprover {
		return true
	}

	var app models.Application
	if err := s.db.WithContext(ctx).Select("id, sender_user_id").Where("id = ?", applicationID).First(&app).Error; err != nil {
		return false
	}

	if app.SenderUserID == user.ID {
		return true
	}

	var count int64
	s.db.WithContext(ctx).Model(&models.ApplicationResponsibleUser{}).
		Where("application_id = ? AND user_id = ?", applicationID, user.ID).
		Count(&count)
	if count > 0 {
		return true
	}

	s.db.WithContext(ctx).Model(&models.ApplicationViewer{}).
		Where("application_id = ? AND user_id = ?", applicationID, user.ID).
		Count(&count)
	return count > 0
}

// IsApplicationSender отвечает, подал ли эту заявку сам пользователь. Узкая проверка
// рядом с CanAccessApplication и намеренно уже неё: доступ к заявке есть и у
// согласующих, принимающих и получателей пересылки, а сведения документов участников
// вводил в форму именно инициатор - прятать их от него нечего и незачем.
func (s *applicationService) IsApplicationSender(ctx context.Context, applicationID, userID int) (bool, error) {
	if applicationID == 0 || userID == 0 {
		return false, nil
	}
	var app models.Application
	if err := s.db.WithContext(ctx).Select("id, sender_user_id").
		Where("id = ?", applicationID).First(&app).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, fmt.Errorf("check application sender: %w", err)
	}
	return app.SenderUserID == userID, nil
}

// GetApplicationIDByAttachment возвращает ID заявки по ID вложения. Для manual-вложения
// без заявки (#1049, application_id NULL) возвращает 0 - вызыватели-гейты доступа к
// заявке трактуют 0 как "нет заявки" (application-detail путь к сироте недоступен).
func (s *applicationService) GetApplicationIDByAttachment(ctx context.Context, attachmentID int) (int, error) {
	var attachment models.Attachment
	if err := s.db.WithContext(ctx).Select("id, application_id").Where("id = ?", attachmentID).First(&attachment).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return 0, echo.NewHTTPError(http.StatusNotFound, "Attachment not found")
		}
		return 0, echo.NewHTTPError(http.StatusInternalServerError, "Database error")
	}
	return safeDerefInt(attachment.ApplicationID), nil
}

func ptrString(s string) *string { return &s }

// optionalString разыменовывает необязательную строку запроса: пустая строка
// означает «поля не было», и в payload уведомления она не попадает.
func optionalString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func safeDerefInt(p *int) int {
	if p != nil {
		return *p
	}
	return 0
}

// notifyApproversAboutNewApplication зовёт принимающих к свежеподанной заявке.
// Принимающий - глобальная роль (строка в application_approvers), не привязанная ни к
// организации, ни к конкретной заявке, поэтому зовём весь реестр. Автор пропускается:
// подавший заявку и сам знает, что подал, а «Заявка отправлена» ему уже ушла.
// Ошибка отдельного получателя не прерывает рассылку - остальные должны узнать.
func (s *applicationService) notifyApproversAboutNewApplication(
	ctx context.Context, authorID, appID int, note pendingAcceptanceNote, payload string,
) {
	var approverIDs []int
	if err := s.db.WithContext(ctx).Model(&models.ApplicationApprover{}).
		Where("user_id <> ?", authorID).
		Pluck("user_id", &approverIDs).Error; err != nil {
		slog.Error("не удалось получить список принимающих для уведомления о новой заявке",
			"app_id", appID, "error", err)
		return
	}
	for _, userID := range approverIDs {
		if err := s.notificationService.CreateForUser(
			ctx, userID,
			NotificationTypeApplicationPendingAcceptance,
			"Новая заявка",
			note.message(),
			&payload,
		); err != nil {
			slog.Warn("не удалось уведомить принимающего о новой заявке",
				"user_id", userID, "app_id", appID, "error", err)
		}
	}
}

// pendingAcceptanceNote -- из чего складывается приглашение принять заявку.
type pendingAcceptanceNote struct {
	number       string
	organization string
	sender       string
	messageText  string
	fileNames    []string
}

// message собирает текст уведомления. Первая строка несёт номер И организацию вместе:
// в свёрнутом уведомлении система показывает заголовок и ровно одну строку текста,
// остальное прячет за многоточием, и содержимое этого многоточия задать нельзя - значит
// всё, что должно быть видно не разворачивая, обязано уместиться в первую строку.
// Дальше через отступ отправитель, потом превью сообщения, потом вложения отдельным
// блоком: приписанные к превью, они там терялись. Пустые части выпадают целиком.
func (n pendingAcceptanceNote) message() string {
	head := n.number
	if org := strings.TrimSpace(n.organization); org != "" {
		head = fmt.Sprintf("%s · %s", head, org)
	}
	blocks := []string{head}

	if sender := strings.TrimSpace(n.sender); sender != "" {
		blocks = append(blocks, sender)
	}
	if preview := previewText(plainTextFromRichText(n.messageText), notificationPreviewLimit); preview != "" {
		blocks = append(blocks, preview)
	}
	if files := filesLabel(n.fileNames); files != "" {
		blocks = append(blocks, files)
	}
	return strings.Join(blocks, "\n\n")
}

// applicationSenderTitle - наименование организации заявки, а если её нет, то компании.
// Читается из справочника, а не берётся из тела запроса: фронт присылает то, что человек
// набрал в поле, и при выборе существующей записи это может расходиться с реальным
// названием в справочнике.
func (s *applicationService) applicationSenderTitle(ctx context.Context, organizationID, companyID *int) string {
	if organizationID != nil {
		var name string
		if err := s.db.WithContext(ctx).Table("organizations").
			Where("id = ?", *organizationID).Limit(1).Pluck("name", &name).Error; err == nil && name != "" {
			return name
		}
	}
	if companyID != nil {
		var name string
		if err := s.db.WithContext(ctx).Table("companies").
			Where("id = ?", *companyID).Limit(1).Pluck("name", &name).Error; err == nil {
			return name
		}
	}
	return ""
}

// applicationFileNames -- имена вложений заявки для уведомления. Читаются из базы после
// привязки, а не берутся из запроса: тело несёт только идентификаторы, а человеку нужны
// названия. Ошибка чтения не повод молчать обо всей заявке - вернём пустой список, и
// строка про вложения просто не появится.
func (s *applicationService) applicationFileNames(ctx context.Context, fileIDs []int) []string {
	if len(fileIDs) == 0 {
		return nil
	}
	var names []string
	if err := s.db.WithContext(ctx).Model(&models.ApplicationFile{}).
		Where("id IN ?", fileIDs).
		Order("id").
		Pluck("file_name", &names).Error; err != nil {
		slog.Warn("не удалось прочитать имена вложений для уведомления", "error", err)
		return nil
	}
	return names
}
