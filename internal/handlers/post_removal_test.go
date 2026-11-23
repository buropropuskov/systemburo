package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPostRemoval_ScopeAndAtomicSelection(t *testing.T) {
	for _, kind := range []string{"cars", "employees"} {
		t.Run(kind, func(t *testing.T) {
			e, db, cleanup := testutil.SetupTestApp(t)
			defer cleanup()
			testutil.CleanDB(t, db)
			td := testutil.SeedTestData(t, db)
			fx := seedPostGateFixture(t, db)
			id, linkTable, column, tableType, firstName := fx.carID, "car_target_tables", "car_id", "cars", fx.tableName
			if kind == "employees" {
				id, linkTable, column, tableType, firstName = fx.empID, "employee_target_tables", "employee_id", "people", "post_gate_people"
			}
			var first models.SystemTable
			require.NoError(t, db.Where("name = ?", firstName).First(&first).Error)
			second := models.SystemTable{Name: "removal_second", TableType: tableType, IsActive: true}
			third := models.SystemTable{Name: "removal_third", TableType: tableType, IsActive: true}
			for _, post := range []*models.SystemTable{&second, &third} {
				require.NoError(t, db.Create(post).Error)
				require.NoError(t, db.Exec(fmt.Sprintf("INSERT INTO %s (%s, table_id) VALUES (?, ?)", linkTable, column), id, post.ID).Error)
			}
			bindings := func() []int {
				var ids []int
				require.NoError(t, db.Table(linkTable).Where(column+" = ?", id).Order("table_id").Pluck("table_id", &ids).Error)
				return ids
			}
			guard := testutil.RegisterAndLogin(t, e, "removal_guard", "pass123", 1, td.OrgID, td.CompanyID)
			var guardID int
			require.NoError(t, db.Table("users").Select("id").Where("username = ?", "removal_guard").Scan(&guardID).Error)
			testutil.GrantPermission(t, guardID, "table."+first.Name+".delete")
			gh := testutil.AuthHeader(guard)
			path := "/" + kind + "/bulk/unbind-table"
			for _, body := range []string{
				`{`, `{}`, `{"table_ids":[0]}`, `{"table_ids":[-1]}`,
				fmt.Sprintf(`{"table_id":%d,"table_ids":[%d]}`, first.ID, first.ID),
				fmt.Sprintf(`{"table_id":%d}`, 999999),
			} {
				require.Equal(t, http.StatusBadRequest, testutil.POST(t, e, path, body, gh).Code)
			}
			rec := testutil.PUT(t, e, fmt.Sprintf("/%s/%d/deactivate", kind, id), fmt.Sprintf(`{"status":0,"table_id":%d}`, first.ID), gh)
			require.Equal(t, http.StatusForbidden, rec.Code, "legacy global action must not accept one-post permission")
			for _, body := range []string{
				fmt.Sprintf(`{"ids":[%d],"table_id":%d}`, id, second.ID),
				fmt.Sprintf(`{"ids":[%d],"table_ids":[%d,%d]}`, id, first.ID, second.ID),
				fmt.Sprintf(`{"ids":[%d],"table_ids":[%d],"all_tables":true}`, id, first.ID),
			} {
				require.Equal(t, http.StatusForbidden, testutil.POST(t, e, path, body, gh).Code)
			}
			require.Len(t, bindings(), 3)
			rec = testutil.POST(t, e, path, fmt.Sprintf(`{"ids":[%d],"table_id":%d}`, id, first.ID), gh)
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			assert.ElementsMatch(t, []int{second.ID, third.ID}, bindings())
			assert.Equal(t, 1, elementStatus(t, db, kind, id))
			rec = testutil.POST(t, e, path, fmt.Sprintf(`{"ids":[%d],"table_id":%d}`, id, first.ID), gh)
			require.Equal(t, http.StatusForbidden, rec.Code, "repeat must not remove another binding or disclose its label")
			assert.ElementsMatch(t, []int{second.ID, third.ID}, bindings())
			admin := testutil.AuthHeader(testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID))
			// One stale selected binding must roll back the valid deletion too.
			rec = testutil.POST(t, e, path, fmt.Sprintf(`{"ids":[%d],"table_ids":[%d,%d]}`, id, second.ID, first.ID), admin)
			require.Equal(t, http.StatusMultiStatus, rec.Code, rec.Body.String())
			assert.ElementsMatch(t, []int{second.ID, third.ID}, bindings())
			// "All" is rejected when the confirmed snapshot misses a post.
			rec = testutil.POST(t, e, path, fmt.Sprintf(`{"ids":[%d],"table_ids":[%d],"all_tables":true}`, id, second.ID), admin)
			require.Equal(t, http.StatusMultiStatus, rec.Code)
			assert.Len(t, bindings(), 2)
			fourth := models.SystemTable{Name: "removal_fourth", TableType: tableType, IsActive: true}
			require.NoError(t, db.Create(&fourth).Error)
			require.NoError(t, db.Exec(fmt.Sprintf("INSERT INTO %s (%s, table_id) VALUES (?, ?)", linkTable, column), id, fourth.ID).Error)
			rec = testutil.POST(t, e, path, fmt.Sprintf(`{"ids":[%d],"table_ids":[%d,%d]}`, id, second.ID, third.ID), admin)
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			assert.Equal(t, []int{fourth.ID}, bindings(), "selected removal must preserve the unselected post")
			assert.Equal(t, 1, elementStatus(t, db, kind, id))
			fifth := models.SystemTable{Name: "removal_fifth", TableType: tableType, IsActive: true}
			require.NoError(t, db.Create(&fifth).Error)
			require.NoError(t, db.Exec(fmt.Sprintf("INSERT INTO %s (%s, table_id) VALUES (?, ?)", linkTable, column), id, fifth.ID).Error)
			// Simultaneous confirmations must create only one set of history events.
			start, results := make(chan struct{}), make(chan int, 2)
			for range 2 {
				go func() {
					<-start
					response := testutil.POST(t, e, path, fmt.Sprintf(`{"ids":[%d],"table_ids":[%d,%d],"all_tables":true}`, id, fourth.ID, fifth.ID), admin)
					results <- response.Code
				}()
			}
			close(start)
			assert.ElementsMatch(t, []int{http.StatusOK, http.StatusMultiStatus}, []int{<-results, <-results})
			assert.Empty(t, bindings())
			assert.Equal(t, 0, elementStatus(t, db, kind, id))
			var records int64
			require.NoError(t, db.Table(kind).Where("id = ?", id).Count(&records).Error)
			assert.EqualValues(t, 1, records, "record must be retained")
			var history int64
			entity := models.AuditEntityCar
			if kind == "employees" {
				entity = models.AuditEntityEmployee
			}
			require.NoError(t, db.Model(&models.AuditLog{}).Where("entity_type = ? AND entity_id = ? AND action = ?", entity, id, models.AuditActionUnboundFromTable).Count(&history).Error)
			assert.EqualValues(t, 5, history, "exactly one event per removed binding")
		})
	}
}
