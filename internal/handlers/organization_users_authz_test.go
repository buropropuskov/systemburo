package handlers_test

// Состав ответственных организации и компании менялся без единого гейта: PUT
// /organizations/:id/users и /companies/:id/users стояли в роутере без middleware,
// пока соседи по той же сущности были закрыты (#1982).
//
// Это не косметика. Метод стирает organization_users целиком и пересобирает набор из
// тела запроса вместе с флагом required_approval, а по нему approver_service.IsReviewer
// решает, согласующий человек или нет, и application_service тянет ответственных в
// новые заявки организации. То есть любой работник вписывал себя согласующим чужой
// организации и получал видимость её заявок - повышение прав, а не правка справочника.

import (
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// countResponsibleUsers считает записи состава - по ним видно, прошла запись или нет.
func countResponsibleUsers(t *testing.T, db *gorm.DB, table string, entityID int) int {
	t.Helper()
	var n int
	column := "organization_id"
	if table == "companies_users" {
		column = "company_id"
	}
	require.NoError(t, db.Raw(
		fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s = ?", table, column), entityID).
		Scan(&n).Error)
	return n
}

func TestOrganizationAndCompanyUsers_WriteRequiresPermission(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	// Чужак: обычный работник без единого админского признака.
	testutil.RegisterUser(t, e, "orgusr_outsider", "password123", 1, td.OrgID, td.CompanyID)
	assignBaseRole(t, db, "orgusr_outsider")

	testutil.RegisterUser(t, e, "orgusr_admin", "password123", 1, td.OrgID, td.CompanyID)
	assignBaseRole(t, db, "orgusr_admin")
	require.NoError(t, db.Table("users").Where("username = ?", "orgusr_admin").
		Update("is_admin", true).Error)

	// Тело, которым чужак делает согласующим самого себя.
	selfApproval := `{"users":[{"username":"orgusr_outsider","is_primary":true,"required_approval":true}]}`

	t.Run("обычный пользователь не переписывает состав организации", func(t *testing.T) {
		token, _ := testutil.LoginUser(t, e, "orgusr_outsider", "password123")
		rec := testutil.PUT(t, e, fmt.Sprintf("/organizations/%d/users", td.OrgID), selfApproval,
			testutil.AuthHeader(token))

		require.Equal(t, http.StatusForbidden, rec.Code, "тело: %s", rec.Body.String())
		assert.Zero(t, countResponsibleUsers(t, db, "organization_users", td.OrgID),
			"отказ должен быть до записи: состав организации остался нетронутым")
	})

	t.Run("обычный пользователь не переписывает состав компании", func(t *testing.T) {
		token, _ := testutil.LoginUser(t, e, "orgusr_outsider", "password123")
		rec := testutil.PUT(t, e, fmt.Sprintf("/companies/%d/users", td.CompanyID), selfApproval,
			testutil.AuthHeader(token))

		require.Equal(t, http.StatusForbidden, rec.Code, "тело: %s", rec.Body.String())
		assert.Zero(t, countResponsibleUsers(t, db, "companies_users", td.CompanyID),
			"отказ должен быть до записи: состав компании остался нетронутым")
	})

	// Обратная сторона гейта: администратор состав по-прежнему ведёт, иначе закрытие
	// дыры молча отключило бы саму функцию.
	t.Run("администратор ведёт состав организации и компании", func(t *testing.T) {
		token, _ := testutil.LoginUser(t, e, "orgusr_admin", "password123")
		body := `{"users":[{"username":"orgusr_outsider","is_primary":true,"required_approval":true}]}`

		rec := testutil.PUT(t, e, fmt.Sprintf("/organizations/%d/users", td.OrgID), body,
			testutil.AuthHeader(token))
		require.Equal(t, http.StatusOK, rec.Code, "тело: %s", rec.Body.String())
		assert.Equal(t, 1, countResponsibleUsers(t, db, "organization_users", td.OrgID))

		rec = testutil.PUT(t, e, fmt.Sprintf("/companies/%d/users", td.CompanyID), body,
			testutil.AuthHeader(token))
		require.Equal(t, http.StatusOK, rec.Code, "тело: %s", rec.Body.String())
		assert.Equal(t, 1, countResponsibleUsers(t, db, "companies_users", td.CompanyID))
	})
}
