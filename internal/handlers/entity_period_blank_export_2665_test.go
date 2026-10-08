package handlers_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

type periodBlank2665Fixture struct {
	archiveWorld
	kind                    services.ElementKind
	app, attachment, entity int
	clock                   time.Time
	parent, own             services.EntityPeriodInput
}

func seedPeriodBlank2665(t *testing.T, w archiveWorld, kind services.ElementKind) periodBlank2665Fixture {
	t.Helper()
	now := time.Now().In(services.MoscowLocation())
	day := func(n int) string { return now.AddDate(0, 0, n).Format("2006-01-02") }
	parent := services.EntityPeriodInput{EntryDateFrom: day(-1), EntryDateTo: day(2), EntryTimeFrom: "08:00:00", EntryTimeTo: "22:00:00"}
	own := services.EntityPeriodInput{EntryDateFrom: day(-3), EntryDateTo: day(5), EntryTimeFrom: "09:15:00", EntryTimeTo: "20:45:00"}
	attachmentType := "people"
	if kind == services.ElementCar {
		attachmentType = "cars"
	}
	ua := w.newExportType(t, "period_blank2665_"+string(kind), false, true)
	require.NoError(t, w.db.Model(&models.UniqueAttachment{}).Where("id=?", ua).Update("attachment_type", attachmentType).Error)
	app, att := w.newExportApp(t, "2665/"+string(kind), ua, parent.EntryDateTo)
	active := 1
	require.NoError(t, w.db.Table("attachments").Where("id=?", att).Updates(map[string]any{"attachment_type": attachmentType, "status": active, "entry_date_from": parent.EntryDateFrom, "entry_time_from": parent.EntryTimeFrom, "entry_time_to": parent.EntryTimeTo}).Error)
	f := excelize.NewFile()
	defer func() { require.NoError(t, f.Close()) }()
	path := filepath.Join(t.TempDir(), "period2665.xlsx")
	require.NoError(t, f.SaveAs(path))
	tpl := models.AttachmentTemplate{UniqueAttachmentID: ua, IsActive: true, FilePath: path, OriginalFileName: "period2665.xlsx", ListStartRow: 5, ListEndRow: 7, MaxListRows: 3}
	require.NoError(t, w.db.Create(&tpl).Error)
	mappings := []models.AttachmentTemplateMapping{}
	for i, field := range []string{"entry_date_from", "entry_date_to", "entry_time_from", "entry_time_to"} {
		mappings = append(mappings, models.AttachmentTemplateMapping{TemplateID: tpl.ID, CellRef: fmt.Sprintf("%c1", 'A'+i), FieldPath: "attachment." + field})
		if kind == services.ElementCar {
			mappings = append(mappings, models.AttachmentTemplateMapping{TemplateID: tpl.ID, CellRef: fmt.Sprintf("%c5", 'A'+i), FieldPath: "car." + field, IsListField: true})
		}
	}
	if kind == services.ElementEmployee {
		mappings = append(mappings, models.AttachmentTemplateMapping{TemplateID: tpl.ID, CellRef: "E5", FieldPath: "employee.passport_series_number", IsListField: true})
	}
	require.NoError(t, w.db.Create(&mappings).Error)
	entity := 0
	if kind == services.ElementCar {
		plate := "BLANK2665"
		car := models.Car{AttachmentID: att, Status: &active, CarNumber: &plate, PeriodMode: models.PeriodIndividual, EntryDateFrom: &own.EntryDateFrom, EntryDateTo: &own.EntryDateTo, EntryTimeFrom: &own.EntryTimeFrom, EntryTimeTo: &own.EntryTimeTo}
		require.NoError(t, w.db.Create(&car).Error)
		entity = car.ID
	} else {
		passport := "9900 000265"
		emp := models.Employee{AttachmentID: &att, Status: &active, PassportSeriesNumber: &passport, PeriodMode: models.PeriodIndividual, EntryDateFrom: &own.EntryDateFrom, EntryDateTo: &own.EntryDateTo, EntryTimeFrom: &own.EntryTimeFrom, EntryTimeTo: &own.EntryTimeTo}
		require.NoError(t, w.db.Create(&emp).Error)
		entity = emp.ID
	}
	return periodBlank2665Fixture{archiveWorld: w, kind: kind, app: app, attachment: att, entity: entity, clock: now, parent: parent, own: own}
}

func TestEntityPeriod2665DBGenerateBlankWholeCarWindow(t *testing.T) {
	w := seedPeriodBlank2665(t, setupArchiveWorld(t), services.ElementCar)
	service := services.NewAttachmentBlankService(w.db)
	for _, documents := range []bool{false, true} {
		t.Run(fmt.Sprintf("documents_%t", documents), func(t *testing.T) {
			reader, _, err := service.GenerateBlank(context.Background(), w.app, w.attachment, services.BlankOptions{IncludeDocuments: documents})
			require.NoError(t, err)
			data, err := io.ReadAll(reader)
			require.NoError(t, err)
			date := func(raw string) string {
				parsed, err := time.Parse("2006-01-02", raw)
				require.NoError(t, err)
				return parsed.Format("02.01.2006")
			}
			for i, want := range []string{date(w.parent.EntryDateFrom), date(w.parent.EntryDateTo), "08:00", "22:00"} {
				require.Equal(t, want, blankCell(t, data, fmt.Sprintf("%c1", 'A'+i)))
			}
			for i, want := range []string{date(w.own.EntryDateFrom), date(w.own.EntryDateTo), "09:15", "20:45"} {
				require.Equal(t, want, blankCell(t, data, fmt.Sprintf("%c5", 'A'+i)))
			}
		})
	}
}

type periodArchive2665Snapshot struct {
	SchemaVersion int `json:"schema_version"`
	Attachments   []struct {
		ID            int                       `json:"id"`
		EntryDateFrom string                    `json:"entry_date_from"`
		EntryDateTo   string                    `json:"entry_date_to"`
		EntryTimeFrom string                    `json:"entry_time_from"`
		EntryTimeTo   string                    `json:"entry_time_to"`
		Employees     []periodArchive2665Entity `json:"employees"`
		Cars          []periodArchive2665Entity `json:"cars"`
	} `json:"attachments"`
}

type periodArchive2665Entity struct {
	ID            int                `json:"id"`
	EntryDateFrom string             `json:"entry_date_from"`
	EntryDateTo   string             `json:"entry_date_to"`
	EntryTimeFrom string             `json:"entry_time_from"`
	EntryTimeTo   string             `json:"entry_time_to"`
	PeriodMode    models.PeriodMode  `json:"period_mode"`
	PeriodSource  string             `json:"period_source"`
	PeriodBounded bool               `json:"period_bounded"`
	StoredPeriod  models.EntryPeriod `json:"stored_period"`
}

func (w periodBlank2665Fixture) snapshotBytes(t *testing.T, result *models.BlankExportResult, want services.EntityPeriodInput) []byte {
	t.Helper()
	require.Equal(t, models.BlankExportOK, result.Snapshot.Status, result.Snapshot.Error)
	data, err := os.ReadFile(w.abs(result.Snapshot.RelPath))
	require.NoError(t, err)
	var snapshot periodArchive2665Snapshot
	require.NoError(t, json.Unmarshal(data, &snapshot))
	require.Equal(t, 2, snapshot.SchemaVersion)
	require.Len(t, snapshot.Attachments, 1)
	att := snapshot.Attachments[0]
	require.Equal(t, w.attachment, att.ID)
	require.Equal(t, []string{w.parent.EntryDateFrom, w.parent.EntryDateTo, w.parent.EntryTimeFrom, w.parent.EntryTimeTo}, []string{att.EntryDateFrom, att.EntryDateTo, att.EntryTimeFrom, att.EntryTimeTo})
	rows := att.Employees
	if w.kind == services.ElementCar {
		rows = att.Cars
	}
	require.Len(t, rows, 1)
	row := rows[0]
	require.Equal(t, w.entity, row.ID)
	require.Equal(t, models.PeriodIndividual, row.PeriodMode)
	require.Equal(t, "individual", row.PeriodSource)
	require.True(t, row.PeriodBounded)
	require.Equal(t, []string{want.EntryDateFrom, want.EntryDateTo, want.EntryTimeFrom, want.EntryTimeTo}, []string{row.EntryDateFrom, row.EntryDateTo, row.EntryTimeFrom, row.EntryTimeTo})
	require.Equal(t, models.EntryPeriod{EntryDateFrom: &want.EntryDateFrom, EntryDateTo: &want.EntryDateTo, EntryTimeFrom: &want.EntryTimeFrom, EntryTimeTo: &want.EntryTimeTo}, row.StoredPeriod)
	hash := sha256.Sum256(data)
	require.Equal(t, hex.EncodeToString(hash[:]), w.registryRow(t, w.app, 0).ContentHash)
	return data
}

func TestEntityPeriod2665DBSchema2ReexportChangesWithPeriod(t *testing.T) {
	for _, kind := range []services.ElementKind{services.ElementEmployee, services.ElementCar} {
		t.Run(string(kind), func(t *testing.T) {
			w := seedPeriodBlank2665(t, setupArchiveWorld(t), kind)
			first := w.reexport(t, w.app)
			require.True(t, first.Snapshot.Written)
			before := w.snapshotBytes(t, first, w.own)
			beforeHash := w.registryRow(t, w.app, 0).ContentHash
			unchanged := w.reexport(t, w.app)
			require.False(t, unchanged.Snapshot.Written)
			require.Equal(t, before, w.snapshotBytes(t, unchanged, w.own))
			commands := services.NewEntityPeriodCommandService(w.db, services.NewAuditRecorder(w.db))
			inspect, err := commands.Inspect(context.Background(), w.senderID, kind, w.entity, nil)
			require.NoError(t, err)
			next := w.own
			next.EntryDateTo = w.clock.AddDate(0, 0, 7).Format("2006-01-02")
			_, err = commands.Change(context.Background(), w.senderID, kind, w.entity, services.ChangeEntityPeriodRequest{PeriodMode: models.PeriodIndividual, Period: &next, Reason: "Изменение периода для архивного регрессионного теста", ExpectedRevision: inspect.Revision})
			require.NoError(t, err)
			changed := w.reexport(t, w.app)
			require.True(t, changed.Snapshot.Written)
			require.NotEqual(t, before, w.snapshotBytes(t, changed, next))
			require.NotEqual(t, beforeHash, w.registryRow(t, w.app, 0).ContentHash)
			// Make the parent equal to the own window, then change only the
			// explicit source. Equal effective dates must not erase metadata.
			require.NoError(t, w.db.Table("attachments").Where("id=?", w.attachment).Updates(map[string]any{"entry_date_from": next.EntryDateFrom, "entry_date_to": next.EntryDateTo, "entry_time_from": next.EntryTimeFrom, "entry_time_to": next.EntryTimeTo}).Error)
			w.parent = next
			equalWindow := w.reexport(t, w.app)
			w.snapshotBytes(t, equalWindow, next)
			metadataHash := w.registryRow(t, w.app, 0).ContentHash
			inspect, err = commands.Inspect(context.Background(), w.senderID, kind, w.entity, nil)
			require.NoError(t, err)
			_, err = commands.Change(context.Background(), w.senderID, kind, w.entity, services.ChangeEntityPeriodRequest{PeriodMode: models.PeriodInherit, Reason: "Возврат к равному сроку вложения", ExpectedRevision: inspect.Revision})
			require.NoError(t, err)
			inherited := w.reexport(t, w.app)
			require.True(t, inherited.Snapshot.Written)
			data, err := os.ReadFile(w.abs(inherited.Snapshot.RelPath))
			require.NoError(t, err)
			var snapshot periodArchive2665Snapshot
			require.NoError(t, json.Unmarshal(data, &snapshot))
			require.Equal(t, 2, snapshot.SchemaVersion)
			require.Len(t, snapshot.Attachments, 1)
			rows := snapshot.Attachments[0].Employees
			if kind == services.ElementCar {
				rows = snapshot.Attachments[0].Cars
			}
			require.Len(t, rows, 1)
			require.Equal(t, models.PeriodInherit, rows[0].PeriodMode)
			require.Equal(t, "attachment", rows[0].PeriodSource)
			require.True(t, rows[0].PeriodBounded)
			require.Equal(t, models.EntryPeriod{}, rows[0].StoredPeriod)
			require.Equal(t, []string{next.EntryDateFrom, next.EntryDateTo, next.EntryTimeFrom, next.EntryTimeTo}, []string{rows[0].EntryDateFrom, rows[0].EntryDateTo, rows[0].EntryTimeFrom, rows[0].EntryTimeTo})
			require.NotEqual(t, metadataHash, w.registryRow(t, w.app, 0).ContentHash)
		})
	}
}

func TestEntityPeriod2665DBSchema2FrozenReexportKeepsPeriod(t *testing.T) {
	for _, kind := range []services.ElementKind{services.ElementEmployee, services.ElementCar} {
		t.Run(string(kind), func(t *testing.T) {
			w := seedPeriodBlank2665(t, setupArchiveWorld(t), kind)
			testutil.SetArchiveSettings(t, w.db, models.UpdateArchiveSettingsRequest{FreezeAfterDays: testutil.Ptr(0)})
			w.parent.EntryDateFrom, w.parent.EntryDateTo = w.clock.AddDate(0, 0, -7).Format("2006-01-02"), w.clock.AddDate(0, 0, -3).Format("2006-01-02")
			w.own.EntryDateFrom, w.own.EntryDateTo = w.clock.AddDate(0, 0, -6).Format("2006-01-02"), w.clock.AddDate(0, 0, -2).Format("2006-01-02")
			require.NoError(t, w.db.Table("attachments").Where("id=?", w.attachment).Updates(map[string]any{"entry_date_from": w.parent.EntryDateFrom, "entry_date_to": w.parent.EntryDateTo}).Error)
			require.NoError(t, w.db.Table(string(kind)).Where("id=?", w.entity).Updates(map[string]any{"entry_date_from": w.own.EntryDateFrom, "entry_date_to": w.own.EntryDateTo}).Error)
			require.NoError(t, w.db.Model(&models.Application{}).Where("id=?", w.app).Update("status", models.StatusCompleted).Error)
			first := w.reexport(t, w.app)
			require.True(t, first.Snapshot.Frozen)
			before := w.snapshotBytes(t, first, w.own)
			beforeHash := w.registryRow(t, w.app, 0).ContentHash
			// Simulate a later historical data edit. This is intentionally not a
			// lifecycle command: a frozen document must retain its prior values.
			require.NoError(t, w.db.Table(string(kind)).Where("id=?", w.entity).Update("entry_time_to", "19:30:00").Error)
			second := w.reexport(t, w.app)
			require.True(t, second.Snapshot.Frozen)
			require.False(t, second.Snapshot.Written)
			require.Equal(t, before, w.snapshotBytes(t, second, w.own))
			require.Equal(t, beforeHash, w.registryRow(t, w.app, 0).ContentHash)
		})
	}
}

func TestEntityPeriod2665DBBlankDocumentsAccessUnchanged(t *testing.T) {
	w := setupArchiveWorld(t)
	fx := seedDocumentsGateApplication(t, w.db, testutil.TestData{OrgID: w.orgID}, w.senderID)
	day := time.Now().In(services.MoscowLocation())
	require.NoError(t, w.db.Table("employees").Where("attachment_id=?", fx.attID).Updates(map[string]any{"period_mode": models.PeriodIndividual, "entry_date_from": day.AddDate(0, 0, -1).Format("2006-01-02"), "entry_date_to": day.AddDate(0, 0, 5).Format("2006-01-02"), "entry_time_from": "08:00:00", "entry_time_to": "22:00:00"}).Error)
	url := fmt.Sprintf("/applications/%d/blank?attachment_id=%d", fx.appID, fx.attID)
	for _, documents := range []bool{false, true} {
		reader, _, err := services.NewAttachmentBlankService(w.db).GenerateBlank(context.Background(), fx.appID, fx.attID, services.BlankOptions{IncludeDocuments: documents})
		require.NoError(t, err)
		data, err := io.ReadAll(reader)
		require.NoError(t, err)
		want := documentMaskInBlank
		if documents {
			want = fx.passport
		}
		require.Equal(t, want, blankCell(t, data, "B10"))
	}
	masked := testutil.GET(t, w.e, url, w.adminH)
	require.Equal(t, http.StatusOK, masked.Code, masked.Body.String())
	require.Equal(t, documentMaskInBlank, blankCell(t, masked.Body.Bytes(), "B10"))
	opened := testutil.GET(t, w.e, url+"&documents=1", w.adminH)
	require.Equal(t, http.StatusOK, opened.Code, opened.Body.String())
	require.Equal(t, fx.passport, blankCell(t, opened.Body.Bytes(), "B10"))
	outsider := testutil.RegisterAndLogin(t, w.e, "period_blank2665_outsider", "periodblank_password_long_enough!", 1, 0, 0)
	for _, query := range []string{"", "&documents=1"} {
		response := testutil.GET(t, w.e, url+query, testutil.AuthHeader(outsider))
		require.Equal(t, http.StatusForbidden, response.Code, response.Body.String())
		require.NotContains(t, response.Body.String(), fx.passport)
	}
}
