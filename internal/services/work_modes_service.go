package services

import (
	"context"
	"fmt"
	"time"

	"systemburo/internal/models"
)

// WorkModeSlot -- единая форма временного слота расписания для всех трёх типов
// (места разгрузки, места прохода, Бюро). Агрегатор режимов работы (C2) приводит
// к ней разные исходные модели слотов, чтобы фронт рисовал расписание одинаково.
type WorkModeSlot struct {
	DayOfWeek int    `json:"day_of_week"` // 0=Пн..6=Вс
	OpenTime  string `json:"open_time"`
	CloseTime string `json:"close_time"`
	IsNextDay bool   `json:"is_next_day"`
	IsActive  bool   `json:"is_active"`
}

// WorkModeEntity -- объект расписания: место разгрузки, место прохода (пост) или
// Бюро. Status -- операционный статус (active|inactive|maintenance), как в
// карточках UnloadPlaceModal/TableInfoModal: при status != "active" фронт рисует
// бейдж «Неактивно»/«На обслуживании», иначе open/closed по CurrentStatus. Бюро
// всегда active (single-owner, отдельного поля статуса у него нет).
type WorkModeEntity struct {
	ID            int            `json:"id"`
	Kind          string         `json:"kind"` // bureau | unload_place | checkpoint
	Name          string         `json:"name"`
	Status        string         `json:"status"`         // active | inactive | maintenance
	CurrentStatus string         `json:"current_status"` // open | closed
	TimeSlots     []WorkModeSlot `json:"time_slots"`
}

// WorkModesResponse -- сгруппированный по типам ответ агрегатора режимов работы.
// Бюро присутствует всегда (single-owner). Списки мест возвращают только
// неархивные записи (is_active=true), включая операционно неактивные
// (status != "active") -- их фронт показывает с бейджем «Неактивно».
type WorkModesResponse struct {
	Bureau       WorkModeEntity   `json:"bureau"`
	UnloadPlaces []WorkModeEntity `json:"unload_places"`
	Checkpoints  []WorkModeEntity `json:"checkpoints"`
}

// WorkModesService -- read-only агрегатор расписаний всех мест и Бюро (C2).
type WorkModesService interface {
	GetWorkModes(ctx context.Context) (*WorkModesResponse, error)
}

type workModesService struct {
	unloadPlaces UnloadPlaceService
	systemTables SystemTableService
	bureau       BureauService
}

// NewWorkModesService создаёт агрегатор поверх сервисов мест разгрузки,
// системных таблиц (места прохода) и Бюро. Своих запросов к БД не делает -- берёт
// готовые детали с уже вычисленным current_status, чтобы не дублировать логику.
func NewWorkModesService(unloadPlaces UnloadPlaceService, systemTables SystemTableService, bureau BureauService) WorkModesService {
	return &workModesService{unloadPlaces: unloadPlaces, systemTables: systemTables, bureau: bureau}
}

// GetWorkModes собирает расписания трёх типов в единую форму. current_status мест
// и постов берётся как есть из их сервисов (canonical-логика), Бюро считается
// здесь (всегда операционно активно).
func (s *workModesService) GetWorkModes(ctx context.Context) (*WorkModesResponse, error) {
	// Осознанно переиспользуем GetAll сервисов (отдают и фото, агрегатору ненужные)
	// ради canonical current_status без дублирования SQL. Эндпоинт редкий (открытие
	// модалки), число мест/постов невелико -- лишние запросы фото приемлемы.
	places, err := s.unloadPlaces.GetAll(ctx, false)
	if err != nil {
		return nil, fmt.Errorf("failed to load unload places for work modes: %w", err)
	}
	tables, err := s.systemTables.GetAll(ctx, false)
	if err != nil {
		return nil, fmt.Errorf("failed to load system tables for work modes: %w", err)
	}
	bureauSlots, err := s.bureau.GetTimeSlots(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load bureau schedule for work modes: %w", err)
	}

	resp := &WorkModesResponse{
		Bureau:       buildBureauWorkMode(bureauSlots),
		UnloadPlaces: make([]WorkModeEntity, 0, len(places)),
		Checkpoints:  make([]WorkModeEntity, 0, len(tables)),
	}

	for _, p := range places {
		resp.UnloadPlaces = append(resp.UnloadPlaces, WorkModeEntity{
			ID:            p.ID,
			Kind:          "unload_place",
			Name:          p.Name,
			Status:        normalizeStatus(p.Status),
			CurrentStatus: p.CurrentStatus,
			TimeSlots:     unloadSlotsToWorkMode(p.TimeSlots),
		})
	}

	for _, t := range tables {
		resp.Checkpoints = append(resp.Checkpoints, WorkModeEntity{
			ID:            t.Table.ID,
			Kind:          "checkpoint",
			Name:          systemTableName(t.Table),
			Status:        normalizeStatus(t.Table.Status),
			CurrentStatus: t.CurrentStatus,
			TimeSlots:     systemTableSlotsToWorkMode(t.TimeSlots),
		})
	}

	return resp, nil
}

// buildBureauWorkMode превращает слоты Бюро в единый объект расписания. Бюро
// single-owner: фиксированное имя, статус всегда active, текущий статус -- по слотам.
func buildBureauWorkMode(slots []models.BureauTimeSlot) WorkModeEntity {
	wmSlots := make([]WorkModeSlot, 0, len(slots))
	for _, s := range slots {
		wmSlots = append(wmSlots, WorkModeSlot{
			DayOfWeek: s.DayOfWeek,
			OpenTime:  s.OpenTime,
			CloseTime: s.CloseTime,
			IsNextDay: s.IsNextDay,
			IsActive:  s.IsActive,
		})
	}
	return WorkModeEntity{
		ID:            0,
		Kind:          "bureau",
		Name:          "Бюро",
		Status:        "active",
		CurrentStatus: computeWorkModeStatus(wmSlots),
		TimeSlots:     wmSlots,
	}
}

// moscowWorkModeLoc -- бизнес-зона расписаний (МСК, UTC+3 без DST с 2014).
// FixedZone, а не LoadLocation: в alpine-образе нет tzdata, LoadLocation упал бы
// на UTC и статус "Открыто/Закрыто" съехал бы на 3 часа (баг #868: пятница в
// рабочее время по Москве показывало "Закрыто", т.к. в контейнере UTC).
var moscowWorkModeLoc = time.FixedZone("MSK", 3*60*60)

// MoscowLocation отдаёт ту же зону наружу пакета: показ времени человеку -
// выгрузки, письма, отметки - идёт по московским часам, а не по зоне сервера
// или машины (#2298).
func MoscowLocation() *time.Location {
	return moscowWorkModeLoc
}

// computeWorkModeStatus вычисляет текущий статус (open/closed) по слотам единой
// формы. Зеркалит canonical computeUnloadPlaceStatus/computeCurrentStatus
// (круглосуточный слот 00:00-23:59, переход через полночь is_next_day), но без
// проверки операционного статуса -- применяется к Бюро, которое всегда active.
func computeWorkModeStatus(slots []WorkModeSlot) string {
	now := time.Now().In(moscowWorkModeLoc)
	// 0=Пн..6=Вс (Go Weekday: 0=Вс).
	currentDay := int(now.Weekday()+6) % 7
	currentTime := now.Format("15:04")

	for _, s := range slots {
		if s.DayOfWeek == currentDay && s.IsActive &&
			s.OpenTime == "00:00" && s.CloseTime == "23:59" && !s.IsNextDay {
			return "open"
		}
	}

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

// normalizeStatus подставляет "active" вместо пустого статуса (старые строки без
// явного значения), чтобы фронт не путал "" с неактивностью.
func normalizeStatus(status string) string {
	if status == "" {
		return "active"
	}
	return status
}

// systemTableName -- отображаемое имя поста: display_name, иначе техническое name.
func systemTableName(t models.SystemTable) string {
	if t.DisplayName != nil && *t.DisplayName != "" {
		return *t.DisplayName
	}
	return t.Name
}

func unloadSlotsToWorkMode(slots []models.UnloadPlaceTimeSlot) []WorkModeSlot {
	out := make([]WorkModeSlot, 0, len(slots))
	for _, s := range slots {
		out = append(out, WorkModeSlot{
			DayOfWeek: s.DayOfWeek,
			OpenTime:  s.OpenTime,
			CloseTime: s.CloseTime,
			IsNextDay: s.IsNextDay,
			IsActive:  s.IsActive,
		})
	}
	return out
}

func systemTableSlotsToWorkMode(slots []models.SystemTableTimeSlot) []WorkModeSlot {
	out := make([]WorkModeSlot, 0, len(slots))
	for _, s := range slots {
		out = append(out, WorkModeSlot{
			DayOfWeek: s.DayOfWeek,
			OpenTime:  s.OpenTime,
			CloseTime: s.CloseTime,
			IsNextDay: s.IsNextDay,
			IsActive:  s.IsActive,
		})
	}
	return out
}
