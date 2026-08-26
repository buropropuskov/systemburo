package handlers_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Журнал доступа к персональным данным пишется при реальном обращении (#1472).
// Юнит на сверку путей живёт в internal/middleware; здесь проверяем сквозь весь стек:
// перечень адресов молча разошёлся с роутером, и поймать это можно только на живом запросе.

const pdAuditPassword = "pdauditpass_long_enough_for_login"

// waitPDAuditRow ждёт запись: middleware пишет в отдельной горутине, чтобы не держать ответ.
func waitPDAuditRow(t *testing.T, db *gorm.DB, path string) models.PDAuditLog {
	t.Helper()
	var row models.PDAuditLog
	require.Eventually(t, func() bool {
		return db.Where("path = ?", path).Order("id DESC").First(&row).Error == nil
	}, 3*time.Second, 20*time.Millisecond, "запись журнала ПДн для %s не появилась", path)
	return row
}

// pdAuditSection - секция общего набора TestBlankAccessAndTemplateRoutes: свой
// SetupTestApp приближал границу go test -timeout у пакета handlers в CI.
func pdAuditSection(t *testing.T, w blankWorld) {
	db := w.h.w.db

	t.Run("просмотр сотрудников вложения попадает в журнал", func(t *testing.T) {
		path := fmt.Sprintf("/attachments/%d/employees", w.attID)
		rec := testutil.GET(t, w.h.e, path, testutil.AuthHeader(w.h.userToken))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		row := waitPDAuditRow(t, db, "/api"+path)
		require.Equal(t, "attachment", row.Resource)
		require.Equal(t, "view", row.Action)
		require.Equal(t, "GET", row.Method)
		require.Equal(t, http.StatusOK, row.StatusCode)
		require.Equal(t, "userhttp", row.Username)
		require.NotNil(t, row.UserID, "рядом с именем пишем id учётки")
	})

	t.Run("выгрузка бланка считается доступом к персональным данным", func(t *testing.T) {
		path := fmt.Sprintf("/applications/%d/blank?attachment_id=%d", w.appID, w.attID)
		rec := testutil.GET(t, w.h.e, path, testutil.AuthHeader(w.h.userToken))
		require.Equal(t, http.StatusOK, rec.Code)

		row := waitPDAuditRow(t, db, fmt.Sprintf("/api/applications/%d/blank", w.appID))
		require.Equal(t, "attachment_blank", row.Resource)
		require.Equal(t, "view", row.Action)
	})

	t.Run("отказ в доступе тоже фиксируется", func(t *testing.T) {
		userTypeID := secUserTypeIDByCode(t, db, "user")
		outsider := testutil.RegisterAndLogin(t, w.h.e, "pdauditoutsider", pdAuditPassword, userTypeID, w.h.w.orgID, 0)

		path := fmt.Sprintf("/applications/%d/blank?attachment_id=%d", w.appID, w.attID)
		rec := testutil.GET(t, w.h.e, path, testutil.AuthHeader(outsider))
		require.Equal(t, http.StatusForbidden, rec.Code)

		var row models.PDAuditLog
		require.Eventually(t, func() bool {
			return db.Where("username = ? AND status_code = ?", "pdauditoutsider", http.StatusForbidden).
				Order("id DESC").First(&row).Error == nil
		}, 3*time.Second, 20*time.Millisecond, "отказ не попал в журнал")
		require.Equal(t, "attachment_blank", row.Resource)
	})

	t.Run("чтение журнала", func(t *testing.T) { listEndpointSection(t, w) })

	t.Run("обращение без персональных данных журнал не трогает", func(t *testing.T) {
		var before int64
		require.NoError(t, db.Model(&models.PDAuditLog{}).Count(&before).Error)

		rec := testutil.GET(t, w.h.e, "/citizenships", testutil.AuthHeader(w.h.userToken))
		require.Equal(t, http.StatusOK, rec.Code)

		// журналу нечего писать, но горутина соседнего запроса могла ещё не долететь -
		// поэтому проверяем именно отсутствие строки с этим путём
		var count int64
		require.NoError(t, db.Model(&models.PDAuditLog{}).Where("path = ?", "/api/citizenships").
			Count(&count).Error)
		require.Zero(t, count)
	})
}

// Чтение журнала (#1472): страница с фильтрами под правом page.admin.pd_audit.
// listEndpointSection живёт секцией внутри TestPDAudit: собственный SetupTestApp с
// CleanDB перебивал границу go test -timeout у пакета handlers в CI.
func listEndpointSection(t *testing.T, w blankWorld) {
	db := w.h.w.db

	// у автора обращений должно быть ФИО: на экране журнала показывается человек, а не логин
	require.NoError(t, db.Table("users").Where("username = ?", "userhttp").
		Updates(map[string]any{"last_name": "Петров", "first_name": "Игорь"}).Error)

	// наполняем журнал реальными обращениями, а не фикстурами
	empPath := fmt.Sprintf("/attachments/%d/employees", w.attID)
	require.Equal(t, http.StatusOK,
		testutil.GET(t, w.h.e, empPath, testutil.AuthHeader(w.h.userToken)).Code)
	blankPath := fmt.Sprintf("/applications/%d/blank?attachment_id=%d", w.appID, w.attID)
	require.Equal(t, http.StatusOK,
		testutil.GET(t, w.h.e, blankPath, testutil.AuthHeader(w.h.userToken)).Code)
	waitPDAuditRow(t, db, fmt.Sprintf("/api/applications/%d/blank", w.appID))

	userTypeID := secUserTypeIDByCode(t, db, "user")
	plain := testutil.RegisterAndLogin(t, w.h.e, "pdauditplain", pdAuditPassword, userTypeID, w.h.w.orgID, 0)

	t.Run("без права журнал недоступен", func(t *testing.T) {
		rec := testutil.GET(t, w.h.e, "/pd-audit", testutil.AuthHeader(plain))
		require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	})

	t.Run("админ видит записи", func(t *testing.T) {
		rec := testutil.GET(t, w.h.e, "/pd-audit", testutil.AuthHeader(w.h.adminToken))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		page := testutil.ParseResponse[models.PDAuditPageResponse](t, rec)
		require.NotZero(t, page.Total)
		require.NotEmpty(t, page.Items)
		require.Equal(t, "userhttp", page.Items[0].Username, "сортировка от свежих к старым")
		require.Equal(t, "Петров Игорь", page.Items[0].UserName, "рядом с логином отдаём ФИО")
	})

	t.Run("фильтр по ресурсу", func(t *testing.T) {
		rec := testutil.GET(t, w.h.e, "/pd-audit?resource=attachment_blank", testutil.AuthHeader(w.h.adminToken))
		require.Equal(t, http.StatusOK, rec.Code)
		page := testutil.ParseResponse[models.PDAuditPageResponse](t, rec)
		require.NotEmpty(t, page.Items)
		for _, it := range page.Items {
			require.Equal(t, "attachment_blank", it.Resource)
		}
	})

	t.Run("фильтр только отказы", func(t *testing.T) {
		outsider := testutil.RegisterAndLogin(t, w.h.e, "pdauditdenied", pdAuditPassword, userTypeID, w.h.w.orgID, 0)
		require.Equal(t, http.StatusForbidden,
			testutil.GET(t, w.h.e, blankPath, testutil.AuthHeader(outsider)).Code)
		require.Eventually(t, func() bool {
			var n int64
			db.Model(&models.PDAuditLog{}).Where("username = ?", "pdauditdenied").Count(&n)
			return n > 0
		}, 3*time.Second, 20*time.Millisecond)

		rec := testutil.GET(t, w.h.e, "/pd-audit?only_denied=true", testutil.AuthHeader(w.h.adminToken))
		require.Equal(t, http.StatusOK, rec.Code)
		page := testutil.ParseResponse[models.PDAuditPageResponse](t, rec)
		require.NotEmpty(t, page.Items)
		for _, it := range page.Items {
			require.GreaterOrEqual(t, it.StatusCode, http.StatusBadRequest)
		}
	})

	t.Run("страница ограничена лимитом", func(t *testing.T) {
		rec := testutil.GET(t, w.h.e, "/pd-audit?limit=1&page=1", testutil.AuthHeader(w.h.adminToken))
		require.Equal(t, http.StatusOK, rec.Code)
		page := testutil.ParseResponse[models.PDAuditPageResponse](t, rec)
		require.Len(t, page.Items, 1)
		require.Equal(t, 1, page.Limit)
		require.Greater(t, page.Total, int64(1))
	})
}
