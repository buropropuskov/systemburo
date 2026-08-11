package testutil

import (
	"systemburo/internal/services"
)

// hashTestPassword хеширует пароль тем же кодом, что и рабочий сервис.
// Собственной копии параметров Argon2id здесь намеренно нет: разойдясь с
// auth_service, она молча ломала бы вход тестовых учётных записей.
func hashTestPassword(password string) string {
	return services.HashPassword(password)
}
