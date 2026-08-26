package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CreateUnloadPlaceRequest -- тело запроса на создание места разгрузки.
type CreateUnloadPlaceRequest struct {
	Name          string  `json:"name" validate:"required,min=1,max=200"`
	Description   *string `json:"description"`
	Warning       *string `json:"warning"`
	MapLink       *string `json:"map_link"`
	Status        *string `json:"status"`
	StatusComment *string `json:"status_comment"`
}

// UpdateUnloadPlaceRequest -- тело запроса на обновление места разгрузки.
type UpdateUnloadPlaceRequest struct {
	Name          *string `json:"name"`
	Description   *string `json:"description"`
	Warning       *string `json:"warning"`
	MapLink       *string `json:"map_link"`
	Status        *string `json:"status"`
	StatusComment *string `json:"status_comment"`
}

// CreateTimeSlotRequest -- тело запроса на создание временного слота.
type CreateTimeSlotRequest struct {
	DayOfWeek int    `json:"day_of_week" validate:"min=0,max=6"`
	OpenTime  string `json:"open_time" validate:"required"`
	CloseTime string `json:"close_time" validate:"required"`
	IsNextDay *bool  `json:"is_next_day"`
	IsActive  *bool  `json:"is_active"`
}

// UpdateTimeSlotRequest -- тело запроса на обновление временного слота.
type UpdateTimeSlotRequest struct {
	DayOfWeek *int    `json:"day_of_week"`
	OpenTime  *string `json:"open_time"`
	CloseTime *string `json:"close_time"`
	IsNextDay *bool   `json:"is_next_day"`
	IsActive  *bool   `json:"is_active"`
}

// UnloadPlaceWithDetails -- место разгрузки с расписанием, фотографиями и текущим статусом.
type UnloadPlaceWithDetails struct {
	ID            int                          `json:"id"`
	Name          string                       `json:"name"`
	Description   *string                      `json:"description"`
	Warning       *string                      `json:"warning"`
	MapLink       *string                      `json:"map_link"`
	Status        string                       `json:"status"`
	StatusComment *string                      `json:"status_comment"`
	IsActive      bool                         `json:"is_active"`
	CurrentStatus string                       `json:"current_status"`
	TimeSlots     []models.UnloadPlaceTimeSlot `json:"time_slots"`
	// WarningWindows -- предупреждения по временным окнам (#1183), показываются
	// заявителю, когда срок заявки пересекается с окном.
	WarningWindows []models.UnloadPlaceWarningWindow `json:"warning_windows"`
	Photos         []models.UnloadPlacePhoto         `json:"photos"`
	CreatedAt      time.Time                         `json:"created_at"`
	UpdatedAt      time.Time                         `json:"updated_at"`
}

// UnloadPlaceBinding -- привязанная к месту разгрузки организация/компания.
// IsActive=false помечает архивную запись (её всё равно показываем: гейт Delete
// считает по junction без фильтра активности, поэтому она держит место).
type UnloadPlaceBinding struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	IsActive bool   `json:"is_active"`
}

// UnloadPlaceUsage -- организации и компании, привязанные к месту разгрузки.
// Набор совпадает с тем, что блокирует удаление (см. Delete).
type UnloadPlaceUsage struct {
	Organizations []UnloadPlaceBinding `json:"organizations"`
	Companies     []UnloadPlaceBinding `json:"companies"`
}

// UnloadPlaceDetachResult -- сколько привязок снято операцией «Отвязать всё».
type UnloadPlaceDetachResult struct {
	OrganizationsDetached int `json:"organizations_detached"`
	CompaniesDetached     int `json:"companies_detached"`
}

// UnloadPlaceService -- интерфейс бизнес-логики мест разгрузки.
// В мутациях callerUserID - актор для аудита (#413).
type UnloadPlaceService interface {
	GetAll(ctx context.Context, includeArchived bool) ([]UnloadPlaceWithDetails, error)
	GetByID(ctx context.Context, id int) (*UnloadPlaceWithDetails, error)
	Create(ctx context.Context, callerUserID int, req CreateUnloadPlaceRequest) (int, error)
	Update(ctx context.Context, callerUserID, id int, req UpdateUnloadPlaceRequest) error
	Delete(ctx context.Context, callerUserID, id int) error
	Restore(ctx context.Context, callerUserID, id int) error
	BulkArchive(ctx context.Context, callerUserID int, ids []int) (*BulkOpResult, error)
	BulkRestore(ctx context.Context, callerUserID int, ids []int) (*BulkOpResult, error)

	// GetUsage возвращает организации и компании, привязанные к месту разгрузки
	// (те же, что блокируют Delete). DetachAll снимает все эти привязки разом,
	// DetachOrganization/DetachCompany - по одной. Все возвращают detached=false
	// без ошибки, если привязки уже нет (идемпотентно).
	GetUsage(ctx context.Context, id int) (*UnloadPlaceUsage, error)
	DetachAll(ctx context.Context, callerUserID, id int) (*UnloadPlaceDetachResult, error)
	DetachOrganization(ctx context.Context, callerUserID, id, organizationID int) (bool, error)
	DetachCompany(ctx context.Context, callerUserID, id, companyID int) (bool, error)

	// GetHistory возвращает историю изменений места разгрузки (новые сверху).
	GetHistory(ctx context.Context, id int) ([]models.UnloadPlaceHistoryItem, error)

	// Временные слоты
	GetTimeSlots(ctx context.Context, placeID int) ([]models.UnloadPlaceTimeSlot, error)
	AddTimeSlot(ctx context.Context, placeID int, req CreateTimeSlotRequest) (int, error)
	UpdateTimeSlot(ctx context.Context, placeID, slotID int, req UpdateTimeSlotRequest) error
	DeleteTimeSlot(ctx context.Context, placeID, slotID int) error

	// Предупреждения по временным окнам (#1183)
	GetWarningWindows(ctx context.Context, placeID int) ([]models.UnloadPlaceWarningWindow, error)
	AddWarningWindow(ctx context.Context, placeID int, req models.WarningWindowRequest) (int, error)
	UpdateWarningWindow(ctx context.Context, placeID, windowID int, req models.WarningWindowRequest) error
	DeleteWarningWindow(ctx context.Context, placeID, windowID int) error

	// Фотографии
	UploadPhoto(ctx context.Context, placeID int, username string, photoURL, fileName, mimeType string, fileSize int64) (int, error)
	DeletePhoto(ctx context.Context, placeID, photoID int) (string, error)
	SetMainPhoto(ctx context.Context, placeID, photoID int) error
}

type unloadPlaceService struct {
	db       *gorm.DB
	recorder AuditRecorder
}

// NewUnloadPlaceService создаёт реализацию UnloadPlaceService.
func NewUnloadPlaceService(db *gorm.DB) UnloadPlaceService {
	return &unloadPlaceService{db: db, recorder: NewAuditRecorder(db)}
}

// computeUnloadPlaceStatus определяет текущий статус (open/closed) по расписанию.
func computeUnloadPlaceStatus(status string, slots []models.UnloadPlaceTimeSlot) string {
	return computeUnloadPlaceStatusAt(time.Now(), status, slots)
}

// computeUnloadPlaceStatusAt - чистое ядро статуса с инъекцией now (для теста).
// День недели и время берутся в МСК (как bureau computeWorkModeStatus): сервер
// крутится в UTC, а слоты заданы в московском дне - без конверсии у границы суток
// (21:00-24:00 UTC) currentDay уходит на сутки назад и статус врёт.
func computeUnloadPlaceStatusAt(now time.Time, status string, slots []models.UnloadPlaceTimeSlot) string {
	if status != "active" {
		return "closed"
	}

	now = now.In(moscowWorkModeLoc)
	// 0=Пн, 6=Вс (совпадает с Rust: weekday().num_days_from_monday())
	currentDay := int(now.Weekday()+6) % 7
	currentTime := now.Format("15:04")

	// Проверяем круглосуточный слот
	for _, s := range slots {
		if s.DayOfWeek == currentDay && s.IsActive &&
			s.OpenTime == "00:00" && s.CloseTime == "23:59" && !s.IsNextDay {
			return "open"
		}
	}

	// Проверяем обычные слоты
	for _, s := range slots {
		if s.DayOfWeek != currentDay || !s.IsActive {
			continue
		}
		if s.IsNextDay {
			if currentTime >= s.OpenTime {
				return "open"
			}
		} else {
			if currentTime >= s.OpenTime && currentTime <= s.CloseTime {
				return "open"
			}
		}
	}

	return "closed"
}

// buildDetails собирает UnloadPlaceWithDetails из места, его слотов и фото.
func (s *unloadPlaceService) buildDetails(ctx context.Context, place models.UnloadPlace) UnloadPlaceWithDetails {
	slots := make([]models.UnloadPlaceTimeSlot, 0)
	s.db.WithContext(ctx).
		Where("unload_place_id = ?", place.ID).
		Order("day_of_week, open_time").
		Find(&slots)

	windows := make([]models.UnloadPlaceWarningWindow, 0)
	s.db.WithContext(ctx).
		Where("unload_place_id = ?", place.ID).
		Order("day_of_week NULLS FIRST, time_from NULLS FIRST").
		Find(&windows)

	photos := make([]models.UnloadPlacePhoto, 0)
	s.db.WithContext(ctx).
		Where("unload_place_id = ?", place.ID).
		Order("is_main DESC, uploaded_at DESC").
		Find(&photos)

	status := place.Status
	if status == "" {
		status = "active"
	}

	return UnloadPlaceWithDetails{
		ID:             place.ID,
		Name:           place.Name,
		Description:    place.Description,
		Warning:        place.Warning,
		MapLink:        place.MapLink,
		Status:         status,
		StatusComment:  place.StatusComment,
		IsActive:       place.IsActive,
		CurrentStatus:  computeUnloadPlaceStatus(status, slots),
		TimeSlots:      slots,
		WarningWindows: windows,
		Photos:         photos,
		CreatedAt:      place.CreatedAt,
		UpdatedAt:      place.UpdatedAt,
	}
}

// GetAll возвращает все места разгрузки с расписанием и фотографиями.
func (s *unloadPlaceService) GetAll(ctx context.Context, includeArchived bool) ([]UnloadPlaceWithDetails, error) {
	places := make([]models.UnloadPlace, 0)
	q := s.db.WithContext(ctx).Order("name")
	if !includeArchived {
		q = q.Where("is_active = ?", true)
	}
	if err := q.Find(&places).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching unload places")
	}

	result := make([]UnloadPlaceWithDetails, 0, len(places))
	for _, p := range places {
		result = append(result, s.buildDetails(ctx, p))
	}
	return result, nil
}

// GetByID возвращает место разгрузки по ID с расписанием и фотографиями.
func (s *unloadPlaceService) GetByID(ctx context.Context, id int) (*UnloadPlaceWithDetails, error) {
	var place models.UnloadPlace
	if err := s.db.WithContext(ctx).First(&place, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, echo.NewHTTPError(http.StatusNotFound, "Место разгрузки не найдено")
		}
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching unload place")
	}

	details := s.buildDetails(ctx, place)
	return &details, nil
}

// Create создаёт новое место разгрузки.
func (s *unloadPlaceService) Create(ctx context.Context, callerUserID int, req CreateUnloadPlaceRequest) (int, error) {
	status := "active"
	if req.Status != nil {
		status = *req.Status
	}

	place := models.UnloadPlace{
		Name:          req.Name,
		Description:   req.Description,
		Warning:       req.Warning,
		MapLink:       req.MapLink,
		Status:        status,
		StatusComment: req.StatusComment,
		IsActive:      true,
		UpdatedAt:     time.Now().UTC(),
	}

	if err := s.db.WithContext(ctx).Create(&place).Error; err != nil {
		slog.Error("не удалось создать место разгрузки", "error", err)
		return 0, echo.NewHTTPError(http.StatusInternalServerError, "Error creating unload place")
	}
	slog.Info("место разгрузки создано", "id", place.ID)
	s.recorder.Log(ctx, nil, models.AuditEntityUnloadPlace, &place.ID, models.UnloadPlaceActionCreated, &callerUserID, map[string]any{"name": place.Name})
	return place.ID, nil
}

// Update обновляет место разгрузки по ID. Переименование логируется в историю;
// смена статуса/описания/ссылки аудитом этого среза не покрывается.
func (s *unloadPlaceService) Update(ctx context.Context, callerUserID, id int, req UpdateUnloadPlaceRequest) error {
	var place models.UnloadPlace
	if err := s.db.WithContext(ctx).First(&place, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "Место разгрузки не найдено")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Error fetching unload place")
	}

	updates := map[string]interface{}{
		"updated_at": time.Now().UTC(),
	}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.Warning != nil {
		updates["warning"] = *req.Warning
	}
	if req.MapLink != nil {
		updates["map_link"] = *req.MapLink
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}
	if req.StatusComment != nil {
		updates["status_comment"] = *req.StatusComment
	}

	result := s.db.WithContext(ctx).Model(&models.UnloadPlace{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		slog.Error("не удалось обновить место разгрузки", "id", id, "error", result.Error)
		return echo.NewHTTPError(http.StatusInternalServerError, "Error updating unload place")
	}
	if result.RowsAffected == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "Место разгрузки не найдено")
	}
	slog.Info("место разгрузки обновлено", "id", id)
	if req.Name != nil && *req.Name != place.Name {
		s.recorder.Log(ctx, nil, models.AuditEntityUnloadPlace, &id, models.UnloadPlaceActionRenamed, &callerUserID, map[string]any{"name": *req.Name})
	}
	return nil
}

// Delete архивирует место разгрузки (soft-delete). Блокируется, если место
// привязано к организациям/компаниям - это и есть гейт активных зависимостей:
// машины планируются на место только через орг/компания-привязку, поэтому после
// отвязки новые car_unload_places не появятся, а существующие переживут архив
// (soft-delete не сиротит). Отдельный блок по car_unload_places не нужен.
func (s *unloadPlaceService) Delete(ctx context.Context, callerUserID, id int) error {
	// Проверяем привязку к организациям
	var orgCount int64
	if err := s.db.WithContext(ctx).
		Model(&models.OrganizationUnloadPlace{}).
		Where("unload_place_id = ?", id).
		Count(&orgCount).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error checking organization dependencies")
	}

	// Проверяем привязку к компаниям
	var companyCount int64
	if err := s.db.WithContext(ctx).
		Model(&models.CompaniesUnloadPlace{}).
		Where("unload_place_id = ?", id).
		Count(&companyCount).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error checking company dependencies")
	}

	if orgCount > 0 || companyCount > 0 {
		msg := "Невозможно удалить место разгрузки, так как оно привязано к: "
		var parts []string
		if orgCount > 0 {
			parts = append(parts, fmt.Sprintf("организациям (%d)", orgCount))
		}
		if companyCount > 0 {
			parts = append(parts, fmt.Sprintf("компаниям (%d)", companyCount))
		}
		for i, p := range parts {
			if i > 0 {
				msg += " и "
			}
			msg += p
		}
		return echo.NewHTTPError(http.StatusBadRequest, msg)
	}

	// Фото/слоты не трогаем - restore должен вернуть запись целой.
	result := s.db.WithContext(ctx).
		Model(&models.UnloadPlace{}).
		Where("id = ?", id).
		Update("is_active", false)
	if result.Error != nil {
		slog.Error("не удалось архивировать место разгрузки", "id", id, "error", result.Error)
		return echo.NewHTTPError(http.StatusInternalServerError, "Error archiving unload place")
	}
	if result.RowsAffected == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "Место разгрузки не найдено")
	}
	slog.Info("место разгрузки архивировано", "id", id)
	s.recorder.Log(ctx, nil, models.AuditEntityUnloadPlace, &id, models.UnloadPlaceActionArchived, &callerUserID, nil)
	return nil
}

// Restore восстанавливает архивное место разгрузки (is_active=true).
func (s *unloadPlaceService) Restore(ctx context.Context, callerUserID, id int) error {
	result := s.db.WithContext(ctx).
		Model(&models.UnloadPlace{}).
		Where("id = ?", id).
		Update("is_active", true)
	if result.Error != nil {
		slog.Error("не удалось восстановить место разгрузки", "id", id, "error", result.Error)
		return echo.NewHTTPError(http.StatusInternalServerError, "Error restoring unload place")
	}
	if result.RowsAffected == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "Место разгрузки не найдено")
	}
	slog.Info("место разгрузки восстановлено", "id", id)
	s.recorder.Log(ctx, nil, models.AuditEntityUnloadPlace, &id, models.UnloadPlaceActionRestored, &callerUserID, nil)
	return nil
}

// GetUsage возвращает организации и компании, привязанные к месту разгрузки.
// Junction читается БЕЗ фильтра is_active орг/компании: набор обязан совпадать с
// тем, что считает гейт в Delete, иначе получилось бы «привязок нет», а удалить
// нельзя. Архивные орг/компании помечаются is_active=false.
func (s *unloadPlaceService) GetUsage(ctx context.Context, id int) (*UnloadPlaceUsage, error) {
	if _, ok := s.loadUnloadPlace(ctx, id); !ok {
		return nil, echo.NewHTTPError(http.StatusNotFound, "Место разгрузки не найдено")
	}

	usage := &UnloadPlaceUsage{
		Organizations: make([]UnloadPlaceBinding, 0),
		Companies:     make([]UnloadPlaceBinding, 0),
	}
	if err := s.db.WithContext(ctx).
		Table("organization_unload_places oup").
		Select("o.id, o.name, o.is_active").
		Joins("JOIN organizations o ON o.id = oup.organization_id").
		Where("oup.unload_place_id = ?", id).
		Order("o.name").
		Scan(&usage.Organizations).Error; err != nil {
		slog.Error("не удалось прочитать привязки организаций места разгрузки", "id", id, "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching organization bindings")
	}
	if err := s.db.WithContext(ctx).
		Table("companies_unload_places cup").
		Select("c.id, c.name, c.is_active").
		Joins("JOIN companies c ON c.id = cup.company_id").
		Where("cup.unload_place_id = ?", id).
		Order("c.name").
		Scan(&usage.Companies).Error; err != nil {
		slog.Error("не удалось прочитать привязки компаний места разгрузки", "id", id, "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching company bindings")
	}
	return usage, nil
}

// DetachAll снимает привязки места разгрузки ко ВСЕМ организациям и компаниям
// (обе join-таблицы удаляются в одной транзакции). На каждую затронутую
// организацию/компанию пишется история «место убрано из набора» - зеркало
// аудита UpdateOrganizationUnloadPlaces/UpdateUnloadPlaces. После этого место
// можно архивировать (Delete больше не заблокирует). Идемпотентно: повтор по
// уже отвязанному месту возвращает нулевые счётчики.
func (s *unloadPlaceService) DetachAll(ctx context.Context, callerUserID, id int) (*UnloadPlaceDetachResult, error) {
	place, ok := s.loadUnloadPlace(ctx, id)
	if !ok {
		return nil, echo.NewHTTPError(http.StatusNotFound, "Место разгрузки не найдено")
	}

	// DELETE ... RETURNING внутри одной транзакции: удаляем привязки и получаем
	// id ровно затронутых сущностей атомарно. Отдельный SELECT-перед-DELETE дал бы
	// гонку - конкурентная привязка попала бы под DELETE, но мимо аудита.
	var orgIDs, companyIDs []int
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var removedOrgs []models.OrganizationUnloadPlace
		if err := tx.Clauses(clause.Returning{Columns: []clause.Column{{Name: "organization_id"}}}).
			Where("unload_place_id = ?", id).Delete(&removedOrgs).Error; err != nil {
			slog.Error("не удалось отвязать организации от места разгрузки", "id", id, "error", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Error detaching organizations")
		}
		var removedCompanies []models.CompaniesUnloadPlace
		if err := tx.Clauses(clause.Returning{Columns: []clause.Column{{Name: "company_id"}}}).
			Where("unload_place_id = ?", id).Delete(&removedCompanies).Error; err != nil {
			slog.Error("не удалось отвязать компании от места разгрузки", "id", id, "error", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Error detaching companies")
		}
		for _, r := range removedOrgs {
			orgIDs = append(orgIDs, r.OrganizationID)
		}
		for _, r := range removedCompanies {
			companyIDs = append(companyIDs, r.CompanyID)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	if len(orgIDs) == 0 && len(companyIDs) == 0 {
		return &UnloadPlaceDetachResult{}, nil
	}

	removed := auditNameDiff{Removed: []string{place.Name}}
	for _, orgID := range orgIDs {
		oid := orgID
		s.recorder.Log(ctx, nil, models.AuditEntityOrganization, &oid, models.OrganizationActionUnloadPlacesChanged, &callerUserID, removed)
	}
	for _, companyID := range companyIDs {
		cid := companyID
		s.recorder.Log(ctx, nil, models.AuditEntityCompany, &cid, models.CompanyActionUnloadPlacesChanged, &callerUserID, removed)
	}

	slog.Info("место разгрузки отвязано от всех орг/компаний", "id", id, "orgs", len(orgIDs), "companies", len(companyIDs))
	return &UnloadPlaceDetachResult{
		OrganizationsDetached: len(orgIDs),
		CompaniesDetached:     len(companyIDs),
	}, nil
}

// DetachOrganization снимает привязку места разгрузки к ОДНОЙ организации.
// Идемпотентно: если привязки уже нет, возвращает false без ошибки (двойной
// клик/гонка не должны падать). Аудит на организацию пишем только при реальном
// удалении строки (RowsAffected>0), removed = имя места.
func (s *unloadPlaceService) DetachOrganization(ctx context.Context, callerUserID, id, organizationID int) (bool, error) {
	place, ok := s.loadUnloadPlace(ctx, id)
	if !ok {
		return false, echo.NewHTTPError(http.StatusNotFound, "Место разгрузки не найдено")
	}
	res := s.db.WithContext(ctx).
		Where("unload_place_id = ? AND organization_id = ?", id, organizationID).
		Delete(&models.OrganizationUnloadPlace{})
	if res.Error != nil {
		slog.Error("не удалось отвязать организацию от места разгрузки", "id", id, "organization_id", organizationID, "error", res.Error)
		return false, echo.NewHTTPError(http.StatusInternalServerError, "Error detaching organization")
	}
	if res.RowsAffected == 0 {
		return false, nil
	}
	oid := organizationID
	s.recorder.Log(ctx, nil, models.AuditEntityOrganization, &oid, models.OrganizationActionUnloadPlacesChanged, &callerUserID, auditNameDiff{Removed: []string{place.Name}})
	slog.Info("место разгрузки отвязано от организации", "id", id, "organization_id", organizationID)
	return true, nil
}

// DetachCompany снимает привязку места разгрузки к ОДНОЙ компании (зеркало
// DetachOrganization, см. его комментарий).
func (s *unloadPlaceService) DetachCompany(ctx context.Context, callerUserID, id, companyID int) (bool, error) {
	place, ok := s.loadUnloadPlace(ctx, id)
	if !ok {
		return false, echo.NewHTTPError(http.StatusNotFound, "Место разгрузки не найдено")
	}
	res := s.db.WithContext(ctx).
		Where("unload_place_id = ? AND company_id = ?", id, companyID).
		Delete(&models.CompaniesUnloadPlace{})
	if res.Error != nil {
		slog.Error("не удалось отвязать компанию от места разгрузки", "id", id, "company_id", companyID, "error", res.Error)
		return false, echo.NewHTTPError(http.StatusInternalServerError, "Error detaching company")
	}
	if res.RowsAffected == 0 {
		return false, nil
	}
	cid := companyID
	s.recorder.Log(ctx, nil, models.AuditEntityCompany, &cid, models.CompanyActionUnloadPlacesChanged, &callerUserID, auditNameDiff{Removed: []string{place.Name}})
	slog.Info("место разгрузки отвязано от компании", "id", id, "company_id", companyID)
	return true, nil
}

// loadUnloadPlace подгружает место разгрузки без сборки полного набора
// деталей (слоты/фото) - для bulk-операций нужно только имя в BulkItemError,
// тяжёлый buildDetails тут ни к чему.
func (s *unloadPlaceService) loadUnloadPlace(ctx context.Context, id int) (models.UnloadPlace, bool) {
	var place models.UnloadPlace
	if err := s.db.WithContext(ctx).First(&place, id).Error; err != nil {
		return place, false
	}
	return place, true
}

// BulkArchive архивирует набор мест разгрузки через Delete. Места, привязанные
// к организациям/компаниям, честно попадают в Errors (частичный успех).
func (s *unloadPlaceService) BulkArchive(ctx context.Context, callerUserID int, ids []int) (*BulkOpResult, error) {
	res := newBulkResult()
	for _, id := range uniqueInts(ids) {
		place, ok := s.loadUnloadPlace(ctx, id)
		if !ok {
			res.addError(id, "", "Место разгрузки не найдено")
			continue
		}
		if err := s.Delete(ctx, callerUserID, id); err != nil {
			res.addError(id, place.Name, bulkErrMsg(err))
			continue
		}
		res.SuccessCount++
	}
	return res.finalize(), nil
}

// BulkRestore восстанавливает набор мест разгрузки через Restore.
func (s *unloadPlaceService) BulkRestore(ctx context.Context, callerUserID int, ids []int) (*BulkOpResult, error) {
	res := newBulkResult()
	for _, id := range uniqueInts(ids) {
		place, ok := s.loadUnloadPlace(ctx, id)
		if !ok {
			res.addError(id, "", "Место разгрузки не найдено")
			continue
		}
		if err := s.Restore(ctx, callerUserID, id); err != nil {
			res.addError(id, place.Name, bulkErrMsg(err))
			continue
		}
		res.SuccessCount++
	}
	return res.finalize(), nil
}

// GetHistory возвращает историю изменений места разгрузки (новые сверху).
// #870, финал F.2: запись и до-cutover строки живут в общем audit_log (старые
// перенесены backfill'ом BackfillAuditFromLegacy), поэтому чтение идёт только из
// audit_log. Замороженная unload_place_histories дропнута в дроп-sweep (F.8).
// Форму стережёт TestUnloadPlaces_History.
func (s *unloadPlaceService) GetHistory(ctx context.Context, id int) ([]models.UnloadPlaceHistoryItem, error) {
	const actorName = `COALESCE(NULLIF(TRIM(BOTH ' ' FROM CONCAT_WS(' ', u.last_name, u.first_name)), ''), u.username, '')`
	sql := `
		SELECT a.id AS id, a.action AS action_type, a.details AS details,
			a.actor_user_id AS actor_user_id, ` + actorName + ` AS actor_name, a.created_at AS created_at
		FROM audit_log a LEFT JOIN users u ON u.id = a.actor_user_id
		WHERE a.entity_type = ? AND a.entity_id = ?
		ORDER BY a.created_at DESC, a.id DESC`

	type row struct {
		ID          int             `gorm:"column:id"`
		ActionType  string          `gorm:"column:action_type"`
		Details     json.RawMessage `gorm:"column:details"`
		ActorUserID *int            `gorm:"column:actor_user_id"`
		ActorName   string          `gorm:"column:actor_name"`
		CreatedAt   time.Time       `gorm:"column:created_at"`
	}
	var rows []row
	if err := s.db.WithContext(ctx).Raw(sql, models.AuditEntityUnloadPlace, id).Scan(&rows).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching unload place history")
	}

	// Логин вместо ФИО у акторов, не давших согласия на обработку данных.
	masks := loadConsentMasks(ctx, s.db)
	items := make([]models.UnloadPlaceHistoryItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, models.UnloadPlaceHistoryItem{
			ID:          r.ID,
			ActionType:  r.ActionType,
			Details:     r.Details,
			ActorUserID: r.ActorUserID,
			ActorName:   maskName(masks, r.ActorUserID, r.ActorName),
			CreatedAt:   r.CreatedAt,
		})
	}
	return items, nil
}
