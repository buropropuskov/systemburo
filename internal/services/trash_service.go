package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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
	db                  *gorm.DB
	recorder            AuditRecorder
	notificationService NotificationService
}

// TrashServiceOption конфигурирует trashService при создании.
type TrashServiceOption func(*trashService)

// WithTrashNotifications включает уведомление trash_restored (#1748) автору записи
// при восстановлении из корзины. Опционально: без неё уведомления не шлются
// (тесты, offline).
func WithTrashNotifications(ns NotificationService) TrashServiceOption {
	return func(s *trashService) { s.notificationService = ns }
}

// NewTrashService создаёт сервис корзины.
func NewTrashService(db *gorm.DB, recorder AuditRecorder, opts ...TrashServiceOption) TrashService {
	s := &trashService{db: db, recorder: recorder}
	for _, opt := range opts {
		opt(s)
	}
	return s
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
			(
				SELECT ch.user_id
				FROM ` + carsHistoryUnion + ` ch
				WHERE ch.car_id = c.id AND ch.action_type = 'delete' AND ch.table_id = ?
				ORDER BY ch.created_at DESC LIMIT 1
			) AS deleted_by_id,
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
	// Три подстановки systemTableID: id удалившего, его имя и EXISTS-условие удаления.
	args := []any{systemTableID, systemTableID, systemTableID}

	if filter.Search != "" {
		sql += ` AND (c.car_number ILIKE ? OR COALESCE(c.mark_name, c.car_brand) ILIKE ?)`
		like := "%" + filter.Search + "%"
		args = append(args, like, like)
	}
	if filter.OrganizationID > 0 {
		sql += ` AND a.organization_id = ?`
		args = append(args, filter.OrganizationID)
	}
	// Мультивыбор организаций (#1398). parseIDList принимает указатель - у ApplicationFilter
	// параметр опциональный (*string), здесь поле обычная строка, поэтому берём адрес.
	if ids := parseIDList(&filter.OrganizationIDs); len(ids) > 0 {
		sql += ` AND a.organization_id IN ?`
		args = append(args, ids)
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

	scanned := make([]trashRowWithActor, 0)
	if err := s.db.WithContext(ctx).Raw(sql, args...).Scan(&scanned).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка получения корзины")
	}
	return s.maskDeletedBy(ctx, scanned), nil
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
			(
				SELECT eh.user_id
				FROM ` + employeesHistoryUnion + ` eh
				WHERE eh.employee_id = e.id AND eh.action_type = 'delete' AND eh.table_id = ?
				ORDER BY eh.created_at DESC LIMIT 1
			) AS deleted_by_id,
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
	// Три подстановки systemTableID: id удалившего, его имя и EXISTS-условие удаления.
	args := []any{systemTableID, systemTableID, systemTableID}

	if filter.Search != "" {
		sql += ` AND (e.last_name ILIKE ? OR e.first_name ILIKE ? OR e.middle_name ILIKE ?)`
		like := "%" + filter.Search + "%"
		args = append(args, like, like, like)
	}
	if filter.OrganizationID > 0 {
		sql += ` AND a.organization_id = ?`
		args = append(args, filter.OrganizationID)
	}
	if ids := parseIDList(&filter.OrganizationIDs); len(ids) > 0 {
		sql += ` AND a.organization_id IN ?`
		args = append(args, ids)
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

	scanned := make([]trashRowWithActor, 0)
	if err := s.db.WithContext(ctx).Raw(sql, args...).Scan(&scanned).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка получения корзины")
	}
	rows := s.maskDeletedBy(ctx, scanned)
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
		Where(passValidNowSQL("a")).
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
		Where(passValidNowSQL("a")).
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
		details := s.carDetails(ctx, restoredIDs)
		s.logTrashAction(ctx, systemTableID, models.TrashActionBulkRestored, len(restoredIDs), userID, details)
		s.notifyTrashRestored(ctx, "Машина", restoredIDs, s.carAuthors(ctx, restoredIDs), trashLabelMap(details), userID)
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
			return s.recorder.Record(ctx, tx, models.AuditEntityEmployee, &id, "restore", &userID, carAuditDetails{TableID: &tableID})
		})
		if err == nil {
			restoredIDs = append(restoredIDs, id)
		}
	}
	if len(restoredIDs) >= 1 {
		details := s.employeeDetails(ctx, restoredIDs)
		s.logTrashAction(ctx, systemTableID, models.TrashActionBulkRestored, len(restoredIDs), userID, details)
		s.notifyTrashRestored(ctx, "Сотрудник", restoredIDs, s.employeeAuthors(ctx, restoredIDs), trashLabelMap(details), userID)
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
		return s.recorder.Record(ctx, tx, models.AuditEntityEmployee, &id, "purge", &userID, carAuditDetails{Comment: &comment, TableID: &tableID})
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

// trashRowWithActor - строка корзины вместе с id того, кто удалил запись. В ответе
// его нет, но маскировке ФИО он нужен.
type trashRowWithActor struct {
	models.TrashItem
	DeletedByID *int `gorm:"column:deleted_by_id"`
}

// maskDeletedBy подменяет ФИО удалившего логином, если тот не давал согласия на
// обработку персональных данных.
func (s *trashService) maskDeletedBy(ctx context.Context, scanned []trashRowWithActor) []models.TrashItem {
	masks := loadConsentMasks(ctx, s.db)
	rows := make([]models.TrashItem, 0, len(scanned))
	for _, r := range scanned {
		item := r.TrashItem
		item.DeletedByName = maskName(masks, r.DeletedByID, item.DeletedByName)
		rows = append(rows, item)
	}
	return rows
}

// ListTrashHistory возвращает лог массовых действий с корзиной таблицы.
// Read-switch #870 (F.3): до-cutover строки system_table_trash_histories подняты в
// audit_log разовым backfill'ом (affected_count + items свёрнуты в details в форму
// recorder'а), читаем только audit_log. Форму стережёт TestTrash_History_BackfillLegacyIntoAudit.
func (s *trashService) ListTrashHistory(ctx context.Context, systemTableID int) ([]models.TrashHistoryItem, error) {
	const userName = `COALESCE(format_short_name(u.last_name, u.first_name, u.middle_name), '')`
	sql := `
		SELECT a.id, a.action AS action_type,
			COALESCE((a.details->>'affected_count')::int, 0) AS affected_count,
			COALESCE(a.details->'items', '[]'::jsonb) AS details,
			` + userName + ` AS user_name,
			a.actor_user_id AS actor_user_id,
			a.created_at
		FROM audit_log a LEFT JOIN users u ON u.id = a.actor_user_id
		WHERE a.entity_type = ? AND a.entity_id = ?
		ORDER BY a.created_at DESC, a.id DESC`
	// Актора читаем отдельным полем: в ответе его нет, а маскировке ФИО он нужен.
	type trashRow struct {
		models.TrashHistoryItem
		ActorUserID *int `gorm:"column:actor_user_id"`
	}
	var scanned []trashRow
	if err := s.db.WithContext(ctx).Raw(sql, models.AuditEntitySystemTableTrash, systemTableID).Scan(&scanned).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка получения истории корзины")
	}
	masks := loadConsentMasks(ctx, s.db)
	rows := make([]models.TrashHistoryItem, 0, len(scanned))
	for _, r := range scanned {
		item := r.TrashHistoryItem
		item.UserName = maskName(masks, r.ActorUserID, item.UserName)
		rows = append(rows, item)
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

// carAuthors возвращает car.id -> sender_user_id заявки, к которой прикреплена
// машина. Ни Car, ни Employee не хранят собственного "автора" - в терминах
// уведомления "восстановлено из корзины" им считается заявитель заявки, откуда
// запись пришла (Car.AttachmentID -> Attachment.ApplicationID -> sender_user_id).
func (s *trashService) carAuthors(ctx context.Context, ids []int) map[int]int {
	if len(ids) == 0 {
		return nil
	}
	type row struct {
		ID           int
		SenderUserID int
	}
	rows := make([]row, 0, len(ids))
	// Ошибку запроса нельзя проглатывать: пустой результат тут означает штатное
	// "у записи нет автора", и сбой базы стал бы неотличим от него - уведомление
	// молча не ушло бы никому.
	if err := s.db.WithContext(ctx).Table("cars c").
		Select("c.id AS id, app.sender_user_id AS sender_user_id").
		Joins("JOIN attachments att ON att.id = c.attachment_id").
		Joins("JOIN applications app ON app.id = att.application_id").
		Where("c.id IN ?", ids).
		Scan(&rows).Error; err != nil {
		slog.Warn("корзина: не удалось определить авторов машин для уведомления", "error", err)
		return nil
	}
	out := make(map[int]int, len(rows))
	for _, r := range rows {
		out[r.ID] = r.SenderUserID
	}
	return out
}

// employeeAuthors - то же для сотрудников. Employee.AttachmentID nullable - у
// записей без вложения (не должно случаться для того, что вообще попало в
// корзину, но на всякий случай) автора просто не найдётся, и notifyTrashRestored
// молча пропустит уведомление.
func (s *trashService) employeeAuthors(ctx context.Context, ids []int) map[int]int {
	if len(ids) == 0 {
		return nil
	}
	type row struct {
		ID           int
		SenderUserID int
	}
	rows := make([]row, 0, len(ids))
	// Ошибку не проглатываем - см. комментарий в carAuthors.
	if err := s.db.WithContext(ctx).Table("employees e").
		Select("e.id AS id, app.sender_user_id AS sender_user_id").
		Joins("JOIN attachments att ON att.id = e.attachment_id").
		Joins("JOIN applications app ON app.id = att.application_id").
		Where("e.id IN ?", ids).
		Scan(&rows).Error; err != nil {
		slog.Warn("корзина: не удалось определить авторов сотрудников для уведомления", "error", err)
		return nil
	}
	out := make(map[int]int, len(rows))
	for _, r := range rows {
		out[r.ID] = r.SenderUserID
	}
	return out
}

// trashLabelMap превращает [{id,label}] в map для точечного поиска label по id -
// notifyTrashRestored формирует текст уведомления на восстановленную запись.
func trashLabelMap(details []models.TrashDetail) map[int]string {
	out := make(map[int]string, len(details))
	for _, d := range details {
		out[d.ID] = d.Label
	}
	return out
}

// notifyTrashRestored сообщает автору восстановленной записи (заявителю, к чьей
// заявке она относилась), что запись вернули из корзины. Само восстановление -
// автору не шлём: он и так знает, что восстановил. Автора не нашлось (authors[id]
// отсутствует) - тоже молча пропускаем, это не ошибка (запись без вложения/заявки).
func (s *trashService) notifyTrashRestored(ctx context.Context, kind string, ids []int, authors map[int]int, labels map[int]string, actorUserID int) {
	if s.notificationService == nil {
		return
	}
	title := "Запись восстановлена из корзины"
	for _, id := range ids {
		authorID, ok := authors[id]
		if !ok || authorID == actorUserID {
			continue
		}
		label := labels[id]
		if label == "" {
			label = kind
		}
		body := fmt.Sprintf("%s «%s» восстановили из корзины.", kind, label)
		if err := s.notificationService.CreateForUser(ctx, authorID, NotificationTypeTrashRestored, title, body, nil); err != nil {
			slog.Warn("не удалось уведомить о восстановлении записи из корзины",
				"entity", kind, "entity_id", id, "user_id", authorID, "error", err)
		}
	}
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
