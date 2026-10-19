package handlers_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"systemburo/internal/config"
	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

func TestArchiveOriginal_CurrentRoleAndPreflight(t *testing.T) {
	w := setupArchiveWorld(t)
	uaID := w.newExportType(t, "S6 archive fixture", true, true)
	appID, attID := w.newExportApp(t, "S6-ARCHIVE-FIXTURE", uaID, "")
	w.reexport(t, appID)
	row := w.registryRow(t, appID, attID)
	path := filepath.Join(w.root, filepath.FromSlash(row.RelDir), row.FileName)
	original := []byte("S6-HISTORICAL-ORIGINAL")
	require.NoError(t, os.WriteFile(path, original, 0600))
	var snapshot models.BlankExport
	require.NoError(t, w.db.Where("application_id = ? AND attachment_id = 0", appID).First(&snapshot).Error)
	snapshotPath := filepath.Join(w.root, filepath.FromSlash(snapshot.RelDir), snapshot.FileName)
	legacy := []byte(`{"schema_version":1,"approvals":[{"name":"Fixture historical actor"}]}`)
	require.NoError(t, os.WriteFile(snapshotPath, legacy, 0600))
	var companyID int
	require.NoError(t, w.db.Raw("SELECT company_id FROM users WHERE id = ?", w.senderID).Scan(&companyID).Error)
	token := testutil.RegisterAndLogin(t, w.e, "s6_archive_fixture", archiveDownloadPassword,
		secUserTypeIDByCode(t, w.db, "user"), w.orgID, companyID)
	id := secUserIDByUsername(t, w.db, "s6_archive_fixture")
	h := testutil.AuthHeader(token)
	for _, key := range []string{"page.admin.file_archive", "action.download.file_archive", services.KeyDetailDocuments, services.KeyDetailDocumentsExport} {
		testutil.GrantPermission(t, id, key)
	}
	require.NoError(t, w.db.Create(&models.ApplicationResponsibleUser{ApplicationID: appID, UserID: id}).Error)
	date := row.BucketDate.Format("2006-01-02")
	period := fmt.Sprintf(`{"date_from":%q,"date_to":%q}`, date, date)
	liveURL := fmt.Sprintf("/applications/%d/blank?attachment_id=%d&documents=true", appID, attID)
	originalURLs := []string{liveURL + "&source=archive", fmt.Sprintf("/applications/%d/archive", appID),
		fmt.Sprintf("/file-archive/files/%d", row.ID), fmt.Sprintf("/file-archive/files/%d", snapshot.ID)}
	for _, admin := range []bool{false, true} {
		t.Run(fmt.Sprintf("non-super/admin=%t", admin), func(t *testing.T) {
			require.NoError(t, w.db.Model(&models.User{}).Where("id = ?", id).Update("is_admin", admin).Error)
			for _, endpoint := range originalURLs {
				rec := testutil.GET(t, w.e, endpoint, h)
				require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
				require.NotContains(t, rec.Body.String(), string(original))
				require.NotContains(t, rec.Body.String(), "Fixture historical actor")
				require.NotContains(t, rec.Header().Get("Content-Type"), "application/zip")
			}
			rec := testutil.POST(t, w.e, "/file-archive/download-ticket", period, h)
			require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
			rec = testutil.GET(t, w.e, liveURL, h)
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			require.True(t, len(rec.Body.Bytes()) > 2)
			require.Equal(t, "PK", rec.Body.String()[:2])
			require.NotContains(t, rec.Body.String(), string(original))
		})
	}

	t.Run("role revocation after ticket preserves grants but closes originals", func(t *testing.T) {
		require.NoError(t, w.db.Model(&models.User{}).Where("id = ?", id).Update("is_super_admin", true).Error)
		rec := testutil.POST(t, w.e, "/file-archive/download-ticket", period, h)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		ticket := testutil.ParseResponse[models.ArchiveDownloadTicketResponse](t, rec).Ticket
		require.NoError(t, w.db.Model(&models.User{}).Where("id = ?", id).Update("is_super_admin", false).Error)
		rec = testutil.GET(t, w.e, "/file-archive/download?ticket="+ticket, nil)
		require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
		require.NotContains(t, rec.Header().Get("Content-Type"), "application/zip")
		require.NotContains(t, rec.Body.String(), string(original))
		rec = testutil.GET(t, w.e, "/file-archive/download?ticket="+ticket, nil)
		require.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("super receives original including legacy snapshot", func(t *testing.T) {
		rec := testutil.GET(t, w.e, originalURLs[0], w.adminH)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		require.Equal(t, original, rec.Body.Bytes())
		rec = testutil.GET(t, w.e, originalURLs[3], w.adminH)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		require.JSONEq(t, string(legacy), rec.Body.String())
	})

	t.Run("generation failure never returns the existing original", func(t *testing.T) {
		require.NoError(t, w.db.Model(&models.AttachmentTemplate{}).Where("unique_attachment_id = ?", uaID).Update("is_active", false).Error)
		rec := testutil.GET(t, w.e, liveURL, h)
		require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
		require.NotContains(t, rec.Body.String(), string(original))
		require.NoError(t, w.db.Model(&models.AttachmentTemplate{}).Where("unique_attachment_id = ?", uaID).Update("is_active", true).Error)
	})

	stored, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, original, stored)
	stored, err = os.ReadFile(snapshotPath)
	require.NoError(t, err)
	require.Equal(t, legacy, stored)

	t.Run("all files are prepared before any ZIP response", func(t *testing.T) {
		rec := testutil.POST(t, w.e, "/file-archive/download-ticket", period, w.adminH)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		ticket := testutil.ParseResponse[models.ArchiveDownloadTicketResponse](t, rec).Ticket
		require.NoError(t, os.Remove(path))
		for _, endpoint := range []string{fmt.Sprintf("/applications/%d/archive", appID), "/file-archive/download?ticket=" + ticket} {
			rec = testutil.GET(t, w.e, endpoint, w.adminH)
			require.Equal(t, http.StatusServiceUnavailable, rec.Code, rec.Body.String())
			require.NotContains(t, rec.Header().Get("Content-Type"), "application/zip")
			require.NotContains(t, rec.Body.String(), "Fixture historical actor")
		}
		require.NoError(t, os.WriteFile(path, original, 0600))
	})

	var template models.AttachmentTemplate
	require.NoError(t, w.db.Where("unique_attachment_id = ?", uaID).First(&template).Error)
	require.NoError(t, w.db.Create(&models.AttachmentTemplateMapping{
		TemplateID: template.ID, CellRef: "B1", FieldPath: "application.approvers",
	}).Error)
	last, label := "FixtureActorSurnameS6", "Оператор бюро"
	require.NoError(t, w.db.Model(&models.User{}).Where("id = ?", id).
		Updates(map[string]any{"last_name": last, "first_name": nil, "middle_name": nil}).Error)
	require.NoError(t, w.db.Create(&models.ApplicationApprover{UserID: id, DisplayName: &label}).Error)
	require.NoError(t, w.db.Model(&models.ApplicationResponsibleUser{}).
		Where("application_id = ? AND user_id = ?", appID, id).Update("approval_status", "approved").Error)
	blankActor := func(headers http.Header) string {
		t.Helper()
		rec := testutil.GET(t, w.e, liveURL, headers)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		book, err := excelize.OpenReader(bytes.NewReader(rec.Body.Bytes()))
		require.NoError(t, err)
		defer book.Close()
		value, err := book.GetCellValue(book.GetSheetName(0), "B1")
		require.NoError(t, err)
		return value
	}
	t.Run("current workbook uses the effective viewer identity policy", func(t *testing.T) {
		require.NoError(t, w.db.Model(&models.User{}).Where("id = ?", id).Update("is_admin", false).Error)
		require.Equal(t, label, blankActor(h))
		require.Equal(t, last, blankActor(w.adminH))
	})

	t.Run("unverifiable legacy originals are unavailable while consent enforcement is active", func(t *testing.T) {
		settings := services.NewSettingsService(w.db, &config.Config{})
		ctx := context.Background()
		require.NoError(t, settings.SetPDConsentText(ctx, "<p>Fixture consent</p>", true))
		require.NoError(t, settings.SetPDConsentRequired(ctx, true))
		metadata := testutil.GET(t, w.e, fmt.Sprintf("/file-archive/items?application_id=%d", appID), w.adminH)
		require.Equal(t, http.StatusOK, metadata.Code, metadata.Body.String())
		require.NotContains(t, metadata.Body.String(), row.FileName)
		require.Contains(t, metadata.Body.String(), "Исторический файл")
		for _, endpoint := range originalURLs {
			rec := testutil.GET(t, w.e, endpoint, w.adminH)
			require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
			require.NotContains(t, rec.Body.String(), "Fixture historical actor")
		}
		require.NoError(t, settings.SetPDConsentRequired(ctx, false))
	})

	t.Run("registry objection closes current and historical workbooks", func(t *testing.T) {
		at := time.Date(2026, 10, 19, 9, 0, 0, 0, time.UTC)
		name, passport, source := "FixtureGuestS6", "S6-GUEST-PASSPORT", "Fixture request"
		registry := models.UniqueEmployee{LastName: &name, PassportSeriesNumber: &passport, PDConsentAt: &at}
		require.NoError(t, w.db.Create(&registry).Error)
		person := models.Employee{AttachmentID: &attID, LastName: &name, PassportSeriesNumber: &passport, PeriodMode: models.PeriodMode("inherit")}
		require.NoError(t, w.db.Create(&person).Error)
		require.NoError(t, w.db.Model(&models.Attachment{}).Where("id = ?", attID).
			Updates(map[string]any{"entry_date_from": "2026-10-19", "entry_date_to": "2026-10-20"}).Error)
		defer func() {
			require.NoError(t, w.db.Delete(&person).Error)
			require.NoError(t, w.db.Delete(&registry).Error)
			require.NoError(t, w.db.Model(&models.Attachment{}).Where("id = ?", attID).
				Updates(map[string]any{"entry_date_from": nil, "entry_date_to": nil}).Error)
		}()
		rec := testutil.GET(t, w.e, liveURL, w.adminH)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		require.NoError(t, w.db.Model(&models.UniqueEmployee{}).Where("id = ?", registry.ID).
			Updates(map[string]any{"pd_objection_at": at, "pd_objection_source": source}).Error)
		for _, endpoint := range append([]string{liveURL}, originalURLs...) {
			rec = testutil.GET(t, w.e, endpoint, w.adminH)
			require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
			require.NotContains(t, rec.Body.String(), name)
			require.NotContains(t, rec.Body.String(), passport)
			require.NotContains(t, rec.Body.String(), string(original))
		}
		var current models.Employee
		require.NoError(t, w.db.First(&current, person.ID).Error)
		otherPassport := "S6-DIFFERENT-GUEST-PASSPORT"
		current.PassportSeriesNumber = &otherPassport
		require.NoError(t, w.db.Save(&current).Error)
		rec = testutil.GET(t, w.e, liveURL, w.adminH)
		require.Equal(t, http.StatusOK, rec.Code, "unrelated identified participant remains available: %s", rec.Body.String())
		require.NoError(t, w.db.Model(&models.UniqueEmployee{}).Where("id = ?", registry.ID).
			UpdateColumn("passport_series_number_hmac", nil).Error)
		rec = testutil.GET(t, w.e, liveURL, w.adminH)
		require.Equal(t, http.StatusForbidden, rec.Code, "unknown legacy identity must not be matched by name")
		require.NoError(t, w.db.Model(&models.UniqueEmployee{}).Where("id = ?", registry.ID).
			UpdateColumn("passport_series_number_hmac", registry.PassportSeriesNumberHMAC).Error)
		current.PassportSeriesNumber = nil
		current.PassportSeriesNumberHMAC = nil
		require.NoError(t, w.db.Save(&current).Error)
		rec = testutil.GET(t, w.e, liveURL, w.adminH)
		require.Equal(t, http.StatusForbidden, rec.Code, "unidentified current participant must fail closed")
	})

	t.Run("withdrawal blocks legacy originals even for super with consent prompt disabled", func(t *testing.T) {
		at := time.Date(2026, 10, 19, 9, 0, 0, 0, time.UTC)
		withdrawal := models.PDConsent{UserID: id, ConsentType: services.ConsentTypePDProcessing,
			Granted: false, GrantedAt: at, RevokedAt: &at}
		require.NoError(t, w.db.Create(&withdrawal).Error)
		require.Equal(t, "@s6_archive_fixture", blankActor(w.adminH), "withdrawal overrides superadmin and display alias")
		for _, endpoint := range originalURLs {
			rec := testutil.GET(t, w.e, endpoint, w.adminH)
			require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
			require.NotContains(t, rec.Body.String(), string(original))
			require.NotContains(t, rec.Body.String(), "Fixture historical actor")
		}
		rec := testutil.POST(t, w.e, "/file-archive/download-ticket", period, w.adminH)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		ticket := testutil.ParseResponse[models.ArchiveDownloadTicketResponse](t, rec).Ticket
		rec = testutil.GET(t, w.e, "/file-archive/download?ticket="+ticket, nil)
		require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
		require.NotContains(t, rec.Header().Get("Content-Type"), "application/zip")
		stored, err := os.ReadFile(snapshotPath)
		require.NoError(t, err)
		require.Equal(t, legacy, stored, "withdrawal must not rewrite retained originals")
	})
}
