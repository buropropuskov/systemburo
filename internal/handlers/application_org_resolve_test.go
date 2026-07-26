package handlers_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// appOrgRefs - организация и компания заявки как они лежат в БД (nullable, поэтому
// указатели). Поля перечислены плоско: gorm молча не маппит анонимно встроенные структуры.
type appOrgRefs struct {
	OrganizationID *int
	CompanyID      *int
}

func readAppOrgRefs(t *testing.T, db *gorm.DB, appID int) appOrgRefs {
	t.Helper()
	var refs appOrgRefs
	require.NoError(t, db.Raw("SELECT organization_id, company_id FROM applications WHERE id = ?", appID).Scan(&refs).Error)
	return refs
}

// submitWithRefs подаёт заявку с произвольным набором полей организации и компании:
// refsJSON - фрагмент тела вида `"organization_id":7` или `"organization":"ООО ..."`.
func submitWithRefs(t *testing.T, e *echo.Echo, token string, uaID int, plate, refsJSON string) *httptest.ResponseRecorder {
	t.Helper()
	body := fmt.Sprintf(`{
		"message": "org resolve test",
		%s,
		"responsible_person": "Test Person",
		"contact_phone": "+79001234567",
		"data_approval": true,
		"attachments": [{
			"attachment_type": "cars",
			"attachment_name": "orgres_cars",
			"attachment_display_name": "Org Resolve Cars",
			"unique_attachment_id": %d,
			"entry_date_from": "2026-04-01",
			"entry_date_to": "2099-12-31",
			"entry_time_from": "08:00",
			"entry_time_to": "18:00",
			"data": {"vehicles": [{"car_number": "%s", "car_brand": "Toyota"}]}
		}]
	}`, refsJSON, uaID, plate)
	return testutil.POST(t, e, "/applications/submit-complete-application", body, testutil.AuthHeader(token))
}

func submittedAppID(t *testing.T, rec *httptest.ResponseRecorder) int {
	t.Helper()
	require.Equal(t, http.StatusOK, rec.Code, "submit: %s", rec.Body.String())
	return testutil.ParseResponse[services.CompleteApplicationResponse](t, rec).ApplicationID
}

func countOrganizations(t *testing.T, db *gorm.DB, key string) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Model(&models.Organization{}).Where("name_normalized = ?", key).Count(&n).Error)
	return n
}

// TestApplicationOrgResolve покрывает резолв организации и компании при подаче (#1437):
// дедупликацию написаний, создание записи «на проверке» и снятие фолбэка, из-за которого
// незнакомое наименование давало заявку с organization_id = NULL.
// Секции живут на одном SetupTestApp: пакет handlers идёт в CI под -race у самой границы
// go test -timeout, и отдельные тесты со своими CleanDB и Seed её уже перебивали.
func TestApplicationOrgResolve(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	uaID := seedUniqueAttachment(t, db, "cars", "orgres_cars", "Org Resolve Cars")
	token := testutil.RegisterAndLogin(t, e, "orgresolve", "pass123", 1, td.OrgID, td.CompanyID)

	var sender models.User
	require.NoError(t, db.Where("username = ?", "orgresolve").First(&sender).Error)

	// Секции резолва подают заявки от чужих организаций, а это послабление гейтится
	// правом (#1437, срез 3): без него автор подачи привязан к своей записи из профиля.
	require.NoError(t, db.Create(&models.UserPermissionOverride{
		UserID:        sender.ID,
		PermissionKey: services.KeyApplicationOrganizationOverride,
		Value:         "allow",
	}).Error)

	// Админский юзер заводится с фиксированным username, поэтому один на весь тест.
	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	orgType := models.OrgTypeContractor
	existing := models.Organization{Name: `ООО "Петрушка"`, Type: &orgType, IsActive: true}
	require.NoError(t, db.Create(&existing).Error)

	// Ровно тот баг из issue: «ооо петрушка» точному сравнению по name не отвечало,
	// заявка уходила без организации и переставала быть видимой коллегам.
	t.Run("другое написание берёт существующую организацию", func(t *testing.T) {
		appID := submittedAppID(t, submitWithRefs(t, e, token, uaID, "A001AA777", `"organization":"ооо петрушка"`))

		refs := readAppOrgRefs(t, db, appID)
		require.NotNil(t, refs.OrganizationID, "организация заявки не должна быть NULL")
		assert.Equal(t, existing.ID, *refs.OrganizationID)
		assert.Equal(t, int64(1), countOrganizations(t, db, "ооо петрушка"), "вторая запись не нужна")
	})

	t.Run("незнакомое наименование заводит запись на проверке", func(t *testing.T) {
		appID := submittedAppID(t, submitWithRefs(t, e, token, uaID, "A002AA777", `"organization_name":"ООО \"Ромашка-Строй\""`))

		refs := readAppOrgRefs(t, db, appID)
		require.NotNil(t, refs.OrganizationID)

		var created models.Organization
		require.NoError(t, db.First(&created, *refs.OrganizationID).Error)
		assert.Equal(t, `ООО "Ромашка-Строй"`, created.Name)
		assert.Equal(t, "ооо ромашка-строй", created.NameNormalized)
		assert.Equal(t, models.ModerationPending, created.ModerationStatus)
		require.NotNil(t, created.Type)
		assert.Equal(t, models.OrgTypeContractor, *created.Type)
		require.NotNil(t, created.CreatedByUserID)
		assert.Equal(t, sender.ID, *created.CreatedByUserID)
		assert.True(t, created.IsActive)
	})

	// Черновая запись участвует в дедупликации так же, как проверенная: иначе каждая
	// следующая заявка на того же подрядчика плодила бы ещё один pending.
	t.Run("повторная подача того же наименования не плодит запись", func(t *testing.T) {
		appID := submittedAppID(t, submitWithRefs(t, e, token, uaID, "A003AA777", `"organization_name":"ооо ромашка - строй"`))

		refs := readAppOrgRefs(t, db, appID)
		require.NotNil(t, refs.OrganizationID)
		assert.Equal(t, int64(1), countOrganizations(t, db, "ооо ромашка-строй"))
	})

	t.Run("organization_id из запроса используется, наименование не смотрится", func(t *testing.T) {
		appID := submittedAppID(t, submitWithRefs(t, e, token, uaID, "A004AA777",
			fmt.Sprintf(`"organization_id":%d,"organization":"ООО \"Мусор\""`, existing.ID)))

		refs := readAppOrgRefs(t, db, appID)
		require.NotNil(t, refs.OrganizationID)
		assert.Equal(t, existing.ID, *refs.OrganizationID)
		assert.Equal(t, int64(0), countOrganizations(t, db, "ооо мусор"), "наименование при заданном id не резолвится")
	})

	t.Run("архивная и несуществующая организация по id отклоняются", func(t *testing.T) {
		// is_active снимаем отдельным UPDATE: у поля есть gorm-тег default:true, и при
		// Create структуры с false gorm вообще опускает колонку, оставляя запись активной.
		archived := models.Organization{Name: `ООО "Архивная"`, Type: &orgType}
		require.NoError(t, db.Create(&archived).Error)
		require.NoError(t, db.Model(&models.Organization{}).Where("id = ?", archived.ID).Update("is_active", false).Error)

		var check models.Organization
		require.NoError(t, db.First(&check, archived.ID).Error)
		require.False(t, check.IsActive, "фикстура должна быть архивной")

		rec := submitWithRefs(t, e, token, uaID, "A005AA777", fmt.Sprintf(`"organization_id":%d`, archived.ID))
		assert.Equal(t, http.StatusBadRequest, rec.Code, "архивная организация: %s", rec.Body.String())

		rec = submitWithRefs(t, e, token, uaID, "A006AA777", `"organization_id":999999`)
		assert.Equal(t, http.StatusBadRequest, rec.Code, "несуществующая организация: %s", rec.Body.String())
	})

	// От вырожденного наименования не остаётся ключа дедупликации, поэтому запись из него
	// не заводим: справочник получил бы мусор от опечатки, который ни с чем не схлопнётся.
	t.Run("вырожденное наименование отклоняется и не заводит запись", func(t *testing.T) {
		rec := submitWithRefs(t, e, token, uaID, "A007AA777", `"organization_name":"\"\""`)
		assert.Equal(t, http.StatusBadRequest, rec.Code, "вырожденное наименование: %s", rec.Body.String())

		var n int64
		require.NoError(t, db.Model(&models.Organization{}).Where("name_normalized = ?", "").Count(&n).Error)
		assert.Equal(t, int64(0), n)

		// Дефисы и прочая пунктуация ключ имеют (нормализация их не выбрасывает), но
		// содержания в таком наименовании столько же: запись из него тоже не заводим.
		for _, junk := range []string{"---", "...", "- - -", "!!!"} {
			rec = submitWithRefs(t, e, token, uaID, "A00"+junk[:1]+"AA111", `"organization_name":"`+junk+`"`)
			assert.Equal(t, http.StatusBadRequest, rec.Code, "наименование без букв и цифр %q: %s", junk, rec.Body.String())
		}
		require.NoError(t, db.Model(&models.Organization{}).Where("name_normalized IN ?", []string{"---", "- - -", "!!!"}).Count(&n).Error)
		assert.Equal(t, int64(0), n, "мусорных записей в справочнике быть не должно")
	})

	// Компании ведёт тот же код, и расхождение между зеркальными сущностями тихое.
	t.Run("компания резолвится так же", func(t *testing.T) {
		appID := submittedAppID(t, submitWithRefs(t, e, token, uaID, "A008AA777",
			`"organization":"","company_name":"ЗАО \"Новая\""`))

		refs := readAppOrgRefs(t, db, appID)
		assert.Nil(t, refs.OrganizationID, "организация не указана - остаётся NULL")
		require.NotNil(t, refs.CompanyID)

		var created models.Company
		require.NoError(t, db.First(&created, *refs.CompanyID).Error)
		assert.Equal(t, "зао новая", created.NameNormalized)
		assert.Equal(t, models.ModerationPending, created.ModerationStatus)
		require.NotNil(t, created.CreatedByUserID)
		assert.Equal(t, sender.ID, *created.CreatedByUserID)
	})

	// Подмена организации закрыта правом: без него подача видит только запись из
	// профиля. Токен заводится один на все секции - права резолвер кэширует на 30 секунд,
	// и повторный логин их не сбросил бы.
	t.Run("подача без права ограничена своей организацией", func(t *testing.T) {
		plainToken := testutil.RegisterAndLogin(t, e, "orgresolveplain", "pass123", 1, td.OrgID, td.CompanyID)

		t.Run("своя организация по id проходит", func(t *testing.T) {
			appID := submittedAppID(t, submitWithRefs(t, e, plainToken, uaID, "A101AA777",
				fmt.Sprintf(`"organization_id":%d`, td.OrgID)))

			refs := readAppOrgRefs(t, db, appID)
			require.NotNil(t, refs.OrganizationID)
			assert.Equal(t, td.OrgID, *refs.OrganizationID)
		})

		// Ключ дедупликации из среза 2 продолжает работать и без права: своя организация,
		// набранная иначе, остаётся своей, иначе фикс «заявки-сироты» упёрся бы в гейт.
		t.Run("своё наименование в другом написании проходит", func(t *testing.T) {
			appID := submittedAppID(t, submitWithRefs(t, e, plainToken, uaID, "A102AA777",
				`"organization_name":"test   organization"`))

			refs := readAppOrgRefs(t, db, appID)
			require.NotNil(t, refs.OrganizationID)
			assert.Equal(t, td.OrgID, *refs.OrganizationID)
		})

		t.Run("чужая организация по id отклоняется", func(t *testing.T) {
			rec := submitWithRefs(t, e, plainToken, uaID, "A103AA777", fmt.Sprintf(`"organization_id":%d`, existing.ID))
			assert.Equal(t, http.StatusForbidden, rec.Code, "чужая организация: %s", rec.Body.String())
		})

		t.Run("чужое наименование не заводит запись", func(t *testing.T) {
			rec := submitWithRefs(t, e, plainToken, uaID, "A104AA777", `"organization_name":"ООО \"Без Права\""`)
			assert.Equal(t, http.StatusForbidden, rec.Code, "новое наименование: %s", rec.Body.String())
			assert.Equal(t, int64(0), countOrganizations(t, db, "ооо без права"),
				"отказ не должен оставлять черновик в справочнике")
		})

		t.Run("чужая компания отклоняется", func(t *testing.T) {
			rec := submitWithRefs(t, e, plainToken, uaID, "A105AA777",
				`"organization":"","company_name":"ЗАО \"Чужая\""`)
			assert.Equal(t, http.StatusForbidden, rec.Code, "чужая компания: %s", rec.Body.String())
		})

		// Организация в профиле есть не у всех: такая заявка держится на компании, и гейт
		// не должен мешать - организация просто не указана.
		t.Run("без организации в профиле заявка идёт по своей компании", func(t *testing.T) {
			noOrgToken := testutil.RegisterAndLogin(t, e, "orgresolvenoorg", "pass123", 1, 0, td.CompanyID)

			appID := submittedAppID(t, submitWithRefs(t, e, noOrgToken, uaID, "A107AA777",
				fmt.Sprintf(`"organization":"","company_id":%d`, td.CompanyID)))

			refs := readAppOrgRefs(t, db, appID)
			assert.Nil(t, refs.OrganizationID)
			require.NotNil(t, refs.CompanyID)
			assert.Equal(t, td.CompanyID, *refs.CompanyID)

			rec := submitWithRefs(t, e, noOrgToken, uaID, "A108AA777", fmt.Sprintf(`"organization_id":%d`, td.OrgID))
			assert.Equal(t, http.StatusForbidden, rec.Code,
				"без организации в профиле любая организация чужая: %s", rec.Body.String())
		})
	})

	// Администратор получает право через allowAll резолвера, отдельный грант ему не нужен.
	t.Run("администратор подаёт от чужой организации", func(t *testing.T) {
		appID := submittedAppID(t, submitWithRefs(t, e, adminToken, uaID, "A106AA777",
			fmt.Sprintf(`"organization_id":%d`, existing.ID)))

		refs := readAppOrgRefs(t, db, appID)
		require.NotNil(t, refs.OrganizationID)
		assert.Equal(t, existing.ID, *refs.OrganizationID)
	})

	t.Run("запись справочника проверена по умолчанию", func(t *testing.T) {
		var seeded models.Organization
		require.NoError(t, db.First(&seeded, td.OrgID).Error)
		assert.Equal(t, models.ModerationApproved, seeded.ModerationStatus,
			"колонка добавляется с DEFAULT: записи, жившие до модерации, читаются как проверенные")

		require.Equal(t, http.StatusOK,
			testutil.POST(t, e, "/organizations", `{"name":"ООО \"Из справочника\"","type":"Подрядчик"}`,
				testutil.AuthHeader(adminToken)).Code)

		var fromDirectory models.Organization
		require.NoError(t, db.Where("name_normalized = ?", "ооо из справочника").First(&fromDirectory).Error)
		assert.Equal(t, models.ModerationApproved, fromDirectory.ModerationStatus)
		assert.Nil(t, fromDirectory.CreatedByUserID)
	})

	// Секция последняя: заводит принимающих, а принимающий видит ВСЕ заявки - в середине
	// теста это поменяло бы видимость соседним секциям.
	//
	// Уведомление о новой записи справочника адресовано пересечению «видит заявку» и
	// «имеет право разбора»: право без заявки разбирать нечего (плашка живёт в детали),
	// заявка без права - призыв к действию, которое сервер всё равно отобьёт.
	t.Run("новая запись справочника зовёт тех, кто может её разобрать", func(t *testing.T) {
		newApprover := func(username string, canModerate bool) models.User {
			t.Helper()
			testutil.RegisterAndLogin(t, e, username, "pass123", 1, td.OrgID, td.CompanyID)
			var user models.User
			require.NoError(t, db.Where("username = ?", username).First(&user).Error)
			require.NoError(t, db.Create(&models.ApplicationApprover{UserID: user.ID}).Error)
			if canModerate {
				require.NoError(t, db.Create(&models.UserPermissionOverride{
					UserID:        user.ID,
					PermissionKey: services.KeyApplicationOrganizationModerate,
					Value:         "allow",
				}).Error)
			}
			return user
		}
		moderator := newApprover("orgresolve_moder", true)
		plainApprover := newApprover("orgresolve_appr", false)

		// Право есть, но заявки не видит: не принимающий, не автор, не согласующий.
		testutil.RegisterAndLogin(t, e, "orgresolve_outsider", "pass123", 1, td.OrgID, td.CompanyID)
		var outsider models.User
		require.NoError(t, db.Where("username = ?", "orgresolve_outsider").First(&outsider).Error)
		require.NoError(t, db.Create(&models.UserPermissionOverride{
			UserID:        outsider.ID,
			PermissionKey: services.KeyApplicationOrganizationModerate,
			Value:         "allow",
		}).Error)

		rec := submitWithRefs(t, e, token, uaID, "A109AA777", `"organization_name":"ООО \"Заря-Новая\""`)
		appID := submittedAppID(t, rec)

		notes := notificationsFor(t, db, moderator.ID, services.NotificationTypeDirectoryPending)
		require.Len(t, notes, 1, "принимающий с правом разбора обязан узнать о новой записи")
		require.NotNil(t, notes[0].Message)
		assert.Contains(t, *notes[0].Message, `ООО "Заря-Новая"`)
		require.NotNil(t, notes[0].Data)
		// application_id в data - по нему уведомление ведёт в заявку, где стоит плашка
		// разбора. Разбираем JSON, а не ищем подстроку: jsonb хранится переформатированным.
		var payload struct {
			ApplicationID int `json:"application_id"`
		}
		require.NoError(t, json.Unmarshal([]byte(*notes[0].Data), &payload))
		assert.Equal(t, appID, payload.ApplicationID)

		assert.Empty(t, notificationsFor(t, db, plainApprover.ID, services.NotificationTypeDirectoryPending),
			"без права разбора звать некуда - действия закрыты middleware")
		assert.Empty(t, notificationsFor(t, db, outsider.ID, services.NotificationTypeDirectoryPending),
			"право без доступа к заявке разбирать нечего")
		assert.Empty(t, notificationsFor(t, db, sender.ID, services.NotificationTypeDirectoryPending),
			"автор подачи сам ввёл это наименование")

		// Повторная подача того же наименования ложится на уже заведённую запись -
		// второй раз звать разбирать её незачем.
		submittedAppID(t, submitWithRefs(t, e, token, uaID, "A110AA777", `"organization_name":"ооо заря-новая"`))
		assert.Len(t, notificationsFor(t, db, moderator.ID, services.NotificationTypeDirectoryPending), 1)

		// Компания идёт тем же кодом, но зеркальность должна держаться тестом, а не верой.
		submittedAppID(t, submitWithRefs(t, e, token, uaID, "A111AA777", `"company_name":"ООО \"Заря-Компания\""`))
		notes = notificationsFor(t, db, moderator.ID, services.NotificationTypeDirectoryPending)
		require.Len(t, notes, 2)
		require.NotNil(t, notes[1].Title)
		assert.Equal(t, "Новая компания на проверке", *notes[1].Title)
		require.NotNil(t, notes[1].Message)
		assert.Contains(t, *notes[1].Message, `ООО "Заря-Компания"`)
	})

	// Ровно тот ввод, с которого правило и потребовали: строчная ОПФ, строчное название,
	// незакрытая кавычка. В справочник обязана уехать аккуратная строка, а не набранная.
	t.Run("оформление наименования из заявки канонизируется", func(t *testing.T) {
		appID := submittedAppID(t, submitWithRefs(t, e, token, uaID, "A113AA777", `"organization_name":"ооо \"братишк"`))

		refs := readAppOrgRefs(t, db, appID)
		require.NotNil(t, refs.OrganizationID)
		var created models.Organization
		require.NoError(t, db.First(&created, *refs.OrganizationID).Error)
		assert.Equal(t, `ООО "Братишк"`, created.Name)
		assert.Equal(t, "ооо братишк", created.NameNormalized)
		assert.Equal(t, models.ModerationPending, created.ModerationStatus)

		// Тот же ключ в другом написании ложится на уже заведённую запись: канонизация
		// оформления не должна плодить второй вариант той же организации.
		second := submittedAppID(t, submitWithRefs(t, e, token, uaID, "A114AA777", `"organization_name":"ООО Братишк"`))
		secondRefs := readAppOrgRefs(t, db, second)
		require.NotNil(t, secondRefs.OrganizationID)
		assert.Equal(t, created.ID, *secondRefs.OrganizationID)
		assert.Equal(t, int64(1), countOrganizations(t, db, "ооо братишк"))
	})

	// Гонка двух подач с одним новым наименованием: partial unique index по ключу (#1437,
	// срез 9) отбивает второй INSERT, и подача обязана лечь на запись соперника, а не
	// упасть пятисоткой. Соперник эмулируется отдельной транзакцией, которая держит свою
	// вставку незакоммиченной: тогда подача упирается в блокировку индекса гарантированно,
	// а не по прихоти планировщика.
	t.Run("гонка подач привязывает заявку к записи соперника", func(t *testing.T) {
		rival := db.Begin()
		require.NoError(t, rival.Error)
		defer rival.Rollback()

		var rivalID int
		require.NoError(t, rival.Raw(
			`INSERT INTO organizations (name, type, is_active, name_normalized, moderation_status, created_by_user_id)
			 VALUES (?, ?, true, ?, ?, ?) RETURNING id`,
			`ООО "Гонка"`, models.OrgTypeContractor, "ооо гонка", models.ModerationPending, sender.ID).Scan(&rivalID).Error)
		require.NotZero(t, rivalID)

		// Наименование написано иначе, чем у соперника: по name конфликта нет, упереться
		// подача должна именно в ключ дедупликации.
		done := make(chan *httptest.ResponseRecorder, 1)
		go func() {
			done <- submitWithRefs(t, e, token, uaID, "A112AA777", `"organization_name":"ООО Гонка"`)
		}()

		waitForIndexLock(t, db)
		require.NoError(t, rival.Commit().Error)

		appID := submittedAppID(t, <-done)
		refs := readAppOrgRefs(t, db, appID)
		require.NotNil(t, refs.OrganizationID, "заявка без организации - тот самый баг, который закрывал эпик")
		assert.Equal(t, rivalID, *refs.OrganizationID, "заявка должна лечь на запись соперника")
		assert.Equal(t, int64(1), countOrganizations(t, db, "ооо гонка"), "второй записи с тем же ключом быть не должно")
	})
}

// waitForIndexLock ждёт, пока чей-то INSERT повиснет на блокировке уникального индекса.
// Без этого ожидания подача успела бы закоммититься до соперника, ветка восстановления
// после конфликта не выполнилась бы, и тест зеленел бы, ничего не проверив.
func waitForIndexLock(t *testing.T, db *gorm.DB) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		var waiting int64
		require.NoError(t, db.Raw(`SELECT COUNT(*) FROM pg_stat_activity
			WHERE wait_event_type = 'Lock' AND query ILIKE '%organizations%'`).Scan(&waiting).Error)
		if waiting > 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("подача не упёрлась в блокировку уникального индекса - гонка не воспроизведена")
		}
		time.Sleep(20 * time.Millisecond)
	}
}
