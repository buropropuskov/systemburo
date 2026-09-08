package handlers_test

// Контакты зашифрованы (#2351), поэтому поиск по почте и телефону идёт по свёртке -
// точным совпадением. Первая же живая проверка на стенде показала, что не находится
// никто: условие включалось только когда скрытых до согласия записей нет ни одной, а
// их там 59 из 109. Тест держит обе половины правила разом - по контакту находят
// того, кто согласие дал, и не находят того, кто под маской.

import (
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// requireConsentGate включает запрос согласия: без него маскировать некого и
// проверка выродилась бы в поиск без скрытых записей.
func requireConsentGate(t *testing.T, db *gorm.DB) {
	t.Helper()
	for key, value := range map[string]string{
		"legal.pd_consent_required": "true",
		"legal.pd_consent_text":     "<p>Текст согласия</p>",
	} {
		require.NoError(t, db.Exec(`
			INSERT INTO system_settings (key, value, type) VALUES (?, ?, 'string')
			ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`, key, value).Error)
	}
}

func setContacts(t *testing.T, db *gorm.DB, username, email, phone string) {
	t.Helper()
	var user models.User
	require.NoError(t, db.Where("username = ?", username).First(&user).Error)
	user.Email = &email
	user.Phone = &phone
	require.NoError(t, db.Save(&user).Error)
}

func TestSearch_Users_ByContacts(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterUser(t, e, "contact_open", "password123", 1, td.OrgID, td.CompanyID)
	testutil.RegisterUser(t, e, "shadow_worker", "password123", 1, td.OrgID, td.CompanyID)
	setContacts(t, db, "contact_open", "Petrov@Example.COM", "+7 (916) 123-45-67")
	setContacts(t, db, "shadow_worker", "zzz@example.com", "+7 (916) 765-43-21")

	requireConsentGate(t, db)
	grantConsent(t, db, "contact_open")

	testutil.RegisterUser(t, e, "contact_admin", "password123", 1, td.OrgID, td.CompanyID)
	require.NoError(t, db.Table("users").Where("username = ?", "contact_admin").Update("is_super_admin", true).Error)
	token, _ := testutil.LoginUser(t, e, "contact_admin", "password123")

	search := func(q string) string {
		rec := testutil.GET(t, e, "/search?q="+q, testutil.AuthHeader(token))
		require.Equal(t, http.StatusOK, rec.Code, "тело: %s", rec.Body.String())
		return rec.Body.String()
	}

	t.Run("почта целиком находит работника, несмотря на чужие маски", func(t *testing.T) {
		assert.Contains(t, search("petrov%40example.com"), "contact_open")
	})

	t.Run("регистр в почте значения не имеет", func(t *testing.T) {
		assert.Contains(t, search("PETROV%40example.com"), "contact_open")
	})

	t.Run("телефон находится в другом написании", func(t *testing.T) {
		assert.Contains(t, search("89161234567"), "contact_open")
	})

	t.Run("часть почты не находит - совпадение только точное", func(t *testing.T) {
		assert.NotContains(t, search("petrov"), "contact_open")
	})

	t.Run("контакт скрытого до согласия работника не подтверждается", func(t *testing.T) {
		assert.NotContains(t, search("zzz%40example.com"), "shadow_worker")
	})
}
