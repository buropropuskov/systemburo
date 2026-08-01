package services

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"gorm.io/gorm"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
)

// Источник привязки, созданной принимающим при разборе заявки (#1393). Рядом
// живут "application" (пришло из заявки) и "manual" (ручное добавление в
// таблицу проходной) - карточка элемента показывает источник бейджем.
const TargetTableSourceApprover = "approver"

// Режимы назначения - те же, что у групповых операций справочников
// (organizationService.BulkAssignTables): add добавляет к текущему набору,
// replace заменяет его целиком.
const (
	AssignModeAdd     = "add"
	AssignModeReplace = "replace"
)

// applicationAssignmentEditableStatuses - когда принимающий может править места.
// Белый список, а не «всё кроме терминальных»: следующий терминальный статус
// иначе молча окажется разрешённым (урок #1083).
var applicationAssignmentEditableStatuses = []string{
	models.StatusUnread,
	models.StatusProcessing,
	models.StatusInWork,
}

// AssignElementTablesRequest - назначение постов проезда/прохода машинам или
// сотрудникам заявки. ElementIDs с одним значением - правка одной строки,
// со всеми - действие «назначить всем».
type AssignElementTablesRequest struct {
	ElementType string `json:"element_type" validate:"required,oneof=cars people"`
	ElementIDs  []int  `json:"element_ids" validate:"required,min=1"`
	TableIDs    []int  `json:"table_ids"`
	Mode        string `json:"mode" validate:"required,oneof=add replace"`
}

// AssignCarUnloadPlacesRequest - назначение мест разгрузки машинам заявки.
// У сотрудников мест разгрузки нет, поэтому запрос только про машины.
type AssignCarUnloadPlacesRequest struct {
	CarIDs   []int  `json:"car_ids" validate:"required,min=1"`
	PlaceIDs []int  `json:"place_ids"`
	Mode     string `json:"mode" validate:"required,oneof=add replace"`
}

// assignmentContext - общий результат проверок доступа для обеих операций.
type assignmentContext struct {
	userID int
}

// checkAssignmentAllowed проверяет, что пользователь - принимающий, а заявка
// ещё в том состоянии, когда набор мест имеет смысл менять.
func (s *applicationService) checkAssignmentAllowed(ctx context.Context, username string, applicationID int) (*assignmentContext, error) {
	user, err := s.getUserByUsername(ctx, username)
	if err != nil {
		return nil, err
	}

	isApprover, err := s.isApprover(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	if !isApprover {
		return nil, echo.NewHTTPError(http.StatusForbidden, "Назначать места может только принимающий")
	}

	var app struct {
		Status *string
	}
	res := s.db.WithContext(ctx).Raw("SELECT status FROM applications WHERE id = ?", applicationID).Scan(&app)
	if res.Error != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка чтения заявки")
	}
	if res.RowsAffected == 0 {
		return nil, echo.NewHTTPError(http.StatusNotFound, "Заявка не найдена")
	}

	status := ""
	if app.Status != nil {
		status = *app.Status
	}
	if !containsString(applicationAssignmentEditableStatuses, status) {
		return nil, echo.NewHTTPError(http.StatusBadRequest,
			fmt.Sprintf("Заявка в статусе «%s»: менять места нельзя", status))
	}

	return &assignmentContext{userID: user.ID}, nil
}

// elementsOfApplication оставляет из запрошенных id только те, что реально
// принадлежат этой заявке: чужой id из запроса не должен трогать чужую машину.
func (s *applicationService) elementsOfApplication(ctx context.Context, applicationID int, table string, ids []int) ([]int, error) {
	unique := uniqueInts(ids)
	if len(unique) == 0 {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Не переданы элементы")
	}

	var own []int
	query := fmt.Sprintf(`
		SELECT e.id FROM %s e
		JOIN attachments a ON e.attachment_id = a.id
		WHERE a.application_id = ? AND e.id IN ?`, table)
	if err := s.db.WithContext(ctx).Raw(query, applicationID, unique).Scan(&own).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка чтения элементов заявки")
	}
	if len(own) == 0 {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Элементы не принадлежат этой заявке")
	}
	return own, nil
}

// validateAssignable проверяет пригодность выбранного набора. Активность
// требуется только от НОВЫХ привязок: пост или место могли отключить уже после
// подачи заявки, и это не повод запрещать правку соседних - иначе окно
// сохранить невозможно, пока такая привязка висит.
func (s *applicationService) validateAssignable(ctx context.Context, table string, requested, alreadyLinked []int) ([]int, error) {
	unique := uniqueInts(requested)
	if len(unique) == 0 {
		return []int{}, nil
	}

	var existing []int
	query := fmt.Sprintf("SELECT id FROM %s WHERE id IN ?", table)
	if err := s.db.WithContext(ctx).Raw(query, unique).Scan(&existing).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка чтения справочника")
	}
	if len(existing) != len(unique) {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Среди выбранного есть несуществующие записи")
	}

	fresh := diffInts(unique, alreadyLinked)
	if len(fresh) == 0 {
		return unique, nil
	}

	var active []int
	activeQuery := fmt.Sprintf("SELECT id FROM %s WHERE id IN ? AND is_active = true AND status = 'active'", table)
	if err := s.db.WithContext(ctx).Raw(activeQuery, fresh).Scan(&active).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка чтения справочника")
	}
	if len(active) != len(fresh) {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Нельзя назначить отключённое место или пост")
	}
	return unique, nil
}

// linkedIDs возвращает всё, что уже привязано к перечисленным элементам.
func (s *applicationService) linkedIDs(ctx context.Context, linkTable, linkColumn, valueColumn string, elementIDs []int) ([]int, error) {
	var ids []int
	query := fmt.Sprintf("SELECT DISTINCT %s FROM %s WHERE %s IN ?", valueColumn, linkTable, linkColumn)
	if err := s.db.WithContext(ctx).Raw(query, elementIDs).Scan(&ids).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка чтения текущих привязок")
	}
	return ids, nil
}

// AssignElementTables добавляет или снимает посты у машин и сотрудников заявки.
// Добавленные принимающим строки помечаются source=approver, каждая правка
// пишется в историю элемента (added_to_table / unbound_from_table), поэтому
// потом видно, кто отправил машину на пост, которого не было в заявке.
func (s *applicationService) AssignElementTables(ctx context.Context, username string, applicationID int, req AssignElementTablesRequest) error {
	actx, err := s.checkAssignmentAllowed(ctx, username, applicationID)
	if err != nil {
		return err
	}

	elementTable, linkTable, linkColumn, entityType := "cars", "car_target_tables", "car_id", models.AuditEntityCar
	if req.ElementType == "people" {
		elementTable, linkTable, linkColumn, entityType = "employees", "employee_target_tables", "employee_id", models.AuditEntityEmployee
	}

	elementIDs, err := s.elementsOfApplication(ctx, applicationID, elementTable, req.ElementIDs)
	if err != nil {
		return err
	}
	linked, err := s.linkedIDs(ctx, linkTable, linkColumn, "table_id", elementIDs)
	if err != nil {
		return err
	}
	tableIDs, err := s.validateAssignable(ctx, "system_tables", req.TableIDs, linked)
	if err != nil {
		return err
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, elementID := range elementIDs {
			var current []int
			if err := tx.Raw(fmt.Sprintf("SELECT table_id FROM %s WHERE %s = ?", linkTable, linkColumn), elementID).
				Scan(&current).Error; err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка чтения текущих постов")
			}

			target := tableIDs
			if req.Mode == AssignModeAdd {
				target = unionInts(current, tableIDs)
			}

			for _, tableID := range diffInts(target, current) {
				if err := tx.Exec(
					fmt.Sprintf("INSERT INTO %s (%s, table_id, source) VALUES (?, ?, ?)", linkTable, linkColumn),
					elementID, tableID, TargetTableSourceApprover,
				).Error; err != nil {
					return echo.NewHTTPError(http.StatusInternalServerError, "Не удалось привязать элемент к посту")
				}
				if err := recordAddedToTable(ctx, s.recorder, tx, entityType, elementID, tableID, &actx.userID); err != nil {
					return err
				}
			}

			for _, tableID := range diffInts(current, target) {
				if err := tx.Exec(
					fmt.Sprintf("DELETE FROM %s WHERE %s = ? AND table_id = ?", linkTable, linkColumn),
					elementID, tableID,
				).Error; err != nil {
					return echo.NewHTTPError(http.StatusInternalServerError, "Не удалось снять элемент с поста")
				}
				if err := recordUnboundFromTable(ctx, s.recorder, tx, entityType, elementID, tableID, &actx.userID); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Слепок заявки хранит посты каждой машины/сотрудника (#1615, B1): назначение
	// постов их меняет и на диске должно обновиться.
	s.enqueueArchiveExport(applicationID, BlankExportReasonUpdate)
	return nil
}

// AssignCarUnloadPlaces добавляет или снимает места разгрузки у машин заявки.
// Отдельная история для мест в проекте не заведена, поэтому правка пишется как
// изменение поля машины: в карточке видно, каким был набор и каким стал.
func (s *applicationService) AssignCarUnloadPlaces(ctx context.Context, username string, applicationID int, req AssignCarUnloadPlacesRequest) error {
	actx, err := s.checkAssignmentAllowed(ctx, username, applicationID)
	if err != nil {
		return err
	}

	carIDs, err := s.elementsOfApplication(ctx, applicationID, "cars", req.CarIDs)
	if err != nil {
		return err
	}
	linked, err := s.linkedIDs(ctx, "car_unload_places", "car_id", "unload_place_id", carIDs)
	if err != nil {
		return err
	}
	placeIDs, err := s.validateAssignable(ctx, "unload_places", req.PlaceIDs, linked)
	if err != nil {
		return err
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, carID := range carIDs {
			var current []int
			if err := tx.Raw("SELECT unload_place_id FROM car_unload_places WHERE car_id = ? ORDER BY order_index", carID).
				Scan(&current).Error; err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка чтения текущих мест разгрузки")
			}

			target := placeIDs
			if req.Mode == AssignModeAdd {
				target = unionInts(current, placeIDs)
			}
			if equalIntSets(current, target) {
				continue
			}

			if err := tx.Exec("DELETE FROM car_unload_places WHERE car_id = ?", carID).Error; err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, "Не удалось обновить места разгрузки")
			}
			for index, placeID := range target {
				orderIndex := index
				if err := tx.Create(&models.CarUnloadPlace{
					CarID:         carID,
					UnloadPlaceID: placeID,
					OrderIndex:    &orderIndex,
				}).Error; err != nil {
					return echo.NewHTTPError(http.StatusInternalServerError, "Не удалось привязать место разгрузки")
				}
			}

			if err := s.recordUnloadPlacesChange(ctx, tx, carID, current, target, actx.userID); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Слепок заявки хранит места разгрузки каждой машины (#1615, B1).
	s.enqueueArchiveExport(applicationID, BlankExportReasonUpdate)
	return nil
}

// recordUnloadPlacesChange пишет в историю машины смену набора мест разгрузки
// названиями, а не идентификаторами: историю читает человек.
func (s *applicationService) recordUnloadPlacesChange(ctx context.Context, tx *gorm.DB, carID int, before, after []int, actorID int) error {
	oldNames := s.unloadPlaceNames(ctx, tx, before)
	newNames := s.unloadPlaceNames(ctx, tx, after)
	field := "unload_places"
	id := carID

	return s.recorder.Record(ctx, tx, models.AuditEntityCar, &id, "data_changed", &actorID, carAuditDetails{
		FieldName: &field,
		OldValue:  &oldNames,
		NewValue:  &newNames,
	})
}

// unloadPlaceNames возвращает названия мест одной строкой в порядке набора.
func (s *applicationService) unloadPlaceNames(ctx context.Context, tx *gorm.DB, ids []int) string {
	if len(ids) == 0 {
		return ""
	}
	type row struct {
		ID   int
		Name string
	}
	var rows []row
	if err := tx.WithContext(ctx).Raw("SELECT id, name FROM unload_places WHERE id IN ?", ids).Scan(&rows).Error; err != nil {
		return ""
	}
	byID := make(map[int]string, len(rows))
	for _, r := range rows {
		byID[r.ID] = r.Name
	}
	names := make([]string, 0, len(ids))
	for _, id := range ids {
		if name, ok := byID[id]; ok {
			names = append(names, name)
		}
	}
	return strings.Join(names, ", ")
}

// diffInts возвращает значения from, которых нет в exclude.
func diffInts(from, exclude []int) []int {
	skip := make(map[int]struct{}, len(exclude))
	for _, v := range exclude {
		skip[v] = struct{}{}
	}
	out := make([]int, 0, len(from))
	for _, v := range from {
		if _, found := skip[v]; !found {
			out = append(out, v)
		}
	}
	return out
}

// equalIntSets сравнивает наборы без учёта порядка.
func equalIntSets(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	seen := make(map[int]struct{}, len(a))
	for _, v := range a {
		seen[v] = struct{}{}
	}
	for _, v := range b {
		if _, found := seen[v]; !found {
			return false
		}
	}
	return true
}

// containsString - есть ли значение в списке.
func containsString(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}
