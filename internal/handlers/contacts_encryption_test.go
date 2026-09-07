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

// Телефон заявки шифруется, а имя инициатора - нет (#2351). Разделение осознанное:
// имя это ФИО, по ним принято решение не шифровать, и оно же идёт в имя каталога
// файлового архива - шифротекст сломал бы пути.
func TestApplicationContacts_PhoneEncryptedNameNot(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	h := testutil.AuthHeader(testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID))
	crypto.SetGlobalKey(contactsKey())
	t.Cleanup(func() { crypto.SetGlobalKey(nil) })

	citizenship := models.Citizenship{Name: "Российская Федерация", IsActive: true}
	require.NoError(t, db.Create(&citizenship).Error)
	uaID := seedUniqueAttachment(t, db, "people", "contacts_tmpl_"+t.Name(), "Люди")
	body := `{
		"message": "проверка шифрования контактов",
		"organization": "Test Organization",
		"responsible_person": "Инициатов Иван",
		"contact_phone": "+7 (911) 222-33-44",
		"data_approval": true,
		"attachments": [{
			"attachment_type": "people", "attachment_name": "people_tmpl",
			"attachment_display_name": "Люди", "unique_attachment_id": ` + itoa(uaID) + `,
			"entry_date_from": "2026-04-01", "entry_date_to": "2099-12-31",
			"entry_time_from": "08:00", "entry_time_to": "18:00",
			"data": {"employees": [{"last_name": "Контактов", "first_name": "Пётр",
				"position": "Слесарь", "passport_series_number": "4700 555666",
				"citizenship_id": ` + itoa(citizenship.ID) + `, "pd_consent": true}]}
		}]
	}`
	require.Equal(t, http.StatusOK, testutil.POST(t, e, "/applications/submit-complete-application", body, h).Code)

	var rawPhone, rawName string
	require.NoError(t, db.Raw(`SELECT COALESCE(contact_phone,''), COALESCE(initiator_name,'')
		FROM applications ORDER BY id DESC LIMIT 1`).Row().Scan(&rawPhone, &rawName))
	assert.NotContains(t, rawPhone, "222-33-44", "телефон не лежит в базе открытым")
	assert.Equal(t, "Инициатов Иван", rawName, "имя инициатора остаётся читаемым: оно идёт в пути архива")

	// Через модель телефон читается обратно - карточка заявки не должна показывать
	// шифротекст.
	var app models.Application
	require.NoError(t, db.Order("id DESC").First(&app).Error)
	require.NotNil(t, app.ContactPhone)
	assert.Equal(t, "+7 (911) 222-33-44", *app.ContactPhone, "телефон расшифровывается при чтении")
}
