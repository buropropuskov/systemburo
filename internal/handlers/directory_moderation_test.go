package handlers_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// seedModerationOrg заводит организацию с нужным статусом разбора.
func seedModerationOrg(t *testing.T, db *gorm.DB, name, moderation string) models.Organization {
	t.Helper()
	orgType := models.OrgTypeContractor
	org := models.Organization{Name: name, Type: &orgType, IsActive: true, ModerationStatus: moderation}
	require.NoError(t, db.Create(&org).Error)
	return org
}

// seedModerationOrgBy заводит черновик организации от имени authorID: разбор сообщает
// исход именно автору наименования (created_by_user_id), у записи без него адресата нет.
func seedModerationOrgBy(t *testing.T, db *gorm.DB, name, moderation string, authorID int) models.Organization {
	t.Helper()
	org := seedModerationOrg(t, db, name, moderation)
	require.NoError(t, db.Model(&models.Organization{}).Where("id = ?", org.ID).
		Update("created_by_user_id", authorID).Error)
	org.CreatedByUserID = &authorID
	return org
}

// notificationsFor - уведомления пользователя заданного типа, в порядке появления.
func notificationsFor(t *testing.T, db *gorm.DB, userID int, notifType string) []models.Notification {
	t.Helper()
	var notes []models.Notification
	require.NoError(t, db.Where("user_id = ? AND type = ?", userID, notifType).Order("id").Find(&notes).Error)
	return notes
}

func orgByID(t *testing.T, db *gorm.DB, id int) models.Organization {
	t.Helper()
	var org models.Organization
	require.NoError(t, db.First(&org, id).Error)
	return org
}

// countRows считает строки таблицы по колонке-ссылке - так проверяется перепривязка.
func countRows(t *testing.T, db *gorm.DB, table, column string, value int) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Table(table).Where(column+" = ?", value).Count(&n).Error)
	return n
}

func auditActions(t *testing.T, db *gorm.DB, entity string, entityID int) []string {
	t.Helper()
	var actions []string
	require.NoError(t, db.Table("audit_log").
		Where("entity_type = ? AND entity_id = ?", entity, entityID).
		Order("id").Pluck("action", &actions).Error)
	return actions
}

// TestDirectoryModeration покрывает разбор записей справочника «на проверке» (#1437):
// гейт по праву, три действия принимающего и полноту перепривязки при слиянии.
// Секции живут на одном SetupTestApp: пакет handlers идёт в CI под -race у самой границы
// go test -timeout, и отдельные тесты со своими CleanDB и Seed её уже перебивали.
func TestDirectoryModeration(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	plainToken := testutil.RegisterAndLogin(t, e, "modplain", "pass123", 1, td.OrgID, td.CompanyID)
	token := testutil.RegisterAndLogin(t, e, "moderator", "pass123", 1, td.OrgID, td.CompanyID)

	var moderator models.User
	require.NoError(t, db.Where("username = ?", "moderator").First(&moderator).Error)
	require.NoError(t, db.Create(&models.UserPermissionOverride{
		UserID:        moderator.ID,
		PermissionKey: services.KeyApplicationOrganizationModerate,
		Value:         "allow",
	}).Error)

	post := func(path, body, tkn string) (int, string) {
		rec := testutil.POST(t, e, path, body, testutil.AuthHeader(tkn))
		return rec.Code, rec.Body.String()
	}
	patch := func(path, body, tkn string) (int, string) {
		rec := testutil.PATCH(t, e, path, body, testutil.AuthHeader(tkn))
		return rec.Code, rec.Body.String()
	}

	t.Run("без права разбор закрыт", func(t *testing.T) {
		org := seedModerationOrg(t, db, `ООО "Гейт"`, models.ModerationPending)

		code, body := post(fmt.Sprintf("/organizations/%d/moderation/approve", org.ID), ``, plainToken)
		assert.Equal(t, http.StatusForbidden, code, body)

		code, body = patch(fmt.Sprintf("/organizations/%d/moderation/rename", org.ID), `{"name":"ООО Новое"}`, plainToken)
		assert.Equal(t, http.StatusForbidden, code, body)

		code, body = post(fmt.Sprintf("/organizations/%d/moderation/merge", org.ID), `{"target_id":1}`, plainToken)
		assert.Equal(t, http.StatusForbidden, code, body)

		code, body = post(fmt.Sprintf("/companies/%d/moderation/approve", td.CompanyID), ``, plainToken)
		assert.Equal(t, http.StatusForbidden, code, body)

		code, body = patch(fmt.Sprintf("/companies/%d/moderation/rename", td.CompanyID), `{"name":"ООО Новое"}`, plainToken)
		assert.Equal(t, http.StatusForbidden, code, body)

		code, body = post(fmt.Sprintf("/companies/%d/moderation/merge", td.CompanyID), `{"target_id":1}`, plainToken)
		assert.Equal(t, http.StatusForbidden, code, body)

		assert.Equal(t, models.ModerationPending, orgByID(t, db, org.ID).ModerationStatus)
	})

	t.Run("подтверждение переводит запись в проверенные", func(t *testing.T) {
		org := seedModerationOrg(t, db, `ООО "Подтверждаемая"`, models.ModerationPending)

		code, body := post(fmt.Sprintf("/organizations/%d/moderation/approve", org.ID), ``, token)
		require.Equal(t, http.StatusOK, code, body)

		assert.Equal(t, models.ModerationApproved, orgByID(t, db, org.ID).ModerationStatus)
		assert.Contains(t, auditActions(t, db, models.AuditEntityOrganization, org.ID), models.OrganizationActionApproved)
	})

	t.Run("повторный разбор проверенной записи отклоняется", func(t *testing.T) {
		org := seedModerationOrg(t, db, `ООО "Уже проверенная"`, models.ModerationApproved)

		code, body := post(fmt.Sprintf("/organizations/%d/moderation/approve", org.ID), ``, token)
		assert.Equal(t, http.StatusBadRequest, code, body)
	})

	// Ветка живёт на базах, где partial unique index по ключу ещё не встал из-за неслитых
	// дублей (#1437, срез 9): с индексом черновик-двойник проверенной записи создать уже
	// нельзя, поэтому состояние воспроизводится с временно снятым индексом.
	t.Run("подтверждение при появившемся дубле предлагает привязку", func(t *testing.T) {
		withoutOrgNameKeyIndex(t, db)
		existing := seedModerationOrg(t, db, `ООО "Двойник"`, models.ModerationApproved)
		draft := seedModerationOrg(t, db, `ооо двойник`, models.ModerationPending)

		code, body := post(fmt.Sprintf("/organizations/%d/moderation/approve", draft.ID), ``, token)
		require.Equal(t, http.StatusOK, code, body)

		result := testutil.ParseResponse[services.DirectoryModerationResult](t, testutil.POST(t, e,
			fmt.Sprintf("/organizations/%d/moderation/approve", draft.ID), ``, testutil.AuthHeader(token)))
		require.Equal(t, services.DirectoryModerationConflict, result.Status)
		require.NotNil(t, result.Existing)
		assert.Equal(t, existing.ID, result.Existing.ID)
		// Черновик не тронут: принимающий ещё выбирает, что с ним делать.
		assert.Equal(t, models.ModerationPending, orgByID(t, db, draft.ID).ModerationStatus)
	})

	// Оформление наименования держит система, а не аккуратность принимающего (#1437):
	// «подтвердить» - самое вероятное действие над записью с верным по смыслу, но криво
	// набранным наименованием, и оставлять её в справочнике как есть нельзя.
	t.Run("подтверждение приводит наименование к канону", func(t *testing.T) {
		org := seedModerationOrg(t, db, `ооо "канон-подтверждение`, models.ModerationPending)

		rec := testutil.POST(t, e, fmt.Sprintf("/organizations/%d/moderation/approve", org.ID), ``, testutil.AuthHeader(token))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		result := testutil.ParseResponse[services.DirectoryModerationResult](t, rec)
		require.Equal(t, services.DirectoryModerationApproved, result.Status)
		require.NotNil(t, result.Entry)
		assert.Equal(t, `ООО "Канон-подтверждение"`, result.Entry.Name)

		stored := orgByID(t, db, org.ID)
		assert.Equal(t, `ООО "Канон-подтверждение"`, stored.Name)
		assert.Equal(t, models.ModerationApproved, stored.ModerationStatus)
		assert.Equal(t, "ооо канон-подтверждение", stored.NameNormalized, "ключ дедупликации не меняется")
	})

	// Принимающий правит опечатку, оформление за ним доводит система.
	t.Run("исправление наименования канонизируется", func(t *testing.T) {
		org := seedModerationOrg(t, db, `ооо канон-ремашка`, models.ModerationPending)

		rec := testutil.PATCH(t, e, fmt.Sprintf("/organizations/%d/moderation/rename", org.ID),
			`{"name":"ооо \"канон-ромашка"}`, testutil.AuthHeader(token))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		result := testutil.ParseResponse[services.DirectoryModerationResult](t, rec)
		require.Equal(t, services.DirectoryModerationRenamed, result.Status)
		require.NotNil(t, result.Entry)
		assert.Equal(t, `ООО "Канон-ромашка"`, result.Entry.Name)
		assert.Equal(t, `ООО "Канон-ромашка"`, orgByID(t, db, org.ID).Name)
	})

	t.Run("исправление наименования разбирает запись", func(t *testing.T) {
		org := seedModerationOrg(t, db, `ооо рмашка`, models.ModerationPending)

		rec := testutil.PATCH(t, e, fmt.Sprintf("/organizations/%d/moderation/rename", org.ID),
			`{"name":"ООО \"Ромашка\""}`, testutil.AuthHeader(token))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		result := testutil.ParseResponse[services.DirectoryModerationResult](t, rec)
		require.Equal(t, services.DirectoryModerationRenamed, result.Status)

		updated := orgByID(t, db, org.ID)
		assert.Equal(t, `ООО "Ромашка"`, updated.Name)
		assert.Equal(t, "ооо ромашка", updated.NameNormalized, "ключ дедупликации обязан пересчитаться")
		assert.Equal(t, models.ModerationApproved, updated.ModerationStatus)
		assert.Contains(t, auditActions(t, db, models.AuditEntityOrganization, org.ID), models.OrganizationActionRenamed)

		// Модалка истории рисует «было -> стало» по name и from.name: свои ключи
		// оставили бы запись «Организация переименована» без содержательной строки.
		var raw string
		require.NoError(t, db.Raw(`SELECT details FROM audit_log WHERE entity_type = ? AND entity_id = ? AND action = ? ORDER BY id DESC LIMIT 1`,
			models.AuditEntityOrganization, org.ID, models.OrganizationActionRenamed).Scan(&raw).Error)
		var details struct {
			Name string `json:"name"`
			From struct {
				Name string `json:"name"`
			} `json:"from"`
		}
		require.NoError(t, json.Unmarshal([]byte(raw), &details))
		assert.Equal(t, `ООО "Ромашка"`, details.Name)
		assert.Equal(t, `ооо рмашка`, details.From.Name)
	})

	// Второй черновик с тем же ключом конфликтом не считается: привязать к нему нельзя
	// (цель обязана быть проверенной), и принимающий упёрся бы в тупик.
	// Соседние черновики разбираются независимо: конфликт считается по ключу дедупликации,
	// а не по факту «рядом есть неразобранная запись». Черновик с ТЕМ ЖЕ ключом рядом
	// существовать не может - его не пускает уникальный индекс (#1437, срез 9).
	t.Run("другой черновик не мешает подтверждению", func(t *testing.T) {
		first := seedModerationOrg(t, db, `ООО "Параллельный"`, models.ModerationPending)
		seedModerationOrg(t, db, `ООО "Соседний"`, models.ModerationPending)

		rec := testutil.POST(t, e, fmt.Sprintf("/organizations/%d/moderation/approve", first.ID), ``, testutil.AuthHeader(token))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		result := testutil.ParseResponse[services.DirectoryModerationResult](t, rec)

		assert.Equal(t, services.DirectoryModerationApproved, result.Status)
		assert.Equal(t, models.ModerationApproved, orgByID(t, db, first.ID).ModerationStatus)
	})

	t.Run("исправление в занятое наименование предлагает привязку", func(t *testing.T) {
		existing := seedModerationOrg(t, db, `ООО "Занятое"`, models.ModerationApproved)
		draft := seedModerationOrg(t, db, `ООО "Черновик занятого"`, models.ModerationPending)

		rec := testutil.PATCH(t, e, fmt.Sprintf("/organizations/%d/moderation/rename", draft.ID),
			`{"name":"ооо занятое"}`, testutil.AuthHeader(token))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		result := testutil.ParseResponse[services.DirectoryModerationResult](t, rec)

		require.Equal(t, services.DirectoryModerationConflict, result.Status)
		require.NotNil(t, result.Existing)
		assert.Equal(t, existing.ID, result.Existing.ID)
		assert.Equal(t, `ООО "Черновик занятого"`, orgByID(t, db, draft.ID).Name, "наименование не должно меняться при конфликте")
	})

	t.Run("вырожденное наименование отклоняется", func(t *testing.T) {
		org := seedModerationOrg(t, db, `ООО "Вырожденная"`, models.ModerationPending)

		code, body := patch(fmt.Sprintf("/organizations/%d/moderation/rename", org.ID), `{"name":"\"\""}`, token)
		assert.Equal(t, http.StatusBadRequest, code, body)
	})

	t.Run("привязка переносит ссылки и удаляет черновик", func(t *testing.T) {
		target := seedModerationOrg(t, db, `ООО "Цель"`, models.ModerationApproved)
		draft := seedModerationOrg(t, db, `ООО "Дубль цели"`, models.ModerationPending)

		// Ссылки черновика: заявка, вложение, машина, сотрудник, пользователь и привязки.
		status := "В работе"
		app := models.Application{OrganizationID: draft.ID, SenderUserID: moderator.ID, Status: &status}
		require.NoError(t, db.Create(&app).Error)
		require.NoError(t, db.Exec(`INSERT INTO attachments (application_id, organization_id, attachment_type) VALUES (?, ?, 'cars')`, app.ID, draft.ID).Error)
		require.NoError(t, db.Exec(`INSERT INTO unique_cars (number, organization_id) VALUES ('А111АА777', ?)`, draft.ID).Error)
		require.NoError(t, db.Exec(`INSERT INTO unique_employees (last_name, organization_id) VALUES ('Иванов', ?)`, draft.ID).Error)
		require.NoError(t, db.Exec(`UPDATE users SET organization_id = ? WHERE id = ?`, draft.ID, moderator.ID).Error)
		require.NoError(t, db.Exec(`INSERT INTO organization_users (organization_id, user_id) VALUES (?, ?)`, draft.ID, moderator.ID).Error)

		rec := testutil.POST(t, e, fmt.Sprintf("/organizations/%d/moderation/merge", draft.ID),
			fmt.Sprintf(`{"target_id":%d}`, target.ID), testutil.AuthHeader(token))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		result := testutil.ParseResponse[services.DirectoryMergeResult](t, rec)
		assert.Equal(t, target.ID, result.Target.ID)

		for _, ref := range []struct{ table, column string }{
			{"applications", "organization_id"},
			{"attachments", "organization_id"},
			{"unique_cars", "organization_id"},
			{"unique_employees", "organization_id"},
			{"users", "organization_id"},
			{"organization_users", "organization_id"},
		} {
			assert.Zero(t, countRows(t, db, ref.table, ref.column, draft.ID), "%s: ссылки черновика должны переехать", ref.table)
			assert.NotZero(t, countRows(t, db, ref.table, ref.column, target.ID), "%s: ссылки должны быть у цели", ref.table)
		}

		var left int64
		require.NoError(t, db.Model(&models.Organization{}).Where("id = ?", draft.ID).Count(&left).Error)
		assert.Zero(t, left, "черновик после привязки не нужен")
		assert.Contains(t, auditActions(t, db, models.AuditEntityOrganization, target.ID), models.OrganizationActionMerged)
	})

	t.Run("привязка не плодит дубли связок", func(t *testing.T) {
		target := seedModerationOrg(t, db, `ООО "Цель связок"`, models.ModerationApproved)
		draft := seedModerationOrg(t, db, `ООО "Дубль связок"`, models.ModerationPending)

		// Один и тот же ответственный привязан и к черновику, и к цели.
		require.NoError(t, db.Exec(`INSERT INTO organization_users (organization_id, user_id) VALUES (?, ?), (?, ?)`,
			draft.ID, moderator.ID, target.ID, moderator.ID).Error)

		rec := testutil.POST(t, e, fmt.Sprintf("/organizations/%d/moderation/merge", draft.ID),
			fmt.Sprintf(`{"target_id":%d}`, target.ID), testutil.AuthHeader(token))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		assert.Equal(t, int64(1), countRows(t, db, "organization_users", "organization_id", target.ID),
			"дублирующая привязка обязана исчезнуть, а не переехать второй строкой")
	})

	t.Run("привязка к непроверенной записи и к самой себе отклоняется", func(t *testing.T) {
		draft := seedModerationOrg(t, db, `ООО "Черновик А"`, models.ModerationPending)
		otherDraft := seedModerationOrg(t, db, `ООО "Черновик Б"`, models.ModerationPending)

		code, body := post(fmt.Sprintf("/organizations/%d/moderation/merge", draft.ID),
			fmt.Sprintf(`{"target_id":%d}`, otherDraft.ID), token)
		assert.Equal(t, http.StatusBadRequest, code, body)

		code, body = post(fmt.Sprintf("/organizations/%d/moderation/merge", draft.ID),
			fmt.Sprintf(`{"target_id":%d}`, draft.ID), token)
		assert.Equal(t, http.StatusBadRequest, code, body)
	})

	t.Run("компании разбираются так же", func(t *testing.T) {
		compType := models.OrgTypeContractor
		target := models.Company{Name: `ООО "Цель-компания"`, Type: &compType, IsActive: true, ModerationStatus: models.ModerationApproved}
		require.NoError(t, db.Create(&target).Error)
		draft := models.Company{Name: `ООО "Дубль компании"`, Type: &compType, IsActive: true, ModerationStatus: models.ModerationPending}
		require.NoError(t, db.Create(&draft).Error)

		require.NoError(t, db.Exec(`INSERT INTO unique_cars (number, company_id) VALUES ('В222ВВ777', ?)`, draft.ID).Error)

		rec := testutil.POST(t, e, fmt.Sprintf("/companies/%d/moderation/merge", draft.ID),
			fmt.Sprintf(`{"target_id":%d}`, target.ID), testutil.AuthHeader(token))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		assert.Zero(t, countRows(t, db, "unique_cars", "company_id", draft.ID))
		assert.NotZero(t, countRows(t, db, "unique_cars", "company_id", target.ID))

		pending := models.Company{Name: `ООО "Компания на проверке"`, Type: &compType, IsActive: true, ModerationStatus: models.ModerationPending}
		require.NoError(t, db.Create(&pending).Error)
		code, body := post(fmt.Sprintf("/companies/%d/moderation/approve", pending.ID), ``, token)
		require.Equal(t, http.StatusOK, code, body)
		var approved models.Company
		require.NoError(t, db.First(&approved, pending.ID).Error)
		assert.Equal(t, models.ModerationApproved, approved.ModerationStatus)
	})

	// Плашка разбора в детали заявки (#1437 срез 7) держится на этих двух полях: без них
	// фронт не отличает заведённое подачей наименование от обычного, а список выбора цели
	// привязки предложил бы черновик, к которому привязывать запрещено.
	t.Run("статус разбора виден в заявке и в справочнике", func(t *testing.T) {
		draft := seedModerationOrg(t, db, `ООО "Из заявки"`, models.ModerationPending)
		number, confirmation, status := "MODERATION-1", models.ConfirmationApproved, models.StatusInWork
		app := models.Application{
			ApplicationNumber: &number,
			Confirmation:      &confirmation,
			Status:            &status,
			OrganizationID:    draft.ID,
			SenderUserID:      moderator.ID,
		}
		require.NoError(t, db.Create(&app).Error)

		rows := testutil.ParseSlice(t, testutil.GET(t, e, "/applications", testutil.AuthHeader(token)))
		var found map[string]interface{}
		for _, row := range rows {
			if id, ok := row["id"].(float64); ok && int(id) == app.ID {
				found = row
			}
		}
		require.NotNil(t, found, "заявка отправителя видна ему в списке")
		assert.Equal(t, models.ModerationPending, found["organization_moderation_status"])
		// Компании у заявки нет - поле приходит пустым, а не статусом чужой записи.
		assert.Nil(t, found["company_moderation_status"])

		// Деталь перечитывается по live-сигналу и после действий - статус обязан приходить
		// и здесь, иначе разбор, сделанный другим принимающим, не погасит плашку.
		details := testutil.ParseResponse[map[string]interface{}](t, testutil.GET(t,
			e, fmt.Sprintf("/applications/%d/details", app.ID), testutil.AuthHeader(token)))
		assert.Equal(t, models.ModerationPending, details["organization_moderation_status"])
		assert.Nil(t, details["company_moderation_status"])

		directory := testutil.ParseSlice(t, testutil.GET(t, e, "/organizations", testutil.AuthHeader(token)))
		statuses := map[int]interface{}{}
		for _, row := range directory {
			if id, ok := row["id"].(float64); ok {
				statuses[int(id)] = row["moderation_status"]
			}
		}
		assert.Equal(t, models.ModerationPending, statuses[draft.ID], "черновик помечен и отсеивается из целей привязки")
		assert.Equal(t, models.ModerationApproved, statuses[td.OrgID])
	})

	// Инициатор наименования узнаёт исход разбора, когда наименование поменялось:
	// исправление и привязка меняют то, что он видит в своей заявке. Подтверждение
	// уведомления не даёт - для него ничего не изменилось, бейдж просто гаснет.
	t.Run("разбор сообщает инициатору наименования", func(t *testing.T) {
		var plain models.User
		require.NoError(t, db.Where("username = ?", "modplain").First(&plain).Error)

		draft := seedModerationOrgBy(t, db, `ооо снежинка`, models.ModerationPending, plain.ID)
		rec := testutil.PATCH(t, e, fmt.Sprintf("/organizations/%d/moderation/rename", draft.ID),
			`{"name":"ООО \"Снежинка\""}`, testutil.AuthHeader(token))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		notes := notificationsFor(t, db, plain.ID, services.NotificationTypeDirectoryResolved)
		require.Len(t, notes, 1, "исправление наименования обязано дойти до инициатора")
		require.NotNil(t, notes[0].Message)
		assert.Contains(t, *notes[0].Message, `ооо снежинка`, "в тексте должно быть исходное наименование")
		assert.Contains(t, *notes[0].Message, `ООО "Снежинка"`, "и то, на которое его исправили")

		// Привязка - тот же класс события; проверяем на компании, чтобы зеркальность
		// справочников держалась тестом, а не только общим кодом.
		compType := models.OrgTypeContractor
		target := models.Company{Name: `ООО "Мороз"`, Type: &compType, IsActive: true, ModerationStatus: models.ModerationApproved}
		require.NoError(t, db.Create(&target).Error)
		companyDraft := models.Company{
			Name: `ООО "Мороз-дубль"`, Type: &compType, IsActive: true,
			ModerationStatus: models.ModerationPending, CreatedByUserID: &plain.ID,
		}
		require.NoError(t, db.Create(&companyDraft).Error)

		code, body := post(fmt.Sprintf("/companies/%d/moderation/merge", companyDraft.ID),
			fmt.Sprintf(`{"target_id":%d}`, target.ID), token)
		require.Equal(t, http.StatusOK, code, body)

		notes = notificationsFor(t, db, plain.ID, services.NotificationTypeDirectoryResolved)
		require.Len(t, notes, 2, "привязка к существующей записи тоже доходит до инициатора")
		require.NotNil(t, notes[1].Message)
		assert.Contains(t, *notes[1].Message, `ООО "Мороз-дубль"`)
		assert.Contains(t, *notes[1].Message, `ООО "Мороз"`)

		// Подтверждение молчит: наименование осталось тем же, что ввёл инициатор.
		approved := seedModerationOrgBy(t, db, `ООО "Тихая"`, models.ModerationPending, plain.ID)
		code, body = post(fmt.Sprintf("/organizations/%d/moderation/approve", approved.ID), ``, token)
		require.Equal(t, http.StatusOK, code, body)
		assert.Len(t, notificationsFor(t, db, plain.ID, services.NotificationTypeDirectoryResolved), 2,
			"подтверждение наименования не меняет - лишнее уведомление было бы шумом")
	})

	// Бейдж «на проверке» в админских справочниках рисуется по этому полю: расширенный
	// список - единственный источник строк таблицы управления организациями и компаниями.
	// Читаем под администратором, а не под разбирающим: список закрыт правом раздела
	// справочников (#2002), а разбирающий работает в заявке, где плашка собирается из
	// самой заявки и этот маршрут ему не нужен.
	t.Run("расширенный список справочника отдаёт статус разбора", func(t *testing.T) {
		adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
		draft := seedModerationOrg(t, db, `ООО "Бейджевая"`, models.ModerationPending)
		compType := models.OrgTypeContractor
		companyDraft := models.Company{Name: `ООО "Бейджевая компания"`, Type: &compType, IsActive: true, ModerationStatus: models.ModerationPending}
		require.NoError(t, db.Create(&companyDraft).Error)

		statuses := func(path string) map[int]interface{} {
			out := map[int]interface{}{}
			for _, row := range testutil.ParseSlice(t, testutil.GET(t, e, path, testutil.AuthHeader(adminToken))) {
				if id, ok := row["id"].(float64); ok {
					out[int(id)] = row["moderation_status"]
				}
			}
			return out
		}

		orgs := statuses("/organizations/with-users-extended")
		assert.Equal(t, models.ModerationPending, orgs[draft.ID])
		assert.Equal(t, models.ModerationApproved, orgs[td.OrgID], "обычная запись приходит проверенной, а не пустой")

		companies := statuses("/companies/with-users-extended")
		assert.Equal(t, models.ModerationPending, companies[companyDraft.ID])
		assert.Equal(t, models.ModerationApproved, companies[td.CompanyID])
	})

	// Список ссылок задан в коде руками: новая таблица с organization_id или company_id,
	// не попавшая в него, после привязки оставит осиротевшие строки, а FK не даст удалить
	// черновик - тест ловит это раньше, чем прод.
	t.Run("список ссылок покрывает все таблицы справочника", func(t *testing.T) {
		for _, spec := range []struct {
			column string
			known  []string
		}{
			{column: "organization_id", known: services.OrganizationRefTables()},
			{column: "company_id", known: services.CompanyRefTables()},
		} {
			var tables []string
			require.NoError(t, db.Raw(`
				SELECT table_name FROM information_schema.columns
				WHERE table_schema = 'public' AND column_name = ?
				ORDER BY table_name`, spec.column).Pluck("table_name", &tables).Error)

			for _, table := range tables {
				assert.Contains(t, spec.known, table,
					"таблица %s ссылается на справочник колонкой %s, но не учтена при слиянии", table, spec.column)
			}
		}
	})
}
