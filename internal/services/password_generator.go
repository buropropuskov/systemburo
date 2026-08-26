package services

import (
	"crypto/rand"
	"math/big"
	"strings"

	"systemburo/internal/models"
)

// Наборы символов генератора. Из каждого исключены визуально неоднозначные
// знаки: сгенерированный пароль переписывают из письма руками, и «единица или
// строчная L» стоит звонка в бюро. По той же причине нет пробела и кавычек -
// их теряют при копировании и экранируют в командной строке.
const (
	generatorUpper   = "ABCDEFGHJKLMNPQRSTUVWXYZ" // без I и O
	generatorLower   = "abcdefghijkmnpqrstuvwxyz" // без l и o
	generatorDigits  = "23456789"                 // без 0 и 1
	generatorSpecial = "!#$%&*+-=?@"              // подмножество passwordSpecialChars
)

// generatedPasswordMinLength - нижняя граница длины независимо от политики.
// Политика допускает шесть символов, но пароль, выданный системой, живёт до
// первой смены человеком и должен переживать эту жизнь без подбора.
const generatedPasswordMinLength = 12

// generatedPasswordMaxLength - потолок из validate-тега на запросах смены
// пароля (max=255). Генератор не должен выдавать то, что не примет собственная
// ручка обновления.
const generatedPasswordMaxLength = 255

// GeneratePassword собирает пароль, заведомо проходящий переданную политику:
// по одному символу на каждое включённое требование, остаток - из общего пула,
// затем перестановка. Источник случайности - crypto/rand: пароль выдаётся
// человеку как учётные данные, и предсказуемый генератор здесь равносилен
// отсутствию пароля.
func GeneratePassword(p models.PasswordPolicy) string {
	length := p.MinLength
	if length < generatedPasswordMinLength {
		length = generatedPasswordMinLength
	}
	if length > generatedPasswordMaxLength {
		length = generatedPasswordMaxLength
	}

	var required []byte
	pool := generatorLower + generatorUpper + generatorDigits

	if p.RequireUppercase {
		required = append(required, pickRune(generatorUpper))
	}
	if p.RequireLowercase {
		required = append(required, pickRune(generatorLower))
	}
	// RequireLetter закрывается любой буквой. Если уже потребована заглавная или
	// строчная, отдельная буква не нужна - иначе на коротком пароле обязательные
	// символы съедят всю длину.
	if p.RequireLetter && !p.RequireUppercase && !p.RequireLowercase {
		required = append(required, pickRune(generatorLower+generatorUpper))
	}
	if p.RequireDigit {
		required = append(required, pickRune(generatorDigits))
	}
	if p.RequireSpecial {
		required = append(required, pickRune(generatorSpecial))
		// Спецсимвол попадает в общий пул только когда его требует политика:
		// иначе пароль пестрит знаками там, где их не просили.
		pool += generatorSpecial
	}

	// Требований оказалось больше запрошенной длины - берём длину по требованиям,
	// иначе пароль не прошёл бы собственную политику.
	if len(required) > length {
		length = len(required)
	}

	out := make([]byte, 0, length)
	out = append(out, required...)
	for len(out) < length {
		out = append(out, pickRune(pool))
	}
	shuffle(out)
	return string(out)
}

// pickRune возвращает случайный символ набора. Ошибка crypto/rand означает
// сломанный источник энтропии в системе - продолжать в таком состоянии нельзя,
// поэтому паника, а не тихий возврат предсказуемого символа.
func pickRune(set string) byte {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(set))))
	if err != nil {
		panic("crypto/rand недоступен: " + err.Error())
	}
	return set[n.Int64()]
}

// shuffle перемешивает символы на месте (Фишер-Йетс). Без него позиция
// обязательного символа предсказуема: заглавная всегда первой, цифра третьей.
func shuffle(b []byte) {
	for i := len(b) - 1; i > 0; i-- {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			panic("crypto/rand недоступен: " + err.Error())
		}
		j := n.Int64()
		b[i], b[j] = b[j], b[i]
	}
}

// generatorCharsetsValid сверяет наборы генератора с набором спецсимволов
// политики. Вынесено функцией, чтобы тест мог проверить это утверждение, а не
// повторять список символов у себя.
func generatorCharsetsValid() bool {
	for _, r := range generatorSpecial {
		if !strings.ContainsRune(passwordSpecialChars, r) {
			return false
		}
	}
	return true
}
