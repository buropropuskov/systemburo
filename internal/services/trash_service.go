package services

import (
	"context"
	"errors"
	"net/http"
	"time"

	"systemburo/internal/crypto"
	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// TrashService - корзина для cars/employees (#186). Использует существующий
// soft-delete (status=0, date_removed/date_deleted) и новое поле is_purged
// для финального удаления. Привязка элемента к таблице идёт через запись
// action_type='delete' в cars_history/employees_history с проставленным table_id.
// Восстановление разрешено только при наличии действующей согласованной заявки.
type TrashService interface {
	ListCarsTrash(ctx context.Context, systemTableID int, filter models.TrashFilter) ([]models.TrashItem, error)
	ListEmployeesTrash(ctx context.Context, systemTableID int, filter models.TrashFilter) ([]models.TrashItem, error)
	RestoreCars(ctx context.Context, systemTableID int, ids []int, userID int) (int, error)
	RestoreEmployees(ctx context.Context, systemTableID int, ids []int, userID int) (int, error)
	PurgeCar(ctx context.Context, systemTableID, id, userID int) error
	PurgeEmployee(ctx context.Context, systemTableID, id, userID int) error
	ClearCarsTrash(ctx context.Context, systemTableID, userID int) (int, error)
	ClearEmployeesTrash(ctx context.Context, systemTableID, userID int) (int, error)
	CanRestoreCar(ctx context.Context, id int) (bool, string)
	CanRestoreEmployee(ctx context.Context, id int) (bool, string)
	ListTrashHistory(ctx context.Context, systemTableID int) ([]models.TrashHistoryItem, error)
}

type trashService struct {
	db       *gorm.DB
	recorder AuditRecorder
}

// NewTrashService создаёт сервис корзины.
func NewTrashService(db *gorm.DB, recorder AuditRecorder) TrashService {
	return &trashService{db: db, recorder: recorder}
}

// ListCarsTrash возвращает удалённые из этой таблицы машины. Скоуп определяется
// записью cars_history(action_type='delete', table_id) - именно она создаётся
// при удалении машины из конкретной таблицы.
func (s *trashService) ListCarsTrash(ctx context.Context, systemTableID int, filter models.TrashFilter) ([]models.TrashItem, error) {
	sql := `
		SELECT c.id, 'car' AS type,
			a.application_number, a.id AS application_id,
			c.entry_date_to, c.entry_time_from, c.entry_time_to,
			COALESCE(c.mark_name, c.car_brand) AS mark_name, c.car_number,
			COALESCE(org.name, '') AS organization,
			COALESCE(comp.name, '') AS company,
			COALESCE((
				SELECT json_agg(json_build_object('id', up.id, 'name', up.name))
				FROM car_unload_places cup JOIN unload_places up ON up.id = cup.unload_place_id
				WHERE cup.car_id = c.id
			), '[]') AS unload_places,
			c.date_removed AS deleted_at,
			COALESCE((
				SELECT format_short_name(u.last_name, u.first_name, u.middle_name)
				FROM ` + carsHistoryUnion + ` ch JOIN users u ON u.id = ch.user_id
				WHERE ch.car_id = c.id AND ch.action_type = 'delete' AND ch.table_id = ?
				ORDER BY ch.created_at DESC LIMIT 1
			), '') AS deleted_by_name
		FROM cars c
		JOIN attachments att ON att.id = c.attachment_id
		JOIN applications a ON a.id = att.application_id
		LEFT JOIN organizations org ON org.id = a.organization_id
		LEFT JOIN companies comp ON comp.id = a.company_id
		WHERE c.status = 0 AND c.date_removed IS NOT NULL AND c.is_purged = false
			AND EXISTS (
				SELECT 1 FROM ` + carsHistoryUnion + ` ch
				WHERE ch.car_id = c.id AND ch.action_type = 'delete' AND ch.table_id = ?
			)`
	args := []any{systemTableID, systemTableID}

	if filter.Search != "" {
		sql += ` AND (c.car_number ILIKE ? OR COALESCE(c.mark_name, c.car_brand) ILIKE ?)`
		like := "%" + filter.Search + "%"
		args = append(args, like, like)
	}
	if filter.OrganizationID > 0 {
		sql += ` AND a.organization_id = ?`
		args = append(args, filter.OrganizationID)
	}
	if filter.DateFrom != "" {
		sql += ` AND c.date_removed >= ?`
		args = append(args, filter.DateFrom)
	}
	if filter.DateTo != "" {
		sql += ` AND c.date_removed <= ?`
		args = append(args, filter.DateTo+" 23:59:59")
	}
	sql += ` ORDER BY c.date_removed DESC`

	rows := make([]models.TrashItem, 0)
	if err := s.db.WithContext(ctx).Raw(sql, args...).Scan(&rows).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка получения корзины")
	}
	return rows, nil
}

// ListEmployeesTrash возвращает удалённых из этой таблицы сотрудников. Скоуп -
// запись employees_history(action_type='delete', table_id).
func (s *trashService) ListEmployeesTrash(ctx context.Context, systemTableID int, filter models.TrashFilter) ([]models.TrashItem, error) {
	sql := `
		SELECT e.id, 'employee' AS type,
			a.application_number, a.id AS application_id,
			e.last_name, e.first_name, e.middle_name,
			e.position,
			e.passport_series_number, e.patent_number, e.other_permission,
			COALESCE(org.name, '') AS organization,
			COALESCE(comp.name, '') AS company,
			COALESCE(cit.name, '') AS citizenship_name,
			COALESCE((
				SELECT json_agg(json_build_object('id', st.id, 'display_name', st.display_name))
				FROM employee_target_tables ett2 JOIN system_tables st ON st.id = ett2.table_id
				WHERE ett2.employee_id = e.id
			), '[]') AS pass_places,
			att.entry_date_to, att.entry_time_from, att.entry_time_to,
			e.date_deleted AS deleted_at,
			COALESCE((
				SELECT format_short_name(u.last_name, u.first_name, u.middle_name)
				FROM ` + employeesHistoryUnion + ` eh JOIN users u ON u.id = eh.user_id
				WHERE eh.employee_id = e.id AND eh.action_type = 'delete' AND eh.table_id = ?
				ORDER BY eh.created_at DESC LIMIT 1
			), '') AS deleted_by_name
		FROM employees e
		JOIN attachments att ON att.id = e.attachment_id
		JOIN applications a ON a.id = att.application_id
		LEFT JOIN organizations org ON org.id = a.organization_id
		LEFT JOIN companies comp ON comp.id = a.company_id
		LEFT JOIN citizenships cit ON cit.id = e.citizenship_id
		WHERE e.status = 0 AND e.date_deleted IS NOT NULL AND e.is_purged = false
			AND EXISTS (
				SELECT 1 FROM ` + employeesHistoryUnion + ` eh
				WHERE eh.employee_id = e.id AND eh.action_type = 'delete' AND eh.table_id = ?
			)`
	args := []any{systemTableID, systemTableID}

	if filter.Search != "" {
		sql += ` AND (e.last_name ILIKE ? OR e.first_name ILIKE ? OR e.middle_name ILIKE ?)`
		like := "%" + filter.Search + "%"
		args = append(args, like, like, like)
	}
	if filter.OrganizationID > 0 {
		sql += ` AND a.organization_id = ?`
		args = append(args, filter.OrganizationID)
	}
	if filter.DateFrom != "" {
		sql += ` AND e.date_deleted >= ?`
		args = append(args, filter.DateFrom)
	}
	if filter.DateTo != "" {
		sql += ` AND e.date_deleted <= ?`
		args = append(args, filter.DateTo+" 23:59:59")
	}
	sql += ` ORDER BY e.date_deleted DESC`

	rows := make([]models.TrashItem, 0)
	if err := s.db.WithContext(ctx).Raw(sql, args...).Scan(&rows).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка получения корзины")
	}
	// Паспорт и патент хранятся в зашифрованном виде; raw-scan не вызывает
	// AfterFind модели Employee, поэтому расшифровываем вручную.
	for i := range rows {
		rows[i].PassportSeriesNumber = crypto.DecryptOptional(rows[i].PassportSeriesNumber)
		rows[i].PatentNumber = crypto.DecryptOptional(rows[i].PatentNumber)
	}
	return rows, nil
}

// CanRestoreCar - проверка что есть действующая согласованная заявка на эту машину.
func (s *trashService) CanRestoreCar(ctx context.Context, id int) (bool, string) {
	var cnt int64
	err := s.db.WithContext(ctx).
		Table("cars c").
		Joins("JOIN attachments a ON a.id = c.attachment_id").
		Joins("JOIN applications app ON app.id = a.application_id").
		Where("c.id = ?", id).
		Where("app.confirmation = ?", models.ConfirmationApproved).
		Where("app.status NOT IN ?", []string{models.StatusCompleted, models.StatusRefused}).
		Where("(a.entry_date_to IS NULL OR a.entry_date_to::date >= CURRENT_DATE)").
		Count(&cnt).Error
	if err != nil || cnt == 0 {
		return false, "Нет активной согласованной заявки - восстановление невозможно"
	}
	return true, ""
}

func (s *trashService) CanRestoreEmployee(ctx context.Context, id int) (bool, string) {
	var cnt int64
	err := s.db.WithContext(ctx).
		Table("employees e").
		Joins("JOIN attachments a ON a.id = e.attachment_id").
		Joins("JOIN applications app ON app.id = a.application_id").
		Where("e.id = ?", id).
		Where("app.confirmation = ?", models.ConfirmationApproved).
		Where("app.status NOT IN ?", []string{models.StatusCompleted, models.StatusRefused}).
		Where("(a.entry_date_to IS NULL OR a.entry_date_to::date >= CURRENT_DATE)").
		Count(&cnt).Error
	if err != nil || cnt == 0 {
		return false, "Нет активной согласованной заявки - восстановление невозможно"
	}
	return true, ""
}

func (s *trashService) RestoreCars(ctx context.Context, systemTableID int, ids []int, userID int) (int, error) {
	restoredIDs := make([]int, 0, len(ids))
	for _, id := range ids {
		ok, _ := s.CanRestoreCar(ctx, id)
		if !ok {
			continue
		}
		err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			res := tx.Model(&models.Car{}).Where("id = ? AND status = 0 AND is_purged = false", id).
				Updates(map[string]any{"status": 1, "date_removed": nil})
			if res.Error != nil || res.RowsAffected == 0 {
				return errors.New("not in trash")
			}
			tableID := systemTableID
			return s.recorder.Record(ctx, tx, models.AuditEntityCar, &id, "restore", &userID, carAuditDetails{TableID: &tableID})
		})
		if err == nil {
			restoredIDs = append(restoredIDs, id)
		}
	}
	if len(restoredIDs) >= 1 {
		s.logTrashAction(ctx, systemTableID, models.TrashActionBulkRestored, len(restoredIDs), userID, s.carDetails(ctx, restoredIDs))
	}
	return len(restoredIDs), nil
}

func (s *trashService) RestoreEmployees(ctx context.Context, systemTableID int, ids []int, userID int) (int, error) {
	restoredIDs := make([]int, 0, len(ids))
	for _, id := range ids {
		ok, _ := s.CanRestoreEmployee(ctx, id)
		if !ok {
			continue
		}
		err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			res := tx.Model(&models.Employee{}).Where("id = ? AND status = 0 AND is_purged = false", id).
				Updates(map[string]any{"status": 1, "date_deleted": nil})
			if res.Error != nil || res.RowsAffected == 0 {
				return errors.New("not in trash")
			}
			tableID := systemTableID
			return tx.Create(&models.EmployeeHistory{
				EmployeeID: id,
				UserID:     &userID,
				ActionType: "restore",
				TableID:    &tableID,
			}).Error
		})
		if err == nil {
			restoredIDs = append(restoredIDs, id)
		}
	}
	if len(restoredIDs) >= 1 {
		s.logTrashAction(ctx, systemTableID, models.TrashActionBulkRestored, len(restoredIDs), userID, s.employeeDetails(ctx, restoredIDs))
	}
	return len(restoredIDs), nil
}

func (s *trashService) PurgeCar(ctx context.Context, systemTableID, id, userID int) error {
	now := time.Now().UTC()
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&models.Car{}).
			Where("id = ? AND status = 0 AND is_purged = false", id).
			Updates(map[string]any{"is_purged": true, "purged_at": now, "purged_by_user_id": userID})
		if res.Error != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка удаления")
		}
		if res.RowsAffected == 0 {
			return echo.NewHTTPError(http.StatusNotFound, "Запись не в корзине")
		}
		tableID := systemTableID
		comment := "Безвозвратно удалён из корзины"
		return s.recorder.Record(ctx, tx, models.AuditEntityCar, &id, "purge", &userID, carAuditDetails{Comment: &comment, TableID: &tableID})
	})
}

func (s *trashService) PurgeEmployee(ctx context.Context, systemTableID, id, userID int) error {
	now := time.Now().UTC()
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&models.Employee{}).
			Where("id = ? AND status = 0 AND is_purged = false", id).
			Updates(map[string]any{"is_purged": true, "purged_at": now, "purged_by_user_id": userID})
		if res.Error != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка удаления")
		}
		if res.RowsAffected == 0 {
			return echo.NewHTTPError(http.StatusNotFound, "Запись не в корзине")
		}
		tableID := systemTableID
		comment := "Безвозвратно удалён из корзины"
		return tx.Create(&models.EmployeeHistory{
			EmployeeID: id,
			UserID:     &userID,
			ActionType: "purge",
			TableID:    &tableID,
			Comment:    &comment,
			CreatedAt:  now,
		}).Error
	})
}

// ClearCarsTrash покрывает все cars в корзине этой таблицы (через cars_history.table_id).
func (s *trashService) ClearCarsTrash(ctx context.Context, systemTableID, userID int) (int, error) {
	now := time.Now().UTC()
	subqIDs := func() *gorm.DB {
		return s.db.Raw(`SELECT DISTINCT ch.car_id FROM `+carsHistoryUnion+` ch
			WHERE ch.action_type = 'delete' AND ch.table_id = ?`, systemTableID)
	}
	// Детали очищаемых машин собираем до purge.
	purgeIDs := make([]int, 0)
	s.db.WithContext(ctx).Model(&models.Car{}).
		Where("status = 0 AND is_purged = false AND id IN (?)", subqIDs()).
		Pluck("id", &purgeIDs)
	details := s.carDetails(ctx, purgeIDs)
	res := s.db.WithContext(ctx).Model(&models.Car{}).
		Where("status = 0 AND is_purged = false AND id IN (?)", subqIDs()).
		Updates(map[string]any{"is_purged": true, "purged_at": now, "purged_by_user_id": userID})
	if res.Error != nil {
		return 0, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка очистки")
	}
	if res.RowsAffected > 0 {
		s.logTrashAction(ctx, systemTableID, models.TrashActionCleared, int(res.RowsAffected), userID, details)
	}
	return int(res.RowsAffected), nil
}

func (s *trashService) ClearEmployeesTrash(ctx context.Context, systemTableID, userID int) (int, error) {
	now := time.Now().UTC()
	subqIDs := func() *gorm.DB {
		return s.db.Raw(`SELECT DISTINCT eh.employee_id FROM `+employeesHistoryUnion+` eh
			WHERE eh.action_type = 'delete' AND eh.table_id = ?`, systemTableID)
	}
	purgeIDs := make([]int, 0)
	s.db.WithContext(ctx).Model(&models.Employee{}).
		Where("status = 0 AND is_purged = false AND id IN (?)", subqIDs()).
		Pluck("id", &purgeIDs)
	details := s.employeeDetails(ctx, purgeIDs)
	res := s.db.WithContext(ctx).Model(&models.Employee{}).
		Where("status = 0 AND is_purged = false AND id IN (?)", subqIDs()).
		Updates(map[string]any{"is_purged": true, "purged_at": now, "purged_by_user_id": userID})
	if res.Error != nil {
		return 0, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка очистки")
	}
	if res.RowsAffected > 0 {
		s.logTrashAction(ctx, systemTableID, models.TrashActionCleared, int(res.RowsAffected), userID, details)
	}
	return int(res.RowsAffected), nil
}

// ListTrashHistory возвращает лог массовых действий с корзиной таблицы.
// Переходный период #870: union объединяет legacy system_table_trash_histories и audit_log.
func (s *trashService) ListTrashHistory(ctx context.Context, systemTableID int) ([]models.TrashHistoryItem, error) {
	const userName = `COALESCE(format_short_name(u.last_name, u.first_name, u.middle_name), '')`
	sql := `
		SELECT id, action_type, affected_count, details, user_name, created_at FROM (
			SELECT h.id, h.action_type, h.affected_count, h.details,
				` + userName + ` AS user_name,
				h.created_at
			FROM system_table_trash_histories h LEFT JOIN users u ON u.id = h.user_id
			WHERE h.system_table_id = ?
			UNION ALL
			SELECT a.id, a.action AS action_type,
				COALESCE((a.details->>'affected_count')::int, 0) AS affected_count,
				COALESCE(a.details->'items', '[]'::jsonb) AS details,
				` + userName + ` AS user_name,
				a.created_at
			FROM audit_log a LEFT JOIN users u ON u.id = a.actor_user_id
			WHERE a.entity_type = ? AND a.entity_id = ?
		) merged
		ORDER BY created_at DESC, id DESC`
	rows := make([]models.TrashHistoryItem, 0)
	if err := s.db.WithContext(ctx).Raw(sql, systemTableID, models.AuditEntitySystemTableTrash, systemTableID).Scan(&rows).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка получения истории корзины")
	}
	return rows, nil
}

// logTrashAction пишет запись аудита для корзины. Ошибка не прерывает основную операцию.
func (s *trashService) logTrashAction(ctx context.Context, systemTableID int, action string, count, userID int, items []models.TrashDetail) {
	uid := userID
	details := struct {
		AffectedCount int                  `json:"affected_count"`
		Items         []models.TrashDetail `json:"items,omitempty"`
	}{
		AffectedCount: count,
		Items:         items,
	}
	s.recorder.Log(ctx, nil, models.AuditEntitySystemTableTrash, &systemTableID, action, &uid, details)
}

// carDetails возвращает [{id,label}] машин для лога корзины (номер + марка).
func (s *trashService) carDetails(ctx context.Context, ids []int) []models.TrashDetail {
	if len(ids) == 0 {
		return nil
	}
	rows := make([]models.TrashDetail, 0)
	s.db.WithContext(ctx).Table("cars").
		Select("id, TRIM(COALESCE(car_number, '') || ' ' || COALESCE(mark_name, car_brand, '')) AS label").
		Where("id IN ?", ids).Scan(&rows)
	return rows
}

// employeeDetails возвращает [{id,label}] сотрудников для лога корзины (ФИО).
func (s *trashService) employeeDetails(ctx context.Context, ids []int) []models.TrashDetail {
	if len(ids) == 0 {
		return nil
	}
	rows := make([]models.TrashDetail, 0)
	s.db.WithContext(ctx).Table("employees").
		Select("id, TRIM(CONCAT_WS(' ', last_name, first_name, middle_name)) AS label").
		Where("id IN ?", ids).Scan(&rows)
	return rows
}
