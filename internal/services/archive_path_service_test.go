package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Каталог дня считается по МЕСТНОЙ дате подачи. Заявка, поданная поздним вечером,
// хранится в UTC уже следующим числом, и без приведения к рабочей зоне уехала бы в
// папку завтрашнего дня - оператор искал бы её не там (#1615, класс бага из #980).
func TestArchivePathService_BucketDateUsesLocalDay(t *testing.T) {
	msk, err := time.LoadLocation("Europe/Moscow")
	require.NoError(t, err, "зоны вшиты в бинарь импортом time/tzdata")

	svc := NewArchivePathService(nil, msk)

	// 31 июля 23:30 МСК = 1 августа 20:30 UTC.
	lateEvening := time.Date(2026, 7, 31, 20, 30, 0, 0, time.UTC)
	bucket := svc.BucketDate(&lateEvening)
	require.Equal(t, 2026, bucket.Year())
	require.Equal(t, time.July, bucket.Month())
	require.Equal(t, 31, bucket.Day(), "вечерняя заявка остаётся в каталоге своего местного дня")
	require.Equal(t, 0, bucket.Hour(), "дата каталога обнуляется до начала суток")

	// Ранее утро следующего дня по МСК уже принадлежит новому каталогу.
	earlyMorning := time.Date(2026, 7, 31, 21, 30, 0, 0, time.UTC)
	require.Equal(t, 1, svc.BucketDate(&earlyMorning).Day())

	// У заявки без даты подачи каталог берётся по сегодняшнему местному дню: файл
	// всё равно должен куда-то лечь, а "нулевой год" в пути читался бы как поломка.
	require.Equal(t, time.Now().In(msk).Day(), svc.BucketDate(nil).Day())
}
