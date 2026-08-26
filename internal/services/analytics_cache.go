package services

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"systemburo/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// periodCache — тёплый кэш значения T, считаемого за период [from,to]. Держит
// результат in-memory для скорости и снимок в таблице analytics_cache для
// durability через рестарт. Фоновый refresh держит горячие ключи свежими и
// выселяет давно не используемые. Параметризован типом значения (summary/insights).
type periodCache[T any] struct {
	db      *gorm.DB
	name    string // префикс ключа и тип снимка: "summary" / "insights"
	evict   time.Duration
	compute func(ctx context.Context, from, to time.Time) (T, error)

	mu    sync.Mutex
	items map[string]*cacheItem[T]
}

type cacheItem[T any] struct {
	value      T
	from, to   time.Time
	computedAt time.Time
	lastAccess time.Time
}

func newPeriodCache[T any](db *gorm.DB, name string, evict time.Duration,
	compute func(context.Context, time.Time, time.Time) (T, error)) *periodCache[T] {
	return &periodCache[T]{
		db: db, name: name, evict: evict, compute: compute,
		items: make(map[string]*cacheItem[T]),
	}
}

func (c *periodCache[T]) key(from, to time.Time) string {
	return c.name + "|" + from.UTC().Format(time.RFC3339) + "|" + to.UTC().Format(time.RFC3339)
}

// get отдаёт кэшированное значение (refresh держит его свежим), а при первом
// обращении к периоду считает синхронно и сохраняет снимок.
func (c *periodCache[T]) get(ctx context.Context, from, to time.Time) (T, error) {
	k := c.key(from, to)
	now := time.Now()

	c.mu.Lock()
	if it, ok := c.items[k]; ok {
		it.lastAccess = now
		v := it.value
		c.mu.Unlock()
		return v, nil
	}
	c.mu.Unlock()

	v, err := c.compute(ctx, from, to)
	if err != nil {
		var zero T
		return zero, err
	}
	c.mu.Lock()
	c.items[k] = &cacheItem[T]{value: v, from: from, to: to, computedAt: now, lastAccess: now}
	c.mu.Unlock()
	c.persist(ctx, k, from, to, v, now)
	return v, nil
}

// persist делает upsert снимка в analytics_cache. Ошибку логируем, но не валим
// запрос - значение уже лежит в памяти.
func (c *periodCache[T]) persist(ctx context.Context, key string, from, to time.Time, v T, at time.Time) {
	if c.db == nil { // кэш без persistence (тесты/деградация) - только in-memory
		return
	}
	payload, err := json.Marshal(v)
	if err != nil {
		slog.Error("analytics cache marshal", "key", key, "error", err)
		return
	}
	row := models.AnalyticsCache{
		CacheKey: key, PeriodFrom: from, PeriodTo: to,
		Payload: string(payload), ComputedAt: at,
	}
	if err := c.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "cache_key"}},
		DoUpdates: clause.AssignmentColumns([]string{"payload", "computed_at", "period_from", "period_to"}),
	}).Create(&row).Error; err != nil {
		slog.Error("analytics cache persist", "key", key, "error", err)
	}
}

// warmup загружает снимки из БД в память при старте, чтобы дашборд не считался
// с нуля после рестарта/деплоя.
func (c *periodCache[T]) warmup(ctx context.Context) {
	if c.db == nil {
		return
	}
	var rows []models.AnalyticsCache
	if err := c.db.WithContext(ctx).Where("cache_key LIKE ?", c.name+"|%").Find(&rows).Error; err != nil {
		slog.Error("analytics cache warmup", "name", c.name, "error", err)
		return
	}
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, r := range rows {
		var v T
		if err := json.Unmarshal([]byte(r.Payload), &v); err != nil {
			slog.Error("analytics cache warmup unmarshal", "key", r.CacheKey, "error", err)
			continue
		}
		c.items[r.CacheKey] = &cacheItem[T]{
			value: v, from: r.PeriodFrom, to: r.PeriodTo,
			computedAt: r.ComputedAt, lastAccess: now,
		}
	}
	if len(rows) > 0 {
		slog.Info("analytics cache warmed", "name", c.name, "entries", len(rows))
	}
}

// refresh пересчитывает горячие ключи и выселяет давно не используемые (из памяти
// и БД). Ошибка пересчёта оставляет прошлое значение (stale-while-revalidate).
func (c *periodCache[T]) refresh(ctx context.Context) {
	now := time.Now()
	c.mu.Lock()
	snapshot := make(map[string]*cacheItem[T], len(c.items))
	for k, v := range c.items {
		snapshot[k] = v
	}
	c.mu.Unlock()

	for k, it := range snapshot {
		if now.Sub(it.lastAccess) > c.evict {
			c.mu.Lock()
			delete(c.items, k)
			c.mu.Unlock()
			if c.db != nil {
				if err := c.db.WithContext(ctx).Where("cache_key = ?", k).Delete(&models.AnalyticsCache{}).Error; err != nil {
					slog.Error("analytics cache evict", "key", k, "error", err)
				}
			}
			continue
		}
		v, err := c.compute(ctx, it.from, it.to)
		if err != nil {
			slog.Error("analytics cache refresh", "key", k, "error", err)
			continue
		}
		c.mu.Lock()
		if cur, ok := c.items[k]; ok {
			cur.value = v
			cur.computedAt = now
		}
		c.mu.Unlock()
		c.persist(ctx, k, it.from, it.to, v, now)
	}
}
