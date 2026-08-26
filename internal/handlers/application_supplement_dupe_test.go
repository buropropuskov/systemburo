package handlers_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Повторное добавление того же человека в то же вложение (#1685).
//
// Само по себе это выглядит безобидно, но таблица поста схлопывает сотрудников по
// паспорту и при равных сроках оставляет строку с большим идентификатором - то есть
// добавленную. Прежняя скрывается целиком вместе со своим набором постов, и человек
// молча оказывается на других проходных. Поэтому дубль отклоняется на входе, а менять
// посты уже допущенному следует назначением постов.
func TestSupplementDupe_RejectsRowAlreadyInAttachment(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	authorID, token := suppVoteUser(t, e, db, "dupe_author", td.OrgID, td.CompanyID)
	require.NoError(t, db.Create(&models.UserPermissionOverride{
		UserID:        authorID,
		PermissionKey: services.KeyActionSupplementApplication,
		Value:         "allow",
		GrantedAt:     time.Now(),
	}).Error)

	citizenship := models.Citizenship{Name: "Дубль"}
	require.NoError(t, db.Create(&citizenship).Error)

	appID := suppApp(t, db, td.OrgID, authorID, "DUPE-1", models.ConfirmationApproved, models.StatusProcessing)
	carAttID := suppAttachment(t, db, appID, "cars", "2030-01-01")
	peopleAttID := suppAttachment(t, db, appID, "people", "2030-01-01")

	passport := "3333 333333"
	require.NoError(t, db.Create(&models.Employee{
		AttachmentID:         &peopleAttID,
		LastName:             testutil.Ptr("Уже"),
		FirstName:            testutil.Ptr("Иван"),
		PassportSeriesNumber: &passport,
		Status:               testutil.Ptr(1),
	}).Error)
	require.NoError(t, db.Create(&models.Car{
		AttachmentID: carAttID,
		CarNumber:    testutil.Ptr("В333ВВ777"),
		CarBrand:     testutil.Ptr("ГАЗель"),
		Status:       testutil.Ptr(1),
	}).Error)

	post := func(body string) *httptest.ResponseRecorder {
		return testutil.POST(t, e, fmt.Sprintf("/applications/%d/supplements", appID), body, testutil.AuthHeader(token))
	}

	rec := post(fmt.Sprintf(`{"additions":[{"attachment_id":%d,"employees":[
		{"last_name":"Уже","first_name":"Иван","passport_series_number":"3333 333333","citizenship_id":%d,"target_tables":[]}]}]}`, peopleAttID, citizenship.ID))
	assert.Equal(t, http.StatusConflict, rec.Code, "тот же паспорт во вложении - отказ: %s", rec.Body.String())
	assert.Contains(t, rec.Body.String(), "уже есть в этом вложении")

	rec = post(fmt.Sprintf(`{"additions":[{"attachment_id":%d,"vehicles":[
		{"car_number":"в333 вв777","car_brand":"ГАЗель","unload_places":[],"passage_tables":[]}]}]}`, carAttID))
	assert.Equal(t, http.StatusConflict, rec.Code, "тот же номер во вложении, иной регистр и пробелы - отказ: %s", rec.Body.String())

	// Другой человек и другая машина в то же вложение проходят - запрет узкий.
	rec = post(fmt.Sprintf(`{"additions":[{"attachment_id":%d,"employees":[
		{"last_name":"Новый","first_name":"Пётр","passport_series_number":"4444 444444","citizenship_id":%d,"target_tables":[]}]}]}`, peopleAttID, citizenship.ID))
	assert.Equal(t, http.StatusOK, rec.Code, "другой сотрудник добавляется штатно: %s", rec.Body.String())
}
