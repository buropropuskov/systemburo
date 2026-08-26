package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"systemburo/internal/export"
	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// snapshotCarLister, snapshotEmployeeLister, snapshotEmployeeStatuser — минимальные
// контракты, которые снимок берёт у существующих сервисов. Снимок обязан отдавать
// РОВНО тот набор строк, что показывает страница таблицы, поэтому переиспользует их
// листинги, а не пишет свой SQL.
type snapshotCarLister interface {
	GetActiveCarsForTable(ctx context.Context, tableID int) ([]TableCarResponse, error)
	GetFactCarsForTable(ctx context.Context, tableID int) ([]TableCarResponse, error)
}

type snapshotEmployeeLister interface {
	GetActiveEmployeesForTable(ctx context.Context, tableID int) ([]TableEmployeeResponse, error)
}

type snapshotEmployeeStatuser interface {
	GetCurrentStatus(ctx context.Context) ([]EmployeeCurrentStatus, error)
}

// TableSnapshotService снимает и хранит слепки состояния таблиц.
type TableSnapshotService interface {
	// SnapshotTable сохраняет текущее состояние таблицы (строки со статусами) как
	// новую версию и возвращает её id. reason - scheduled|manual.
	SnapshotTable(ctx context.Context, tableID int, reason string, actorUserID *int) (int, error)
	// SnapshotAllActiveTables снимает слепок каждой активной cars/people-таблицы.
	// Провал одной таблицы логируется и не прерывает остальные: created/failed -
	// сколько удалось/сорвалось. err возвращается только если не удалось получить
	// сам список таблиц (иначе дневная джоба должна дойти до сброса статусов).
	SnapshotAllActiveTables(ctx context.Context, reason string) (created, failed int, err error)
	// ListSnapshots отдаёт версии таблицы (метаданные без тяжёлого payload):
	// дата, причина, автор, агрегаты. Пагинация + опциональный фильтр периода
	// [from, to] по taken_at. Возвращает страницу и общее число под фильтром.
	ListSnapshots(ctx context.Context, tableID int, from, to *time.Time, page, perPage int) ([]SnapshotListItem, int64, error)
	// GetSnapshot отдаёт одну версию с полным payload. Скоуплено по tableID -
	// чужой sid другой таблицы = 404 (не даём читать снимок вне таблицы из URL).
	GetSnapshot(ctx context.Context, tableID, snapshotID int) (*models.TableSnapshot, error)
	// DeleteSnapshotsOlderThan удаляет версии таблицы старше months месяцев и
	// возвращает число удалённых. Свежие (>= порога) остаются. months > 0.
	DeleteSnapshotsOlderThan(ctx context.Context, tableID, months int) (int64, error)
	// BuildSnapshotExport собирает формат-нейтральные табличные данные версии для
	// выгрузки (Excel/PDF). snapshotID == nil - текущее состояние таблицы (тот же
	// слепок, что снял бы снимок сейчас, без записи в БД). Возвращает готовую таблицу
	// и ASCII-базу имени файла (без расширения) для Content-Disposition.
	BuildSnapshotExport(ctx context.Context, tableID int, snapshotID *int) (export.Table, string, error)
}

// SnapshotListItem - строка списка версий: метаданные без payload (он тяжёлый и
// нужен только при открытии конкретной версии). Counts распакованы для UI.
type SnapshotListItem struct {
	ID          int                   `json:"id"`
	TableID     int                   `json:"table_id"`
	TakenAt     time.Time             `json:"taken_at"`
	Reason      string                `json:"reason"`
	ActorUserID *int                  `json:"actor_user_id,omitempty"`
	ActorName   string                `json:"actor_name,omitempty"`
	Counts      models.SnapshotCounts `json:"counts"`
}

type tableSnapshotService struct {
	db        *gorm.DB
	cars      snapshotCarLister
	employees snapshotEmployeeLister
	empStatus snapshotEmployeeStatuser
}

// NewTableSnapshotService собирает сервис снимков поверх листингов машин/сотрудников.
func NewTableSnapshotService(db *gorm.DB, cars snapshotCarLister, employees snapshotEmployeeLister, empStatus snapshotEmployeeStatuser) TableSnapshotService {
	return &tableSnapshotService{db: db, cars: cars, employees: employees, empStatus: empStatus}
}

// snapshotEmployeeRow — строка сотрудника в слепке: те же поля, что отдаёт страница,
// плюс territory_status (страница подмешивает его отдельным вызовом current-status).
type snapshotEmployeeRow struct {
	TableEmployeeResponse
	TerritoryStatus *int `json:"territory_status"`
}

// snapshotCarRow — строка машины в слепке: те же поля, что отдаёт страница, плюс
// is_fact - машина из блока «по факту», который страница показывает при show_fact_table.
type snapshotCarRow struct {
	TableCarResponse
	IsFact bool `json:"is_fact"`
}

func (s *tableSnapshotService) SnapshotTable(ctx context.Context, tableID int, reason string, actorUserID *int) (int, error) {
	if reason != models.SnapshotReasonScheduled && reason != models.SnapshotReasonManual {
		return 0, echo.NewHTTPError(http.StatusBadRequest, "Invalid snapshot reason")
	}

	var table models.SystemTable
	if err := s.db.WithContext(ctx).Select("id", "table_type", "show_fact_table").First(&table, tableID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, echo.NewHTTPError(http.StatusNotFound, "Table not found")
		}
		return 0, fmt.Errorf("failed to load table %d for snapshot: %w", tableID, err)
	}

	rowsJSON, counts, err := s.collectRows(ctx, table)
	if err != nil {
		return 0, err
	}
	fields, err := s.collectFields(ctx, tableID)
	if err != nil {
		return 0, err
	}

	payloadJSON, err := json.Marshal(models.SnapshotPayload{TableType: table.TableType, Rows: rowsJSON, Fields: fields})
	if err != nil {
		return 0, fmt.Errorf("failed to marshal snapshot payload: %w", err)
	}
	countsJSON, err := json.Marshal(counts)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal snapshot counts: %w", err)
	}

	snap := models.TableSnapshot{
		TableID:     tableID,
		TakenAt:     time.Now().UTC(),
		Reason:      reason,
		ActorUserID: actorUserID,
		Payload:     payloadJSON,
		Counts:      countsJSON,
	}
	if err := s.db.WithContext(ctx).Create(&snap).Error; err != nil {
		return 0, fmt.Errorf("failed to create table snapshot for table %d: %w", tableID, err)
	}
	return snap.ID, nil
}

// SnapshotAllActiveTables снимает слепок каждой активной cars/people-таблицы.
// Провал одной таблицы логируется и не прерывает остальные - дневная джоба обязана
// дойти до сброса статусов, поэтому per-table ошибки не всплывают наверх.
func (s *tableSnapshotService) SnapshotAllActiveTables(ctx context.Context, reason string) (created, failed int, err error) {
	var tables []models.SystemTable
	if err := s.db.WithContext(ctx).
		Select("id").
		Where("is_active = ?", true).
		Where("table_type IN ?", []string{models.TableTypeCars, models.TableTypePeople}).
		Find(&tables).Error; err != nil {
		return 0, 0, fmt.Errorf("failed to list active tables for snapshot: %w", err)
	}
	for _, t := range tables {
		if _, serr := s.SnapshotTable(ctx, t.ID, reason, nil); serr != nil {
			failed++
			slog.Error("не удалось снять слепок таблицы", "table_id", t.ID, "reason", reason, "error", serr)
			continue
		}
		created++
	}
	return created, failed, nil
}

// collectRows строит слепок строк таблицы и агрегаты по её типу. Возвращает сырой JSON
// строк (для payload) и подсчитанные counts.
func (s *tableSnapshotService) collectRows(ctx context.Context, table models.SystemTable) (json.RawMessage, models.SnapshotCounts, error) {
	switch table.TableType {
	case models.TableTypeCars:
		// Машины скоуплены по «Проезду» (table_id, #1036) - как и страница cars-таблицы;
		// territory_status уже в TableCarResponse. Если таблица показывает блок «по факту»
		// (show_fact_table), подмешиваем его тем же листингом, что и страница, помечая
		// строки is_fact - иначе слепок терял бы машины «по факту», стоявшие на территории.
		cars, err := s.cars.GetActiveCarsForTable(ctx, table.ID)
		if err != nil {
			return nil, models.SnapshotCounts{}, fmt.Errorf("failed to list cars for snapshot: %w", err)
		}
		rows := make([]snapshotCarRow, 0, len(cars))
		statuses := make([]*int, 0, len(cars))
		for _, c := range cars {
			rows = append(rows, snapshotCarRow{TableCarResponse: c})
			statuses = append(statuses, c.TerritoryStatus)
		}
		if table.ShowFactTable {
			facts, err := s.cars.GetFactCarsForTable(ctx, table.ID)
			if err != nil {
				return nil, models.SnapshotCounts{}, fmt.Errorf("failed to list fact cars for snapshot: %w", err)
			}
			for _, c := range facts {
				rows = append(rows, snapshotCarRow{TableCarResponse: c, IsFact: true})
				statuses = append(statuses, c.TerritoryStatus)
			}
		}
		raw, err := json.Marshal(rows)
		if err != nil {
			return nil, models.SnapshotCounts{}, fmt.Errorf("failed to marshal car rows: %w", err)
		}
		return raw, computeSnapshotCounts(statuses), nil

	case models.TableTypePeople:
		// Сотрудники скоуплены по table_id; territory_status страница берёт отдельным
		// current-status, здесь подмешиваем его тем же источником (колонка territory_status).
		emps, err := s.employees.GetActiveEmployeesForTable(ctx, table.ID)
		if err != nil {
			return nil, models.SnapshotCounts{}, fmt.Errorf("failed to list employees for snapshot: %w", err)
		}
		statusList, err := s.empStatus.GetCurrentStatus(ctx)
		if err != nil {
			return nil, models.SnapshotCounts{}, fmt.Errorf("failed to list employee statuses for snapshot: %w", err)
		}
		statusByID := make(map[int]int, len(statusList))
		for _, st := range statusList {
			statusByID[st.EmployeeID] = st.TerritoryStatus
		}

		rows := make([]snapshotEmployeeRow, len(emps))
		statuses := make([]*int, len(emps))
		for i, emp := range emps {
			var ts *int
			if v, ok := statusByID[emp.ID]; ok {
				ts = &v
			}
			rows[i] = snapshotEmployeeRow{TableEmployeeResponse: emp, TerritoryStatus: ts}
			statuses[i] = ts
		}
		raw, err := json.Marshal(rows)
		if err != nil {
			return nil, models.SnapshotCounts{}, fmt.Errorf("failed to marshal employee rows: %w", err)
		}
		return raw, computeSnapshotCounts(statuses), nil

	default:
		return nil, models.SnapshotCounts{}, echo.NewHTTPError(http.StatusUnprocessableEntity, "Unsupported table type for snapshot")
	}
}

// collectFields снимает настройку колонок таблицы (видимость/порядок/ширина) в порядке
// display_order - чтобы просмотр версии отрисовал те же столбцы, что были настроены на
// момент слепка (самодостаточность снимка). Скрытые поля тоже сохраняются: их видимость
// - часть состояния, а решение показать/скрыть остаётся за рендером страницы.
func (s *tableSnapshotService) collectFields(ctx context.Context, tableID int) ([]models.SnapshotField, error) {
	var fields []models.TableField
	if err := s.db.WithContext(ctx).
		Where("table_id = ?", tableID).
		Order("display_order ASC NULLS LAST, id ASC").
		Find(&fields).Error; err != nil {
		return nil, fmt.Errorf("failed to load fields for snapshot of table %d: %w", tableID, err)
	}
	out := make([]models.SnapshotField, len(fields))
	for i, f := range fields {
		out[i] = models.SnapshotField{
			FieldName:          f.FieldName,
			FieldType:          f.FieldType,
			DisplayOrder:       f.DisplayOrder,
			IsVisible:          f.IsVisible,
			Width:              f.Width,
			Priority:           f.Priority,
			EnlargedIsVisible:  f.EnlargedIsVisible,
			EnlargedWidth:      f.EnlargedWidth,
			EnlargedFontWeight: f.EnlargedFontWeight,
		}
	}
	return out, nil
}

// snapshotListRow - строка выборки списка версий с приклеенным автором (LEFT JOIN
// users). Payload намеренно не выбирается - он тяжёлый и в списке не нужен.
type snapshotListRow struct {
	ID             int
	TableID        int
	TakenAt        time.Time
	Reason         string
	ActorUserID    *int
	Counts         json.RawMessage
	ActorFirstName *string
	ActorLastName  *string
	ActorUsername  *string
}

func (s *tableSnapshotService) ListSnapshots(ctx context.Context, tableID int, from, to *time.Time, page, perPage int) ([]SnapshotListItem, int64, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}

	// Единый набор условий для Count и выборки - строится заново на каждый запрос,
	// чтобы состояние билдера не перетекало между вызовами.
	base := func() *gorm.DB {
		q := s.db.WithContext(ctx).Model(&models.TableSnapshot{}).
			Where("table_snapshots.table_id = ?", tableID)
		if from != nil {
			q = q.Where("table_snapshots.taken_at >= ?", *from)
		}
		if to != nil {
			q = q.Where("table_snapshots.taken_at <= ?", *to)
		}
		return q
	}

	var total int64
	if err := base().Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count snapshots for table %d: %w", tableID, err)
	}

	var rows []snapshotListRow
	if err := base().
		Select("table_snapshots.id, table_snapshots.table_id, table_snapshots.taken_at, " +
			"table_snapshots.reason, table_snapshots.actor_user_id, table_snapshots.counts, " +
			"u.first_name AS actor_first_name, u.last_name AS actor_last_name, u.username AS actor_username").
		Joins("LEFT JOIN users u ON u.id = table_snapshots.actor_user_id").
		Order("table_snapshots.taken_at DESC, table_snapshots.id DESC").
		Limit(perPage).Offset((page - 1) * perPage).
		Scan(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list snapshots for table %d: %w", tableID, err)
	}

	// Логин вместо ФИО у акторов, не давших согласия на обработку данных.
	masks := loadConsentMasks(ctx, s.db)
	items := make([]SnapshotListItem, len(rows))
	for i, r := range rows {
		var counts models.SnapshotCounts
		if len(r.Counts) > 0 {
			if err := json.Unmarshal(r.Counts, &counts); err != nil {
				return nil, 0, fmt.Errorf("failed to unmarshal counts of snapshot %d: %w", r.ID, err)
			}
		}
		items[i] = SnapshotListItem{
			ID:          r.ID,
			TableID:     r.TableID,
			TakenAt:     r.TakenAt,
			Reason:      r.Reason,
			ActorUserID: r.ActorUserID,
			ActorName:   maskName(masks, r.ActorUserID, snapshotActorName(r.ActorFirstName, r.ActorLastName, r.ActorUsername)),
			Counts:      counts,
		}
	}
	return items, total, nil
}

func (s *tableSnapshotService) GetSnapshot(ctx context.Context, tableID, snapshotID int) (*models.TableSnapshot, error) {
	var snap models.TableSnapshot
	err := s.db.WithContext(ctx).
		Where("id = ? AND table_id = ?", snapshotID, tableID).
		First(&snap).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, echo.NewHTTPError(http.StatusNotFound, "Snapshot not found")
		}
		return nil, fmt.Errorf("failed to load snapshot %d of table %d: %w", snapshotID, tableID, err)
	}
	return &snap, nil
}

func (s *tableSnapshotService) DeleteSnapshotsOlderThan(ctx context.Context, tableID, months int) (int64, error) {
	if months <= 0 {
		return 0, echo.NewHTTPError(http.StatusBadRequest, "older_than must be a positive number of months")
	}
	cutoff := time.Now().UTC().AddDate(0, -months, 0)
	res := s.db.WithContext(ctx).
		Where("table_id = ? AND taken_at < ?", tableID, cutoff).
		Delete(&models.TableSnapshot{})
	if res.Error != nil {
		return 0, fmt.Errorf("failed to delete snapshots older than %d months for table %d: %w", months, tableID, res.Error)
	}
	return res.RowsAffected, nil
}

// snapshotActorName собирает отображаемое имя автора снимка: "Имя Фамилия", либо
// username, если ФИО пустое, либо "" для дневного (без актора).
func snapshotActorName(first, last, username *string) string {
	deref := func(p *string) string {
		if p == nil {
			return ""
		}
		return *p
	}
	full := strings.TrimSpace(deref(first) + " " + deref(last))
	if full != "" {
		return full
	}
	return deref(username)
}

// computeSnapshotCounts агрегирует строки по территориальному статусу.
// 1=на территории, 2=выехал, 0/nil=не въезжал.
func computeSnapshotCounts(statuses []*int) models.SnapshotCounts {
	counts := models.SnapshotCounts{Total: len(statuses)}
	for _, s := range statuses {
		switch {
		case s != nil && *s == 1:
			counts.OnTerritory++
		case s != nil && *s == 2:
			counts.Exited++
		default:
			counts.NotEntered++
		}
	}
	return counts
}
