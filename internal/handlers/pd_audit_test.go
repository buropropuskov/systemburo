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
	}, 5*time.Second, 50*time.Millisecond, "запись журнала ПДн для %s не появилась", path)
	return row
}

func TestPDAudit_WritesOnPersonalDataRequest(t *testing.T) {
	w := setupBlankWorld(t)
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
		}, 5*time.Second, 50*time.Millisecond, "отказ не попал в журнал")
		require.Equal(t, "attachment_blank", row.Resource)
	})

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
