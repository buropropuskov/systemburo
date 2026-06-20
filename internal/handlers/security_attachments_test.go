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
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// HTTP-тесты эндпоинтов "Доступные мне" (#706, срез BE-S4). Реюз DB-хелперов secWorld из
// security_visibility_test.go (тот же пакет handlers_test, единственный DB-тест-бинарь проекта).

const secHTTPPassword = "guardpass_long_enough_for_login"

// secHTTPWorld - поднятый echo + БД + secWorld (DB-хелперы) + логинабельные токены трёх ролей.
type secHTTPWorld struct {
	e          *echo.Echo
	w          secWorld
	guardToken string
	userToken  string
	adminToken string
}

func setupSecurityHTTP(t *testing.T) secHTTPWorld {
	t.Helper()
	e, db, cleanup := testutil.SetupTestApp(t)
	t.Cleanup(cleanup)
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	secTypeID := secUserTypeIDByCode(t, db, "security")
	userTypeID := secUserTypeIDByCode(t, db, "user")

	guardToken := testutil.RegisterAndLogin(t, e, "guardhttp", secHTTPPassword, secTypeID, td.OrgID, 0)
	userToken := testutil.RegisterAndLogin(t, e, "userhttp", secHTTPPassword, userTypeID, td.OrgID, 0)
	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, 0)

	guardID := secUserIDByUsername(t, db, "guardhttp")
	senderID := secUserIDByUsername(t, db, "userhttp")

	svc := services.NewApplicationService(db, nil, nil, nil, nil)
	w := secWorld{db: db, svc: svc, orgID: td.OrgID, senderID: senderID, guardID: guardID}
	return secHTTPWorld{e: e, w: w, guardToken: guardToken, userToken: userToken, adminToken: adminToken}
}

func secUserIDByUsername(t *testing.T, db *gorm.DB, username string) int {
	t.Helper()
	var id int
	require.NoError(t, db.Table("users").Where("username = ?", username).Select("id").Scan(&id).Error)
	require.NotZero(t, id, "user %q not found", username)
	return id
}

// secMetaEnvelope читает meta-блок пагинированного ответа (testutil.ParseResponse даёт только data).
type secMetaEnvelope struct {
	Success bool `json:"success"`
	Meta    struct {
		Total   int64 `json:"total"`
		Page    int   `json:"page"`
		PerPage int   `json:"per_page"`
	} `json:"meta"`
}

// secDetailResponse зеркалит handlers.availableAttachmentDetail (тип хендлера неэкспортируемый).
type secDetailResponse struct {
	Attachment services.AvailableAttachment  `json:"attachment"`
	Cars       []services.CarWithPlaces      `json:"cars"`
	Employees  []services.EmployeeWithTables `json:"employees"`
	Items      []services.ItemInfo           `json:"items"`
}

func secGetDetail(t *testing.T, h secHTTPWorld, attID int, token string) secDetailResponse {
	t.Helper()
	rec := testutil.GET(t, h.e, fmt.Sprintf("/applications/available-attachments/%d", attID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return testutil.ParseResponse[secDetailResponse](t, rec)
}

func TestAvailableAttachments_RoleGate(t *testing.T) {
	h := setupSecurityHTTP(t)

	rec := testutil.GET(t, h.e, "/applications/available-attachments", testutil.AuthHeader(h.userToken))
	require.Equal(t, http.StatusForbidden, rec.Code, "обычный пользователь не имеет доступа: %s", rec.Body.String())

	rec = testutil.GET(t, h.e, "/applications/available-attachments", testutil.AuthHeader(h.guardToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rows := testutil.ParseResponse[[]services.AvailableAttachment](t, rec)
	require.Empty(t, rows, "у охранника нет назначенных мест - список пуст")

	rec = testutil.GET(t, h.e, "/applications/available-attachments", testutil.AuthHeader(h.adminToken))
	require.Equal(t, http.StatusOK, rec.Code, "супер-админ имеет доступ: %s", rec.Body.String())
}

func TestAvailableAttachments_PaginationMeta(t *testing.T) {
	h := setupSecurityHTTP(t)
	w := h.w

	place := w.newUnloadPlace(t, "Склад А", true)
	w.assignUnloadPlace(t, place)
	app := w.newApp(t, models.ConfirmationApproved)
	for i := 0; i < 5; i++ {
		att := w.newAttachment(t, app, "cars")
		w.attachPlace(t, att, place)
	}

	rec := testutil.GET(t, h.e, "/applications/available-attachments?page=1&per_page=2", testutil.AuthHeader(h.guardToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	rows := testutil.ParseResponse[[]services.AvailableAttachment](t, rec)
	require.Len(t, rows, 2, "страница ограничена per_page")

	var env secMetaEnvelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env), rec.Body.String())
	require.EqualValues(t, 5, env.Meta.Total, "total считает все совпадения, не размер страницы")
	require.Equal(t, 1, env.Meta.Page)
	require.Equal(t, 2, env.Meta.PerPage)
}

func TestAvailableAttachmentDetail_Access(t *testing.T) {
	h := setupSecurityHTTP(t)
	w := h.w

	myPlace := w.newUnloadPlace(t, "Склад А", true)
	otherPlace := w.newUnloadPlace(t, "Склад Б", true)
	w.assignUnloadPlace(t, myPlace)

	app := w.newApp(t, models.ConfirmationApproved)
	ownAtt := w.newAttachment(t, app, "cars")
	w.attachPlace(t, ownAtt, myPlace)
	foreignAtt := w.newAttachment(t, app, "cars")
	w.attachPlace(t, foreignAtt, otherPlace)

	detail := secGetDetail(t, h, ownAtt, h.guardToken)
	require.Equal(t, ownAtt, detail.Attachment.AttachmentID)
	require.Equal(t, "cars", detail.Attachment.AttachmentType)
	require.Equal(t, app, detail.Attachment.ApplicationID, "деталь несёт инфо родительской заявки")

	rec := testutil.GET(t, h.e, fmt.Sprintf("/applications/available-attachments/%d", foreignAtt), testutil.AuthHeader(h.guardToken))
	require.Equal(t, http.StatusForbidden, rec.Code, "вложение на чужом месте недоступно охраннику")

	rec = testutil.GET(t, h.e, fmt.Sprintf("/applications/available-attachments/%d", ownAtt), testutil.AuthHeader(h.userToken))
	require.Equal(t, http.StatusForbidden, rec.Code, "обычный пользователь не имеет доступа к детали")

	rec = testutil.GET(t, h.e, fmt.Sprintf("/applications/available-attachments/%d", foreignAtt), testutil.AuthHeader(h.adminToken))
	require.Equal(t, http.StatusOK, rec.Code, "супер-админ видит любое подтверждённое вложение: %s", rec.Body.String())
}

func TestAvailableAttachmentDetail_ContentByType(t *testing.T) {
	h := setupSecurityHTTP(t)
	w := h.w

	place := w.newUnloadPlace(t, "Склад А", true)
	table := w.newPeopleTable(t, "Проходная 1")
	w.assignUnloadPlace(t, place)
	w.assignTable(t, table)

	app := w.newApp(t, models.ConfirmationApproved)

	carsAtt := w.newAttachment(t, app, "cars")
	w.attachPlace(t, carsAtt, place)
	num := "А123ВС777"
	require.NoError(t, w.db.Create(&models.Car{AttachmentID: carsAtt, CarNumber: &num}).Error)

	itemsAtt := w.newAttachment(t, app, "items")
	w.attachPlace(t, itemsAtt, place)
	iname := "Ноутбук"
	icount := 2
	require.NoError(t, w.db.Create(&models.Item{AttachmentID: itemsAtt, Name: &iname, Count: &icount}).Error)

	peopleAtt := w.newAttachment(t, app, "people")
	w.attachEmployeeWithTable(t, peopleAtt, table)

	carsDetail := secGetDetail(t, h, carsAtt, h.guardToken)
	require.Len(t, carsDetail.Cars, 1, "cars-вложение возвращает автомобили")
	require.Empty(t, carsDetail.Employees)
	require.Empty(t, carsDetail.Items)

	itemsDetail := secGetDetail(t, h, itemsAtt, h.guardToken)
	require.Len(t, itemsDetail.Items, 1, "items-вложение возвращает ТМЦ")
	require.Empty(t, itemsDetail.Cars)

	peopleDetail := secGetDetail(t, h, peopleAtt, h.guardToken)
	require.Len(t, peopleDetail.Employees, 1, "people-вложение возвращает сотрудников")
	require.Empty(t, peopleDetail.Cars)
}
