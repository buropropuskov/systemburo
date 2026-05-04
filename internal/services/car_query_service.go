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
}

// GetActiveCarsForTables возвращает активные автомобили для всех таблиц (без «по факту»).
func (s *carService) GetActiveCarsForTables(ctx context.Context) ([]TableCarResponse, error) {
	rows := make([]tableCarRow, 0)
	err := s.db.WithContext(ctx).
		Table("cars c").
		Select(`c.id, c.car_number, c.car_brand, c.unload_place,
			c.territory_status, c.territory_entry_time,
			o.name AS organization, o.id AS organization_id,
			c2.name AS company, c2.id AS company_id,
			c.entry_date_to, c.entry_time_from, c.entry_time_to,
			c.status, app.id AS application_id`).
		Joins("JOIN attachments a ON c.attachment_id = a.id").
		Joins("JOIN applications app ON a.application_id = app.id").
		Joins("LEFT JOIN organizations o ON app.organization_id = o.id").
		Joins("LEFT JOIN companies c2 ON app.company_id = c2.id").
		Where("c.status = ?", 1).
		Where("app.confirmation = ?", models.ConfirmationApproved).
		Where("app.status IN ?", []string{models.StatusInWork, models.StatusCompleted}).
		Where("LOWER(TRIM(c.car_number)) != ?", "по факту").
		Order("c.car_number").
		Scan(&rows).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching active cars")
	}

	return s.enrichTableCars(ctx, rows)
}

// GetFactCarsForTables возвращает автомобили с номером «по факту».
func (s *carService) GetFactCarsForTables(ctx context.Context) ([]TableCarResponse, error) {
	rows := make([]tableCarRow, 0)

	baseQuery := func() *gorm.DB {
		return s.db.WithContext(ctx).
			Table("cars c").
			Select(`c.id, c.car_number, c.car_brand, c.unload_place,
				c.territory_status, c.territory_entry_time,
				o.name AS organization, o.id AS organization_id,
				c2.name AS company, c2.id AS company_id,
				c.entry_date_to, c.entry_time_from, c.entry_time_to,
				c.status, app.id AS application_id`).
			Joins("JOIN attachments a ON c.attachment_id = a.id").
			Joins("JOIN applications app ON a.application_id = app.id").
			Joins("LEFT JOIN organizations o ON app.organization_id = o.id").
			Joins("LEFT JOIN companies c2 ON app.company_id = c2.id").
			Where("c.status = ?", 1).
			Where("app.confirmation = ?", models.ConfirmationApproved).
			Where("app.status IN ?", []string{models.StatusInWork, models.StatusCompleted}).
			Order("organization, c.entry_date_to")
	}

	err := baseQuery().
		Where("LOWER(TRIM(c.car_number)) = ?", "по факту").
		Scan(&rows).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching fact cars")
	}

	if len(rows) == 0 {
		err = baseQuery().
			Where("c.car_number ILIKE ? OR c.car_number ILIKE ? OR c.car_number ILIKE ?",
				"%по факту%", "%пофакту%", "%факт%").
			Scan(&rows).Error
		if err != nil {
			return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching alternative fact cars")
		}
	}

	return s.enrichTableCars(ctx, rows)
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
			TerritoryStatus:    row.TerritoryStatus,
			TerritoryEntryTime: territoryEntryTimeStr,
		})
	}
	return cars, nil
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
		Joins("JOIN applications app ON a.application_id = app.id").
		Where("c.status = ?", 1).
		Where("app.confirmation = ?", models.ConfirmationApproved).
		Where("app.status IN ?", []string{models.StatusInWork, models.StatusCompleted}).
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
		Joins("JOIN applications app ON a.application_id = app.id").
		Where("c.status = ?", 1).
		Where("app.confirmation = ?", models.ConfirmationApproved).
		Where("app.status IN ?", []string{models.StatusInWork, models.StatusCompleted}).
		Where("LOWER(TRIM(c.car_number)) = ?", "по факту").
		Order("cup.car_id, cup.order_index").
		Scan(&places).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching fact car unload places")
	}
	return places, nil
}
