package handlers_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/labstack/echo/v5"
	echomw "github.com/labstack/echo/v5/middleware"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// All comparisons read both sides through pgx, retaining its timestamp precision
// and location. Full row snapshots catch changed markers, not just extra rows.
type questionsTransactionState struct {
	Counts      map[string]int64
	Application models.Application
	Questions   []models.ApplicationQuestion
	Answers     []models.ApplicationAnswer
	Attachments []models.ApplicationQuestionAttachment
	Views       []models.ApplicationQuestionView
	Reads       []models.ApplicationQuestionRead
}

func questionsTransactionSnapshot(t *testing.T, db *gorm.DB, appID int) questionsTransactionState {
	t.Helper()
	state := questionsTransactionState{Counts: transactionIntegritySnapshot(t, db)}
	require.NoError(t, db.First(&state.Application, appID).Error)
	require.NoError(t, db.Order("id").Find(&state.Questions).Error)
	require.NoError(t, db.Order("id").Find(&state.Answers).Error)
	require.NoError(t, db.Order("id").Find(&state.Attachments).Error)
	require.NoError(t, db.Order("id").Find(&state.Views).Error)
	require.NoError(t, db.Order("id").Find(&state.Reads).Error)
	return state
}

// Raw SQL errors are injected before driver execution: PostgreSQL-aborted state
// must not accidentally enforce the rollback that this test is meant to prove.
func questionsTransactionRawFault(t *testing.T, db *gorm.DB, prefix string, at int, fault func(*gorm.DB)) *int {
	t.Helper()
	name := "test:questions_transaction:" + strings.ReplaceAll(t.Name(), "/", ":")
	calls := 0
	require.NoError(t, db.Callback().Raw().Before("gorm:raw").Register(name, func(tx *gorm.DB) {
		sql := strings.ToLower(strings.Join(strings.Fields(tx.Statement.SQL.String()), " "))
		if strings.HasPrefix(sql, prefix) {
			calls++
			if calls == at {
				fault(tx)
			}
		}
	}))
	t.Cleanup(func() { require.NoError(t, db.Callback().Raw().Remove(name)) })
	return &calls
}

func TestQuestionsTransactionIntegrity_RollbackPanicAndMandatoryWrites(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	e.Use(echomw.Recover()) // SetupTestApp does not install production recovery.
	const sender, participant = "q_integrity_sender", "q_integrity_participant"
	senderToken := testutil.RegisterAndLogin(t, e, sender, "pass123", 1, td.OrgID, td.CompanyID)
	participantToken := testutil.RegisterAndLogin(t, e, participant, "pass123", 1, td.OrgID, td.CompanyID)
	senderID, participantID := getUserID(t, db, sender), getUserID(t, db, participant)
	appID := suppApp(t, db, td.OrgID, senderID, "QUESTIONS-INTEGRITY", models.ConfirmationApproved, models.StatusProcessing)
	// Fixed past instants make every marker assertion independent of wall-clock day.
	oldTime := time.Date(2000, 1, 1, 12, 0, 0, 0, time.UTC)
	require.NoError(t, db.Create(&models.ApplicationResponsibleUser{ApplicationID: appID,
		UserID: participantID, ApprovalStatus: testutil.Ptr("approved"), CreatedAt: oldTime}).Error)
	attIDs := []int{suppAttachment(t, db, appID, "people", "2099-12-31"), suppAttachment(t, db, appID, "cars", "2099-12-31")}
	oldQuestion := models.ApplicationQuestion{ApplicationID: appID, AuthorUserID: participantID,
		Subject: "Original topic", Text: "Original question", CreatedAt: oldTime}
	require.NoError(t, db.Create(&oldQuestion).Error)
	require.NoError(t, db.Create(&models.ApplicationAnswer{QuestionID: oldQuestion.ID, ApplicationID: appID,
		AuthorUserID: participantID, Text: "Original answer", CreatedAt: oldTime}).Error)
	require.NoError(t, db.Create(&models.ApplicationQuestionAttachment{QuestionID: oldQuestion.ID, AttachmentID: attIDs[0]}).Error)
	for _, id := range []int{senderID, participantID} {
		require.NoError(t, db.Create(&models.ApplicationQuestionView{ApplicationID: appID, UserID: id, LastSeenAt: oldTime}).Error)
		require.NoError(t, db.Create(&models.ApplicationQuestionRead{QuestionID: oldQuestion.ID, UserID: id, ReadAt: oldTime}).Error)
	}
	questionReq := services.CreateQuestionRequest{Subject: "  Integrity topic  ", Text: "  Integrity question  ", AttachmentIDs: attIDs}
	answerReq := services.CreateAnswerRequest{Text: "  Integrity answer  "}

	// Each panic occurs after an observable primary insert. The answer failure
	// occurs even later, after the existing application marker has been updated.
	for _, tc := range []struct{ operation, mode string }{
		{"question", "second_attachment_error"}, {"question", "mandatory_audit_error"},
		{"question", "view_error"}, {"answer", "read_error_after_view"},
		{"question", "panic_service"}, {"question", "panic_HTTP"},
		{"answer", "panic_service"}, {"answer", "panic_HTTP"},
	} {
		t.Run(tc.operation+"_"+tc.mode, func(t *testing.T) {
			before := questionsTransactionSnapshot(t, db, appID)
			var calls *int
			fault := errors.New("synthetic Q&A write error")
			panicMode := strings.HasPrefix(tc.mode, "panic_")
			checkPrimary := func(tx *gorm.DB) {
				readTx := tx.Session(&gorm.Session{NewDB: true})
				table, baseline := "application_questions", len(before.Questions)
				if tc.operation == "answer" {
					table, baseline = "application_answers", len(before.Answers)
				}
				var count int64
				require.NoError(t, readTx.Table(table).Count(&count).Error)
				require.EqualValues(t, baseline+1, count, "fault must follow the primary insert")
				if tc.mode == "second_attachment_error" {
					var links int64
					require.NoError(t, tx.Session(&gorm.Session{NewDB: true}).Model(&models.ApplicationQuestionAttachment{}).Count(&links).Error)
					require.EqualValues(t, len(before.Attachments)+1, links, "the first question attachment must exist before the second write fails")
				}
				if tc.operation == "answer" {
					var view models.ApplicationQuestionView
					require.NoError(t, tx.Session(&gorm.Session{NewDB: true}).Where("application_id = ? AND user_id = ?", appID, senderID).First(&view).Error)
					require.False(t, view.LastSeenAt.Equal(oldTime), "answer must have updated the view marker before the read fault")
				}
			}
			inject := func(tx *gorm.DB) {
				checkPrimary(tx)
				if panicMode {
					panic("synthetic Q&A transaction panic")
				}
				tx.AddError(fault)
			}
			if tc.mode == "mandatory_audit_error" {
				name := "test:question_mandatory_audit:" + strings.ReplaceAll(t.Name(), "/", ":")
				n := 0
				calls = &n
				require.NoError(t, db.Callback().Create().Before("gorm:create").Register(name, func(tx *gorm.DB) {
					entry, ok := tx.Statement.Dest.(*models.AuditLog)
					if ok && entry.Action == "question_created" {
						n++
						inject(tx)
					}
				}))
				t.Cleanup(func() { require.NoError(t, db.Callback().Create().Remove(name)) })
			} else {
				prefix, at := "insert into application_question_attachments", 1
				if tc.mode == "second_attachment_error" {
					at = 2
				}
				if tc.mode == "view_error" {
					prefix = "insert into application_question_views"
				}
				if tc.operation == "answer" {
					prefix = "insert into application_question_reads"
				}
				calls = questionsTransactionRawFault(t, db, prefix, at, inject)
			}
			publisher := &fakePublisher{}
			svc := transactionIntegrityService(db, publisher)
			if tc.mode == "panic_HTTP" {
				url, token, body := fmt.Sprintf("/applications/%d/questions", appID), participantToken, transactionIntegrityJSON(t, questionReq)
				if tc.operation == "answer" {
					url, token, body = fmt.Sprintf("/applications/%d/questions/%d/answers", appID, oldQuestion.ID), senderToken, transactionIntegrityJSON(t, answerReq)
				}
				rec := testutil.POST(t, e, url, body, testutil.AuthHeader(token))
				require.Equal(t, http.StatusInternalServerError, rec.Code)
			} else {
				invoke := func() {
					if tc.operation == "question" {
						resp, err := svc.CreateApplicationQuestion(context.Background(), participant, appID, false, questionReq)
						require.Error(t, err)
						require.Nil(t, resp)
						var httpErr *echo.HTTPError
						require.ErrorAs(t, err, &httpErr)
						require.Equal(t, http.StatusInternalServerError, httpErr.Code)
					} else {
						resp, err := svc.CreateApplicationAnswer(context.Background(), sender, appID, oldQuestion.ID, answerReq)
						require.Error(t, err)
						require.Nil(t, resp)
						var httpErr *echo.HTTPError
						require.ErrorAs(t, err, &httpErr)
						require.Equal(t, http.StatusInternalServerError, httpErr.Code)
					}
				}
				if panicMode {
					require.PanicsWithValue(t, "synthetic Q&A transaction panic", invoke)
				} else {
					invoke()
				}
				require.Empty(t, publisher.events, "connected service publisher must not emit after rollback")
			}
			wantCalls := 1
			if tc.mode == "second_attachment_error" {
				wantCalls = 2
			}
			require.Equal(t, wantCalls, *calls)
			require.Equal(t, before, questionsTransactionSnapshot(t, db, appID), "rollback must restore all prior Q&A rows and marker values, with no success notifications")
		})
	}

	t.Run("successful_question_and_answer_keep_audit_and_notifications", func(t *testing.T) {
		pub := &fakePublisher{}
		svc := transactionIntegrityService(db, pub)
		question, err := svc.CreateApplicationQuestion(context.Background(), participant, appID, false, questionReq)
		require.NoError(t, err)
		require.NotNil(t, question)
		require.Equal(t, "Integrity topic", question.Subject)
		require.Equal(t, "Integrity question", question.Text)
		require.Len(t, question.Attachments, 2)
		var historyBeforeAnswer int64
		require.NoError(t, db.Table("audit_log").Where("entity_type = ? AND entity_id = ?", models.AuditEntityApplication, appID).Count(&historyBeforeAnswer).Error)
		require.EqualValues(t, 1, historyBeforeAnswer, "successful question retains its mandatory history")
		require.NotEmpty(t, pub.events)
		pub.reset()
		answer, err := svc.CreateApplicationAnswer(context.Background(), sender, appID, question.ID, answerReq)
		require.NoError(t, err)
		require.NotNil(t, answer)
		require.Equal(t, "Integrity answer", answer.Text)
		var historyAfterAnswer int64
		require.NoError(t, db.Table("audit_log").Where("entity_type = ? AND entity_id = ?", models.AuditEntityApplication, appID).Count(&historyAfterAnswer).Error)
		require.Equal(t, historyBeforeAnswer, historyAfterAnswer, "answer must not acquire question-style history")
		var read models.ApplicationQuestionRead
		require.NoError(t, db.Where("question_id = ? AND user_id = ?", question.ID, senderID).First(&read).Error)
		require.False(t, read.ReadAt.IsZero())
		require.NotEmpty(t, pub.events)
		for _, expected := range []struct {
			userID int
			kind   string
		}{
			{senderID, services.NotificationTypeApplicationQuestion}, {participantID, services.NotificationTypeApplicationAnswer},
		} {
			var notices int64
			require.NoError(t, db.Table("notifications").Where("user_id = ? AND type = ? AND data->>'question_id' = ?", expected.userID, expected.kind, fmt.Sprint(question.ID)).Count(&notices).Error)
			require.EqualValues(t, 1, notices)
		}
	})
}
