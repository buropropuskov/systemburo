package models

// PasswordPolicy — настраиваемые требования к паролю. Значения берутся из
// system_settings (ключи password.*). Сериализуется как ответ
// GET /api/settings/password-policy.
type PasswordPolicy struct {
	MinLength        int  `json:"min_length"`
	RequireLetter    bool `json:"require_letter"`
	RequireUppercase bool `json:"require_uppercase"`
	RequireLowercase bool `json:"require_lowercase"`
	RequireDigit     bool `json:"require_digit"`
	RequireSpecial   bool `json:"require_special"`
}

// DefaultPasswordPolicy — дефолт "из коробки" (мин 8, буква + цифра).
// Должен совпадать с defaults в NewSettingsService и с фронтовым
// DEFAULT_PASSWORD_POLICY.
func DefaultPasswordPolicy() PasswordPolicy {
	return PasswordPolicy{
		MinLength:     8,
		RequireLetter: true,
		RequireDigit:  true,
	}
}
