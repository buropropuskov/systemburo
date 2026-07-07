package services

import (
	"context"
	"fmt"
	"log/slog"

	"gorm.io/gorm"

	"systemburo/internal/models"
	"systemburo/internal/realtime"
)

// TablesRefreshPublisher публикует real-time сигнал tables.refresh аудитории
// затронутых таблиц проходной (#840). Событие лёгкое ("сходи обнови") - клиент,
// получив его, делает обычный fetch с правами. Аудитория каждой таблицы -
// TableAudience (зеркало права table.<name>.view, по которому фронт показывает
// таблицу).
//
// Scope сигнала - tables:<tableID>: каждая таблица проходной на фронте
// подписывается на свой id. Куда попадает изменённая строка:
//   - Машины: эндпоинт /cars/active-for-tables НЕ scoped по таблице - строка
//     активной машины видна в КАЖДОЙ cars-таблице, поэтому любое изменение машины
//     (активация при принятии, въезд/выезд) обновляет ВСЕ cars-таблицы.
//   - Сотрудники: /employees/active-for-table/:table_id scoped - сотрудник виден
//     только в своих целевых таблицах (employee_target_tables), туда и шлём.
//
// Best-effort: сбой вычисления аудитории/публикации не прерывает бизнес-операцию
// (активацию/въезд), лишь пропускает сигнал - клиент подстрахуется fallback-
// поллингом. Все методы безопасны на nil-получателе (продюсер не инжектирован в
// тестах/offline): проверяют p == nil до обращения к полям.
type TablesRefreshPublisher struct {
	db        *gorm.DB
	resolver  *PermissionResolver
	publisher realtime.Publisher
}

// NewTablesRefreshPublisher создаёт продюсер сигналов обновления таблиц проходной.
func NewTablesRefreshPublisher(db *gorm.DB, resolver *PermissionResolver, publisher realtime.Publisher) *TablesRefreshPublisher {
	return &TablesRefreshPublisher{db: db, resolver: resolver, publisher: publisher}
}

// NotifyCarsChanged публикует обновление всем активным cars-таблицам: строка
// машины видна в каждой из них, поэтому изменение любой машины обновляет их все.
func (p *TablesRefreshPublisher) NotifyCarsChanged(ctx context.Context) {
	if p == nil || p.publisher == nil {
		return
	}
	ids, err := p.carsTableIDs(ctx)
	if err != nil {
		slog.Warn("tables.refresh: load cars tables failed", "err", err)
		return
	}
	p.publishTables(ctx, ids)
}

// NotifyEmployeeChanged публикует обновление целевым таблицам сотрудника
// (employee_target_tables) - только там он показан.
func (p *TablesRefreshPublisher) NotifyEmployeeChanged(ctx context.Context, employeeID int) {
	if p == nil || p.publisher == nil {
		return
	}
	ids, err := p.employeeTargetTableIDs(ctx, []int{employeeID})
	if err != nil {
		slog.Warn("tables.refresh: load employee tables failed", "employee_id", employeeID, "err", err)
		return
	}
	p.publishTables(ctx, ids)
}

// NotifyApplicationActivated публикует обновление таблицам, затронутым принятием
// заявки: если активированы машины - всем cars-таблицам; для активированных
// сотрудников - их целевым таблицам. Один момент принятия рождает сигналы всем
// местам, где новые строки появляются live.
func (p *TablesRefreshPublisher) NotifyApplicationActivated(ctx context.Context, applicationID int) {
	if p == nil || p.publisher == nil {
		return
	}

	var tableIDs []int

	var hasCars bool
	if err := p.db.WithContext(ctx).Raw(
		`SELECT EXISTS(SELECT 1 FROM attachments WHERE application_id = ? AND attachment_type = 'cars')`,
		applicationID).Scan(&hasCars).Error; err != nil {
		slog.Warn("tables.refresh: application cars check failed", "application_id", applicationID, "err", err)
	} else if hasCars {
		if ids, err := p.carsTableIDs(ctx); err != nil {
			slog.Warn("tables.refresh: load cars tables failed", "err", err)
		} else {
			tableIDs = append(tableIDs, ids...)
		}
	}

	var employeeIDs []int
	if err := p.db.WithContext(ctx).Raw(
		`SELECT e.id FROM employees e
		 JOIN attachments a ON a.id = e.attachment_id
		 WHERE a.application_id = ? AND a.attachment_type = 'people'`,
		applicationID).Scan(&employeeIDs).Error; err != nil {
		slog.Warn("tables.refresh: load application employees failed", "application_id", applicationID, "err", err)
	} else if len(employeeIDs) > 0 {
		if ids, err := p.employeeTargetTableIDs(ctx, employeeIDs); err != nil {
			slog.Warn("tables.refresh: load employee tables failed", "err", err)
		} else {
			tableIDs = append(tableIDs, ids...)
		}
	}

	p.publishTables(ctx, tableIDs)
}

// publishTables для каждой уникальной таблицы вычисляет аудиторию и шлёт ей
// tables.refresh со scope tables:<id>. Пустая аудитория пропускается (публиковать
// некому).
func (p *TablesRefreshPublisher) publishTables(ctx context.Context, tableIDs []int) {
	if len(tableIDs) == 0 {
		// Нечего обновлять (напр. изменение машины при отсутствии активных
		// cars-таблиц) - не сканируем юзеров впустую.
		return
	}
	// Список активных юзеров одинаков для всех таблиц сигнала - сканируем users один
	// раз и переиспользуем как кандидатов для каждой таблицы (аудитория считается
	// резолвером, кеш прав 30с). Раньше TableAudience делал этот скан на каждую таблицу.
	candidates, err := activeUserIDs(ctx, p.db)
	if err != nil {
		slog.Warn("tables.refresh: load active users failed", "err", err)
		return
	}

	seen := make(map[int]struct{}, len(tableIDs))
	for _, tableID := range tableIDs {
		if tableID <= 0 {
			continue
		}
		if _, dup := seen[tableID]; dup {
			continue
		}
		seen[tableID] = struct{}{}

		audience, err := tableAudienceFrom(ctx, p.db, p.resolver, tableID, candidates)
		if err != nil {
			slog.Warn("tables.refresh: audience failed", "table_id", tableID, "err", err)
			continue
		}
		if len(audience) == 0 {
			continue
		}
		p.publisher.PublishMany(audience, realtime.Event{
			Type:  "tables.refresh",
			Scope: fmt.Sprintf("tables:%d", tableID),
		})
	}
}

// carsTableIDs - id всех активных таблиц типа cars.
func (p *TablesRefreshPublisher) carsTableIDs(ctx context.Context) ([]int, error) {
	var ids []int
	if err := p.db.WithContext(ctx).
		Table("system_tables").
		Where("table_type = ? AND is_active = ?", models.TableTypeCars, true).
		Pluck("id", &ids).Error; err != nil {
		return nil, fmt.Errorf("failed to load cars table ids: %w", err)
	}
	return ids, nil
}

// employeeTargetTableIDs - id целевых таблиц переданных сотрудников (без дублей).
func (p *TablesRefreshPublisher) employeeTargetTableIDs(ctx context.Context, employeeIDs []int) ([]int, error) {
	if len(employeeIDs) == 0 {
		return nil, nil
	}
	var ids []int
	if err := p.db.WithContext(ctx).
		Table("employee_target_tables").
		Where("employee_id IN ?", employeeIDs).
		Distinct("table_id").
		Pluck("table_id", &ids).Error; err != nil {
		return nil, fmt.Errorf("failed to load employee target table ids: %w", err)
	}
	return ids, nil
}
