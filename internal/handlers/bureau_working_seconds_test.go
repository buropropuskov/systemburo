package handlers_test

import (
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/require"
)

// SQL-функция bureau_working_seconds (installSQLFunctions, #1251 S1) считает
// рабочие секунды Бюро между двумя моментами по расписанию bureau_time_slots:
// пересечение окна с рабочими слотами по дню недели в МСК, учёт перехода через
// полночь (is_next_day), слияние перекрывающихся слотов. go build/vet функцию не
// исполняют -- корректность SQL ловится только прогоном (см. урок про SQL от
// субагента), поэтому кейсы детерминированы и посчитаны вручную.
//
// Даты фиксированы и их дни недели проверены: 2026-06-15 Пн, -19 Пт, -20 Сб,
// -21 Вс, -22 Пн. Времена задаём в МСК (FixedZone +3), как хранит расписание.
func TestBureauWorkingSeconds(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	msk := time.FixedZone("MSK", 3*60*60)
	at := func(y int, m time.Month, d, hh, mm int) time.Time {
		return time.Date(y, m, d, hh, mm, 0, 0, msk)
	}
	// Страховка от опечатки в дате: если день недели уедет, кейсы врут молча.
	require.Equal(t, time.Monday, at(2026, 6, 15, 0, 0).Weekday())
	require.Equal(t, time.Friday, at(2026, 6, 19, 0, 0).Weekday())

	setSchedule := func(slots []models.BureauTimeSlot) {
		require.NoError(t, db.Exec("DELETE FROM bureau_time_slots").Error)
		for i := range slots {
			require.NoError(t, db.Create(&slots[i]).Error)
		}
	}
	// allDays строит одинаковый слот на каждый день недели (0=Пн..6=Вс).
	allDays := func(open, close string, nextDay bool) []models.BureauTimeSlot {
		out := make([]models.BureauTimeSlot, 0, 7)
		for dow := 0; dow < 7; dow++ {
			out = append(out, models.BureauTimeSlot{
				DayOfWeek: dow, OpenTime: open, CloseTime: close, IsNextDay: nextDay, IsActive: true,
			})
		}
		return out
	}
	workSeconds := func(from, to time.Time) int64 {
		var secs int64
		require.NoError(t, db.Raw("SELECT bureau_working_seconds(?, ?)", from, to).Scan(&secs).Error)
		return secs
	}

	const h = int64(3600)

	// --- График: все 7 дней 08:00-22:00 ---
	t.Run("all_days_08_22", func(t *testing.T) {
		setSchedule(allDays("08:00", "22:00", false))

		// Эталонный кейс плана: подача Пт 21:00 -> согласование Сб 09:00.
		// Пт 21-22 (1ч) + Сб 08-09 (1ч) = 2ч, ночь между ними не считается.
		require.Equal(t, 2*h, workSeconds(at(2026, 6, 19, 21, 0), at(2026, 6, 20, 9, 0)))

		// Оба момента внутри одного рабочего окна.
		require.Equal(t, 2*h, workSeconds(at(2026, 6, 15, 9, 0), at(2026, 6, 15, 11, 0)))

		// Полностью в нерабочее время -> 0.
		require.Equal(t, int64(0), workSeconds(at(2026, 6, 15, 23, 0), at(2026, 6, 15, 23, 30)))

		// Начало до открытия -> считается с 08:00.
		require.Equal(t, h, workSeconds(at(2026, 6, 15, 7, 0), at(2026, 6, 15, 9, 0)))

		// Окно шире рабочего дня -> ровно рабочий день 08-22 = 14ч.
		require.Equal(t, 14*h, workSeconds(at(2026, 6, 15, 6, 0), at(2026, 6, 15, 23, 0)))

		// to <= from -> 0.
		require.Equal(t, int64(0), workSeconds(at(2026, 6, 15, 11, 0), at(2026, 6, 15, 9, 0)))

		// NULL-момент -> 0 (незавершённый этап отсекается вызывающей стороной, но
		// функция не должна падать).
		var secs int64
		require.NoError(t, db.Raw("SELECT bureau_working_seconds(NULL::timestamptz, ?)", at(2026, 6, 15, 9, 0)).Scan(&secs).Error)
		require.Equal(t, int64(0), secs)
	})

	// --- График: только будни (Пн-Пт) 08:00-22:00 ---
	t.Run("weekdays_only", func(t *testing.T) {
		slots := make([]models.BureauTimeSlot, 0, 5)
		for dow := 0; dow <= 4; dow++ { // 0=Пн..4=Пт
			slots = append(slots, models.BureauTimeSlot{
				DayOfWeek: dow, OpenTime: "08:00", CloseTime: "22:00", IsActive: true,
			})
		}
		setSchedule(slots)

		// Пт 21:00 -> Пн 09:00: Пт 21-22 (1ч) + выходные закрыты + Пн 08-09 (1ч) = 2ч.
		require.Equal(t, 2*h, workSeconds(at(2026, 6, 19, 21, 0), at(2026, 6, 22, 9, 0)))

		// Полностью в субботу -> Бюро закрыто -> 0.
		require.Equal(t, int64(0), workSeconds(at(2026, 6, 20, 10, 0), at(2026, 6, 20, 14, 0)))

		// Широкое окно на 2 недели через 2 выходных подряд: еженедельный матчинг
		// day_of_week должен просуммироваться по многим итерациям generate_series.
		// Пн 15.06 00:00 -> Пн 29.06 00:00 = 10 рабочих дней (Пн-Пт x2) x 14ч = 140ч.
		require.Equal(t, 140*h, workSeconds(at(2026, 6, 15, 0, 0), at(2026, 6, 29, 0, 0)))
	})

	// --- Слот с переходом через полночь: Пт 22:00 -> Сб 06:00 ---
	t.Run("next_day_slot", func(t *testing.T) {
		setSchedule([]models.BureauTimeSlot{
			{DayOfWeek: 4, OpenTime: "22:00", CloseTime: "06:00", IsNextDay: true, IsActive: true}, // 4=Пт
		})

		// Оба момента внутри ночного слота: Пт 23:00 -> Сб 02:00 = 3ч.
		require.Equal(t, 3*h, workSeconds(at(2026, 6, 19, 23, 0), at(2026, 6, 20, 2, 0)))

		// Окно шире слота: Пт 21:00 -> Сб 07:00 -> считается только слот Пт22-Сб06 = 8ч.
		require.Equal(t, 8*h, workSeconds(at(2026, 6, 19, 21, 0), at(2026, 6, 20, 7, 0)))
	})

	// --- Круглосуточный слот 00:00-23:59 (соглашение UI «весь день») ---
	t.Run("round_the_clock", func(t *testing.T) {
		setSchedule(allDays("00:00", "23:59", false))
		// Полный день по слоту 00:00-23:59 = 86340с. Соглашение проекта задаёт «весь
		// день» как 23:59 (не 24:00), поэтому недостаёт последней минуты -- на
		// длительностях в часах это не значимо, но фиксируем поведение явно.
		require.Equal(t, int64(86340), workSeconds(at(2026, 6, 15, 0, 0), at(2026, 6, 15, 23, 59)))
	})

	// --- Перекрывающиеся слоты одного дня не считаются дважды ---
	t.Run("overlapping_slots_merged", func(t *testing.T) {
		setSchedule([]models.BureauTimeSlot{
			{DayOfWeek: 0, OpenTime: "08:00", CloseTime: "12:00", IsActive: true}, // Пн 08-12
			{DayOfWeek: 0, OpenTime: "10:00", CloseTime: "14:00", IsActive: true}, // Пн 10-14
		})
		// Объединение [08:00,14:00] = 6ч, а не 4ч+4ч=8ч.
		require.Equal(t, 6*h, workSeconds(at(2026, 6, 15, 6, 0), at(2026, 6, 15, 23, 0)))
	})

	// --- Неактивный слот в расчёт не идёт ---
	t.Run("inactive_slot_excluded", func(t *testing.T) {
		require.NoError(t, db.Exec("DELETE FROM bureau_time_slots").Error)
		slot := models.BureauTimeSlot{DayOfWeek: 0, OpenTime: "08:00", CloseTime: "22:00"}
		require.NoError(t, db.Create(&slot).Error)
		// is_active имеет gorm default:true -> zero-value false при Create опускается,
		// БД ставит true; форсируем false явным Update (как в work_modes_test.go).
		require.NoError(t, db.Model(&slot).Update("is_active", false).Error)
		require.Equal(t, int64(0), workSeconds(at(2026, 6, 15, 9, 0), at(2026, 6, 15, 11, 0)))
	})

	// --- Пустое расписание: рабочего времени нет -> 0 (риск для S2 отдельно) ---
	t.Run("empty_schedule", func(t *testing.T) {
		setSchedule(nil)
		require.Equal(t, int64(0), workSeconds(at(2026, 6, 15, 9, 0), at(2026, 6, 15, 11, 0)))
	})
}
