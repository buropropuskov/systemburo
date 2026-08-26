package services

import (
	"testing"

	"systemburo/internal/models"

	"github.com/stretchr/testify/assert"
)

func TestEvaluatePassword(t *testing.T) {
	t.Parallel()
	full := models.PasswordPolicy{MinLength: 8, RequireLetter: true, RequireUppercase: true, RequireLowercase: true, RequireDigit: true, RequireSpecial: true}

	tests := []struct {
		name      string
		policy    models.PasswordPolicy
		password  string
		wantValid bool
	}{
		{"default ok", models.DefaultPasswordPolicy(), "password1", true},
		{"default too short", models.DefaultPasswordPolicy(), "pass1", false},
		{"default no digit", models.DefaultPasswordPolicy(), "passwordonly", false},
		{"default no letter", models.DefaultPasswordPolicy(), "12345678", false},
		{"full ok", full, "Passw0rd!", true},
		{"full no special", full, "Passw0rdd", false},
		{"full no upper", full, "passw0rd!", false},
		{"length counts runes", models.PasswordPolicy{MinLength: 8}, "абвгдежз", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rules := EvaluatePassword(tt.policy, tt.password)
			valid := true
			for _, r := range rules {
				if !r.OK {
					valid = false
				}
			}
			assert.Equal(t, tt.wantValid, valid)
		})
	}
}

func TestEvaluatePassword_OnlyEnabledRules(t *testing.T) {
	t.Parallel()
	// Выключенные требования не появляются в списке.
	rules := EvaluatePassword(models.PasswordPolicy{MinLength: 4}, "ab")
	assert.Len(t, rules, 1)
	assert.Equal(t, "min_length", rules[0].Key)
}

func TestValidatePassword_Returns400(t *testing.T) {
	t.Parallel()
	err := ValidatePassword(models.DefaultPasswordPolicy(), "short")
	assert.Error(t, err)
	assert.Nil(t, ValidatePassword(models.DefaultPasswordPolicy(), "password1"))
}
