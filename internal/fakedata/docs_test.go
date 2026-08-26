package fakedata_test

import (
	"regexp"
	"strconv"
	"testing"
	"time"

	"systemburo/internal/fakedata"

	"github.com/stretchr/testify/require"
)

var passportRe = regexp.MustCompile(`^(\d{2})(\d{2}) (\d{6})$`)

// Главная гарантия паспорта вымышленного сотрудника: вторая пара цифр серии -- год
// выпуска бланка, и он обязан быть из тех, что ещё не наступили. Бланков будущих лет
// не существует, поэтому такая серия не может принадлежать настоящему паспорту.
//
// Год сравнивается с календарём, а не с копией той же константы из docs.go: сторож,
// сверяющий значение с самим собой, истинен по построению и промолчал бы ровно тогда,
// когда приём перестанет работать. Проверка обязана краснеть от течения времени -- в
// год, когда настоящие бланки дойдут до нижней границы диапазона.
func TestPassport_SeriesYearPartIsUnissued(t *testing.T) {
	currentTwoDigitYear := time.Now().Year() % 100

	s := fakedata.NewStream(1, "docs")
	for i := 0; i < 300; i++ {
		doc := fakedata.Passport(s)
		m := passportRe.FindStringSubmatch(doc)
		require.Len(t, m, 4, "паспорт %q не соответствует формату 'серия номер'", doc)
		year, err := strconv.Atoi(m[2])
		require.NoError(t, err)
		require.Greater(t, year, currentTwoDigitYear,
			"год бланка %d уже наступил: серия может совпасть с настоящим паспортом, "+
				"подними passportYearFrom в docs.go", year)

		region, err := strconv.Atoi(m[1])
		require.NoError(t, err)
		require.Contains(t, fakedata.PassportRegionCodes, region, "код региона должен быть правдоподобным")
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

// У патента проверяемого признака «не выдавался» нет, поэтому сторожим только форму:
// номер должен выглядеть как номер, а не как случайная строка в поле анкеты.
func TestPatent_MatchesExpectedShape(t *testing.T) {
	s := fakedata.NewStream(2, "docs")
	for i := 0; i < 300; i++ {
		doc := fakedata.Patent(s)
		m := patentRe.FindStringSubmatch(doc)
		require.Len(t, m, 2, "патент %q не соответствует ожидаемому формату", doc)
		region, err := strconv.Atoi(m[1])
		require.NoError(t, err)
		require.Contains(t, fakedata.PassportRegionCodes, region)
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
