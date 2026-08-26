package services

import (
	"strings"
	"testing"

	"systemburo/internal/models"
)

// generatorPolicies - политики, на которых гоняется замок. Покрывают крайности:
// только длина, все требования разом, длина ниже и выше собственного минимума
// генератора.
func generatorPolicies() map[string]models.PasswordPolicy {
	return map[string]models.PasswordPolicy{
		"только длина":      {MinLength: 6},
		"по умолчанию":      models.DefaultPasswordPolicy(),
		"буква и цифра":     {MinLength: 8, RequireLetter: true, RequireDigit: true},
		"регистры":          {MinLength: 10, RequireUppercase: true, RequireLowercase: true},
		"со спецсимволом":   {MinLength: 12, RequireLetter: true, RequireDigit: true, RequireSpecial: true},
		"всё сразу":         {MinLength: 16, RequireLetter: true, RequireUppercase: true, RequireLowercase: true, RequireDigit: true, RequireSpecial: true},
		"короткая со всем":  {MinLength: 6, RequireLetter: true, RequireUppercase: true, RequireLowercase: true, RequireDigit: true, RequireSpecial: true},
		"длинная":           {MinLength: 64, RequireLetter: true, RequireDigit: true},
		"только спецсимвол": {MinLength: 8, RequireSpecial: true},
		"только заглавные":  {MinLength: 8, RequireUppercase: true},
		"нулевая длина":     {},
		// 128 - потолок password.min_length в настройках (validateSettingValue).
		// Выше политика физически не поднимается, поэтому и проверяем её.
		"предельная длина": {MinLength: 128, RequireLetter: true, RequireDigit: true, RequireSpecial: true},
	}
}

// TestGeneratePassword_AlwaysPassesPolicy - главный замок среза. Генератор и
// валидатор обязаны сойтись: разойдутся - плановая смена паролей выдаст пароль,
// который система сама же не примет, причём молча и только у части работников.
func TestGeneratePassword_AlwaysPassesPolicy(t *testing.T) {
	for name, policy := range generatorPolicies() {
		t.Run(name, func(t *testing.T) {
			for i := 0; i < 1000; i++ {
				pwd := GeneratePassword(policy)
				if err := ValidatePassword(policy, pwd); err != nil {
					t.Fatalf("итерация %d: пароль %q не прошёл собственную политику: %v", i, pwd, err)
				}
			}
		})
	}
}

// TestGeneratePassword_LengthBounds: длина не опускается ниже собственного
// минимума генератора и не вылезает за потолок, который принимает ручка смены
// пароля (validate max=255).
func TestGeneratePassword_LengthBounds(t *testing.T) {
	short := GeneratePassword(models.PasswordPolicy{MinLength: 6})
	if len(short) < generatedPasswordMinLength {
		t.Errorf("короткая политика: длина %d меньше минимума генератора %d", len(short), generatedPasswordMinLength)
	}

	// Потолок генератора - предел, который принимает ручка смены пароля
	// (validate max=255). Настройки политики выше 128 не поднимаются, так что
	// потолок недостижим в бою и стоит здесь как защита от будущей правки границ.
	long := GeneratePassword(models.PasswordPolicy{MinLength: 400})
	if len(long) != generatedPasswordMaxLength {
		t.Errorf("политика выше потолка: ожидали %d символов, получили %d", generatedPasswordMaxLength, len(long))
	}

	exact := GeneratePassword(models.PasswordPolicy{MinLength: 20})
	if len(exact) != 20 {
		t.Errorf("ожидали ровно 20 символов, получили %d", len(exact))
	}
}

// TestGeneratePassword_NoAmbiguousChars: пароль переписывают из письма руками,
// поэтому ноль с буквой O и единица с буквой l в нём недопустимы.
func TestGeneratePassword_NoAmbiguousChars(t *testing.T) {
	const ambiguous = "0O1lI"
	policy := models.PasswordPolicy{
		MinLength: 32, RequireLetter: true, RequireUppercase: true,
		RequireLowercase: true, RequireDigit: true, RequireSpecial: true,
	}
	for i := 0; i < 500; i++ {
		pwd := GeneratePassword(policy)
		if idx := strings.IndexAny(pwd, ambiguous); idx >= 0 {
			t.Fatalf("итерация %d: пароль %q содержит неоднозначный символ %q", i, pwd, pwd[idx])
		}
	}
}

// TestGeneratePassword_NoSpecialsUnlessRequired: без требования спецсимвола
// пароль состоит только из букв и цифр - иначе он пестрит знаками там, где их
// не просили, и его труднее продиктовать по телефону.
func TestGeneratePassword_NoSpecialsUnlessRequired(t *testing.T) {
	policy := models.PasswordPolicy{MinLength: 24, RequireLetter: true, RequireDigit: true}
	for i := 0; i < 200; i++ {
		pwd := GeneratePassword(policy)
		if strings.ContainsAny(pwd, passwordSpecialChars) {
			t.Fatalf("итерация %d: пароль %q содержит спецсимвол, хотя политика его не требует", i, pwd)
		}
	}
}

// TestGeneratePassword_Varies: два подряд сгенерированных пароля не совпадают.
// Проверка на грубую поломку источника случайности - вырожденный генератор
// выдавал бы одно и то же и прошёл бы все остальные тесты.
func TestGeneratePassword_Varies(t *testing.T) {
	policy := models.DefaultPasswordPolicy()
	seen := make(map[string]bool, 200)
	for i := 0; i < 200; i++ {
		pwd := GeneratePassword(policy)
		if seen[pwd] {
			t.Fatalf("повтор пароля %q на итерации %d", pwd, i)
		}
		seen[pwd] = true
	}
}

// TestGeneratePassword_RequiredCharsNotAlwaysFirst: перестановка работает. Без
// неё позиция обязательного символа предсказуема, и подбор сужается.
func TestGeneratePassword_RequiredCharsNotAlwaysFirst(t *testing.T) {
	policy := models.PasswordPolicy{MinLength: 16, RequireUppercase: true, RequireDigit: true}
	firstIsUpper := 0
	const runs = 300
	for i := 0; i < runs; i++ {
		pwd := GeneratePassword(policy)
		if pwd[0] >= 'A' && pwd[0] <= 'Z' {
			firstIsUpper++
		}
	}
	// При честной перестановке заглавная оказывается первой примерно в трети
	// случаев (в пуле все три класса). Порог выбран с большим запасом: замок
	// ловит вырожденный случай «всегда первая», а не проверяет равномерность.
	if firstIsUpper >= runs*9/10 {
		t.Errorf("заглавная оказалась первой в %d случаях из %d - похоже, перестановки нет", firstIsUpper, runs)
	}
}

// TestGeneratorSpecialsAreSubsetOfPolicy: набор спецсимволов генератора обязан
// целиком лежать внутри набора, который признаёт валидатор. Разойдутся -
// пароль со «своим» знаком не пройдёт require_special.
func TestGeneratorSpecialsAreSubsetOfPolicy(t *testing.T) {
	if !generatorCharsetsValid() {
		t.Errorf("набор спецсимволов генератора %q не входит в набор политики %q", generatorSpecial, passwordSpecialChars)
	}
}
