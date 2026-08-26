package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUniqueEmployeeService_Update_RecordsChanges проверяет, что при апдейте
// мастер-записи сотрудника пишутся записи data_changed на каждое изменённое
// поле (и только на них).
func TestUniqueEmployeeService_Update_RecordsChanges(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	owner := models.User{
		Username:       "owner_emp_change",
		Password:       "x",
		TypeID:         6,
		OrganizationID: &td.OrgID,
		CompanyID:      &td.CompanyID,
	}
	require.NoError(t, db.Create(&owner).Error)

	lastNameOld := "Иванов"
	firstNameOld := "Иван"
	middleNameOld := "Иванович"
	posOld := "Грузчик"
	otherOld := "разрешение-1"
	consentAt := time.Now().UTC()
	emp := models.UniqueEmployee{
		// Согласие субъекта у записи уже есть - так её создаёт живой поток (Create
		// требует отметку). Без этого правка добавила бы отметку и история получила
		// бы лишнее изменение pd_consent_at.
		PDConsentAt:     &consentAt,
		LastName:        &lastNameOld,
		FirstName:       &firstNameOld,
		MiddleName:      &middleNameOld,
		Position:        &posOld,
		OtherPermission: &otherOld,
		OrganizationID:  &td.OrgID,
		CompanyID:       &td.CompanyID,
		UserID:          &owner.ID,
	}
	require.NoError(t, db.Create(&emp).Error)

	svc := services.NewUniqueEmployeeService(db)

	// Меняем фамилию и должность, остальное оставляем.
	newLast := "Петров"
	newPos := "Старший грузчик"
	req := services.NewUniqueEmployeeRequest{
		PDConsent:       true,
		LastName:        &newLast,
		FirstName:       &firstNameOld,
		MiddleName:      &middleNameOld,
		Position:        &newPos,
		OtherPermission: &otherOld,
		OrganizationID:  &td.OrgID,
		CompanyID:       &td.CompanyID,
		UserID:          &owner.ID,
	}
	resp, err := svc.Update(context.Background(), owner.Username, emp.ID, req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "Петров", *resp.LastName)

	// После cutover (#870, срез 1.13c) запись идёт в audit_log[unique_employee].
	// Должно быть ровно две записи — last_name и position.
	var entries []models.AuditLog
	require.NoError(t, db.Where("entity_type = ? AND entity_id = ?", models.AuditEntityUniqueEmployee, emp.ID).
		Order("id").Find(&entries).Error)
	require.Len(t, entries, 2, "ожидаются ровно две записи (last_name, position) в audit_log")

	type fieldDetails struct {
		FieldName *string `json:"field_name"`
		OldValue  *string `json:"old_value"`
		NewValue  *string `json:"new_value"`
	}
	byField := make(map[string]fieldDetails, len(entries))
	for _, e := range entries {
		assert.Equal(t, "data_changed", e.Action)
		require.NotNil(t, e.ActorUserID)
		assert.Equal(t, owner.ID, *e.ActorUserID)
		var det fieldDetails
		require.NoError(t, json.Unmarshal(e.Details, &det))
		require.NotNil(t, det.FieldName)
		byField[*det.FieldName] = det
	}

	if d, ok := byField["last_name"]; assert.True(t, ok, "ожидалась запись last_name") {
		assert.Equal(t, "Иванов", deref(d.OldValue))
		assert.Equal(t, "Петров", deref(d.NewValue))
	}
	if d, ok := byField["position"]; assert.True(t, ok, "ожидалась запись position") {
		assert.Equal(t, "Грузчик", deref(d.OldValue))
		assert.Equal(t, "Старший грузчик", deref(d.NewValue))
	}
}

// TestUniqueEmployeeService_Update_NoChange проверяет, что при апдейте без
// изменений (новые значения == старым) запись истории не создаётся.
func TestUniqueEmployeeService_Update_NoChange(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	owner := models.User{
		Username:       "owner_emp_nochange",
		Password:       "x",
		TypeID:         6,
		OrganizationID: &td.OrgID,
		CompanyID:      &td.CompanyID,
	}
	require.NoError(t, db.Create(&owner).Error)

	last := "Сидоров"
	first := "Пётр"
	middle := "Алексеевич"
	emp := models.UniqueEmployee{
		// Согласие субъекта у записи уже есть - так её создаёт живой поток.
		PDConsentAt:    ptrTime(time.Now().UTC()),
		LastName:       &last,
		FirstName:      &first,
		MiddleName:     &middle,
		OrganizationID: &td.OrgID,
		CompanyID:      &td.CompanyID,
		UserID:         &owner.ID,
	}
	require.NoError(t, db.Create(&emp).Error)

	svc := services.NewUniqueEmployeeService(db)

	req := services.NewUniqueEmployeeRequest{
		PDConsent:      true,
		LastName:       &last,
		FirstName:      &first,
		MiddleName:     &middle,
		OrganizationID: &td.OrgID,
		CompanyID:      &td.CompanyID,
		UserID:         &owner.ID,
	}
	_, err := svc.Update(context.Background(), owner.Username, emp.ID, req)
	require.NoError(t, err)

	// При no-op апдейте audit_log[unique_employee] не пополняется (#870, срез 1.13c).
	var count int64
	require.NoError(t, db.Model(&models.AuditLog{}).
		Where("entity_type = ? AND entity_id = ?", models.AuditEntityUniqueEmployee, emp.ID).
		Count(&count).Error)
	assert.Equal(t, int64(0), count, "не должно создаваться записей истории при no-op апдейте")
}

// TestUniqueCarService_UpdateByNumber_RecordsChanges проверяет аудит для
// машины при изменении format_id и user_id через UpdateByNumber.
func TestUniqueCarService_UpdateByNumber_RecordsChanges(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	owner := models.User{
		Username:       "owner_car_change",
		Password:       "x",
		TypeID:         6,
		OrganizationID: &td.OrgID,
		CompanyID:      &td.CompanyID,
	}
	require.NoError(t, db.Create(&owner).Error)

	number := "А123БВ77"
	mark := "Лада"
	formatOld := 1
	car := models.UniqueCar{
		Number:         &number,
		Mark:           &mark,
		OrganizationID: &td.OrgID,
		CompanyID:      &td.CompanyID,
		FormatID:       &formatOld,
		UserID:         &owner.ID,
	}
	require.NoError(t, db.Create(&car).Error)

	svc := services.NewUniqueCarService(db)

	formatNew := 2
	req := services.UpdateCarByNumberRequest{
		Number: number,
		Mark:   mark,
		UpdateData: services.NewUniqueCarRequest{
			Number:         number,
			Mark:           mark,
			OrganizationID: &td.OrgID,
			CompanyID:      &td.CompanyID,
			FormatID:       &formatNew,
			UserID:         &owner.ID,
		},
	}
	resp, err := svc.UpdateByNumber(context.Background(), owner.Username, req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// После cutover (#870, срез 1.12d) запись идёт в audit_log[unique_car].
	var entries []models.AuditLog
	require.NoError(t, db.Where("entity_type = ? AND entity_id = ?", models.AuditEntityUniqueCar, car.ID).
		Order("id").Find(&entries).Error)
	require.Len(t, entries, 1, "ожидается ровно одна запись (format_id) в audit_log")
	assert.Equal(t, "data_changed", entries[0].Action)

	var det struct {
		FieldName *string `json:"field_name"`
		OldValue  *string `json:"old_value"`
		NewValue  *string `json:"new_value"`
	}
	require.NoError(t, json.Unmarshal(entries[0].Details, &det))
	assert.Equal(t, "format_id", deref(det.FieldName))
	assert.Equal(t, "1", deref(det.OldValue))
	assert.Equal(t, "2", deref(det.NewValue))
}

// TestUniqueEmployeeService_GetHistory_ReturnsRecords проверяет, что GetHistory
// возвращает записи аудита по data_changed после Update и фильтрует по правам.
func TestUniqueEmployeeService_GetHistory_ReturnsRecords(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	owner := models.User{
		Username:       "owner_emp_history",
		Password:       "x",
		TypeID:         6,
		OrganizationID: &td.OrgID,
		CompanyID:      &td.CompanyID,
	}
	require.NoError(t, db.Create(&owner).Error)

	last := "Иванов"
	first := "Иван"
	pos := "Грузчик"
	emp := models.UniqueEmployee{
		// Согласие субъекта у записи уже есть - так её создаёт живой поток.
		PDConsentAt:    ptrTime(time.Now().UTC()),
		LastName:       &last,
		FirstName:      &first,
		Position:       &pos,
		OrganizationID: &td.OrgID,
		CompanyID:      &td.CompanyID,
		UserID:         &owner.ID,
	}
	require.NoError(t, db.Create(&emp).Error)

	svc := services.NewUniqueEmployeeService(db)

	newLast := "Петров"
	req := services.NewUniqueEmployeeRequest{
		PDConsent:      true,
		LastName:       &newLast,
		FirstName:      &first,
		Position:       &pos,
		OrganizationID: &td.OrgID,
		CompanyID:      &td.CompanyID,
		UserID:         &owner.ID,
	}
	_, err := svc.Update(context.Background(), owner.Username, emp.ID, req)
	require.NoError(t, err)

	items, err := svc.GetHistory(context.Background(), owner.Username, emp.ID)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "data_changed", items[0].ActionType)
	require.NotNil(t, items[0].FieldName)
	assert.Equal(t, "last_name", *items[0].FieldName)
	assert.Equal(t, "Иванов", deref(items[0].OldValue))
	assert.Equal(t, "Петров", deref(items[0].NewValue))
	require.NotNil(t, items[0].Username)
	assert.Equal(t, owner.Username, *items[0].Username)
}

// TestUniqueEmployeeService_GetHistory_Forbidden проверяет, что юзер без прав
// на редактирование сотрудника получает 403 при запросе истории.
func TestUniqueEmployeeService_GetHistory_Forbidden(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	otherOrgID := td.OrgID + 1000
	otherOrg := models.Organization{ID: otherOrgID, Name: "other-org"}
	require.NoError(t, db.Create(&otherOrg).Error)

	otherCompID := td.CompanyID + 1000
	otherComp := models.Company{ID: otherCompID, Name: "other-comp"}
	require.NoError(t, db.Create(&otherComp).Error)

	owner := models.User{
		Username:       "owner_emp_forbid",
		Password:       "x",
		TypeID:         6,
		OrganizationID: &td.OrgID,
		CompanyID:      &td.CompanyID,
	}
	require.NoError(t, db.Create(&owner).Error)

	stranger := models.User{
		Username:       "stranger_emp_forbid",
		Password:       "x",
		TypeID:         6,
		OrganizationID: &otherOrgID,
		CompanyID:      &otherCompID,
	}
	require.NoError(t, db.Create(&stranger).Error)

	last := "Сидоров"
	emp := models.UniqueEmployee{
		// Согласие субъекта у записи уже есть - так её создаёт живой поток.
		PDConsentAt:    ptrTime(time.Now().UTC()),
		LastName:       &last,
		OrganizationID: &td.OrgID,
		CompanyID:      &td.CompanyID,
		UserID:         &owner.ID,
	}
	require.NoError(t, db.Create(&emp).Error)

	svc := services.NewUniqueEmployeeService(db)
	_, err := svc.GetHistory(context.Background(), stranger.Username, emp.ID)
	require.Error(t, err)
	httpErr, ok := err.(*echo.HTTPError)
	require.True(t, ok)
	assert.Equal(t, http.StatusForbidden, httpErr.Code)
}

// TestUniqueCarService_GetHistory_ReturnsRecords проверяет аналогичный сценарий
// для машин: создание записи через UpdateByNumber и чтение через GetHistory.
func TestUniqueCarService_GetHistory_ReturnsRecords(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	owner := models.User{
		Username:       "owner_car_history",
		Password:       "x",
		TypeID:         6,
		OrganizationID: &td.OrgID,
		CompanyID:      &td.CompanyID,
	}
	require.NoError(t, db.Create(&owner).Error)

	number := "А999ВВ77"
	mark := "Лада"
	formatOld := 1
	car := models.UniqueCar{
		Number:         &number,
		Mark:           &mark,
		OrganizationID: &td.OrgID,
		CompanyID:      &td.CompanyID,
		FormatID:       &formatOld,
		UserID:         &owner.ID,
	}
	require.NoError(t, db.Create(&car).Error)

	svc := services.NewUniqueCarService(db)
	formatNew := 2
	_, err := svc.UpdateByNumber(context.Background(), owner.Username, services.UpdateCarByNumberRequest{
		Number: number,
		Mark:   mark,
		UpdateData: services.NewUniqueCarRequest{
			Number:         number,
			Mark:           mark,
			OrganizationID: &td.OrgID,
			CompanyID:      &td.CompanyID,
			FormatID:       &formatNew,
			UserID:         &owner.ID,
		},
	})
	require.NoError(t, err)

	items, err := svc.GetHistory(context.Background(), owner.Username, car.ID)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "data_changed", items[0].ActionType)
	require.NotNil(t, items[0].FieldName)
	assert.Equal(t, "format_id", *items[0].FieldName)
	require.NotNil(t, items[0].Username)
	assert.Equal(t, owner.Username, *items[0].Username)
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
