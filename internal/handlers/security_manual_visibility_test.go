package handlers_test

import (
	"context"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/stretchr/testify/require"
)

// Тесты видимости РУЧНЫХ вложений (#1049 S6) во вкладке "Доступные мне" охраны. Ручное вложение -
// сирота без заявки (application_id NULL, is_manual=true, org/company на самом вложении). Мандат
// среза: охрана видит ручные наравне с заявочными - критерий = пересечение мест/target-таблиц, а
// не наличие согласованной заявки. Хелперы secWorld/attachPlace/assignUnloadPlace и пр. - из
// security_visibility_test.go (тот же пакет handlers_test, общий cachedDB, изоляция CleanDB).

// newManualAttachment создаёт вложение-сироту (application_id NULL, is_manual, org = w.orgID) как
// его пишут CreateManualCars/CreateManualEmployees (S3/S4). Места/сотрудники навешиваются отдельно
// теми же attachPlace/attachEmployeeWithTable, что и на заявочные.
func (w secWorld) newManualAttachment(t *testing.T, atype string) int {
	t.Helper()
	statusOne := 1
	att := models.Attachment{
		ApplicationID:  nil,
		AttachmentType: atype,
		IsManual:       true,
		OrganizationID: secPtrInt(w.orgID),
		Status:         &statusOne,
	}
	require.NoError(t, w.db.Create(&att).Error)
	// Ручное вложение без состава охране не показывается (#2450): пропускать по нему
	// некого. Тесты видимости проверяют критерий МЕСТА, поэтому строку кладём сразу -
	// иначе они бы проверяли не место, а пустоту.
	w.fillManualAttachment(t, att.ID, atype)
	return att.ID
}

// fillManualAttachment кладёт во вложение одну строку состава - минимум, при котором
// оно вообще имеет смысл для охраны.
func (w secWorld) fillManualAttachment(t *testing.T, attID int, atype string) {
	t.Helper()
	switch atype {
	case "cars":
		number := "М001АА777"
		require.NoError(t, w.db.Create(&models.Car{AttachmentID: attID, CarNumber: &number}).Error)
	case "people":
		require.NoError(t, w.db.Create(&models.Employee{AttachmentID: secPtrInt(attID)}).Error)
	case "items":
		require.NoError(t, w.db.Create(&models.Item{AttachmentID: attID}).Error)
	}
}

// Ручное cars-вложение видно охраннику по назначенному месту разгрузки, без всякой заявки;
// на чужом месте - скрыто. Заодно: app.* у сироты NULL (метка "добавлено вручную"), а org
// резолвится с самого вложения через COALESCE (не из отсутствующей заявки).
func TestGetAvailableAttachments_ManualCarsVisibleByPlace(t *testing.T) {
	w := setupSecurityWorld(t)
	ctx := context.Background()

	myPlace := w.newUnloadPlace(t, "Склад А", true)
	otherPlace := w.newUnloadPlace(t, "Склад Б", true)
	w.assignUnloadPlace(t, myPlace)

	mineAtt := w.newManualAttachment(t, "cars")
	w.attachPlace(t, mineAtt, myPlace)
	foreignAtt := w.newManualAttachment(t, "cars")
	w.attachPlace(t, foreignAtt, otherPlace)

	rows, total, err := w.svc.GetAvailableAttachmentsForSecurity(ctx, w.guardID, false, services.AvailableAttachmentFilters{}, 1, 50)
	require.NoError(t, err)
	require.EqualValues(t, 1, total, "ручное на моём месте видно без заявки, чужое - нет")
	require.True(t, secContainsAttachment(rows, mineAtt))
	require.False(t, secContainsAttachment(rows, foreignAtt))

	require.Len(t, rows, 1)
	require.Zero(t, rows[0].ApplicationID, "у ручного application_id NULL - метка добавлено вручную")
	require.Nil(t, rows[0].Confirmation, "у ручного нет согласования")
	require.NotNil(t, rows[0].OrganizationName)
	require.Equal(t, "Test Organization", *rows[0].OrganizationName, "org берётся с вложения (COALESCE), а не из NULL-заявки")
}

// Ручное people-вложение видно по назначенной target-таблице прохода; на чужой таблице - скрыто.
func TestGetAvailableAttachments_ManualPeopleVisibleByTable(t *testing.T) {
	w := setupSecurityWorld(t)
	ctx := context.Background()

	myTable := w.newPeopleTable(t, "Проходная 1")
	otherTable := w.newPeopleTable(t, "Проходная 2")
	w.assignTable(t, myTable)

	mineAtt := w.newManualAttachment(t, "people")
	w.attachEmployeeWithTable(t, mineAtt, myTable)
	foreignAtt := w.newManualAttachment(t, "people")
	w.attachEmployeeWithTable(t, foreignAtt, otherTable)

	rows, total, err := w.svc.GetAvailableAttachmentsForSecurity(ctx, w.guardID, false, services.AvailableAttachmentFilters{}, 1, 50)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.True(t, secContainsAttachment(rows, mineAtt), "ручные люди на моей проходной видны без заявки")
	require.False(t, secContainsAttachment(rows, foreignAtt))
}

// Ключевой тест риска: ручное минует confirmation-гейт, но заявочное ПО-ПРЕЖНЕМУ требует
// согласования - ветка is_manual не ослабляет проверку заявочных. Все три на одном месте
// охранника: согласованное + ручное видны, несогласованное - нет.
func TestGetAvailableAttachments_ManualBypassesConfirmationButAppStillGated(t *testing.T) {
	w := setupSecurityWorld(t)
	ctx := context.Background()

	myPlace := w.newUnloadPlace(t, "Склад А", true)
	w.assignUnloadPlace(t, myPlace)

	approvedApp := w.newApp(t, models.ConfirmationApproved)
	approvedAtt := w.newAttachment(t, approvedApp, "cars")
	w.attachPlace(t, approvedAtt, myPlace)

	pendingApp := w.newApp(t, "Согласование")
	pendingAtt := w.newAttachment(t, pendingApp, "cars")
	w.attachPlace(t, pendingAtt, myPlace)

	manualAtt := w.newManualAttachment(t, "cars")
	w.attachPlace(t, manualAtt, myPlace)

	rows, total, err := w.svc.GetAvailableAttachmentsForSecurity(ctx, w.guardID, false, services.AvailableAttachmentFilters{}, 1, 50)
	require.NoError(t, err)
	require.EqualValues(t, 2, total, "согласованное + ручное видны, несогласованное заявочное - нет")
	require.True(t, secContainsAttachment(rows, approvedAtt))
	require.True(t, secContainsAttachment(rows, manualAtt), "ручное без согласования видно")
	require.False(t, secContainsAttachment(rows, pendingAtt), "несогласованное заявочное по-прежнему скрыто")
}

// Ручное без пересечения мест не видно охраннику (паритет с заявочными: доступ несёт место),
// но unrestricted (super/admin) видит его - у него нет place-гейта, а confirmation ручному не нужен.
func TestGetAvailableAttachments_ManualWithoutPlaceHiddenFromGuard(t *testing.T) {
	w := setupSecurityWorld(t)
	ctx := context.Background()

	myPlace := w.newUnloadPlace(t, "Склад А", true)
	w.assignUnloadPlace(t, myPlace)

	// Ручное вложение без единой строки attachment_unload_places - пересечение мест пусто.
	orphanAtt := w.newManualAttachment(t, "cars")

	rows, total, err := w.svc.GetAvailableAttachmentsForSecurity(ctx, w.guardID, false, services.AvailableAttachmentFilters{}, 1, 50)
	require.NoError(t, err)
	require.EqualValues(t, 0, total)
	require.False(t, secContainsAttachment(rows, orphanAtt), "ручное без мест не должно течь охраннику")

	// unrestricted видит ручное без места и без заявки.
	rows, total, err = w.svc.GetAvailableAttachmentsForSecurity(ctx, w.guardID, true, services.AvailableAttachmentFilters{}, 1, 50)
	require.NoError(t, err)
	require.EqualValues(t, 1, total, "super-admin видит ручное без места")
	require.True(t, secContainsAttachment(rows, orphanAtt))
}

// Регрессия list-vs-detail (#1049): вложение, видимое в списке, должно открываться деталью тем же
// набором ролей. Без LEFT JOIN в CanSecurityViewAttachment ручное давало бы 403 при видимом в списке.
func TestCanSecurityViewManualAttachment(t *testing.T) {
	w := setupSecurityWorld(t)
	ctx := context.Background()

	myPlace := w.newUnloadPlace(t, "Склад А", true)
	otherPlace := w.newUnloadPlace(t, "Склад Б", true)
	w.assignUnloadPlace(t, myPlace)

	mineAtt := w.newManualAttachment(t, "cars")
	w.attachPlace(t, mineAtt, myPlace)
	foreignAtt := w.newManualAttachment(t, "cars")
	w.attachPlace(t, foreignAtt, otherPlace)

	can, err := w.svc.CanSecurityViewAttachment(ctx, w.guardID, false, mineAtt)
	require.NoError(t, err)
	require.True(t, can, "ручное на моём месте открывается деталью (не 403)")

	can, err = w.svc.CanSecurityViewAttachment(ctx, w.guardID, false, foreignAtt)
	require.NoError(t, err)
	require.False(t, can, "ручное на чужом месте деталью недоступно")

	can, err = w.svc.CanSecurityViewAttachment(ctx, w.guardID, true, foreignAtt)
	require.NoError(t, err)
	require.True(t, can, "super-admin открывает ручное независимо от места")
}

// Фильтр по организации учитывает org ручного вложения (COALESCE app.*, a.*): свою орг находит,
// чужую - отсекает. Без COALESCE фильтр по org молча прятал бы все ручные (у них app.org NULL).
func TestGetAvailableAttachments_ManualOrgFilter(t *testing.T) {
	w := setupSecurityWorld(t)
	ctx := context.Background()

	myPlace := w.newUnloadPlace(t, "Склад А", true)
	w.assignUnloadPlace(t, myPlace)

	manualAtt := w.newManualAttachment(t, "cars") // org = w.orgID
	w.attachPlace(t, manualAtt, myPlace)

	orgID := w.orgID
	rows, total, err := w.svc.GetAvailableAttachmentsForSecurity(ctx, w.guardID, false, services.AvailableAttachmentFilters{OrganizationID: &orgID}, 1, 50)
	require.NoError(t, err)
	require.EqualValues(t, 1, total, "фильтр по орг ручного находит его по a.organization_id")
	require.True(t, secContainsAttachment(rows, manualAtt))

	otherOrg := w.orgID + 100000
	rows, total, err = w.svc.GetAvailableAttachmentsForSecurity(ctx, w.guardID, false, services.AvailableAttachmentFilters{OrganizationID: &otherOrg}, 1, 50)
	require.NoError(t, err)
	require.EqualValues(t, 0, total, "фильтр по чужой орг отсекает ручное")
	require.False(t, secContainsAttachment(rows, manualAtt))
}

// Фильтр по компании учитывает company ручного вложения (COALESCE app.*, a.*) - симметрично
// org-фильтру: у сироты company_id хранится на вложении, app.* NULL.
func TestGetAvailableAttachments_ManualCompanyFilter(t *testing.T) {
	w := setupSecurityWorld(t)
	ctx := context.Background()

	myPlace := w.newUnloadPlace(t, "Склад А", true)
	w.assignUnloadPlace(t, myPlace)

	company := models.Company{Name: "ООО Ручная", IsActive: true}
	require.NoError(t, w.db.Create(&company).Error)

	statusOne := 1
	att := models.Attachment{
		ApplicationID:  nil,
		AttachmentType: "cars",
		IsManual:       true,
		OrganizationID: secPtrInt(w.orgID),
		CompanyID:      secPtrInt(company.ID),
		Status:         &statusOne,
	}
	require.NoError(t, w.db.Create(&att).Error)
	w.fillManualAttachment(t, att.ID, "cars")
	w.attachPlace(t, att.ID, myPlace)

	rows, total, err := w.svc.GetAvailableAttachmentsForSecurity(ctx, w.guardID, false, services.AvailableAttachmentFilters{CompanyID: &company.ID}, 1, 50)
	require.NoError(t, err)
	require.EqualValues(t, 1, total, "фильтр по компании ручного находит его по a.company_id")
	require.True(t, secContainsAttachment(rows, att.ID))

	otherCompany := company.ID + 100000
	rows, total, err = w.svc.GetAvailableAttachmentsForSecurity(ctx, w.guardID, false, services.AvailableAttachmentFilters{CompanyID: &otherCompany}, 1, 50)
	require.NoError(t, err)
	require.EqualValues(t, 0, total, "фильтр по чужой компании отсекает ручное")
	require.False(t, secContainsAttachment(rows, att.ID))
}

// TestGetAvailableAttachments_EmptyManualHidden - ручное вложение, из которого пропал
// весь состав, охране не показывается (#2450).
//
// Жалоба владельца была именно об этом: в списке пять карточек, две из них открываются
// пустыми - пропускать некого, а место назначено. Проверяем парой: одно вложение с
// машиной, второе без, оба на моём месте разгрузки.
func TestGetAvailableAttachments_EmptyManualHidden(t *testing.T) {
	w := setupSecurityWorld(t)
	ctx := context.Background()

	myPlace := w.newUnloadPlace(t, "Склад А", true)
	w.assignUnloadPlace(t, myPlace)

	filled := w.newManualAttachment(t, "cars")
	w.attachPlace(t, filled, myPlace)

	statusOne := 1
	empty := models.Attachment{
		ApplicationID: nil, AttachmentType: "cars", IsManual: true,
		OrganizationID: secPtrInt(w.orgID), Status: &statusOne,
	}
	require.NoError(t, w.db.Create(&empty).Error)
	w.attachPlace(t, empty.ID, myPlace)

	rows, total, err := w.svc.GetAvailableAttachmentsForSecurity(ctx, w.guardID, false, services.AvailableAttachmentFilters{}, 1, 50)
	require.NoError(t, err)
	require.EqualValues(t, 1, total, "пустое ручное вложение в списке не место")
	require.True(t, secContainsAttachment(rows, filled), "ручное с машиной видно как прежде")
	require.False(t, secContainsAttachment(rows, empty.ID), "ручное без состава скрыто")

	// И по прямой ссылке тоже: иначе карточка открывалась бы пустой в обход списка.
	canView, err := w.svc.CanSecurityViewAttachment(ctx, w.guardID, false, empty.ID)
	require.NoError(t, err)
	require.False(t, canView, "пустое ручное не открывается и по прямой ссылке")
}

// TestGetAvailableAttachments_EmptyApplicationAttachmentStaysVisible - заявочных вложений
// правило не касается. У них есть номер, отправитель и свой жизненный цикл, и скрывать
// их по составу значит менять видимость шире, чем просили.
func TestGetAvailableAttachments_EmptyApplicationAttachmentStaysVisible(t *testing.T) {
	w := setupSecurityWorld(t)
	ctx := context.Background()

	myPlace := w.newUnloadPlace(t, "Склад А", true)
	w.assignUnloadPlace(t, myPlace)

	attID := w.newAttachment(t, w.newApp(t, models.ConfirmationApproved), "cars")
	w.attachPlace(t, attID, myPlace)

	rows, total, err := w.svc.GetAvailableAttachmentsForSecurity(ctx, w.guardID, false, services.AvailableAttachmentFilters{}, 1, 50)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.True(t, secContainsAttachment(rows, attID), "пустое заявочное вложение видно как прежде")
}

// TestGetAvailableAttachments_ManualCarriesOwnFlag - фронту нужен явный признак
// происхождения: по нему карточка подписывается «Автомобили · добавлено вручную»
// вместо бессмысленного «Без названия» (#2450).
func TestGetAvailableAttachments_ManualCarriesOwnFlag(t *testing.T) {
	w := setupSecurityWorld(t)
	ctx := context.Background()

	myPlace := w.newUnloadPlace(t, "Склад А", true)
	w.assignUnloadPlace(t, myPlace)

	manual := w.newManualAttachment(t, "cars")
	w.attachPlace(t, manual, myPlace)
	fromApp := w.newAttachment(t, w.newApp(t, models.ConfirmationApproved), "cars")
	w.attachPlace(t, fromApp, myPlace)

	rows, _, err := w.svc.GetAvailableAttachmentsForSecurity(ctx, w.guardID, false, services.AvailableAttachmentFilters{}, 1, 50)
	require.NoError(t, err)

	flags := map[int]bool{}
	for _, r := range rows {
		flags[r.AttachmentID] = r.IsManual
	}
	require.True(t, flags[manual], "ручное вложение обязано прийти с признаком")
	require.False(t, flags[fromApp], "заявочное - без признака")
}

// TestGetAttachmentCars_ManualAttachmentShowsCars - состав ручного вложения «Автомобили»
// обязан открываться (#2453).
//
// Соединение с заявкой было внутренним, а у ручного вложения заявки нет - выборка
// отсекала все строки разом, и карточка открывалась пустой при любом числе машин.
// Именно с этого («внутри просто пусто») начался разбор жалобы владельца.
func TestGetAttachmentCars_ManualAttachmentShowsCars(t *testing.T) {
	w := setupSecurityWorld(t)
	ctx := context.Background()

	attID := w.newManualAttachment(t, "cars")

	cars, err := w.svc.GetAttachmentCars(ctx, attID, services.SupplementScopeAdmitted)
	require.NoError(t, err)
	require.Len(t, cars, 1, "машина ручного вложения обязана быть видна в карточке")
	require.NotNil(t, cars[0].Organization)
	require.Equal(t, "Test Organization", *cars[0].Organization,
		"организация у сироты берётся с самого вложения, а не из отсутствующей заявки")
}

// TestGetAttachmentCars_ApplicationAttachmentUnchanged - заявочные вложения читаются
// как прежде: внешнее соединение не должно было ничего им поменять.
func TestGetAttachmentCars_ApplicationAttachmentUnchanged(t *testing.T) {
	w := setupSecurityWorld(t)
	ctx := context.Background()

	attID := w.newAttachment(t, w.newApp(t, models.ConfirmationApproved), "cars")
	number := "В002ВВ777"
	require.NoError(t, w.db.Create(&models.Car{AttachmentID: attID, CarNumber: &number}).Error)

	cars, err := w.svc.GetAttachmentCars(ctx, attID, services.SupplementScopeAdmitted)
	require.NoError(t, err)
	require.Len(t, cars, 1)
	require.Equal(t, number, cars[0].CarNumber)
	require.NotNil(t, cars[0].Organization)
}

// TestGetAttachmentEmployees_ManualAttachmentHasOrganization - у людей соединение и так
// было внешним, но организация читалась только из заявки: у сироты выходила пустой.
func TestGetAttachmentEmployees_ManualAttachmentHasOrganization(t *testing.T) {
	w := setupSecurityWorld(t)
	ctx := context.Background()

	attID := w.newManualAttachment(t, "people")

	emps, err := w.svc.GetAttachmentEmployees(ctx, attID, services.SupplementScopeAdmitted)
	require.NoError(t, err)
	require.Len(t, emps, 1)
	require.NotNil(t, emps[0].Organization)
	require.Equal(t, "Test Organization", *emps[0].Organization)
}
