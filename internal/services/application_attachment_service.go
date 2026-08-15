package services

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"systemburo/internal/crypto"
	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
)

// GetApplicationViewers возвращает просматривающих заявки с информацией о пользователях.
func (s *applicationService) GetApplicationViewers(ctx context.Context, applicationID int) ([]ViewerWithUser, error) {
	viewers := make([]ViewerWithUser, 0)
	err := s.db.WithContext(ctx).Raw(`
		SELECT
			av.id,
			av.user_id,
			u.username,
			u.last_name,
			u.first_name,
			u.middle_name,
			u.position,
			av.created_at
		FROM application_viewers av
		JOIN users u ON av.user_id = u.id
		WHERE av.application_id = ?
		ORDER BY u.last_name, u.first_name
	`, applicationID).Scan(&viewers).Error

	if err != nil {
		slog.Error("Ошибка получения просматривающих", "application_id", applicationID, "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching viewers")
	}

	if masks := loadConsentMasks(ctx, s.db); len(masks) > 0 {
		for i := range viewers {
			maskUserParts(masks, viewers[i].UserID, &viewers[i].LastName, &viewers[i].FirstName, &viewers[i].MiddleName)
		}
	}
	return viewers, nil
}

// resolveForwardFilter определяет, ограничен ли просматривающий пер-вложенным пересылом
// (#680), и какие вложения ему доступны. restricted=false означает «видит все вложения
// заявки»: супер-админ (viewerUserID==0), отправитель заявки или пользователь без строк
// forward_attachments для этой заявки (обратная совместимость). restricted=true -> allowed
// содержит только перечисленные при пересылке вложения.
func (s *applicationService) resolveForwardFilter(ctx context.Context, applicationID, viewerUserID int) (allowed map[int]bool, restricted bool, err error) {
	if viewerUserID == 0 {
		return nil, false, nil
	}

	var ids []int
	if e := s.db.WithContext(ctx).Raw(
		"SELECT attachment_id FROM forward_attachments WHERE application_id = ? AND recipient_user_id = ?",
		applicationID, viewerUserID,
	).Scan(&ids).Error; e != nil {
		slog.Error("Ошибка чтения forward_attachments", "application_id", applicationID, "user_id", viewerUserID, "error", e)
		return nil, false, echo.NewHTTPError(http.StatusInternalServerError, "Database error")
	}
	if len(ids) == 0 {
		return nil, false, nil
	}

	// Отправитель видит все вложения независимо от строк (напр. пересылка ему же).
	var isSender bool
	if e := s.db.WithContext(ctx).Raw(
		"SELECT EXISTS(SELECT 1 FROM applications WHERE id = ? AND sender_user_id = ?)",
		applicationID, viewerUserID,
	).Scan(&isSender).Error; e != nil {
		slog.Error("Ошибка проверки отправителя", "application_id", applicationID, "user_id", viewerUserID, "error", e)
		return nil, false, echo.NewHTTPError(http.StatusInternalServerError, "Database error")
	}
	if isSender {
		return nil, false, nil
	}

	allowed = make(map[int]bool, len(ids))
	for _, id := range ids {
		allowed[id] = true
	}
	return allowed, true, nil
}

// CanViewAttachment сообщает, доступно ли конкретное вложение просматривающему с учётом
// пер-вложенного пересыла (#680). viewerUserID==0 (супер-админ) и отправитель видят любое
// вложение заявки; получатель с ограничением - только перечисленные.
func (s *applicationService) CanViewAttachment(ctx context.Context, applicationID, attachmentID, viewerUserID int) (bool, error) {
	allowed, restricted, err := s.resolveForwardFilter(ctx, applicationID, viewerUserID)
	if err != nil {
		return false, err
	}
	if !restricted {
		return true, nil
	}
	return allowed[attachmentID], nil
}

// GetApplicationAttachments возвращает вложения заявки с информацией о шаблонах.
// viewerUserID ограничивает выдачу вложениями, доступными получателю пересылки (#680);
// 0 - супер-админ, фильтр не применяется.
func (s *applicationService) GetApplicationAttachments(ctx context.Context, applicationID, viewerUserID int) ([]AttachmentInfo, error) {
	attachments := make([]AttachmentInfo, 0)
	err := s.db.WithContext(ctx).Raw(`
		SELECT
			a.id,
			a.attachment_type,
			a.attachment_name,
			COALESCE(a.attachment_display_name, '') as attachment_display_name,
			a.entry_date_from,
			a.entry_date_to,
			a.entry_time_from,
			a.entry_time_to,
			a.roof_access,
			a.free_parking,
			a.created_at,
			a.unique_attachment_id,
			ua.title as unique_attachment_title,
			ua.display_name as unique_attachment_display_name,
			EXISTS (SELECT 1 FROM attachment_templates at2 WHERE at2.unique_attachment_id = a.unique_attachment_id) as has_template,
			COALESCE(be.status, '') as archive_status
		FROM attachments a
		LEFT JOIN unique_attachments ua ON a.unique_attachment_id = ua.id
		LEFT JOIN blank_exports be ON be.application_id = a.application_id AND be.attachment_id = a.id
		WHERE a.application_id = ?
		ORDER BY ua.title, a.created_at
	`, applicationID).Scan(&attachments).Error

	if err != nil {
		slog.Error("Ошибка получения вложений", "application_id", applicationID, "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching attachments")
	}

	allowed, restricted, err := s.resolveForwardFilter(ctx, applicationID, viewerUserID)
	if err != nil {
		return nil, err
	}
	if restricted {
		visible := make([]AttachmentInfo, 0, len(allowed))
		for _, a := range attachments {
			if allowed[a.ID] {
				visible = append(visible, a)
			}
		}
		attachments = visible
	}

	if len(attachments) > 0 {
		attIDs := make([]int, len(attachments))
		for i, a := range attachments {
			attIDs[i] = a.ID
		}
		type cvRow struct {
			AttachmentID int    `gorm:"column:attachment_id"`
			FieldID      int    `gorm:"column:field_id"`
			Label        string `gorm:"column:label"`
			Value        string `gorm:"column:value"`
		}
		var cvRows []cvRow
		s.db.WithContext(ctx).Raw(`
			SELECT acv.attachment_id, acf.id as field_id, acf.label, acv.value
			FROM attachment_custom_values acv
			JOIN attachment_custom_fields acf ON acv.custom_field_id = acf.id
			WHERE acv.attachment_id IN ?
			ORDER BY acf.sort_order, acf.id
		`, attIDs).Scan(&cvRows)

		cvMap := map[int][]CustomValueDetail{}
		for _, r := range cvRows {
			cvMap[r.AttachmentID] = append(cvMap[r.AttachmentID], CustomValueDetail{
				FieldID: r.FieldID,
				Label:   r.Label,
				Value:   r.Value,
			})
		}
		for i := range attachments {
			attachments[i].CustomValues = cvMap[attachments[i].ID]
		}
	}

	return attachments, nil
}

// fetchBlacklistFlags возвращает per-element предупреждения о возможном обходе ЧС (#481)
// по типу элемента и его id в рамках заявки; ключ результата - element_id. Best-effort
// overlay для детали: ошибку чтения логируем и возвращаем пустую карту - деталь должна
// отрисоваться и без предупреждений, а не падать.
// Фильтр по application_id - защита-в-глубину: cars/employees сейчас создаются свежими на
// каждый сабмит (element_id уникален на заявку), но завязываться на этот инвариант в
// security-оверлее не хочется - иначе будущий рефактор с переиспользованием строк тихо
// показал бы чужой override.
func (s *applicationService) fetchBlacklistFlags(ctx context.Context, elementType string, applicationID int, ids []int) map[int]*BlacklistFlagInfo {
	out := make(map[int]*BlacklistFlagInfo)
	if len(ids) == 0 {
		return out
	}
	var rows []struct {
		FlagID        int     `gorm:"column:flag_id"`
		ElementID     int     `gorm:"column:element_id"`
		MatchedValue  string  `gorm:"column:matched_value"`
		MatchedReason string  `gorm:"column:matched_reason"`
		Similarity    float64 `gorm:"column:similarity"`
		Overridden    bool    `gorm:"column:overridden"`
	}
	// LEFT JOIN на override: overridden=true, если по флагу подтверждён пропуск (срез 4).
	if err := s.db.WithContext(ctx).Raw(`
		SELECT f.id AS flag_id, f.element_id, f.matched_value, f.matched_reason, f.similarity,
		       (o.id IS NOT NULL) AS overridden
		FROM application_blacklist_flags f
		LEFT JOIN application_blacklist_overrides o ON o.flag_id = f.id
		WHERE f.application_id = ? AND f.element_type = ? AND f.element_id IN ?
		  AND (
		        (f.element_type = 'car' AND EXISTS (
		           SELECT 1 FROM vehicle_blacklists vb WHERE vb.id = f.matched_blacklist_id AND vb.is_active))
		     OR (f.element_type = 'employee' AND EXISTS (
		           SELECT 1 FROM person_blacklists pb WHERE pb.id = f.matched_blacklist_id AND pb.is_active))
		      )
		ORDER BY f.id`, applicationID, elementType, ids).Scan(&rows).Error; err != nil {
		slog.Warn("fetch blacklist flags failed", "err", err, "element_type", elementType)
		return out
	}
	for _, r := range rows {
		out[r.ElementID] = &BlacklistFlagInfo{
			FlagID:        r.FlagID,
			MatchedValue:  r.MatchedValue,
			MatchedReason: r.MatchedReason,
			Similarity:    r.Similarity,
			Overridden:    r.Overridden,
		}
	}
	return out
}

// GetAttachmentCars возвращает автомобили вложения с привязанными местами разгрузки.
// scope решает, попадают ли в выдачу машины ещё не принятого дополнения (#1685).
// Связи строк вложения с местами и таблицами берутся одним запросом на всю выборку
// (#1050). Раньше на каждую машину уходило два подзапроса, на каждого сотрудника один:
// заявка на двадцать машин давала сорок обращений к базе вместо двух, и деталь заявки
// линейно тяжелела с числом строк.

// carUnloadPlacesByCar возвращает места разгрузки, разложенные по машинам.
func (s *applicationService) carUnloadPlacesByCar(ctx context.Context, carIDs []int) map[int][]UnloadPlaceRef {
	out := make(map[int][]UnloadPlaceRef, len(carIDs))
	if len(carIDs) == 0 {
		return out
	}
	var rows []struct {
		CarID       int    `gorm:"column:car_id"`
		ID          int    `gorm:"column:id"`
		Name        string `gorm:"column:name"`
		Description *string
	}
	// Порядок внутри машины сохраняем тем же order_index, что и раньше: он задаёт
	// последовательность мест в карточке.
	s.db.WithContext(ctx).Raw(`
		SELECT cup.car_id, up.id, up.name, up.description
		FROM car_unload_places cup
		JOIN unload_places up ON cup.unload_place_id = up.id
		WHERE cup.car_id IN ?
		ORDER BY cup.car_id, cup.order_index
	`, carIDs).Scan(&rows)
	for _, r := range rows {
		out[r.CarID] = append(out[r.CarID], UnloadPlaceRef{ID: r.ID, Name: r.Name, Description: r.Description})
	}
	return out
}

// carTargetTablesByCar возвращает таблицы «Проезда», разложенные по машинам.
func (s *applicationService) carTargetTablesByCar(ctx context.Context, carIDs []int) map[int][]TableInfoRef {
	out := make(map[int][]TableInfoRef, len(carIDs))
	if len(carIDs) == 0 {
		return out
	}
	var rows []struct {
		CarID       int    `gorm:"column:car_id"`
		ID          int    `gorm:"column:id"`
		Name        string `gorm:"column:name"`
		DisplayName string `gorm:"column:display_name"`
	}
	s.db.WithContext(ctx).Raw(`
		SELECT ctt.car_id, st.id, st.name, st.display_name
		FROM car_target_tables ctt
		JOIN system_tables st ON ctt.table_id = st.id
		WHERE ctt.car_id IN ?
		ORDER BY ctt.car_id, ctt.order_index
	`, carIDs).Scan(&rows)
	for _, r := range rows {
		out[r.CarID] = append(out[r.CarID], TableInfoRef{ID: r.ID, Name: r.Name, DisplayName: r.DisplayName})
	}
	return out
}

// employeeTargetTablesByEmployee возвращает места прохода, разложенные по сотрудникам.
func (s *applicationService) employeeTargetTablesByEmployee(ctx context.Context, empIDs []int) map[int][]TableInfoRef {
	out := make(map[int][]TableInfoRef, len(empIDs))
	if len(empIDs) == 0 {
		return out
	}
	var rows []struct {
		EmployeeID  int    `gorm:"column:employee_id"`
		ID          int    `gorm:"column:id"`
		Name        string `gorm:"column:name"`
		DisplayName string `gorm:"column:display_name"`
	}
	s.db.WithContext(ctx).Raw(`
		SELECT ett.employee_id, st.id, st.name, st.display_name
		FROM employee_target_tables ett
		JOIN system_tables st ON ett.table_id = st.id
		WHERE ett.employee_id IN ?
		ORDER BY ett.employee_id, ett.order_index
	`, empIDs).Scan(&rows)
	for _, r := range rows {
		out[r.EmployeeID] = append(out[r.EmployeeID], TableInfoRef{ID: r.ID, Name: r.Name, DisplayName: r.DisplayName})
	}
	return out
}

func (s *applicationService) GetAttachmentCars(ctx context.Context, attachmentID int, scope SupplementScope) ([]CarWithPlaces, error) {
	type carRow struct {
		ID             int
		CarNumber      string  `gorm:"column:car_number"`
		CarBrand       string  `gorm:"column:car_brand"`
		UnloadPlace    *string `gorm:"column:unload_place"`
		EntryDateFrom  *string `gorm:"column:entry_date_from"`
		EntryTimeFrom  *string `gorm:"column:entry_time_from"`
		EntryDateTo    *string `gorm:"column:entry_date_to"`
		EntryTimeTo    *string `gorm:"column:entry_time_to"`
		Organization   *string `gorm:"column:organization"`
		OrganizationID *int    `gorm:"column:organization_id"`
		Company        *string `gorm:"column:company"`
		CompanyID      *int    `gorm:"column:company_id"`
		// Раунд дополнения, принёсший строку (#1685). Номер и статус подтягиваются тем же
		// запросом: отдельный поход за ними на каждую машину дал бы N+1.
		SupplementID     *int    `gorm:"column:supplement_id"`
		SupplementNumber *int    `gorm:"column:supplement_number"`
		SupplementStatus *string `gorm:"column:supplement_status"`
		// Точное попадание в действующий чёрный список - см. empRow.IsBlacklisted.
		IsBlacklisted bool `gorm:"column:is_blacklisted"`
	}
	cars := make([]carRow, 0)
	if err := s.db.WithContext(ctx).Raw(`
		SELECT
			c.id,
			c.car_number,
			c.car_brand,
			c.unload_place,
			c.entry_date_from,
			c.entry_time_from,
			c.entry_date_to,
			c.entry_time_to,
			o.name AS organization,
			o.id   AS organization_id,
			comp.name AS company,
			comp.id   AS company_id,
			c.supplement_id,
			sup.number AS supplement_number,
			sup.status AS supplement_status,
			EXISTS(
				SELECT 1 FROM vehicle_blacklists vbl
				WHERE vbl.is_active
				  AND LOWER(TRIM(vbl.car_number)) = LOWER(TRIM(c.car_number))
				  AND (
				        c.mark_id = vbl.mark_id
				        OR (c.mark_id IS NULL
				            AND LOWER(TRIM(COALESCE(c.mark_name, c.car_brand))) = LOWER(TRIM(vbl.mark_name)))
				      )
			) AS is_blacklisted
		FROM cars c
		JOIN attachments a ON c.attachment_id = a.id
		JOIN applications app ON a.application_id = app.id
		LEFT JOIN organizations o ON app.organization_id = o.id
		LEFT JOIN companies comp ON app.company_id = comp.id
		LEFT JOIN application_supplements sup ON sup.id = c.supplement_id
		WHERE c.attachment_id = ?`+supplementScopeWhere(scope, "c"),
		attachmentID).Scan(&cars).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching cars")
	}

	carIDs := make([]int, 0, len(cars))
	for _, car := range cars {
		carIDs = append(carIDs, car.ID)
	}
	appID, _ := s.GetApplicationIDByAttachment(ctx, attachmentID)
	flags := s.fetchBlacklistFlags(ctx, models.BlacklistElementCar, appID, carIDs)

	placesByCar := s.carUnloadPlacesByCar(ctx, carIDs)
	tablesByCar := s.carTargetTablesByCar(ctx, carIDs)

	result := make([]CarWithPlaces, 0)
	for _, car := range cars {
		// Пустой срез, а не nil: фронт различает «привязок нет» и «поле не пришло».
		places := placesByCar[car.ID]
		if places == nil {
			places = make([]UnloadPlaceRef, 0)
		}
		tables := tablesByCar[car.ID]
		if tables == nil {
			tables = make([]TableInfoRef, 0)
		}

		result = append(result, CarWithPlaces{
			IsBlacklisted:    car.IsBlacklisted,
			ID:               car.ID,
			CarNumber:        car.CarNumber,
			CarBrand:         car.CarBrand,
			UnloadPlace:      car.UnloadPlace,
			EntryDateFrom:    car.EntryDateFrom,
			EntryTimeFrom:    car.EntryTimeFrom,
			EntryDateTo:      car.EntryDateTo,
			EntryTimeTo:      car.EntryTimeTo,
			Organization:     car.Organization,
			OrganizationID:   car.OrganizationID,
			Company:          car.Company,
			CompanyID:        car.CompanyID,
			UnloadPlaces:     places,
			TargetTables:     tables,
			BlacklistSimilar: flags[car.ID],
			SupplementMark:   supplementMark(car.SupplementID, car.SupplementNumber, car.SupplementStatus),
		})
	}

	return result, nil
}

// GetAttachmentEmployees возвращает сотрудников вложения с целевыми таблицами. scope
// решает, попадают ли в выдачу сотрудники ещё не принятого дополнения (#1685); выборка
// несёт серию и номер паспорта и номер патента, поэтому охране идёт только допущенное.
func (s *applicationService) GetAttachmentEmployees(ctx context.Context, attachmentID int, scope SupplementScope) ([]EmployeeWithTables, error) {
	type empRow struct {
		ID                   int
		LastName             string  `gorm:"column:last_name"`
		FirstName            string  `gorm:"column:first_name"`
		MiddleName           *string `gorm:"column:middle_name"`
		Position             *string `gorm:"column:position"`
		CitizenshipID        *int    `gorm:"column:citizenship_id"`
		CitizenshipName      *string `gorm:"column:citizenship_name"`
		PassportSeriesNumber *string `gorm:"column:passport_series_number"`
		PatentNumber         *string `gorm:"column:patent_number"`
		OtherPermission      *string `gorm:"column:other_permission"`
		EntryDateTo          *string `gorm:"column:entry_date_to"`
		PassTime             *string `gorm:"column:pass_time"`
		Organization         *string `gorm:"column:organization"`
		OrganizationID       *int    `gorm:"column:organization_id"`
		Company              *string `gorm:"column:company"`
		CompanyID            *int    `gorm:"column:company_id"`
		// Раунд дополнения, принёсший строку (#1685), тем же запросом - см. carRow.
		SupplementID     *int    `gorm:"column:supplement_id"`
		SupplementNumber *int    `gorm:"column:supplement_number"`
		SupplementStatus *string `gorm:"column:supplement_status"`
		// Точное попадание в действующий чёрный список - строка в заявке остаётся,
		// но помечается зачёркнутой. Считается на чтении, а не хранится: запись
		// могли внести после подачи заявки, и тогда флага в ней нет.
		IsBlacklisted bool `gorm:"column:is_blacklisted"`
	}
	employees := make([]empRow, 0)
	if err := s.db.WithContext(ctx).Raw(`
		SELECT
			e.id,
			e.last_name,
			e.first_name,
			e.middle_name,
			e.position,
			e.citizenship_id,
			ci.name AS citizenship_name,
			e.passport_series_number,
			e.patent_number,
			e.other_permission,
			a.entry_date_to,
			CONCAT(a.entry_time_from, ' - ', a.entry_time_to) AS pass_time,
			o.name AS organization,
			o.id   AS organization_id,
			comp.name AS company,
			comp.id   AS company_id,
			e.supplement_id,
			sup.number AS supplement_number,
			sup.status AS supplement_status,
			EXISTS(
				SELECT 1 FROM person_blacklists pbl
				WHERE pbl.is_active
				  AND LOWER(TRIM(pbl.last_name)) = LOWER(TRIM(e.last_name))
				  AND LOWER(TRIM(pbl.first_name)) = LOWER(TRIM(e.first_name))
				  AND LOWER(TRIM(COALESCE(pbl.middle_name, ''))) = LOWER(TRIM(COALESCE(e.middle_name, '')))
			) AS is_blacklisted
		FROM employees e
		JOIN attachments a ON e.attachment_id = a.id
		LEFT JOIN citizenships ci ON e.citizenship_id = ci.id
		LEFT JOIN applications app ON a.application_id = app.id
		LEFT JOIN organizations o ON app.organization_id = o.id
		LEFT JOIN companies comp ON app.company_id = comp.id
		LEFT JOIN application_supplements sup ON sup.id = e.supplement_id
		WHERE e.attachment_id = ?`+supplementScopeWhere(scope, "e"),
		attachmentID).Scan(&employees).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching employees")
	}
	for i := range employees {
		employees[i].PassportSeriesNumber = crypto.DecryptOptional(employees[i].PassportSeriesNumber)
		employees[i].PatentNumber = crypto.DecryptOptional(employees[i].PatentNumber)
	}

	empIDs := make([]int, 0, len(employees))
	for _, emp := range employees {
		empIDs = append(empIDs, emp.ID)
	}
	appID, _ := s.GetApplicationIDByAttachment(ctx, attachmentID)
	flags := s.fetchBlacklistFlags(ctx, models.BlacklistElementEmployee, appID, empIDs)

	tablesByEmployee := s.employeeTargetTablesByEmployee(ctx, empIDs)

	result := make([]EmployeeWithTables, 0)
	for _, emp := range employees {
		// Пустой срез, а не nil: фронт различает «привязок нет» и «поле не пришло».
		tables := tablesByEmployee[emp.ID]
		if tables == nil {
			tables = make([]TableInfoRef, 0)
		}

		result = append(result, EmployeeWithTables{
			IsBlacklisted:        emp.IsBlacklisted,
			ID:                   emp.ID,
			LastName:             emp.LastName,
			FirstName:            emp.FirstName,
			MiddleName:           emp.MiddleName,
			Position:             emp.Position,
			CitizenshipID:        emp.CitizenshipID,
			CitizenshipName:      emp.CitizenshipName,
			PassportSeriesNumber: emp.PassportSeriesNumber,
			PatentNumber:         emp.PatentNumber,
			OtherPermission:      emp.OtherPermission,
			EntryDateTo:          emp.EntryDateTo,
			PassTime:             emp.PassTime,
			Organization:         emp.Organization,
			OrganizationID:       emp.OrganizationID,
			Company:              emp.Company,
			CompanyID:            emp.CompanyID,
			TargetTables:         tables,
			BlacklistSimilar:     flags[emp.ID],
			SupplementMark:       supplementMark(emp.SupplementID, emp.SupplementNumber, emp.SupplementStatus),
		})
	}

	return result, nil
}

// GetAttachmentItems возвращает ТМЦ вложения. scope решает, попадают ли в выдачу позиции
// ещё не принятого дополнения (#1685).
func (s *applicationService) GetAttachmentItems(ctx context.Context, attachmentID int, scope SupplementScope) ([]ItemInfo, error) {
	type itemRow struct {
		ID          int
		Name        string
		Count       int
		DateCreated *time.Time `gorm:"column:date_created"`
		// Раунд дополнения, принёсший строку (#1685), тем же запросом - см. carRow.
		SupplementID     *int    `gorm:"column:supplement_id"`
		SupplementNumber *int    `gorm:"column:supplement_number"`
		SupplementStatus *string `gorm:"column:supplement_status"`
	}

	rows := make([]itemRow, 0)
	err := s.db.WithContext(ctx).Raw(`
		SELECT i.id, i.name, i.count, i.date_created,
			i.supplement_id,
			sup.number AS supplement_number,
			sup.status AS supplement_status
		FROM items i
		LEFT JOIN application_supplements sup ON sup.id = i.supplement_id
		WHERE i.attachment_id = ?`+supplementScopeWhere(scope, "i")+`
		ORDER BY i.id
	`, attachmentID).Scan(&rows).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching items")
	}

	items := make([]ItemInfo, 0, len(rows))
	for _, r := range rows {
		items = append(items, ItemInfo{
			ID:             r.ID,
			Name:           r.Name,
			Count:          r.Count,
			DateCreated:    r.DateCreated,
			SupplementMark: supplementMark(r.SupplementID, r.SupplementNumber, r.SupplementStatus),
		})
	}
	return items, nil
}
