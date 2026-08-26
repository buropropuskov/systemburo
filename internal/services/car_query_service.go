package services

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

type tableCarRow struct {
	ID                 int
	CarNumber          string
	CarBrand           string
	UnloadPlace        *string
	TerritoryStatus    *int
	TerritoryEntryTime *time.Time
	Organization       *string
	OrganizationID     *int
	Company            *string
	CompanyID          *int
	EntryDateTo        *string
	EntryTimeFrom      *string
	EntryTimeTo        *string
	Status             *int
	ApplicationID      *int
	ApplicationNumber  *string
}

// tableCarsBase — общий каркас выборки машин для таблиц проходной. Когда tableID
// не nil, добавляет scope по «Проезду» (car_target_tables): машина видна только в
// выбранной таблице. tableID == nil — все активные машины (legacy-эндпоинт до
// перевода потребителей на scoped; #1036).
func (s *carService) tableCarsBase(ctx context.Context, tableID *int) *gorm.DB {
	q := s.db.WithContext(ctx).
		Table("cars c").
		Select(`c.id, c.car_number, c.car_brand, c.unload_place,
			c.territory_status, c.territory_entry_time,
			o.name AS organization, o.id AS organization_id,
			c2.name AS company, c2.id AS company_id,
			c.entry_date_to, c.entry_time_from, c.entry_time_to,
			c.status, app.id AS application_id, app.application_number AS application_number`).
		Joins("JOIN attachments a ON c.attachment_id = a.id").
		// LEFT JOIN: ручные машины (#1049) висят на вложении-сироте без заявки
		// (a.application_id IS NULL, a.is_manual). org/company тогда берутся с самого
		// вложения (COALESCE), а app.* остаются NULL - это и есть метка «добавлено вручную».
		Joins("LEFT JOIN applications app ON a.application_id = app.id").
		Joins("LEFT JOIN organizations o ON o.id = COALESCE(app.organization_id, a.organization_id)").
		Joins("LEFT JOIN companies c2 ON c2.id = COALESCE(app.company_id, a.company_id)").
		Where("c.status = ?", 1).
		// Заявочные машины видны только по согласованной активной заявке; ручные
		// минуют это требование - у них заявки нет вовсе, гейт видимости берёт на себя
		// принадлежность целевой таблице (car_target_tables) + security-видимость (S6).
		Where("a.is_manual OR (app.confirmation = ? AND app.status IN ?)",
			models.ConfirmationApproved, []string{models.StatusInWork, models.StatusCompleted})
	if tableID != nil {
		q = q.Joins("JOIN car_target_tables ctt ON ctt.car_id = c.id").
			Where("ctt.table_id = ?", *tableID)
	}
	return q
}

// activeCars возвращает активные машины (без «по факту») с опциональным scope таблицы.
func (s *carService) activeCars(ctx context.Context, tableID *int) ([]TableCarResponse, error) {
	rows := make([]tableCarRow, 0)
	err := s.tableCarsBase(ctx, tableID).
		Where("LOWER(TRIM(c.car_number)) != ?", "по факту").
		Order("c.car_number").
		Scan(&rows).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching active cars")
	}
	return s.enrichTableCars(ctx, rows)
}

// factCars возвращает машины «по факту» с опциональным scope таблицы. Fallback на
// ILIKE, если точного совпадения «по факту» нет (совместимость со старыми данными).
func (s *carService) factCars(ctx context.Context, tableID *int) ([]TableCarResponse, error) {
	rows := make([]tableCarRow, 0)

	err := s.tableCarsBase(ctx, tableID).
		Where("LOWER(TRIM(c.car_number)) = ?", "по факту").
		Order("organization, c.entry_date_to").
		Scan(&rows).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching fact cars")
	}

	if len(rows) == 0 {
		err = s.tableCarsBase(ctx, tableID).
			Where("c.car_number ILIKE ? OR c.car_number ILIKE ? OR c.car_number ILIKE ?",
				"%по факту%", "%пофакту%", "%факт%").
			Order("organization, c.entry_date_to").
			Scan(&rows).Error
		if err != nil {
			return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching alternative fact cars")
		}
	}

	return s.enrichTableCars(ctx, rows)
}


// GetActiveCarsForTable возвращает активные машины конкретной таблицы «Проезд» (#1036).
func (s *carService) GetActiveCarsForTable(ctx context.Context, tableID int) ([]TableCarResponse, error) {
	return s.activeCars(ctx, &tableID)
}


// GetFactCarsForTable возвращает машины «по факту» конкретной таблицы «Проезд» (#1036).
func (s *carService) GetFactCarsForTable(ctx context.Context, tableID int) ([]TableCarResponse, error) {
	return s.factCars(ctx, &tableID)
}

// enrichTableCars добавляет unload_places к каждому автомобилю.
func (s *carService) enrichTableCars(ctx context.Context, rows []tableCarRow) ([]TableCarResponse, error) {
	if len(rows) == 0 {
		return []TableCarResponse{}, nil
	}

	carIDs := make([]int, len(rows))
	for i, row := range rows {
		carIDs[i] = row.ID
	}

	type placeRow struct {
		CarID int
		Name  string
	}
	var allPlaces []placeRow
	err := s.db.WithContext(ctx).
		Table("car_unload_places cup").
		Select("cup.car_id, up.name").
		Joins("JOIN unload_places up ON cup.unload_place_id = up.id").
		Where("cup.car_id IN ?", carIDs).
		Order("cup.car_id, cup.order_index").
		Scan(&allPlaces).Error
	if err != nil {
		slog.Error("не удалось загрузить места разгрузки", "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching unload places")
	}

	placesByCarID := make(map[int][]string)
	for _, p := range allPlaces {
		placesByCarID[p.CarID] = append(placesByCarID[p.CarID], p.Name)
	}

	targetTablesCount, err := s.targetTablesCountByCarID(ctx, carIDs)
	if err != nil {
		return nil, err
	}

	targetTables, err := s.targetTablesByCarID(ctx, carIDs)
	if err != nil {
		return nil, err
	}

	cars := make([]TableCarResponse, 0, len(rows))
	for _, row := range rows {
		status := 0
		if row.Status != nil {
			status = *row.Status
		}

		territoryEntryTimeStr := FormatUTCPtr(row.TerritoryEntryTime)

		places := placesByCarID[row.ID]
		if places == nil {
			places = []string{}
		}

		tables := targetTables[row.ID]
		if tables == nil {
			tables = []CarPassageTableRef{}
		}

		cars = append(cars, TableCarResponse{
			ID:                 row.ID,
			CarNumber:          row.CarNumber,
			CarBrand:           row.CarBrand,
			Organization:       row.Organization,
			OrganizationID:     row.OrganizationID,
			Company:            row.Company,
			CompanyID:          row.CompanyID,
			UnloadPlace:        row.UnloadPlace,
			UnloadPlaces:       places,
			EntryDateTo:        row.EntryDateTo,
			EntryTimeFrom:      row.EntryTimeFrom,
			EntryTimeTo:        row.EntryTimeTo,
			Status:             status,
			ApplicationID:      row.ApplicationID,
			ApplicationNumber:  row.ApplicationNumber,
			TerritoryStatus:    row.TerritoryStatus,
			TerritoryEntryTime: territoryEntryTimeStr,
			TargetTablesCount:  targetTablesCount[row.ID],
			TargetTables:       tables,
		})
	}
	return cars, nil
}

// targetTablesCountByCarID считает число привязок car_target_tables на каждую
// машину из carIDs (#1194) - FE решает по нему, показывать ли per-row подменю
// «Убрать из этой/из всех» (>1) или сразу деактивировать (единственная).
func (s *carService) targetTablesCountByCarID(ctx context.Context, carIDs []int) (map[int]int, error) {
	counts := make(map[int]int, len(carIDs))
	if len(carIDs) == 0 {
		return counts, nil
	}
	type countRow struct {
		CarID int
		Cnt   int
	}
	var rows []countRow
	if err := s.db.WithContext(ctx).
		Table("car_target_tables").
		Select("car_id, COUNT(*) AS cnt").
		Where("car_id IN ?", carIDs).
		Group("car_id").
		Scan(&rows).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error counting target tables")
	}
	for _, r := range rows {
		counts[r.CarID] = r.Cnt
	}
	return counts, nil
}

// targetTablesByCarID возвращает список привязок «Проезд» {id,name,source} на
// каждую машину из carIDs (#1227) - карточка машины из контекста проходной
// показывает бейдж источника вместо голого счётчика.
func (s *carService) targetTablesByCarID(ctx context.Context, carIDs []int) (map[int][]CarPassageTableRef, error) {
	result := make(map[int][]CarPassageTableRef, len(carIDs))
	if len(carIDs) == 0 {
		return result, nil
	}
	type tableRow struct {
		CarID  int
		ID     int
		Name   string
		Source string
	}
	var rows []tableRow
	if err := s.db.WithContext(ctx).
		Table("car_target_tables ctt").
		Select("ctt.car_id AS car_id, st.id AS id, COALESCE(NULLIF(st.display_name, ''), st.name) AS name, ctt.source AS source").
		Joins("JOIN system_tables st ON st.id = ctt.table_id").
		Where("ctt.car_id IN ?", carIDs).
		Order("ctt.car_id, ctt.order_index, ctt.table_id").
		Scan(&rows).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching target tables")
	}
	for _, r := range rows {
		result[r.CarID] = append(result[r.CarID], CarPassageTableRef{ID: r.ID, Name: r.Name, Source: r.Source})
	}
	return result, nil
}

// GetCarUnloadPlaces возвращает связи активных автомобилей с местами разгрузки.
func (s *carService) GetCarUnloadPlaces(ctx context.Context) ([]CarUnloadPlaceInfo, error) {
	places := make([]CarUnloadPlaceInfo, 0)
	err := s.db.WithContext(ctx).
		Table("car_unload_places cup").
		Select("cup.car_id, cup.unload_place_id, up.name AS unload_place_name").
		Joins("JOIN unload_places up ON cup.unload_place_id = up.id").
		Joins("JOIN cars c ON cup.car_id = c.id").
		Joins("JOIN attachments a ON c.attachment_id = a.id").
		// LEFT JOIN и ветка is_manual - зеркало предиката самой таблицы
		// (tableCarsBase): ручная машина висит на вложении без заявки, и внутренний
		// джойн выкидывал её связи. Строка таблицы место при этом показывала (там
		// имена грузятся отдельно, без заявки), а карточка строит секцию по
		// unload_place_ids из этого запроса - и оставалась пустой (#1238).
		Joins("LEFT JOIN applications app ON a.application_id = app.id").
		Where("c.status = ?", 1).
		Where("a.is_manual OR (app.confirmation = ? AND app.status IN ?)",
			models.ConfirmationApproved, []string{models.StatusInWork, models.StatusCompleted}).
		Order("cup.car_id, cup.order_index").
		Scan(&places).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching car unload places")
	}
	return places, nil
}

// GetFactCarUnloadPlaces возвращает связи «по факту» автомобилей с местами разгрузки.
func (s *carService) GetFactCarUnloadPlaces(ctx context.Context) ([]CarUnloadPlaceInfo, error) {
	places := make([]CarUnloadPlaceInfo, 0)
	err := s.db.WithContext(ctx).
		Table("car_unload_places cup").
		Select("cup.car_id, cup.unload_place_id, up.name AS unload_place_name").
		Joins("JOIN unload_places up ON cup.unload_place_id = up.id").
		Joins("JOIN cars c ON cup.car_id = c.id").
		Joins("JOIN attachments a ON c.attachment_id = a.id").
		// Тот же зеркальный предикат, что у активных: ручная машина заявки не имеет.
		Joins("LEFT JOIN applications app ON a.application_id = app.id").
		Where("c.status = ?", 1).
		Where("a.is_manual OR (app.confirmation = ? AND app.status IN ?)",
			models.ConfirmationApproved, []string{models.StatusInWork, models.StatusCompleted}).
		Where("LOWER(TRIM(c.car_number)) = ?", "по факту").
		Order("cup.car_id, cup.order_index").
		Scan(&places).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching fact car unload places")
	}
	return places, nil
}
