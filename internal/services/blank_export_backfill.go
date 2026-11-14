package services

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v5"
	"gorm.io/gorm"
)

// Backfill ставит в очередь на выгрузку все заявки периода [from, toExclusive), по
// желанию суженные типом вложения. Разбор идёт фоновым воркером через обычную
// очередь (B1) - ручка не пишет на диск сама и не ждёт результата, иначе широкий
// диапазон держал бы HTTP-запрос администратора часами.
//
// toExclusive - конец периода уже с поправкой на включительность даты (вызывающий
// прибавляет сутки к date_to): сервис сравнивает время без знания о том, как дата
// была введена в форме.
// ParsePeriod разбирает календарные границы бэкфилла в РАБОЧЕЙ таймзоне и отдаёт
// полуинтервал [from, toExclusive). Расчёт живёт в сервисе, а не в обработчике,
// потому что зона тут не украшение: раскладка кладёт заявку в каталог по местной
// дате подачи, и граница, посчитанная в UTC, отрезает не тот кусок суток - на
// московском смещении из периода выпадали бы заявки, поданные до трёх часов ночи
// первого дня, и добавлялись бы вечерние заявки следующего за периодом дня.
//
// date_to включителен: конец периода - начало следующих местных суток. AddDate, а не
// сложение с 24 часами: сутки с переводом часов короче или длиннее.
func (s *BlankExportService) ParsePeriod(dateFrom, dateTo string) (time.Time, time.Time, error) {
	loc := s.paths.Location()
	from, err := time.ParseInLocation("2006-01-02", dateFrom, loc)
	if err != nil {
		return time.Time{}, time.Time{}, echo.NewHTTPError(http.StatusBadRequest, "Некорректная дата date_from (ожидается YYYY-MM-DD)")
	}
	to, err := time.ParseInLocation("2006-01-02", dateTo, loc)
	if err != nil {
		return time.Time{}, time.Time{}, echo.NewHTTPError(http.StatusBadRequest, "Некорректная дата date_to (ожидается YYYY-MM-DD)")
	}
	if to.Before(from) {
		return time.Time{}, time.Time{}, echo.NewHTTPError(http.StatusBadRequest, "date_to не может быть раньше date_from")
	}
	return from, to.AddDate(0, 0, 1), nil
}

func (s *BlankExportService) Backfill(ctx context.Context, from, toExclusive time.Time, uniqueAttachmentID *int) (int, error) {
	settings, err := s.settings.GetArchiveSettings(ctx)
	if err != nil {
		return 0, err
	}
	if !settings.Enabled {
		return 0, ErrArchiveDisabled
	}

	selection := "SELECT a.id AS application_id FROM applications a WHERE a.sending_datetime >= ? AND a.sending_datetime < ?"
	args := []interface{}{from, toExclusive}
	if uniqueAttachmentID != nil {
		selection += " AND EXISTS (SELECT 1 FROM attachments att WHERE att.application_id = a.id AND att.unique_attachment_id = ?)"
		args = append(args, *uniqueAttachmentID)
	}
	selection += " ORDER BY application_id"
	var queued int64
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var enqueueErr error
		queued, enqueueErr = s.outbox.EnqueueSelectionTx(ctx, tx, selection, args, BlankExportReasonBackfill)
		return enqueueErr
	})
	if err != nil {
		return 0, fmt.Errorf("failed to enqueue archive backfill: %w", err)
	}
	s.outbox.Nudge()
	return int(queued), nil
}
