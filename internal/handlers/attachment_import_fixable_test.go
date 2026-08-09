package handlers_test

import (
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/require"
)

// errorByCode ищет причину отказа строки по машинному коду - тесты правил сверяют
// код и признак исправимости, а не совпадение русской фразы: формулировку правят
// (и правили), а правило от этого не меняется.
func errorByCode(t *testing.T, errs []services.ImportRowError, code services.ImportRowErrorCode) services.ImportRowError {
	t.Helper()
	for _, e := range errs {
		if e.Code == code {
			return e
		}
	}
	require.Failf(t, "причина не найдена", "код %q, есть: %+v", code, errs)
	return services.ImportRowError{}
}

// Признак «правится прямо в таблице разбора» считает сервер и отдаёт его полем fixable
// у каждой причины. До этого его угадывал фронт, разбирая текст причины по префиксу
// «Поле «<подпись>»», и любая формулировка вне шаблона (несовпадение формата номера)
// блокировала строку навсегда.
func TestAttachmentImportListRows_ErrorFixability(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	t.Cleanup(cleanup)
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	peopleUA := seedPeopleFieldsTemplate(t, db, "import_fixable_people", 6)
	require.NoError(t, db.Create(&models.Citizenship{Name: "Россия", IsActive: true}).Error)

	t.Run("пустые ФИО исправимы, отсутствие должности и паспорта - нет", func(t *testing.T) {
		data := buildPeopleRowsUpload(t, 6, []importPersonRow{{citizenship: "Россия"}})
		rec := postImportFile(t, e, peopleUA, "list.xlsx", data, admin)
		require.Equal(t, http.StatusMultiStatus, rec.Code, rec.Body.String())

		result := testutil.ParseResponse[services.ImportListResult](t, rec)
		byField := map[string]services.ImportRowError{}
		for _, err := range result.Rows[0].Errors {
			require.Equal(t, services.ImportErrFieldRequired, err.Code)
			byField[err.Field] = err
		}
		require.True(t, byField["last_name"].Fixable, "фамилия правится прямо в таблице разбора")
		require.True(t, byField["first_name"].Fixable)
		require.False(t, byField["position"].Fixable, "поля должности в таблице разбора нет")
		require.False(t, byField["passport"].Fixable, "паспорт в интерфейс импорта не выводится (152-ФЗ)")
		require.Contains(t, byField["last_name"].Text, "Фамилия", "текст остаётся готовой русской фразой")
	})

	t.Run("неопознанное гражданство исправимо выбором из справочника", func(t *testing.T) {
		r := importPersonRow{last: "Иванов", first: "Иван", citizenship: "Нетакойстраны", passport: "1234 567890", position: "Монтажник"}
		data := buildPeopleRowsUpload(t, 6, []importPersonRow{r})
		rec := postImportFile(t, e, peopleUA, "list.xlsx", data, admin)
		require.Equal(t, http.StatusMultiStatus, rec.Code, rec.Body.String())

		result := testutil.ParseResponse[services.ImportListResult](t, rec)
		err := errorByCode(t, result.Rows[0].Errors, services.ImportErrCitizenshipUnknown)
		require.True(t, err.Fixable)
		require.Equal(t, "citizenship", err.Field)
	})

	t.Run("дубль внутри файла и чёрный список неисправимы", func(t *testing.T) {
		require.NoError(t, db.Create(&models.PersonBlacklist{
			LastName: "Сидоров", FirstName: "Сидор", Reason: "решение суда", IsActive: true,
		}).Error)

		rows := []importPersonRow{
			{last: "Петров", first: "Пётр", citizenship: "Россия", passport: "1111 222233", position: "Монтажник"},
			{last: "Петров", first: "Пётр", citizenship: "Россия", passport: "1111 222233", position: "Монтажник"},
			{last: "Сидоров", first: "Сидор", citizenship: "Россия", passport: "3333 444455", position: "Монтажник"},
		}
		data := buildPeopleRowsUpload(t, 6, rows)
		rec := postImportFile(t, e, peopleUA, "list.xlsx", data, admin)
		require.Equal(t, http.StatusMultiStatus, rec.Code, rec.Body.String())

		result := testutil.ParseResponse[services.ImportListResult](t, rec)
		require.Len(t, result.Rows, 3)
		require.False(t, errorByCode(t, result.Rows[1].Errors, services.ImportErrDuplicateInFile).Fixable)
		require.False(t, errorByCode(t, result.Rows[2].Errors, services.ImportErrBlacklisted).Fixable)
	})

	t.Run("патент по гражданству неисправим - поля патента в разборе нет", func(t *testing.T) {
		require.NoError(t, db.Create(&models.Citizenship{Name: "Узбекистан", IsActive: true, PatentRequired: true}).Error)

		r := importPersonRow{last: "Рахимов", first: "Азиз", citizenship: "Узбекистан", passport: "5555 666677", position: "Монтажник"}
		data := buildPeopleRowsUpload(t, 6, []importPersonRow{r})
		rec := postImportFile(t, e, peopleUA, "list.xlsx", data, admin)
		require.Equal(t, http.StatusMultiStatus, rec.Code, rec.Body.String())

		result := testutil.ParseResponse[services.ImportListResult](t, rec)
		require.False(t, errorByCode(t, result.Rows[0].Errors, services.ImportErrPatentRequired).Fixable)
	})

	t.Run("несовпадение формата номера исправимо, чёрный список машин - нет", func(t *testing.T) {
		carsUA := seedCarsFieldsTemplate(t, db, "import_fixable_cars", 6)
		seedRussianPlateFormat(t, db, true)
		require.NoError(t, db.Create(&models.VehicleBlacklist{
			CarNumber: "А 123 ВС 777", MarkName: "Toyota", Reason: "решение суда", IsActive: true,
		}).Error)

		data := buildCarsRowsUpload(t, 6, []importCarRow{
			{number: "Не номер вовсе", mark: "Volvo"},
			{number: "А123ВС777", mark: "Toyota"},
		})
		rec := postImportFile(t, e, carsUA, "list.xlsx", data, admin)
		require.Equal(t, http.StatusMultiStatus, rec.Code, rec.Body.String())

		result := testutil.ParseResponse[services.ImportListResult](t, rec)
		require.Len(t, result.Rows, 2)

		plate := errorByCode(t, result.Rows[0].Errors, services.ImportErrPlateFormat)
		require.True(t, plate.Fixable, "номер правится прямо в таблице разбора")
		require.Equal(t, "number", plate.Field)
		require.Contains(t, plate.Text, "не соответствует ни одному формату номеров")

		require.False(t, errorByCode(t, result.Rows[1].Errors, services.ImportErrBlacklisted).Fixable)
	})
}
