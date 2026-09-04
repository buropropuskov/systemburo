package services

import (
	"context"
	"encoding/json"
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
	// BulkArchive снимает набор записей из чёрного списка через Archive (полный каскад
	// реактивации employees для каждой). Несуществующие id -> в Errors (частичный успех
	// 207), не валят операцию. Дубли id дедуплицируются.
	BulkArchive(ctx context.Context, ids []int, userID int) (*BulkOpResult, error)
	// BulkRestore возвращает набор записей в чёрный список через Restore.
	BulkRestore(ctx context.Context, ids []int, userID int) (*BulkOpResult, error)
	Check(ctx context.Context, lastName, firstName, middleName string) (models.PersonBlacklistCheckResult, error)
	// Impact - предпросмотр последствий внесения: сколько активных работников
	// перестанет действовать, из каких таблиц постов они уйдут и в каких заявках есть.
	Impact(ctx context.Context, lastName, firstName, middleName string) (*BlacklistImpact, error)
	// FindSimilar - активные записи ЧС, чьё нормализованное ФИО БЛИЗКО (но не обязательно
	// равно) нормализованному ФИО заявки: триграммная similarity + word_similarity (учёт
	// отсутствия отчества), порог 0.7. Слой предупреждения о возможном обходе (#481): точное
	// совпадение ловит Check (409). Пустой срез - похожих нет.
	FindSimilar(ctx context.Context, lastName, firstName, middleName string) ([]models.BlacklistSimilarMatch, error)
	// Update - правка активной записи (ФИО/причина) + лог в историю (updated). При смене
	// ФИО перекаскадивает employees: реактивирует совпадавших со старым ФИО, деактивирует
	// совпадающих с новым. Дубль активной записи -> 409.
	Update(ctx context.Context, id int, req models.UpdatePersonBlacklistRequest, userID int) (*models.PersonBlacklist, error)
	// Purge - удаление архивной записи навсегда: запись удаляется физически, но событие
	// purged (с ФИО) остаётся в общем журнале ЧС. Активную удалять нельзя.
	Purge(ctx context.Context, id int, userID int) error
	GetHistory(ctx context.Context, id int) ([]models.PersonBlacklistHistoryItem, error)
	// GetAllHistory - весь журнал ЧС людей (все события всех записей, включая удалённые).
	GetAllHistory(ctx context.Context) ([]models.PersonBlacklistHistoryItem, error)
}

type personBlacklistService struct {
	db       *gorm.DB
	recorder AuditRecorder
}

// NewPersonBlacklistService создаёт реализацию.
func NewPersonBlacklistService(db *gorm.DB, recorder AuditRecorder) PersonBlacklistService {
	return &personBlacklistService{db: db, recorder: recorder}
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
		return s.recorder.Record(ctx, tx, models.AuditEntityPersonBlacklist, &entry.ID, models.BlacklistActionCreated, &userID, details)
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
		return s.recorder.Record(ctx, tx, models.AuditEntityPersonBlacklist, &e.ID, models.BlacklistActionArchived, &userID, details)
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
		return s.recorder.Record(ctx, tx, models.AuditEntityPersonBlacklist, &e.ID, models.BlacklistActionRestored, &userID, details)
	})
	if err != nil {
		if isUniqueViolation(err) {
			return echo.NewHTTPError(http.StatusConflict, "Этот человек уже в активном чёрном списке")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка возврата в чёрный список")
	}
	return nil
}

// BulkArchive снимает набор записей чёрного списка людей через Archive (полный
// каскад реактивации employees для каждой). Несуществующие id -> в Errors
// (частичный успех 207), не валят операцию. Дубли id дедуплицируются.
func (s *personBlacklistService) BulkArchive(ctx context.Context, ids []int, userID int) (*BulkOpResult, error) {
	res := newBulkResult()
	for _, id := range uniqueInts(ids) {
		e, err := s.GetByID(ctx, id)
		if err != nil {
			res.addError(id, "", "Запись чёрного списка не найдена")
			continue
		}
		if err := s.Archive(ctx, id, userID); err != nil {
			res.addError(id, personFullName(*e), bulkErrMsg(err))
			continue
		}
		res.SuccessCount++
	}
	return res.finalize(), nil
}

// BulkRestore возвращает набор записей в чёрный список людей через Restore.
func (s *personBlacklistService) BulkRestore(ctx context.Context, ids []int, userID int) (*BulkOpResult, error) {
	res := newBulkResult()
	for _, id := range uniqueInts(ids) {
		e, err := s.GetByID(ctx, id)
		if err != nil {
			res.addError(id, "", "Запись чёрного списка не найдена")
			continue
		}
		if err := s.Restore(ctx, id, userID); err != nil {
			res.addError(id, personFullName(*e), bulkErrMsg(err))
			continue
		}
		res.SuccessCount++
	}
	return res.finalize(), nil
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

func (s *personBlacklistService) Update(ctx context.Context, id int, req models.UpdatePersonBlacklistRequest, userID int) (*models.PersonBlacklist, error) {
	e, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !e.IsActive {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Нельзя редактировать архивную запись")
	}
	lastName := strings.TrimSpace(req.LastName)
	firstName := strings.TrimSpace(req.FirstName)
	middleName := strings.TrimSpace(req.MiddleName)
	newReason := strings.TrimSpace(req.Reason)
	if lastName == "" || firstName == "" {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Фамилия и имя обязательны")
	}
	if newReason == "" {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Причина обязательна")
	}

	oldMiddle := ""
	if e.MiddleName != nil {
		oldMiddle = *e.MiddleName
	}
	identityChanged := !strings.EqualFold(strings.TrimSpace(e.LastName), lastName) ||
		!strings.EqualFold(strings.TrimSpace(e.FirstName), firstName) ||
		!strings.EqualFold(strings.TrimSpace(oldMiddle), middleName)
	if !identityChanged && newReason == e.Reason {
		return e, nil // без изменений - не пишем историю
	}

	old := *e
	updated := *e
	updated.LastName = lastName
	updated.FirstName = firstName
	updated.MiddleName = normalizeMiddleName(req.MiddleName)
	updated.NormalizedFIO = normalize.Name(lastName, firstName, middleName)
	updated.Reason = newReason

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		reactivated, deactivated := 0, 0
		// При смене ФИО employees, привязанные к старому ФИО, больше не покрыты этой записью -
		// возвращаем их; новые совпадения гасим.
		if identityChanged {
			r, err := s.reactivateMatchingEmployees(ctx, tx, old, userID)
			if err != nil {
				return err
			}
			reactivated = r
		}
		if err := tx.Model(&models.PersonBlacklist{}).Where("id = ?", e.ID).Updates(map[string]interface{}{
			"last_name":      updated.LastName,
			"first_name":     updated.FirstName,
			"middle_name":    updated.MiddleName,
			"normalized_fio": updated.NormalizedFIO,
			"reason":         updated.Reason,
		}).Error; err != nil {
			return err
		}
		if identityChanged {
			d, err := s.deactivateMatchingEmployees(ctx, tx, updated, userID)
			if err != nil {
				return err
			}
			deactivated = d
		}
		details := map[string]interface{}{
			"full_name_old": personFullName(old),
			"full_name_new": personFullName(updated),
			"reason_old":    old.Reason,
			"reason_new":    updated.Reason,
		}
		if identityChanged {
			details["employees_reactivated"] = reactivated
			details["employees_deactivated"] = deactivated
		}
		return s.recorder.Record(ctx, tx, models.AuditEntityPersonBlacklist, &e.ID, models.BlacklistActionUpdated, &userID, details)
	})
	if err != nil {
		if isUniqueViolation(err) {
			return nil, echo.NewHTTPError(http.StatusConflict, "Этот человек уже в чёрном списке")
		}
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка обновления записи")
	}
	return &updated, nil
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
		if err := s.recorder.Record(ctx, tx, models.AuditEntityPersonBlacklist, &e.ID, models.BlacklistActionPurged, &userID, details); err != nil {
			return err
		}
		return tx.Where("id = ?", e.ID).Delete(&models.PersonBlacklist{}).Error
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка удаления записи чёрного списка")
	}
	return nil
}

// GetHistory возвращает историю записи ЧС людей (новые сверху).
// Read-switch #870 (F.4): до-cutover строки person_blacklist_histories подняты в
// audit_log разовым backfill'ом (details уже jsonb - перенос verbatim), читаем
// только audit_log. Форму стережёт TestPersonBlacklist_History_BackfillLegacyIntoAudit.
func (s *personBlacklistService) GetHistory(ctx context.Context, id int) ([]models.PersonBlacklistHistoryItem, error) {
	return s.queryHistory(ctx, &id)
}

// GetAllHistory возвращает весь журнал ЧС людей (все записи, включая удалённые).
func (s *personBlacklistService) GetAllHistory(ctx context.Context) ([]models.PersonBlacklistHistoryItem, error) {
	return s.queryHistory(ctx, nil)
}

const personBLActorName = `COALESCE(NULLIF(TRIM(BOTH ' ' FROM CONCAT_WS(' ', u.last_name, u.first_name)), ''), u.username, '')`

func (s *personBlacklistService) queryHistory(ctx context.Context, entityID *int) ([]models.PersonBlacklistHistoryItem, error) {
	where := "a.entity_type = ?"
	args := []interface{}{models.AuditEntityPersonBlacklist}
	if entityID != nil {
		where += " AND a.entity_id = ?"
		args = append(args, *entityID)
	}

	query := `
		SELECT a.id, a.entity_id, a.action AS action_type, a.details, a.actor_user_id AS user_id,
			` + personBLActorName + ` AS user_name, a.created_at
		FROM audit_log a LEFT JOIN users u ON u.id = a.actor_user_id
		WHERE ` + where + `
		ORDER BY a.created_at DESC, a.id DESC`

	type row struct {
		ID         int             `gorm:"column:id"`
		EntityID   int             `gorm:"column:entity_id"`
		ActionType string          `gorm:"column:action_type"`
		Details    json.RawMessage `gorm:"column:details"`
		UserID     *int            `gorm:"column:user_id"`
		UserName   string          `gorm:"column:user_name"`
		CreatedAt  time.Time       `gorm:"column:created_at"`
	}
	var rows []row
	if err := s.db.WithContext(ctx).Raw(query, args...).Scan(&rows).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка получения истории чёрного списка")
	}
	// Логин вместо ФИО у акторов, не давших согласия на обработку данных.
	masks := loadConsentMasks(ctx, s.db)
	items := make([]models.PersonBlacklistHistoryItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, models.PersonBlacklistHistoryItem{
			ID:         r.ID,
			EntityID:   r.EntityID,
			ActionType: r.ActionType,
			Details:    r.Details,
			UserID:     r.UserID,
			UserName:   maskName(masks, r.UserID, r.UserName),
			CreatedAt:  r.CreatedAt,
		})
	}
	return items, nil
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
		if err := s.recorder.Record(ctx, tx, models.AuditEntityEmployee, &id, "blacklisted", &userID, carAuditDetails{Comment: &comment}); err != nil {
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
		Where(passValidNowSQL("a")).
		Pluck("emp.id", &ids).Error; err != nil {
		return 0, err
	}
	comment := fmt.Sprintf("Сотрудник %s снят с чёрного списка", personFullName(e))
	for _, id := range ids {
		if err := tx.Model(&models.Employee{}).Where("id = ?", id).
			Updates(map[string]interface{}{"status": 1, "date_deleted": nil}).Error; err != nil {
			return 0, err
		}
		if err := s.recorder.Record(ctx, tx, models.AuditEntityEmployee, &id, "unblacklisted", &userID, carAuditDetails{Comment: &comment}); err != nil {
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

// Impact - см. PersonBlacklistService.Impact.
func (s *personBlacklistService) Impact(ctx context.Context, lastName, firstName, middleName string) (*BlacklistImpact, error) {
	return personBlacklistImpact(ctx, s.db, lastName, firstName, middleName)
}
