package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Границы бэкфилла считаются в рабочей таймзоне (#1615, B4).
//
// Тест-окружение проекта поднимает раскладку на UTC, где смещение нулевое и ошибка
// не видна вовсе - поэтому проверка идёт напрямую на сервисе с московской зоной,
// той самой, что стоит на боевом сервере. Базы расчёту не нужно.
func TestParsePeriod_UsesWorkingTimezone(t *testing.T) {
	msk := time.FixedZone("MSK", 3*60*60)
	svc := &BlankExportService{paths: NewArchivePathService(nil, msk)}

	from, toExclusive, err := svc.ParsePeriod("2026-03-10", "2026-03-12")
	require.NoError(t, err)

	assert.Equal(t, "2026-03-10T00:00:00+03:00", from.Format(time.RFC3339),
		"начало периода - местная полночь, а не UTC")
	assert.Equal(t, "2026-03-13T00:00:00+03:00", toExclusive.Format(time.RFC3339),
		"date_to включителен: конец периода - начало следующих МЕСТНЫХ суток")

	// Заявка, поданная в час ночи по Москве первого дня периода, в UTC относится к
	// предыдущим суткам. При разборе границ в UTC она выпадала бы из бэкфилла, хотя
	// её каталог на диске - внутри периода.
	earlyMorning := time.Date(2026, 3, 10, 1, 30, 0, 0, msk)
	assert.False(t, earlyMorning.Before(from), "ночная заявка первого дня обязана попасть в период")

	// Зеркальный случай: заявка, поданная в 23:00 последнего дня периода, лежит в
	// каталоге этого дня и обязана попасть, а такая же заявка следующего дня - нет.
	lateEvening := time.Date(2026, 3, 12, 23, 0, 0, 0, msk)
	assert.True(t, lateEvening.Before(toExclusive), "вечерняя заявка последнего дня входит в период")
	nextDayEvening := time.Date(2026, 3, 13, 23, 0, 0, 0, msk)
	assert.False(t, nextDayEvening.Before(toExclusive), "заявка следующего дня в период не входит")
}

// Границы, посчитанные в UTC, дают ровно тот сдвиг, ради которого расчёт и переехал
// в сервис: замок на разницу, чтобы будущая «упрощающая» правка не вернула time.Parse.
func TestParsePeriod_DiffersFromNaiveUTC(t *testing.T) {
	msk := time.FixedZone("MSK", 3*60*60)
	svc := &BlankExportService{paths: NewArchivePathService(nil, msk)}

	from, _, err := svc.ParsePeriod("2026-03-10", "2026-03-10")
	require.NoError(t, err)

	naive, err := time.Parse("2006-01-02", "2026-03-10")
	require.NoError(t, err)

	assert.Equal(t, -3*time.Hour, from.Sub(naive),
		"местная полночь наступает раньше UTC-полуночи ровно на смещение зоны")
}

func TestParsePeriod_RejectsBadInput(t *testing.T) {
	svc := &BlankExportService{paths: NewArchivePathService(nil, time.UTC)}

	_, _, err := svc.ParsePeriod("10.03.2026", "2026-03-12")
	assert.Error(t, err, "дата не в формате YYYY-MM-DD")

	_, _, err = svc.ParsePeriod("2026-03-12", "не дата")
	assert.Error(t, err)

	_, _, err = svc.ParsePeriod("2026-03-12", "2026-03-10")
	assert.Error(t, err, "конец периода раньше начала")
}
