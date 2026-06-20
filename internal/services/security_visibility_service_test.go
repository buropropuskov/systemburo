package services_test

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
// Каждый тест изолируется CleanDB; параллелизма нет - общий cachedDB.

func ptrInt(i int) *int { return &i }

type svcWorld struct {
	db       *gorm.DB
	svc      services.ApplicationService
	orgID    int
	senderID int
	guardID  int
}

func setupSecurityWorld(t *testing.T) svcWorld {
	t.Helper()
	_, db, cleanup := testutil.SetupTestApp(t)
	t.Cleanup(cleanup)
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	secTypeID := userTypeIDByCode(t, db, "security")
	userTypeID := userTypeIDByCode(t, db, "user")

	guard := models.User{Username: "guard1", Password: "x", TypeID: secTypeID, OrganizationID: ptrInt(td.OrgID)}
	require.NoError(t, db.Create(&guard).Error)
	sender := models.User{Username: "sender1", Password: "x", TypeID: userTypeID, OrganizationID: ptrInt(td.OrgID)}
	require.NoError(t, db.Create(&sender).Error)

	svc := services.NewApplicationService(db, nil, nil, nil, nil)
	return svcWorld{db: db, svc: svc, orgID: td.OrgID, senderID: sender.ID, guardID: guard.ID}
}

func userTypeIDByCode(t *testing.T, db *gorm.DB, code string) int {
	t.Helper()
	var id int
	require.NoError(t, db.Table("user_types").Where("code = ?", code).Select("id").Scan(&id).Error)
	require.NotZero(t, id, "user_type %q not seeded", code)
	return id
}

func (w svcWorld) newApp(t *testing.T, confirmation string) int {
	t.Helper()
	now := time.Now()
	conf := confirmation
	app := models.Application{
		OrganizationID:  w.orgID,
		SenderUserID:    w.senderID,
		Confirmation:    &conf,
		SendingDatetime: &now,
	}
	require.NoError(t, w.db.Create(&app).Error)
	return app.ID
}

func (w svcWorld) newAttachment(t *testing.T, appID int, atype string) int {
	t.Helper()
	att := models.Attachment{ApplicationID: appID, AttachmentType: atype}
	require.NoError(t, w.db.Create(&att).Error)
	return att.ID
}

func (w svcWorld) newUnloadPlace(t *testing.T, name string, active bool) int {
	t.Helper()
	p := models.UnloadPlace{Name: name, IsActive: active}
	require.NoError(t, w.db.Create(&p).Error)
	return p.ID
}

func (w svcWorld) newPeopleTable(t *testing.T, name string) int {
	t.Helper()
	st := models.SystemTable{Name: name, TableType: "people", IsActive: true}
	require.NoError(t, w.db.Create(&st).Error)
	return st.ID
}

func (w svcWorld) attachPlace(t *testing.T, attID, placeID int) {
	t.Helper()
	require.NoError(t, w.db.Create(&models.AttachmentUnloadPlace{AttachmentID: attID, UnloadPlaceID: placeID}).Error)
}

func (w svcWorld) attachEmployeeWithTable(t *testing.T, attID, tableID int) {
	t.Helper()
	emp := models.Employee{AttachmentID: ptrInt(attID)}
	require.NoError(t, w.db.Create(&emp).Error)
	require.NoError(t, w.db.Create(&models.EmployeeTargetTable{EmployeeID: emp.ID, TableID: tableID}).Error)
}

func (w svcWorld) assignUnloadPlace(t *testing.T, placeID int) {
	t.Helper()
	require.NoError(t, w.db.Create(&models.SecurityUserUnloadPlace{UserID: w.guardID, UnloadPlaceID: placeID}).Error)
}

func (w svcWorld) assignTable(t *testing.T, tableID int) {
	t.Helper()
	require.NoError(t, w.db.Create(&models.SecurityUserTable{UserID: w.guardID, TableID: tableID}).Error)
}

func containsAttachment(rows []services.AvailableAttachment, attID int) bool {
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

	rows, total, err := w.svc.GetAvailableAttachmentsForSecurity(ctx, w.guardID, false, 1, 50)
	require.NoError(t, err)
	require.EqualValues(t, 2, total)
	require.True(t, containsAttachment(rows, carsAtt), "cars at guard place visible")
	require.True(t, containsAttachment(rows, itemsAtt), "items at guard place visible")
	require.False(t, containsAttachment(rows, foreignAtt), "attachment at foreign place hidden")
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

	rows, total, err := w.svc.GetAvailableAttachmentsForSecurity(ctx, w.guardID, false, 1, 50)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.True(t, containsAttachment(rows, peopleAtt), "people at guard passage visible")
	require.False(t, containsAttachment(rows, foreignAtt), "people at foreign passage hidden")
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

	rows, total, err := w.svc.GetAvailableAttachmentsForSecurity(ctx, w.guardID, false, 1, 50)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.True(t, containsAttachment(rows, approvedAtt))
	require.False(t, containsAttachment(rows, pendingAtt), "non-approved application hidden")
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

	rows, total, err := w.svc.GetAvailableAttachmentsForSecurity(ctx, w.guardID, true, 1, 50)
	require.NoError(t, err)
	require.EqualValues(t, 1, total, "super-admin sees approved regardless of place, still approved-gated")
	require.True(t, containsAttachment(rows, approvedAtt))
	require.False(t, containsAttachment(rows, pendingAtt), "approved-gate applies to super-admin too")
}

func TestGetAvailableAttachments_NoPlacesEmpty(t *testing.T) {
	w := setupSecurityWorld(t)
	ctx := context.Background()

	// Охраннику не назначено ни одного места, но согласованное вложение существует.
	place := w.newUnloadPlace(t, "Склад А", true)
	app := w.newApp(t, models.ConfirmationApproved)
	att := w.newAttachment(t, app, "cars")
	w.attachPlace(t, att, place)

	rows, total, err := w.svc.GetAvailableAttachmentsForSecurity(ctx, w.guardID, false, 1, 50)
	require.NoError(t, err)
	require.EqualValues(t, 0, total)
	require.Empty(t, rows)
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

	rows, total, err := w.svc.GetAvailableAttachmentsForSecurity(ctx, w.guardID, false, 1, 50)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.True(t, containsAttachment(rows, att), "inactive place still grants visibility")
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

	rows, total, err := w.svc.GetAvailableAttachmentsForSecurity(ctx, w.guardID, false, 1, 50)
	require.NoError(t, err)
	require.EqualValues(t, 2, total, "forward_attachments must not restrict security visibility")
	require.True(t, containsAttachment(rows, attA))
	require.True(t, containsAttachment(rows, attB))
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

	page1, total, err := w.svc.GetAvailableAttachmentsForSecurity(ctx, w.guardID, false, 1, 2)
	require.NoError(t, err)
	require.EqualValues(t, 5, total, "total counts all matches, not page size")
	require.Len(t, page1, 2)

	page3, total, err := w.svc.GetAvailableAttachmentsForSecurity(ctx, w.guardID, false, 3, 2)
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
