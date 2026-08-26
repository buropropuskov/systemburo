package handlers_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Голосование согласующих по раунду дополнения (#1685, срез S3).
//
// Ядро проверок: как бы ни сложился кворум раунда, applications.confirmation и
// applications.status остаются нетронутыми. От этой пары производен допуск строки на КПП,
// и сдвиг вердикта заявки снял бы пропуска, уже выданные по исходному составу.
//
// Секциями на одном поднятом приложении: отдельные SetupTestApp с CleanDB на каждую секцию
// перебивают границу go test -timeout у пакета handlers (та же причина, что в
// application_supplement_guards_test).

// suppVoteUser - согласующий: логин без права дополнять заявки (оно тут ни при чём).
func suppVoteUser(t *testing.T, e *echo.Echo, db *gorm.DB, username string, orgID, companyID int) (int, string) {
	t.Helper()
	testutil.RegisterUser(t, e, username, "pass123", 1, orgID, companyID)
	var user models.User
	require.NoError(t, db.Where("username = ?", username).First(&user).Error)
	token, _ := testutil.LoginUser(t, e, username, "pass123")
	return user.ID, token
}

// suppVoteApproval - строка снимка голосующих раунда.
func suppVoteApproval(t *testing.T, db *gorm.DB, supplementID, userID int, required bool) {
	t.Helper()
	pending := "pending"
	require.NoError(t, db.Create(&models.ApplicationSupplementApproval{
		SupplementID:     supplementID,
		UserID:           userID,
		RequiredApproval: required,
		ApprovalStatus:   &pending,
	}).Error)
}

func suppVoteReadRoundByID(t *testing.T, db *gorm.DB, roundID int) models.ApplicationSupplement {
	t.Helper()
	var round models.ApplicationSupplement
	require.NoError(t, db.First(&round, roundID).Error)
	return round
}

func suppVoteReadVote(t *testing.T, db *gorm.DB, supplementID, userID int) models.ApplicationSupplementApproval {
	t.Helper()
	var vote models.ApplicationSupplementApproval
	require.NoError(t, db.Where("supplement_id = ? AND user_id = ?", supplementID, userID).First(&vote).Error)
	return vote
}

// suppVoteFlag - предупреждение о возможном обходе ЧС по строке заявки. supplementID nil -
// флаг исходного состава подачи.
// suppVoteFlag - пометка о сходстве в заявке. Ссылается на настоящую запись чёрного
// списка: пометка на снятый запрет заявку не держит, поэтому фиктивный
// matched_blacklist_id гейт бы не включил.
func suppVoteFlag(t *testing.T, db *gorm.DB, appID int, supplementID *int, value string) {
	t.Helper()
	mark := seedMark(t, db, "SuppVote_"+value)
	entry := models.VehicleBlacklist{
		CarNumber: value,
		MarkID:    mark.ID,
		MarkName:  mark.Name,
		Reason:    "похожий номер",
		IsActive:  true,
	}
	require.NoError(t, db.Create(&entry).Error)

	require.NoError(t, db.Create(&models.ApplicationBlacklistFlag{
		ApplicationID:      appID,
		SupplementID:       supplementID,
		ElementType:        models.BlacklistElementCar,
		ElementID:          1,
		ElementNormalized:  value,
		MatchedBlacklistID: entry.ID,
		MatchedValue:       value,
		MatchedReason:      "похожий номер",
		Similarity:         0.9,
	}).Error)
}

// suppVoteAssertApplicationIntact - вердикт заявки на месте. Ради этого дополнение и заведено
// отдельной сущностью, поэтому проверка стоит в каждой секции, где раунд меняет статус.
func suppVoteAssertApplicationIntact(t *testing.T, db *gorm.DB, appID int) {
	t.Helper()
	app := suppReadApplication(t, db, appID)
	require.NotNil(t, app.Confirmation)
	require.NotNil(t, app.Status)
	assert.Equal(t, models.ConfirmationApproved, *app.Confirmation, "исход раунда не двигает согласование заявки")
	assert.Equal(t, models.StatusInWork, *app.Status, "исход раунда не двигает статус заявки")
}

func TestSupplementVote(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	authorID, _ := suppVoteUser(t, e, db, "vote_author", td.OrgID, td.CompanyID)

	approve := func(token string, appID, roundID int, body string) *httptest.ResponseRecorder {
		return testutil.POST(t, e, fmt.Sprintf("/applications/%d/supplements/%d/approve", appID, roundID),
			body, testutil.AuthHeader(token))
	}
	revoke := func(token string, appID, roundID int, body string) *httptest.ResponseRecorder {
		return testutil.POST(t, e, fmt.Sprintf("/applications/%d/supplements/%d/revoke-approval", appID, roundID),
			body, testutil.AuthHeader(token))
	}

	t.Run("согласие обязательного закрывает раунд, вердикт заявки цел", func(t *testing.T) {
		voterID, voterToken := suppVoteUser(t, e, db, "vote_ok_voter", td.OrgID, td.CompanyID)
		acceptorID, _ := suppVoteUser(t, e, db, "vote_ok_acceptor", td.OrgID, td.CompanyID)
		require.NoError(t, db.Create(&models.ApplicationApprover{UserID: acceptorID}).Error)

		appID := suppApp(t, db, td.OrgID, authorID, "VOTE-OK-1", models.ConfirmationApproved, models.StatusInWork)
		roundID := suppNewSupplement(t, db, appID, authorID, models.SupplementPending)
		suppVoteApproval(t, db, roundID, voterID, true)

		rec := approve(voterToken, appID, roundID, `{"status":"approved","comment":"строки в порядке"}`)
		require.Equal(t, http.StatusOK, rec.Code, "голос: %s", rec.Body.String())

		resp := testutil.ParseResponse[services.SupplementVoteResponse](t, rec)
		assert.Equal(t, models.SupplementApproved, resp.Status, "все обязательные за - раунд согласован")
		assert.Equal(t, "approved", resp.MyStatus)
		assert.Equal(t, roundID, resp.SupplementID)

		round := suppVoteReadRoundByID(t, db, roundID)
		assert.Equal(t, models.SupplementApproved, round.Status)
		assert.NotNil(t, round.ConfirmationDatetime, "первый выход раунда из pending датируется")

		vote := suppVoteReadVote(t, db, roundID, voterID)
		require.NotNil(t, vote.ApprovalStatus)
		assert.Equal(t, "approved", *vote.ApprovalStatus)
		require.NotNil(t, vote.ApprovalComment)
		assert.Equal(t, "строки в порядке", *vote.ApprovalComment)
		assert.NotNil(t, vote.ApprovalDatetime)

		suppVoteAssertApplicationIntact(t, db, appID)

		var audits int64
		require.NoError(t, db.Table("audit_log").
			Where("entity_type = ? AND entity_id = ? AND action = ?", models.AuditEntityApplication, appID, models.AuditActionSupplementApprove).
			Count(&audits).Error)
		assert.EqualValues(t, 1, audits, "голос записан в историю заявки")
		require.NoError(t, db.Table("audit_log").
			Where("entity_type = ? AND entity_id = ? AND action = ?", models.AuditEntityApplication, appID, models.AuditActionSupplementConfirmationChange).
			Count(&audits).Error)
		assert.EqualValues(t, 1, audits, "смена итога раунда записана в историю заявки")

		// Принимающему надо узнать, что появилось что принимать: статус заявки от исхода
		// раунда не двигается, и в его списках без уведомления ничего бы не изменилось.
		var notifications int64
		require.NoError(t, db.Model(&models.Notification{}).
			Where("user_id = ? AND type = ?", acceptorID, "application_supplement_ready").
			Count(&notifications).Error)
		assert.EqualValues(t, 1, notifications, "принимающий уведомлён о согласованном дополнении")
	})

	t.Run("отказ обязательного хоронит раунд, вердикт заявки цел", func(t *testing.T) {
		voterID, voterToken := suppVoteUser(t, e, db, "vote_reject_voter", td.OrgID, td.CompanyID)
		otherID, _ := suppVoteUser(t, e, db, "vote_reject_other", td.OrgID, td.CompanyID)

		appID := suppApp(t, db, td.OrgID, authorID, "VOTE-REJ-1", models.ConfirmationApproved, models.StatusInWork)
		roundID := suppNewSupplement(t, db, appID, authorID, models.SupplementPending)
		suppVoteApproval(t, db, roundID, voterID, true)
		suppVoteApproval(t, db, roundID, otherID, true)

		rec := approve(voterToken, appID, roundID, `{"status":"rejected","comment":"паспорт не сходится"}`)
		require.Equal(t, http.StatusOK, rec.Code, "голос: %s", rec.Body.String())

		resp := testutil.ParseResponse[services.SupplementVoteResponse](t, rec)
		assert.Equal(t, models.SupplementRejected, resp.Status, "один обязательный отказ хоронит раунд, не дожидаясь второго")
		assert.Equal(t, "rejected", resp.MyStatus)

		round := suppVoteReadRoundByID(t, db, roundID)
		assert.Equal(t, models.SupplementRejected, round.Status)
		suppVoteAssertApplicationIntact(t, db, appID)

		var audits int64
		require.NoError(t, db.Table("audit_log").
			Where("entity_type = ? AND entity_id = ? AND action = ?", models.AuditEntityApplication, appID, models.AuditActionSupplementReject).
			Count(&audits).Error)
		assert.EqualValues(t, 1, audits, "отказ записан в историю заявки")
	})

	t.Run("голосуют только согласующие раунда", func(t *testing.T) {
		voterID, _ := suppVoteUser(t, e, db, "vote_403_voter", td.OrgID, td.CompanyID)
		_, strangerToken := suppVoteUser(t, e, db, "vote_403_stranger", td.OrgID, td.CompanyID)

		appID := suppApp(t, db, td.OrgID, authorID, "VOTE-403-1", models.ConfirmationApproved, models.StatusInWork)
		roundID := suppNewSupplement(t, db, appID, authorID, models.SupplementPending)
		suppVoteApproval(t, db, roundID, voterID, true)

		rec := approve(strangerToken, appID, roundID, `{"status":"approved"}`)
		assert.Equal(t, http.StatusForbidden, rec.Code, "посторонний не голосует по чужому раунду")

		// Тот же 403 на выдуманный раунд: иначе перебором id вычислялось бы, у каких
		// заявок дополнения есть.
		rec = approve(strangerToken, appID, roundID+9999, `{"status":"approved"}`)
		assert.Equal(t, http.StatusForbidden, rec.Code, "несуществующий раунд отвечает постороннему так же")

		round := suppVoteReadRoundByID(t, db, roundID)
		assert.Equal(t, models.SupplementPending, round.Status, "чужой голос раунда не сдвинул")
	})

	t.Run("повторный голос отклоняется", func(t *testing.T) {
		voterID, voterToken := suppVoteUser(t, e, db, "vote_dup_voter", td.OrgID, td.CompanyID)
		otherID, _ := suppVoteUser(t, e, db, "vote_dup_other", td.OrgID, td.CompanyID)

		appID := suppApp(t, db, td.OrgID, authorID, "VOTE-DUP-1", models.ConfirmationApproved, models.StatusInWork)
		roundID := suppNewSupplement(t, db, appID, authorID, models.SupplementPending)
		// Второй обязательный держит раунд открытым - иначе повторный голос упёрся бы
		// в закрытый раунд, а проверяем мы именно запрет переголосовать.
		suppVoteApproval(t, db, roundID, voterID, true)
		suppVoteApproval(t, db, roundID, otherID, true)

		rec := approve(voterToken, appID, roundID, `{"status":"approved"}`)
		require.Equal(t, http.StatusOK, rec.Code, "первый голос: %s", rec.Body.String())
		require.Equal(t, models.SupplementPending, suppVoteReadRoundByID(t, db, roundID).Status,
			"второй обязательный ещё не голосовал - раунд идёт")

		rec = approve(voterToken, appID, roundID, `{"status":"rejected"}`)
		assert.Equal(t, http.StatusBadRequest, rec.Code, "переголосовать нельзя")

		vote := suppVoteReadVote(t, db, roundID, voterID)
		require.NotNil(t, vote.ApprovalStatus)
		assert.Equal(t, "approved", *vote.ApprovalStatus, "отбитый повтор не перезаписал первый голос")
	})

	t.Run("в закрытом раунде голосовать нечего", func(t *testing.T) {
		voterID, voterToken := suppVoteUser(t, e, db, "vote_closed_voter", td.OrgID, td.CompanyID)

		appID := suppApp(t, db, td.OrgID, authorID, "VOTE-CLOSED-1", models.ConfirmationApproved, models.StatusInWork)
		roundID := suppNewSupplement(t, db, appID, authorID, models.SupplementAccepted)
		suppVoteApproval(t, db, roundID, voterID, true)

		rec := approve(voterToken, appID, roundID, `{"status":"approved"}`)
		assert.Equal(t, http.StatusConflict, rec.Code, "принятый раунд голосов не принимает")

		round := suppVoteReadRoundByID(t, db, roundID)
		assert.Equal(t, models.SupplementAccepted, round.Status, "статус принятого раунда не сдвинулся")
	})

	t.Run("недопустимый голос отбивается", func(t *testing.T) {
		voterID, voterToken := suppVoteUser(t, e, db, "vote_badstatus_voter", td.OrgID, td.CompanyID)

		appID := suppApp(t, db, td.OrgID, authorID, "VOTE-BAD-1", models.ConfirmationApproved, models.StatusInWork)
		roundID := suppNewSupplement(t, db, appID, authorID, models.SupplementPending)
		suppVoteApproval(t, db, roundID, voterID, true)

		rec := approve(voterToken, appID, roundID, `{"status":"maybe"}`)
		assert.Equal(t, http.StatusBadRequest, rec.Code, "голос бывает только approved или rejected")
		assert.Equal(t, models.SupplementPending, suppVoteReadRoundByID(t, db, roundID).Status)
	})

	t.Run("отзыв голоса возвращает раунд в pending", func(t *testing.T) {
		voterID, voterToken := suppVoteUser(t, e, db, "vote_revoke_voter", td.OrgID, td.CompanyID)

		appID := suppApp(t, db, td.OrgID, authorID, "VOTE-REV-1", models.ConfirmationApproved, models.StatusInWork)
		roundID := suppNewSupplement(t, db, appID, authorID, models.SupplementPending)
		suppVoteApproval(t, db, roundID, voterID, true)

		rec := approve(voterToken, appID, roundID, `{"status":"approved","comment":"согласен"}`)
		require.Equal(t, http.StatusOK, rec.Code, "голос: %s", rec.Body.String())
		require.Equal(t, models.SupplementApproved, suppVoteReadRoundByID(t, db, roundID).Status)

		rec = revoke(voterToken, appID, roundID, `{"comment":"поторопился"}`)
		require.Equal(t, http.StatusOK, rec.Code, "отзыв голоса: %s", rec.Body.String())

		resp := testutil.ParseResponse[services.SupplementVoteResponse](t, rec)
		assert.Equal(t, models.SupplementPending, resp.Status, "кворум распался - раунд снова идёт")
		assert.Equal(t, "pending", resp.MyStatus)

		round := suppVoteReadRoundByID(t, db, roundID)
		assert.Equal(t, models.SupplementPending, round.Status)
		assert.NotNil(t, round.ConfirmationDatetime,
			"дата первого выхода из pending - история, отзыв её не стирает")

		vote := suppVoteReadVote(t, db, roundID, voterID)
		require.NotNil(t, vote.ApprovalStatus)
		assert.Equal(t, "pending", *vote.ApprovalStatus)
		assert.Nil(t, vote.ApprovalComment, "комментарий прежнего голоса снят")
		assert.Nil(t, vote.ApprovalDatetime, "дата прежнего голоса снята")

		suppVoteAssertApplicationIntact(t, db, appID)

		var audits int64
		require.NoError(t, db.Table("audit_log").
			Where("entity_type = ? AND entity_id = ? AND action = ?", models.AuditEntityApplication, appID, models.AuditActionSupplementRevokeApproval).
			Count(&audits).Error)
		assert.EqualValues(t, 1, audits, "отзыв голоса записан в историю заявки")

		// Отзывать нечего, пока не проголосовал.
		rec = revoke(voterToken, appID, roundID, `{}`)
		assert.Equal(t, http.StatusBadRequest, rec.Code, "повторный отзыв отбивается")
	})

	// Отзыв отказа открывает раунд заново, а открытое дополнение у заявки бывает одно
	// (частичный уникальный индекс) и только пока заявка жива. Без гардов отзыв упирался бы
	// в индекс уже на UPDATE и отвечал пятисоткой.
	t.Run("отзыв отказа не воскрешает раунд поверх следующего", func(t *testing.T) {
		voterID, voterToken := suppVoteUser(t, e, db, "vote_reopen_voter", td.OrgID, td.CompanyID)

		appID := suppApp(t, db, td.OrgID, authorID, "VOTE-REOPEN-1", models.ConfirmationApproved, models.StatusInWork)
		firstID := suppNewSupplement(t, db, appID, authorID, models.SupplementPending)
		suppVoteApproval(t, db, firstID, voterID, true)

		rec := approve(voterToken, appID, firstID, `{"status":"rejected"}`)
		require.Equal(t, http.StatusOK, rec.Code, "отказ: %s", rec.Body.String())
		require.Equal(t, models.SupplementRejected, suppVoteReadRoundByID(t, db, firstID).Status)

		// Автор подал следующее дополнение - отклонённый раунд этого не запрещает.
		second := models.ApplicationSupplement{
			ApplicationID: appID, Number: 2, Status: models.SupplementPending, CreatedByUserID: authorID,
		}
		require.NoError(t, db.Create(&second).Error)

		rec = revoke(voterToken, appID, firstID, `{}`)
		assert.Equal(t, http.StatusConflict, rec.Code, "воскрешать отклонённый раунд поверх идущего нельзя")
		assert.Equal(t, models.SupplementRejected, suppVoteReadRoundByID(t, db, firstID).Status,
			"отбитый отзыв статус раунда не сдвинул")
		assert.Equal(t, models.SupplementPending, suppVoteReadRoundByID(t, db, second.ID).Status,
			"идущее дополнение не задето")
	})

	t.Run("отзыв отказа не воскрешает раунд у закрытой заявки", func(t *testing.T) {
		voterID, voterToken := suppVoteUser(t, e, db, "vote_reopen_closed_voter", td.OrgID, td.CompanyID)

		appID := suppApp(t, db, td.OrgID, authorID, "VOTE-REOPEN-2", models.ConfirmationApproved, models.StatusInWork)
		roundID := suppNewSupplement(t, db, appID, authorID, models.SupplementPending)
		suppVoteApproval(t, db, roundID, voterID, true)

		rec := approve(voterToken, appID, roundID, `{"status":"rejected"}`)
		require.Equal(t, http.StatusOK, rec.Code, "отказ: %s", rec.Body.String())

		// Заявка закрылась после отказа: терминальный раунд снятию не подлежит и остаётся
		// отклонённым, но возвращать его на согласование уже некому.
		require.NoError(t, db.Model(&models.Application{}).Where("id = ?", appID).
			Update("status", models.StatusCompleted).Error)

		rec = revoke(voterToken, appID, roundID, `{}`)
		assert.Equal(t, http.StatusConflict, rec.Code, "у закрытой заявки дополнение на согласование не возвращается")
		assert.Equal(t, models.SupplementRejected, suppVoteReadRoundByID(t, db, roundID).Status)
	})

	t.Run("чёрный список раунда блокирует согласование", func(t *testing.T) {
		voterID, voterToken := suppVoteUser(t, e, db, "vote_bl_voter", td.OrgID, td.CompanyID)

		appID := suppApp(t, db, td.OrgID, authorID, "VOTE-BL-1", models.ConfirmationApproved, models.StatusInWork)
		roundID := suppNewSupplement(t, db, appID, authorID, models.SupplementPending)
		suppVoteApproval(t, db, roundID, voterID, true)
		suppVoteFlag(t, db, appID, &roundID, "X001XX777")

		rec := approve(voterToken, appID, roundID, `{"status":"approved"}`)
		assert.Equal(t, http.StatusConflict, rec.Code, "неподтверждённое предупреждение ЧС не даёт согласовать раунд")
		assert.Equal(t, models.SupplementPending, suppVoteReadRoundByID(t, db, roundID).Status)

		// Отказ гейт не блокирует: отклонить подозрительную добавку можно сразу.
		rec = approve(voterToken, appID, roundID, `{"status":"rejected"}`)
		require.Equal(t, http.StatusOK, rec.Code, "отказ: %s", rec.Body.String())
		assert.Equal(t, models.SupplementRejected, suppVoteReadRoundByID(t, db, roundID).Status)
	})

	t.Run("флаг исходного состава согласованию раунда не мешает", func(t *testing.T) {
		voterID, voterToken := suppVoteUser(t, e, db, "vote_bl_base_voter", td.OrgID, td.CompanyID)

		appID := suppApp(t, db, td.OrgID, authorID, "VOTE-BL-2", models.ConfirmationApproved, models.StatusInWork)
		roundID := suppNewSupplement(t, db, appID, authorID, models.SupplementPending)
		suppVoteApproval(t, db, roundID, voterID, true)
		// Флаг подачи (supplement_id NULL): тот состав прошёл свой круг, голосующий по
		// добавке за него не отвечает.
		suppVoteFlag(t, db, appID, nil, "Y002YY777")

		rec := approve(voterToken, appID, roundID, `{"status":"approved"}`)
		require.Equal(t, http.StatusOK, rec.Code, "голос: %s", rec.Body.String())
		assert.Equal(t, models.SupplementApproved, suppVoteReadRoundByID(t, db, roundID).Status)
		suppVoteAssertApplicationIntact(t, db, appID)
	})
}
