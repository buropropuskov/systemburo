package services

import (
	"context"
	"fmt"
	"log/slog"

	"gorm.io/gorm"
)

// TableAudience возвращает id пользователей, которым системная таблица проходной
// (tableID) видна, - аудиторию real-time сигнала tables.refresh (#840).
//
// Зеркалит authoritative-гейт видимости таблицы. Эндпоинты проходной
// (GET /system-tables, /cars/active-for-table/:id) не фильтруют по юзеру - они
// auth-only, а видимость КАЖДОЙ таблицы фронт гейтит правом table.<name>.view
// (NavMenu показывает пункт таблицы только при can(`table.${name}.view`)). Значит
// аудитория сигнала = все, у кого резолвер прав подтверждает это право.
//
// Резолвер - единственный источник истины (banned исключён, super/admin проходят,
// роль/группа/override учтены), поэтому аудиторию НЕ собираем реимплементацией
// предиката по grant-таблицам (урок #840: зеркалить authoritative-фильтр целиком,
// не его подмножество), а спрашиваем у того же резолвера, что стоит за FE can(...).
//
// Стоимость: резолвим всех активных юзеров, но резолвер кеширует набор на 30с, а
// хаб доставляет сигнал только онлайн-подписчикам - лишние id в аудитории
// безвредны (Publish для неподписанного userID это no-op). При росте числа юзеров
// возможна оптимизация пересечением с онлайн-набором хаба, но она за контрактом
// аудитории и не меняет его.
func TableAudience(ctx context.Context, db *gorm.DB, resolver *PermissionResolver, tableID int) ([]int, error) {
	userIDs, err := activeUserIDs(ctx, db)
	if err != nil {
		return nil, err
	}
	return tableAudienceFrom(ctx, db, resolver, tableID, userIDs)
}

// activeUserIDs - id всех активных аккаунтов (кандидаты любой аудитории, считаемой
// резолвером). Вынесено, чтобы при публикации на несколько таблиц сразу
// (publishTables) сканировать users ОДИН раз, а не по разу на таблицу.
func activeUserIDs(ctx context.Context, db *gorm.DB) ([]int, error) {
	var userIDs []int
	if err := db.WithContext(ctx).
		Table("users").
		Where("is_active = ?", true).
		Order("id").
		Pluck("id", &userIDs).Error; err != nil {
		return nil, fmt.Errorf("failed to load active users: %w", err)
	}
	return userIDs, nil
}

// tableAudienceFrom фильтрует переданных кандидатов по праву table.<name>.view
// таблицы tableID. candidateUserIDs обычно = activeUserIDs (переиспользуется между
// таблицами одного сигнала). resolver кеширует набор прав на 30с, поэтому повторные
// резолвы тех же юзеров для соседних таблиц - кеш-хиты.
func tableAudienceFrom(ctx context.Context, db *gorm.DB, resolver *PermissionResolver, tableID int, candidateUserIDs []int) ([]int, error) {
	var name string
	if err := db.WithContext(ctx).
		Table("system_tables").
		Select("name").
		Where("id = ?", tableID).
		Scan(&name).Error; err != nil {
		return nil, fmt.Errorf("failed to load table name for audience: %w", err)
	}
	if name == "" {
		// Таблицы нет (удалена/неверный id) - пустая аудитория, не ошибка: сигнал
		// best-effort, публиковать некому.
		return nil, nil
	}
	key := fmt.Sprintf("table.%s.view", name)

	audience := make([]int, 0, len(candidateUserIDs))
	for _, uid := range candidateUserIDs {
		set, err := resolver.Resolve(ctx, uid)
		if err != nil {
			// best-effort: сбой резолва одного юзера сужает аудиторию, но не должен
			// валить весь сигнал обновления - пропускаем его.
			slog.Warn("table audience: resolve failed", "user_id", uid, "table_id", tableID, "err", err)
			continue
		}
		if set.Has(key) {
			audience = append(audience, uid)
		}
	}
	return audience, nil
}
