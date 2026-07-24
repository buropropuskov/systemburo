package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Мультивыбор справочников в фильтрах Центра (#1398): организации/компании списком,
// места разгрузки и таблицы проходной. SQL этих фильтров живёт в подзапросах
// applyApplicationFilters и валидируется только исполнением - go build видит их как
// обычную строку, поэтому проверяем через реальный endpoint.

// attachWithPlace создаёт вложение заявки и вешает на него место разгрузки.
func attachWithPlace(t *testing.T, db *gorm.DB, appID, placeID int, atype string) int {
	t.Helper()
	att := models.Attachment{ApplicationID: &appID, AttachmentType: atype}
	require.NoError(t, db.Create(&att).Error)
	require.NoError(t, db.Create(&models.AttachmentUnloadPlace{AttachmentID: att.ID, UnloadPlaceID: placeID}).Error)
	return att.ID
}

// newUnloadPlace/newPassageTable - минимальные справочные записи для привязок.
func newUnloadPlace(t *testing.T, db *gorm.DB, name string) int {
	t.Helper()
	place := models.UnloadPlace{Name: name, IsActive: true, Status: "active"}
	require.NoError(t, db.Create(&place).Error)
	return place.ID
}

func newPassageTable(t *testing.T, db *gorm.DB, name, tableType string) int {
	t.Helper()
	st := models.SystemTable{Name: name, TableType: tableType, IsActive: true}
	require.NoError(t, db.Create(&st).Error)
	return st.ID
}

func TestApplicationsMultiSelectFilters(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	adminToken := testutil.RegisterAndLogin(t, e, "mf_admin", "pass123", 6, td.OrgID, td.CompanyID)
	adminID := getUserID(t, db, "mf_admin")

	// Вторая организация и компания - чтобы мультивыбор было чем отличать от одиночного.
	org2 := models.Organization{Name: "MF Орг-2"}
	require.NoError(t, db.Create(&org2).Error)
	comp2 := models.Company{Name: "MF Компания-2"}
	require.NoError(t, db.Create(&comp2).Error)

	appOrg1 := seedAttachableApp(t, db, td.OrgID, adminID, "MF-ORG1", "Согласование", "В работе")
	appOrg2 := seedAttachableApp(t, db, org2.ID, adminID, "MF-ORG2", "Согласование", "В работе")
	appOrg3 := seedAttachableApp(t, db, td.OrgID, adminID, "MF-ORG3", "Согласование", "В работе")
	require.NoError(t, db.Model(&models.Application{}).Where("id = ?", appOrg1).Update("company_id", td.CompanyID).Error)
	require.NoError(t, db.Model(&models.Application{}).Where("id = ?", appOrg2).Update("company_id", comp2.ID).Error)

	// Места разгрузки: одна заявка с cars-вложением, другая с items - у обеих место
	// лежит в attachment_unload_places (для items это единственная привязка, #706).
	placeA := newUnloadPlace(t, db, "MF Место A")
	placeB := newUnloadPlace(t, db, "MF Место B")
	appPlaceCars := seedAttachableApp(t, db, td.OrgID, adminID, "MF-PLACE-CARS", "Согласование", "В работе")
	appPlaceItems := seedAttachableApp(t, db, td.OrgID, adminID, "MF-PLACE-ITEMS", "Согласование", "В работе")
	appPlaceOther := seedAttachableApp(t, db, td.OrgID, adminID, "MF-PLACE-OTHER", "Согласование", "В работе")
	attachWithPlace(t, db, appPlaceCars, placeA, "cars")
	attachWithPlace(t, db, appPlaceItems, placeA, "items")
	attachWithPlace(t, db, appPlaceOther, placeB, "cars")

	// Таблицы проходной: у одной заявки привязана машина ("Проезд"), у другой -
	// сотрудник ("Места прохода"). Фильтр обязан находить обе.
	tableCars := newPassageTable(t, db, "MF КПП авто", "cars")
	tablePeople := newPassageTable(t, db, "MF КПП люди", "people")
	tableUnused := newPassageTable(t, db, "MF КПП пустой", "cars")

	appCar := seedAttachableApp(t, db, td.OrgID, adminID, "MF-PASS-CAR", "Согласование", "В работе")
	attCar := models.Attachment{ApplicationID: &appCar, AttachmentType: "cars"}
	require.NoError(t, db.Create(&attCar).Error)
	car := models.Car{AttachmentID: attCar.ID}
	require.NoError(t, db.Create(&car).Error)
	require.NoError(t, db.Create(&models.CarTargetTable{CarID: car.ID, TableID: tableCars}).Error)

	appEmp := seedAttachableApp(t, db, td.OrgID, adminID, "MF-PASS-EMP", "Согласование", "В работе")
	attEmp := models.Attachment{ApplicationID: &appEmp, AttachmentType: "people"}
	require.NoError(t, db.Create(&attEmp).Error)
	emp := models.Employee{AttachmentID: &attEmp.ID}
	require.NoError(t, db.Create(&emp).Error)
	require.NoError(t, db.Create(&models.EmployeeTargetTable{EmployeeID: emp.ID, TableID: tablePeople}).Error)

	idsFor := func(t *testing.T, query string) map[int]bool {
		t.Helper()
		rec := testutil.GET(t, e, "/applications?per_page=100"+query, testutil.AuthHeader(adminToken))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		ids := map[int]bool{}
		for _, row := range testutil.ParseSlice(t, rec) {
			if id, ok := row["id"].(float64); ok {
				ids[int(id)] = true
			}
		}
		return ids
	}

	t.Run("organization_ids - обе организации в выборке", func(t *testing.T) {
		ids := idsFor(t, fmt.Sprintf("&organization_ids=%d,%d", td.OrgID, org2.ID))
		assert.True(t, ids[appOrg1], "заявка первой организации должна быть")
		assert.True(t, ids[appOrg2], "заявка второй организации должна быть")
	})

	t.Run("organization_ids с одним значением сужает до него", func(t *testing.T) {
		ids := idsFor(t, fmt.Sprintf("&organization_ids=%d", org2.ID))
		assert.True(t, ids[appOrg2])
		assert.False(t, ids[appOrg1], "чужая организация не должна попасть")
		assert.False(t, ids[appOrg3], "чужая организация не должна попасть")
	})

	t.Run("company_ids - мультивыбор компаний", func(t *testing.T) {
		ids := idsFor(t, fmt.Sprintf("&company_ids=%d,%d", td.CompanyID, comp2.ID))
		assert.True(t, ids[appOrg1])
		assert.True(t, ids[appOrg2])
		assert.False(t, ids[appOrg3], "заявка без компании не проходит фильтр по компаниям")
	})

	t.Run("unload_place_ids ловит и cars-, и items-вложение", func(t *testing.T) {
		ids := idsFor(t, fmt.Sprintf("&unload_place_ids=%d", placeA))
		assert.True(t, ids[appPlaceCars], "заявка с cars-вложением на этом месте")
		assert.True(t, ids[appPlaceItems], "заявка с items-вложением на этом месте")
		assert.False(t, ids[appPlaceOther], "заявка с другим местом не должна проходить")
	})

	t.Run("unload_place_ids мультивыбор - объединение мест", func(t *testing.T) {
		ids := idsFor(t, fmt.Sprintf("&unload_place_ids=%d,%d", placeA, placeB))
		assert.True(t, ids[appPlaceCars] && ids[appPlaceItems] && ids[appPlaceOther])
		assert.False(t, ids[appOrg3], "заявка без мест разгрузки не проходит фильтр по местам")
	})

	t.Run("passage_table_ids ловит привязку машины", func(t *testing.T) {
		ids := idsFor(t, fmt.Sprintf("&passage_table_ids=%d", tableCars))
		assert.True(t, ids[appCar], "заявка с машиной в этой таблице")
		assert.False(t, ids[appEmp], "заявка с сотрудником в другой таблице")
	})

	t.Run("passage_table_ids ловит привязку сотрудника", func(t *testing.T) {
		ids := idsFor(t, fmt.Sprintf("&passage_table_ids=%d", tablePeople))
		assert.True(t, ids[appEmp], "заявка с сотрудником в этой таблице")
		assert.False(t, ids[appCar], "заявка с машиной в другой таблице")
	})

	t.Run("passage_table_ids мультивыбор - машины и люди вместе", func(t *testing.T) {
		ids := idsFor(t, fmt.Sprintf("&passage_table_ids=%d,%d", tableCars, tablePeople))
		assert.True(t, ids[appCar] && ids[appEmp])
	})

	t.Run("таблица без привязок даёт пустую выборку", func(t *testing.T) {
		ids := idsFor(t, fmt.Sprintf("&passage_table_ids=%d", tableUnused))
		assert.Empty(t, ids, "к этой таблице никто не привязан: %v", ids)
	})

	t.Run("мусор в параметре не роняет запрос и не фильтрует", func(t *testing.T) {
		// Опечатка в query не должна отдавать 500 и не должна выглядеть как "заявок нет":
		// невалидные элементы отбрасываются, пустой список = фильтр не применён.
		ids := idsFor(t, "&organization_ids=abc&unload_place_ids=&passage_table_ids=xx,yy")
		assert.True(t, ids[appOrg1] && ids[appOrg2], "мусорный фильтр не должен сужать выборку: %v", ids)
	})

	t.Run("комбинация организаций и мест разгрузки сужает по И", func(t *testing.T) {
		ids := idsFor(t, fmt.Sprintf("&organization_ids=%d&unload_place_ids=%d", td.OrgID, placeA))
		assert.True(t, ids[appPlaceCars] && ids[appPlaceItems])
		assert.False(t, ids[appOrg1], "без места разгрузки заявка не проходит комбинацию")
	})
}
