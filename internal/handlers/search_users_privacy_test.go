package handlers_test

import (
	"net/http"
	"testing"

	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearch_Users_HiddenNamesExcluded(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	testutil.RegisterUser(t, e, "privacy_subject", "password123", 1, td.OrgID, td.CompanyID)
	setUserName(t, db, "privacy_subject", "Скрытофамилиев", "Редкоимениев", "Особоотчествич")

	search := func(q string) string {
		t.Helper()
		rec := testutil.GET(t, e, "/search?types=users&q="+urlQuery(q), testutil.AuthHeader(admin))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.NotContains(t, rec.Body.String(), `"degraded":true`)
		return rec.Body.String()
	}
	for _, tc := range []struct {
		name, required, text string
		hidden               bool
	}{
		{"выключено", "false", "<p>Согласие</p>", false},
		{"пустой текст", "true", "<p><br></p>", false},
		{"активный запрос", "true", "<p>Согласие</p>", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for key, value := range map[string]string{"legal.pd_consent_required": tc.required, "legal.pd_consent_text": tc.text} {
				require.NoError(t, db.Exec(`INSERT INTO system_settings (key, value, type) VALUES (?, ?, 'string')
					ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`, key, value).Error)
			}
			for _, q := range []string{"Скрытофамилиев", "Редкоимениев", "Особоотчествич", "Скрытофамилиев Редкоимениев", "Скрытофамилие"} {
				body := search(q)
				if tc.hidden {
					assert.NotContains(t, body, "privacy_subject", q)
				} else {
					assert.Contains(t, body, "privacy_subject", q)
				}
			}
			assert.Contains(t, search("privacy_subject"), "privacy_subject", "логин остаётся доступен")
		})
	}
	grantConsent(t, db, "privacy_subject")
	assert.Contains(t, search("Скрытофамилиев"), "privacy_subject", "согласие снимает маску")
	// Архивные, заблокированные и суперадминистраторы не входят в gatedUsersWhere.
	for _, flags := range []map[string]any{{"is_active": false}, {"is_active": true, "is_banned": true}, {"is_banned": false, "is_super_admin": true}} {
		require.NoError(t, db.Exec("DELETE FROM pd_consents WHERE user_id = (SELECT id FROM users WHERE username = ?)", "privacy_subject").Error)
		require.NoError(t, db.Table("users").Where("username = ?", "privacy_subject").Updates(flags).Error)
		assert.Contains(t, search("Скрытофамилиев"), "privacy_subject", "сохраняется существующая политика gatedUsersWhere")
	}
}

func TestSearch_Users_HiddenNameDoesNotAffectRank(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	for _, username := range []string{"rankprivacy_first", "rankprivacy_second"} {
		testutil.RegisterUser(t, e, username, "password123", 1, td.OrgID, td.CompanyID)
	}
	first := setUserName(t, db, "rankprivacy_first", "rankprivacy", "", "")
	second := setUserName(t, db, "rankprivacy_second", "Нейтральный", "", "")
	requireConsentGate(t, db)
	rec := testutil.GET(t, e, "/search?types=users&limit=1&q=rankprivacy", testutil.AuthHeader(admin))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	resp := decodeSearch(t, rec.Body.String())
	var ids []int
	for _, group := range resp.Data.Groups {
		if group.Type == "users" {
			for _, item := range group.Items {
				ids = append(ids, item.ID)
			}
		}
	}
	require.Equal(t, []int{second}, ids, "скрытая фамилия первой записи не повышает её ранг перед LIMIT")
	assert.NotEqual(t, first, second)
}
