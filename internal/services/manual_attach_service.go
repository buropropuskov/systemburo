package services

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ManualAttachService interface {
	AttachToApplication(ctx context.Context, orphanAttID int, req AttachToApplicationRequest, userID int) (*AttachToApplicationResponse, error)
}

// The original application/target XOR remains unchanged. Only a previously
// unbounded manual window requires an explicit finite-period choice.
type AttachToApplicationRequest struct {
	ApplicationID      *int               `json:"application_id"`
	TargetAttachmentID *int               `json:"target_attachment_id"`
	PeriodChoice       string             `json:"period_choice,omitempty"`
	SourceAttachmentID *int               `json:"source_attachment_id,omitempty"`
	Period             *EntityPeriodInput `json:"period,omitempty"`
}

type AttachToApplicationResponse struct {
	Success       bool   `json:"success"`
	Message       string `json:"message"`
	ApplicationID int    `json:"application_id"`
	AttachmentID  int    `json:"attachment_id"`
}

type manualAttachService struct {
	db                *gorm.DB
	recorder          AuditRecorder
	tablesProducer    *TablesRefreshPublisher
	availableProducer *AvailableRefreshPublisher
	afterAttach       func(context.Context, int)
}

func NewManualAttachService(db *gorm.DB, recorder AuditRecorder, tablesProducer *TablesRefreshPublisher, availableProducer *AvailableRefreshPublisher) ManualAttachService {
	return &manualAttachService{db: db, recorder: recorder, tablesProducer: tablesProducer, availableProducer: availableProducer}
}

func (s *manualAttachService) AttachToApplication(ctx context.Context, orphanAttID int, req AttachToApplicationRequest, userID int) (*AttachToApplicationResponse, error) {
	if err := validateManualAttachRequest(orphanAttID, req); err != nil {
		return nil, err
	}
	if userID <= 0 {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "Требуется авторизация")
	}
	var appID int
	var attachmentType string
	var carIDs, empIDs []int
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Fresh transaction-local grants: no cached admin/account state and no
		// role bypass of personal denies. The existing router uses this key too.
		permissions, err := NewPermissionResolver(tx).Resolve(ctx, userID)
		if err != nil {
			return err
		}
		if !permissions.Has(KeyPageAdmin) {
			return echo.NewHTTPError(http.StatusForbidden, "Недостаточно прав")
		}
		var initial models.Attachment
		if err := tx.First(&initial, orphanAttID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return echo.NewHTTPError(http.StatusNotFound, "Вложение не найдено")
			}
			return err
		}
		if !initial.IsManual || initial.ApplicationID != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "Вложение не является ручным или уже привязано к заявке")
		}
		if req.ApplicationID != nil {
			appID = *req.ApplicationID
		} else {
			var locator models.Attachment
			if err := tx.First(&locator, *req.TargetAttachmentID).Error; err != nil {
				if err != gorm.ErrRecordNotFound {
					return err
				}
				return echo.NewHTTPError(http.StatusUnprocessableEntity, "Целевое вложение не найдено")
			}
			if locator.ApplicationID == nil {
				return echo.NewHTTPError(http.StatusUnprocessableEntity, "Целевое вложение не принадлежит заявке")
			}
			appID = *locator.ApplicationID
		}
		// Stable writer/cron order: application -> all attachments by ID ->
		// cars by ID -> employees by ID. Revalidate every locator under locks.
		var app models.Application
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&app, appID).Error; err != nil {
			if err != gorm.ErrRecordNotFound {
				return err
			}
			return echo.NewHTTPError(http.StatusUnprocessableEntity, "Заявка не найдена")
		}
		if app.Confirmation == nil || *app.Confirmation != models.ConfirmationApproved || app.Status == nil || *app.Status != models.StatusInWork {
			return echo.NewHTTPError(http.StatusUnprocessableEntity, "Привязка возможна только к активной согласованной заявке")
		}
		ids := []int{orphanAttID}
		if req.TargetAttachmentID != nil {
			ids = append(ids, *req.TargetAttachmentID)
		}
		if req.SourceAttachmentID != nil {
			ids = append(ids, *req.SourceAttachmentID)
		}
		slices.Sort(ids)
		ids = slices.Compact(ids)
		var attachments []models.Attachment
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id IN ?", ids).Order("id").Find(&attachments).Error; err != nil {
			return err
		}
		locked := make(map[int]models.Attachment, len(attachments))
		for _, attachment := range attachments {
			locked[attachment.ID] = attachment
		}
		orphan, ok := locked[orphanAttID]
		if !ok || !orphan.IsManual || orphan.ApplicationID != nil {
			return echo.NewHTTPError(http.StatusConflict, "Вложение уже изменено другим запросом")
		}
		if orphan.AttachmentType != "cars" && orphan.AttachmentType != "people" {
			return echo.NewHTTPError(http.StatusUnprocessableEntity, "Неподдерживаемый тип вложения")
		}
		if orphan.Status == nil || *orphan.Status != 1 {
			return echo.NewHTTPError(http.StatusUnprocessableEntity, "Ручное вложение неактивно")
		}
		attachmentType = orphan.AttachmentType
		var target *models.Attachment
		if req.TargetAttachmentID != nil {
			value, found := locked[*req.TargetAttachmentID]
			if !found || value.ApplicationID == nil || *value.ApplicationID != appID || value.IsManual {
				return echo.NewHTTPError(http.StatusConflict, "Целевое вложение изменилось")
			}
			if value.AttachmentType != orphan.AttachmentType || value.Status == nil || *value.Status != 1 {
				return echo.NewHTTPError(http.StatusUnprocessableEntity, "Тип или состояние целевого вложения не подходит")
			}
			target = &value
		}
		var source *models.Attachment
		if req.SourceAttachmentID != nil {
			value, found := locked[*req.SourceAttachmentID]
			if !found || value.ApplicationID == nil || *value.ApplicationID != appID || value.IsManual || value.Status == nil || *value.Status != 1 {
				return echo.NewHTTPError(http.StatusUnprocessableEntity, "Источник срока должен быть активным вложением выбранной заявки")
			}
			source = &value
		}
		oldParent := manualAttachWindow(orphan)
		needsChoice := strVal(oldParent.EntryDateTo) == ""
		if needsChoice && !permissions.Has(KeyDetailPeriodChange) {
			return echo.NewHTTPError(http.StatusForbidden, "Недостаточно прав для назначения срока")
		}
		now := time.Now()
		chosen, err := manualAttachChosenWindow(req, needsChoice, source, now)
		if err != nil {
			return err
		}
		newParent := oldParent
		if target != nil {
			newParent = manualAttachWindow(*target)
		} else if needsChoice {
			newParent = chosen
		}
		var cars []models.Car
		var employees []models.Employee
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("attachment_id = ?", orphan.ID).Order("id").Find(&cars).Error; err != nil {
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("attachment_id = ?", orphan.ID).Order("id").Find(&employees).Error; err != nil {
			return err
		}
		if (attachmentType == "cars" && len(employees) > 0) || (attachmentType == "people" && len(cars) > 0) {
			return echo.NewHTTPError(http.StatusUnprocessableEntity, "Состав ручного вложения не соответствует типу")
		}
		permissions, err = NewPermissionResolver(tx).Resolve(ctx, userID)
		if err != nil {
			return err
		}
		if !permissions.Has(KeyPageAdmin) || (needsChoice && !permissions.Has(KeyDetailPeriodChange)) {
			return echo.NewHTTPError(http.StatusForbidden, "Недостаточно прав")
		}
		// Validate the entire car batch before moving or updating any row.
		for _, car := range cars {
			own := models.EntryPeriod{EntryDateFrom: car.EntryDateFrom, EntryDateTo: car.EntryDateTo, EntryTimeFrom: car.EntryTimeFrom, EntryTimeTo: car.EntryTimeTo}
			nextMode, nextOwn, _, err := manualAttachNextPeriod(car.PeriodMode, own, oldParent, newParent, chosen, req, needsChoice, target == nil)
			if err != nil {
				return err
			}
			if err := manualAttachValidateByFact(strVal(car.CarNumber), car.PeriodMode, own, oldParent, nextMode, nextOwn, newParent, now); err != nil {
				return err
			}
		}
		apply := func(table string, id int, mode models.PeriodMode, own models.EntryPeriod) error {
			nextMode, nextOwn, periodChanged, err := manualAttachNextPeriod(mode, own, oldParent, newParent, chosen, req, needsChoice, target == nil)
			if err != nil {
				return err
			}
			updates := map[string]any{}
			if target != nil {
				updates["attachment_id"] = target.ID
			}
			if periodChanged {
				updates["period_mode"] = nextMode
				updates["entry_date_from"], updates["entry_date_to"] = nextOwn.EntryDateFrom, nextOwn.EntryDateTo
				updates["entry_time_from"], updates["entry_time_to"] = nextOwn.EntryTimeFrom, nextOwn.EntryTimeTo
			}
			if len(updates) > 0 {
				updates["updated_at"] = canonicalEntityPeriodTime(now)
				write := tx.Table(table).Where("id = ? AND attachment_id = ?", id, orphan.ID).Updates(updates)
				if write.Error != nil {
					return write.Error
				}
				if write.RowsAffected != 1 {
					return echo.NewHTTPError(http.StatusConflict, "Состав вложения изменился")
				}
			}
			if needsChoice && mode != models.PeriodIndividual {
				entityType := models.AuditEntityEmployee
				if table == "cars" {
					entityType = models.AuditEntityCar
				}
				comment := "При привязке назначен конечный срок ручного допуска"
				details := struct {
					Comment            *string            `json:"comment"`
					PeriodChoice       string             `json:"period_choice"`
					SourceAttachmentID *int               `json:"source_attachment_id,omitempty"`
					OldPeriod          models.EntryPeriod `json:"old_period"`
					NewPeriod          models.EntryPeriod `json:"new_period"`
				}{&comment, req.PeriodChoice, req.SourceAttachmentID, oldParent, chosen}
				if err := s.recorder.Record(ctx, tx, entityType, &id, models.AuditActionDatesChanged, &userID, details); err != nil {
					return err
				}
			}
			return nil
		}
		for _, car := range cars {
			own := models.EntryPeriod{EntryDateFrom: car.EntryDateFrom, EntryDateTo: car.EntryDateTo, EntryTimeFrom: car.EntryTimeFrom, EntryTimeTo: car.EntryTimeTo}
			if err := apply("cars", car.ID, car.PeriodMode, own); err != nil {
				return err
			}
			carIDs = append(carIDs, car.ID)
		}
		for _, employee := range employees {
			own := models.EntryPeriod{EntryDateFrom: employee.EntryDateFrom, EntryDateTo: employee.EntryDateTo, EntryTimeFrom: employee.EntryTimeFrom, EntryTimeTo: employee.EntryTimeTo}
			if err := apply("employees", employee.ID, employee.PeriodMode, own); err != nil {
				return err
			}
			empIDs = append(empIDs, employee.ID)
		}
		if target == nil {
			updates := map[string]any{"application_id": appID, "organization_id": nil, "company_id": nil, "is_manual": false}
			if needsChoice {
				updates["entry_date_from"], updates["entry_date_to"] = chosen.EntryDateFrom, chosen.EntryDateTo
				updates["entry_time_from"], updates["entry_time_to"] = chosen.EntryTimeFrom, chosen.EntryTimeTo
			}
			if err := tx.Model(&models.Attachment{}).Where("id = ?", orphan.ID).Updates(updates).Error; err != nil {
				return err
			}
		} else {
			if attachmentType == "cars" {
				if err := tx.Exec(`INSERT INTO attachment_unload_places (attachment_id, unload_place_id, order_index, created_at)
					SELECT ?, unload_place_id, order_index, NOW() FROM attachment_unload_places WHERE attachment_id = ?
					ON CONFLICT (attachment_id, unload_place_id) DO NOTHING`, target.ID, orphan.ID).Error; err != nil {
					return err
				}
			}
			if err := tx.Delete(&models.Attachment{}, orphan.ID).Error; err != nil {
				return err
			}
		}
		return s.recordAttach(ctx, tx, attachmentType, carIDs, empIDs, app.ApplicationNumber, userID)
	})
	if err != nil {
		return nil, err
	}
	s.notifyChanged(ctx, attachmentType, carIDs, empIDs)
	if s.afterAttach != nil {
		s.afterAttach(ctx, appID)
	}
	slog.Info("ручное вложение привязано к заявке", "attachment_id", orphanAttID, "application_id", appID, "user_id", userID)
	return &AttachToApplicationResponse{Success: true, Message: "Attachment linked to application", ApplicationID: appID, AttachmentID: orphanAttID}, nil
}

func validateManualAttachRequest(orphanID int, req AttachToApplicationRequest) error {
	if orphanID <= 0 || (req.ApplicationID == nil) == (req.TargetAttachmentID == nil) {
		return echo.NewHTTPError(http.StatusBadRequest, "Укажите либо application_id, либо target_attachment_id")
	}
	if (req.ApplicationID != nil && *req.ApplicationID <= 0) || (req.TargetAttachmentID != nil && (*req.TargetAttachmentID <= 0 || *req.TargetAttachmentID == orphanID)) {
		return echo.NewHTTPError(http.StatusBadRequest, "Некорректная заявка или целевое вложение")
	}
	switch req.PeriodChoice {
	case "":
		if req.SourceAttachmentID == nil && req.Period == nil {
			return nil
		}
	case "source":
		if req.SourceAttachmentID != nil && *req.SourceAttachmentID > 0 && *req.SourceAttachmentID != orphanID && req.Period == nil {
			return nil
		}
	case "individual":
		if req.SourceAttachmentID == nil && req.Period != nil {
			return nil
		}
	}
	return echo.NewHTTPError(http.StatusBadRequest, "Выберите источник срока или задайте индивидуальный срок")
}

func manualAttachWindow(attachment models.Attachment) models.EntryPeriod {
	return models.EntryPeriod{EntryDateFrom: attachment.EntryDateFrom, EntryDateTo: attachment.EntryDateTo, EntryTimeFrom: attachment.EntryTimeFrom, EntryTimeTo: attachment.EntryTimeTo}
}

func manualAttachChosenWindow(req AttachToApplicationRequest, needed bool, source *models.Attachment, now time.Time) (models.EntryPeriod, error) {
	if !needed {
		if req.PeriodChoice != "" {
			return models.EntryPeriod{}, echo.NewHTTPError(http.StatusBadRequest, "Конечный срок сохраняется при привязке")
		}
		return models.EntryPeriod{}, nil
	}
	var input EntityPeriodInput
	switch req.PeriodChoice {
	case "source":
		if source == nil {
			return models.EntryPeriod{}, echo.NewHTTPError(http.StatusBadRequest, "Выберите вложение-источник срока")
		}
		input = EntityPeriodInput{EntryDateFrom: strVal(source.EntryDateFrom), EntryDateTo: strVal(source.EntryDateTo), EntryTimeFrom: strVal(source.EntryTimeFrom), EntryTimeTo: strVal(source.EntryTimeTo)}
		if input.EntryTimeFrom == "" {
			input.EntryTimeFrom = "00:00:00"
		}
		if input.EntryTimeTo == "" {
			input.EntryTimeTo = "23:59:59"
		}
	case "individual":
		if req.Period == nil {
			return models.EntryPeriod{}, echo.NewHTTPError(http.StatusBadRequest, "Задайте индивидуальный срок")
		}
		input = *req.Period
	default:
		return models.EntryPeriod{}, echo.NewHTTPError(http.StatusBadRequest, "Для бессрочного допуска явно выберите источник срока или задайте индивидуальный срок")
	}
	period, err := parseApplicationPeriod(ChangeApplicationDatesRequest{EntryDateFrom: input.EntryDateFrom, EntryDateTo: input.EntryDateTo, EntryTimeFrom: input.EntryTimeFrom, EntryTimeTo: input.EntryTimeTo}, now)
	if err != nil {
		return models.EntryPeriod{}, err
	}
	return models.EntryPeriod{EntryDateFrom: &period.DateFrom, EntryDateTo: &period.DateTo, EntryTimeFrom: &period.TimeFrom, EntryTimeTo: &period.TimeTo}, nil
}

func manualAttachNextPeriod(mode models.PeriodMode, own, oldParent, newParent, chosen models.EntryPeriod, req AttachToApplicationRequest, unbounded, adopt bool) (models.PeriodMode, models.EntryPeriod, bool, error) {
	if _, err := models.ResolveEntityPeriod(mode, own, oldParent, true); err != nil {
		return mode, own, false, echo.NewHTTPError(http.StatusUnprocessableEntity, "Некорректный режим или срок ручной записи")
	}
	if mode == models.PeriodIndividual {
		return mode, own, false, nil
	}
	if unbounded {
		if models.ValidateStoredIndividualPeriod(chosen) != nil {
			return mode, own, false, echo.NewHTTPError(http.StatusBadRequest, "Не задан конечный срок")
		}
		// An explicit individual choice stays individual even if its values
		// equal the target. Source==target is the only reattach inherit choice.
		if req.PeriodChoice == "source" && (adopt || (req.SourceAttachmentID != nil && req.TargetAttachmentID != nil && *req.SourceAttachmentID == *req.TargetAttachmentID)) {
			return models.PeriodInherit, models.EntryPeriod{}, true, nil
		}
		return models.PeriodIndividual, chosen, true, nil
	}
	if !adopt && !manualAttachWindowWithin(oldParent, newParent) {
		return mode, own, false, echo.NewHTTPError(http.StatusUnprocessableEntity, "Период ручного допуска выходит за окно вложения заявки")
	}
	if !adopt {
		return models.PeriodIndividual, oldParent, true, nil
	}
	return mode, own, false, nil
}

func manualAttachValidateByFact(plate string, oldMode models.PeriodMode, oldOwn, oldParent models.EntryPeriod, nextMode models.PeriodMode, nextOwn, newParent models.EntryPeriod, now time.Time) error {
	if !isByFactPlate(plate) {
		return nil
	}
	previous, err := models.ResolveEntityPeriod(oldMode, oldOwn, oldParent, true)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnprocessableEntity, "Некорректный срок ручной записи")
	}
	next, err := models.ResolveEntityPeriod(nextMode, nextOwn, newParent, false)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnprocessableEntity, "Некорректный срок ручной записи")
	}
	if previous.Bounded == next.Bounded && equalEntityPeriod(previous.EntryPeriod, next.EntryPeriod) {
		return nil
	}
	if !next.Bounded || strVal(next.EntryDateTo) > ByFactMaxDate(now) {
		return echo.NewHTTPError(http.StatusBadRequest, byFactDeadlineHint(now))
	}
	return nil
}

func manualAttachWindowWithin(inner, outer models.EntryPeriod) bool {
	if models.ValidateStoredIndividualPeriod(inner) != nil {
		return false
	}
	// Preserve the old target's open-boundary semantics. Blank target bounds
	// are unlimited, not a new invalid period. Finite bounds include their clocks.
	bound := func(date, clock *string, fallback string) (time.Time, bool) {
		day, err := time.Parse("2006-01-02", strVal(date))
		if err != nil {
			return time.Time{}, false
		}
		value := strVal(clock)
		if value == "" {
			value = fallback
		}
		duration, ok := parseClock(value)
		return day.Add(duration), ok
	}
	start, startOK := bound(inner.EntryDateFrom, inner.EntryTimeFrom, "00:00:00")
	end, endOK := bound(inner.EntryDateTo, inner.EntryTimeTo, "23:59:59")
	if !startOK || !endOK {
		return false
	}
	if strVal(outer.EntryDateFrom) != "" {
		outerStart, ok := bound(outer.EntryDateFrom, outer.EntryTimeFrom, "00:00:00")
		if !ok || start.Before(outerStart) {
			return false
		}
	}
	if strVal(outer.EntryDateTo) != "" {
		outerEnd, ok := bound(outer.EntryDateTo, outer.EntryTimeTo, "23:59:59")
		if !ok || end.After(outerEnd) {
			return false
		}
	}
	return true
}

// recordAttach пишет запись истории "Привязано к заявке N" на каждую перевешенную сущность.
// Действие "update" (существующий словарь FE getActionText), суть - в комментарии.
func (s *manualAttachService) recordAttach(ctx context.Context, tx *gorm.DB, attType string, carIDs, empIDs []int, appNumber *string, userID int) error {
	num := ""
	if appNumber != nil {
		num = *appNumber
	}
	comment := strings.TrimSpace(fmt.Sprintf("Привязано к заявке %s", num))

	if attType == "cars" {
		for i := range carIDs {
			id := carIDs[i]
			if err := s.recorder.Record(ctx, tx, models.AuditEntityCar, &id, "update", &userID, carAuditDetails{Comment: &comment}); err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, "Error recording car history")
			}
		}
	}
	if attType == "people" {
		for i := range empIDs {
			id := empIDs[i]
			if err := s.recorder.Record(ctx, tx, models.AuditEntityEmployee, &id, "update", &userID, carAuditDetails{Comment: &comment}); err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, "Error recording employee history")
			}
		}
	}
	return nil
}

// notifyChanged шлёт real-time обновления (таблицы проходной + "Доступные мне") по сменившимся
// сущностям. Best-effort: nil-продюсеры допустимы (тесты/окружение без событий).
func (s *manualAttachService) notifyChanged(ctx context.Context, attType string, carIDs, empIDs []int) {
	if s.tablesProducer != nil {
		switch attType {
		case "cars":
			s.tablesProducer.NotifyCarsChangedBatch(ctx, carIDs)
		case "people":
			s.tablesProducer.NotifyEmployeesChangedBatch(ctx, empIDs)
		}
	}
	if s.availableProducer != nil {
		s.availableProducer.NotifyAvailableChanged(ctx)
	}
}

// lockManualOrphan блокирует строку вложения-сироты (SELECT ... FOR UPDATE) внутри транзакции
// и подтверждает инвариант «is_manual И application_id IS NULL». Сериализует конкурентные
// привязки одного orphan: второй вызов дожидается lock и видит уже привязанную/удалённую
// сироту -> 409 вместо тихого last-write-wins.
func lockManualOrphan(tx *gorm.DB, orphanID int) error {
	var locked models.Attachment
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&locked, orphanID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return echo.NewHTTPError(http.StatusConflict, "Вложение уже изменено другим запросом")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Error locking attachment")
	}
	if !locked.IsManual || locked.ApplicationID != nil {
		return echo.NewHTTPError(http.StatusConflict, "Вложение уже привязано к заявке")
	}
	return nil
}

// dateRangeWithin проверяет, что диапазон дат сущности [entityFrom, entityTo] вложен в диапазон
// вложения [containerFrom, containerTo]. Даты - строки формата YYYY-MM-DD, где лексикографический
// порядок совпадает с хронологическим (тот же инвариант, что у scoped-запросов с CURRENT_DATE
// BETWEEN). Открытый край НЕ нарушает вложенность: пустая граница вложения = «без ограничения»,
// пустая граница сущности = «наследует окно вложения» (scoped-показ гейтит машину по датам
// ВЛОЖЕНИЯ, не сущности) - обе трактовки корректно дают «вложено».
func dateRangeWithin(entityFrom, entityTo, containerFrom, containerTo *string) bool {
	ef, et := strVal(entityFrom), strVal(entityTo)
	cf, ct := strVal(containerFrom), strVal(containerTo)
	if ef != "" && cf != "" && ef < cf {
		return false
	}
	if et != "" && ct != "" && et > ct {
		return false
	}
	return true
}

func strVal(p *string) string {
	if p == nil {
		return ""
	}
	return strings.TrimSpace(*p)
}
