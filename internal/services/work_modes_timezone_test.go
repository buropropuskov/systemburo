package services

import (
	"testing"
	"time"

	"systemburo/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestComputeStatus_MoscowTimezoneAtUTCMidnight фиксирует регрессию #868: статус мест
// разгрузки и системных таблиц считается в МСК, а не в серверном UTC. В окне
// 21:00-24:00 UTC московский день недели уже следующий - если функция берёт UTC-день
// (как было до фикса), круглосуточный слот сегодняшнего МСК-дня не совпадёт и вернётся
// "closed". Существующий TestWorkModes_Aggregates ловил это лишь когда CI бежал в этом
// окне (3 ч/сутки), поэтому здесь момент инъектируется фиксированным.
func TestComputeStatus_MoscowTimezoneAtUTCMidnight(t *testing.T) {
	// 21:30 UTC = 00:30 МСК следующих суток - UTC-день и МСК-день расходятся.
	utcEvening := time.Date(2026, 6, 29, 21, 30, 0, 0, time.UTC)
	msk := time.FixedZone("MSK", 3*60*60)
	mskDay := int(utcEvening.In(msk).Weekday()+6) % 7
	utcDay := int(utcEvening.Weekday()+6) % 7
	require.NotEqual(t, mskDay, utcDay, "момент должен попадать в окно расхождения UTC/МСК дней")

	// Место разгрузки: круглосуточный слот на МСК-день открыт в 00:30 МСК.
	unloadMSK := []models.UnloadPlaceTimeSlot{{DayOfWeek: mskDay, OpenTime: "00:00", CloseTime: "23:59", IsActive: true}}
	assert.Equal(t, "open", computeUnloadPlaceStatusAt(utcEvening, "active", unloadMSK),
		"место разгрузки: круглосуточный слот МСК-дня открыт")
	// Слот на UTC-день (вчерашний по МСК) закрыт - доказывает, что берётся МСК, не UTC.
	unloadUTC := []models.UnloadPlaceTimeSlot{{DayOfWeek: utcDay, OpenTime: "00:00", CloseTime: "23:59", IsActive: true}}
	assert.Equal(t, "closed", computeUnloadPlaceStatusAt(utcEvening, "active", unloadUTC),
		"место разгрузки: слот UTC-дня закрыт - функция не должна брать UTC-день")

	// То же для системных таблиц (общий баг, общий фикс).
	tableMSK := []models.SystemTableTimeSlot{{DayOfWeek: mskDay, OpenTime: "00:00", CloseTime: "23:59", IsActive: true}}
	assert.Equal(t, "open", computeCurrentStatusAt(utcEvening, "active", tableMSK),
		"системная таблица: круглосуточный слот МСК-дня открыт")
	tableUTC := []models.SystemTableTimeSlot{{DayOfWeek: utcDay, OpenTime: "00:00", CloseTime: "23:59", IsActive: true}}
	assert.Equal(t, "closed", computeCurrentStatusAt(utcEvening, "active", tableUTC),
		"системная таблица: слот UTC-дня закрыт")
}
