package handlers_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// markerFor возвращает has_unseen_questions заявки из списка /applications глазами токена.
func markerFor(t *testing.T, e *echo.Echo, token string, appID int) bool {
	t.Helper()
	rec := testutil.GET(t, e, "/applications", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	apps := testutil.ParseResponse[[]services.ApplicationWithDetails](t, rec)
	for _, a := range apps {
		if a.ID == appID {
			return a.HasUnseenQuestions
		}
	}
	t.Fatalf("заявка %d не найдена в списке", appID)
	return false
}

// TestQuestions_RoundTrip закрывает #973: согласующий задаёт вопрос (тема+текст+вложение),
// это пишется в историю (question_created) и шлёт уведомление инициатору; вопрос отдаётся
// GET /questions с ФИО автора и вложениями; посторонний получает 403; инициатор ТОЖЕ может
// задать вопрос к своей заявке (#973 followup).
func TestQuestions_RoundTrip(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	senderToken := testutil.RegisterAndLogin(t, e, "q_sender", "pass123", 1, td.OrgID, td.CompanyID)
	senderID := getUserID(t, db, "q_sender")

	// Согласующий = автор вопроса, ФИО для детерминизма author_name.
	testutil.RegisterUser(t, e, "q_resp", "pass123", 1, td.OrgID, td.CompanyID)
	respID := getUserID(t, db, "q_resp")
	require.NoError(t, db.Model(&models.User{}).Where("username = ?", "q_resp").
		Updates(map[string]interface{}{"last_name": "Иванов", "first_name": "Иван", "middle_name": "Иванович"}).Error)
	const wantAuthor = "Иванов Иван Иванович"
	respToken, _ := testutil.LoginUser(t, e, "q_resp", "pass123")

	// Посторонний.
	testutil.RegisterUser(t, e, "q_outsider", "pass123", 1, td.OrgID, td.CompanyID)
	outsiderToken, _ := testutil.LoginUser(t, e, "q_outsider", "pass123")

	uaID := seedUniqueAttachment(t, db, "cars", "cars_q", "Cars Q")
	appID := submitCompleteApplication(t, e, senderToken, "Test Organization", uaID)

	// Делаем q_resp согласующим этой заявки.
	require.NoError(t, db.Exec(`INSERT INTO application_responsible_users
		(application_id, user_id, required_approval, approval_status, created_at, created_by, is_primary)
		VALUES (?, ?, false, 'pending', NOW(), ?, false)`, appID, respID, senderID).Error)

	var attID int
	require.NoError(t, db.Raw("SELECT id FROM attachments WHERE application_id = ? LIMIT 1", appID).Scan(&attID).Error)
	require.NotZero(t, attID)

	// Согласующий задаёт вопрос по вложению.
	const wantSubject = "Прицеп у фуры"
	const wantText = "Заезжает ли фура с прицепом? В списке машин прицепа нет."
	body := fmt.Sprintf(`{"subject":%q,"text":%q,"attachment_ids":[%d]}`, wantSubject, wantText, attID)
	rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/questions", appID), body, testutil.AuthHeader(respToken))
	require.Equal(t, http.StatusCreated, rec.Code, "создание вопроса: %s", rec.Body.String())
	created := testutil.ParseResponse[services.QuestionWithAnswers](t, rec)
	assert.Equal(t, wantSubject, created.Subject)
	assert.Equal(t, wantAuthor, created.AuthorName)
	require.Len(t, created.Attachments, 1)
	assert.Equal(t, attID, created.Attachments[0].ID)

	// История: question_created с текстом в comment.
	rec = testutil.GET(t, e, fmt.Sprintf("/applications/%d/history", appID), testutil.AuthHeader(senderToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	hist := testutil.ParseSlice(t, rec)
	entry := findHistoryEntry(t, hist, "question_created")
	assert.Equal(t, wantText, entry["comment"], "question_created должен нести текст вопроса")

	// Уведомление инициатору: тип application_question, payload с application_id,
	// текст содержит имя автора вопроса (информативность).
	var notif struct {
		Type    *string
		Data    *string
		Message *string
	}
	require.NoError(t, db.Raw("SELECT type, data, message FROM notifications WHERE user_id = ? ORDER BY id DESC LIMIT 1", senderID).Scan(&notif).Error)
	require.NotNil(t, notif.Type)
	assert.Equal(t, "application_question", *notif.Type)
	require.NotNil(t, notif.Data)
	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(*notif.Data), &payload))
	assert.EqualValues(t, appID, payload["application_id"], "payload несёт application_id для навигации")
	require.NotNil(t, notif.Message)
	assert.Contains(t, *notif.Message, wantAuthor, "уведомление называет автора вопроса")

	// GET /questions виден инициатору.
	rec = testutil.GET(t, e, fmt.Sprintf("/applications/%d/questions", appID), testutil.AuthHeader(senderToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	list := testutil.ParseResponse[[]services.QuestionWithAnswers](t, rec)
	require.Len(t, list, 1)
	assert.Equal(t, wantSubject, list[0].Subject)
	require.Len(t, list[0].Attachments, 1)

	// Посторонний: 403 и на чтение, и на создание.
	rec = testutil.GET(t, e, fmt.Sprintf("/applications/%d/questions", appID), testutil.AuthHeader(outsiderToken))
	assert.Equal(t, http.StatusForbidden, rec.Code)
	rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/questions", appID), `{"subject":"x","text":"y"}`, testutil.AuthHeader(outsiderToken))
	assert.Equal(t, http.StatusForbidden, rec.Code)

	// Инициатор ТОЖЕ может задать вопрос к своей заявке (#973 followup).
	rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/questions", appID), `{"subject":"Свой вопрос","text":"уточнение"}`, testutil.AuthHeader(senderToken))
	assert.Equal(t, http.StatusCreated, rec.Code, "инициатор задаёт вопрос к своей заявке: %s", rec.Body.String())
}

// TestQuestions_AnswerAndNotify: ответ отдаётся в треде, НЕ пишется в историю, шлёт
// уведомление автору вопроса (не автору ответа); отвечать может любой с доступом (инициатор).
func TestQuestions_AnswerAndNotify(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	senderToken := testutil.RegisterAndLogin(t, e, "qa_sender", "pass123", 1, td.OrgID, td.CompanyID)
	senderID := getUserID(t, db, "qa_sender")
	testutil.RegisterUser(t, e, "qa_resp", "pass123", 1, td.OrgID, td.CompanyID)
	respID := getUserID(t, db, "qa_resp")
	respToken, _ := testutil.LoginUser(t, e, "qa_resp", "pass123")

	appID := createSimpleApplication(t, e, senderToken, td.OrgID)
	require.NoError(t, db.Exec(`INSERT INTO application_responsible_users
		(application_id, user_id, required_approval, approval_status, created_at, created_by, is_primary)
		VALUES (?, ?, false, 'pending', NOW(), ?, false)`, appID, respID, senderID).Error)

	// Согласующий задаёт вопрос.
	rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/questions", appID), `{"subject":"Тема","text":"Вопрос?"}`, testutil.AuthHeader(respToken))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	q := testutil.ParseResponse[services.QuestionWithAnswers](t, rec)

	// Инициатор (любой с доступом) отвечает.
	rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/questions/%d/answers", appID, q.ID), `{"text":"Да, отвечаю"}`, testutil.AuthHeader(senderToken))
	require.Equal(t, http.StatusCreated, rec.Code, "ответ инициатора: %s", rec.Body.String())

	// История: только question_created, никаких answer-действий.
	rec = testutil.GET(t, e, fmt.Sprintf("/applications/%d/history", appID), testutil.AuthHeader(senderToken))
	hist := testutil.ParseSlice(t, rec)
	actions := historyActionTypes(hist)
	assert.Contains(t, actions, "question_created")
	for _, a := range actions {
		assert.NotContains(t, a, "answer", "ответ не должен попадать в историю")
	}

	// Уведомление об ответе: автору вопроса (resp) есть application_answer, автору ответа (sender) - нет.
	var respAnswerNotif, senderAnswerNotif int64
	db.Raw("SELECT COUNT(*) FROM notifications WHERE user_id = ? AND type = 'application_answer'", respID).Scan(&respAnswerNotif)
	db.Raw("SELECT COUNT(*) FROM notifications WHERE user_id = ? AND type = 'application_answer'", senderID).Scan(&senderAnswerNotif)
	assert.Equal(t, int64(1), respAnswerNotif, "автор вопроса уведомлён об ответе")
	assert.Equal(t, int64(0), senderAnswerNotif, "автор ответа себе не шлёт уведомление")

	// Тред-форма: вопрос несёт вложенный ответ.
	rec = testutil.GET(t, e, fmt.Sprintf("/applications/%d/questions", appID), testutil.AuthHeader(senderToken))
	list := testutil.ParseResponse[[]services.QuestionWithAnswers](t, rec)
	require.Len(t, list, 1)
	require.Len(t, list[0].Answers, 1)
	assert.Equal(t, "Да, отвечаю", list[0].Answers[0].Text)
}

// TestQuestions_Marker: маркер has_unseen_questions в списке заявок (per-user last-seen).
func TestQuestions_Marker(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	senderToken := testutil.RegisterAndLogin(t, e, "qm_sender", "pass123", 1, td.OrgID, td.CompanyID)
	senderID := getUserID(t, db, "qm_sender")
	testutil.RegisterUser(t, e, "qm_resp", "pass123", 1, td.OrgID, td.CompanyID)
	respID := getUserID(t, db, "qm_resp")
	respToken, _ := testutil.LoginUser(t, e, "qm_resp", "pass123")
	testutil.RegisterUser(t, e, "qm_viewer", "pass123", 1, td.OrgID, td.CompanyID)
	viewerID := getUserID(t, db, "qm_viewer")
	viewerToken, _ := testutil.LoginUser(t, e, "qm_viewer", "pass123")

	appID := createSimpleApplication(t, e, senderToken, td.OrgID)
	require.NoError(t, db.Exec(`INSERT INTO application_responsible_users
		(application_id, user_id, required_approval, approval_status, created_at, created_by, is_primary)
		VALUES (?, ?, false, 'pending', NOW(), ?, false)`, appID, respID, senderID).Error)
	require.NoError(t, db.Exec(`INSERT INTO application_viewers (application_id, user_id, created_at, created_by)
		VALUES (?, ?, NOW(), ?)`, appID, viewerID, senderID).Error)

	// Согласующий задаёт вопрос.
	rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/questions", appID), `{"subject":"Т","text":"В?"}`, testutil.AuthHeader(respToken))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	q := testutil.ParseResponse[services.QuestionWithAnswers](t, rec)

	// Читатель видит маркер; автор вопроса - нет (свой вопрос не светит, author-exclusion).
	assert.True(t, markerFor(t, e, viewerToken, appID), "читатель видит новые вопросы")
	assert.False(t, markerFor(t, e, respToken, appID), "автор вопроса свой маркер не видит")

	// Читатель пометил вопрос прочитанным (per-топик) -> маркер гаснет.
	rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/questions/%d/read", appID, q.ID), ``, testutil.AuthHeader(viewerToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.False(t, markerFor(t, e, viewerToken, appID), "после прочтения топика маркер гаснет")

	// Новый ответ -> у читателя снова активность после его прочтения топика.
	rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/questions/%d/answers", appID, q.ID), `{"text":"ответ"}`, testutil.AuthHeader(senderToken))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	assert.True(t, markerFor(t, e, viewerToken, appID), "новый ответ снова зажигает маркер")
}

// TestQuestions_IsNewPerTopic закрывает #973 (индикатор новизны в блоке): флаг is_new per-топик
// в GET /questions. Новый топик - is_new=true; прочитанный - false; недочитанный ОСТАЁТСЯ new
// (per-топик отметка, не одна граница на заявку); свой вопрос - false; новый ответ в прочитанном
// топике снова зажигает is_new.
func TestQuestions_IsNewPerTopic(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	senderToken := testutil.RegisterAndLogin(t, e, "qn_sender", "pass123", 1, td.OrgID, td.CompanyID)
	senderID := getUserID(t, db, "qn_sender")
	testutil.RegisterUser(t, e, "qn_resp", "pass123", 1, td.OrgID, td.CompanyID)
	respID := getUserID(t, db, "qn_resp")
	respToken, _ := testutil.LoginUser(t, e, "qn_resp", "pass123")

	appID := createSimpleApplication(t, e, senderToken, td.OrgID)
	require.NoError(t, db.Exec(`INSERT INTO application_responsible_users
		(application_id, user_id, required_approval, approval_status, created_at, created_by, is_primary)
		VALUES (?, ?, false, 'pending', NOW(), ?, false)`, appID, respID, senderID).Error)

	// Согласующий задаёт ДВА топика.
	rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/questions", appID), `{"subject":"Первый","text":"вопрос 1"}`, testutil.AuthHeader(respToken))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	q1 := testutil.ParseResponse[services.QuestionWithAnswers](t, rec)
	rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/questions", appID), `{"subject":"Второй","text":"вопрос 2"}`, testutil.AuthHeader(respToken))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	q2 := testutil.ParseResponse[services.QuestionWithAnswers](t, rec)

	isNewByID := func(token string) map[int]bool {
		rc := testutil.GET(t, e, fmt.Sprintf("/applications/%d/questions", appID), testutil.AuthHeader(token))
		require.Equal(t, http.StatusOK, rc.Code, rc.Body.String())
		list := testutil.ParseResponse[[]services.QuestionWithAnswers](t, rc)
		m := make(map[int]bool, len(list))
		for _, q := range list {
			m[q.ID] = q.IsNew
		}
		return m
	}

	// Инициатор: оба топика чужие и непрочитанные -> is_new.
	m := isNewByID(senderToken)
	assert.True(t, m[q1.ID], "новый топик 1 is_new для читателя")
	assert.True(t, m[q2.ID], "новый топик 2 is_new для читателя")

	// Автор (resp): свои вопросы не новые.
	ma := isNewByID(respToken)
	assert.False(t, ma[q1.ID], "свой вопрос не is_new для автора")
	assert.False(t, ma[q2.ID], "свой вопрос не is_new для автора")

	// Инициатор прочитал ТОЛЬКО топик 1 -> он гаснет, топик 2 ОСТАЁТСЯ новым (не одна граница).
	rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/questions/%d/read", appID, q1.ID), ``, testutil.AuthHeader(senderToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	m = isNewByID(senderToken)
	assert.False(t, m[q1.ID], "прочитанный топик 1 больше не is_new")
	assert.True(t, m[q2.ID], "недочитанный топик 2 остаётся is_new")

	// Новый ответ в прочитанном топике 1 (от resp) -> снова is_new для инициатора.
	rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/questions/%d/answers", appID, q1.ID), `{"text":"новый ответ"}`, testutil.AuthHeader(respToken))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	m = isNewByID(senderToken)
	assert.True(t, m[q1.ID], "новый чужой ответ снова зажигает is_new прочитанного топика")
}

// TestQuestions_AttachmentForwardRestriction: вложения вопроса уважают пер-вложенный
// пересыл (#680) - автор не сошлётся на невидимое ему вложение, и читатель с ограничением
// не увидит имя невидимого вложения в чужом вопросе.
func TestQuestions_AttachmentForwardRestriction(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	senderToken := testutil.RegisterAndLogin(t, e, "qfr_sender", "pass123", 1, td.OrgID, td.CompanyID)
	senderID := getUserID(t, db, "qfr_sender")
	// Принимающий - видит все вложения (не ограничен пересылом), задаёт вопрос по обоим.
	testutil.RegisterUser(t, e, "qfr_appr", "pass123", 1, td.OrgID, td.CompanyID)
	makeApprover(t, db, "qfr_appr")
	apprToken, _ := testutil.LoginUser(t, e, "qfr_appr", "pass123")
	// Согласующий с ограниченным пересылом (виден только att1).
	testutil.RegisterUser(t, e, "qfr_viewer", "pass123", 1, td.OrgID, td.CompanyID)
	viewerID := getUserID(t, db, "qfr_viewer")
	viewerToken, _ := testutil.LoginUser(t, e, "qfr_viewer", "pass123")

	uaID := seedUniqueAttachment(t, db, "cars", "cars_qfr", "UA")
	appID := submitCompleteApplication(t, e, senderToken, "Test Organization", uaID)

	var att1 int
	require.NoError(t, db.Raw("SELECT id FROM attachments WHERE application_id = ? LIMIT 1", appID).Scan(&att1).Error)
	var att2 int
	require.NoError(t, db.Raw(`INSERT INTO attachments
		(application_id, unique_attachment_id, attachment_type, attachment_name, attachment_display_name,
		 entry_date_from, entry_date_to, entry_time_from, entry_time_to, created_at)
		VALUES (?, ?, 'cars', 'att2', 'Скрытое вложение', '2026-04-01', '2099-12-31', '08:00', '18:00', NOW())
		RETURNING id`, appID, uaID).Scan(&att2).Error)

	require.NoError(t, db.Exec(`INSERT INTO application_responsible_users
		(application_id, user_id, required_approval, approval_status, created_at, created_by, is_primary)
		VALUES (?, ?, false, 'pending', NOW(), ?, false)`, appID, viewerID, senderID).Error)
	// Пересыл viewer'у только att1 -> att2 ему не виден.
	require.NoError(t, db.Exec(`INSERT INTO forward_attachments (application_id, recipient_user_id, attachment_id, created_at)
		VALUES (?, ?, ?, NOW())`, appID, viewerID, att1).Error)

	// Принимающий (видит все) задаёт вопрос по обоим вложениям.
	body := fmt.Sprintf(`{"subject":"Оба","text":"по двум вложениям","attachment_ids":[%d,%d]}`, att1, att2)
	rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/questions", appID), body, testutil.AuthHeader(apprToken))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	// Чтение фильтра: viewer (ограничен) видит в вопросе только att1, не att2.
	rec = testutil.GET(t, e, fmt.Sprintf("/applications/%d/questions", appID), testutil.AuthHeader(viewerToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	list := testutil.ParseResponse[[]services.QuestionWithAnswers](t, rec)
	require.Len(t, list, 1)
	require.Len(t, list[0].Attachments, 1, "скрытое вложение не должно быть видно ограниченному читателю")
	assert.Equal(t, att1, list[0].Attachments[0].ID)

	// Создание фильтра: viewer задаёт вопрос со ссылкой на att1+att2 -> сохранится только att1.
	body = fmt.Sprintf(`{"subject":"Мой","text":"вопрос","attachment_ids":[%d,%d]}`, att1, att2)
	rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/questions", appID), body, testutil.AuthHeader(viewerToken))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	created := testutil.ParseResponse[services.QuestionWithAnswers](t, rec)
	require.Len(t, created.Attachments, 1, "автор не может сослаться на невидимое ему вложение")
	assert.Equal(t, att1, created.Attachments[0].ID)
}

// TestQuestions_AnswerWrongApplication: ответить на вопрос через чужой appID -> 404.
func TestQuestions_AnswerWrongApplication(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	senderToken := testutil.RegisterAndLogin(t, e, "qw_sender", "pass123", 1, td.OrgID, td.CompanyID)
	senderID := getUserID(t, db, "qw_sender")
	testutil.RegisterUser(t, e, "qw_resp", "pass123", 1, td.OrgID, td.CompanyID)
	respID := getUserID(t, db, "qw_resp")
	respToken, _ := testutil.LoginUser(t, e, "qw_resp", "pass123")

	appID := createSimpleApplication(t, e, senderToken, td.OrgID)
	otherAppID := createSimpleApplication(t, e, senderToken, td.OrgID)
	require.NoError(t, db.Exec(`INSERT INTO application_responsible_users
		(application_id, user_id, required_approval, approval_status, created_at, created_by, is_primary)
		VALUES (?, ?, false, 'pending', NOW(), ?, false)`, appID, respID, senderID).Error)

	rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/questions", appID), `{"subject":"Т","text":"В"}`, testutil.AuthHeader(respToken))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	q := testutil.ParseResponse[services.QuestionWithAnswers](t, rec)

	// Ответить на этот вопрос через ДРУГУЮ заявку -> 404.
	rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/questions/%d/answers", otherAppID, q.ID), `{"text":"x"}`, testutil.AuthHeader(senderToken))
	assert.Equal(t, http.StatusNotFound, rec.Code, "ответ через чужую заявку не проходит")

	// Пометить прочитанным вопрос через ДРУГУЮ заявку -> 404 (вопрос ей не принадлежит).
	rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/questions/%d/read", otherAppID, q.ID), ``, testutil.AuthHeader(senderToken))
	assert.Equal(t, http.StatusNotFound, rec.Code, "пометка прочтения через чужую заявку не проходит")
}

// TestQuestions_Unauthorized: без токена все Q&A-эндпоинты отдают 401.
func TestQuestions_Unauthorized(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	rec := testutil.GET(t, e, "/applications/1/questions", nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	rec = testutil.POST(t, e, "/applications/1/questions", `{"subject":"x","text":"y"}`, nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	rec = testutil.POST(t, e, "/applications/1/questions/1/answers", `{"text":"y"}`, nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	rec = testutil.POST(t, e, "/applications/1/questions/seen", ``, nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	rec = testutil.POST(t, e, "/applications/1/questions/1/read", ``, nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	_ = db
}
