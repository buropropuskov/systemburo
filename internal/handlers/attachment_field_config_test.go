package handlers_test

import (
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fieldByKey ищет базовое поле в смерженном ответе по ключу реестра.
func fieldByKey(t *testing.T, fields []models.MergedField, key string) models.MergedField {
	t.Helper()
	for _, f := range fields {
		if f.Key == key {
			return f
		}
	}
	t.Fatalf("base field %q not found in response", key)
	return models.MergedField{}
}

// TestGetFieldConfig_DefaultsForFreshAttachment: у вложения без оверрайдов
// GET отдаёт дефолты реестра типа (common + people) и пустой список кастомных.
func TestGetFieldConfig_DefaultsForFreshAttachment(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	token := testutil.RegisterAdmin(t, e, 0, 0)
	uaID := seedUniqueAttachment(t, db, "people", "people_tpl", "Люди")

	rec := testutil.GET(t, e, "/attachments/"+itoa(uaID)+"/field-config", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	resp := testutil.ParseResponse[models.FieldConfigResponse](t, rec)
	require.NotEmpty(t, resp.Base)
	assert.Empty(t, resp.Custom)

	// people-тип не должен содержать поля cars/items.
	for _, f := range resp.Base {
		assert.NotEqual(t, "number", f.Key, "cars field leaked into people config")
		assert.NotEqual(t, "item_name", f.Key, "items field leaked into people config")
	}

	last := fieldByKey(t, resp.Base, "last_name")
	assert.True(t, last.Visible)
	assert.True(t, last.Required)

	middle := fieldByKey(t, resp.Base, "middle_name")
	assert.True(t, middle.Visible)
	assert.False(t, middle.Required, "middle_name по дефолту необязателен")

	// Дата/время - залочены: всегда visible+required и помечены Locked.
	dateFrom := fieldByKey(t, resp.Base, "entry_date_from")
	assert.True(t, dateFrom.Locked)
	assert.True(t, dateFrom.Visible)
	assert.True(t, dateFrom.Required)
}

// TestSaveAndGetFieldConfig_AppliesOverrides: PUT сохраняет оверрайды, GET их
// отражает; залоченные и не-requirable поля ведут себя по правилам реестра.
func TestSaveAndGetFieldConfig_AppliesOverrides(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	token := testutil.RegisterAdmin(t, e, 0, 0)
	uaID := seedUniqueAttachment(t, db, "people", "people_tpl", "Люди")

	// Скрываем patent, делаем work_permission обязательным; пытаемся снять
	// залоченный entry_date_from и пометить required чекбокс roof_access.
	body := `{"base":[
		{"key":"patent","visible":false,"required":false},
		{"key":"work_permission","visible":true,"required":true},
		{"key":"entry_date_from","visible":false,"required":false},
		{"key":"roof_access","visible":false,"required":true}
	]}`
	rec := testutil.PUT(t, e, "/attachments/"+itoa(uaID)+"/field-config", body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	rec = testutil.GET(t, e, "/attachments/"+itoa(uaID)+"/field-config", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	resp := testutil.ParseResponse[models.FieldConfigResponse](t, rec)

	patent := fieldByKey(t, resp.Base, "patent")
	assert.False(t, patent.Visible, "patent должен быть скрыт оверрайдом")

	wp := fieldByKey(t, resp.Base, "work_permission")
	assert.True(t, wp.Required, "work_permission стал обязательным")

	// Залоченное поле игнорирует оверрайд - остаётся visible+required.
	dateFrom := fieldByKey(t, resp.Base, "entry_date_from")
	assert.True(t, dateFrom.Visible)
	assert.True(t, dateFrom.Required)

	// roof_access - булевый чекбокс: видимость меняется, required форсится в false.
	roof := fieldByKey(t, resp.Base, "roof_access")
	assert.False(t, roof.Visible)
	assert.False(t, roof.Required, "required не имеет смысла для булевого чекбокса")

	// Повторный PUT того же ключа - идемпотентен (upsert, не дубль-вставка).
	body2 := `{"base":[{"key":"patent","visible":true,"required":true}]}`
	rec = testutil.PUT(t, e, "/attachments/"+itoa(uaID)+"/field-config", body2, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = testutil.GET(t, e, "/attachments/"+itoa(uaID)+"/field-config", testutil.AuthHeader(token))
	resp = testutil.ParseResponse[models.FieldConfigResponse](t, rec)
	patent = fieldByKey(t, resp.Base, "patent")
	assert.True(t, patent.Visible)
	assert.True(t, patent.Required)
}

// TestSaveFieldConfig_RejectsUnknownKey: ключ не из реестра типа - 400.
func TestSaveFieldConfig_RejectsUnknownKey(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	token := testutil.RegisterAdmin(t, e, 0, 0)
	uaID := seedUniqueAttachment(t, db, "people", "people_tpl", "Люди")

	// number - поле машин, для people-типа неизвестно.
	body := `{"base":[{"key":"number","visible":true,"required":true}]}`
	rec := testutil.PUT(t, e, "/attachments/"+itoa(uaID)+"/field-config", body, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())

	body = `{"base":[{"key":"totally_bogus","visible":true,"required":true}]}`
	rec = testutil.PUT(t, e, "/attachments/"+itoa(uaID)+"/field-config", body, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
}

// TestFieldConfig_NotFound: несуществующий UA - 404 на GET и PUT.
func TestFieldConfig_NotFound(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	token := testutil.RegisterAdmin(t, e, 0, 0)

	rec := testutil.GET(t, e, "/attachments/999999/field-config", testutil.AuthHeader(token))
	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())

	body := `{"base":[{"key":"last_name","visible":true,"required":true}]}`
	rec = testutil.PUT(t, e, "/attachments/999999/field-config", body, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
}

// TestFieldConfig_RequiresAuth: без токена - 401.
func TestFieldConfig_RequiresAuth(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	rec := testutil.GET(t, e, "/attachments/1/field-config", nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code, rec.Body.String())
}

// TestCustomField_PersistsIsRequired: is_required проходит через CRUD кастомных
// полей и попадает в ответ field-config.
func TestCustomField_PersistsIsRequired(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	token := testutil.RegisterAdmin(t, e, 0, 0)
	uaID := seedUniqueAttachment(t, db, "people", "people_tpl", "Люди")

	// Создаём обязательное кастомное поле.
	create := `{"label":"Телефон","placeholder":"+7","sort_order":1,"is_required":true}`
	rec := testutil.POST(t, e, "/attachments/"+itoa(uaID)+"/custom-fields", create, testutil.AuthHeader(token))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	created := testutil.ParseResponse[models.AttachmentCustomField](t, rec)
	assert.True(t, created.IsRequired)

	// field-config отдаёт его в custom с is_required=true.
	rec = testutil.GET(t, e, "/attachments/"+itoa(uaID)+"/field-config", testutil.AuthHeader(token))
	resp := testutil.ParseResponse[models.FieldConfigResponse](t, rec)
	require.Len(t, resp.Custom, 1)
	assert.Equal(t, "Телефон", resp.Custom[0].Label)
	assert.True(t, resp.Custom[0].IsRequired)

	// Снимаем обязательность через UpdateCustomField - отражается в ответе.
	update := `{"label":"Телефон","placeholder":"+7","sort_order":1,"is_required":false}`
	rec = testutil.PUT(t, e, "/attachments/custom-fields/"+itoa(created.ID), update, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	rec = testutil.GET(t, e, "/attachments/"+itoa(uaID)+"/field-config", testutil.AuthHeader(token))
	resp = testutil.ParseResponse[models.FieldConfigResponse](t, rec)
	require.Len(t, resp.Custom, 1)
	assert.False(t, resp.Custom[0].IsRequired)
}
