package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/normalize"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// PersonBlacklistService - бизнес-логика чёрного списка людей (#443).
//
// Зеркало VehicleBlacklistService: каскад в транзакции деактивирует/восстанавливает
// совпадающих employees. Совпадение строгое по ФИО (фамилия+имя+отчество), отсутствие
// отчества (NULL/пусто) приводится через COALESCE к пустой строке.
type PersonBlacklistService interface {
	GetAll(ctx context.Context, includeArchived bool) ([]models.PersonBlacklist, error)
	GetByID(ctx context.Context, id int) (*models.PersonBlacklist, error)
	Create(ctx context.Context, req models.CreatePersonBlacklistRequest, userID int) (*models.PersonBlacklist, error)
	Archive(ctx context.Context, id int, userID int) error
	Restore(ctx context.Context, id int, userID int) error
	Check(ctx context.Context, lastName, firstName, middleName string) (models.PersonBlacklistCheckResult, error)
	// FindSimilar - активные записи ЧС, чьё нормализованное ФИО БЛИЗКО (но не обязательно
	// равно) нормализованному ФИО заявки: триграммная similarity + word_similarity (учёт
	// отсутствия отчества), порог 0.7. Слой предупреждения о возможном обходе (#481): точное
	// совпадение ловит Check (409). Пустой срез - похожих нет.
	FindSimilar(ctx context.Context, lastName, firstName, middleName string) ([]models.BlacklistSimilarMatch, error)
	// UpdateReason - редактирование причины записи + лог в историю (updated).
	UpdateReason(ctx context.Context, id int, reason string, userID int) (*models.PersonBlacklist, error)
	// Purge - удаление архивной записи навсегда: запись удаляется физически, но событие
	// purged (с ФИО) остаётся в общем журнале ЧС. Активную удалять нельзя.
	Purge(ctx context.Context, id int, userID int) error
	GetHistory(ctx context.Context, id int) ([]models.PersonBlacklistHistoryItem, error)
	// GetAllHistory - весь журнал ЧС людей (все события всех записей, включая удалённые).
	GetAllHistory(ctx context.Context) ([]models.PersonBlacklistHistoryItem, error)
}

type personBlacklistService struct {
	db      *gorm.DB
	history PersonBlacklistHistoryService
}

// NewPersonBlacklistService создаёт реализацию.
func NewPersonBlacklistService(db *gorm.DB, history PersonBlacklistHistoryService) PersonBlacklistService {
	return &personBlacklistService{db: db, history: history}
}

func (s *personBlacklistService) GetAll(ctx context.Context, includeArchived bool) ([]models.PersonBlacklist, error) {
	items := make([]models.PersonBlacklist, 0)
	q := s.db.WithContext(ctx).Order("created_at DESC")
	if !includeArchived {
		q = q.Where("is_active = ?", true)
	}
	if err := q.Find(&items).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка получения чёрного списка")
	}
	return items, nil
}

func (s *personBlacklistService) GetByID(ctx context.Context, id int) (*models.PersonBlacklist, error) {
	var e models.PersonBlacklist
	if err := s.db.WithContext(ctx).First(&e, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, echo.NewHTTPError(http.StatusNotFound, "Запись чёрного списка не найдена")
		}
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка получения записи чёрного списка")
	}
	return &e, nil
}

func (s *personBlacklistService) Create(ctx context.Context, req models.CreatePersonBlacklistRequest, userID int) (*models.PersonBlacklist, error) {
	lastName := strings.TrimSpace(req.LastName)
	firstName := strings.TrimSpace(req.FirstName)
	middleName := strings.TrimSpace(req.MiddleName)
	entry := models.PersonBlacklist{
		LastName:        lastName,
		FirstName:       firstName,
		MiddleName:      normalizeMiddleName(req.MiddleName),
		Reason:          strings.TrimSpace(req.Reason),
		NormalizedFIO:   normalize.Name(lastName, firstName, middleName),
		IsActive:        true,
		CreatedByUserID: &userID,
	}

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&entry).Error; err != nil {
			return err
		}
		deactivated, err := s.deactivateMatchingEmployees(ctx, tx, entry, userID)
		if err != nil {
			return err
		}
		details := map[string]interface{}{
			"full_name":             personFullName(entry),
			"reason":                entry.Reason,
			"employees_deactivated": deactivated,
		}
		return s.history.Log(ctx, tx, entry.ID, &userID, models.BlacklistActionCreated, details)
	})
	if err != nil {
		if isUniqueViolation(err) {
			return nil, echo.NewHTTPError(http.StatusConflict, "Этот человек уже в чёрном списке")
		}
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка добавления в чёрный список")
	}
	return &entry, nil
}

func (s *personBlacklistService) Archive(ctx context.Context, id int, userID int) error {
	e, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if !e.IsActive {
		return nil
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.PersonBlacklist{}).Where("id = ?", e.ID).Update("is_active", false).Error; err != nil {
			return err
		}
		reactivated, err := s.reactivateMatchingEmployees(ctx, tx, *e, userID)
		if err != nil {
			return err
		}
		details := map[string]interface{}{
			"full_name":             personFullName(*e),
			"employees_reactivated": reactivated,
		}
		return s.history.Log(ctx, tx, e.ID, &userID, models.BlacklistActionArchived, details)
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка снятия из чёрного списка")
	}
	return nil
}

func (s *personBlacklistService) Restore(ctx context.Context, id int, userID int) error {
	e, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if e.IsActive {
		return nil
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.PersonBlacklist{}).Where("id = ?", e.ID).Update("is_active", true).Error; err != nil {
			return err
		}
		deactivated, err := s.deactivateMatchingEmployees(ctx, tx, *e, userID)
		if err != nil {
			return err
		}
		details := map[string]interface{}{
			"full_name":             personFullName(*e),
			"employees_deactivated": deactivated,
		}
		return s.history.Log(ctx, tx, e.ID, &userID, models.BlacklistActionRestored, details)
	})
	if err != nil {
		if isUniqueViolation(err) {
			return echo.NewHTTPError(http.StatusConflict, "Этот человек уже в активном чёрном списке")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка возврата в чёрный список")
	}
	return nil
}

func (s *personBlacklistService) Check(ctx context.Context, lastName, firstName, middleName string) (models.PersonBlacklistCheckResult, error) {
	var e models.PersonBlacklist
	err := s.db.WithContext(ctx).
		Where("is_active = ?", true).
		Where("LOWER(TRIM(last_name)) = LOWER(TRIM(?))", lastName).
		Where("LOWER(TRIM(first_name)) = LOWER(TRIM(?))", firstName).
		Where("LOWER(TRIM(COALESCE(middle_name, ''))) = LOWER(TRIM(?))", middleName).
		First(&e).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.PersonBlacklistCheckResult{IsBlacklisted: false}, nil
		}
		return models.PersonBlacklistCheckResult{}, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка проверки чёрного списка")
	}
	return models.PersonBlacklistCheckResult{IsBlacklisted: true, Reason: e.Reason}, nil
}

// FindSimilar ищет активные записи, чьё нормализованное ФИО близко к нормализованному ФИО
// заявки. Метрика - GREATEST из триграммной similarity и word_similarity в обе стороны:
//   - similarity ловит опечатку/гомоглиф (нормализованные формы почти равны);
//   - word_similarity(@q, normalized_fio) ловит "без отчества" в заявке (запрос - подстрока эталона);
//   - word_similarity(normalized_fio, @q) ловит обратное (отчество добавлено в заявке).
//
// Сравниваем по normalized_fio (канон-форма из Create/бэкфилла), порог 0.7. Нормализованно-
// точную форму не исключаем: совпадение в нормали при различии в сырых данных (латиница) -
// это и есть обход (Check его не сматчит). Результат - по убыванию близости.
func (s *personBlacklistService) FindSimilar(ctx context.Context, lastName, firstName, middleName string) ([]models.BlacklistSimilarMatch, error) {
	q := normalize.Name(lastName, firstName, middleName)
	if q == "" {
		return []models.BlacklistSimilarMatch{}, nil
	}
	type simRow struct {
		ID         int
		LastName   string
		FirstName  string
		MiddleName *string
		Reason     string
		Sim        float64
	}
	var rows []simRow
	err := s.db.WithContext(ctx).Raw(`
		SELECT id, last_name, first_name, middle_name, reason, sim FROM (
			SELECT id, last_name, first_name, middle_name, reason,
			       GREATEST(
			         similarity(normalized_fio, @q),
			         word_similarity(@q, normalized_fio),
			         word_similarity(normalized_fio, @q)
			       ) AS sim
			FROM person_blacklists
			WHERE is_active = true AND normalized_fio <> ''
		) t
		WHERE sim >= @threshold
		ORDER BY sim DESC, id`,
		map[string]interface{}{"q": q, "threshold": blacklistSimilarityThreshold},
	).Scan(&rows).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка поиска похожих записей чёрного списка")
	}
	matches := make([]models.BlacklistSimilarMatch, 0, len(rows))
	for _, r := range rows {
		middle := ""
		if r.MiddleName != nil {
			middle = *r.MiddleName
		}
		label := strings.Join(strings.Fields(r.LastName+" "+r.FirstName+" "+middle), " ")
		matches = append(matches, models.BlacklistSimilarMatch{
			ID:           r.ID,
			Similarity:   r.Sim,
			MatchedValue: label,
			Reason:       r.Reason,
		})
	}
	return matches, nil
}

func (s *personBlacklistService) UpdateReason(ctx context.Context, id int, reason string, userID int) (*models.PersonBlacklist, error) {
	e, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !e.IsActive {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Нельзя редактировать причину архивной записи")
	}
	newReason := strings.TrimSpace(reason)
	if newReason == "" {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Причина обязательна")
	}
	if newReason == e.Reason {
		return e, nil // без изменений - не пишем историю
	}
	oldReason := e.Reason
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.PersonBlacklist{}).Where("id = ?", e.ID).Update("reason", newReason).Error; err != nil {
			return err
		}
		details := map[string]interface{}{
			"full_name":  personFullName(*e),
			"reason_old": oldReason,
			"reason_new": newReason,
		}
		return s.history.Log(ctx, tx, e.ID, &userID, models.BlacklistActionUpdated, details)
	})
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка обновления причины")
	}
	e.Reason = newReason
	return e, nil
}

// Purge удаляет архивную запись навсегда. Логируем purged-событие (с ФИО в details),
// затем физически удаляем запись - в одной транзакции. История по entity_id остаётся.
func (s *personBlacklistService) Purge(ctx context.Context, id int, userID int) error {
	e, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if e.IsActive {
		return echo.NewHTTPError(http.StatusBadRequest, "Нельзя удалить навсегда активную запись - сначала уберите из чёрного списка")
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		details := map[string]interface{}{
			"full_name": personFullName(*e),
			"reason":    e.Reason,
		}
		if err := s.history.Log(ctx, tx, e.ID, &userID, models.BlacklistActionPurged, details); err != nil {
			return err
		}
		return tx.Where("id = ?", e.ID).Delete(&models.PersonBlacklist{}).Error
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка удаления записи чёрного списка")
	}
	return nil
}

func (s *personBlacklistService) GetHistory(ctx context.Context, id int) ([]models.PersonBlacklistHistoryItem, error) {
	return s.history.GetHistory(ctx, id)
}

func (s *personBlacklistService) GetAllHistory(ctx context.Context) ([]models.PersonBlacklistHistoryItem, error) {
	return s.history.GetAllHistory(ctx)
}

// deactivateMatchingEmployees гасит (status 1 -> 0, date_deleted) активных employees,
// совпадающих по ФИО, и пишет employees_history. Возвращает число затронутых.
func (s *personBlacklistService) deactivateMatchingEmployees(ctx context.Context, tx *gorm.DB, e models.PersonBlacklist, userID int) (int, error) {
	var ids []int
	if err := tx.WithContext(ctx).
		Table("employees").
		Where("LOWER(TRIM(last_name)) = LOWER(TRIM(?))", e.LastName).
		Where("LOWER(TRIM(first_name)) = LOWER(TRIM(?))", e.FirstName).
		Where("LOWER(TRIM(COALESCE(middle_name, ''))) = LOWER(TRIM(COALESCE(?, '')))", e.MiddleName).
		Where("status = ?", 1).
		Where("is_purged = ?", false).
		Pluck("id", &ids).Error; err != nil {
		return 0, err
	}
	now := time.Now().UTC()
	comment := fmt.Sprintf("Сотрудник %s добавлен в чёрный список: %s", personFullName(e), e.Reason)
	for _, id := range ids {
		if err := tx.Model(&models.Employee{}).Where("id = ?", id).
			Updates(map[string]interface{}{"status": 0, "date_deleted": now}).Error; err != nil {
			return 0, err
		}
		hist := models.EmployeeHistory{EmployeeID: id, UserID: &userID, ActionType: "blacklisted", Comment: &comment, CreatedAt: now}
		if err := tx.Create(&hist).Error; err != nil {
			return 0, err
		}
	}
	return len(ids), nil
}

// reactivateMatchingEmployees восстанавливает status=1 у деактивированных employees,
// совпадающих по ФИО, с активной заявкой (Согласовано + рабочий статус) и актуальной
// датой пропуска (attachments.entry_date_to - у employees своего поля даты нет).
func (s *personBlacklistService) reactivateMatchingEmployees(ctx context.Context, tx *gorm.DB, e models.PersonBlacklist, userID int) (int, error) {
	var ids []int
	if err := tx.WithContext(ctx).
		Table("employees emp").
		Joins("JOIN attachments a ON emp.attachment_id = a.id").
		Joins("JOIN applications app ON a.application_id = app.id").
		Where("LOWER(TRIM(emp.last_name)) = LOWER(TRIM(?))", e.LastName).
		Where("LOWER(TRIM(emp.first_name)) = LOWER(TRIM(?))", e.FirstName).
		Where("LOWER(TRIM(COALESCE(emp.middle_name, ''))) = LOWER(TRIM(COALESCE(?, '')))", e.MiddleName).
		Where("emp.status = ?", 0).
		Where("emp.is_purged = ?", false).
		Where("app.confirmation = ?", models.ConfirmationApproved).
		Where("app.status IN ?", []string{models.StatusInWork, models.StatusCompleted}).
		Where("(NULLIF(TRIM(a.entry_date_to), '') IS NULL OR (NULLIF(TRIM(a.entry_date_to), ''))::date >= CURRENT_DATE)").
		Pluck("emp.id", &ids).Error; err != nil {
		return 0, err
	}
	now := time.Now().UTC()
	comment := fmt.Sprintf("Сотрудник %s снят с чёрного списка", personFullName(e))
	for _, id := range ids {
		if err := tx.Model(&models.Employee{}).Where("id = ?", id).
			Updates(map[string]interface{}{"status": 1, "date_deleted": nil}).Error; err != nil {
			return 0, err
		}
		hist := models.EmployeeHistory{EmployeeID: id, UserID: &userID, ActionType: "unblacklisted", Comment: &comment, CreatedAt: now}
		if err := tx.Create(&hist).Error; err != nil {
			return 0, err
		}
	}
	return len(ids), nil
}

// normalizeMiddleName приводит отчество к *string: пустое/пробелы -> nil (нет отчества).
func normalizeMiddleName(s string) *string {
	t := strings.TrimSpace(s)
	if t == "" {
		return nil
	}
	return &t
}

// personFullName собирает ФИО для логов/деталей.
func personFullName(e models.PersonBlacklist) string {
	fio := strings.TrimSpace(e.LastName + " " + e.FirstName)
	if e.MiddleName != nil && strings.TrimSpace(*e.MiddleName) != "" {
		fio += " " + strings.TrimSpace(*e.MiddleName)
	}
	return fio
}
