package services

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
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

	var required, nonRequired []models.ApplicationResponsibleUser
	for _, r := range responsibles {
		if r.RequiredApproval {
			required = append(required, r)
		} else {
			nonRequired = append(nonRequired, r)
		}
	}

	newConfirmation := models.ConfirmationPending

	hasRequiredRejected := false
	for _, r := range required {
		if r.ApprovalStatus != nil && *r.ApprovalStatus == "rejected" {
			hasRequiredRejected = true
			break
		}
	}

	if hasRequiredRejected {
		newConfirmation = models.ConfirmationRejected
	} else if len(required) > 0 {
		allApproved := true
		for _, r := range required {
			if r.ApprovalStatus == nil || *r.ApprovalStatus != "approved" {
				allApproved = false
				break
			}
		}
		if allApproved {
			newConfirmation = models.ConfirmationApproved
		}
	} else if len(nonRequired) > 0 {
		hasAnyApproved := false
		hasAnyRejected := false
		for _, r := range nonRequired {
			if r.ApprovalStatus != nil && *r.ApprovalStatus == "approved" {
				hasAnyApproved = true
			}
			if r.ApprovalStatus != nil && *r.ApprovalStatus == "rejected" {
				hasAnyRejected = true
			}
		}
		if hasAnyApproved && !hasAnyRejected {
			newConfirmation = models.ConfirmationApproved
		} else if hasAnyRejected {
			newConfirmation = models.ConfirmationRejected
		}
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

// activateApplicationItems активирует/деактивирует машины и сотрудников заявки.
func (s *applicationService) activateApplicationItems(tx *gorm.DB, applicationID int, activate bool) error {
	newStatus := 0
	if activate {
		newStatus = 1
	}

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
		switch att.AttachmentType {
		case "cars":
			if err := tx.Exec("UPDATE cars SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE attachment_id = ?", newStatus, att.ID).Error; err != nil {
				slog.Error("Ошибка обновления статуса машин", "attachment_id", att.ID, "error", err)
				return echo.NewHTTPError(http.StatusInternalServerError, "Error updating cars status")
			}
		case "people":
			if err := tx.Exec("UPDATE employees SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE attachment_id = ?", newStatus, att.ID).Error; err != nil {
				slog.Error("Ошибка обновления статуса сотрудников", "attachment_id", att.ID, "error", err)
				return echo.NewHTTPError(http.StatusInternalServerError, "Error updating employees status")
			}
		}
	}

	return nil
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
func (s *applicationService) notifyApplicationUpdated(ctx context.Context, applicationID int) {
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

func applyApplicationFilters(query *gorm.DB, filter ApplicationFilter, includeUserSearch bool) *gorm.DB {
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
		carNumCond, carNumArgs := ilikePatternsArgs([]string{"c2.car_number", "c2.mark_name", "c2.unload_place"}, variants)
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
	if filter.Confirmation != nil {
		query = query.Where("a.confirmation = ?", *filter.Confirmation)
	}
	if filter.Status != nil {
		query = query.Where("a.status = ?", *filter.Status)
	}
	if filter.DateFrom != nil && *filter.DateFrom != "" {
		query = query.Where("a.sending_datetime >= ?", *filter.DateFrom+" 00:00:00")
	}
	if filter.DateTo != nil && *filter.DateTo != "" {
		query = query.Where("a.sending_datetime <= ?", *filter.DateTo+" 23:59:59")
	}

	// Archive filter: by default exclude archived, archive=true shows only archived
	archiveCondition := `
		(a.status IN ? AND EXISTS(
			SELECT 1 FROM attachments att WHERE att.application_id = a.id
			AND att.entry_date_to IS NOT NULL
			AND CAST(att.entry_date_to AS DATE) + INTERVAL '1 month' < NOW()
		))
	`
	if filter.Archive != nil && *filter.Archive {
		query = query.Where(archiveCondition, models.ArchivableStatuses)
	} else {
		query = query.Where("NOT "+archiveCondition, models.ArchivableStatuses)
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
				AND CURRENT_DATE BETWEEN CAST(att.entry_date_from AS DATE) AND CAST(att.entry_date_to AS DATE)
			)
		`)
	}

	return query
}

func (s *applicationService) fetchResponsibleUsers(db *gorm.DB, applicationID int) ([]ResponsibleUserInfo, error) {
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
			aru.approval_datetime
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

// GetApplicationIDByAttachment возвращает ID заявки по ID вложения.
func (s *applicationService) GetApplicationIDByAttachment(ctx context.Context, attachmentID int) (int, error) {
	var attachment models.Attachment
	if err := s.db.WithContext(ctx).Select("id, application_id").Where("id = ?", attachmentID).First(&attachment).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return 0, echo.NewHTTPError(http.StatusNotFound, "Attachment not found")
		}
		return 0, echo.NewHTTPError(http.StatusInternalServerError, "Database error")
	}
	return attachment.ApplicationID, nil
}

func ptrString(s string) *string { return &s }

func safeDerefInt(p *int) int {
	if p != nil {
		return *p
	}
	return 0
}
