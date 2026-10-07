package services

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v4"
)

// TicketOwnerStatus проверяет текущее состояние владельца одноразового билета.
// На публичных входах используется BanCheckService с TTL 0: выдача билета
// не должна сохранять доступ после блокировки или архивирования.
type TicketOwnerStatus interface {
	Status(context.Context, int) (banned, active bool, err error)
}

// RequireActiveTicketOwner вызывается после consume, до чтения файлов или
// открытия потока. Ошибка проверки закрывает доступ, а билет остаётся погашенным.
func RequireActiveTicketOwner(ctx context.Context, userID int, status TicketOwnerStatus) error {
	if status == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "Проверка учётной записи недоступна")
	}
	banned, active, err := status.Status(ctx, userID)
	if err != nil {
		slog.Warn("ticket_owner: status lookup failed, access denied", "user_id", userID, "error", err)
		return echo.NewHTTPError(http.StatusServiceUnavailable, "Проверка учётной записи недоступна")
	}
	if banned {
		return echo.NewHTTPError(http.StatusForbidden, "Учётная запись заблокирована")
	}
	if !active {
		return echo.NewHTTPError(http.StatusForbidden, "Учётная запись отключена")
	}
	return nil
}
