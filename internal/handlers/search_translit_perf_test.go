package handlers_test

// Скорость поиска после добавления транслитерации (#2414). Варианты запроса умножаются
// на колонки: было слово × колонки, стало слово × формы × колонки. У провайдера бюджет
// 800 мс, и раздел заявок уже однажды в него не уложился из-за одного нечёткого условия
// по телу письма - поэтому запас проверяется тестом, а не на глаз.

import (
	"encoding/json"
	"net/http"
	"testing"

	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearch_Translit_StaysWithinBudget(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterUser(t, e, "translit_perf", "password123", 1, td.OrgID, td.CompanyID)
	assignBaseRole(t, db, "translit_perf")
	grantPermission(t, db, "translit_perf", "page.admin.directories")
	userID := userIDByName(t, db, "translit_perf")

	// Набор, на котором есть что перебирать: и кириллица, и латиница.
	for i := 0; i < 40; i++ {
		seedSearchEmployee(t, db, "Фамилия", userID, td.OrgID, "")
		seedSearchEmployee(t, db, "Familiya", userID, td.OrgID, "")
	}

	token, _ := testutil.LoginUser(t, e, "translit_perf", "password123")

	// Кириллический запрос из двух слов - худший случай: транслит даёт формы каждому.
	rec := testutil.GET(t, e, "/search?q="+urlQuery("Александрович Харитонов"), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "тело: %s", rec.Body.String())

	var ответ struct {
		Data struct {
			TookMS int64 `json:"took_ms"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &ответ))

	t.Logf("сквозной поиск с транслитерацией: %d мс (бюджет провайдера 800)", ответ.Data.TookMS)
	assert.Less(t, ответ.Data.TookMS, int64(800),
		"поиск обязан укладываться в бюджет провайдера, иначе выдача уходит в degraded")
}
