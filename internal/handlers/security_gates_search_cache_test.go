package handlers_test

import (
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/require"
)

func TestSecurityGates_SearchDoesNotReuseOldVisibility(t *testing.T) {
	for _, change := range []string{"permission", "types", "publication"} {
		t.Run(change, func(t *testing.T) {
			w := newSecurityGateWorld(t)
			assignBaseRole(t, w.db, "security_gate_user")
			const query = "Cacheprobe"
			seedSearchEmployee(t, w.db, query, w.userID, w.orgID, "")
			news := models.News{Title: query, Description: searchStrPtr("test search cache")}
			require.NoError(t, w.db.Create(&news).Error)
			path := "/search?q=" + query
			header := testutil.AuthHeader(w.user)
			rec := testutil.GET(t, w.e, path, header)
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			initial := decodeSearch(t, rec.Body.String())
			for _, kind := range []string{"employees", "content"} {
				_, found := groupByType(initial, kind)
				require.True(t, found, "положительный контроль: %s", rec.Body.String())
			}
			denied := "employees"
			switch change {
			case "permission":
				testutil.DenyPermission(t, w.userID, services.KeyEntityEmployeesRead)
			case "types":
				path += "&types=content"
			case "publication":
				require.NoError(t, w.db.Model(&news).Update("is_active", false).Error)
				denied = "content"
			}
			rec = testutil.GET(t, w.e, path, header)
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			response := decodeSearch(t, rec.Body.String())
			_, found := groupByType(response, denied)
			require.False(t, found, "поиск вернул прежний запрещённый раздел: %s", rec.Body.String())
			allowed := "content"
			if change == "publication" {
				allowed = "employees"
			}
			_, found = groupByType(response, allowed)
			require.True(t, found, "доступный раздел должен продолжить работать: %s", rec.Body.String())
		})
	}
}
