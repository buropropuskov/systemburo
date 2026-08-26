package services

import (
	"context"
	"fmt"
	"log/slog"

	"gorm.io/gorm"

	"systemburo/internal/realtime"
)

// TablesRefreshPublisher публикует real-time сигнал tables.refresh аудитории
// затронутых таблиц проходной (#840). Событие лёгкое ("сходи обнови") - клиент,
// получив его, делает обычный fetch с правами. Аудитория каждой таблицы -
// TableAudience (зеркало права table.<name>.view, по которому фронт показывает
// таблицу).
//
// Scope сигнала - tables:<tableID>: каждая таблица проходной на фронте
// подписывается на свой id. Куда попадает изменённая строка (обе сущности scoped по
// целевым таблицам, поэтому сигнал идёт только затронутым):
//   - Машины: видны в выбранных таблицах «Проезд» (car_target_tables, #1036) - шлём
//     по ним (carTargetTableIDs), а не во все cars-таблицы разом.
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

// NotifyCarsChanged публикует обновление таблицам «Проезд» изменённой машины
// (#1036): строка видна только в выбранных cars-таблицах, туда и шлём сигнал.
func (p *TablesRefreshPublisher) NotifyCarsChanged(ctx context.Context, carID int) {
	if p == nil || p.publisher == nil {
		return
	}
	ids, err := p.carTargetTableIDs(ctx, []int{carID})
	if err != nil {
		slog.Warn("tables.refresh: load car tables failed", "car_id", carID, "err", err)
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

// NotifyCarsChangedBatch публикует обновление таблицам «Проезд» пачки изменённых
// машин одним проходом (дедуп таблиц внутри publishTables). Используется ручным
// добавлением (#1049): у сироты нет applicationId, поэтому аудитория считается по
// car_target_tables машин - той же связи, по которой строка видна в таблице.
func (p *TablesRefreshPublisher) NotifyCarsChangedBatch(ctx context.Context, carIDs []int) {
	if p == nil || p.publisher == nil || len(carIDs) == 0 {
		return
	}
	ids, err := p.carTargetTableIDs(ctx, carIDs)
	if err != nil {
		slog.Warn("tables.refresh: load car tables failed", "car_ids", carIDs, "err", err)
		return
	}
	p.publishTables(ctx, ids)
}

// NotifyEmployeesChangedBatch - зеркало NotifyCarsChangedBatch для пачки сотрудников
// (ручное добавление #1049): аудитория по employee_target_tables, без заявки.
func (p *TablesRefreshPublisher) NotifyEmployeesChangedBatch(ctx context.Context, employeeIDs []int) {
	if p == nil || p.publisher == nil || len(employeeIDs) == 0 {
		return
	}
	ids, err := p.employeeTargetTableIDs(ctx, employeeIDs)
	if err != nil {
		slog.Warn("tables.refresh: load employee tables failed", "employee_ids", employeeIDs, "err", err)
		return
	}
	p.publishTables(ctx, ids)
}

// NotifyTables публикует tables.refresh явному набору таблиц вместо вычисления
// аудитории из текущего членства car_target_tables/employee_target_tables (#1194).
// Нужен операциям, которые МЕНЯЮТ сами эти привязки (bulk перенос/добавление/снятие):
// после коммита таблица-источник (from/unbind) уже не содержит сущность, поэтому
// NotifyCarsChangedBatch её бы не увидел, а её зрителям как раз нужно обновиться,
// чтобы строка live исчезла.
func (p *TablesRefreshPublisher) NotifyTables(ctx context.Context, tableIDs []int) {
	if p == nil || p.publisher == nil || len(tableIDs) == 0 {
		return
	}
	p.publishTables(ctx, tableIDs)
}

// NotifyApplicationActivated публикует обновление таблицам, затронутым принятием
// заявки: активированным машинам и сотрудникам - их целевым таблицам («Проезд» /
// «Места прохода»). Один момент принятия рождает сигналы всем местам, где новые
// строки появляются live.
func (p *TablesRefreshPublisher) NotifyApplicationActivated(ctx context.Context, applicationID int) {
	if p == nil || p.publisher == nil {
		return
	}

	var tableIDs []int

	var carIDs []int
	if err := p.db.WithContext(ctx).Raw(
		`SELECT c.id FROM cars c
		 JOIN attachments a ON a.id = c.attachment_id
		 WHERE a.application_id = ? AND a.attachment_type = 'cars'`,
		applicationID).Scan(&carIDs).Error; err != nil {
		slog.Warn("tables.refresh: load application cars failed", "application_id", applicationID, "err", err)
	} else if len(carIDs) > 0 {
		if ids, err := p.carTargetTableIDs(ctx, carIDs); err != nil {
			slog.Warn("tables.refresh: load car tables failed", "err", err)
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

// carTargetTableIDs - id таблиц «Проезд» переданных машин (без дублей, #1036).
func (p *TablesRefreshPublisher) carTargetTableIDs(ctx context.Context, carIDs []int) ([]int, error) {
	if len(carIDs) == 0 {
		return nil, nil
	}
	var ids []int
	if err := p.db.WithContext(ctx).
		Table("car_target_tables").
		Where("car_id IN ?", carIDs).
		Distinct("table_id").
		Pluck("table_id", &ids).Error; err != nil {
		return nil, fmt.Errorf("failed to load car target table ids: %w", err)
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
