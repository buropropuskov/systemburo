package services

import (
	"context"
	"net/http"
	"net/mail"
	"strings"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// normalizeUserEmail приводит адрес к виду, в котором он хранится: без окружающих
// пробелов. Регистр не трогаем - локальная часть адреса регистрозависима по RFC,
// и хотя на практике все крупные почтовые службы её схлопывают, приводить чужой
// адрес к нижнему регистру система не вправе.
func normalizeUserEmail(raw string) string {
	return strings.TrimSpace(raw)
}

// validateUserEmail проверяет формат адреса и его свободность.
//
// До этого адрес работника не проверялся вообще: значение из формы уходило в базу
// как есть. С плановой рассылкой паролей (#1905) это перестало быть безобидным -
// `ivanov@` означает пароль, который никуда не доставлен, а один ящик на двоих -
// пароль соседа в чужой почте.
//
// Пустой адрес допустим: почта не обязательна, работник без неё просто не попадает
// в плановую смену, и администратор видит это в отчёте.
func validateUserEmail(ctx context.Context, db *gorm.DB, email string, excludeUserID int) (string, error) {
	normalized := normalizeUserEmail(email)
	if normalized == "" {
		return "", nil
	}

	addr, err := mail.ParseAddress(normalized)
	if err != nil {
		return "", echo.NewHTTPError(http.StatusBadRequest,
			"Некорректный адрес почты: "+normalized)
	}
	// ParseAddress принимает и форму «Имя <ящик@домен>»; в карточке нужен голый
	// адрес, иначе он не совпадёт сам с собой при проверке на дубль.
	if addr.Address != normalized {
		return "", echo.NewHTTPError(http.StatusBadRequest,
			"Укажите только адрес почты, без имени получателя: "+normalized)
	}

	q := db.WithContext(ctx).Table("users").Where("LOWER(email) = LOWER(?)", normalized)
	if excludeUserID > 0 {
		q = q.Where("id <> ?", excludeUserID)
	}
	var taken int64
	if err := q.Count(&taken).Error; err != nil {
		return "", echo.NewHTTPError(http.StatusInternalServerError, "Error checking email")
	}
	if taken > 0 {
		// Сравнение регистронезависимое намеренно: два ящика, различающиеся только
		// регистром, на всех почтовых службах ведут в один и тот же ящик, и пароль
		// одного работника пришёл бы другому.
		return "", echo.NewHTTPError(http.StatusBadRequest,
			"Этот адрес почты уже указан у другого работника: "+normalized)
	}
	return normalized, nil
}
