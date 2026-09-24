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
	"gorm.io/gorm"
)

// ChangeApplicationDatesRequest - новый срок действия заявки от принимающего.
// Дата в формате ГГГГ-ММ-ДД, время ЧЧ:ММ или ЧЧ:ММ:СС, как их шлёт форма подачи.
type ChangeApplicationDatesRequest struct {
	EntryDateFrom string `json:"entry_date_from" validate:"required"`
	EntryDateTo   string `json:"entry_date_to" validate:"required"`
	EntryTimeFrom string `json:"entry_time_from" validate:"required"`
	EntryTimeTo   string `json:"entry_time_to" validate:"required"`
	Reason        string `json:"reason" validate:"required,min=1,max=1000"`
}

// ChangeApplicationDatesResult - что изменилось: окно до правки, после неё и сброшены
// ли голоса согласующих.
type ChangeApplicationDatesResult struct {
	OldPeriod      string `json:"old_period"`
	NewPeriod      string `json:"new_period"`
	ApprovalsReset bool   `json:"approvals_reset"`
}

// applicationDatesEditableStatuses - когда принимающий может сдвинуть срок: заявка ещё
// не принята в работу. Белый список, как у мест (урок #1083): следующий статус иначе
// молча окажется разрешённым.
var applicationDatesEditableStatuses = []string{
	models.StatusUnread,
	models.StatusProcessing,
}

// applicationPeriod - окно допуска вложения или машины в формате хранения.
type applicationPeriod struct {
	DateFrom string
	DateTo   string
	TimeFrom string
	TimeTo   string
}

// ChangeApplicationDates задаёт одно окно допуска всем вложениям заявки и всем её машинам.
//
// Зачем: заявитель часто промахивается со сроком на день или на час, и раньше бюро могло
// только отказать и ждать новую заявку. Править можно, пока заявка не принята и по ней нет
// итога согласования: после этого срок уже стал обещанием охране и заявителю.
//
// Голоса согласующих сбрасываются, если кто-то уже голосовал: они одобряли другое окно.
// Машины получают то же окно, иначе пост видел бы у машины прежний срок.
func (s *applicationService) ChangeApplicationDates(ctx context.Context, username string, applicationID int, req ChangeApplicationDatesRequest) (*ChangeApplicationDatesResult, error) {
	user, err := s.getUserByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	isApprover, err := s.isApprover(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	if !isApprover {
		return nil, echo.NewHTTPError(http.StatusForbidden, "Менять срок заявки может только принимающий")
	}
	if err := s.checkNotArchived(ctx, applicationID); err != nil {
		return nil, err
	}

	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Укажите причину изменения срока")
	}
	period, err := parseApplicationPeriod(req, time.Now())
	if err != nil {
		return nil, err
	}

	var result *ChangeApplicationDatesResult
	var applicationNumber string
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		number, err := lockApplicationForDatesChange(tx, applicationID)
		if err != nil {
			return err
		}
		applicationNumber = number

		oldPeriods, err := applicationAttachmentPeriods(tx, applicationID)
		if err != nil {
			return err
		}
		if len(oldPeriods) == 0 {
			return echo.NewHTTPError(http.StatusBadRequest, "В заявке нет вложений со сроком")
		}
		unchanged, err := applicationPeriodUnchanged(tx, applicationID, oldPeriods, period)
		if err != nil {
			return err
		}
		if unchanged {
			return echo.NewHTTPError(http.StatusBadRequest, "Срок не изменился")
		}
		if err := checkByFactPeriodOnChange(tx, applicationID, period, time.Now()); err != nil {
			return err
		}

		if err := writeApplicationPeriod(tx, applicationID, period); err != nil {
			return err
		}

		approvalsReset, err := s.resetApprovalsIfVoted(ctx, tx, applicationID, user.ID)
		if err != nil {
			return err
		}

		result = &ChangeApplicationDatesResult{
			OldPeriod:      formatApplicationPeriods(oldPeriods),
			NewPeriod:      formatApplicationPeriod(period),
			ApprovalsReset: approvalsReset,
		}
		if err := s.recorder.Record(ctx, tx, models.AuditEntityApplication, &applicationID,
			models.AuditActionDatesChanged, &user.ID, applicationAuditDetails{
				OldValue: &result.OldPeriod, NewValue: &result.NewPeriod, Comment: &reason,
			}); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка записи в историю заявки")
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	s.notifyApplicationUpdated(ctx, applicationID, archiveDataChanged)
	s.notifyDatesChanged(ctx, applicationID, applicationNumber, user.ID, *result, reason)
	return result, nil
}

// parseApplicationPeriod проверяет окно так же, как форма подачи: даты и время по
// московским часам, конец не раньше начала и ещё не наступил.
func parseApplicationPeriod(req ChangeApplicationDatesRequest, now time.Time) (applicationPeriod, error) {
	dateFrom, errFrom := time.Parse("2006-01-02", strings.TrimSpace(req.EntryDateFrom))
	dateTo, errTo := time.Parse("2006-01-02", strings.TrimSpace(req.EntryDateTo))
	if errFrom != nil || errTo != nil {
		return applicationPeriod{}, echo.NewHTTPError(http.StatusBadRequest, "Даты указаны в неверном формате")
	}
	timeFrom, okFrom := parseClock(req.EntryTimeFrom)
	timeTo, okTo := parseClock(req.EntryTimeTo)
	if !okFrom || !okTo {
		return applicationPeriod{}, echo.NewHTTPError(http.StatusBadRequest, "Время указано в неверном формате")
	}

	start := dateFrom.Add(timeFrom)
	end := dateTo.Add(timeTo)
	if dateTo.Before(dateFrom) {
		return applicationPeriod{}, echo.NewHTTPError(http.StatusBadRequest, "Дата окончания не может быть раньше даты начала")
	}
	if !end.After(start) {
		return applicationPeriod{}, echo.NewHTTPError(http.StatusBadRequest, "Время окончания должно быть позже времени начала")
	}
	// Строки срока не несут зоны и вводятся по часам бюро, поэтому «сейчас» переводим
	// в Москву и сравниваем как настенное время.
	moscowNow := now.In(moscowWorkModeLoc)
	nowWall := time.Date(moscowNow.Year(), moscowNow.Month(), moscowNow.Day(),
		moscowNow.Hour(), moscowNow.Minute(), moscowNow.Second(), 0, time.UTC)
	if !end.After(nowWall) {
		return applicationPeriod{}, echo.NewHTTPError(http.StatusBadRequest, "Срок уже истёк: укажите окончание в будущем")
	}

	return applicationPeriod{
		DateFrom: dateFrom.Format("2006-01-02"),
		DateTo:   dateTo.Format("2006-01-02"),
		TimeFrom: formatClock(timeFrom),
		TimeTo:   formatClock(timeTo),
	}, nil
}

// parseClock разбирает ЧЧ:ММ или ЧЧ:ММ:СС в смещение от начала суток.
func parseClock(raw string) (time.Duration, bool) {
	raw = strings.TrimSpace(raw)
	for _, layout := range []string{"15:04:05", "15:04"} {
		if t, err := time.Parse(layout, raw); err == nil {
			return time.Duration(t.Hour())*time.Hour + time.Duration(t.Minute())*time.Minute +
				time.Duration(t.Second())*time.Second, true
		}
	}
	return 0, false
}

// formatClock - время в формате хранения, как пишет форма подачи (ЧЧ:ММ:СС).
func formatClock(d time.Duration) string {
	return time.Date(0, 1, 1, 0, 0, 0, 0, time.UTC).Add(d).Format("15:04:05")
}

// lockApplicationForDatesChange берёт заявку под FOR UPDATE и проверяет, что срок ещё
// можно менять. Лок нужен против встречного принятия в работу или последнего голоса:
// иначе срок сдвинулся бы у заявки, которая в ту же секунду стала обещанием охране.
func lockApplicationForDatesChange(tx *gorm.DB, applicationID int) (string, error) {
	var app struct {
		Status            *string
		Confirmation      *string
		ApplicationNumber *string
	}
	res := tx.Raw(`SELECT status, confirmation, application_number FROM applications WHERE id = ? FOR UPDATE`,
		applicationID).Scan(&app)
	if res.Error != nil {
		slog.Error("срок заявки: не удалось прочитать заявку", "application_id", applicationID, "error", res.Error)
		return "", echo.NewHTTPError(http.StatusInternalServerError, "Ошибка чтения заявки")
	}
	if res.RowsAffected == 0 {
		return "", echo.NewHTTPError(http.StatusNotFound, "Заявка не найдена")
	}

	status := derefStr(app.Status)
	if !containsString(applicationDatesEditableStatuses, status) {
		return "", echo.NewHTTPError(http.StatusBadRequest,
			fmt.Sprintf("Заявка в статусе «%s»: срок менять нельзя", status))
	}
	if confirmation := derefStr(app.Confirmation); confirmation != "" && confirmation != models.ConfirmationPending {
		return "", echo.NewHTTPError(http.StatusBadRequest,
			fmt.Sprintf("По заявке уже есть итог согласования («%s»): срок менять нельзя", confirmation))
	}

	number := derefStr(app.ApplicationNumber)
	if number == "" {
		number = fmt.Sprintf("№ %d", applicationID)
	}
	return number, nil
}

// applicationAttachmentPeriods - окна всех вложений заявки в порядке создания.
func applicationAttachmentPeriods(tx *gorm.DB, applicationID int) ([]applicationPeriod, error) {
	var rows []struct {
		EntryDateFrom *string
		EntryDateTo   *string
		EntryTimeFrom *string
		EntryTimeTo   *string
	}
	if err := tx.Raw(`
		SELECT entry_date_from, entry_date_to, entry_time_from, entry_time_to
		FROM attachments WHERE application_id = ? ORDER BY id
	`, applicationID).Scan(&rows).Error; err != nil {
		slog.Error("срок заявки: не удалось прочитать вложения", "application_id", applicationID, "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка чтения вложений заявки")
	}
	periods := make([]applicationPeriod, 0, len(rows))
	for _, r := range rows {
		periods = append(periods, applicationPeriod{
			DateFrom: derefStr(r.EntryDateFrom), DateTo: derefStr(r.EntryDateTo),
			TimeFrom: derefStr(r.EntryTimeFrom), TimeTo: derefStr(r.EntryTimeTo),
		})
	}
	return periods, nil
}

// applicationPeriodUnchanged - у всех вложений и всех машин уже стоит это окно.
// Машины проверяем отдельно: у них бывает свой срок, и правка, выравнивающая только
// их, тоже правка.
func applicationPeriodUnchanged(tx *gorm.DB, applicationID int, attachments []applicationPeriod, p applicationPeriod) (bool, error) {
	for _, a := range attachments {
		if a != p {
			return false, nil
		}
	}
	var differentCars int64
	if err := tx.Raw(`
		SELECT COUNT(*) FROM cars c
		JOIN attachments a ON a.id = c.attachment_id
		WHERE a.application_id = ?
		  AND (c.entry_date_from IS DISTINCT FROM ? OR c.entry_date_to IS DISTINCT FROM ?
		    OR c.entry_time_from IS DISTINCT FROM ? OR c.entry_time_to IS DISTINCT FROM ?)
	`, applicationID, p.DateFrom, p.DateTo, p.TimeFrom, p.TimeTo).Scan(&differentCars).Error; err != nil {
		slog.Error("срок заявки: не удалось сверить сроки машин", "application_id", applicationID, "error", err)
		return false, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка чтения машин заявки")
	}
	return differentCars == 0, nil
}

// checkByFactPeriodOnChange держит правило машины «По факту» (#2320) и на этом пути:
// иначе заявку на сутки подают, а принимающий растягивает её на месяц, и правило
// «обхода для бюро нет» обходится ровно бюро.
func checkByFactPeriodOnChange(tx *gorm.DB, applicationID int, p applicationPeriod, now time.Time) error {
	var byFact int64
	if err := tx.Raw(`
		SELECT COUNT(*) FROM cars c
		JOIN attachments a ON a.id = c.attachment_id
		WHERE a.application_id = ?
		  AND c.date_removed IS NULL
		  AND LOWER(REPLACE(TRIM(c.car_number), ' ', '')) = ?
	`, applicationID, byFactCompactPlate()).Scan(&byFact).Error; err != nil {
		slog.Error("срок заявки: не удалось проверить машины «По факту»", "application_id", applicationID, "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка чтения машин заявки")
	}
	if byFact == 0 {
		return nil
	}
	if p.DateTo > ByFactMaxDate(now) {
		return echo.NewHTTPError(http.StatusBadRequest,
			fmt.Sprintf("В заявке машина «По факту», указана дата окончания %s. %s",
				formatRuDate(p.DateTo), byFactDeadlineHint(now)))
	}
	return nil
}

// writeApplicationPeriod пишет окно во все вложения заявки и во все её машины.
func writeApplicationPeriod(tx *gorm.DB, applicationID int, p applicationPeriod) error {
	now := time.Now().UTC()
	if err := tx.Exec(`
		UPDATE attachments
		SET entry_date_from = ?, entry_date_to = ?, entry_time_from = ?, entry_time_to = ?, updated_at = ?
		WHERE application_id = ?
	`, p.DateFrom, p.DateTo, p.TimeFrom, p.TimeTo, now, applicationID).Error; err != nil {
		slog.Error("срок заявки: не удалось обновить вложения", "application_id", applicationID, "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка изменения срока вложений")
	}
	if err := tx.Exec(`
		UPDATE cars c
		SET entry_date_from = ?, entry_date_to = ?, entry_time_from = ?, entry_time_to = ?, updated_at = ?
		FROM attachments a
		WHERE a.id = c.attachment_id AND a.application_id = ?
	`, p.DateFrom, p.DateTo, p.TimeFrom, p.TimeTo, now, applicationID).Error; err != nil {
		slog.Error("срок заявки: не удалось обновить машины", "application_id", applicationID, "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка изменения срока машин")
	}
	return nil
}

// resetApprovalsIfVoted сбрасывает круг согласования, только если в нём уже есть голос:
// без голосов сбрасывать нечего, а лишняя запись «статус согласования изменился» в
// истории читалась бы как событие.
func (s *applicationService) resetApprovalsIfVoted(ctx context.Context, tx *gorm.DB, applicationID, actorID int) (bool, error) {
	var voted int64
	if err := tx.Raw(`
		SELECT COUNT(*) FROM application_responsible_users
		WHERE application_id = ? AND approval_status IS NOT NULL AND approval_status <> 'pending'
	`, applicationID).Scan(&voted).Error; err != nil {
		slog.Error("срок заявки: не удалось прочитать голоса", "application_id", applicationID, "error", err)
		return false, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка чтения согласования")
	}
	if voted == 0 {
		return false, nil
	}
	if err := s.resetApprovalRound(ctx, tx, applicationID, actorID); err != nil {
		return false, err
	}
	return true, nil
}

// formatApplicationPeriod - окно для человека: «01.10.2026 09:00 - 03.10.2026 18:00».
func formatApplicationPeriod(p applicationPeriod) string {
	side := func(date, clock string) string {
		text := formatRuDate(date)
		if len(clock) >= 5 {
			text += " " + clock[:5]
		}
		return strings.TrimSpace(text)
	}
	return side(p.DateFrom, p.TimeFrom) + " - " + side(p.DateTo, p.TimeTo)
}

// formatApplicationPeriods - прежние окна вложений без повторов; у многих заявок оно одно.
func formatApplicationPeriods(periods []applicationPeriod) string {
	seen := make(map[applicationPeriod]bool, len(periods))
	parts := make([]string, 0, len(periods))
	for _, p := range periods {
		if seen[p] {
			continue
		}
		seen[p] = true
		parts = append(parts, formatApplicationPeriod(p))
	}
	return strings.Join(parts, "; ")
}

// notifyDatesChanged сообщает о новом сроке всем, кому видна заявка, кроме автора правки:
// заявителю срок пропуска, согласующим - что одобряли другое окно, остальным принимающим -
// что заявку уже трогали. Best-effort после commit: сбой уведомления правку не отменяет.
func (s *applicationService) notifyDatesChanged(ctx context.Context, applicationID int, number string, actorID int, result ChangeApplicationDatesResult, reason string) {
	if s.notificationService == nil {
		return
	}

	message := fmt.Sprintf("Принимающий изменил срок заявки %s: было %s, стало %s. Причина: %s",
		number, result.OldPeriod, result.NewPeriod, reason)
	if result.ApprovalsReset {
		message += " Голоса согласующих сброшены, заявка снова на согласовании."
	}
	payload, _ := json.Marshal(map[string]any{
		"application_id":     applicationID,
		"application_number": number,
		"old_period":         result.OldPeriod,
		"new_period":         result.NewPeriod,
		"approvals_reset":    result.ApprovalsReset,
	})
	payloadStr := string(payload)

	for _, userID := range s.applicationParticipants(ctx, applicationID) {
		if userID == actorID {
			continue
		}
		if err := s.notificationService.CreateForUser(ctx, userID, NotificationTypeApplicationDatesChanged,
			"Изменён срок заявки", message, &payloadStr); err != nil {
			slog.Warn("срок заявки: уведомление не создано", "user_id", userID,
				"application_id", applicationID, "err", err)
		}
	}
}
