package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

func (s *tableSnapshotService) SnapshotTable(ctx context.Context, tableID int, reason string, actorUserID *int) (int, error) {
	if reason != models.SnapshotReasonScheduled && reason != models.SnapshotReasonManual {
		return 0, echo.NewHTTPError(http.StatusBadRequest, "Invalid snapshot reason")
	}

	var table models.SystemTable
	if err := s.db.WithContext(ctx).Select("id", "table_type").First(&table, tableID).Error; err != nil {
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

// collectRows строит слепок строк таблицы и агрегаты по её типу. Возвращает сырой JSON
// строк (для payload) и подсчитанные counts.
func (s *tableSnapshotService) collectRows(ctx context.Context, table models.SystemTable) (json.RawMessage, models.SnapshotCounts, error) {
	switch table.TableType {
	case models.TableTypeCars:
		// Листинг машин глобален (не скоуплен по table_id) - как и страница cars-таблицы;
		// territory_status уже в TableCarResponse.
		cars, err := s.cars.GetActiveCarsForTables(ctx)
		if err != nil {
			return nil, models.SnapshotCounts{}, fmt.Errorf("failed to list cars for snapshot: %w", err)
		}
		statuses := make([]*int, len(cars))
		for i, c := range cars {
			statuses[i] = c.TerritoryStatus
		}
		raw, err := json.Marshal(cars)
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
