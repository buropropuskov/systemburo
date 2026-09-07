package handlers_test

import (
	"net/http"
	"testing"

	"systemburo/internal/crypto"
	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Контакты работника шифруются (#2351). Поиск по части строки при этом теряется -
// это цена решения; остаётся точный поиск по свёртке, и он обязан пережить разное
// написание: у почты регистр, у телефона +7, 8, скобки и пробелы.

func contactsKey() []byte {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 11)
	}
	return key
}

func TestContacts_StoredEncrypted(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	h := testutil.AuthHeader(testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID))
	crypto.SetGlobalKey(contactsKey())
	t.Cleanup(func() { crypto.SetGlobalKey(nil) })

	testutil.RegisterAndLogin(t, e, "contact_user", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)
	require.Equal(t, http.StatusOK, testutil.PUT(t, e, "/users/contact_user/info",
		`{"email":"Secret.Box@Example.COM","phone":"+7 (900) 123-45-67","position":"Инженер"}`, h).Code)

	var rawEmail, rawPhone string
	require.NoError(t, db.Raw("SELECT COALESCE(email,''), COALESCE(phone,'') FROM users WHERE username = ?", "contact_user").
		Row().Scan(&rawEmail, &rawPhone))
	assert.NotContains(t, rawEmail, "Example", "адрес не лежит в базе открытым")
	assert.NotContains(t, rawPhone, "123-45-67", "номер не лежит в базе открытым")

	// Через модель читается обратно - работа с карточкой не должна ломаться.
	var user models.User
	require.NoError(t, db.Where("username = ?", "contact_user").First(&user).Error)
	require.NotNil(t, user.Email)
	// Регистр адреса сохраняется как введён: часть почтовых служб различает его в
	// левой части, и приводить чужой адрес к нижнему регистру система не вправе
	// (см. normalizeUserEmail). К нижнему регистру приводится только СВЁРТКА, ради
	// сравнения и поиска.
	assert.Equal(t, "Secret.Box@Example.COM", *user.Email, "адрес хранится как введён")
	require.NotNil(t, user.Phone)
	assert.Equal(t, "+7 (900) 123-45-67", *user.Phone, "номер сохраняется как введён")
}

// Занятость адреса проверяется по свёртке: два ящика, различающиеся регистром, на
// почтовых службах ведут в один и тот же, и пароль одного работника пришёл бы другому.
func TestContacts_EmailUniquenessSurvivesEncryption(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	h := testutil.AuthHeader(testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID))
	crypto.SetGlobalKey(contactsKey())
	t.Cleanup(func() { crypto.SetGlobalKey(nil) })

	testutil.RegisterAndLogin(t, e, "contact_first", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)
	testutil.RegisterAndLogin(t, e, "contact_second", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)

	require.Equal(t, http.StatusOK, testutil.PUT(t, e, "/users/contact_first/info",
		`{"email":"busy@example.com"}`, h).Code)

	rec := testutil.PUT(t, e, "/users/contact_second/info", `{"email":"BUSY@EXAMPLE.COM"}`, h)
	assert.Equal(t, http.StatusBadRequest, rec.Code, "тот же адрес в другом регистре занят: "+rec.Body.String())
}
