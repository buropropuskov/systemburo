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
	Reverted      bool
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
			h.reverted,
			app.id AS application_id
		FROM `+carsHistoryUnion+` h
		LEFT JOIN users u ON h.user_id = u.id
		-- car.attachment_id иммутабелен (машина не перепривязывается к другой заявке),
		-- поэтому app.id = заявка-источник машины (NULL у ручных #1049 - метка «добавлено
		-- вручную»). LEFT JOIN, чтобы не терять записи истории.
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
	details := carAuditDetails{
		FieldName: req.FieldName,
		OldValue:  req.OldValue,
		NewValue:  req.NewValue,
		Comment:   req.Comment,
	}
	if req.Metadata != nil {
		details.Metadata = *req.Metadata
	}
	if err := s.recorder.Record(ctx, nil, models.AuditEntityCar, &carID, req.ActionType, req.UserID, details); err != nil {
		slog.Error("не удалось добавить запись в историю автомобиля", "car_id", carID, "action_type", req.ActionType, "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Error adding car history entry")
	}
	slog.Info("запись в историю автомобиля добавлена", "car_id", carID, "action_type", req.ActionType)
	return nil
}

// allCarsHistoryRow - сырая строка выборки истории въездов/выездов.
type allCarsHistoryRow struct {
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
	Reverted     bool
}

// allCarsHistoryUserNameSQL - ФИО отметившего одной строкой. Выражение нужно и
// выборке, и поиску, поэтому вынесено из SELECT.
const allCarsHistoryUserNameSQL = `CONCAT(
			COALESCE(u.last_name, ''),
			CASE WHEN u.first_name IS NOT NULL AND u.first_name != '' THEN ' ' || u.first_name ELSE '' END,
			CASE WHEN u.middle_name IS NOT NULL AND u.middle_name != '' THEN ' ' || u.middle_name ELSE '' END
		)`

// allCarsHistoryFromSQL - источник истории въездов/выездов со всеми соединениями.
// Один и тот же FROM читают страница, счётчик для meta.total и значения выпадающих
// списков: иначе «показано 50 из 431» разошлось бы с самой выборкой на фильтре.
const allCarsHistoryFromSQL = `
	FROM ` + carsHistoryUnion + ` h
	LEFT JOIN users u ON h.user_id = u.id
	JOIN cars c ON h.car_id = c.id
	LEFT JOIN attachments a ON c.attachment_id = a.id
	LEFT JOIN applications app ON a.application_id = app.id
	-- Ручные машины (#1049) висят на вложении-сироте без заявки (app.* NULL),
	-- поэтому org/company берём через COALESCE с самого вложения.
	LEFT JOIN organizations o ON o.id = COALESCE(app.organization_id, a.organization_id)
	LEFT JOIN companies c2 ON c2.id = COALESCE(app.company_id, a.company_id)
	LEFT JOIN system_tables st ON h.table_id = st.id
	WHERE h.action_type IN ('entry', 'exit')
`

// allCarsHistorySelectSQL - общая часть выборки истории въездов/выездов;
// вызывающий дописывает условия, сортировку и страницу.
const allCarsHistorySelectSQL = `
	SELECT
		h.id,
		h.car_id,
		h.user_id,
		` + allCarsHistoryUserNameSQL + ` AS user_name,
		h.action_type,
		h.comment,
		h.created_at,
		c.car_number,
		c.car_brand,
		COALESCE(o.name, '') AS organization,
		COALESCE(c2.name, '') AS company,
		h.table_id,
		st.display_name AS table_name,
		h.reverted` + allCarsHistoryFromSQL

// allCarsHistoryCountSQL - число строк журнала по тем же условиям, что и страница.
const allCarsHistoryCountSQL = `SELECT COUNT(*)` + allCarsHistoryFromSQL

// carsHistoryTableScopeSQL - скоуп таблицы проходной. Запись с проставленным
// table_id принадлежит только своей таблице, иначе проезд через один пост попал бы
// в историю всех постов, где числится машина. По привязке подбираются лишь записи
// без table_id - те, что писались до её появления (сейчас это большая часть журнала).
const carsHistoryTableScopeSQL = `
		AND (
			h.table_id = ?
			OR (
				h.table_id IS NULL
				AND h.car_id IN (SELECT ctt.car_id FROM car_target_tables ctt WHERE ctt.table_id = ?)
			)
		)`

// carsHistorySearchExprs - по чему ищет строка поиска в журнале машин. Подписи
// действий («Прибытие», «Убытие») здесь нет намеренно: это текст интерфейса, а не
// данные, и поле поиска его никогда не обещало.
var carsHistorySearchExprs = []string{
	"c.car_number",
	"c.car_brand",
	"o.name",
	"c2.name",
	allCarsHistoryUserNameSQL,
}

// GetAllCarsHistory возвращает страницу истории въездов/выездов всех автомобилей и
// общее число строк по фильтру.
func (s *carService) GetAllCarsHistory(ctx context.Context, q models.PassageHistoryQuery) ([]AllCarsHistoryItem, int64, error) {
	return s.queryCarsHistory(ctx, q, "", nil)
}

// GetCarsHistoryByTable возвращает страницу истории въездов/выездов таблицы
// проходной и общее число строк по фильтру.
func (s *carService) GetCarsHistoryByTable(ctx context.Context, tableID int, q models.PassageHistoryQuery) ([]AllCarsHistoryItem, int64, error) {
	return s.queryCarsHistory(ctx, q, carsHistoryTableScopeSQL, []any{tableID, tableID})
}

// queryCarsHistory - общая механика журнала машин: скоуп (вся история или одна
// таблица проходной), фильтры, счётчик и страница.
//
// До #2469 оба метода отдавали историю целиком, а искал и фильтровал фронт в
// памяти. Порядок аргументов важен: скоуп, затем фильтры, затем страница - именно в
// этой последовательности `?` встречаются в собранном запросе.
func (s *carService) queryCarsHistory(ctx context.Context, q models.PassageHistoryQuery, scopeSQL string, scopeArgs []any) ([]AllCarsHistoryItem, int64, error) {
	filterSQL, filterArgs, err := passageHistoryConditions(q, passageFilterSpec{
		alias:        "h",
		entityColumn: "h.car_id",
		entityID:     q.CarID,
		searchExprs:  carsHistorySearchExprs,
	})
	if err != nil {
		return nil, 0, err
	}

	where := scopeSQL + filterSQL
	whereArgs := append(append([]any{}, scopeArgs...), filterArgs...)

	var total int64
	if err := s.db.WithContext(ctx).Raw(allCarsHistoryCountSQL+where, whereArgs...).Scan(&total).Error; err != nil {
		return nil, 0, echo.NewHTTPError(http.StatusInternalServerError, "Error counting cars history")
	}

	limitSQL, limitArgs := passageHistoryLimitSQL(q)
	rows := make([]allCarsHistoryRow, 0, q.PerPage)
	err = s.db.WithContext(ctx).Raw(
		allCarsHistorySelectSQL+where+passageHistoryOrderSQL(q, "h")+limitSQL,
		append(append([]any{}, whereArgs...), limitArgs...)...,
	).Scan(&rows).Error
	if err != nil {
		return nil, 0, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching cars history")
	}

	return mapAllCarsHistoryRows(rows), total, nil
}

// GetCarsHistoryFilterOptions отдаёт, кто отмечал проходы: значения выпадающего
// списка «Пользователь». tableID сужает до одной таблицы проходной, nil берёт весь
// журнал.
//
// Список собирается отдельным методом, а не из первой страницы: иначе выбор в фильтре
// зависел бы от того, что попало в 50 загруженных строк. Записи без отметившего
// (действия системы) в список не идут - выбирать «Систему» журнал и раньше не предлагал.
func (s *carService) GetCarsHistoryFilterOptions(ctx context.Context, tableID *int) (CarsHistoryFilterOptions, error) {
	options := CarsHistoryFilterOptions{Users: make([]PassageFilterUser, 0)}

	scopeSQL := ""
	var scopeArgs []any
	if tableID != nil {
		scopeSQL = carsHistoryTableScopeSQL
		scopeArgs = []any{*tableID, *tableID}
	}

	err := s.db.WithContext(ctx).Raw(
		`SELECT DISTINCT h.user_id AS id, `+allCarsHistoryUserNameSQL+` AS name`+
			allCarsHistoryFromSQL+scopeSQL+` AND h.user_id IS NOT NULL ORDER BY name`,
		scopeArgs...,
	).Scan(&options.Users).Error
	if err != nil {
		return options, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching cars history users")
	}

	return options, nil
}

// mapAllCarsHistoryRows преобразует сырые строки истории в DTO.
func mapAllCarsHistoryRows(rows []allCarsHistoryRow) []AllCarsHistoryItem {
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
			Reverted:     r.Reverted,
		})
	}
	return items
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
	// Ручные машины (#1049) без заявки (application_id NULL) - LEFT JOIN applications,
	// org/company через COALESCE с вложения-сироты, иначе INNER JOIN выкинул бы их из
	// объединённой истории тёзок по номеру+марке.
	err := s.db.WithContext(ctx).Raw(`
		SELECT c.id
		FROM cars c
		JOIN attachments a ON c.attachment_id = a.id
		LEFT JOIN applications app ON a.application_id = app.id
		WHERE LOWER(TRIM(c.car_number)) = LOWER(TRIM(?))
		AND LOWER(TRIM(c.car_brand)) = LOWER(TRIM(?))
		AND (?::integer IS NULL OR COALESCE(app.organization_id, a.organization_id) = ?)
		AND (?::integer IS NULL OR COALESCE(app.company_id, a.company_id) = ?)
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
			h.reverted,
			app.id AS application_id
		FROM `+carsHistoryUnion+` h
		LEFT JOIN users u ON h.user_id = u.id
		JOIN cars c ON h.car_id = c.id
		LEFT JOIN attachments a ON c.attachment_id = a.id
		LEFT JOIN applications app ON a.application_id = app.id
		-- Ручные машины (#1049): org/company с вложения-сироты через COALESCE (app.* NULL).
		LEFT JOIN organizations o ON o.id = COALESCE(app.organization_id, a.organization_id)
		LEFT JOIN companies c2 ON c2.id = COALESCE(app.company_id, a.company_id)
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
			Reverted:      r.Reverted,
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
