package handlers_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"systemburo/internal/apperr"
	"systemburo/internal/handlers"
	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

func TestAttachmentBlank_OpenErrorsKeepPrivateCause(t *testing.T) {
	world := setupBlankWorld(t)
	db := world.h.w.db
	var attachment models.Attachment
	require.NoError(t, db.First(&attachment, world.attID).Error)
	require.NotNil(t, attachment.UniqueAttachmentID)
	uniqueID := *attachment.UniqueAttachmentID

	for _, malformed := range []bool{false, true} {
		name := "missing"
		if malformed {
			name = "malformed"
		}
		t.Run(name, func(t *testing.T) {
			privatePath := filepath.Join(t.TempDir(), "private-template-marker.xlsx")
			if malformed {
				require.NoError(t, os.WriteFile(privatePath, []byte("invalid spreadsheet"), 0600))
			}
			require.NoError(t, db.Model(&models.AttachmentTemplate{}).
				Where("unique_attachment_id = ?", uniqueID).Update("file_path", privatePath).Error)

			for _, empty := range []bool{false, true} {
				service := services.NewAttachmentBlankService(db)
				var err error
				if empty {
					_, _, err = service.GenerateEmptyBlank(context.Background(), uniqueID)
				} else {
					_, _, err = service.GenerateBlank(context.Background(), world.appID, world.attID, services.BlankOptions{})
				}
				var domain *apperr.Error
				require.ErrorAs(t, err, &domain)
				require.Equal(t, http.StatusInternalServerError, domain.Code)
				require.Equal(t, "Не удалось открыть шаблон", domain.Message)
				require.NotNil(t, errors.Unwrap(domain))

				e := echo.New()
				rec := httptest.NewRecorder()
				c := e.NewContext(httptest.NewRequest(http.MethodGet, "/blank", nil), rec)
				c.SetPath("/applications/:id/blank")
				handlers.CustomHTTPErrorHandler(err, c)
				require.Equal(t, http.StatusInternalServerError, rec.Code)
				require.Contains(t, rec.Body.String(), "Не удалось открыть шаблон")
				require.NotContains(t, rec.Body.String(), "private-template-marker")
				require.NotContains(t, rec.Body.String(), privatePath)
			}
		})
	}
}
