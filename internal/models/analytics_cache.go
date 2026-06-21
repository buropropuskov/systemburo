package models

import "time"

// AnalyticsCache — персистентный снимок тяжёлой аналитики за период [from,to].
// Пишется сервисом статистики при пересчёте, читается при старте для прогрева
// in-memory кэша, чтобы дашборд/insights не считались с нуля после рестарта или
// деплоя. Ключ кодирует тип снимка и период (см. periodCache.key).
type AnalyticsCache struct {
	CacheKey   string    `gorm:"primaryKey;column:cache_key;size:200" json:"cache_key"`
	PeriodFrom time.Time `gorm:"column:period_from;not null" json:"period_from"`
	PeriodTo   time.Time `gorm:"column:period_to;not null" json:"period_to"`
	Payload    string    `gorm:"type:jsonb;not null" json:"payload"`
	ComputedAt time.Time `gorm:"column:computed_at;not null" json:"computed_at"`
}

// TableName задаёт имя таблицы.
func (AnalyticsCache) TableName() string { return "analytics_cache" }
