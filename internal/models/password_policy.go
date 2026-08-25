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

	// --- Плановая смена паролей (#1909). Живут в тех же system_settings и в том
	// же ответе: форма смены пароля показывает человеку не только требования к
	// символам, но и то, что пароль придётся менять раз в N суток. ---

	// RotationEnabled - включена ли плановая смена.
	RotationEnabled bool `json:"rotation_enabled"`
	// RotationDays - через сколько суток пароль истекает.
	RotationDays int `json:"rotation_days"`
	// RotationNotifyDaysBefore - за сколько суток предупреждать работника. 0 -
	// не предупреждать вовсе.
	RotationNotifyDaysBefore int `json:"rotation_notify_days_before"`
	// ForceChangeOnNextLogin - требовать ли задать свой пароль при первом входе
	// после плановой смены. Пароль уходит письмом открытым текстом, и это
	// единственное, что ограничивает срок его жизни в чужом почтовом ящике.
	ForceChangeOnNextLogin bool `json:"force_change_on_next_login"`
}

// Границы периодичности плановой смены. Верхняя взята из приказа ФСТЭК России
// N 21: для ИСПДн смена пароля требуется не реже чем раз в 120 суток.
const (
	MinRotationDays = 30
	MaxRotationDays = 120
	// MaxRotationNotifyDaysBefore - дальше предупреждать бессмысленно: человек
	// забудет о письме раньше, чем пароль истечёт.
	MaxRotationNotifyDaysBefore = 30

	DefaultRotationDays             = 90
	DefaultRotationNotifyDaysBefore = 7
)

// DefaultPasswordPolicy — дефолт "из коробки" (мин 8, буква + цифра).
// Должен совпадать с defaults в NewSettingsService и с фронтовым
// DEFAULT_PASSWORD_POLICY.
func DefaultPasswordPolicy() PasswordPolicy {
	return PasswordPolicy{
		MinLength:     8,
		RequireLetter: true,
		RequireDigit:  true,
		// Плановая смена по умолчанию выключена: включать её должен человек,
		// осознанно и после настройки почты.
		RotationEnabled:          false,
		RotationDays:             DefaultRotationDays,
		RotationNotifyDaysBefore: DefaultRotationNotifyDaysBefore,
		ForceChangeOnNextLogin:   true,
	}
}
