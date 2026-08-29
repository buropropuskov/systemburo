package crypto

import (
	"log/slog"
	"sync/atomic"
	"time"
)

// Учёт неудачных расшифровок (#2253).
//
// DecryptOptional по построению возвращает значение как есть, когда расшифровать
// его не вышло: иначе один сбойный столбец ронял бы выдачу целиком. Но молчать об
// этом нельзя - именно в этом виде проблема с ключом доезжала до оператора в виде
// набора символов вместо серии паспорта, и никто не знал, что происходит.
//
// Подмену ключа целиком теперь ловит контрольная запись при старте
// (database.EnsureEncryptionKeyMatches), поэтому сюда доходит другой случай: часть
// записей осталась на прежнем ключе либо открытыми. Об этом достаточно сообщать
// сводкой, а не строкой на каждое значение - иначе один список сотрудников
// напишет в журнал тысячу одинаковых строк.

// decryptWarnInterval - как часто повторяется предупреждение. Первая неудача
// сообщается сразу, дальше не чаще этого срока.
const decryptWarnInterval = time.Minute

var (
	decryptFailures atomic.Int64
	// lastDecryptWarnUnix - время прошлого предупреждения. Ноль означает, что
	// предупреждения ещё не было, и ближайшая неудача сообщается немедленно.
	lastDecryptWarnUnix atomic.Int64
)

// DecryptFailures - сколько значений не удалось расшифровать с запуска.
// Нужна диагностике и тестам: по одному журналу число не собрать.
func DecryptFailures() int64 { return decryptFailures.Load() }

// ResetDecryptFailures обнуляет учёт. Для тестов, которые проверяют счётчик.
func ResetDecryptFailures() {
	decryptFailures.Store(0)
	lastDecryptWarnUnix.Store(0)
}

// noteDecryptFailure считает неудачу и при необходимости пишет сводку в журнал.
func noteDecryptFailure() {
	total := decryptFailures.Add(1)

	now := time.Now().Unix()
	last := lastDecryptWarnUnix.Load()
	if last != 0 && now-last < int64(decryptWarnInterval/time.Second) {
		return
	}
	// Проигрывать гонку здесь не страшно: победитель пишет строку, остальные
	// молчат до следующего срока, а счётчик у всех общий и верный.
	if !lastDecryptWarnUnix.CompareAndSwap(last, now) {
		return
	}

	slog.Warn("значение не расшифровано действующим ключом, показано как есть",
		"всего_с_запуска", total,
		"вероятная_причина", "часть записей осталась на прежнем ключе или лежит открытой",
		"что_делать", "перевести данные командой server reencrypt")
}
