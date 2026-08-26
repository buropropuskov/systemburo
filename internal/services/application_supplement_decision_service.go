package services

// Решение по раунду дополнения (#1685, срез S4): принятие и отказ принимающего, снятие
// раунда автором.
//
// Принятие - единственная операция дополнения, которая что-то выпускает на КПП, и потому
// единственная, где важно, КАКИЕ строки поднимаются. Активируется только состав своего
// раунда: строки прошлых раундов и исходной подачи уже стоят со status = 1, и повторный
// проход по вложению сплодил бы им вторую запись «Добавлен в таблицу проходной».
//
// applications.confirmation и applications.status не двигает ни одна ветка этого файла -
// как и в голосовании. На этой паре держится допуск уже выданных пропусков.

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

const (
	supplementActionAccept = "accept"
	supplementActionReject = "reject"
)

// Тип уведомления -- NotificationTypeApplicationSupplementDecided (каталог,
// notification_catalog.go): автору об исходе его дополнения. Отдельно от
// NotificationTypeApplicationStatusChanged: статус заявки при этом не меняется, и общий
// текст «Ваша заявка ...» ввёл бы автора в заблуждение.

// SupplementDecisionRequest - решение принимающего по согласованному раунду.
type SupplementDecisionRequest struct {
	Action  string  `json:"action" validate:"required,oneof=accept reject"`
	Comment *string `json:"comment"`
}

// SupplementCancelRequest - снятие раунда автором заявки.
type SupplementCancelRequest struct {
	Comment *string `json:"comment"`
}

// SupplementDecisionResponse - раунд после решения. Activated - сколько строк реально
// встало на КПП: оно меньше состава раунда, если часть строк успела уехать в корзину или
// в чёрный список, и по этой разнице видно, что принято не всё поданное.
type SupplementDecisionResponse struct {
	SupplementID int    `json:"supplement_id"`
	Number       int    `json:"number"`
	Status       string `json:"status"`
	Activated    int    `json:"activated"`
}

// DecideSupplement фиксирует решение принимающего по согласованному раунду дополнения.
func (s *applicationService) DecideSupplement(ctx context.Context, username string, applicationID, supplementID int, req SupplementDecisionRequest) (*SupplementDecisionResponse, error) {
	if req.Action != supplementActionAccept && req.Action != supplementActionReject {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Недопустимое решение: ожидается accept или reject")
	}
	user, err := s.getUserByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	// Принимающий - глобальная роль (application_approvers), как и у принятия самой заявки:
	// решение по добавке принимает тот же, кто принимал заявку.
	approver, err := s.isApprover(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	if !approver {
		return nil, echo.NewHTTPError(http.StatusForbidden, "Решение по дополнению принимает только принимающий")
	}
	if err := s.checkNotArchived(ctx, applicationID); err != nil {
		return nil, err
	}

	tx := s.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to start transaction")
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	round, err := s.lockSupplementRound(tx, applicationID, supplementID)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	if round.Status != models.SupplementApproved {
		tx.Rollback()
		return nil, echo.NewHTTPError(http.StatusConflict,
			"Решение принимают по согласованному дополнению - круг согласования по этому ещё не закрыт")
	}

	now := time.Now().UTC()
	comment := trimmedSupplementComment(req.Comment)
	newStatus := models.SupplementAccepted
	action := models.AuditActionSupplementAccepted
	if req.Action == supplementActionReject {
		newStatus = models.SupplementRefused
		action = models.AuditActionSupplementRefused
	}

	activated := 0
	if req.Action == supplementActionAccept {
		// Статус заявки перечитываем внутри транзакции: между гардом принимающего и
		// активацией заявку могли вывести из работы, и тогда её строки погашены, а состав
		// раунда встал бы на КПП поверх снятых пропусков.
		//
		// Отдельный FOR UPDATE здесь не нужен - строка заявки уже заблокирована
		// lockSupplementRound, он берёт её первой именно ради единого порядка захвата.
		var appStatus *string
		if err := tx.Raw("SELECT status FROM applications WHERE id = ?", applicationID).Scan(&appStatus).Error; err != nil {
			tx.Rollback()
			slog.Error("дополнение: не удалось перечитать статус заявки", "application_id", applicationID, "error", err)
			return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to load application")
		}
		current := ""
		if appStatus != nil {
			current = *appStatus
		}
		if current != models.StatusInWork {
			tx.Rollback()
			return nil, echo.NewHTTPError(http.StatusConflict,
				fmt.Sprintf("Заявка в статусе «%s» - принять дополнение нельзя", current))
		}

		activated, err = s.activateSupplementItems(ctx, tx, round.ID, &user.ID)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	if err := tx.Exec(`
		UPDATE application_supplements
		SET status = ?, decided_by_user_id = ?, decided_at = ?, decision_comment = ?
		WHERE id = ?
	`, newStatus, user.ID, now, comment, round.ID).Error; err != nil {
		tx.Rollback()
		slog.Error("дополнение: не удалось записать решение", "supplement_id", round.ID, "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to save supplement decision")
	}

	oldStatus := round.Status
	s.recorder.Log(ctx, tx, models.AuditEntityApplication, &applicationID, action, &user.ID,
		applicationAuditDetails{
			OldValue: &oldStatus,
			NewValue: &newStatus,
			Comment:  comment,
			Metadata: supplementAuditMetadata(round, map[string]any{"activated": activated}),
		})

	if err := s.bumpStatusUpdated(tx, applicationID, &user.ID); err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		slog.Error("дополнение: не удалось зафиксировать решение", "supplement_id", round.ID, "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to commit transaction")
	}

	slog.Info("решение по дополнению", "application_id", applicationID, "supplement_id", round.ID,
		"user_id", user.ID, "decision", req.Action, "activated", activated)

	if req.Action == supplementActionAccept {
		// Строки раунда встали в таблицы проходной - без этих сигналов охрана увидела бы
		// их только после F5, а состав вложения ушёл бы в бланк и слепок протухшим.
		s.tablesProducer.NotifyApplicationActivated(ctx, applicationID)
		s.availableProducer.NotifyAvailableChanged(ctx)
		s.notifyApplicationUpdated(ctx, applicationID, archiveDataChanged)
	} else {
		// Отказ допущенный состав не двигает: ни бланк, ни заявка.json от него не меняются.
		s.notifyApplicationUpdated(ctx, applicationID, archiveDataUnchanged)
	}
	s.notifySupplementDecided(ctx, applicationID, round, newStatus, &user.ID)

	return &SupplementDecisionResponse{
		SupplementID: round.ID,
		Number:       round.Number,
		Status:       newStatus,
		Activated:    activated,
	}, nil
}

// CancelSupplement снимает собственный незакрытый раунд по воле автора заявки.
func (s *applicationService) CancelSupplement(ctx context.Context, username string, applicationID, supplementID int, isSuperAdmin bool, req SupplementCancelRequest) (*SupplementDecisionResponse, error) {
	user, err := s.getUserByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	if err := s.checkNotArchived(ctx, applicationID); err != nil {
		return nil, err
	}

	// Владение проверяем ДО того, как трогаем раунд: иначе разные ответы на «не твоя заявка»
	// и «нет такого раунда» выдавали бы перебором id, у каких чужих заявок есть дополнения.
	var app struct{ SenderUserID int }
	res := s.db.WithContext(ctx).Raw("SELECT sender_user_id FROM applications WHERE id = ?", applicationID).Scan(&app)
	if res.Error != nil {
		slog.Error("дополнение: не удалось прочитать заявку", "application_id", applicationID, "error", res.Error)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to load application")
	}
	if res.RowsAffected == 0 {
		return nil, echo.NewHTTPError(http.StatusNotFound, "Application not found")
	}
	if !isSuperAdmin && app.SenderUserID != user.ID {
		return nil, echo.NewHTTPError(http.StatusForbidden, "Снять дополнение может только автор заявки")
	}

	tx := s.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to start transaction")
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	round, err := s.lockSupplementRound(tx, applicationID, supplementID)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	// Снимать можно только идущий раунд. Принятый снять нельзя тем более: его строки уже
	// на КПП, и «снятие» пришлось бы делать отзывом пропусков, а это другая операция.
	if !slices.Contains(models.OpenSupplementStatuses, round.Status) {
		tx.Rollback()
		return nil, echo.NewHTTPError(http.StatusConflict, "По этому дополнению решение уже принято - снять его нельзя")
	}

	now := time.Now().UTC()
	comment := trimmedSupplementComment(req.Comment)
	if err := tx.Exec(`
		UPDATE application_supplements
		SET status = ?, decided_by_user_id = ?, decided_at = ?, decision_comment = ?
		WHERE id = ?
	`, models.SupplementCancelled, user.ID, now, comment, round.ID).Error; err != nil {
		tx.Rollback()
		slog.Error("дополнение: не удалось снять раунд", "supplement_id", round.ID, "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to cancel supplement")
	}

	oldStatus := round.Status
	s.recorder.Log(ctx, tx, models.AuditEntityApplication, &applicationID, models.AuditActionSupplementCancelledByAuthor, &user.ID,
		applicationAuditDetails{
			OldValue: &oldStatus,
			NewValue: ptrString(models.SupplementCancelled),
			Comment:  comment,
			Metadata: supplementAuditMetadata(round, nil),
		})

	if err := s.bumpStatusUpdated(tx, applicationID, &user.ID); err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		slog.Error("дополнение: не удалось зафиксировать снятие раунда", "supplement_id", round.ID, "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to commit transaction")
	}

	slog.Info("дополнение снято автором", "application_id", applicationID, "supplement_id", round.ID, "user_id", user.ID)
	// Строки раунда так и остаются со status = 0 - снятие ничего с КПП не убирает и не
	// выпускает, поэтому ни бланк, ни таблицы проходной трогать не надо.
	s.notifyApplicationUpdated(ctx, applicationID, archiveDataUnchanged)

	return &SupplementDecisionResponse{
		SupplementID: round.ID,
		Number:       round.Number,
		Status:       models.SupplementCancelled,
	}, nil
}

// activateSupplementItems поднимает на КПП строки принятого раунда - и только их. Возвращает
// число реально активированных машин и сотрудников.
//
// ТМЦ здесь нет намеренно: у items нет поля status, их невидимость до принятия держится
// фильтром читателей по supplement_id, и смены статуса раунда им достаточно.
//
// Три условия выборки, каждое закрывает свой риск:
//   - supplement_id = ? - только свой раунд. Исходный состав и прошлые принятые раунды уже
//     стоят со status = 1, и повторная активация записала бы им вторую «Добавлен в таблицу
//     проходной» - историю попадания, которого не было.
//   - status IS DISTINCT FROM 1 - дедуп той же истории внутри раунда. IS DISTINCT FROM, а не
//     <>: NULL-статус тоже подлежит активации, а `<> 1` его молча пропустил бы.
//   - date_removed/date_deleted IS NULL AND is_purged = false - строку из корзины и строку,
//     погашенную чёрным списком, воскрешать нельзя. Обе лежат ровно с тем же status = 0, что
//     и непринятая добавка, поэтому один предикат статуса поднял бы их обратно на КПП.
//     Метка у обоих случаев одна (trash_service ставит date_removed/date_deleted при
//     удалении, deactivateMatchingCars/Employees - при попадании в ЧС), поэтому одного
//     условия хватает на оба.
func (s *applicationService) activateSupplementItems(ctx context.Context, tx *gorm.DB, supplementID int, actorID *int) (int, error) {
	var carIDs []int
	if err := tx.Raw(`
		UPDATE cars SET status = 1, updated_at = CURRENT_TIMESTAMP
		WHERE supplement_id = ? AND status IS DISTINCT FROM 1
		  AND date_removed IS NULL AND is_purged = false
		RETURNING id
	`, supplementID).Scan(&carIDs).Error; err != nil {
		slog.Error("дополнение: не удалось активировать машины раунда", "supplement_id", supplementID, "error", err)
		return 0, echo.NewHTTPError(http.StatusInternalServerError, "Failed to activate supplement cars")
	}
	s.recordEntitiesAddedToTable(ctx, tx, models.AuditEntityCar, carIDs, actorID)

	var employeeIDs []int
	if err := tx.Raw(`
		UPDATE employees SET status = 1, updated_at = CURRENT_TIMESTAMP
		WHERE supplement_id = ? AND status IS DISTINCT FROM 1
		  AND date_deleted IS NULL AND is_purged = false
		RETURNING id
	`, supplementID).Scan(&employeeIDs).Error; err != nil {
		slog.Error("дополнение: не удалось активировать сотрудников раунда", "supplement_id", supplementID, "error", err)
		return 0, echo.NewHTTPError(http.StatusInternalServerError, "Failed to activate supplement employees")
	}
	s.recordEntitiesAddedToTable(ctx, tx, models.AuditEntityEmployee, employeeIDs, actorID)

	return len(carIDs) + len(employeeIDs), nil
}

// notifySupplementDecided зовёт автора заявки: статус самой заявки решением по раунду не
// двигается, и без уведомления автор не узнал бы, пустили его добавку на КПП или нет.
// Best-effort после commit: сбой уведомления уже записанное решение не отменяет.
func (s *applicationService) notifySupplementDecided(ctx context.Context, applicationID int, round supplementRound, status string, actorID *int) {
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
		slog.Warn("дополнение: не удалось прочитать автора заявки", "application_id", applicationID, "err", err)
		return
	}
	if app.SenderUserID == nil {
		return
	}
	// Автор сам и принял решение (принимающий по своей заявке) - себе не шлём.
	if actorID != nil && *actorID == *app.SenderUserID {
		return
	}

	number := app.ApplicationNumber
	if number == "" {
		number = fmt.Sprintf("№ %d", applicationID)
	}

	var title, message string
	switch status {
	case models.SupplementAccepted:
		title = "Дополнение принято"
		message = fmt.Sprintf("Дополнение №%d к заявке %s принято - добавленные строки на проходной.", round.Number, number)
	case models.SupplementRefused:
		title = "Дополнение отклонено"
		message = fmt.Sprintf("Дополнение №%d к заявке %s отклонено принимающим.", round.Number, number)
	default:
		return
	}

	payload, _ := json.Marshal(map[string]any{
		"application_id":     applicationID,
		"application_number": number,
		"supplement_id":      round.ID,
		"supplement_number":  round.Number,
		"status":             status,
	})
	payloadStr := string(payload)
	if err := s.notificationService.CreateForUser(ctx, *app.SenderUserID, NotificationTypeApplicationSupplementDecided,
		title, message, &payloadStr); err != nil {
		slog.Warn("дополнение: уведомление автору не создано", "user_id", *app.SenderUserID,
			"application_id", applicationID, "err", err)
	}
}
