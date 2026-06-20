package services

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
)

type carHistoryRow struct {
	ID            int
	CarID         int
	ApplicationID *int
	UserID        *int
	UserName      string
	LastName      *string
	FirstName     *string
	MiddleName    *string
	ActionType    string
	FieldName     *string
	OldValue      *string
	NewValue      *string
	Comment       *string
	CreatedAt     time.Time
	Metadata      *string
	CarNumber     *string
	CarBrand      *string
	Organization  *string
	Company       *string
	TableID       *int
	TableName     *string
}

// GetCarHistory возвращает историю конкретного автомобиля.
func (s *carService) GetCarHistory(ctx context.Context, carID int) ([]CarHistoryItemResponse, error) {
	rows := make([]carHistoryRow, 0)
	err := s.db.WithContext(ctx).Raw(`
		SELECT
			h.id,
			h.car_id,
			h.user_id,
			CONCAT(
				COALESCE(u.last_name, ''),
				CASE WHEN u.first_name IS NOT NULL AND u.first_name != '' THEN ' ' || u.first_name ELSE '' END,
				CASE WHEN u.middle_name IS NOT NULL AND u.middle_name != '' THEN ' ' || u.middle_name ELSE '' END
			) AS user_name,
			u.last_name,
			u.first_name,
			u.middle_name,
			h.action_type,
			h.field_name,
			h.old_value,
			h.new_value,
			h.comment,
			h.created_at,
			h.metadata::text AS metadata,
			h.table_id,
			st.display_name AS table_name,
			app.id AS application_id
		FROM cars_history h
		LEFT JOIN users u ON h.user_id = u.id
		-- car.attachment_id иммутабелен (машина не перепривязывается к другой заявке),
		-- поэтому app.id = заявка-источник машины. LEFT JOIN, чтобы не терять записи истории.
		LEFT JOIN cars c ON h.car_id = c.id
		LEFT JOIN attachments a ON c.attachment_id = a.id
		LEFT JOIN applications app ON a.application_id = app.id
		LEFT JOIN system_tables st ON h.table_id = st.id
		WHERE h.car_id = ?
		ORDER BY h.created_at DESC
	`, carID).Scan(&rows).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching car history")
	}

	return s.mapHistoryRows(rows, false), nil
}

// AddCarHistoryEntry добавляет запись в историю автомобиля.
func (s *carService) AddCarHistoryEntry(ctx context.Context, carID int, req AddCarHistoryRequest) error {
	var metadataStr *string
	if req.Metadata != nil {
		s := string(*req.Metadata)
		metadataStr = &s
	}

	history := models.CarHistory{
		CarID:      carID,
		UserID:     req.UserID,
		ActionType: req.ActionType,
		FieldName:  req.FieldName,
		OldValue:   req.OldValue,
		NewValue:   req.NewValue,
		Comment:    req.Comment,
		Metadata:   metadataStr,
	}
	if err := s.db.WithContext(ctx).Create(&history).Error; err != nil {
		slog.Error("не удалось добавить запись в историю автомобиля", "car_id", carID, "action_type", req.ActionType, "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Error adding car history entry")
	}
	slog.Info("запись в историю автомобиля добавлена", "car_id", carID, "action_type", req.ActionType)
	return nil
}

// GetAllCarsHistory возвращает историю въездов/выездов всех автомобилей.
func (s *carService) GetAllCarsHistory(ctx context.Context) ([]AllCarsHistoryItem, error) {
	type allHistRow struct {
		ID           int
		CarID        int
		UserID       *int
		UserName     string
		ActionType   string
		Comment      *string
		CreatedAt    time.Time
		CarNumber    *string
		CarBrand     *string
		Organization *string
		Company      *string
		TableID      *int
		TableName    *string
	}

	rows := make([]allHistRow, 0)
	err := s.db.WithContext(ctx).Raw(`
		SELECT
			h.id,
			h.car_id,
			h.user_id,
			CONCAT(
				COALESCE(u.last_name, ''),
				CASE WHEN u.first_name IS NOT NULL AND u.first_name != '' THEN ' ' || u.first_name ELSE '' END,
				CASE WHEN u.middle_name IS NOT NULL AND u.middle_name != '' THEN ' ' || u.middle_name ELSE '' END
			) AS user_name,
			h.action_type,
			h.comment,
			h.created_at,
			c.car_number,
			c.car_brand,
			COALESCE(o.name, '') AS organization,
			COALESCE(c2.name, '') AS company,
			h.table_id,
			st.display_name AS table_name
		FROM cars_history h
		LEFT JOIN users u ON h.user_id = u.id
		JOIN cars c ON h.car_id = c.id
		LEFT JOIN attachments a ON c.attachment_id = a.id
		LEFT JOIN applications app ON a.application_id = app.id
		LEFT JOIN organizations o ON app.organization_id = o.id
		LEFT JOIN companies c2 ON app.company_id = c2.id
		LEFT JOIN system_tables st ON h.table_id = st.id
		WHERE h.action_type IN ('entry', 'exit')
		ORDER BY h.created_at DESC
	`).Scan(&rows).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching all cars history")
	}

	items := make([]AllCarsHistoryItem, 0, len(rows))
	for _, r := range rows {
		userName := r.UserName
		if strings.TrimSpace(userName) == "" {
			userName = "Система"
		}
		items = append(items, AllCarsHistoryItem{
			ID:           r.ID,
			CarID:        r.CarID,
			UserID:       r.UserID,
			UserName:     userName,
			ActionType:   r.ActionType,
			Comment:      r.Comment,
			CreatedAt:    FormatUTC(r.CreatedAt),
			CarNumber:    r.CarNumber,
			CarBrand:     r.CarBrand,
			Organization: r.Organization,
			Company:      r.Company,
			TableID:      r.TableID,
			TableName:    r.TableName,
		})
	}
	return items, nil
}

// GetUnifiedCarHistory возвращает объединённую историю для всех автомобилей с одинаковыми параметрами.
func (s *carService) GetUnifiedCarHistory(ctx context.Context, req UnifiedCarHistoryQuery) ([]CarHistoryItemResponse, error) {
	// Находим все машины с одинаковыми параметрами
	type carIDRow struct {
		ID int
	}
	var carIDs []carIDRow
	// Фильтры по organization_id/company_id работают так:
	// - nil: не фильтруем (любая организация/компания) — агрегируем историю по ВСЕМ заявкам
	//   с такой же парой car_number+car_brand. Клиент часто не знает org/comp машины.
	// - не nil: точное совпадение.
	err := s.db.WithContext(ctx).Raw(`
		SELECT c.id
		FROM cars c
		JOIN attachments a ON c.attachment_id = a.id
		JOIN applications app ON a.application_id = app.id
		WHERE LOWER(TRIM(c.car_number)) = LOWER(TRIM(?))
		AND LOWER(TRIM(c.car_brand)) = LOWER(TRIM(?))
		AND (?::integer IS NULL OR app.organization_id = ?)
		AND (?::integer IS NULL OR app.company_id = ?)
		ORDER BY c.id
	`, req.CarNumber, req.CarBrand,
		req.OrganizationID, req.OrganizationID,
		req.CompanyID, req.CompanyID,
	).Scan(&carIDs).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching cars")
	}

	if len(carIDs) == 0 {
		return []CarHistoryItemResponse{}, nil
	}

	ids := make([]int, len(carIDs))
	for i, c := range carIDs {
		ids[i] = c.ID
	}

	rows := make([]carHistoryRow, 0)
	err = s.db.WithContext(ctx).Raw(`
		SELECT
			h.id,
			h.car_id,
			h.user_id,
			CONCAT(
				COALESCE(u.last_name, ''),
				CASE WHEN u.first_name IS NOT NULL AND u.first_name != '' THEN ' ' || u.first_name ELSE '' END,
				CASE WHEN u.middle_name IS NOT NULL AND u.middle_name != '' THEN ' ' || u.middle_name ELSE '' END
			) AS user_name,
			u.last_name,
			u.first_name,
			u.middle_name,
			h.action_type,
			h.field_name,
			h.old_value,
			h.new_value,
			h.comment,
			h.created_at,
			h.metadata::text AS metadata,
			c.car_number,
			c.car_brand,
			COALESCE(o.name, '') AS organization,
			COALESCE(c2.name, '') AS company,
			h.table_id,
			st.display_name AS table_name,
			app.id AS application_id
		FROM cars_history h
		LEFT JOIN users u ON h.user_id = u.id
		JOIN cars c ON h.car_id = c.id
		LEFT JOIN attachments a ON c.attachment_id = a.id
		LEFT JOIN applications app ON a.application_id = app.id
		LEFT JOIN organizations o ON app.organization_id = o.id
		LEFT JOIN companies c2 ON app.company_id = c2.id
		LEFT JOIN system_tables st ON h.table_id = st.id
		WHERE h.car_id IN ?
		ORDER BY h.created_at DESC
	`, ids).Scan(&rows).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching unified car history")
	}

	return s.mapHistoryRows(rows, true), nil
}

// mapHistoryRows преобразует сырые строки истории в DTO.
func (s *carService) mapHistoryRows(rows []carHistoryRow, includeCarInfo bool) []CarHistoryItemResponse {
	items := make([]CarHistoryItemResponse, 0, len(rows))
	for _, r := range rows {
		userName := r.UserName
		if strings.TrimSpace(userName) == "" {
			userName = "Система"
		}

		var metadata *json.RawMessage
		if r.Metadata != nil {
			raw := json.RawMessage(*r.Metadata)
			metadata = &raw
		}

		item := CarHistoryItemResponse{
			ID:            r.ID,
			CarID:         r.CarID,
			ApplicationID: r.ApplicationID,
			UserID:        r.UserID,
			UserName:      userName,
			LastName:      r.LastName,
			FirstName:     r.FirstName,
			MiddleName:    r.MiddleName,
			ActionType:    r.ActionType,
			FieldName:     r.FieldName,
			OldValue:      r.OldValue,
			NewValue:      r.NewValue,
			Comment:       r.Comment,
			CreatedAt:     FormatUTC(r.CreatedAt),
			Metadata:      metadata,
			TableID:       r.TableID,
			TableName:     r.TableName,
		}
		if includeCarInfo {
			item.CarNumber = r.CarNumber
			item.CarBrand = r.CarBrand
			item.Organization = r.Organization
			item.Company = r.Company
		}

		items = append(items, item)
	}
	return items
}
