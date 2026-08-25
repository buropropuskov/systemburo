package handlers_test

// Тело письма к заявке ищется только точным вхождением. Нечёткое сравнение (%>>)
// читает значение целиком, а письма доходят до 70 килобайт: на стенде из-за одного
// такого сравнения поиск по заявкам занимал 1123 мс при бюджете провайдера 800 и
// стабильно попадал в degraded - человек видел "Не удалось опросить: Заявки".
//
// Тест держит обе половины сделки: по точному фрагменту письма заявка находится
// по-прежнему, а опечатка в теле письма её больше не поднимает. Опечатки в номере,
// организации и составе заявки при этом продолжают работать.

import (
	"net/http"
	"strings"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearch_Applications_MessageExactOnly(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterUser(t, e, "msg_author", "password123", 1, td.OrgID, td.CompanyID)
	assignBaseRole(t, db, "msg_author")
	authorID := userIDByName(t, db, "msg_author")

	// Длинное письмо - как в жизни: сопроводительный текст с разметкой.
	long := strings.Repeat("<p>Сопроводительное письмо к заявке на пропуск.</p>", 200)
	app := models.Application{
		ApplicationNumber: searchStrPtr("20260825/501"),
		Message:           searchStrPtr(long + "<p>Договорённость с Пантелеймоновым</p>"),
		Status:            searchStrPtr("В обработке"),
		SenderUserID:      authorID,
		OrganizationID:    td.OrgID,
	}
	require.NoError(t, db.Create(&app).Error)

	token, _ := testutil.LoginUser(t, e, "msg_author", "password123")

	t.Run("точный фрагмент письма по-прежнему находит заявку", func(t *testing.T) {
		rec := testutil.GET(t, e, "/search?q=Пантелеймоновым", testutil.AuthHeader(token))
		require.Equal(t, http.StatusOK, rec.Code, "тело: %s", rec.Body.String())

		count, found := groupByType(decodeSearch(t, rec.Body.String()), "applications")
		require.True(t, found, "по тексту письма заявка обязана находиться: %s", rec.Body.String())
		assert.Equal(t, 1, count)
	})

	t.Run("опечатка в теле письма заявку не поднимает", func(t *testing.T) {
		rec := testutil.GET(t, e, "/search?q=Пантелеймановым", testutil.AuthHeader(token))
		require.Equal(t, http.StatusOK, rec.Code)

		_, found := groupByType(decodeSearch(t, rec.Body.String()), "applications")
		assert.False(t, found, "нечёткое сравнение по телу письма стоит дороже всего запроса: %s", rec.Body.String())
	})

	t.Run("раздел заявок отвечает, а не уходит в degraded", func(t *testing.T) {
		rec := testutil.GET(t, e, "/search?q=Пантелеймоновым", testutil.AuthHeader(token))
		require.Equal(t, http.StatusOK, rec.Code)

		assert.NotContains(t, rec.Body.String(), `"applications"`+`]`, "раздел не должен попадать в degraded: %s", rec.Body.String())
	})
}
