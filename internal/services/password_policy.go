package services

import (
	"fmt"
	"net/http"
	"strings"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
)

// passwordSpecialChars — печатаемые ASCII-символы, не входящие в [A-Za-z0-9].
// Набор синхронизирован с фронтовым SPECIAL_CHARS (utils/passwordPolicy.js).
const passwordSpecialChars = "!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~"

// PolicyRule — результат проверки одного включённого требования политики.
type PolicyRule struct {
	Key   string
	Label string
	OK    bool
}

// EvaluatePassword проверяет пароль против политики и возвращает результат по
// каждому ВКЛЮЧЁННОМУ требованию (выключенные не попадают в список).
// Классы символов считаются по ASCII, чтобы совпадать с фронтом.
func EvaluatePassword(p models.PasswordPolicy, password string) []PolicyRule {
	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, r := range password {
		switch {
		case r >= 'A' && r <= 'Z':
			hasUpper = true
		case r >= 'a' && r <= 'z':
			hasLower = true
		case r >= '0' && r <= '9':
			hasDigit = true
		case strings.ContainsRune(passwordSpecialChars, r):
			hasSpecial = true
		}
	}
	hasLetter := hasUpper || hasLower
	length := len([]rune(password))

	rules := []PolicyRule{
		{Key: "min_length", Label: fmt.Sprintf("Минимум %d символов", p.MinLength), OK: length >= p.MinLength},
	}
	if p.RequireLetter {
		rules = append(rules, PolicyRule{Key: "letter", Label: "Хотя бы одна буква", OK: hasLetter})
	}
	if p.RequireUppercase {
		rules = append(rules, PolicyRule{Key: "uppercase", Label: "Хотя бы одна заглавная буква", OK: hasUpper})
	}
	if p.RequireLowercase {
		rules = append(rules, PolicyRule{Key: "lowercase", Label: "Хотя бы одна строчная буква", OK: hasLower})
	}
	if p.RequireDigit {
		rules = append(rules, PolicyRule{Key: "digit", Label: "Хотя бы одна цифра", OK: hasDigit})
	}
	if p.RequireSpecial {
		rules = append(rules, PolicyRule{Key: "special", Label: "Хотя бы один спецсимвол", OK: hasSpecial})
	}
	return rules
}

// ValidatePassword возвращает echo 400 с текстом первого нарушенного требования,
// либо nil. Вызывается ДО хеширования.
func ValidatePassword(p models.PasswordPolicy, password string) error {
	for _, r := range EvaluatePassword(p, password) {
		if !r.OK {
			return echo.NewHTTPError(http.StatusBadRequest, "Пароль не соответствует требованиям: "+r.Label)
		}
	}
	return nil
}
