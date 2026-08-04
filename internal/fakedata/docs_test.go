package fakedata_test

import (
	"regexp"
	"strconv"
	"testing"

	"systemburo/internal/fakedata"

	"github.com/stretchr/testify/require"
)

var passportRe = regexp.MustCompile(`^(\d{2})(\d{2}) (\d{6})$`)

// Главная гарантия паспорта вымышленного сотрудника: старшие две цифры серии
// обязаны быть из PassportReservedPrefixes, которых не существует ни у одного
// субъекта РФ. От этого зависит, что документ не совпадёт с настоящим.
func TestPassport_SeriesPrefixAlwaysReserved(t *testing.T) {
	reserved := map[int]bool{}
	for _, p := range fakedata.PassportReservedPrefixes {
		reserved[p] = true
	}

	s := fakedata.NewStream(1, "docs")
	for i := 0; i < 300; i++ {
		doc := fakedata.Passport(s)
		m := passportRe.FindStringSubmatch(doc)
		require.Len(t, m, 4, "паспорт %q не соответствует формату 'серия номер'", doc)
		prefix, err := strconv.Atoi(m[1])
		require.NoError(t, err)
		require.True(t, reserved[prefix], "префикс серии %d должен быть из зарезервированного диапазона", prefix)
	}
}

func TestPassport_RepeatableBySeed(t *testing.T) {
	a := fakedata.NewStream(55, "docs")
	b := fakedata.NewStream(55, "docs")
	for i := 0; i < 20; i++ {
		require.Equal(t, fakedata.Passport(a), fakedata.Passport(b))
	}
}

var patentRe = regexp.MustCompile(`^(\d{2}) \d{2} \d{7}$`)

func TestPatent_SeriesPrefixAlwaysReserved(t *testing.T) {
	reserved := map[int]bool{}
	for _, p := range fakedata.PassportReservedPrefixes {
		reserved[p] = true
	}

	s := fakedata.NewStream(2, "docs")
	for i := 0; i < 300; i++ {
		doc := fakedata.Patent(s)
		m := patentRe.FindStringSubmatch(doc)
		require.Len(t, m, 2, "патент %q не соответствует ожидаемому формату", doc)
		prefix, err := strconv.Atoi(m[1])
		require.NoError(t, err)
		require.True(t, reserved[prefix])
	}
}

// Формат должен остаться валидным российским номером: код страны 7, вторая
// цифра кода оператора в диапазоне 3-9 (см. isValidRussianPhone во фронтовой
// маске) - иначе номер выглядел бы явно битым, а не просто вымышленным.
var phoneRe = regexp.MustCompile(`^\+7(\d{10})$`)

func TestPhone_MatchesRussianFormat(t *testing.T) {
	s := fakedata.NewStream(1, "docs")
	for i := 0; i < 300; i++ {
		phone := fakedata.Phone(s)
		m := phoneRe.FindStringSubmatch(phone)
		require.Len(t, m, 2, "номер %q не соответствует формату +7XXXXXXXXXX", phone)
		require.GreaterOrEqual(t, m[1][0], byte('3'))
		require.LessOrEqual(t, m[1][0], byte('9'))
	}
}

func TestPhone_UsesOperatorCodeConstant(t *testing.T) {
	s := fakedata.NewStream(1, "docs")
	phone := fakedata.Phone(s)
	require.Equal(t, "+7"+fakedata.PhoneOperatorCode, phone[:2+len(fakedata.PhoneOperatorCode)])
}

func TestPhone_RepeatableBySeed(t *testing.T) {
	a := fakedata.NewStream(7, "docs")
	b := fakedata.NewStream(7, "docs")
	require.Equal(t, fakedata.Phone(a), fakedata.Phone(b))
}
