package services

import (
	"context"
	"fmt"
	"log/slog"

	"gorm.io/gorm"

	"systemburo/internal/realtime"
)

// AvailableRefreshPublisher публикует real-time сигнал available.new аудитории
// вкладки "Доступные мне" охраны (#840 V3), когда заявка становится согласованной
// и её вложения появляются в списке доступных.
//
// Сигнал безданных (event-then-fetch): клиент, получив его, делает обычный fetch,
// который сам применяет per-user фильтр видимости (securityVisibilityWhere). Поэтому
// аудитория - БЕЗОПАСНЫЙ суперсет: все, кто вообще имеет доступ к вкладке (носители
// права page.available - super/admin/грант - ЛИБО тип "Охранник"), зеркало гейта
// эндпоинта /applications/available-attachments (#976). Пересечение мест НЕ считаем:
// сузить аудиторию значило бы повторить рискованный place-SQL (уроки #706/#951) и
// рискнуть промахнуться мимо получателя (урок #840 - не сужать authoritative-фильтр);
// лишний сигнал безвреден (клиент рефетчит, у него ничего нового - список тот же).
//
// Best-effort и nil-safe: сбой аудитории не рвёт согласование, методы на nil-
// получателе (без инжекта) не паникуют.
type AvailableRefreshPublisher struct {
	db        *gorm.DB
	resolver  *PermissionResolver
	publisher realtime.Publisher
}

// NewAvailableRefreshPublisher создаёт продюсер сигналов обновления "Доступные мне".
func NewAvailableRefreshPublisher(db *gorm.DB, resolver *PermissionResolver, publisher realtime.Publisher) *AvailableRefreshPublisher {
	return &AvailableRefreshPublisher{db: db, resolver: resolver, publisher: publisher}
}

// NotifyAvailableChanged публикует available.new аудитории вкладки "Доступные мне".
// Зовётся, когда заявка стала согласованной (её вложения теперь доступны охране).
func (p *AvailableRefreshPublisher) NotifyAvailableChanged(ctx context.Context) {
	if p == nil || p.publisher == nil {
		return
	}
	audience, err := p.availableAudience(ctx)
	if err != nil {
		slog.Warn("available.new: audience failed", "err", err)
		return
	}
	if len(audience) == 0 {
		return
	}
	p.publisher.PublishMany(audience, realtime.Event{Type: "available.new", Scope: "available"})
}

// availableAudience - id активных юзеров, имеющих доступ к вкладке "Доступные мне":
// носители права page.available (резолвер: super/admin/грант) ИЛИ тип "Охранник"
// (user_types.code='security'). Зеркалит гейт requireSecurityOrAdmin эндпоинта (#976).
func (p *AvailableRefreshPublisher) availableAudience(ctx context.Context) ([]int, error) {
	type userRow struct {
		ID   int
		Code string
	}
	var rows []userRow
	// is_active=true: архивным аккаунтам сигнал не шлём (живой сессии у них нет).
	// Гейт эндпоинта is_active не смотрит, но это сужение безвредно - в худшем
	// случае деградация до pre-V3 (обновление по F5), а не потеря доступа.
	if err := p.db.WithContext(ctx).
		Table("users").
		Select("users.id, user_types.code").
		Joins("JOIN user_types ON user_types.id = users.type_id").
		Where("users.is_active = ?", true).
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to load users for available audience: %w", err)
	}

	audience := make([]int, 0, len(rows))
	for _, u := range rows {
		if u.Code == securityUserTypeCode {
			audience = append(audience, u.ID)
			continue
		}
		set, err := p.resolver.Resolve(ctx, u.ID)
		if err != nil {
			slog.Warn("available.new: resolve failed", "user_id", u.ID, "err", err)
			continue
		}
		if set.Has(KeyPageAvailable) {
			audience = append(audience, u.ID)
		}
	}
	return audience, nil
}
