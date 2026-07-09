package handlers_test

import (
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/testutil"
)

// TestManualAttachment_OrphanSupported — guard миграции #1049 (срез S1): вложение-сирота
// без заявки (application_id NULL, is_manual=true, org/company хранятся на вложении)
// вставляется и корректно читается. Доказывает, что relaxAttachmentApplicationNotNull
// снял NOT NULL с attachments.application_id и новые колонки существуют. Регресс
// заявочного пути (application_id заполнен) покрыт submit-flow тестами.
func TestManualAttachment_OrphanSupported(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	org := models.Organization{Name: "Орг-Ручная", IsActive: true}
	if err := db.Create(&org).Error; err != nil {
		t.Fatalf("seed org: %v", err)
	}

	orphan := models.Attachment{
		AttachmentType: "cars",
		IsManual:       true,
		OrganizationID: &org.ID,
	}
	if err := db.Create(&orphan).Error; err != nil {
		t.Fatalf("создать вложение-сироту (application_id NULL): %v", err)
	}

	var got models.Attachment
	if err := db.First(&got, orphan.ID).Error; err != nil {
		t.Fatalf("прочитать сироту: %v", err)
	}
	if got.ApplicationID != nil {
		t.Errorf("application_id сироты = %d, ожидался NULL", *got.ApplicationID)
	}
	if !got.IsManual {
		t.Error("is_manual сироты = false, ожидался true")
	}
	if got.OrganizationID == nil || *got.OrganizationID != org.ID {
		t.Errorf("organization_id сироты = %v, ожидался %d", got.OrganizationID, org.ID)
	}
}
