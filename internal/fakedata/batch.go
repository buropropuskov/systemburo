package fakedata

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"systemburo/internal/models"

	"gorm.io/gorm"
)

// itemInsertChunk -- по столько строк реестра уходит в базу за раз. Крупная партия
// заводит десятки тысяч записей, и вставка по одной заметно дольше самой наливки.
const itemInsertChunk = 500

// Batch -- открытая партия: всё созданное регистрируется в ней, и по этому перечню
// партия потом удаляется целиком.
type Batch struct {
	db     *gorm.DB
	record models.FakeBatch
	counts map[string]int
	marks  map[string]int
}

// OpenBatch заводит партию. Пустая метка означает «назвать по моменту запуска».
func OpenBatch(ctx context.Context, db *gorm.DB, label string, seed int64, profile string) (*Batch, error) {
	label = strings.TrimSpace(label)
	if label == "" {
		label = "fake-" + time.Now().UTC().Format("20060102-150405")
	}
	record := models.FakeBatch{
		Label:     label,
		Seed:      seed,
		Profile:   profile,
		CreatedAt: time.Now().UTC(),
		Summary:   "{}",
	}
	if err := db.WithContext(ctx).Create(&record).Error; err != nil {
		return nil, fmt.Errorf("не удалось завести партию %q: %w", label, err)
	}
	return &Batch{db: db, record: record, counts: make(map[string]int), marks: make(map[string]int)}, nil
}

// ID партии.
func (b *Batch) ID() int { return b.record.ID }

// Label партии.
func (b *Batch) Label() string { return b.record.Label }

// Add регистрирует созданные записи одного вида. Entity берётся из констант
// models.AuditEntity*.
func (b *Batch) Add(ctx context.Context, entity string, ids ...int) error {
	if len(ids) == 0 {
		return nil
	}
	items := make([]models.FakeBatchItem, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			// Идентификатор, не полученный из базы, в реестр не пишем: по нему нечего
			// удалять, а незамеченный ноль превратил бы удаление партии в удаление
			// всего, что попало под условие entity_id = 0.
			return fmt.Errorf("в партию %q передан недопустимый идентификатор %d для %q", b.record.Label, id, entity)
		}
		items = append(items, models.FakeBatchItem{BatchID: b.record.ID, Entity: entity, EntityID: id})
	}
	if err := b.db.WithContext(ctx).CreateInBatches(items, itemInsertChunk).Error; err != nil {
		return fmt.Errorf("не удалось записать в партию %d записей %q: %w", len(ids), entity, err)
	}
	b.counts[entity] += len(ids)
	return nil
}

// Counts -- сколько чего создано в партии.
func (b *Batch) Counts() map[string]int {
	out := make(map[string]int, len(b.counts))
	for entity, n := range b.counts {
		out[entity] = n
	}
	return out
}

// Mark считает ДЕЙСТВИЯ шага над уже существующими записями -- отметки въезда и выезда
// (passagesStep). В перечень партии они не идут: удалять там нечего, машина и сотрудник
// принадлежат заявке и уходят вместе с ней, а сама отметка живёт в журнале.
//
// Считать их всё равно надо: без этого предварительный показ обещал отметки прохода, а
// отчёт о наливке молчал о них вовсе -- и человек видел «создастся 1465, создано 963»
// без объяснения, куда делась разница.
func (b *Batch) Mark(entity string, n int) {
	if n <= 0 {
		return
	}
	b.marks[entity] += n
}

// Marks -- сколько записей каких видов отмечено (не создано).
func (b *Batch) Marks() map[string]int {
	out := make(map[string]int, len(b.marks))
	for entity, n := range b.marks {
		out[entity] = n
	}
	return out
}

// Total -- сколько записей создано всего.
func (b *Batch) Total() int {
	total := 0
	for _, n := range b.counts {
		total += n
	}
	return total
}

// Close записывает сводку партии.
//
// В сводку идут только созданные записи: по ней считается объём партии в перечне
// (-list), а он должен совпадать с тем, что удалится. Отметки прохода (Mark) в неё не
// попадают -- они не записи партии, показываются в отчёте о наливке отдельной таблицей
// и остаются в журнале.
func (b *Batch) Close(ctx context.Context) error {
	payload, err := json.Marshal(b.counts)
	if err != nil {
		return fmt.Errorf("сводка партии не собрана: %w", err)
	}
	if err := b.db.WithContext(ctx).Model(&models.FakeBatch{}).
		Where("id = ?", b.record.ID).
		Update("summary", string(payload)).Error; err != nil {
		return fmt.Errorf("сводка партии не сохранена: %w", err)
	}
	b.record.Summary = string(payload)
	return nil
}

// ListBatches отдаёт партии, свежие первыми.
func ListBatches(ctx context.Context, db *gorm.DB) ([]models.FakeBatch, error) {
	var batches []models.FakeBatch
	if err := db.WithContext(ctx).Order("created_at DESC, id DESC").Find(&batches).Error; err != nil {
		return nil, fmt.Errorf("не удалось прочитать список партий: %w", err)
	}
	return batches, nil
}

// FindBatch ищет партию по метке.
func FindBatch(ctx context.Context, db *gorm.DB, label string) (*models.FakeBatch, error) {
	var batch models.FakeBatch
	err := db.WithContext(ctx).Where("label = ?", strings.TrimSpace(label)).First(&batch).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("партия %q не найдена", label)
	}
	if err != nil {
		return nil, fmt.Errorf("не удалось прочитать партию %q: %w", label, err)
	}
	return &batch, nil
}

// SummaryCounts разбирает сводку партии для вывода. Битую сводку показываем пустой:
// список партий не то место, где стоит падать из-за испорченной строки.
func SummaryCounts(batch models.FakeBatch) map[string]int {
	counts := make(map[string]int)
	if strings.TrimSpace(batch.Summary) == "" {
		return counts
	}
	if err := json.Unmarshal([]byte(batch.Summary), &counts); err != nil {
		return make(map[string]int)
	}
	return counts
}

// SortedEntities -- виды сущностей сводки в устойчивом порядке, чтобы вывод команды
// не менялся от запуска к запуску.
func SortedEntities(counts map[string]int) []string {
	entities := make([]string, 0, len(counts))
	for entity := range counts {
		entities = append(entities, entity)
	}
	sort.Strings(entities)
	return entities
}
