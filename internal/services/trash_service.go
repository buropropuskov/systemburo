package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// TrashService - корзина для cars/employees (#186). Использует существующий
// soft-delete (status=0, date_removed/date_deleted) и новое поле is_purged
// для финального удаления. Восстановление разрешено только при наличии
// действующей согласованной заявки на эту запись.
type TrashService interface {
	ListCarsTrash(ctx context.Context, systemTableID int, filter models.TrashFilter) ([]models.TrashItem, error)
	ListEmployeesTrash(ctx context.Context, systemTableID int, filter models.TrashFilter) ([]models.TrashItem, error)
	RestoreCars(ctx context.Context, systemTableID int, ids []int, userID int) (int, error)
	RestoreEmployees(ctx context.Context, systemTableID int, ids []int, userID int) (int, error)
	PurgeCar(ctx context.Context, id, userID int) error
	PurgeEmployee(ctx context.Context, id, userID int) error
	ClearCarsTrash(ctx context.Context, systemTableID, userID int) (int, error)
	ClearEmployeesTrash(ctx context.Context, systemTableID, userID int) (int, error)
	CanRestoreCar(ctx context.Context, id int) (bool, string)
	CanRestoreEmployee(ctx context.Context, id int) (bool, string)
}

type trashService struct {
	db *gorm.DB
}

// NewTrashService создаёт сервис корзины.
func NewTrashService(db *gorm.DB) TrashService {
	return &trashService{db: db}
}

// ListCarsTrash возвращает удалённые машины таблицы. Привязка car → system_table
// идёт через attachment → application_responsible_users или через cars_history.table_id.
// Используем cars_history.table_id - проще и надёжней.
func (s *trashService) ListCarsTrash(ctx context.Context, systemTableID int, filter models.TrashFilter) ([]models.TrashItem, error) {
	rows, err := s.queryCarsTrash(ctx, systemTableID, filter)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка получения корзины")
	}
	return rows, nil
}

func (s *trashService) queryCarsTrash(ctx context.Context, tableID int, f models.TrashFilter) ([]models.TrashItem, error) {
	q := s.db.WithContext(ctx).
		Table("cars c").
		Select(`c.id, 'car' AS type,
			a.application_number, c.entry_date_to, c.entry_time_to,
			COALESCE(c.mark_name, c.car_brand) AS mark_name, c.car_number,
			COALESCE(org.name, '') AS organization,
			c.date_removed AS deleted_at,
			COALESCE(format_short_name(u.last_name, u.first_name, u.middle_name), '') AS deleted_by_name`).
		Joins(`JOIN attachments att ON att.id = c.attachment_id`).
		Joins(`JOIN applications a ON a.id = att.application_id`).
		Joins(`LEFT JOIN organizations org ON org.id = a.organization_id`).
		Joins(`LEFT JOIN cars_history ch ON ch.car_id = c.id AND ch.action_type = 'delete' AND ch.table_id = ?`, tableID).
		Joins(`LEFT JOIN users u ON u.id = ch.user_id`).
		Where("c.status = 0 AND c.date_removed IS NOT NULL AND c.is_purged = false")
	if f.Search != "" {
		q = q.Where("(c.car_number ILIKE ? OR COALESCE(c.mark_name, c.car_brand) ILIKE ?)", "%"+f.Search+"%", "%"+f.Search+"%")
	}
	if f.OrganizationID > 0 {
		q = q.Where("a.organization_id = ?", f.OrganizationID)
	}
	if f.DateFrom != "" {
		q = q.Where("c.date_removed >= ?", f.DateFrom)
	}
	if f.DateTo != "" {
		q = q.Where("c.date_removed <= ?", f.DateTo+" 23:59:59")
	}
	q = q.Order("c.date_removed DESC")

	rows := make([]models.TrashItem, 0)
	if err := q.Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *trashService) ListEmployeesTrash(ctx context.Context, systemTableID int, filter models.TrashFilter) ([]models.TrashItem, error) {
	q := s.db.WithContext(ctx).
		Table("employees e").
		Select(`e.id, 'employee' AS type,
			a.application_number, e.last_name, e.first_name, e.middle_name,
			COALESCE(org.name, '') AS organization,
			e.date_deleted AS deleted_at,
			COALESCE(format_short_name(u.last_name, u.first_name, u.middle_name), '') AS deleted_by_name`).
		Joins(`JOIN attachments att ON att.id = e.attachment_id`).
		Joins(`JOIN applications a ON a.id = att.application_id`).
		Joins(`LEFT JOIN organizations org ON org.id = a.organization_id`).
		Joins(`LEFT JOIN employees_history eh ON eh.employee_id = e.id AND eh.action_type = 'delete' AND eh.table_id = ?`, systemTableID).
		Joins(`LEFT JOIN users u ON u.id = eh.user_id`).
		Where("e.status = 0 AND e.date_deleted IS NOT NULL AND e.is_purged = false")
	if filter.Search != "" {
		q = q.Where("(e.last_name ILIKE ? OR e.first_name ILIKE ?)", "%"+filter.Search+"%", "%"+filter.Search+"%")
	}
	if filter.OrganizationID > 0 {
		q = q.Where("a.organization_id = ?", filter.OrganizationID)
	}
	if filter.DateFrom != "" {
		q = q.Where("e.date_deleted >= ?", filter.DateFrom)
	}
	if filter.DateTo != "" {
		q = q.Where("e.date_deleted <= ?", filter.DateTo+" 23:59:59")
	}
	q = q.Order("e.date_deleted DESC")

	rows := make([]models.TrashItem, 0)
	if err := q.Scan(&rows).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка получения корзины")
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
		Where("a.entry_date_to IS NULL OR a.entry_date_to::date >= CURRENT_DATE").
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
		Where("a.entry_date_to IS NULL OR a.entry_date_to::date >= CURRENT_DATE").
		Count(&cnt).Error
	if err != nil || cnt == 0 {
		return false, "Нет активной согласованной заявки - восстановление невозможно"
	}
	return true, ""
}

func (s *trashService) RestoreCars(ctx context.Context, systemTableID int, ids []int, userID int) (int, error) {
	restored := 0
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
			return tx.Create(&models.CarHistory{
				CarID:      id,
				UserID:     &userID,
				ActionType: "restore",
				TableID:    &tableID,
			}).Error
		})
		if err == nil {
			restored++
		}
	}
	if restored > 1 {
		tid := systemTableID
		s.db.WithContext(ctx).Create(&models.SystemTableTrashHistory{
			SystemTableID: tid, ActionType: models.TrashActionBulkRestored,
			AffectedCount: restored, UserID: &userID,
		})
	}
	return restored, nil
}

func (s *trashService) RestoreEmployees(ctx context.Context, systemTableID int, ids []int, userID int) (int, error) {
	restored := 0
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
			restored++
		}
	}
	if restored > 1 {
		s.db.WithContext(ctx).Create(&models.SystemTableTrashHistory{
			SystemTableID: systemTableID, ActionType: models.TrashActionBulkRestored,
			AffectedCount: restored, UserID: &userID,
		})
	}
	return restored, nil
}

func (s *trashService) PurgeCar(ctx context.Context, id, userID int) error {
	now := time.Now().UTC()
	res := s.db.WithContext(ctx).Model(&models.Car{}).
		Where("id = ? AND status = 0 AND is_purged = false", id).
		Updates(map[string]any{"is_purged": true, "purged_at": now, "purged_by_user_id": userID})
	if res.Error != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка удаления")
	}
	if res.RowsAffected == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "Запись не в корзине")
	}
	return nil
}

func (s *trashService) PurgeEmployee(ctx context.Context, id, userID int) error {
	now := time.Now().UTC()
	res := s.db.WithContext(ctx).Model(&models.Employee{}).
		Where("id = ? AND status = 0 AND is_purged = false", id).
		Updates(map[string]any{"is_purged": true, "purged_at": now, "purged_by_user_id": userID})
	if res.Error != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка удаления")
	}
	if res.RowsAffected == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "Запись не в корзине")
	}
	return nil
}

// ClearCarsTrash покрывает все cars в корзине этой таблицы (через cars_history.table_id).
func (s *trashService) ClearCarsTrash(ctx context.Context, systemTableID, userID int) (int, error) {
	now := time.Now().UTC()
	// Подзапрос: cars где есть запись 'delete' в cars_history с нужным table_id.
	subq := s.db.Table("cars_history").
		Select("DISTINCT car_id").
		Where("action_type = 'delete' AND table_id = ?", systemTableID)
	res := s.db.WithContext(ctx).Model(&models.Car{}).
		Where("status = 0 AND is_purged = false AND id IN (?)", subq).
		Updates(map[string]any{"is_purged": true, "purged_at": now, "purged_by_user_id": userID})
	if res.Error != nil {
		return 0, echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Ошибка очистки: %v", res.Error))
	}
	if res.RowsAffected > 0 {
		s.db.WithContext(ctx).Create(&models.SystemTableTrashHistory{
			SystemTableID: systemTableID, ActionType: models.TrashActionCleared,
			AffectedCount: int(res.RowsAffected), UserID: &userID,
		})
	}
	return int(res.RowsAffected), nil
}

func (s *trashService) ClearEmployeesTrash(ctx context.Context, systemTableID, userID int) (int, error) {
	now := time.Now().UTC()
	subq := s.db.Table("employees_history").
		Select("DISTINCT employee_id").
		Where("action_type = 'delete' AND table_id = ?", systemTableID)
	res := s.db.WithContext(ctx).Model(&models.Employee{}).
		Where("status = 0 AND is_purged = false AND id IN (?)", subq).
		Updates(map[string]any{"is_purged": true, "purged_at": now, "purged_by_user_id": userID})
	if res.Error != nil {
		return 0, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка очистки")
	}
	if res.RowsAffected > 0 {
		s.db.WithContext(ctx).Create(&models.SystemTableTrashHistory{
			SystemTableID: systemTableID, ActionType: models.TrashActionCleared,
			AffectedCount: int(res.RowsAffected), UserID: &userID,
		})
	}
	return int(res.RowsAffected), nil
}
