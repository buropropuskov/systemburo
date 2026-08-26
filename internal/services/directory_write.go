package services

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// directoryWriteError переводит ошибку записи наименования справочника в ответ
// пользователю.
//
// Дубль сервисы ищут запросом перед записью, а между проверкой и записью блокировки нет:
// два админа, правящих одно наименование одновременно, проходят проверку оба, и partial
// unique index по ключу дедупликации (#1437) отбивает второго. Без этого перевода второй
// получал бы 500 на ровном месте, хотя причина понятна и объясняется теми же словами, что
// и при проверке. Сообщение приходит от вызывающего: у создания, переименования и
// восстановления оно разное.
func directoryWriteError(err error, duplicateMsg, failureMsg string) error {
	if isUniqueViolation(err) {
		return echo.NewHTTPError(http.StatusBadRequest, duplicateMsg)
	}
	return echo.NewHTTPError(http.StatusInternalServerError, failureMsg)
}
