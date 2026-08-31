package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Замки видимости заметки бюро. Заметка - рабочий стикер принимающих о том, почему
// заявка не сделана; её текст не должен попасть ни к заявителю, ни к согласующему, ни
// к получателю пересылки, ни в ленту истории заявки, которую заявитель открывает
// наравне со всеми. Регрессия здесь тихая: утечка не роняет ни один другой тест и
// заметна только тому, от кого прятали.

const bureauNoteText = "Ждём паспорт водителя, без него на КПП не пустят"

// bureauNoteScene - заявка с сохранённой заметкой и четыре человека вокруг неё:
// автор заявки, принимающий, обязательный согласующий и получатель пересылки. Все
// четверо проходят CanAccessApplication и получают деталь заявки - различать их
// обязана только заметка.
type bureauNoteScene struct {
	td            testutil.TestData
	appID         int
	senderToken   string
	approverToken string
	approvalToken string
	viewerToken   string
}

func setupBureauNoteScene(t *testing.T, e *echo.Echo, db *gorm.DB, prefix string) bureauNoteScene {
	t.Helper()
	td := testutil.SeedTestData(t, db)

	senderToken := testutil.RegisterAndLogin(t, e, prefix+"sender", "pass123", 1, td.OrgID, td.CompanyID)
	appID := createSimpleApplication(t, e, senderToken, td.OrgID)

	approverToken := testutil.RegisterAndLogin(t, e, prefix+"approver", "pass123", 1, td.OrgID, td.CompanyID)
	makeApprover(t, db, prefix+"approver")
	// ФИО принимающему: регистрация его не заводит, а format_full_name без него
	// вернул бы пустую строку, и «кто оставил заметку» проверять было бы нечем.
	require.NoError(t, db.Model(&models.User{}).
		Where("id = ?", getUserID(t, db, prefix+"approver")).
		Updates(map[string]interface{}{"last_name": "Петров", "first_name": "Пётр"}).Error)

	approvalToken := testutil.RegisterAndLogin(t, e, prefix+"approval", "pass123", 1, td.OrgID, td.CompanyID)
	viewerToken := testutil.RegisterAndLogin(t, e, prefix+"viewer", "pass123", 1, td.OrgID, td.CompanyID)

	fwd := fmt.Sprintf(
		`{"users":[{"user_id":%d,"required_approval":true,"can_view":false},{"user_id":%d,"required_approval":false,"can_view":true}]}`,
		getUserID(t, db, prefix+"approval"), getUserID(t, db, prefix+"viewer"))
	rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/forward", appID), fwd, testutil.AuthHeader(senderToken))
	require.Equal(t, http.StatusOK, rec.Code, "forward: %s", rec.Body.String())

	body := fmt.Sprintf(`{"note":%q}`, bureauNoteText)
	rec = testutil.PUT(t, e, fmt.Sprintf("/applications/%d/bureau-note", appID), body, testutil.AuthHeader(approverToken))
	require.Equal(t, http.StatusOK, rec.Code, "сохранение заметки принимающим: %s", rec.Body.String())

	return bureauNoteScene{
		td:            td,
		appID:         appID,
		senderToken:   senderToken,
		approverToken: approverToken,
		approvalToken: approvalToken,
		viewerToken:   viewerToken,
	}
}

// detailsBody возвращает сырое тело GET /applications/:id/details. Проверяем именно
// сырой ответ, а не разобранное поле: утечь заметка может любым ключом, в том числе
// новым, о котором тест не знает.
func detailsBody(t *testing.T, e *echo.Echo, appID int, token string) string {
	t.Helper()
	rec := testutil.GET(t, e, fmt.Sprintf("/applications/%d/details", appID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "details: %s", rec.Body.String())
	return rec.Body.String()
}

// TestBureauNote_VisibleOnlyToApprover: заметка едет в деталь заявки принимающему и
// никому больше, хотя деталь открывается всем четверым.
func TestBureauNote_VisibleOnlyToApprover(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	sc := setupBureauNoteScene(t, e, db, "bnv")

	t.Run("approver_sees", func(t *testing.T) {
		rec := testutil.GET(t, e, fmt.Sprintf("/applications/%d/details", sc.appID), testutil.AuthHeader(sc.approverToken))
		require.Equal(t, http.StatusOK, rec.Code, "details: %s", rec.Body.String())

		data := testutil.ParseMap(t, rec)
		note, ok := data["bureau_note"].(map[string]interface{})
		require.True(t, ok, "принимающему заметка приходит объектом: %s", rec.Body.String())
		assert.Equal(t, bureauNoteText, note["text"], "текст заметки")
		assert.Equal(t, "Петров Пётр", note["author_name"], "кто оставил заметку")
		assert.NotEmpty(t, note["updated_at"], "когда заметку правили")
	})

	// Заявитель, согласующий и получатель пересылки - три разных пути в
	// CanAccessApplication, и каждый обязан привести к ответу без заметки.
	for name, token := range map[string]string{
		"sender_does_not_see":   sc.senderToken,
		"approval_does_not_see": sc.approvalToken,
		"viewer_does_not_see":   sc.viewerToken,
	} {
		t.Run(name, func(t *testing.T) {
			body := detailsBody(t, e, sc.appID, token)
			assert.NotContains(t, body, bureauNoteText, "текст заметки в ответе детали: %s", body)
			assert.NotContains(t, body, "bureau_note", "ключ заметки в ответе детали: %s", body)
		})
	}
}

// TestBureauNote_SuperAdminWithoutRoleDoesNotSee: супер-администратор проходит
// CanAccessApplication первой же веткой, но заметку видят принимающие, а не все, у кого
// есть доступ. Понадобилась - заводит себя принимающим, это его собственный раздел.
func TestBureauNote_SuperAdminWithoutRoleDoesNotSee(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	sc := setupBureauNoteScene(t, e, db, "bnsa")

	// Признак супер-админа едет в токене, поэтому вход только ПОСЛЕ выставления флага:
	// у выданного раньше токена в контексте остаётся is_super_admin=false.
	testutil.RegisterUser(t, e, "bnsasuper", "pass123", 1, sc.td.OrgID, sc.td.CompanyID)
	require.NoError(t, db.Model(&models.User{}).
		Where("id = ?", getUserID(t, db, "bnsasuper")).
		Update("is_super_admin", true).Error)
	superToken, _ := testutil.LoginUser(t, e, "bnsasuper", "pass123")

	body := detailsBody(t, e, sc.appID, superToken)
	assert.NotContains(t, body, bureauNoteText, "супер-админ без роли принимающего заметку не видит: %s", body)

	// А став принимающим - видит: гейт именно роль, а не «кто угодно кроме админа».
	makeApprover(t, db, "bnsasuper")
	assert.Contains(t, detailsBody(t, e, sc.appID, superToken), bureauNoteText,
		"заведя себя принимающим, супер-админ заметку видит")
}

// TestBureauNote_NotInApplicationHistory: главная ловушка задачи. GetApplicationHistory
// тянет все строки audit_log[application] без фильтра по смотрящему и отдаёт их любому,
// кто проходит CanAccessApplication, включая ЗАЯВИТЕЛЯ. Запись заметки под этим типом
// сущности показала бы её текст тому, от кого её прячут.
func TestBureauNote_NotInApplicationHistory(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	sc := setupBureauNoteScene(t, e, db, "bnh")

	for name, token := range map[string]string{
		"sender":   sc.senderToken,
		"approver": sc.approverToken,
		"approval": sc.approvalToken,
		"viewer":   sc.viewerToken,
	} {
		t.Run(name, func(t *testing.T) {
			rec := testutil.GET(t, e, fmt.Sprintf("/applications/%d/history", sc.appID), testutil.AuthHeader(token))
			require.Equal(t, http.StatusOK, rec.Code, "history: %s", rec.Body.String())
			assert.NotContains(t, rec.Body.String(), bureauNoteText, "заметка просочилась в ленту заявки")
		})
	}

	// Дубль проверки на уровне хранилища: лента строится из audit_log[application], и
	// пустая строка там - причина, по которой её нет в ответе. Без этого замок краснел
	// бы и от невинной смены формы ответа истории.
	var n int64
	require.NoError(t, db.Model(&models.AuditLog{}).
		Where("entity_type = ? AND details::text LIKE ?", models.AuditEntityApplication, "%"+bureauNoteText+"%").
		Count(&n).Error)
	assert.Zero(t, n, "текст заметки записан в audit_log под entity_type=application")

	// И нигде в журнале вообще: отдельный тип сущности мы тоже не заводили - след
	// заметки живёт в bureau_note_author_id/bureau_note_updated_at самой заявки.
	require.NoError(t, db.Model(&models.AuditLog{}).
		Where("details::text LIKE ?", "%"+bureauNoteText+"%").
		Count(&n).Error)
	assert.Zero(t, n, "текст заметки попал в audit_log")
}

// TestBureauNote_OnlyApproverCanSave: сохранение под тем же гейтом, что и чтение.
// Без замка на запись прятать заметку от заявителя бессмысленно: он бы её просто
// перезаписал и прочитал ответ метода.
func TestBureauNote_OnlyApproverCanSave(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	sc := setupBureauNoteScene(t, e, db, "bns")

	for name, token := range map[string]string{
		"sender":   sc.senderToken,
		"approval": sc.approvalToken,
		"viewer":   sc.viewerToken,
	} {
		t.Run(name+"_forbidden", func(t *testing.T) {
			rec := testutil.PUT(t, e, fmt.Sprintf("/applications/%d/bureau-note", sc.appID),
				`{"note":"подменённая заметка"}`, testutil.AuthHeader(token))
			assert.Equal(t, http.StatusForbidden, rec.Code, "не принимающий сохранять заметку не вправе: %s", rec.Body.String())
		})
	}

	// Отказ должен быть настоящим, а не «ответили 403 и всё равно записали».
	var stored *string
	require.NoError(t, db.Model(&models.Application{}).
		Where("id = ?", sc.appID).Select("bureau_note").Scan(&stored).Error)
	require.NotNil(t, stored)
	assert.Equal(t, bureauNoteText, *stored, "заметка пережила чужие попытки записи")
}

// TestBureauNote_ApproverEditsAndClears: принимающий переписывает и снимает заметку.
// Снятие обязано убрать и автора со временем - иначе в карточке осталась бы строка
// «изменил такой-то тогда-то» без самой заметки.
func TestBureauNote_ApproverEditsAndClears(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	sc := setupBureauNoteScene(t, e, db, "bne")
	path := fmt.Sprintf("/applications/%d/bureau-note", sc.appID)

	rec := testutil.PUT(t, e, path, `{"note":"Заявитель обещал дозагрузить доверенность"}`, testutil.AuthHeader(sc.approverToken))
	require.Equal(t, http.StatusOK, rec.Code, "правка заметки: %s", rec.Body.String())
	assert.Contains(t, detailsBody(t, e, sc.appID, sc.approverToken), "дозагрузить доверенность")

	// Пустой текст снимает заметку. Пробелы - тот же пустой текст: иначе случайный
	// пробел оставлял бы заметку "непустой" и она висела бы в карточке невидимой строкой.
	rec = testutil.PUT(t, e, path, `{"note":"   "}`, testutil.AuthHeader(sc.approverToken))
	require.Equal(t, http.StatusOK, rec.Code, "снятие заметки: %s", rec.Body.String())

	data := testutil.ParseMap(t, testutil.GET(t, e, fmt.Sprintf("/applications/%d/details", sc.appID), testutil.AuthHeader(sc.approverToken)))
	require.Contains(t, data, "bureau_note", "снятая заметка приходит ключом со значением null - иначе фронт не затрёт свою копию")
	assert.Nil(t, data["bureau_note"], "заметка снята")

	var row struct {
		BureauNote          *string
		BureauNoteAuthorID  *int
		BureauNoteUpdatedAt *string
	}
	require.NoError(t, db.Table("applications").
		Select("bureau_note, bureau_note_author_id, bureau_note_updated_at").
		Where("id = ?", sc.appID).Scan(&row).Error)
	assert.Nil(t, row.BureauNote, "текст очищен")
	assert.Nil(t, row.BureauNoteAuthorID, "автор очищен вместе с текстом")
	assert.Nil(t, row.BureauNoteUpdatedAt, "время очищено вместе с текстом")
}
