package handlers_test

import (
	"net/http"
	"testing"

	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBugReport_Unauthorized(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	body := `{"bug_hash":"abc12345","route":"POST /applications","http_status":500,"message":"Internal Server Error"}`
	rec := testutil.POST(t, e, "/bug-report", body, nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestBugReport_Submit_Ok(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "buguser", "password123", 1, td.OrgID, td.CompanyID)
	body := `{"bug_hash":"abc12345","route":"POST /applications","http_status":500,"message":"Internal Server Error"}`

	rec := testutil.POST(t, e, "/bug-report", body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)

	data := testutil.ParseMap(t, rec)
	assert.Equal(t, "abc12345", data["bug_hash"])
	assert.Equal(t, float64(500), data["http_status"])
}

func TestBugReport_Submit_Duplicate_409(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "duper", "password123", 1, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)
	body := `{"bug_hash":"dup123ab","route":"GET /news","http_status":500,"message":"boom"}`

	rec := testutil.POST(t, e, "/bug-report", body, h)
	require.Equal(t, http.StatusOK, rec.Code)

	// Второй раз тот же bug_hash - 409
	rec2 := testutil.POST(t, e, "/bug-report", body, h)
	assert.Equal(t, http.StatusConflict, rec2.Code)
}

func TestBugReport_Submit_RateLimit_429(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "rateuser", "password123", 1, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	// Лимит - 3 репорта за 5 минут. Четвёртый уходит в 429.
	hashes := []string{"h1234567", "h2345678", "h3456789"}
	for _, hx := range hashes {
		body := `{"bug_hash":"` + hx + `","route":"/x","http_status":500,"message":"x"}`
		rec := testutil.POST(t, e, "/bug-report", body, h)
		require.Equal(t, http.StatusOK, rec.Code, "hash %s", hx)
	}
	body4 := `{"bug_hash":"h4567890","route":"/x","http_status":500,"message":"x"}`
	rec := testutil.POST(t, e, "/bug-report", body4, h)
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
}

func TestBugReport_Submit_InvalidPayload_400(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "badpayload", "password123", 1, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	// слишком короткий bug_hash (меньше 8)
	rec := testutil.POST(t, e, "/bug-report", `{"bug_hash":"ab","route":"/x","http_status":500}`, h)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	// невалидный http_status (не в 4xx/5xx)
	rec = testutil.POST(t, e, "/bug-report", `{"bug_hash":"validhash","route":"/x","http_status":200}`, h)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
