package handlers_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Строку во вложении заводят два независимых пути: подача заявки и дополнение (#1685).
// Кода они не делят - у подачи даты приходят телом запроса, у дополнения копируются из
// самого вложения, - и свести их в общий хелпер значило бы переписать горячий путь подачи
// ради гипотетической будущей ошибки. Опасность при этом настоящая: новое поле добавят в
// один путь и забудут во втором, а заметят это на КПП, где у половины строк поле пустое.
//
// Поэтому тут не рефакторинг, а инвариант: набор реально заполняемых колонок у обоих путей
// обязан совпадать. Добавили поле только в один - тест назовёт его поимённо.

// suppParityFilled возвращает имена колонок таблицы, у которых значение не NULL.
func suppParityFilled(t *testing.T, db *gorm.DB, table string, id int) map[string]bool {
	t.Helper()
	rows := []map[string]interface{}{}
	require.NoError(t, db.Raw(fmt.Sprintf("SELECT * FROM %s WHERE id = ?", table), id).Scan(&rows).Error)
	require.Len(t, rows, 1, "строка %s id=%d должна существовать", table, id)

	filled := make(map[string]bool)
	for col, val := range rows[0] {
		if val != nil {
			filled[col] = true
		}
	}
	return filled
}

// suppParityDiff называет колонки, заполненные одним путём и пропущенные другим.
func suppParityDiff(submitted, supplemented map[string]bool, ignore map[string]bool) (onlySubmit, onlySupplement []string) {
	for col := range submitted {
		if !supplemented[col] && !ignore[col] {
			onlySubmit = append(onlySubmit, col)
		}
	}
	for col := range supplemented {
		if !submitted[col] && !ignore[col] {
			onlySupplement = append(onlySupplement, col)
		}
	}
	return
}

func TestSupplementParity_SameColumnsFilledAsSubmit(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	authorID, authorToken := suppVoteUser(t, e, db, "parity_author", td.OrgID, td.CompanyID)
	require.NoError(t, db.Create(&models.UserPermissionOverride{
		UserID:        authorID,
		PermissionKey: services.KeyActionSupplementApplication,
		Value:         "allow",
		GrantedAt:     time.Now(),
	}).Error)

	citizenship := models.Citizenship{Name: "Паритет"}
	require.NoError(t, db.Create(&citizenship).Error)

	appID := suppApp(t, db, td.OrgID, authorID, "PARITY-1", models.ConfirmationApproved, models.StatusProcessing)
	carAttID := suppAttachment(t, db, appID, "cars", "2030-01-01")
	peopleAttID := suppAttachment(t, db, appID, "people", "2030-01-01")

	// Строки «как при подаче»: те же поля, что заполняет SubmitCompleteApplication -
	// включая срок и время, которые она берёт из тела запроса (у дополнения они
	// копируются из самого вложения, значения обязаны совпасть).
	submitCar := models.Car{
		AttachmentID:  carAttID,
		CarNumber:     testutil.Ptr("А111АА777"),
		CarBrand:      testutil.Ptr("ГАЗель"),
		UnloadPlace:   testutil.Ptr("Ворота 1"),
		EntryDateFrom: testutil.Ptr("2026-01-01"),
		EntryDateTo:   testutil.Ptr("2030-01-01"),
		EntryTimeFrom: testutil.Ptr("08:00:00"),
		EntryTimeTo:   testutil.Ptr("20:00:00"),
		Status:        testutil.Ptr(0),
	}
	require.NoError(t, db.Create(&submitCar).Error)
	submitEmp := models.Employee{
		AttachmentID:         &peopleAttID,
		LastName:             testutil.Ptr("Поданный"),
		FirstName:            testutil.Ptr("Иван"),
		Position:             testutil.Ptr("Монтажник"),
		CitizenshipID:        &citizenship.ID,
		PassportSeriesNumber: testutil.Ptr("1111 111111"),
		Status:               testutil.Ptr(0),
	}
	require.NoError(t, db.Create(&submitEmp).Error)

	// Те же данные, но через дополнение - реальным запросом, а не прямой вставкой.
	body := fmt.Sprintf(`{"comment":"паритет","additions":[
		{"attachment_id":%d,"vehicles":[{"car_number":"Б222ББ777","car_brand":"ГАЗель","unload_place":"Ворота 1","unload_places":[],"passage_tables":[]}]},
		{"attachment_id":%d,"employees":[{"last_name":"Дополненный","first_name":"Иван","position":"Монтажник","passport_series_number":"2222 222222","citizenship_id":%d,"target_tables":[]}]}
	]}`, carAttID, peopleAttID, citizenship.ID)
	rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/supplements", appID), body, testutil.AuthHeader(authorToken))
	require.Equal(t, http.StatusOK, rec.Code, "подача дополнения: %s", rec.Body.String())

	var suppCarID, suppEmpID int
	require.NoError(t, db.Raw("SELECT id FROM cars WHERE supplement_id IS NOT NULL ORDER BY id DESC LIMIT 1").Scan(&suppCarID).Error)
	require.NoError(t, db.Raw("SELECT id FROM employees WHERE supplement_id IS NOT NULL ORDER BY id DESC LIMIT 1").Scan(&suppEmpID).Error)
	require.NotZero(t, suppCarID)
	require.NotZero(t, suppEmpID)

	// Колонки, расхождение по которым осмысленно и ожидаемо.
	ignore := map[string]bool{
		"id":            true, // разные строки
		"supplement_id": true, // и есть признак происхождения: у подачи его нет по определению
		"created_at":    true,
		"updated_at":    true,
		"date_added":    true, // проставляется дефолтом БД, а не путём создания
		"date_created":  true,
	}

	onlySubmitCar, onlySuppCar := suppParityDiff(
		suppParityFilled(t, db, "cars", submitCar.ID),
		suppParityFilled(t, db, "cars", suppCarID), ignore)
	assert.Empty(t, onlySubmitCar, "машина: подача заполняет колонки, а дополнение их пропускает - %v", onlySubmitCar)
	assert.Empty(t, onlySuppCar, "машина: дополнение заполняет колонки, а подача их пропускает - %v", onlySuppCar)

	onlySubmitEmp, onlySuppEmp := suppParityDiff(
		suppParityFilled(t, db, "employees", submitEmp.ID),
		suppParityFilled(t, db, "employees", suppEmpID), ignore)
	assert.Empty(t, onlySubmitEmp, "сотрудник: подача заполняет колонки, а дополнение их пропускает - %v", onlySubmitEmp)
	assert.Empty(t, onlySuppEmp, "сотрудник: дополнение заполняет колонки, а подача их пропускает - %v", onlySuppEmp)
}
