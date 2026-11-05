package services_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"systemburo/internal/models"
	"systemburo/internal/services"
)

type notificationBatchQueryProbe struct {
	mu           sync.Mutex
	queries      []string
	maxVariables int
}

func notificationBatchProbe(t *testing.T, db *gorm.DB) *notificationBatchQueryProbe {
	t.Helper()
	p := &notificationBatchQueryProbe{}
	observe := func(tx *gorm.DB) {
		sql := strings.ToLower(strings.TrimSpace(tx.Statement.SQL.String()))
		if !strings.HasPrefix(sql, "select") {
			return
		}
		p.mu.Lock()
		defer p.mu.Unlock()
		p.queries = append(p.queries, sql)
		if len(tx.Statement.Vars) > p.maxVariables {
			p.maxVariables = len(tx.Statement.Vars)
		}
	}
	const callback = "notification_batch_query_probe"
	require.NoError(t, db.Callback().Query().After("gorm:query").Register(callback, observe))
	require.NoError(t, db.Callback().Row().After("gorm:row").Register(callback, observe))
	t.Cleanup(func() { db.Callback().Query().Remove(callback); db.Callback().Row().Remove(callback) })
	return p
}

func (p *notificationBatchQueryProbe) reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.queries, p.maxVariables = nil, 0
}

func (p *notificationBatchQueryProbe) counts() (total, applications, actorMetadata, names, maxVariables int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, sql := range p.queries {
		if strings.Contains(sql, "from \"applications\"") || strings.Contains(sql, "from applications ") {
			applications++
		}
		if strings.Contains(sql, "as revoked") {
			actorMetadata++
		}
		if strings.Contains(strings.SplitN(sql, " from ", 2)[0], "last_name") {
			names++
		}
	}
	return len(p.queries), applications, actorMetadata, names, p.maxVariables
}

func (p *notificationBatchQueryProbe) tableCount(table string) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	count := 0
	for _, sql := range p.queries {
		if strings.Contains(sql, "from \""+table+"\"") || strings.Contains(sql, "from "+table+" ") {
			count++
		}
	}
	return count
}

func (p *notificationBatchQueryProbe) displayCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	count := 0
	for _, sql := range p.queries {
		if strings.Contains(strings.SplitN(sql, " from ", 2)[0], "display_name") {
			count++
		}
	}
	return count
}

func seedNotificationBatchRows(t *testing.T, f intentFixture, count int, unique bool) []models.Notification {
	t.Helper()
	// Consent querying is disabled for this throughput fixture so the name-query
	// count has an exact, stable expectation. The privacy fixture below tests it.
	require.NoError(t, f.db.Where("key = ?", "legal.pd_consent_required").Delete(&models.SystemSetting{}).Error)
	require.NoError(t, f.db.Create(&models.SystemSetting{Key: "legal.pd_consent_required", Value: "false", Type: "bool"}).Error)
	objects := 1
	if unique {
		objects = count
	}
	actors := make([]models.User, objects)
	apps := make([]models.Application, objects)
	for i := 0; i < objects; i++ {
		name := fmt.Sprintf("BatchActorName%04d", i)
		actors[i] = models.User{Username: fmt.Sprintf("batch-actor-%04d", i), Password: "unused", OrganizationID: &f.orgID, TypeID: 1, IsActive: true, LastName: &name}
		number := fmt.Sprintf("BATCH-%04d", i)
		apps[i] = models.Application{OrganizationID: f.orgID, SenderUserID: f.approver.ID, ApplicationNumber: &number}
	}
	require.NoError(t, f.db.CreateInBatches(&actors, 100).Error)
	require.NoError(t, f.db.CreateInBatches(&apps, 100).Error)
	kind := services.NotificationTypeApplicationForwarded
	base := time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC)
	rows := make([]models.Notification, count)
	for i := range rows {
		object := 0
		if unique {
			object = i
		}
		ref := fmt.Sprintf(`{"v":1,"type":%q,"actor_id":%d,"application_id":%d,"facts":{}}`, kind, actors[object].ID, apps[object].ID)
		group := fmt.Sprintf("app:%d", apps[object].ID)
		rows[i] = models.Notification{UserID: f.approver.ID, Type: &kind, ProjectionRef: &ref, GroupKey: &group, Count: 1, CreatedAt: base.Add(time.Duration(i) * time.Second)}
	}
	require.NoError(t, f.db.CreateInBatches(&rows, 100).Error)
	return rows
}

func TestNotificationProjectionBulkQueryBudgetAndLegacyLength(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		count, uniqueObjects int
		unique               bool
	}{
		{"repeated_100", 100, 1, false},
		{"unique_1000", 1000, 1000, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := setupIntentFixture(t)
			stored := seedNotificationBatchRows(t, f, tc.count, tc.unique)
			probe := notificationBatchProbe(t, f.db)
			ctx := services.WithSessionIdentity(context.Background(), models.SessionIdentity{OwnerUserID: f.approver.ID, EffectiveUserID: f.approver.ID})
			rows, err := f.notifications.GetByUserID(ctx, f.approver.ID)
			require.NoError(t, err)
			require.Len(t, rows, tc.count, "legacy still returns the complete array")
			for i, row := range rows {
				require.Equal(t, stored[len(stored)-1-i].ID, row.ID, "bulk hydration must not reorder feed rows")
				require.NotNil(t, row.Data)
				require.NotNil(t, row.GroupKey)
			}
			total, applications, actors, names, variables := probe.counts()
			batches := (tc.uniqueObjects + 99) / 100
			require.Equal(t, batches, applications, "application access is one metadata query per bounded batch")
			require.Equal(t, batches, actors, "actor consent state is bulk-loaded")
			require.Equal(t, batches, probe.displayCount(), "permitted display masks use one bounded source query per batch")
			require.Equal(t, batches, names, "permitted names are bulk-loaded")
			require.LessOrEqual(t, total, 20+4*batches, "fixed policy overhead plus batch-count queries, never per row")
			require.LessOrEqual(t, variables, 110, "unique feeds must not create an unbounded IN query")
			probe.reset()
			page, totalRows, unread, err := f.notifications.GetByUserIDPaginated(ctx, f.approver.ID, 20, 0, "all")
			require.NoError(t, err)
			require.Len(t, page, 20)
			require.EqualValues(t, tc.count, totalRows)
			require.EqualValues(t, tc.count, unread)
			total, applications, actors, names, _ = probe.counts()
			require.Equal(t, 1, applications)
			require.Equal(t, 1, actors)
			require.Equal(t, 1, probe.displayCount())
			require.Equal(t, 1, names)
			require.LessOrEqual(t, total, 24, "a 20-row page must not issue 40-80 domain reads")
		})
	}
}

func TestNotificationProjectionBulkRelationsCacheMissingAndDeniedReferences(t *testing.T) {
	f := setupIntentFixture(t)
	app := f.createApplication(t, f.db)
	denied := models.Application{OrganizationID: f.orgID, SenderUserID: f.author.ID}
	require.NoError(t, f.db.Create(&denied).Error)
	supplement := models.ApplicationSupplement{ApplicationID: app.ID, Number: 1, Status: models.SupplementPending, CreatedByUserID: f.author.ID}
	require.NoError(t, f.db.Create(&supplement).Error)
	question := models.ApplicationQuestion{ApplicationID: app.ID, AuthorUserID: f.author.ID, Subject: "Private subject", Text: "Private question"}
	require.NoError(t, f.db.Create(&question).Error)
	answer := models.ApplicationAnswer{ApplicationID: app.ID, QuestionID: question.ID, AuthorUserID: f.author.ID, Text: "Private answer"}
	require.NoError(t, f.db.Create(&answer).Error)
	cases := []struct {
		kind    string
		fields  map[string]any
		visible bool
	}{
		{services.NotificationTypeApplicationSupplementReady, map[string]any{"application_id": app.ID, "supplement_id": supplement.ID}, true},
		{services.NotificationTypeApplicationAnswer, map[string]any{"application_id": app.ID, "question_id": question.ID, "answer_id": answer.ID, "actor_id": f.author.ID}, true},
		{services.NotificationTypeDirectoryResolved, map[string]any{"directory_kind": "organizations", "directory_id": f.orgID}, true},
		{services.NotificationTypeApplicationForwarded, map[string]any{"application_id": denied.ID, "actor_id": f.author.ID}, false},
		{services.NotificationTypeApplicationForwarded, map[string]any{"application_id": 2147483647, "actor_id": f.author.ID}, false},
		{services.NotificationTypeApplicationSupplementReady, map[string]any{"application_id": app.ID, "supplement_id": 2147483647}, false},
		{services.NotificationTypeApplicationQuestion, map[string]any{"application_id": app.ID, "question_id": 2147483647, "actor_id": f.author.ID}, false},
		{services.NotificationTypeApplicationAnswer, map[string]any{"application_id": app.ID, "question_id": question.ID, "answer_id": answer.ID, "actor_id": f.approver.ID}, false},
		{services.NotificationTypeDirectoryResolved, map[string]any{"directory_kind": "organizations", "directory_id": 2147483647}, false},
	}
	var stored []models.Notification
	var visibility []bool
	for i := 0; i < 120; i++ {
		for _, tc := range cases {
			fields := map[string]any{"v": 1, "type": tc.kind, "facts": map[string]any{}}
			for key, value := range tc.fields {
				fields[key] = value
			}
			encoded, err := json.Marshal(fields)
			require.NoError(t, err)
			ref, kind := string(encoded), tc.kind
			stored = append(stored, models.Notification{UserID: f.approver.ID, Type: &kind, ProjectionRef: &ref})
			visibility = append(visibility, tc.visible)
		}
	}
	require.NoError(t, f.db.CreateInBatches(&stored, 100).Error)
	wants := map[int]bool{}
	for i, row := range stored {
		wants[row.ID] = visibility[i]
	}
	probe := notificationBatchProbe(t, f.db)
	feed, err := f.notifications.GetByUserID(context.Background(), f.approver.ID)
	require.NoError(t, err)
	require.Len(t, feed, len(stored))
	for _, row := range feed {
		if wants[row.ID] {
			require.NotEqual(t, "Есть обновление", *row.Title)
		} else {
			require.Equal(t, "Есть обновление", *row.Title)
			require.Nil(t, row.Data)
			require.Nil(t, row.GroupKey)
		}
	}
	for _, table := range []string{"applications", "application_supplements", "application_questions", "application_answers", "organizations"} {
		require.Equal(t, 1, probe.tableCount(table), "%s must cache missing and denied results as well as existing rows", table)
	}
}

// Inspect the bound ID list before gorm executes a name SELECT. This works with
// qualified/unqualified table SQL, rather than assuming a callback table alias.
func notificationBatchBoundIDs(expression clause.Expression) []int {
	var ids []int
	switch value := expression.(type) {
	case clause.Where:
		for _, child := range value.Exprs {
			ids = append(ids, notificationBatchBoundIDs(child)...)
		}
	case clause.AndConditions:
		for _, child := range value.Exprs {
			ids = append(ids, notificationBatchBoundIDs(child)...)
		}
	case clause.OrConditions:
		for _, child := range value.Exprs {
			ids = append(ids, notificationBatchBoundIDs(child)...)
		}
	case clause.Expr:
		for _, bound := range value.Vars {
			switch argument := bound.(type) {
			case []int:
				ids = append(ids, argument...)
			case int:
				ids = append(ids, argument)
			}
		}
	}
	return ids
}

func TestNotificationProjectionBatchNeverSelectsRevokedOrMaskedActorNames(t *testing.T) {
	f := setupIntentFixture(t)
	require.NoError(t, f.db.Where("key = ?", "legal.pd_consent_required").Delete(&models.SystemSetting{}).Error)
	require.NoError(t, f.db.Create(&models.SystemSetting{Key: "legal.pd_consent_required", Value: "false", Type: "bool"}).Error)
	ctx := services.WithSessionIdentity(context.Background(), models.SessionIdentity{OwnerUserID: f.author.ID, EffectiveUserID: f.approver.ID})
	revokedName, maskedName, allowedName := "RevokedBatchPrivateName", "MaskedBatchPrivateName", "AllowedBatchName"
	display := "BatchPublicDisplayMask"
	require.NoError(t, f.db.Model(&models.User{}).Where("id = ?", f.author.ID).Updates(map[string]any{"last_name": revokedName, "is_banned": true, "is_active": false, "is_super_admin": true}).Error)
	revokedAt := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	require.NoError(t, f.db.Create(&models.PDConsent{UserID: f.author.ID, ConsentType: services.ConsentTypePDProcessing, Granted: false, GrantedAt: revokedAt.Add(-time.Hour), RevokedAt: &revokedAt}).Error)
	// An unrelated active transfer grant must not undo processing revocation.
	require.NoError(t, f.db.Create(&models.PDConsent{UserID: f.author.ID, ConsentType: "pd_transfer", Granted: true, GrantedAt: revokedAt.Add(time.Hour)}).Error)
	masked := models.User{Username: "batch-masked", Password: "unused", OrganizationID: &f.orgID, TypeID: 1, IsActive: true, LastName: &maskedName}
	allowed := models.User{Username: "batch-allowed", Password: "unused", OrganizationID: &f.orgID, TypeID: 1, IsActive: true, LastName: &allowedName}
	require.NoError(t, f.db.Create(&masked).Error)
	require.NoError(t, f.db.Create(&allowed).Error)
	// The revoked actor's display mask itself contains private name data. It must
	// never be selected, even though that actor is a super/admin-exempt account.
	require.NoError(t, f.db.Create(&models.ApplicationApprover{UserID: f.author.ID, DisplayName: &revokedName}).Error)
	require.NoError(t, f.db.Create(&models.ApplicationApprover{UserID: masked.ID, DisplayName: &display}).Error)
	app := models.Application{OrganizationID: f.orgID, SenderUserID: f.approver.ID}
	require.NoError(t, f.db.Create(&app).Error)
	kind := services.NotificationTypeApplicationForwarded
	rows := make([]models.Notification, 0, 120)
	for _, actorID := range []int{f.author.ID, masked.ID, allowed.ID} {
		for i := 0; i < 40; i++ {
			ref := fmt.Sprintf(`{"v":1,"type":%q,"actor_id":%d,"application_id":%d,"facts":{}}`, kind, actorID, app.ID)
			rows = append(rows, models.Notification{UserID: f.approver.ID, Type: &kind, ProjectionRef: &ref})
		}
	}
	require.NoError(t, f.db.CreateInBatches(&rows, 100).Error)
	forbidMasked := true
	forbidConsentActors := false
	nameReads, displayReads := 0, 0
	const callback = "notification_batch_private_name_guard"
	require.NoError(t, f.db.Callback().Query().Before("gorm:query").Register(callback, func(tx *gorm.DB) {
		selected := strings.Join(tx.Statement.Selects, ",")
		isName, isDisplay := strings.Contains(selected, "last_name"), strings.Contains(selected, "display_name")
		if !isName && !isDisplay {
			return
		}
		if isName {
			nameReads++
		}
		if isDisplay {
			displayReads++
		}
		ids := notificationBatchBoundIDs(tx.Statement.Clauses["WHERE"].Expression)
		if len(ids) == 0 {
			tx.AddError(errors.New("personal name/mask SELECT lacks an explicit allowed ID list"))
			return
		}
		for _, id := range ids {
			if id == f.author.ID || isName && forbidMasked && id == masked.ID || forbidConsentActors && (id == masked.ID || id == allowed.ID) {
				tx.AddError(errors.New("private actor entered name/mask SELECT before execution"))
			}
		}
	}))
	t.Cleanup(func() { f.db.Callback().Query().Remove(callback) })
	for _, super := range []bool{false, true} {
		forbidMasked = !super
		displaysBefore := displayReads
		require.NoError(t, f.db.Model(&models.User{}).Where("id = ?", f.approver.ID).Update("is_super_admin", super).Error)
		feed, err := f.notifications.GetByUserID(ctx, f.approver.ID)
		require.NoError(t, err, "forbidden IDs must be absent before name SQL executes")
		require.Len(t, feed, 120)
		body, err := json.Marshal(feed)
		require.NoError(t, err)
		require.NotContains(t, string(body), revokedName)
		require.Contains(t, string(body), "@"+f.author.Username)
		require.Contains(t, string(body), allowedName)
		if super {
			require.Contains(t, string(body), maskedName)
		} else {
			require.NotContains(t, string(body), maskedName)
			require.Contains(t, string(body), display)
		}
		if super {
			require.Equal(t, displaysBefore, displayReads, "verified effective super bypasses display selection")
		}
	}
	require.Positive(t, nameReads, "the SQL guard must observe real permitted name reads")
	require.Positive(t, displayReads, "the SQL guard must observe a real guarded display-mask source query")
	// Enabling the ordinary processing-consent gate also masks a never-granted
	// actor for a super viewer. An active transfer grant is still insufficient.
	require.NoError(t, f.db.Model(&models.SystemSetting{}).Where("key = ?", "legal.pd_consent_required").Update("value", "true").Error)
	require.NoError(t, f.db.Where("key = ?", "legal.pd_consent_text").Delete(&models.SystemSetting{}).Error)
	require.NoError(t, f.db.Create(&models.SystemSetting{Key: "legal.pd_consent_text", Value: "<p>Processing consent</p>", Type: "html"}).Error)
	require.NoError(t, f.db.Create(&models.PDConsent{UserID: allowed.ID, ConsentType: "pd_transfer", Granted: true, GrantedAt: revokedAt}).Error)
	require.NoError(t, f.db.Model(&models.User{}).Where("id = ?", f.approver.ID).Update("is_super_admin", false).Error)
	require.NoError(t, f.db.Model(&models.ApplicationApprover{}).Where("user_id = ?", masked.ID).Update("display_name", maskedName).Error)
	forbidConsentActors = true
	before, displaysBefore := nameReads, displayReads
	feed, err := f.notifications.GetByUserID(ctx, f.approver.ID)
	require.NoError(t, err)
	body, err := json.Marshal(feed)
	require.NoError(t, err)
	require.NotContains(t, string(body), revokedName)
	require.NotContains(t, string(body), maskedName)
	require.NotContains(t, string(body), allowedName)
	require.Contains(t, string(body), "@"+allowed.Username)
	require.Equal(t, before, nameReads, "all actors are masked/revoked before any raw-name SELECT")
	require.Equal(t, displaysBefore, displayReads, "consent-masked actors never enter a display-name source SELECT")
}
