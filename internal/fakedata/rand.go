package fakedata

import (
	"encoding/binary"
	"hash/fnv"
	"math/rand/v2"
)

// Stream -- независимый источник случайности одного домена данных партии (ФИО,
// номера машин, телефоны, наименования организаций и так далее).
//
// Домены не делят между собой один rand.Rand намеренно. Общий источник на всю
// партию связал бы домены очерёдностью вызовов: добавление одного Pick в середину
// шага сдвинуло бы всю последующую последовательность, и повтор с тем же -seed
// перестал бы давать ту же партию при следующей правке кода где угодно раньше по
// потоку. Каждый домен получает поток, производный от seed партии и своего имени
// (hash(seed, domain)), поэтому правка в одном домене не задевает остальные.
type Stream struct {
	rng *rand.Rand
}

// NewStream заводит поток для домена. Одинаковые (seed, domain) всегда дают
// одинаковую последовательность значений -- в этом и есть обещанная командой
// "server fake" повторяемость партии по -seed.
func NewStream(seed int64, domain string) *Stream {
	return &Stream{rng: rand.New(rand.NewPCG(streamSeed(seed, domain, 1), streamSeed(seed, domain, 2)))}
}

// streamSeed хэширует seed партии, имя домена и соль в 64 бита. Соль разводит два
// слова PCG-источника: без неё seed1 и seed2 совпадали бы и часть внутреннего
// состояния генератора была бы вырождена.
func streamSeed(seed int64, domain string, salt byte) uint64 {
	h := fnv.New64a()
	var buf [9]byte
	binary.LittleEndian.PutUint64(buf[:8], uint64(seed))
	buf[8] = salt
	_, _ = h.Write(buf[:])
	_, _ = h.Write([]byte(domain))
	return h.Sum64()
}

// Pick возвращает случайный элемент среза. Паникует на пустом срезе: это ошибка
// словаря в коде, а не входных данных партии, и подменять её нулевым значением
// означало бы молча налить в базу пустые ФИО или номера.
func Pick[T any](s *Stream, items []T) T {
	if len(items) == 0 {
		panic("fakedata: Pick вызван на пустом словаре")
	}
	return items[s.rng.IntN(len(items))]
}

// IntRange возвращает целое число в диапазоне [lo, hi] включительно.
func IntRange(s *Stream, lo, hi int) int {
	if hi < lo {
		panic("fakedata: IntRange получил hi < lo")
	}
	return lo + s.rng.IntN(hi-lo+1)
}

// Chance возвращает true с вероятностью probability (0..1). Используется там, где
// нужен не равновероятный выбор из списка, а вероятностный признак -- например,
// доля партии с патентом или архивных записей.
func Chance(s *Stream, probability float64) bool {
	return s.rng.Float64() < probability
}
