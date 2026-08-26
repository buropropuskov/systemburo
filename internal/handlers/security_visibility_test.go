package handlers_test

import (
	"context"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Тесты фильтра "Доступные мне" (#706, срез BE-S3). DB-backed: реальный Postgres через testutil,
// сервис конструируется напрямую (методы фильтра используют только db, прочие зависимости nil).
// Живут в пакете handlers_test - единственном DB-использующем тест-бинаре проекта: при `go test
// ./...` пакеты гоняются параллельно, и второй бинарь с теми же CleanDB/Seed по общей тест-БД
// ловит FK-гонки. Каждый тест изолируется CleanDB; параллелизма внутри нет - общий cachedDB.

func secPtrInt(i int) *int { return &i }

type secWorld struct {
	db       *gorm.DB
	svc      services.ApplicationService
	orgID    int
	senderID int
	guardID  int
}

func setupSecurityWorld(t *testing.T) secWorld {
	t.Helper()
	_, db, cleanup := testutil.SetupTestApp(t)
	t.Cleanup(cleanup)
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	secTypeID := secUserTypeIDByCode(t, db, "security")
	userTypeID := secUserTypeIDByCode(t, db, "user")

	guard := models.User{Username: "guard1", Password: "x", TypeID: secTypeID, OrganizationID: secPtrInt(td.OrgID)}
	require.NoError(t, db.Create(&guard).Error)
	sender := models.User{Username: "sender1", Password: "x", TypeID: userTypeID, OrganizationID: secPtrInt(td.OrgID)}
	require.NoError(t, db.Create(&sender).Error)

	svc := services.NewApplicationService(db, nil, nil, nil, nil, services.NewAuditRecorder(db))
	return secWorld{db: db, svc: svc, orgID: td.OrgID, senderID: sender.ID, guardID: guard.ID}
}

func secUserTypeIDByCode(t *testing.T, db *gorm.DB, code string) int {
	t.Helper()
	var id int
	require.NoError(t, db.Table("user_types").Where("code = ?", code).Select("id").Scan(&id).Error)
	require.NotZero(t, id, "user_type %q not seeded", code)
	return id
}

// newApp создаёт согласованную заявку в активном допуске (status='В работе') - именно такую видит
// охрана во вкладке "Доступные мне" по умолчанию. Для нестандартного статуса - newAppWithStatus.
func (w secWorld) newApp(t *testing.T, confirmation string) int {
	t.Helper()
	now := time.Now()
	conf := confirmation
	st := models.StatusInWork
	app := models.Application{
		OrganizationID:  w.orgID,
		SenderUserID:    w.senderID,
		Confirmation:    &conf,
		Status:          &st,
		SendingDatetime: &now,
	}
	require.NoError(t, w.db.Create(&app).Error)
	return app.ID
}

func (w secWorld) newAttachment(t *testing.T, appID int, atype string) int {
	t.Helper()
	att := models.Attachment{ApplicationID: &appID, AttachmentType: atype}
	require.NoError(t, w.db.Create(&att).Error)
	return att.ID
}

func (w secWorld) newAppWithStatus(t *testing.T, confirmation, status string) int {
	t.Helper()
	now := time.Now()
	conf, st := confirmation, status
	app := models.Application{
		OrganizationID:  w.orgID,
		SenderUserID:    w.senderID,
		Confirmation:    &conf,
		Status:          &st,
		SendingDatetime: &now,
	}
	require.NoError(t, w.db.Create(&app).Error)
	return app.ID
}

func (w secWorld) newUnloadPlace(t *testing.T, name string, active bool) int {
	t.Helper()
	p := models.UnloadPlace{Name: name, IsActive: active}
	require.NoError(t, w.db.Create(&p).Error)
	return p.ID
}

func (w secWorld) newPeopleTable(t *testing.T, name string) int {
	t.Helper()
	st := models.SystemTable{Name: name, TableType: "people", IsActive: true}
	require.NoError(t, w.db.Create(&st).Error)
	return st.ID
}

func (w secWorld) newPeopleTableWithDisplay(t *testing.T, name, display string) int {
	t.Helper()
	d := display
	st := models.SystemTable{Name: name, DisplayName: &d, TableType: "people", IsActive: true}
	require.NoError(t, w.db.Create(&st).Error)
	return st.ID
}

func (w secWorld) attachPlace(t *testing.T, attID, placeID int) {
	t.Helper()
	require.NoError(t, w.db.Create(&models.AttachmentUnloadPlace{AttachmentID: attID, UnloadPlaceID: placeID}).Error)
}

func (w secWorld) attachEmployeeWithTable(t *testing.T, attID, tableID int) {
	t.Helper()
	emp := models.Employee{AttachmentID: secPtrInt(attID)}
	require.NoError(t, w.db.Create(&emp).Error)
	require.NoError(t, w.db.Create(&models.EmployeeTargetTable{EmployeeID: emp.ID, TableID: tableID}).Error)
}

func (w secWorld) assignUnloadPlace(t *testing.T, placeID int) {
	t.Helper()
	require.NoError(t, w.db.Create(&models.SecurityUserUnloadPlace{UserID: w.guardID, UnloadPlaceID: placeID}).Error)
}

func (w secWorld) assignTable(t *testing.T, tableID int) {
	t.Helper()
	require.NoError(t, w.db.Create(&models.SecurityUserTable{UserID: w.guardID, TableID: tableID}).Error)
}

func secContainsAttachment(rows []services.AvailableAttachment, attID int) bool {
	for _, r := range rows {
		if r.AttachmentID == attID {
			return true
		}
	}
	return false
}

func TestIsSecurityUser(t *testing.T) {
	w := setupSecurityWorld(t)
	ctx := context.Background()

	isSec, err := w.svc.IsSecurityUser(ctx, w.guardID)
	require.NoError(t, err)
	require.True(t, isSec, "guard must be recognised as security")

	isSec, err = w.svc.IsSecurityUser(ctx, w.senderID)
	require.NoError(t, err)
	require.False(t, isSec, "regular user must not be security")

	isSec, err = w.svc.IsSecurityUser(ctx, 999999)
	require.NoError(t, err)
	require.False(t, isSec, "non-existent user must not be security")
}

func TestGetAvailableAttachments_MatchByUnloadPlace(t *testing.T) {
	w := setupSecurityWorld(t)
	ctx := context.Background()

	myPlace := w.newUnloadPlace(t, "Склад А", true)
	otherPlace := w.newUnloadPlace(t, "Склад Б", true)
	w.assignUnloadPlace(t, myPlace)

	app := w.newApp(t, models.ConfirmationApproved)
	carsAtt := w.newAttachment(t, app, "cars")
	w.attachPlace(t, carsAtt, myPlace)
	itemsAtt := w.newAttachment(t, app, "items")
	w.attachPlace(t, itemsAtt, myPlace)
	foreignAtt := w.newAttachment(t, app, "cars")
	w.attachPlace(t, foreignAtt, otherPlace)

	rows, total, err := w.svc.GetAvailableAttachmentsForSecurity(ctx, w.guardID, false, services.AvailableAttachmentFilters{}, 1, 50)
	require.NoError(t, err)
	require.EqualValues(t, 2, total)
	require.True(t, secContainsAttachment(rows, carsAtt), "cars at guard place visible")
	require.True(t, secContainsAttachment(rows, itemsAtt), "items at guard place visible")
	require.False(t, secContainsAttachment(rows, foreignAtt), "attachment at foreign place hidden")
}

func TestGetAvailableAttachments_PeopleMatchByTable(t *testing.T) {
	w := setupSecurityWorld(t)
	ctx := context.Background()

	myTable := w.newPeopleTable(t, "Проходная 1")
	otherTable := w.newPeopleTable(t, "Проходная 2")
	w.assignTable(t, myTable)

	app := w.newApp(t, models.ConfirmationApproved)
	peopleAtt := w.newAttachment(t, app, "people")
	w.attachEmployeeWithTable(t, peopleAtt, myTable)
	foreignAtt := w.newAttachment(t, app, "people")
	w.attachEmployeeWithTable(t, foreignAtt, otherTable)

	rows, total, err := w.svc.GetAvailableAttachmentsForSecurity(ctx, w.guardID, false, services.AvailableAttachmentFilters{}, 1, 50)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.True(t, secContainsAttachment(rows, peopleAtt), "people at guard passage visible")
	require.False(t, secContainsAttachment(rows, foreignAtt), "people at foreign passage hidden")
}

func TestGetAvailableAttachments_ApprovedGate(t *testing.T) {
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

	rows, total, err := w.svc.GetAvailableAttachmentsForSecurity(ctx, w.guardID, false, services.AvailableAttachmentFilters{}, 1, 50)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.True(t, secContainsAttachment(rows, approvedAtt))
	require.False(t, secContainsAttachment(rows, pendingAtt), "non-approved application hidden")
}

// TestGetAvailableAttachments_WithdrawnHidden закрывает дыру #951: отозванная заявка
// остаётся confirmation='Согласовано', но должна пропасть из "Доступные мне" охраны -
// иначе обещание "охрана не пропустит" ложно для уже согласованных заявок.
func TestGetAvailableAttachments_WithdrawnHidden(t *testing.T) {
	w := setupSecurityWorld(t)
	ctx := context.Background()

	myPlace := w.newUnloadPlace(t, "Склад В", true)
	w.assignUnloadPlace(t, myPlace)

	// Контроль: согласованная активная заявка видна охране.
	activeApp := w.newApp(t, models.ConfirmationApproved)
	activeAtt := w.newAttachment(t, activeApp, "cars")
	w.attachPlace(t, activeAtt, myPlace)

	// Отозванная: confirmation='Согласовано', но статус "Отозвана" - скрыта.
	withdrawnApp := w.newAppWithStatus(t, models.ConfirmationApproved, models.StatusWithdrawn)
	withdrawnAtt := w.newAttachment(t, withdrawnApp, "cars")
	w.attachPlace(t, withdrawnAtt, myPlace)

	rows, total, err := w.svc.GetAvailableAttachmentsForSecurity(ctx, w.guardID, false, services.AvailableAttachmentFilters{}, 1, 50)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.True(t, secContainsAttachment(rows, activeAtt))
	require.False(t, secContainsAttachment(rows, withdrawnAtt), "отозванная заявка скрыта от охраны несмотря на confirmation=Согласовано")
}

// TestGetAvailableAttachments_RefusedAndProcessingHidden закрывает ту же дыру, что #951, но для
// других не-активных статусов: принимающий отказывает в заявке (reject -> status='Отказано') либо
// возвращает из работы (revoke_from_work -> status='В обработке'), при этом confirmation остаётся
// 'Согласовано'. Ни то, ни другое не является активным допуском - вложение должно пропасть из
// "Доступные мне" охраны, иначе она пропустит по отменённому пропуску.
func TestGetAvailableAttachments_RefusedAndProcessingHidden(t *testing.T) {
	w := setupSecurityWorld(t)
	ctx := context.Background()

	myPlace := w.newUnloadPlace(t, "Склад Г", true)
	w.assignUnloadPlace(t, myPlace)

	// Контроль: согласованная заявка в работе видна охране.
	activeApp := w.newApp(t, models.ConfirmationApproved)
	activeAtt := w.newAttachment(t, activeApp, "cars")
	w.attachPlace(t, activeAtt, myPlace)

	// Отказанная: confirmation='Согласовано', но статус "Отказано" - скрыта.
	refusedApp := w.newAppWithStatus(t, models.ConfirmationApproved, models.StatusRefused)
	refusedAtt := w.newAttachment(t, refusedApp, "cars")
	w.attachPlace(t, refusedAtt, myPlace)

	// Возвращённая из работы: confirmation='Согласовано', статус "В обработке" - не активный допуск, скрыта.
	processingApp := w.newAppWithStatus(t, models.ConfirmationApproved, models.StatusProcessing)
	processingAtt := w.newAttachment(t, processingApp, "cars")
	w.attachPlace(t, processingAtt, myPlace)

	rows, total, err := w.svc.GetAvailableAttachmentsForSecurity(ctx, w.guardID, false, services.AvailableAttachmentFilters{}, 1, 50)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.True(t, secContainsAttachment(rows, activeAtt))
	require.False(t, secContainsAttachment(rows, refusedAtt), "отказанная заявка скрыта от охраны несмотря на confirmation=Согласовано")
	require.False(t, secContainsAttachment(rows, processingAtt), "заявка 'В обработке' скрыта от охраны - не активный допуск")
}

func TestGetAvailableAttachments_SuperAdminSeesAllApproved(t *testing.T) {
	w := setupSecurityWorld(t)
	ctx := context.Background()

	// Место никому не назначено - охранник бы не увидел, супер-админ видит.
	place := w.newUnloadPlace(t, "Склад без назначения", true)

	approvedApp := w.newApp(t, models.ConfirmationApproved)
	approvedAtt := w.newAttachment(t, approvedApp, "cars")
	w.attachPlace(t, approvedAtt, place)

	pendingApp := w.newApp(t, "Согласование")
	pendingAtt := w.newAttachment(t, pendingApp, "cars")
	w.attachPlace(t, pendingAtt, place)

	rows, total, err := w.svc.GetAvailableAttachmentsForSecurity(ctx, w.guardID, true, services.AvailableAttachmentFilters{}, 1, 50)
	require.NoError(t, err)
	require.EqualValues(t, 1, total, "super-admin sees approved regardless of place, still approved-gated")
	require.True(t, secContainsAttachment(rows, approvedAtt))
	require.False(t, secContainsAttachment(rows, pendingAtt), "approved-gate applies to super-admin too")
}

// TestGetAvailableAttachments_UnrestrictedRefusedHidden — статус-гейт применяется и к unrestricted
// (super/admin/носитель page.available, #976): именно этой веткой отказанная заявка (confirmation
// остаётся 'Согласовано') протекала в допуск. Место не назначено - охрана бы не увидела, но
// unrestricted снимает только place-фильтр, статус-гейт остаётся.
func TestGetAvailableAttachments_UnrestrictedRefusedHidden(t *testing.T) {
	w := setupSecurityWorld(t)
	ctx := context.Background()

	place := w.newUnloadPlace(t, "Склад без назначения", true)

	activeApp := w.newAppWithStatus(t, models.ConfirmationApproved, models.StatusInWork)
	activeAtt := w.newAttachment(t, activeApp, "cars")
	w.attachPlace(t, activeAtt, place)

	refusedApp := w.newAppWithStatus(t, models.ConfirmationApproved, models.StatusRefused)
	refusedAtt := w.newAttachment(t, refusedApp, "cars")
	w.attachPlace(t, refusedAtt, place)

	rows, total, err := w.svc.GetAvailableAttachmentsForSecurity(ctx, w.guardID, true, services.AvailableAttachmentFilters{}, 1, 50)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.True(t, secContainsAttachment(rows, activeAtt))
	require.False(t, secContainsAttachment(rows, refusedAtt), "статус-гейт применяется и к unrestricted: отказанная скрыта")
}

func TestGetAvailableAttachments_NoPlacesEmpty(t *testing.T) {
	w := setupSecurityWorld(t)
	ctx := context.Background()

	// Охраннику не назначено ни одного места, но согласованное вложение существует.
	place := w.newUnloadPlace(t, "Склад А", true)
	app := w.newApp(t, models.ConfirmationApproved)
	att := w.newAttachment(t, app, "cars")
	w.attachPlace(t, att, place)

	rows, total, err := w.svc.GetAvailableAttachmentsForSecurity(ctx, w.guardID, false, services.AvailableAttachmentFilters{}, 1, 50)
	require.NoError(t, err)
	require.EqualValues(t, 0, total)
	require.Empty(t, rows)
}

func TestGetAvailableAttachments_AttachmentWithoutPlacesHidden(t *testing.T) {
	w := setupSecurityWorld(t)
	ctx := context.Background()

	// У охранника есть место, но у согласованного cars-вложения нет ни одной строки
	// attachment_unload_places - пересечение пусто, вложение не видно.
	myPlace := w.newUnloadPlace(t, "Склад А", true)
	w.assignUnloadPlace(t, myPlace)

	app := w.newApp(t, models.ConfirmationApproved)
	att := w.newAttachment(t, app, "cars")

	rows, total, err := w.svc.GetAvailableAttachmentsForSecurity(ctx, w.guardID, false, services.AvailableAttachmentFilters{}, 1, 50)
	require.NoError(t, err)
	require.EqualValues(t, 0, total)
	require.False(t, secContainsAttachment(rows, att), "attachment without places must not leak")

	can, err := w.svc.CanSecurityViewAttachment(ctx, w.guardID, false, att)
	require.NoError(t, err)
	require.False(t, can)
}

func TestGetAvailableAttachments_InactivePlaceStillMatches(t *testing.T) {
	w := setupSecurityWorld(t)
	ctx := context.Background()

	// is_active=false не должно скрывать вложение: доступ = факт назначения места.
	inactivePlace := w.newUnloadPlace(t, "Склад на обслуживании", false)
	w.assignUnloadPlace(t, inactivePlace)

	app := w.newApp(t, models.ConfirmationApproved)
	att := w.newAttachment(t, app, "items")
	w.attachPlace(t, att, inactivePlace)

	rows, total, err := w.svc.GetAvailableAttachmentsForSecurity(ctx, w.guardID, false, services.AvailableAttachmentFilters{}, 1, 50)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.True(t, secContainsAttachment(rows, att), "inactive place still grants visibility")
}

func TestGetAvailableAttachments_IndependentFromForwardAttachments(t *testing.T) {
	w := setupSecurityWorld(t)
	ctx := context.Background()

	myPlace := w.newUnloadPlace(t, "Склад А", true)
	w.assignUnloadPlace(t, myPlace)

	app := w.newApp(t, models.ConfirmationApproved)
	attA := w.newAttachment(t, app, "cars")
	w.attachPlace(t, attA, myPlace)
	attB := w.newAttachment(t, app, "cars")
	w.attachPlace(t, attB, myPlace)

	// Пересыл ограничил бы охранника только attB, если бы фильтр звал forward-логику.
	// Security-фильтр её игнорирует - оба вложения по месту видимы.
	require.NoError(t, w.db.Create(&models.ForwardAttachment{
		ApplicationID:   app,
		RecipientUserID: w.guardID,
		AttachmentID:    attB,
	}).Error)

	rows, total, err := w.svc.GetAvailableAttachmentsForSecurity(ctx, w.guardID, false, services.AvailableAttachmentFilters{}, 1, 50)
	require.NoError(t, err)
	require.EqualValues(t, 2, total, "forward_attachments must not restrict security visibility")
	require.True(t, secContainsAttachment(rows, attA))
	require.True(t, secContainsAttachment(rows, attB))
}

func TestGetAvailableAttachments_Pagination(t *testing.T) {
	w := setupSecurityWorld(t)
	ctx := context.Background()

	myPlace := w.newUnloadPlace(t, "Склад А", true)
	w.assignUnloadPlace(t, myPlace)

	app := w.newApp(t, models.ConfirmationApproved)
	for i := 0; i < 5; i++ {
		att := w.newAttachment(t, app, "cars")
		w.attachPlace(t, att, myPlace)
	}

	page1, total, err := w.svc.GetAvailableAttachmentsForSecurity(ctx, w.guardID, false, services.AvailableAttachmentFilters{}, 1, 2)
	require.NoError(t, err)
	require.EqualValues(t, 5, total, "total counts all matches, not page size")
	require.Len(t, page1, 2)

	page3, total, err := w.svc.GetAvailableAttachmentsForSecurity(ctx, w.guardID, false, services.AvailableAttachmentFilters{}, 3, 2)
	require.NoError(t, err)
	require.EqualValues(t, 5, total)
	require.Len(t, page3, 1, "last page holds the remainder")
}

func TestCanSecurityViewAttachment(t *testing.T) {
	w := setupSecurityWorld(t)
	ctx := context.Background()

	myPlace := w.newUnloadPlace(t, "Склад А", true)
	otherPlace := w.newUnloadPlace(t, "Склад Б", true)
	w.assignUnloadPlace(t, myPlace)

	app := w.newApp(t, models.ConfirmationApproved)
	ownAtt := w.newAttachment(t, app, "cars")
	w.attachPlace(t, ownAtt, myPlace)
	foreignAtt := w.newAttachment(t, app, "cars")
	w.attachPlace(t, foreignAtt, otherPlace)

	pendingApp := w.newApp(t, "Согласование")
	pendingAtt := w.newAttachment(t, pendingApp, "cars")
	w.attachPlace(t, pendingAtt, myPlace)

	can, err := w.svc.CanSecurityViewAttachment(ctx, w.guardID, false, ownAtt)
	require.NoError(t, err)
	require.True(t, can, "own place attachment viewable")

	can, err = w.svc.CanSecurityViewAttachment(ctx, w.guardID, false, foreignAtt)
	require.NoError(t, err)
	require.False(t, can, "foreign place attachment not viewable")

	can, err = w.svc.CanSecurityViewAttachment(ctx, w.guardID, false, pendingAtt)
	require.NoError(t, err)
	require.False(t, can, "non-approved attachment not viewable")

	can, err = w.svc.CanSecurityViewAttachment(ctx, w.guardID, true, foreignAtt)
	require.NoError(t, err)
	require.True(t, can, "super-admin views attachment regardless of place")

	can, err = w.svc.CanSecurityViewAttachment(ctx, w.guardID, true, pendingAtt)
	require.NoError(t, err)
	require.False(t, can, "approved-gate applies to super-admin")
}

func TestGetAvailableAttachments_CompletedFilter(t *testing.T) {
	w := setupSecurityWorld(t)
	ctx := context.Background()

	myPlace := w.newUnloadPlace(t, "Склад А", true)
	w.assignUnloadPlace(t, myPlace)

	activeApp := w.newAppWithStatus(t, models.ConfirmationApproved, models.StatusInWork)
	activeAtt := w.newAttachmentNamed(t, activeApp, "cars", "Ромашка активная")
	w.attachPlace(t, activeAtt, myPlace)

	doneApp := w.newAppWithStatus(t, models.ConfirmationApproved, models.StatusCompleted)
	doneAtt := w.newAttachmentNamed(t, doneApp, "cars", "Ромашка завершённая")
	w.attachPlace(t, doneAtt, myPlace)

	// Дефолт вкладки: завершённые скрыты.
	rows, total, err := w.svc.GetAvailableAttachmentsForSecurity(ctx, w.guardID, false, services.AvailableAttachmentFilters{}, 1, 50)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.True(t, secContainsAttachment(rows, activeAtt))
	require.False(t, secContainsAttachment(rows, doneAtt), "завершённые скрыты по умолчанию")

	// Фильтр "Завершённые": только завершённые.
	completed := true
	rows, total, err = w.svc.GetAvailableAttachmentsForSecurity(ctx, w.guardID, false, services.AvailableAttachmentFilters{Completed: &completed}, 1, 50)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.True(t, secContainsAttachment(rows, doneAtt))
	require.False(t, secContainsAttachment(rows, activeAtt), "фильтр Завершённые показывает только их")

	// Поиск отменяет статус-предикат: видны и завершённые, и нет (даже при completed=true).
	query := "Ромашка"
	rows, total, err = w.svc.GetAvailableAttachmentsForSecurity(ctx, w.guardID, false, services.AvailableAttachmentFilters{Search: &query, Completed: &completed}, 1, 50)
	require.NoError(t, err)
	require.EqualValues(t, 2, total, "поиск показывает и завершённые, и незавершённые")
	require.True(t, secContainsAttachment(rows, activeAtt))
	require.True(t, secContainsAttachment(rows, doneAtt))
}

func TestGetAvailableAttachments_PeoplePlacesUseDisplayName(t *testing.T) {
	w := setupSecurityWorld(t)
	ctx := context.Background()

	// system_tables.name - технический код (post_72), display_name - человекочитаемое. В карточке
	// "Места" должно быть display_name, а не код (баг "Места: post_72").
	myTable := w.newPeopleTableWithDisplay(t, "post_72", "КПП Север")
	w.assignTable(t, myTable)

	app := w.newApp(t, models.ConfirmationApproved)
	att := w.newAttachment(t, app, "people")
	w.attachEmployeeWithTable(t, att, myTable)

	rows, total, err := w.svc.GetAvailableAttachmentsForSecurity(ctx, w.guardID, false, services.AvailableAttachmentFilters{}, 1, 50)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.NotNil(t, rows[0].Places)
	require.Equal(t, "КПП Север", *rows[0].Places, "места прохода показывают display_name, не код")
}
