package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// snapshotCarLister, snapshotEmployeeLister, snapshotEmployeeStatuser — минимальные
// контракты, которые снимок берёт у существующих сервисов. Снимок обязан отдавать
// РОВНО тот набор строк, что показывает страница таблицы, поэтому переиспользует их
// листинги, а не пишет свой SQL.
type snapshotCarLister interface {
	GetActiveCarsForTables(ctx context.Context) ([]TableCarResponse, error)
	GetFactCarsForTables(ctx context.Context) ([]TableCarResponse, error)
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

	payloadJSON, err := json.Marshal(models.SnapshotPayload{TableType: table.TableType, Rows: rowsJSON})
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
		// Листинг машин глобален (не скоуплен по table_id) - как и страница cars-таблицы;
		// territory_status уже в TableCarResponse. Если таблица показывает блок «по факту»
		// (show_fact_table), подмешиваем его тем же листингом, что и страница, помечая
		// строки is_fact - иначе слепок терял бы машины «по факту», стоявшие на территории.
		cars, err := s.cars.GetActiveCarsForTables(ctx)
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
			facts, err := s.cars.GetFactCarsForTables(ctx)
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
