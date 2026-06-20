package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Тесты write-path для attachment_unload_places (#706, срез BE-S2).
// DB-backed через HTTP (submit-complete-application), живут в пакете handlers_test
// (единственный DB-тест-бинарь проекта, изолируется CleanDB).

// attachmentUnloadPlaceRows возвращает строки attachment_unload_places для вложения.
func attachmentUnloadPlaceRows(t *testing.T, db *gorm.DB, attachmentID int) []models.AttachmentUnloadPlace {
	t.Helper()
	var rows []models.AttachmentUnloadPlace
	require.NoError(t, db.Where("attachment_id = ?", attachmentID).Find(&rows).Error)
	return rows
}

// getAttachmentIDsByAppID возвращает все ID вложений по ID заявки.
func getAttachmentIDsByAppID(t *testing.T, db *gorm.DB, appID int) []int {
	t.Helper()
	var ids []int
	require.NoError(t, db.Table("attachments").Where("application_id = ?", appID).Select("id").Scan(&ids).Error)
	return ids
}

// TestWritePath_ItemsUnloadPlacesWritten проверяет, что items-вложение с UnloadPlaces
// пишет строки в attachment_unload_places.
func TestWritePath_ItemsUnloadPlacesWritten(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	uaID := seedUniqueAttachment(t, db, "items", "wp_items", "WP Items")
	token := testutil.RegisterAndLogin(t, e, "wpitems1", "pass_long_123", 1, td.OrgID, td.CompanyID)

	// Создаём два места разгрузки
	place1 := models.UnloadPlace{Name: "Склад WP-1", IsActive: true}
	place2 := models.UnloadPlace{Name: "Склад WP-2", IsActive: true}
	require.NoError(t, db.Create(&place1).Error)
	require.NoError(t, db.Create(&place2).Error)

	body := fmt.Sprintf(`{
		"message": "writepath items test",
		"organization": "Test Organization",
		"responsible_person": "Test Person",
		"contact_phone": "+79001234567",
		"data_approval": true,
		"attachments": [{
			"attachment_type": "items",
			"attachment_name": "wp_items",
			"attachment_display_name": "WP Items",
			"unique_attachment_id": %d,
			"unload_places": [%d, %d],
			"data": {
				"items": [{"name": "Ноутбук", "count": 1, "order_index": 0}]
			}
		}]
	}`, uaID, place1.ID, place2.ID)

	rec := testutil.POST(t, e, "/applications/submit-complete-application", body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "submit items: %s", rec.Body.String())

	resp := testutil.ParseResponse[services.CompleteApplicationResponse](t, rec)
	require.NotZero(t, resp.ApplicationID)

	attIDs := getAttachmentIDsByAppID(t, db, resp.ApplicationID)
	require.Len(t, attIDs, 1, "одно вложение")

	rows := attachmentUnloadPlaceRows(t, db, attIDs[0])
	require.Len(t, rows, 2, "items-вложение записало оба места в attachment_unload_places")

	placeIDs := make(map[int]bool, 2)
	for _, r := range rows {
		placeIDs[r.UnloadPlaceID] = true
	}
	require.True(t, placeIDs[place1.ID], "место 1 записано")
	require.True(t, placeIDs[place2.ID], "место 2 записано")
}

// TestWritePath_CarsUnloadPlacesDeduped проверяет, что cars-вложение пишет дедуп-union
// мест всех машин в attachment_unload_places без дублей (ON CONFLICT DO NOTHING).
func TestWritePath_CarsUnloadPlacesDeduped(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	uaID := seedUniqueAttachment(t, db, "cars", "wp_cars", "WP Cars")
	token := testutil.RegisterAndLogin(t, e, "wpcars1", "pass_long_123", 1, td.OrgID, td.CompanyID)

	// Три места: машина 1 имеет [place1, place2], машина 2 имеет [place2, place3].
	// В attachment_unload_places должно быть 3 уникальных строки (place2 не дублируется).
	place1 := models.UnloadPlace{Name: "Склад C-1", IsActive: true}
	place2 := models.UnloadPlace{Name: "Склад C-2", IsActive: true}
	place3 := models.UnloadPlace{Name: "Склад C-3", IsActive: true}
	require.NoError(t, db.Create(&place1).Error)
	require.NoError(t, db.Create(&place2).Error)
	require.NoError(t, db.Create(&place3).Error)

	body := fmt.Sprintf(`{
		"message": "writepath cars dedup test",
		"organization": "Test Organization",
		"responsible_person": "Test Person",
		"contact_phone": "+79001234567",
		"data_approval": true,
		"attachments": [{
			"attachment_type": "cars",
			"attachment_name": "wp_cars",
			"attachment_display_name": "WP Cars",
			"unique_attachment_id": %d,
			"data": {
				"vehicles": [
					{"car_number": "А001ВС777", "car_brand": "Toyota", "unload_places": [%d, %d]},
					{"car_number": "А002ВС777", "car_brand": "BMW",    "unload_places": [%d, %d]}
				]
			}
		}]
	}`, uaID, place1.ID, place2.ID, place2.ID, place3.ID)

	rec := testutil.POST(t, e, "/applications/submit-complete-application", body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "submit cars: %s", rec.Body.String())

	resp := testutil.ParseResponse[services.CompleteApplicationResponse](t, rec)
	require.NotZero(t, resp.ApplicationID)

	attIDs := getAttachmentIDsByAppID(t, db, resp.ApplicationID)
	require.Len(t, attIDs, 1, "одно вложение")

	rows := attachmentUnloadPlaceRows(t, db, attIDs[0])
	require.Len(t, rows, 3, "три уникальных места (place2 не дублируется)")

	placeIDs := make(map[int]bool, 3)
	for _, r := range rows {
		placeIDs[r.UnloadPlaceID] = true
	}
	require.True(t, placeIDs[place1.ID], "место 1 записано")
	require.True(t, placeIDs[place2.ID], "место 2 записано (без дубля)")
	require.True(t, placeIDs[place3.ID], "место 3 записано")

	// car_unload_places продолжает писаться: у машины 1 и 2 суммарно 4 строки (с дублём).
	var carPlaceCount int64
	require.NoError(t, db.Table("car_unload_places cup").
		Joins("JOIN cars c ON c.id = cup.car_id").
		Where("c.attachment_id = ?", attIDs[0]).
		Count(&carPlaceCount).Error)
	require.EqualValues(t, 4, carPlaceCount, "car_unload_places по-прежнему пишется для обеих машин")
}

// TestWritePath_CarsNoPlacesNoAttachmentRows проверяет, что cars-вложение без мест
// не создаёт строк в attachment_unload_places.
func TestWritePath_CarsNoPlacesNoAttachmentRows(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	uaID := seedUniqueAttachment(t, db, "cars", "wp_cars_nopl", "WP Cars No Places")
	token := testutil.RegisterAndLogin(t, e, "wpcars2", "pass_long_123", 1, td.OrgID, td.CompanyID)

	body := fmt.Sprintf(`{
		"message": "no places",
		"organization": "Test Organization",
		"responsible_person": "Test Person",
		"contact_phone": "+79001234567",
		"data_approval": true,
		"attachments": [{
			"attachment_type": "cars",
			"attachment_name": "wp_cars_nopl",
			"attachment_display_name": "WP Cars No Places",
			"unique_attachment_id": %d,
			"data": {
				"vehicles": [{"car_number": "А003ВС777", "car_brand": "Kia", "unload_places": []}]
			}
		}]
	}`, uaID)

	rec := testutil.POST(t, e, "/applications/submit-complete-application", body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "submit cars no places: %s", rec.Body.String())

	resp := testutil.ParseResponse[services.CompleteApplicationResponse](t, rec)
	attIDs := getAttachmentIDsByAppID(t, db, resp.ApplicationID)
	require.Len(t, attIDs, 1)

	rows := attachmentUnloadPlaceRows(t, db, attIDs[0])
	require.Empty(t, rows, "нет мест - нет строк в attachment_unload_places")
}
