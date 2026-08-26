package services

// Голосование согласующих по раунду дополнения (#1685, срез S3).
//
// Круг раунда живёт в application_supplement_approvals и пишет свой итог в
// application_supplements.status. Ни одна операция этого файла не трогает
// applications.confirmation и applications.status - на этой паре держится допуск строк на
// КПП, и откат вердикта заявки снял бы пропуска, уже выданные по исходному составу. Это
// главное, что здесь стережётся, и ради этого раунд вообще заведён отдельной сущностью.
//
// Правило кворума общее с основным кругом заявки - tallyApprovals (approval_tally.go).

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

// SupplementApprovalRequest - голос согласующего по раунду дополнения.
//
// user_id в теле нет намеренно: голосующий - владелец токена. Основной круг заявки его
// принимает и тут же сверяет со своим (ApproveApplicationByUser), то есть поле не решает
// ничего, кроме лишнего повода ошибиться на фронте.
type SupplementApprovalRequest struct {
	Status  string  `json:"status" validate:"required,oneof=approved rejected"`
	Comment *string `json:"comment"`
}

// SupplementRevokeApprovalRequest - отзыв своего голоса по раунду.
type SupplementRevokeApprovalRequest struct {
	Comment *string `json:"comment"`
}

// SupplementVoteResponse - состояние раунда после голоса: его итог и голос вызывающего.
// Карточка дополнения перерисовывается по этой паре, не дожидаясь перезапроса списка.
type SupplementVoteResponse struct {
	SupplementID int `json:"supplement_id"`
	Number       int `json:"number"`
	// Status - итог раунда (application_supplements.status) после пересчёта кворума.
	Status string `json:"status"`
	// MyStatus - голос вызывающего после операции: approved/rejected/pending.
	MyStatus string `json:"my_status"`
}

// supplementRevocableStatuses - раунды, по которым голос ещё можно отозвать: решение
// принимающего (accepted/refused) и снятие раунда отзыв не переживает - отозванный голос
// уже ничего не изменил бы, а статус раунда пришлось бы двигать назад из терминального.
var supplementRevocableStatuses = []string{models.SupplementPending, models.SupplementApproved, models.SupplementRejected}

// supplementRound - раунд в том виде, в каком его читают гарды голосования.
type supplementRound struct {
	ID            int
	ApplicationID int
	Number        int
	Status        string
}

// ApproveSupplement фиксирует голос согласующего по раунду дополнения.
func (s *applicationService) ApproveSupplement(ctx context.Context, username string, applicationID, supplementID int, req SupplementApprovalRequest) (*SupplementVoteResponse, error) {
	if req.Status != voteStatusApproved && req.Status != voteStatusRejected {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Недопустимый голос: ожидается approved или rejected")
	}
	user, err := s.getUserByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	// Архивная заявка доступна только для чтения; открытый раунд в архиве пережить может -
	// архивация, в отличие от закрытия заявки, его не снимает.
	if err := s.checkNotArchived(ctx, applicationID); err != nil {
		return nil, err
	}
	// Членство в раунде - до всего остального и по одному supplement_id: посторонний
	// получает 403 и на существующий раунд, и на выдуманный, и перебором id состав чужих
	// заявок не нащупывает.
	if err := s.ensureSupplementVoter(ctx, s.db, supplementID, user.ID); err != nil {
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
	if round.Status != models.SupplementPending {
		tx.Rollback()
		return nil, echo.NewHTTPError(http.StatusConflict, "По этому дополнению голосование уже закрыто")
	}

	// Свой голос перечитываем под блокировкой раунда: между проверкой членства и записью
	// тот же пользователь мог проголосовать соседним запросом.
	vote, err := s.readSupplementVote(tx, supplementID, user.ID)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	if vote.ApprovalStatus == nil || *vote.ApprovalStatus != voteStatusPending {
		tx.Rollback()
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Вы уже голосовали по этому дополнению")
	}

	// Гейт обхода ЧС (#481) в границах раунда: согласовать нельзя, пока по помеченным
	// строкам ЭТОГО раунда не подтверждён пропуск. Отказ не блокируем - отклонить
	// подозрительную добавку можно сразу.
	if req.Status == voteStatusApproved {
		blocked, err := hasUnoverriddenSupplementBlacklistFlags(ctx, tx, applicationID, supplementID)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
		if blocked {
			tx.Rollback()
			return nil, echo.NewHTTPError(http.StatusConflict,
				"Дополнение содержит элементы, похожие на чёрный список. Подтвердите пропуск каждого ('Всё равно пропустить') перед согласованием")
		}
	}

	now := time.Now().UTC()
	if err := tx.Exec(`
		UPDATE application_supplement_approvals
		SET approval_status = ?, approval_comment = ?, approval_datetime = ?
		WHERE supplement_id = ? AND user_id = ?
	`, req.Status, req.Comment, now, supplementID, user.ID).Error; err != nil {
		tx.Rollback()
		slog.Error("дополнение: не удалось записать голос", "supplement_id", supplementID, "user_id", user.ID, "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to save supplement vote")
	}

	action := models.AuditActionSupplementApprove
	if req.Status == voteStatusRejected {
		action = models.AuditActionSupplementReject
	}
	s.recorder.Log(ctx, tx, models.AuditEntityApplication, &applicationID, action, &user.ID,
		applicationAuditDetails{
			Comment:  req.Comment,
			Metadata: supplementAuditMetadata(round, map[string]any{"required_approval": vote.RequiredApproval}),
		})

	newStatus, err := s.recalcSupplementStatus(ctx, tx, applicationID, round, &user.ID, now)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		slog.Error("дополнение: не удалось зафиксировать голос", "supplement_id", supplementID, "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to commit transaction")
	}

	slog.Info("голос по дополнению", "application_id", applicationID, "supplement_id", supplementID,
		"user_id", user.ID, "vote", req.Status, "round_status", newStatus)
	// Состав вложения голосом не меняется - строки раунда лежат там с момента подачи и на
	// КПП попадут только после принятия, поэтому бланк и заявка.json перевыгружать незачем.
	s.notifyApplicationUpdated(ctx, applicationID, archiveDataUnchanged)
	if newStatus == models.SupplementApproved {
		s.notifySupplementReadyForApprovers(ctx, applicationID, round)
	}

	return &SupplementVoteResponse{
		SupplementID: round.ID,
		Number:       round.Number,
		Status:       newStatus,
		MyStatus:     req.Status,
	}, nil
}

// RevokeSupplementApproval возвращает голос согласующего по раунду в pending.
func (s *applicationService) RevokeSupplementApproval(ctx context.Context, username string, applicationID, supplementID int, req SupplementRevokeApprovalRequest) (*SupplementVoteResponse, error) {
	user, err := s.getUserByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	if err := s.checkNotArchived(ctx, applicationID); err != nil {
		return nil, err
	}
	if err := s.ensureSupplementVoter(ctx, s.db, supplementID, user.ID); err != nil {
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
	if !slices.Contains(supplementRevocableStatuses, round.Status) {
		tx.Rollback()
		return nil, echo.NewHTTPError(http.StatusConflict, "По этому дополнению голос отозвать уже нельзя")
	}
	// Отклонённый раунд отзыв голоса открывает заново, а открытое дополнение у заявки
	// бывает только одно и только пока заявка жива. Оба условия проверяем до записи.
	if round.Status == models.SupplementRejected {
		if err := s.ensureRoundCanReopen(tx, applicationID, round.ID); err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	vote, err := s.readSupplementVote(tx, supplementID, user.ID)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	if vote.ApprovalStatus == nil || *vote.ApprovalStatus == voteStatusPending {
		tx.Rollback()
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Вы ещё не голосовали по этому дополнению")
	}

	if err := tx.Exec(`
		UPDATE application_supplement_approvals
		SET approval_status = ?, approval_comment = NULL, approval_datetime = NULL
		WHERE supplement_id = ? AND user_id = ?
	`, voteStatusPending, supplementID, user.ID).Error; err != nil {
		tx.Rollback()
		slog.Error("дополнение: не удалось отозвать голос", "supplement_id", supplementID, "user_id", user.ID, "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to revoke supplement vote")
	}

	s.recorder.Log(ctx, tx, models.AuditEntityApplication, &applicationID, models.AuditActionSupplementRevokeApproval, &user.ID,
		applicationAuditDetails{
			Comment:  req.Comment,
			Metadata: supplementAuditMetadata(round, nil),
		})

	newStatus, err := s.recalcSupplementStatus(ctx, tx, applicationID, round, &user.ID, time.Now().UTC())
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		slog.Error("дополнение: не удалось зафиксировать отзыв голоса", "supplement_id", supplementID, "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to commit transaction")
	}

	slog.Info("голос по дополнению отозван", "application_id", applicationID, "supplement_id", supplementID, "user_id", user.ID)
	s.notifyApplicationUpdated(ctx, applicationID, archiveDataUnchanged)

	return &SupplementVoteResponse{
		SupplementID: round.ID,
		Number:       round.Number,
		Status:       newStatus,
		MyStatus:     voteStatusPending,
	}, nil
}

// supplementVoteRow - строка голоса, какой её читают гарды.
type supplementVoteRow struct {
	ApprovalStatus   *string
	RequiredApproval bool
}

// ensureSupplementVoter пускает к голосованию только состав раунда - снимок ответственных
// заявки на момент подачи дополнения. Отвечает 403 и когда раунда нет вовсе: разные ответы
// на «не мой раунд» и «нет такого раунда» выдавали бы перебором id существование чужих
// дополнений.
func (s *applicationService) ensureSupplementVoter(ctx context.Context, db *gorm.DB, supplementID, userID int) error {
	var cnt int64
	err := db.WithContext(ctx).Model(&models.ApplicationSupplementApproval{}).
		Where("supplement_id = ? AND user_id = ?", supplementID, userID).
		Count(&cnt).Error
	if err != nil {
		slog.Error("дополнение: не удалось проверить состав голосующих", "supplement_id", supplementID, "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to check supplement approvers")
	}
	if cnt == 0 {
		return echo.NewHTTPError(http.StatusForbidden, "Вы не согласующий по этому дополнению")
	}
	return nil
}

// ensureRoundCanReopen проверяет, что отклонённый раунд вообще можно вернуть в работу:
// заявка ещё принимает дополнения и другого открытого раунда у неё нет.
//
// Оба ограничения не выдуманы здесь: статусы заявки те же, при которых её разрешено
// дополнять, а единственность открытого раунда стережёт частичный уникальный индекс
// uidx_app_supplement_open - без проверки отзыв голоса упирался бы в него уже на UPDATE.
// Автор мог подать следующее дополнение сразу после отказа: терминальный раунд этого не
// запрещает, и тогда возвращать отклонённый в работу поздно.
func (s *applicationService) ensureRoundCanReopen(tx *gorm.DB, applicationID, supplementID int) error {
	var status *string
	if err := tx.Raw("SELECT status FROM applications WHERE id = ?", applicationID).Scan(&status).Error; err != nil {
		slog.Error("дополнение: не удалось прочитать статус заявки", "application_id", applicationID, "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to load application")
	}
	current := ""
	if status != nil {
		current = *status
	}
	if !slices.Contains(supplementAllowedStatuses, current) {
		return echo.NewHTTPError(http.StatusConflict,
			fmt.Sprintf("Заявка в статусе «%s» - вернуть дополнение на согласование нельзя", current))
	}

	var openCount int64
	err := tx.Model(&models.ApplicationSupplement{}).
		Where("application_id = ? AND id <> ? AND status IN ?", applicationID, supplementID, models.OpenSupplementStatuses).
		Count(&openCount).Error
	if err != nil {
		slog.Error("дополнение: не удалось проверить открытые раунды", "application_id", applicationID, "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to check open supplements")
	}
	if openCount > 0 {
		return echo.NewHTTPError(http.StatusConflict,
			"По заявке уже идёт другое дополнение - отозвать отказ по этому нельзя")
	}
	return nil
}

// lockSupplementRound читает раунд под блокировкой строки, ограничив выборку заявкой из
// пути: чужой раунд по прямой ссылке отвечает тем же 404, что и несуществующий. Блокировка
// сериализует голоса - иначе два одновременных считали бы кворум по одному и тому же
// доголосового расклада и разошлись бы в итоге.
func (s *applicationService) lockSupplementRound(tx *gorm.DB, applicationID, supplementID int) (supplementRound, error) {
	var round supplementRound

	// Заявка блокируется ПЕРВОЙ и только потом раунд. Порядок именно такой, потому что
	// встречный путь его уже занял: закрытие заявки (отзыв, вывод из работы, отказ, срок)
	// сперва пишет в applications, а затем снимает открытые раунды через
	// cancelOpenSupplements. Возьми мы раунд первым - получилась бы встречная пара
	// «раунд -> заявка» против «заявка -> раунд», то есть дедлок: любой путь отсюда
	// рано или поздно трогает саму заявку (хотя бы отметкой status_updated_at).
	//
	// Блокировка здесь, а не у вызывающих: точек входа четыре (голос, отзыв голоса,
	// решение принимающего, снятие автором), и порядок, который можно забыть в одной из
	// них, инвариантом не является.
	if err := tx.Exec("SELECT id FROM applications WHERE id = ? FOR UPDATE", applicationID).Error; err != nil {
		slog.Error("дополнение: не удалось заблокировать заявку", "application_id", applicationID, "error", err)
		return round, echo.NewHTTPError(http.StatusInternalServerError, "Failed to lock application")
	}

	res := tx.Raw(`
		SELECT id, application_id, number, status
		FROM application_supplements
		WHERE id = ? AND application_id = ?
		FOR UPDATE
	`, supplementID, applicationID).Scan(&round)
	if res.Error != nil {
		slog.Error("дополнение: не удалось заблокировать раунд", "supplement_id", supplementID, "error", res.Error)
		return round, echo.NewHTTPError(http.StatusInternalServerError, "Failed to load supplement")
	}
	if res.RowsAffected == 0 {
		return round, echo.NewHTTPError(http.StatusNotFound, "Дополнение не найдено у этой заявки")
	}
	return round, nil
}

// readSupplementVote читает голос пользователя по раунду.
func (s *applicationService) readSupplementVote(tx *gorm.DB, supplementID, userID int) (supplementVoteRow, error) {
	var vote supplementVoteRow
	res := tx.Raw(`
		SELECT approval_status, COALESCE(required_approval, false) AS required_approval
		FROM application_supplement_approvals
		WHERE supplement_id = ? AND user_id = ?
	`, supplementID, userID).Scan(&vote)
	if res.Error != nil {
		slog.Error("дополнение: не удалось прочитать голос", "supplement_id", supplementID, "user_id", userID, "error", res.Error)
		return vote, echo.NewHTTPError(http.StatusInternalServerError, "Failed to load supplement vote")
	}
	if res.RowsAffected == 0 {
		return vote, echo.NewHTTPError(http.StatusForbidden, "Вы не согласующий по этому дополнению")
	}
	return vote, nil
}

// recalcSupplementStatus пересчитывает итог раунда по его голосам и, если он сменился,
// пишет новый статус, дату выхода из pending и запись в историю заявки. Возвращает
// актуальный статус раунда.
//
// Заявку не трогает ни одним UPDATE: единственная её колонка, которой касается пересчёт, -
// status_updated_at (отметка «в карточке что-то изменилось», #1349). Ни confirmation, ни
// status заявки от исхода раунда не зависят.
func (s *applicationService) recalcSupplementStatus(ctx context.Context, tx *gorm.DB, applicationID int, round supplementRound, actorID *int, now time.Time) (string, error) {
	var rows []supplementVoteRow
	if err := tx.Raw(`
		SELECT approval_status, COALESCE(required_approval, false) AS required_approval
		FROM application_supplement_approvals
		WHERE supplement_id = ?
	`, round.ID).Scan(&rows).Error; err != nil {
		slog.Error("дополнение: не удалось прочитать голоса раунда", "supplement_id", round.ID, "error", err)
		return "", echo.NewHTTPError(http.StatusInternalServerError, "Failed to load supplement votes")
	}
	// Круг без голосующих вердикта не производит - оставляем раунд как есть.
	if len(rows) == 0 {
		return round.Status, nil
	}

	votes := make([]approvalVote, 0, len(rows))
	for _, r := range rows {
		votes = append(votes, approvalVote{Required: r.RequiredApproval, Status: r.ApprovalStatus})
	}

	newStatus := models.SupplementPending
	switch tallyApprovals(votes) {
	case voteStatusApproved:
		newStatus = models.SupplementApproved
	case voteStatusRejected:
		newStatus = models.SupplementRejected
	}
	if newStatus == round.Status {
		return round.Status, nil
	}

	// confirmation_datetime фиксирует ПЕРВЫЙ выход раунда из pending и назад не откатывается -
	// как confirmation_datetime заявки: момент, когда круг впервые дал ответ, история.
	if err := tx.Exec(`
		UPDATE application_supplements
		SET status = ?,
		    confirmation_datetime = CASE
		        WHEN ? != ? AND confirmation_datetime IS NULL THEN ?
		        ELSE confirmation_datetime
		    END
		WHERE id = ?
	`, newStatus, newStatus, models.SupplementPending, now, round.ID).Error; err != nil {
		// Гонка с подачей следующего дополнения: она блокирует строку заявки, отзыв голоса -
		// строку раунда, друг друга эти локи не видят. Частичный уникальный индекс открытых
		// раундов ловит второго, и ответ у него тот же, что у явного гарда.
		if isUniqueViolation(err) {
			return "", echo.NewHTTPError(http.StatusConflict,
				"По заявке уже идёт другое дополнение - отозвать отказ по этому нельзя")
		}
		slog.Error("дополнение: не удалось обновить итог раунда", "supplement_id", round.ID, "error", err)
		return "", echo.NewHTTPError(http.StatusInternalServerError, "Failed to update supplement status")
	}

	oldStatus := round.Status
	s.recorder.Log(ctx, tx, models.AuditEntityApplication, &applicationID, models.AuditActionSupplementConfirmationChange, actorID,
		applicationAuditDetails{
			OldValue: &oldStatus,
			NewValue: &newStatus,
			Metadata: supplementAuditMetadata(round, nil),
		})
	if err := s.bumpStatusUpdated(tx, applicationID, actorID); err != nil {
		return "", err
	}
	return newStatus, nil
}

// supplementAuditMetadata - общая метаданная событий раунда: по ней читатель истории заявки
// отличает событие раунда от события основного круга и знает, какого именно раунда.
func supplementAuditMetadata(round supplementRound, extra map[string]any) json.RawMessage {
	meta := map[string]any{
		"supplement_id": round.ID,
		"number":        round.Number,
	}
	for k, v := range extra {
		meta[k] = v
	}
	raw, err := json.Marshal(meta)
	if err != nil {
		slog.Warn("дополнение: не удалось собрать метаданные истории", "supplement_id", round.ID, "err", err)
		return nil
	}
	return raw
}

// notifySupplementReadyForApprovers зовёт принимающих, когда раунд прошёл круг согласования.
// Без этого зова о добавке никто не узнает: заявка уже в работе, её статус от раунда не
// двигается, и в списках принимающего ничего не меняется - принимать пришедшее было бы некому.
// Best-effort после commit: сбой уведомления не отменяет уже записанный итог.
func (s *applicationService) notifySupplementReadyForApprovers(ctx context.Context, applicationID int, round supplementRound) {
	if s.notificationService == nil {
		return
	}

	var recipients []int
	if err := s.db.WithContext(ctx).Table("application_approvers").
		Distinct("user_id").Pluck("user_id", &recipients).Error; err != nil {
		slog.Warn("дополнение: не удалось собрать принимающих", "application_id", applicationID, "err", err)
		return
	}
	if len(recipients) == 0 {
		return
	}

	var number *string
	if err := s.db.WithContext(ctx).Raw("SELECT application_number FROM applications WHERE id = ?", applicationID).
		Scan(&number).Error; err != nil {
		slog.Warn("дополнение: не удалось прочитать номер заявки", "application_id", applicationID, "err", err)
	}
	appNumber := ""
	if number != nil {
		appNumber = *number
	}

	payload, _ := json.Marshal(map[string]any{
		"application_id":     applicationID,
		"application_number": appNumber,
		"supplement_id":      round.ID,
		"supplement_number":  round.Number,
	})
	payloadStr := string(payload)
	message := fmt.Sprintf("Дополнение №%d к заявке %s согласовано - требуется принятие.", round.Number, appNumber)

	for _, userID := range recipients {
		if err := s.notificationService.CreateForUser(ctx, userID, NotificationTypeApplicationSupplementReady,
			"Дополнение согласовано", message, &payloadStr); err != nil {
			slog.Warn("дополнение: уведомление принимающему не создано", "user_id", userID,
				"application_id", applicationID, "err", err)
		}
	}
}
