package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"systemburo/internal/apperr"
	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestAPIBoundaryPackage2903(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	headers := testutil.AuthHeader(token)
	tableID := seedPeopleTable(t, db, "boundary_people", "Boundary people")
	encode := func(value any) string { raw, err := json.Marshal(value); require.NoError(t, err); return string(raw) }
	base := func() map[string]any {
		return map[string]any{"organization_id": td.OrgID, "company_id": td.CompanyID, "table_id": tableID,
			"entry_date_from": "2028-02-28", "entry_date_to": "2030-01-01", "entry_time_from": "08:00", "entry_time_to": "18:00",
			"employees": []map[string]any{{"last_name": "Boundary", "first_name": "Person", "citizenship_id": 0, "target_tables": []int{tableID, tableID}}}}
	}
	counts := func() []int64 {
		result := make([]int64, 4)
		for i, table := range []string{"attachments", "employees", "employee_target_tables", "audit_log"} {
			require.NoError(t, db.Table(table).Count(&result[i]).Error)
		}
		return result
	}
	t.Run("valid_optional_citizenship_and_duplicate_tables", func(t *testing.T) {
		rec := testutil.POST(t, e, "/employees/manual", encode(base()), headers)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		response := testutil.ParseResponse[services.ManualEmployeeResponse](t, rec)
		var links int64
		require.NoError(t, db.Model(&models.EmployeeTargetTable{}).Where("employee_id = ?", response.EmployeeIDs[0]).Count(&links).Error)
		require.EqualValues(t, 1, links)
	})
	for _, field := range []string{"organization_id", "company_id", "table_id", "citizenship_id", "target_tables"} {
		t.Run("missing_"+field, func(t *testing.T) {
			body := base()
			employees := body["employees"].([]map[string]any)
			switch field {
			case "citizenship_id":
				employees[0][field] = 1000000000
			case "target_tables":
				employees[0][field] = []int{tableID, 1000000000}
			default:
				body[field] = 1000000000
			}
			before := counts()
			rec := testutil.POST(t, e, "/employees/manual", encode(body), headers)
			require.Equal(t, http.StatusUnprocessableEntity, rec.Code, rec.Body.String())
			require.Equal(t, before, counts(), "a rejected reference must not persist part of the operation")
		})
	}
	t.Run("mixed_batch_is_atomic", func(t *testing.T) {
		body := base()
		body["employees"] = append(body["employees"].([]map[string]any), map[string]any{"last_name": "Second", "first_name": "Person", "citizenship_id": 1000000000})
		before := counts()
		rec := testutil.POST(t, e, "/employees/manual", encode(body), headers)
		require.Equal(t, http.StatusUnprocessableEntity, rec.Code, rec.Body.String())
		require.Equal(t, before, counts())
	})
	for _, field := range []string{"entry_date_from", "entry_date_to", "entry_time_from", "entry_time_to", "nul"} {
		t.Run("invalid_"+field, func(t *testing.T) {
			body := base()
			if field == "nul" {
				body["employees"].([]map[string]any)[0]["last_name"] = "Boundary\x00Person"
			} else {
				body[field] = "invalid"
			}
			before := counts()
			rec := testutil.POST(t, e, "/employees/manual", encode(body), headers)
			require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
			require.Equal(t, before, counts())
		})
	}
	t.Run("query_text_and_literal_percent", func(t *testing.T) {
		for _, path := range []string{"/search?q=abc%00suffix", "/cars/check-active?car_number=abc%00suffix", "/unique-cars/lookup?number=abc%00suffix", "/unique-employees/lookup?last_name=abc%00suffix&first_name=Name"} {
			rec := testutil.GET(t, e, path, headers)
			require.Equal(t, http.StatusBadRequest, rec.Code, path+rec.Body.String())
		}
		rec := testutil.GET(t, e, "/search?q=abc%2500suffix", headers)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	})
	t.Run("absent_group_and_user", func(t *testing.T) {
		require.Equal(t, http.StatusNotFound, testutil.GET(t, e, "/permission-groups/1000000000", headers).Code)
		var user models.User
		require.NoError(t, db.Where("is_super_admin = ?", true).First(&user).Error)
		rec := testutil.POST(t, e, fmt.Sprintf("/users/%d/permission-groups/1000000000", user.ID), `{}`, headers)
		require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
		group := models.PermissionGroup{Name: "Boundary group"}
		require.NoError(t, db.Create(&group).Error)
		rec = testutil.POST(t, e, fmt.Sprintf("/users/1000000000/permission-groups/%d", group.ID), `{}`, headers)
		require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
		resolver := services.NewPermissionResolver(db)
		svc := services.NewPermissionGroupService(db, resolver)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := svc.Get(ctx, group.ID)
		require.ErrorIs(t, err, context.Canceled)
		var typed *apperr.Error
		require.False(t, errors.As(err, &typed))
		var assignments int64
		require.NoError(t, db.Model(&models.UserGroup{}).Where("group_id = ?", group.ID).Count(&assignments).Error)
		require.Zero(t, assignments)
	})
	t.Run("empty_delete_filter_keeps_rows", func(t *testing.T) {
		denial := models.AccessDenial{Resource: "boundary-resource", Reason: models.DenialReasonPermission}
		require.NoError(t, db.Create(&denial).Error)
		for _, query := range []string{"", "?page=1&limit=50", "?resource=%20%20&reason=%20"} {
			rec := testutil.DELETE(t, e, "/access-denials"+query, headers)
			require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
			require.NoError(t, db.First(&models.AccessDenial{}, denial.ID).Error)
		}
		rec := testutil.DELETE(t, e, "/access-denials?resource=boundary-resource", headers)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		require.ErrorIs(t, db.First(&models.AccessDenial{}, denial.ID).Error, gorm.ErrRecordNotFound)
	})
	t.Run("duplicate_roles_are_conflicts", func(t *testing.T) {
		body := `{"code":"boundary_concurrent","name":"Boundary concurrent"}`
		statuses := make(chan int, 2)
		var wg sync.WaitGroup
		for range 2 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				req := httptest.NewRequest(http.MethodPost, "/api/roles", strings.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer "+token)
				rec := httptest.NewRecorder()
				e.ServeHTTP(rec, req)
				statuses <- rec.Code
			}()
		}
		wg.Wait()
		close(statuses)
		var actual []int
		for status := range statuses {
			actual = append(actual, status)
		}
		require.ElementsMatch(t, []int{http.StatusCreated, http.StatusConflict}, actual)
		var total int64
		require.NoError(t, db.Model(&models.Role{}).Where("code = ?", "boundary_concurrent").Count(&total).Error)
		require.EqualValues(t, 1, total)
	})
	t.Run("assignment_is_idempotent_during_concurrency", func(t *testing.T) {
		var user models.User
		require.NoError(t, db.Where("is_super_admin = ?", true).First(&user).Error)
		group := models.PermissionGroup{Name: "Concurrent boundary group"}
		require.NoError(t, db.Create(&group).Error)
		svc := services.NewPermissionGroupService(db, services.NewPermissionResolver(db))
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		results := make(chan error, 2)
		for range 2 {
			go func() { results <- svc.AssignToUser(ctx, user.ID, group.ID, user.ID) }()
		}
		require.NoError(t, <-results)
		require.NoError(t, <-results)
		var total int64
		require.NoError(t, db.Model(&models.UserGroup{}).Where("user_id = ? AND group_id = ?", user.ID, group.ID).Count(&total).Error)
		require.EqualValues(t, 1, total)
	})
	t.Run("assignment_and_deletion_leave_no_dangling_link", func(t *testing.T) {
		var user models.User
		require.NoError(t, db.Where("is_super_admin = ?", true).First(&user).Error)
		group := models.PermissionGroup{Name: "Boundary deletion group"}
		require.NoError(t, db.Create(&group).Error)
		svc := services.NewPermissionGroupService(db, services.NewPermissionResolver(db))
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		started := make(chan struct{})
		assigned, deleted := make(chan error, 1), make(chan error, 1)
		go func() { <-started; assigned <- svc.AssignToUser(ctx, user.ID, group.ID, user.ID) }()
		go func() { <-started; deleted <- svc.Delete(ctx, group.ID) }()
		close(started)
		assignErr, deleteErr := <-assigned, <-deleted
		require.NoError(t, deleteErr)
		if assignErr != nil {
			var app *apperr.Error
			require.ErrorAs(t, assignErr, &app)
			require.Equal(t, http.StatusNotFound, app.Code)
		}
		var total int64
		require.NoError(t, db.Model(&models.UserGroup{}).Where("group_id = ?", group.ID).Count(&total).Error)
		require.Zero(t, total)
	})
	t.Run("credential_bytes_are_not_database_text", func(t *testing.T) {
		password := "valid\x00password2903"
		testutil.RegisterUser(t, e, "boundary_password", password, 1, td.OrgID, td.CompanyID)
		rec := testutil.POST(t, e, "/login", encode(map[string]string{"username": "boundary_password", "password": password}), nil)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	})
}
