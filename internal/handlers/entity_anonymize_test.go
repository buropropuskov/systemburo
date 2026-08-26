package handlers_test

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"systemburo/internal/crypto"
	"systemburo/internal/entityarchive"
	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Срез 5 entity-archive: server entity anonymize. DB-тест лежит в internal/handlers, а
// не в entityarchive - см. комментарий в entity_retire_test.go (второй тестовый бинарь
// с базой даёт гонку миграций).

// anonymizeFixture - организация с вложением и по одной строке в каждой из трёх PII-
// таблиц сотрудников (employees/unique_employees/application_employees), один
// пользователь организации и одна заявка - у всех заполнены ФИО, документы и контакты.
// Инициатор заявки (InitiatorName/ContactPhone) намеренно ДРУГОЙ человек, чем
// отправитель (f.user) - ровно та ситуация, которую описывает комментарий модели
// Application и которую пропустила первая версия среза (дефект, найденный ревью).
// Вложение сделано "ручным" (organization_id прямо на attachment, без заявки) - для
// теста полей путь создания значения не имеет, а без заявки фикстура короче.
type anonymizeFixture struct {
	org         models.Organization
	attachment  models.Attachment
	employee    models.Employee
	unique      models.UniqueEmployee
	appEmp      models.ApplicationEmployee
	user        models.User
	application models.Application
}

func setupAnonymizeFixture(t *testing.T, db *gorm.DB, orgName string) anonymizeFixture {
	t.Helper()

	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(t, err)
	crypto.SetGlobalKey(key)
	t.Cleanup(func() { crypto.SetGlobalKey(nil) })

	last, first, middle := "Иванов", "Иван", "Иванович"
	passport, patent, other := "4500 123456", "77 1234567", "иное основание пребывания"

	var f anonymizeFixture
	f.org = models.Organization{Name: orgName}
	require.NoError(t, db.Create(&f.org).Error)

	f.attachment = models.Attachment{AttachmentType: "people", OrganizationID: &f.org.ID, IsManual: true}
	require.NoError(t, db.Create(&f.attachment).Error)

	f.employee = models.Employee{
		AttachmentID: &f.attachment.ID,
		LastName:     &last, FirstName: &first, MiddleName: &middle,
		PassportSeriesNumber: &passport, PatentNumber: &patent, OtherPermission: &other,
	}
	require.NoError(t, db.Create(&f.employee).Error)

	f.unique = models.UniqueEmployee{
		OrganizationID: &f.org.ID,
		LastName:       &last, FirstName: &first, MiddleName: &middle,
		PassportSeriesNumber: &passport, PatentNumber: &patent, OtherPermission: &other,
	}
	require.NoError(t, db.Create(&f.unique).Error)

	f.appEmp = models.ApplicationEmployee{
		AttachmentID: f.attachment.ID,
		LastName:     &last, FirstName: &first, MiddleName: &middle,
		PassportSeriesNumber: &passport, PatentNumber: &patent, OtherPermission: &other,
	}
	require.NoError(t, db.Create(&f.appEmp).Error)

	email, phone := "ivanov@example.com", "+79991234567"
	f.user = models.User{
		Username: fmt.Sprintf("anon_user_%s", orgName), OrganizationID: &f.org.ID,
		LastName: &last, FirstName: &first, MiddleName: &middle, Email: &email, Phone: &phone,
	}
	require.NoError(t, db.Create(&f.user).Error)

	initiator, initiatorPhone := "Петров Пётр Петрович", "+79997654321"
	f.application = models.Application{
		OrganizationID: f.org.ID, SenderUserID: f.user.ID,
		InitiatorName: &initiator, ContactPhone: &initiatorPhone,
	}
	require.NoError(t, db.Create(&f.application).Error)

	return f
}

func loadEmployee(t *testing.T, db *gorm.DB, id int) models.Employee {
	t.Helper()
	var e models.Employee
	require.NoError(t, db.First(&e, id).Error)
	return e
}

func loadUniqueEmployee(t *testing.T, db *gorm.DB, id int) models.UniqueEmployee {
	t.Helper()
	var e models.UniqueEmployee
	require.NoError(t, db.First(&e, id).Error)
	return e
}

func loadApplicationEmployee(t *testing.T, db *gorm.DB, id int) models.ApplicationEmployee {
	t.Helper()
	var e models.ApplicationEmployee
	require.NoError(t, db.First(&e, id).Error)
	return e
}

func loadUser(t *testing.T, db *gorm.DB, id int) models.User {
	t.Helper()
	var u models.User
	require.NoError(t, db.First(&u, id).Error)
	return u
}

func loadApplication(t *testing.T, db *gorm.DB, id int) models.Application {
	t.Helper()
	var a models.Application
	require.NoError(t, db.First(&a, id).Error)
	return a
}

// tableRows находит счётчик строк по имени таблицы в результате Anonymize.
func tableRows(t *testing.T, res entityarchive.AnonymizeResult, table string) int {
	t.Helper()
	for _, tr := range res.Tables {
		if tr.Table == table {
			return tr.Rows
		}
	}
	t.Fatalf("в результате нет таблицы %s", table)
	return -1
}

// TestEntityAnonymize_DryRunChangesNothing: без -apply команда только считает - ни одна
// из четырёх таблиц не должна измениться, и след в audit_log не появляется.
func TestEntityAnonymize_DryRunChangesNothing(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	f := setupAnonymizeFixture(t, db, "dry-run")
	rec := services.NewAuditRecorder(db)
	ctx := context.Background()

	dry, err := entityarchive.Anonymize(ctx, db, rec, entityarchive.TypeOrganization, f.org.ID, nil, false)
	require.NoError(t, err)
	assert.Equal(t, 1, tableRows(t, dry, "employees"))
	assert.Equal(t, 1, tableRows(t, dry, "unique_employees"))
	assert.Equal(t, 1, tableRows(t, dry, "application_employees"))
	assert.Equal(t, 1, tableRows(t, dry, "applications"))
	assert.Equal(t, 1, tableRows(t, dry, "users"))
	assert.NotEmpty(t, dry.Warnings, "dry-run обязан показать предупреждение про application_files")

	emp := loadEmployee(t, db, f.employee.ID)
	assert.NotNil(t, emp.LastName, "dry-run не должен был менять базу")

	app := loadApplication(t, db, f.application.ID)
	assert.NotNil(t, app.InitiatorName, "dry-run не должен был менять заявку")

	var count int64
	require.NoError(t, db.Model(&models.AuditLog{}).
		Where("entity_type = ? AND entity_id = ?", models.AuditEntityOrganization, f.org.ID).
		Count(&count).Error)
	assert.Zero(t, count, "dry-run не пишет в audit_log")
}

// TestEntityAnonymize_ClearsValuesAndHMACTogether - ключевой сторож среза: значение
// закрытого поля (паспорт, патент) и его HMAC-отпечаток обязаны обнуляться ВМЕСТЕ.
// Отпечаток детерминирован, и оставшийся отпечаток при стёртом значении позволил бы
// проверить гипотезу "это документ такого-то" - затирание значения без отпечатка
// обезличиванием не является. Проверено сломом: убрал обнуление HMAC-полей в
// anonymizeEmployees/anonymizeUniqueEmployees/anonymizeApplicationEmployees -
// TestEntityAnonymize_ClearsValuesAndHMACTogether покраснел на трёх assert-ах отпечатка
// (PassportSeriesNumberHMAC/PatentNumberHMAC не nil), вернул обнуление обратно - тест
// снова зелёный.
func TestEntityAnonymize_ClearsValuesAndHMACTogether(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	f := setupAnonymizeFixture(t, db, "clears-fields")
	rec := services.NewAuditRecorder(db)
	ctx := context.Background()

	res, err := entityarchive.Anonymize(ctx, db, rec, entityarchive.TypeOrganization, f.org.ID, nil, true)
	require.NoError(t, err)
	assert.Equal(t, 1, tableRows(t, res, "employees"))
	assert.Equal(t, 1, tableRows(t, res, "unique_employees"))
	assert.Equal(t, 1, tableRows(t, res, "application_employees"))
	assert.Equal(t, 1, tableRows(t, res, "applications"))
	assert.Equal(t, 1, tableRows(t, res, "users"))

	emp := loadEmployee(t, db, f.employee.ID)
	assert.Nil(t, emp.LastName)
	assert.Nil(t, emp.FirstName)
	assert.Nil(t, emp.MiddleName)
	assert.Nil(t, emp.PassportSeriesNumber)
	assert.Nil(t, emp.PatentNumber)
	assert.Nil(t, emp.PassportSeriesNumberHMAC, "отпечаток паспорта должен быть стёрт вместе со значением")
	assert.Nil(t, emp.PatentNumberHMAC, "отпечаток патента должен быть стёрт вместе со значением")
	assert.Nil(t, emp.OtherPermission)

	uniq := loadUniqueEmployee(t, db, f.unique.ID)
	assert.Nil(t, uniq.LastName)
	assert.Nil(t, uniq.PassportSeriesNumber)
	assert.Nil(t, uniq.PassportSeriesNumberHMAC, "unique_employees: отпечаток паспорта должен уйти вместе со значением")
	assert.Nil(t, uniq.PatentNumberHMAC, "unique_employees: отпечаток патента должен уйти вместе со значением")

	appEmp := loadApplicationEmployee(t, db, f.appEmp.ID)
	assert.Nil(t, appEmp.LastName)
	assert.Nil(t, appEmp.PassportSeriesNumber)
	assert.Nil(t, appEmp.PassportSeriesNumberHMAC, "application_employees: отпечаток паспорта должен уйти вместе со значением")
	assert.Nil(t, appEmp.PatentNumberHMAC, "application_employees: отпечаток патента должен уйти вместе со значением")

	user := loadUser(t, db, f.user.ID)
	assert.Nil(t, user.LastName)
	assert.Nil(t, user.FirstName)
	assert.Nil(t, user.MiddleName)
	assert.Nil(t, user.Email)
	assert.Nil(t, user.Phone)
	assert.Equal(t, fmt.Sprintf("deleted_%d", f.user.ID), user.Username, "username заменён на псевдоним по id")

	app := loadApplication(t, db, f.application.ID)
	assert.Nil(t, app.InitiatorName, "инициатор заявки обязан быть стёрт - дефект, найденный ревью")
	assert.Nil(t, app.ContactPhone, "телефон инициатора обязан быть стёрт вместе с именем")

	var audit models.AuditLog
	require.NoError(t, db.Where("entity_type = ? AND entity_id = ? AND action = ?",
		models.AuditEntityOrganization, f.org.ID, models.OrganizationActionAnonymized).First(&audit).Error)
	var details struct {
		Tables []struct {
			Table string `json:"table"`
			Rows  int    `json:"rows"`
		} `json:"tables"`
	}
	require.NoError(t, json.Unmarshal(audit.Details, &details))
	require.Len(t, details.Tables, 7)
	assert.NotContains(t, string(audit.Details), "Иванов", "в audit_log не должно быть персональных данных, только счётчики")
	assert.NotContains(t, string(audit.Details), "Петров", "в audit_log не должно быть имени инициатора заявки")

	// Повторный apply на уже обезличенных строках безопасен: значения остаются nil,
	// новая запись в audit_log появляется (это отдельный факт "команда была запущена"),
	// ошибки нет.
	res2, err := entityarchive.Anonymize(ctx, db, rec, entityarchive.TypeOrganization, f.org.ID, nil, true)
	require.NoError(t, err)
	assert.Equal(t, 1, tableRows(t, res2, "employees"))
	emp2 := loadEmployee(t, db, f.employee.ID)
	assert.Nil(t, emp2.LastName)
	assert.Nil(t, emp2.PassportSeriesNumberHMAC)
}

// TestEntityAnonymize_PreservesRelationsAndHistory: обезличивание трогает ТОЛЬКО
// перечисленные поля - связи, история, должности, номера машин, счётчики и даты
// остаются как есть.
func TestEntityAnonymize_PreservesRelationsAndHistory(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	f := setupAnonymizeFixture(t, db, "preserves")

	position := "Монтажник"
	require.NoError(t, db.Model(&models.Employee{}).Where("id = ?", f.employee.ID).
		Update("position", position).Error)

	table := models.SystemTable{Name: "Пост обезличивания"}
	require.NoError(t, db.Create(&table).Error)
	binding := models.EmployeeTargetTable{EmployeeID: f.employee.ID, TableID: table.ID, Source: "manual"}
	require.NoError(t, db.Create(&binding).Error)

	preExisting := models.AuditLog{EntityType: models.AuditEntityEmployee, EntityID: &f.employee.ID, Action: "create"}
	require.NoError(t, db.Create(&preExisting).Error)

	rec := services.NewAuditRecorder(db)
	ctx := context.Background()
	_, err := entityarchive.Anonymize(ctx, db, rec, entityarchive.TypeOrganization, f.org.ID, nil, true)
	require.NoError(t, err)

	// Вложение и сотрудник не удалены, связь между ними цела.
	var attCount int64
	require.NoError(t, db.Model(&models.Attachment{}).Where("id = ?", f.attachment.ID).Count(&attCount).Error)
	assert.EqualValues(t, 1, attCount, "вложение не удалено")

	emp := loadEmployee(t, db, f.employee.ID)
	require.NotNil(t, emp.AttachmentID)
	assert.Equal(t, f.attachment.ID, *emp.AttachmentID, "привязка сотрудника к вложению цела")
	require.NotNil(t, emp.Position)
	assert.Equal(t, position, *emp.Position, "должность не трогается")

	// Привязка к посту цела.
	var bindingCount int64
	require.NoError(t, db.Model(&models.EmployeeTargetTable{}).
		Where("employee_id = ? AND table_id = ?", f.employee.ID, table.ID).Count(&bindingCount).Error)
	assert.EqualValues(t, 1, bindingCount, "привязка сотрудника к посту не тронута")

	// Существовавшая ДО обезличивания запись истории сотрудника не изменилась.
	var afterEntry models.AuditLog
	require.NoError(t, db.First(&afterEntry, preExisting.ID).Error)
	assert.Equal(t, preExisting.Action, afterEntry.Action)
	assert.Equal(t, string(preExisting.Details), string(afterEntry.Details))
}

// TestEntityAnonymize_UsernamePseudonymUnique: несколько пользователей организации
// получают РАЗНЫЕ псевдонимы (по своему id), и unique-индекс username не нарушается.
func TestEntityAnonymize_UsernamePseudonymUnique(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	nameA, nameB := "Петров", "Сидоров"
	userA := models.User{Username: "pseudonym_user_a", OrganizationID: &td.OrgID, LastName: &nameA}
	require.NoError(t, db.Create(&userA).Error)
	userB := models.User{Username: "pseudonym_user_b", OrganizationID: &td.OrgID, LastName: &nameB}
	require.NoError(t, db.Create(&userB).Error)

	rec := services.NewAuditRecorder(db)
	_, err := entityarchive.Anonymize(context.Background(), db, rec, entityarchive.TypeOrganization, td.OrgID, nil, true)
	require.NoError(t, err)

	a := loadUser(t, db, userA.ID)
	b := loadUser(t, db, userB.ID)
	assert.Equal(t, fmt.Sprintf("deleted_%d", userA.ID), a.Username)
	assert.Equal(t, fmt.Sprintf("deleted_%d", userB.ID), b.Username)
	assert.NotEqual(t, a.Username, b.Username, "псевдонимы разных пользователей не совпадают")
}

// TestEntityAnonymize_DoesNotTouchOtherOrganization: обезличивание одной организации не
// задевает сотрудников и пользователей другой.
func TestEntityAnonymize_DoesNotTouchOtherOrganization(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	target := setupAnonymizeFixture(t, db, "target-org")
	other := setupAnonymizeFixture(t, db, "other-org")
	rec := services.NewAuditRecorder(db)

	_, err := entityarchive.Anonymize(context.Background(), db, rec, entityarchive.TypeOrganization, target.org.ID, nil, true)
	require.NoError(t, err)

	targetEmp := loadEmployee(t, db, target.employee.ID)
	assert.Nil(t, targetEmp.LastName, "своя организация обезличена")

	otherEmp := loadEmployee(t, db, other.employee.ID)
	assert.NotNil(t, otherEmp.LastName, "чужой сотрудник не задет")
	assert.NotNil(t, otherEmp.PassportSeriesNumberHMAC, "чужой отпечаток не задет")

	otherUser := loadUser(t, db, other.user.ID)
	assert.NotEqual(t, fmt.Sprintf("deleted_%d", other.user.ID), otherUser.Username, "чужой пользователь не переименован")
	assert.NotNil(t, otherUser.Email, "чужой пользователь не задет")

	otherApp := loadApplication(t, db, other.application.ID)
	assert.NotNil(t, otherApp.InitiatorName, "инициатор чужой заявки не задет")
	assert.NotNil(t, otherApp.ContactPhone, "телефон инициатора чужой заявки не задет")

	var count int64
	require.NoError(t, db.Model(&models.AuditLog{}).
		Where("entity_type = ? AND entity_id = ?", models.AuditEntityOrganization, other.org.ID).
		Count(&count).Error)
	assert.Zero(t, count, "у чужой организации не появилось записи об обезличивании")
}

// TestEntityAnonymize_RejectsMissingOrganization: опечатка в -id обязана получить явный
// отказ и в dry-run, и в apply, и не оставить след в audit_log.
func TestEntityAnonymize_RejectsMissingOrganization(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	rec := services.NewAuditRecorder(db)
	ctx := context.Background()
	const missingID = 999999

	_, err := entityarchive.Anonymize(ctx, db, rec, entityarchive.TypeOrganization, missingID, nil, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "не найдена")

	_, err = entityarchive.Anonymize(ctx, db, rec, entityarchive.TypeOrganization, missingID, nil, true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "не найдена")

	var count int64
	require.NoError(t, db.Model(&models.AuditLog{}).
		Where("entity_type = ? AND entity_id = ?", models.AuditEntityOrganization, missingID).
		Count(&count).Error)
	assert.Zero(t, count)
}

// TestEntityAnonymize_WarnsAboutApplicationFiles - дефект, который легко пропустить:
// обезличивание полей не убирает сканы документов, приложенные к заявкам организации, не
// убирает слепки бланков в файловом архиве (заявка.json - тот же паспорт/патент, ФИО и
// телефон инициатора открытым текстом) и не убирает снимок совпавшей записи чёрного
// списка (matched_value), который текстуально может быть похож на только что обезличенное
// имя. Команда обязана честно назвать все три факта и в dry-run, и в apply - молчание про
// любой из них читалось бы как "персональных данных не осталось".
func TestEntityAnonymize_WarnsAboutApplicationFiles(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	f := setupAnonymizeFixture(t, db, "with-files")
	rec := services.NewAuditRecorder(db)
	ctx := context.Background()

	file := models.ApplicationFile{
		ApplicationID: &f.application.ID, FileName: "Паспорт Иванова.pdf", StoredName: "warn-fixture.bin",
		MimeType: "application/pdf", FileSize: 10, UploadedBy: f.user.ID,
	}
	require.NoError(t, db.Create(&file).Error)

	dry, err := entityarchive.Anonymize(ctx, db, rec, entityarchive.TypeOrganization, f.org.ID, nil, false)
	require.NoError(t, err)
	require.Len(t, dry.Warnings, 3, "обязаны быть все три предупреждения - файлы заявок, слепки бланков, снимок ЧС")
	assert.Contains(t, dry.Warnings[0], "application_files")
	assert.Contains(t, dry.Warnings[0], "1 шт.", "предупреждение обязано назвать реальное число приложенных файлов")
	assert.Contains(t, dry.Warnings[1], "заявка.json")
	assert.Contains(t, dry.Warnings[1], "ARCHIVE_PATH")
	assert.Contains(t, dry.Warnings[2], "matched_value")
	assert.Contains(t, dry.Warnings[2], "application_blacklist_flags")

	res, err := entityarchive.Anonymize(ctx, db, rec, entityarchive.TypeOrganization, f.org.ID, nil, true)
	require.NoError(t, err)
	require.Len(t, res.Warnings, 3)
	assert.Contains(t, res.Warnings[0], "application_files")
	assert.Contains(t, res.Warnings[1], "заявка.json")
	assert.Contains(t, res.Warnings[2], "matched_value")

	// Файл заявки не тронут - обезличивание его не удаляет и не переименовывает.
	var stillThere models.ApplicationFile
	require.NoError(t, db.First(&stillThere, file.ID).Error)
	assert.Equal(t, "Паспорт Иванова.pdf", stillThere.FileName, "имя файла заявки не тронуто")
}

// seedAnonymizeSuperAdmin создаёт супер-администратора организации с заполненными
// персональными полями - тем же приёмом, что seedRetireSuperAdmin в entity_retire_test.go.
func seedAnonymizeSuperAdmin(t *testing.T, db *gorm.DB, username string, orgID int) models.User {
	t.Helper()
	last := "Суперадминов"
	u := models.User{Username: username, OrganizationID: &orgID, IsSuperAdmin: true, LastName: &last}
	require.NoError(t, db.Create(&u).Error)
	return u
}

// TestEntityAnonymize_SkipsSuperAdmin - дефект, найденный ревью: anonymize не должен
// обезличивать супер-администратора организации. Смена username на deleted_<id> означает,
// что войти под прежним логином уже нельзя, - это НЕОБРАТИМАЯ блокировка входа, только
// неочевидная, и организация владельца системы вполне может стать целью среза. Пропуск
// обязан быть виден и в dry-run, и после apply, а токен супер-админа - остаться живым:
// его самого не тронули, значит и его сессию гасить не за что.
func TestEntityAnonymize_SkipsSuperAdmin(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	regularLast := "Обычный"
	regular := models.User{Username: "anon_regular_user", OrganizationID: &td.OrgID, LastName: &regularLast}
	require.NoError(t, db.Create(&regular).Error)
	super := seedAnonymizeSuperAdmin(t, db, "anon_super_admin", td.OrgID)
	superToken := models.RefreshToken{UserID: super.ID, TokenHash: "anon-super-hash", ExpiresAt: time.Now().Add(24 * time.Hour)}
	require.NoError(t, db.Create(&superToken).Error)

	rec := services.NewAuditRecorder(db)
	ctx := context.Background()

	dry, err := entityarchive.Anonymize(ctx, db, rec, entityarchive.TypeOrganization, td.OrgID, nil, false)
	require.NoError(t, err)
	assert.Equal(t, 1, tableRows(t, dry, "users"), "dry-run считает только обычного пользователя")
	assert.Equal(t, []int{super.ID}, dry.SkippedSuperAdmins, "dry-run уже показывает пропуск супер-админа")

	res, err := entityarchive.Anonymize(ctx, db, rec, entityarchive.TypeOrganization, td.OrgID, nil, true)
	require.NoError(t, err)
	assert.Equal(t, 1, tableRows(t, res, "users"))
	assert.Equal(t, []int{super.ID}, res.SkippedSuperAdmins)

	reg := loadUser(t, db, regular.ID)
	assert.Nil(t, reg.LastName, "обычный пользователь организации обезличен")
	assert.Equal(t, fmt.Sprintf("deleted_%d", regular.ID), reg.Username)

	sup := loadUser(t, db, super.ID)
	require.NotNil(t, sup.LastName, "супер-админ не обезличен")
	assert.Equal(t, "Суперадминов", *sup.LastName)
	assert.Equal(t, "anon_super_admin", sup.Username, "у супер-админа логин не меняется")

	var revoked bool
	require.NoError(t, db.Raw("SELECT is_revoked FROM refresh_tokens WHERE id = ?", superToken.ID).Scan(&revoked).Error)
	assert.False(t, revoked, "токен супер-админа не отзывается - его самого не тронули")
}

// TestEntityAnonymize_TreatsNullSuperAdminAsRegularUser - зеркало одноимённого теста в
// entity_retire_test.go: users.is_super_admin в схеме DEFAULT false, но БЕЗ NOT NULL.
// "AND is_super_admin"/"AND NOT is_super_admin" на строке с NULL дают NULL (SQL
// three-valued logic) - без COALESCE такая строка не попала бы ни в обезличенные, ни в
// пропущенные, оставшись с настоящим ФИО, пока команда рапортует полный охват.
func TestEntityAnonymize_TreatsNullSuperAdminAsRegularUser(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	last := "Нуль"
	u := models.User{Username: "anon_null_super_admin", OrganizationID: &td.OrgID, LastName: &last}
	require.NoError(t, db.Create(&u).Error)
	require.NoError(t, db.Exec("UPDATE users SET is_super_admin = NULL WHERE id = ?", u.ID).Error)

	rec := services.NewAuditRecorder(db)
	ctx := context.Background()

	dry, err := entityarchive.Anonymize(ctx, db, rec, entityarchive.TypeOrganization, td.OrgID, nil, false)
	require.NoError(t, err)
	assert.Equal(t, 1, tableRows(t, dry, "users"), "NULL в is_super_admin - обычный пользователь, попадает под обезличивание")
	assert.NotContains(t, dry.SkippedSuperAdmins, u.ID)

	res, err := entityarchive.Anonymize(ctx, db, rec, entityarchive.TypeOrganization, td.OrgID, nil, true)
	require.NoError(t, err)
	assert.NotContains(t, res.SkippedSuperAdmins, u.ID)

	reloaded := loadUser(t, db, u.ID)
	assert.Nil(t, reloaded.LastName, "пользователь с NULL в is_super_admin обезличен как обычный")
}

// TestEntityAnonymize_RevokesActiveRefreshTokens: обезличенный пользователь не может
// войти под прежним логином сразу, но без отдельного отзыва его уже открытая сессия
// дожила бы до истечения собственного TTL - зеркалит user_service.setActive/Retire.
func TestEntityAnonymize_RevokesActiveRefreshTokens(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	last := "Токенов"
	u := models.User{Username: "anon_token_user", OrganizationID: &td.OrgID, LastName: &last}
	require.NoError(t, db.Create(&u).Error)
	token := models.RefreshToken{UserID: u.ID, TokenHash: "anon-token-hash", ExpiresAt: time.Now().Add(24 * time.Hour)}
	require.NoError(t, db.Create(&token).Error)

	rec := services.NewAuditRecorder(db)
	_, err := entityarchive.Anonymize(context.Background(), db, rec, entityarchive.TypeOrganization, td.OrgID, nil, true)
	require.NoError(t, err)

	var revoked bool
	require.NoError(t, db.Raw("SELECT is_revoked FROM refresh_tokens WHERE id = ?", token.ID).Scan(&revoked).Error)
	assert.True(t, revoked, "anonymize отзывает активные refresh-токены обезличенных пользователей")
}

// TestEntityAnonymize_ClearsApplicationInitiatorFields - дефект, найденный ревью:
// заявка несёт ФИО и телефон инициатора отдельно от отправителя (может быть указан
// ДРУГОЙ человек, не сам отправитель), и первая версия среза их не трогала - организация
// выглядела бы обезличенной, а в каждой её заявке оставалось бы читаемое имя и телефон.
// Обезличивание заявки трогает ТОЛЬКО эту пару полей - остальные (сообщение, связь с
// организацией и отправителем) остаются как есть.
func TestEntityAnonymize_ClearsApplicationInitiatorFields(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	f := setupAnonymizeFixture(t, db, "app-fields")

	message := "Пропуск для монтажной бригады"
	require.NoError(t, db.Model(&models.Application{}).Where("id = ?", f.application.ID).
		Update("message", message).Error)

	rec := services.NewAuditRecorder(db)
	_, err := entityarchive.Anonymize(context.Background(), db, rec, entityarchive.TypeOrganization, f.org.ID, nil, true)
	require.NoError(t, err)

	app := loadApplication(t, db, f.application.ID)
	assert.Nil(t, app.InitiatorName, "инициатор заявки стёрт")
	assert.Nil(t, app.ContactPhone, "телефон инициатора стёрт")
	assert.Equal(t, f.org.ID, app.OrganizationID, "связь с организацией цела")
	assert.Equal(t, f.user.ID, app.SenderUserID, "связь с отправителем цела")
	require.NotNil(t, app.Message, "сообщение заявки не трогается")
	assert.Equal(t, message, *app.Message)
}

// TestEntityAnonymize_DoesNotTouchPDAuditLogs: журнал доступа к персональным данным
// (152-ФЗ) вне графа организации - это уже гарантировано тем, что pd_audit_logs не входит
// в organizationNodes(), но для необратимой операции лишняя строка проверки не в тягость.
func TestEntityAnonymize_DoesNotTouchPDAuditLogs(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	f := setupAnonymizeFixture(t, db, "pd-audit")

	entry := models.PDAuditLog{
		UserID: &f.user.ID, Username: f.user.Username, Action: "view",
		Resource: "employee", ResourceID: &f.employee.ID,
		IPAddress: "127.0.0.1", Method: "GET", Path: "/api/employees/" + fmt.Sprint(f.employee.ID), StatusCode: 200,
	}
	require.NoError(t, db.Create(&entry).Error)

	rec := services.NewAuditRecorder(db)
	_, err := entityarchive.Anonymize(context.Background(), db, rec, entityarchive.TypeOrganization, f.org.ID, nil, true)
	require.NoError(t, err)

	var after models.PDAuditLog
	require.NoError(t, db.First(&after, entry.ID).Error)
	assert.Equal(t, entry.Username, after.Username, "pd_audit_logs не трогается anonymize - имя в записи то же, что было до")
	assert.Equal(t, entry.Path, after.Path)
	assert.Equal(t, entry.Resource, after.Resource)
}

// TestEntityAnonymize_ClearsBlacklistElementNormalizedForEmployeeOnly - дефект, найденный
// ревью во второй итерации: element_normalized в application_blacklist_flags/overrides -
// нормализованная форма САМОГО элемента заявки (комментарий модели), то есть у
// element_type=employee это ФИО СВОЕГО сотрудника - такой же идентификатор человека, как
// имя в employees, и обязан затираться. У element_type=car это номер машины - трогать
// нельзя (задача на срез явно исключает номера машин). matched_value/matched_reason/
// comment - снимок ЗАПИСИ чёрного списка (данные ЧУЖОГО человека, занесённого в ЧС не
// этой организацией) - остаются как есть при обезличивании ЭТОЙ организации.
func TestEntityAnonymize_ClearsBlacklistElementNormalizedForEmployeeOnly(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	f := setupAnonymizeFixture(t, db, "blacklist")

	flagEmployee := models.ApplicationBlacklistFlag{
		ApplicationID: f.application.ID, ElementType: models.BlacklistElementEmployee, ElementID: f.employee.ID,
		ElementNormalized: "иванов иван иванович", MatchedBlacklistID: 1, MatchedValue: "иванов и и",
		MatchedReason: "похожее ФИО", Similarity: 0.8,
	}
	require.NoError(t, db.Create(&flagEmployee).Error)

	flagCar := models.ApplicationBlacklistFlag{
		ApplicationID: f.application.ID, ElementType: models.BlacklistElementCar, ElementID: 999999,
		ElementNormalized: "а123бв777", MatchedBlacklistID: 2, MatchedValue: "а123бв177",
		MatchedReason: "похожий номер", Similarity: 0.75,
	}
	require.NoError(t, db.Create(&flagCar).Error)

	override := models.ApplicationBlacklistOverride{
		FlagID: flagEmployee.ID, ApplicationID: f.application.ID,
		ElementType: models.BlacklistElementEmployee, ElementID: f.employee.ID,
		ElementNormalized: "иванов иван иванович", MatchedBlacklistID: 1, MatchedValue: "иванов и и",
		OverriddenByUserID: f.user.ID, Comment: "проверено вручную, пропустить",
	}
	require.NoError(t, db.Create(&override).Error)

	rec := services.NewAuditRecorder(db)
	ctx := context.Background()

	dry, err := entityarchive.Anonymize(ctx, db, rec, entityarchive.TypeOrganization, f.org.ID, nil, false)
	require.NoError(t, err)
	assert.Equal(t, 1, tableRows(t, dry, "application_blacklist_flags"),
		"dry-run считает только employee-строку, не car")
	assert.Equal(t, 1, tableRows(t, dry, "application_blacklist_overrides"))

	res, err := entityarchive.Anonymize(ctx, db, rec, entityarchive.TypeOrganization, f.org.ID, nil, true)
	require.NoError(t, err)
	assert.Equal(t, 1, tableRows(t, res, "application_blacklist_flags"))
	assert.Equal(t, 1, tableRows(t, res, "application_blacklist_overrides"))

	var afterFlagEmployee models.ApplicationBlacklistFlag
	require.NoError(t, db.First(&afterFlagEmployee, flagEmployee.ID).Error)
	assert.Empty(t, afterFlagEmployee.ElementNormalized, "ФИО своего сотрудника в flags стёрто")
	assert.Equal(t, "иванов и и", afterFlagEmployee.MatchedValue, "чужая запись ЧС не трогается")
	assert.Equal(t, "похожее ФИО", afterFlagEmployee.MatchedReason, "текст причины совпадения не трогается")

	var afterFlagCar models.ApplicationBlacklistFlag
	require.NoError(t, db.First(&afterFlagCar, flagCar.ID).Error)
	assert.Equal(t, "а123бв777", afterFlagCar.ElementNormalized, "номер машины не трогается - element_type=car")

	var afterOverride models.ApplicationBlacklistOverride
	require.NoError(t, db.First(&afterOverride, override.ID).Error)
	assert.Empty(t, afterOverride.ElementNormalized, "ФИО своего сотрудника в overrides стёрто")
	assert.Equal(t, "иванов и и", afterOverride.MatchedValue, "чужая запись ЧС не трогается")
	assert.Equal(t, "проверено вручную, пропустить", afterOverride.Comment, "комментарий решения не трогается")
}
