package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"systemburo/internal/handlers"
	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func setDocumentGrant(t *testing.T, db *gorm.DB, userID int, key, value string) {
	t.Helper()
	require.NoError(t, db.Clauses(clause.OnConflict{UpdateAll: true}).Create(&models.UserPermissionOverride{UserID: userID, PermissionKey: key, Value: value}).Error)
}

func assertDocumentJSON(t *testing.T, row map[string]interface{}, hidden bool, fx documentsGateFixture) {
	t.Helper()
	assert.Equal(t, hidden, row["documents_hidden"])
	if hidden {
		for _, key := range []string{"passport_series_number", "patent_number", "other_permission"} {
			assert.Nil(t, row[key])
		}
	} else {
		assert.Equal(t, fx.passport, row["passport_series_number"])
		assert.Equal(t, fx.patent, row["patent_number"])
		assert.Equal(t, fx.other, row["other_permission"])
	}
}

func TestDocumentPolicy_AttachmentJSONAndBlank(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	senderToken := testutil.RegisterAndLogin(t, e, "projection_sender", "pass123", 1, td.OrgID, td.CompanyID)
	viewerToken := testutil.RegisterAndLogin(t, e, "projection_viewer", "pass123", 1, td.OrgID, td.CompanyID)
	sender := fetchUser(t, db, "projection_sender")
	viewer := fetchUser(t, db, "projection_viewer")
	// Absence is different from explicit deny, including for the author.
	require.NoError(t, db.Model(&models.User{}).Where("id = ?", viewer.ID).Update("role_id", nil).Error)
	fx := seedDocumentsGateApplication(t, db, td, sender.ID)
	require.NoError(t, db.Create(&models.ApplicationResponsibleUser{ApplicationID: fx.appID, UserID: viewer.ID}).Error)
	jsonURL := fmt.Sprintf("/attachments/%d/employees", fx.attID)
	blankURL := fmt.Sprintf("/applications/%d/blank?attachment_id=%d&documents=1", fx.appID, fx.attID)
	for _, key := range []string{services.KeyDetailDocuments, services.KeyDetailDocumentsExport} {
		setDocumentGrant(t, db, sender.ID, key, "deny")
	}
	rec := testutil.GET(t, e, jsonURL, testutil.AuthHeader(senderToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rows := testutil.ParseSlice(t, rec)
	require.Len(t, rows, 1)
	assertDocumentJSON(t, rows[0], false, fx)
	rec = testutil.GET(t, e, blankURL, testutil.AuthHeader(senderToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, fx.passport, blankCell(t, rec.Body.Bytes(), "B10"), "owner sees submitted documents even with deny")

	rec = testutil.GET(t, e, jsonURL, testutil.AuthHeader(viewerToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rows = testutil.ParseSlice(t, rec)
	require.Len(t, rows, 1)
	assertDocumentJSON(t, rows[0], true, fx)
	assert.NotContains(t, rec.Body.String(), fx.passport)
	setDocumentGrant(t, db, viewer.ID, services.KeyDetailDocuments, "allow")
	rec = testutil.GET(t, e, jsonURL, testutil.AuthHeader(viewerToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rows = testutil.ParseSlice(t, rec)
	assertDocumentJSON(t, rows[0], false, fx)
	rec = testutil.GET(t, e, blankURL, testutil.AuthHeader(viewerToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, documentMaskInBlank, blankCell(t, rec.Body.Bytes(), "B10"), "read alone does not grant foreign export")
	setDocumentGrant(t, db, viewer.ID, services.KeyDetailDocumentsExport, "allow")
	rec = testutil.GET(t, e, blankURL, testutil.AuthHeader(viewerToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, fx.passport, blankCell(t, rec.Body.Bytes(), "B10"))
	setDocumentGrant(t, db, viewer.ID, services.KeyDetailDocuments, "deny")
	rec = testutil.GET(t, e, jsonURL, testutil.AuthHeader(viewerToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rows = testutil.ParseSlice(t, rec)
	assertDocumentJSON(t, rows[0], true, fx)
	rec = testutil.GET(t, e, blankURL, testutil.AuthHeader(viewerToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, documentMaskInBlank, blankCell(t, rec.Body.Bytes(), "B10"))
	superToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	super := fetchUser(t, db, "testadmin")
	for _, key := range []string{services.KeyDetailDocuments, services.KeyDetailDocumentsExport} {
		setDocumentGrant(t, db, super.ID, key, "deny")
	}
	rec = testutil.GET(t, e, jsonURL, testutil.AuthHeader(superToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rows = testutil.ParseSlice(t, rec)
	assertDocumentJSON(t, rows[0], false, fx)
	rec = testutil.GET(t, e, blankURL, testutil.AuthHeader(superToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, fx.passport, blankCell(t, rec.Body.Bytes(), "B10"), "super keeps the existing full access model")
	outsider := testutil.RegisterAndLogin(t, e, "projection_outsider", "pass123", 1, td.OrgID, td.CompanyID)
	rec = testutil.GET(t, e, jsonURL, testutil.AuthHeader(outsider))
	assert.Equal(t, http.StatusForbidden, rec.Code, "document grants never expand application scope")
}

func TestDocumentPolicy_RegistryPartialUpdateAndLookup(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	owner := s1PolicyUser(t, db, td, "projection_registry_owner", false, false)
	colleague := s1PolicyUser(t, db, td, "projection_registry_colleague", false, false)
	svc := services.NewUniqueEmployeeService(db)
	fx := documentsGateFixture{passport: "SYN-001", patent: "PAT-001", other: "OTHER-001"}
	setDocumentGrant(t, db, owner.ID, services.KeyDetailDocuments, "deny")
	created, err := svc.Create(context.Background(), owner.Username, services.NewUniqueEmployeeRequest{PDConsent: true, LastName: strPtr("Projection"), FirstName: strPtr("Synthetic"), OrganizationID: &td.OrgID, CompanyID: &td.CompanyID, PassportSeriesNumber: &fx.passport, PatentNumber: &fx.patent, OtherPermission: &fx.other}, false)
	require.NoError(t, err)
	require.False(t, created.DocumentsHidden)
	assert.Equal(t, fx.passport, *created.PassportSeriesNumber, "create response is plaintext DTO, not encrypted model")
	rows, err := svc.GetAll(context.Background(), colleague.Username, "organization")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.True(t, rows[0].DocumentsHidden)
	assert.Nil(t, rows[0].PassportSeriesNumber)
	rows, total, err := svc.GetAllPaginated(context.Background(), colleague.Username, "organization", "", 1, 20)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	assert.True(t, rows[0].DocumentsHidden)
	found, err := svc.LookupByFIO(context.Background(), "Projection", "Synthetic", "", colleague.ID)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.True(t, found.DocumentsHidden)
	assert.Nil(t, found.OtherPermission)
	// Hidden and omitted document fields retain the stored values.
	updated, err := svc.Update(context.Background(), colleague.Username, created.ID, services.NewUniqueEmployeeRequest{Position: strPtr("updated position"), OrganizationID: &td.OrgID, CompanyID: &td.CompanyID}, false)
	require.NoError(t, err)
	assert.True(t, updated.DocumentsHidden)
	assert.Nil(t, updated.PassportSeriesNumber)
	for _, replacement := range []string{"forged", ""} {
		_, err = svc.Update(context.Background(), colleague.Username, created.ID, services.NewUniqueEmployeeRequest{Position: strPtr("must not save"), PassportSeriesNumber: &replacement, OrganizationID: &td.OrgID, CompanyID: &td.CompanyID}, false)
		s1Status(t, err, http.StatusForbidden)
	}
	var stored models.UniqueEmployee
	require.NoError(t, db.First(&stored, created.ID).Error)
	assert.Equal(t, "updated position", *stored.Position)
	assert.Equal(t, fx.passport, *stored.PassportSeriesNumber)
	assert.Equal(t, fx.patent, *stored.PatentNumber)
	assert.Equal(t, fx.other, *stored.OtherPermission)
	setDocumentGrant(t, db, colleague.ID, services.KeyDetailDocuments, "allow")
	updated, err = svc.Update(context.Background(), colleague.Username, created.ID, services.NewUniqueEmployeeRequest{OtherPermission: strPtr("permitted"), OrganizationID: &td.OrgID, CompanyID: &td.CompanyID}, false)
	require.NoError(t, err)
	assert.False(t, updated.DocumentsHidden)
	assert.Equal(t, "permitted", *updated.OtherPermission)
	_, err = svc.LookupByFIO(context.Background(), "Projection", "Synthetic", "", 0)
	s1Status(t, err, http.StatusForbidden)
}

func TestDocumentPolicy_TrashAndMissingActor(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	owner := s1PolicyUser(t, db, td, "projection_trash_owner", false, false)
	viewer := s1PolicyUser(t, db, td, "projection_trash_viewer", false, false)
	fx := seedDocumentsGateApplication(t, db, td, owner.ID)
	// A single controlled timestamp drives both deletion and its audit event.
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	var emp models.Employee
	require.NoError(t, db.Where("attachment_id = ?", fx.attID).First(&emp).Error)
	require.NoError(t, db.Model(&models.Employee{}).Where("id = ?", emp.ID).Updates(map[string]interface{}{"status": 0, "date_deleted": now}).Error)
	table := models.SystemTable{Name: "projection_trash_table", TableType: "people", IsActive: true}
	require.NoError(t, db.Create(&table).Error)
	require.NoError(t, db.Create(&models.AuditLog{EntityType: models.AuditEntityEmployee, EntityID: &emp.ID, ActorUserID: &owner.ID, Action: "delete", CreatedAt: now, Details: json.RawMessage(fmt.Sprintf(`{"table_id":%d}`, table.ID))}).Error)
	svc := services.NewTrashService(db, nil)
	rows, err := svc.ListEmployeesTrash(context.Background(), table.ID, models.TrashFilter{}, viewer.ID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.True(t, rows[0].DocumentsHidden)
	assert.Nil(t, rows[0].PassportSeriesNumber)
	setDocumentGrant(t, db, owner.ID, services.KeyDetailDocuments, "deny")
	rows, err = svc.ListEmployeesTrash(context.Background(), table.ID, models.TrashFilter{}, owner.ID)
	require.NoError(t, err)
	assert.False(t, rows[0].DocumentsHidden)
	assert.Equal(t, fx.passport, *rows[0].PassportSeriesNumber)
	_, err = svc.ListEmployeesTrash(context.Background(), table.ID, models.TrashFilter{}, 0)
	s1Status(t, err, http.StatusForbidden)
	apps := services.NewApplicationService(db, nil, nil, nil, nil, nil)
	_, err = apps.GetAttachmentEmployees(context.Background(), fx.attID, services.SupplementScopeAll, 0)
	s1Status(t, err, http.StatusForbidden)
}

func TestDocumentPolicy_ManualAttachmentAndFreshFailure(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	owner := s1PolicyUser(t, db, td, "projection_manual_owner", false, false)
	viewer := s1PolicyUser(t, db, td, "projection_manual_viewer", false, false)
	att := models.Attachment{AttachmentType: "people", IsManual: true, CreatedByUserID: &owner.ID, OrganizationID: &td.OrgID}
	require.NoError(t, db.Create(&att).Error)
	value := "SYNTHETIC-MANUAL-DOC"
	emp := models.Employee{AttachmentID: &att.ID, LastName: strPtr("Synthetic"), PassportSeriesNumber: &value}
	require.NoError(t, db.Create(&emp).Error)
	apps := services.NewApplicationService(db, nil, nil, nil, nil, nil)
	setDocumentGrant(t, db, owner.ID, services.KeyDetailDocuments, "deny")
	rows, err := apps.GetAttachmentEmployees(context.Background(), att.ID, services.SupplementScopeAdmitted, owner.ID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.False(t, rows[0].DocumentsHidden)
	assert.Equal(t, value, *rows[0].PassportSeriesNumber)
	rows, err = apps.GetAttachmentEmployees(context.Background(), att.ID, services.SupplementScopeAdmitted, viewer.ID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.True(t, rows[0].DocumentsHidden)
	assert.Nil(t, rows[0].PassportSeriesNumber)
	const callback = "s3:document_policy_read_failure"
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Table == "users" {
			tx.AddError(errors.New("synthetic policy lookup failure"))
		}
	}))
	t.Cleanup(func() { db.Callback().Query().Remove(callback) })
	rows, err = apps.GetAttachmentEmployees(context.Background(), att.ID, services.SupplementScopeAdmitted, owner.ID)
	require.NoError(t, db.Callback().Query().Remove(callback))
	s1Status(t, err, http.StatusServiceUnavailable)
	assert.Nil(t, rows, "failed policy never returns the document row")
	require.NoError(t, db.Model(&models.User{}).Where("id = ?", owner.ID).Update("is_banned", true).Error)
	rows, err = apps.GetAttachmentEmployees(context.Background(), att.ID, services.SupplementScopeAdmitted, owner.ID)
	require.NoError(t, err)
	assert.True(t, rows[0].DocumentsHidden, "owner exception does not restore banned service access")
	assert.Nil(t, rows[0].PassportSeriesNumber)
}

// Only the owner lookup fails: application visibility is already established,
// and the policy still resolves the actor's current state and grants from DB.
type documentOwnerLookupFailure struct {
	services.ApplicationService
	cause error
	calls int
}

func (s *documentOwnerLookupFailure) CanAccessApplication(context.Context, int, string, bool) bool {
	return true
}

func (s *documentOwnerLookupFailure) IsApplicationSender(context.Context, int, int) (bool, error) {
	s.calls++
	return false, fmt.Errorf("synthetic application owner lookup: %w", s.cause)
}

func TestDocumentPolicy_BlankOwnerLookupFailure(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	actor := s1PolicyUser(t, db, td, "projection_owner_lookup_error", false, false)
	cause := errors.New("synthetic owner source unavailable")
	access := &documentOwnerLookupFailure{cause: cause}
	// No generator is supplied: owner lookup failure must stop before output.
	handler := handlers.NewAttachmentBlankHandler(nil, access, services.NewPermissionResolver(db), nil)
	e := echo.New()
	rec := httptest.NewRecorder()
	c := e.NewContext(httptest.NewRequest(http.MethodGet, "/applications/1/blank?attachment_id=1&documents=1", nil), rec)
	c.SetParamNames("id")
	c.SetParamValues("1")
	c.Set("user_id", actor.ID)
	c.Set("username", actor.Username)
	err := handler.Download(c)
	s1Status(t, err, http.StatusServiceUnavailable)
	assert.ErrorIs(t, err, cause, "retain the lookup cause for internal diagnostics")
	assert.Equal(t, 1, access.calls, "failure occurs after successful fresh policy resolution")
	assert.Empty(t, rec.Body.String(), "no file bytes after owner lookup failure")
}
