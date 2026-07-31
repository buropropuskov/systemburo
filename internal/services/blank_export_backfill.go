package services

import (
	"context"
	"fmt"
	"time"

	"systemburo/internal/models"
)

// Backfill ставит в очередь на выгрузку все заявки периода [from, toExclusive), по
// желанию суженные типом вложения. Разбор идёт фоновым воркером через обычную
// очередь (B1) - ручка не пишет на диск сама и не ждёт результата, иначе широкий
// диапазон держал бы HTTP-запрос администратора часами.
//
// toExclusive - конец периода уже с поправкой на включительность даты (вызывающий
// прибавляет сутки к date_to): сервис сравнивает время без знания о том, как дата
// была введена в форме.
func (s *BlankExportService) Backfill(ctx context.Context, from, toExclusive time.Time, uniqueAttachmentID *int) (int, error) {
	settings, err := s.settings.GetArchiveSettings(ctx)
	if err != nil {
		return 0, err
	}
	if !settings.Enabled {
		return 0, ErrArchiveDisabled
	}

	q := s.db.WithContext(ctx).Model(&models.Application{}).
		Where("sending_datetime >= ? AND sending_datetime < ?", from, toExclusive)
	if uniqueAttachmentID != nil {
		// EXISTS, а не JOIN: заявке с двумя вложениями искомого типа полагается одна
		// запись в очереди, а не по одной на каждое совпадение.
		q = q.Where(
			"EXISTS (SELECT 1 FROM attachments att WHERE att.application_id = applications.id AND att.unique_attachment_id = ?)",
			*uniqueAttachmentID,
		)
	}

	var ids []int
	if err := q.Pluck("id", &ids).Error; err != nil {
		return 0, fmt.Errorf("failed to select applications for archive backfill: %w", err)
	}

	s.EnqueueApplications(ids, BlankExportReasonBackfill)
	return len(ids), nil
}
