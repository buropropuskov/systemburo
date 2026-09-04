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

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// blacklistSimilarityThreshold - порог [0..1] нечёткого совпадения для предупреждения о
// возможном обходе ЧС (#481). Дефолт 0.7 (решение владельца). Общий для машин (Левенштейн
// по номеру) и людей (similarity/word_similarity по ФИО). Может стать настройкой админки
// отдельным срезом - пока фиксированная константа (YAGNI).
const blacklistSimilarityThreshold = 0.7

// VehicleBlacklistService - бизнес-логика чёрного списка автомобилей (#443).
//
// Create/Restore каскадно деактивируют совпадающие cars (status 1 -> 0) в одной
// транзакции; Archive (снятие) восстанавливает status=1 только тем cars, у которых
// есть активная заявка. Все каскады пишут cars_history и историю самой записи.
type VehicleBlacklistService interface {
	GetAll(ctx context.Context, includeArchived bool) ([]models.VehicleBlacklist, error)
	GetByID(ctx context.Context, id int) (*models.VehicleBlacklist, error)
	Create(ctx context.Context, req models.CreateVehicleBlacklistRequest, userID int) (*models.VehicleBlacklist, error)
	// Archive - снятие из чёрного списка (soft-delete): is_active=false + восстановление
	// status=1 у совпадающих cars с активной заявкой.
	Archive(ctx context.Context, id int, userID int) error
	// Restore - повторное добавление архивной записи в список: is_active=true +
	// повторная деактивация совпадающих активных cars.
	Restore(ctx context.Context, id int, userID int) error
	// BulkArchive снимает набор записей из чёрного списка через Archive (полный каскад
	// реактивации cars для каждой). Несуществующие id -> в Errors (частичный успех
	// 207), не валят операцию. Дубли id дедуплицируются.
	BulkArchive(ctx context.Context, ids []int, userID int) (*BulkOpResult, error)
	// BulkRestore возвращает набор записей в чёрный список через Restore.
	BulkRestore(ctx context.Context, ids []int, userID int) (*BulkOpResult, error)
	// Check - заблокирована ли машина (number+mark) активной записью.
	Check(ctx context.Context, carNumber string, markID int) (models.VehicleBlacklistCheckResult, error)
	// Impact - предпросмотр последствий внесения: какие активные машины перестанут
	// действовать, из каких таблиц постов уйдут и в каких заявках фигурируют.
	Impact(ctx context.Context, carNumber string, markID int) (*BlacklistImpact, error)
	// CheckByName - проверка по номеру и имени марки (для машин без mark_id, например
	// выбранных из существующих unique_cars). Совпадение по mark_name, как в каскаде.
	CheckByName(ctx context.Context, carNumber, markName string) (models.VehicleBlacklistCheckResult, error)
	// FindSimilar - активные записи ЧС, чьи нормализованные номера БЛИЗКИ (но не обязательно
	// равны) нормализованному номеру заявки: Левенштейн по normalized_number, порог 0.7.
	// Слой предупреждения о возможном обходе (#481): точное совпадение ловит Check (409),
	// сюда попадают опечатка/гомоглиф/подмена 0<->О. Пустой срез - похожих нет.
	FindSimilar(ctx context.Context, carNumber string) ([]models.BlacklistSimilarMatch, error)
	// Update - правка активной записи (номер/марка/причина) + лог в историю (updated). При
	// смене идентичности перекаскадивает cars: реактивирует совпадавшие со старым номером,
	// деактивирует совпадающие с новым. Дубль активной записи -> 409.
	Update(ctx context.Context, id int, req models.UpdateVehicleBlacklistRequest, userID int) (*models.VehicleBlacklist, error)
	// Purge - удаление архивной записи навсегда: запись удаляется физически, но событие
	// purged (с лейблом машины) остаётся в общем журнале ЧС. Активную удалять нельзя.
	Purge(ctx context.Context, id int, userID int) error
	GetHistory(ctx context.Context, id int) ([]models.VehicleBlacklistHistoryItem, error)
	// GetAllHistory - весь журнал ЧС машин (все события всех записей, включая удалённые).
	GetAllHistory(ctx context.Context) ([]models.VehicleBlacklistHistoryItem, error)
}

type vehicleBlacklistService struct {
	db       *gorm.DB
	recorder AuditRecorder
}

// NewVehicleBlacklistService создаёт реализацию.
func NewVehicleBlacklistService(db *gorm.DB, recorder AuditRecorder) VehicleBlacklistService {
	return &vehicleBlacklistService{db: db, recorder: recorder}
}

func (s *vehicleBlacklistService) GetAll(ctx context.Context, includeArchived bool) ([]models.VehicleBlacklist, error) {
	items := make([]models.VehicleBlacklist, 0)
	q := s.db.WithContext(ctx).Order("created_at DESC")
	if !includeArchived {
		q = q.Where("is_active = ?", true)
	}
	if err := q.Find(&items).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка получения чёрного списка")
	}
	return items, nil
}

func (s *vehicleBlacklistService) GetByID(ctx context.Context, id int) (*models.VehicleBlacklist, error) {
	var e models.VehicleBlacklist
	if err := s.db.WithContext(ctx).First(&e, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, echo.NewHTTPError(http.StatusNotFound, "Запись чёрного списка не найдена")
		}
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка получения записи чёрного списка")
	}
	return &e, nil
}

func (s *vehicleBlacklistService) Create(ctx context.Context, req models.CreateVehicleBlacklistRequest, userID int) (*models.VehicleBlacklist, error) {
	var mark models.Mark
	if err := s.db.WithContext(ctx).First(&mark, req.MarkID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "Марка не найдена")
		}
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка получения марки")
	}

	carNumber := strings.TrimSpace(req.CarNumber)
	entry := models.VehicleBlacklist{
		CarNumber:        carNumber,
		MarkID:           req.MarkID,
		MarkName:         mark.Name,
		Reason:           strings.TrimSpace(req.Reason),
		NormalizedNumber: normalize.Plate(carNumber),
		IsActive:         true,
		CreatedByUserID:  &userID,
	}

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&entry).Error; err != nil {
			return err
		}
		deactivated, err := s.deactivateMatchingCars(ctx, tx, entry, userID)
		if err != nil {
			return err
		}
		details := map[string]interface{}{
			"car_number":       entry.CarNumber,
			"mark_name":        entry.MarkName,
			"reason":           entry.Reason,
			"cars_deactivated": deactivated,
		}
		return s.recorder.Record(ctx, tx, models.AuditEntityVehicleBlacklist, &entry.ID, models.BlacklistActionCreated, &userID, details)
	})
	if err != nil {
		if isUniqueViolation(err) {
			return nil, echo.NewHTTPError(http.StatusConflict, "Эта машина уже в чёрном списке")
		}
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка добавления в чёрный список")
	}
	return &entry, nil
}

func (s *vehicleBlacklistService) Archive(ctx context.Context, id int, userID int) error {
	e, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if !e.IsActive {
		return nil // уже снята - no-op
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.VehicleBlacklist{}).Where("id = ?", e.ID).Update("is_active", false).Error; err != nil {
			return err
		}
		reactivated, err := s.reactivateMatchingCars(ctx, tx, *e, userID)
		if err != nil {
			return err
		}
		details := map[string]interface{}{
			"car_number":       e.CarNumber,
			"mark_name":        e.MarkName,
			"cars_reactivated": reactivated,
		}
		return s.recorder.Record(ctx, tx, models.AuditEntityVehicleBlacklist, &e.ID, models.BlacklistActionArchived, &userID, details)
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка снятия из чёрного списка")
	}
	return nil
}

// BulkArchive снимает набор записей чёрного списка машин через Archive (полный
// каскад реактивации cars для каждой). Несуществующие id -> в Errors (частичный
// успех 207), не валят операцию. Дубли id дедуплицируются.
func (s *vehicleBlacklistService) BulkArchive(ctx context.Context, ids []int, userID int) (*BulkOpResult, error) {
	res := newBulkResult()
	for _, id := range uniqueInts(ids) {
		e, err := s.GetByID(ctx, id)
		if err != nil {
			res.addError(id, "", "Запись чёрного списка не найдена")
			continue
		}
		if err := s.Archive(ctx, id, userID); err != nil {
			res.addError(id, vehicleBlacklistLabel(*e), bulkErrMsg(err))
			continue
		}
		res.SuccessCount++
	}
	return res.finalize(), nil
}

// BulkRestore возвращает набор записей в чёрный список машин через Restore.
func (s *vehicleBlacklistService) BulkRestore(ctx context.Context, ids []int, userID int) (*BulkOpResult, error) {
	res := newBulkResult()
	for _, id := range uniqueInts(ids) {
		e, err := s.GetByID(ctx, id)
		if err != nil {
			res.addError(id, "", "Запись чёрного списка не найдена")
			continue
		}
		if err := s.Restore(ctx, id, userID); err != nil {
			res.addError(id, vehicleBlacklistLabel(*e), bulkErrMsg(err))
			continue
		}
		res.SuccessCount++
	}
	return res.finalize(), nil
}

func (s *vehicleBlacklistService) Update(ctx context.Context, id int, req models.UpdateVehicleBlacklistRequest, userID int) (*models.VehicleBlacklist, error) {
	e, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !e.IsActive {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Нельзя редактировать архивную запись")
	}

	var mark models.Mark
	if err := s.db.WithContext(ctx).First(&mark, req.MarkID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "Марка не найдена")
		}
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка получения марки")
	}

	carNumber := strings.TrimSpace(req.CarNumber)
	newReason := strings.TrimSpace(req.Reason)
	if carNumber == "" {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Номер обязателен")
	}
	if newReason == "" {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Причина обязательна")
	}

	// Идентичность - номер (без учёта регистра/пробелов, как в uidx) либо марка.
	identityChanged := !strings.EqualFold(strings.TrimSpace(e.CarNumber), carNumber) || e.MarkID != req.MarkID
	if !identityChanged && newReason == e.Reason {
		return e, nil // без изменений - не пишем историю
	}

	old := *e
	updated := *e
	updated.CarNumber = carNumber
	updated.MarkID = req.MarkID
	updated.MarkName = mark.Name
	updated.NormalizedNumber = normalize.Plate(carNumber)
	updated.Reason = newReason

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		reactivated, deactivated := 0, 0
		// При смене идентичности cars, привязанные к старому номеру, больше не покрыты этой
		// записью - возвращаем их; новые совпадения гасим. Иначе авто осталось бы ошибочно
		// заблокированным/разблокированным.
		if identityChanged {
			r, err := s.reactivateMatchingCars(ctx, tx, old, userID)
			if err != nil {
				return err
			}
			reactivated = r
		}
		if err := tx.Model(&models.VehicleBlacklist{}).Where("id = ?", e.ID).Updates(map[string]interface{}{
			"car_number":        updated.CarNumber,
			"mark_id":           updated.MarkID,
			"mark_name":         updated.MarkName,
			"normalized_number": updated.NormalizedNumber,
			"reason":            updated.Reason,
		}).Error; err != nil {
			return err
		}
		if identityChanged {
			d, err := s.deactivateMatchingCars(ctx, tx, updated, userID)
			if err != nil {
				return err
			}
			deactivated = d
		}
		details := map[string]interface{}{
			"car_number_old": old.CarNumber,
			"car_number_new": updated.CarNumber,
			"mark_name_old":  old.MarkName,
			"mark_name_new":  updated.MarkName,
			"reason_old":     old.Reason,
			"reason_new":     updated.Reason,
		}
		if identityChanged {
			details["cars_reactivated"] = reactivated
			details["cars_deactivated"] = deactivated
		}
		return s.recorder.Record(ctx, tx, models.AuditEntityVehicleBlacklist, &e.ID, models.BlacklistActionUpdated, &userID, details)
	})
	if err != nil {
		if isUniqueViolation(err) {
			return nil, echo.NewHTTPError(http.StatusConflict, "Эта машина уже в чёрном списке")
		}
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка обновления записи")
	}
	return &updated, nil
}

// Purge удаляет архивную запись навсегда. Сначала логируем purged-событие (с лейблом
// машины в details), затем физически удаляем запись - в одной транзакции. История по
// entity_id остаётся (FK нет), общий журнал ЧС сохраняет весь жизненный цикл записи.
func (s *vehicleBlacklistService) Purge(ctx context.Context, id int, userID int) error {
	e, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if e.IsActive {
		return echo.NewHTTPError(http.StatusBadRequest, "Нельзя удалить навсегда активную запись - сначала уберите из чёрного списка")
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		details := map[string]interface{}{
			"car_number": e.CarNumber,
			"mark_name":  e.MarkName,
			"reason":     e.Reason,
		}
		if err := s.recorder.Record(ctx, tx, models.AuditEntityVehicleBlacklist, &e.ID, models.BlacklistActionPurged, &userID, details); err != nil {
			return err
		}
		return tx.Where("id = ?", e.ID).Delete(&models.VehicleBlacklist{}).Error
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка удаления записи чёрного списка")
	}
	return nil
}

func (s *vehicleBlacklistService) Restore(ctx context.Context, id int, userID int) error {
	e, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if e.IsActive {
		return nil // уже активна - no-op
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.VehicleBlacklist{}).Where("id = ?", e.ID).Update("is_active", true).Error; err != nil {
			return err
		}
		deactivated, err := s.deactivateMatchingCars(ctx, tx, *e, userID)
		if err != nil {
			return err
		}
		details := map[string]interface{}{
			"car_number":       e.CarNumber,
			"mark_name":        e.MarkName,
			"cars_deactivated": deactivated,
		}
		return s.recorder.Record(ctx, tx, models.AuditEntityVehicleBlacklist, &e.ID, models.BlacklistActionRestored, &userID, details)
	})
	if err != nil {
		if isUniqueViolation(err) {
			return echo.NewHTTPError(http.StatusConflict, "Эта машина уже в активном чёрном списке")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка возврата в чёрный список")
	}
	return nil
}

func (s *vehicleBlacklistService) Check(ctx context.Context, carNumber string, markID int) (models.VehicleBlacklistCheckResult, error) {
	var e models.VehicleBlacklist
	err := s.db.WithContext(ctx).
		Where("is_active = ?", true).
		Where("LOWER(TRIM(car_number)) = LOWER(TRIM(?))", carNumber).
		Where("mark_id = ?", markID).
		First(&e).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.VehicleBlacklistCheckResult{IsBlacklisted: false}, nil
		}
		return models.VehicleBlacklistCheckResult{}, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка проверки чёрного списка")
	}
	return models.VehicleBlacklistCheckResult{IsBlacklisted: true, Reason: e.Reason}, nil
}

func (s *vehicleBlacklistService) CheckByName(ctx context.Context, carNumber, markName string) (models.VehicleBlacklistCheckResult, error) {
	// .First: при нескольких активных записях с одним номером+именем марки (разные mark_id
	// после пересоздания марки) берём любую - для блокировки достаточно факта совпадения.
	var e models.VehicleBlacklist
	err := s.db.WithContext(ctx).
		Where("is_active = ?", true).
		Where("LOWER(TRIM(car_number)) = LOWER(TRIM(?))", carNumber).
		Where("LOWER(TRIM(mark_name)) = LOWER(TRIM(?))", markName).
		First(&e).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.VehicleBlacklistCheckResult{IsBlacklisted: false}, nil
		}
		return models.VehicleBlacklistCheckResult{}, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка проверки чёрного списка")
	}
	return models.VehicleBlacklistCheckResult{IsBlacklisted: true, Reason: e.Reason}, nil
}

// FindSimilar ищет активные записи, чей нормализованный номер близок к нормализованному
// номеру заявки. Метрика - 1 - levenshtein/max(len): расстояние редактирования fuzzystrmatch,
// приведённое к [0..1]. Сравниваем по normalized_number (та же канон-форма, что при Create/
// бэкфилле), чтобы гомоглиф/0<->О/пробелы не давали ложного расхождения. Нормализованно-точную
// форму НЕ исключаем: совпадение в нормали при различии в сырых данных (латиница) - и есть
// обход (Check его не сматчит). Результат - по убыванию близости.
func (s *vehicleBlacklistService) FindSimilar(ctx context.Context, carNumber string) ([]models.BlacklistSimilarMatch, error) {
	q := normalize.Plate(carNumber)
	if q == "" {
		return []models.BlacklistSimilarMatch{}, nil
	}
	// levenshtein (fuzzystrmatch) падает на аргументе >255 байт, а FindSimilar зовётся с
	// пользовательским вводом (normalize.Plate длину не ограничивает). Реальные номера
	// короткие - обрезаем по рунам, чтобы мусорный ввод не ронял запрос (64 руны <= 128 байт).
	if r := []rune(q); len(r) > 64 {
		q = string(r[:64])
	}
	type simRow struct {
		ID        int
		CarNumber string
		MarkName  string
		Reason    string
		Sim       float64
	}
	var rows []simRow
	err := s.db.WithContext(ctx).Raw(`
		SELECT id, car_number, mark_name, reason, sim FROM (
			SELECT id, car_number, mark_name, reason,
			       1 - levenshtein(normalized_number, @q)::float
			           / GREATEST(LENGTH(normalized_number), LENGTH(@q), 1) AS sim
			FROM vehicle_blacklists
			WHERE is_active = true AND normalized_number <> ''
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
		matches = append(matches, models.BlacklistSimilarMatch{
			ID:           r.ID,
			Similarity:   r.Sim,
			MatchedValue: strings.TrimSpace(r.CarNumber + " " + r.MarkName),
			Reason:       r.Reason,
		})
	}
	return matches, nil
}

// GetHistory возвращает историю записи ЧС машин (новые сверху).
// Read-switch #870 (F.4): до-cutover строки vehicle_blacklist_histories подняты в
// audit_log разовым backfill'ом (details уже jsonb - перенос verbatim), читаем
// только audit_log. Форму стережёт TestVehicleBlacklist_History_BackfillLegacyIntoAudit.
func (s *vehicleBlacklistService) GetHistory(ctx context.Context, id int) ([]models.VehicleBlacklistHistoryItem, error) {
	return s.queryHistory(ctx, &id)
}

// GetAllHistory возвращает весь журнал ЧС машин (все записи, включая удалённые).
func (s *vehicleBlacklistService) GetAllHistory(ctx context.Context) ([]models.VehicleBlacklistHistoryItem, error) {
	return s.queryHistory(ctx, nil)
}

const vehicleBLActorName = `COALESCE(NULLIF(TRIM(BOTH ' ' FROM CONCAT_WS(' ', u.last_name, u.first_name)), ''), u.username, '')`

func (s *vehicleBlacklistService) queryHistory(ctx context.Context, entityID *int) ([]models.VehicleBlacklistHistoryItem, error) {
	where := "a.entity_type = ?"
	args := []interface{}{models.AuditEntityVehicleBlacklist}
	if entityID != nil {
		where += " AND a.entity_id = ?"
		args = append(args, *entityID)
	}

	query := `
		SELECT a.id, a.entity_id, a.action AS action_type, a.details, a.actor_user_id AS user_id,
			` + vehicleBLActorName + ` AS user_name, a.created_at
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
	items := make([]models.VehicleBlacklistHistoryItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, models.VehicleBlacklistHistoryItem{
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

// deactivateMatchingCars деактивирует (status 1 -> 0) активные cars, совпадающие с записью
// по номеру и марке, и пишет cars_history. Возвращает число затронутых машин.
// Совпадение по марке: c.mark_id ИЛИ имя (mark_name/устаревший car_brand) - чтобы покрыть
// и новые (mark_id), и легаси (только car_brand) машины.
func (s *vehicleBlacklistService) deactivateMatchingCars(ctx context.Context, tx *gorm.DB, e models.VehicleBlacklist, userID int) (int, error) {
	var ids []int
	if err := tx.WithContext(ctx).
		Table("cars c").
		Where("LOWER(TRIM(c.car_number)) = LOWER(TRIM(?))", e.CarNumber).
		Where("(c.mark_id = ? OR (c.mark_id IS NULL AND LOWER(TRIM(COALESCE(c.mark_name, c.car_brand))) = LOWER(TRIM(?))))", e.MarkID, e.MarkName).
		Where("c.status = ?", 1).
		Where("c.is_purged = ?", false).
		Pluck("c.id", &ids).Error; err != nil {
		return 0, err
	}
	now := time.Now().UTC()
	comment := fmt.Sprintf("Автомобиль %s %s добавлен в чёрный список: %s", e.CarNumber, e.MarkName, e.Reason)
	for _, id := range ids {
		if err := tx.Model(&models.Car{}).Where("id = ?", id).
			Updates(map[string]interface{}{"status": 0, "date_removed": now}).Error; err != nil {
			return 0, err
		}
		if err := s.recorder.Record(ctx, tx, models.AuditEntityCar, &id, "blacklisted", &userID, carAuditDetails{Comment: &comment}); err != nil {
			return 0, err
		}
	}
	return len(ids), nil
}

// reactivateMatchingCars восстанавливает status=1 у деактивированных (status=0) cars,
// совпадающих с записью, у которых есть активная заявка (confirmation='Согласовано',
// application.status в рабочих статусах) И не истёк пропуск (дата актуальна). Машины без
// активной заявки или с просроченным пропуском остаются деактивированными. Совпадение по
// марке - как в deactivateMatchingCars (mark_id, для легаси - имя).
func (s *vehicleBlacklistService) reactivateMatchingCars(ctx context.Context, tx *gorm.DB, e models.VehicleBlacklist, userID int) (int, error) {
	var ids []int
	if err := tx.WithContext(ctx).
		Table("cars c").
		Joins("JOIN attachments a ON c.attachment_id = a.id").
		Joins("JOIN applications app ON a.application_id = app.id").
		Where("LOWER(TRIM(c.car_number)) = LOWER(TRIM(?))", e.CarNumber).
		Where("(c.mark_id = ? OR (c.mark_id IS NULL AND LOWER(TRIM(COALESCE(c.mark_name, c.car_brand))) = LOWER(TRIM(?))))", e.MarkID, e.MarkName).
		Where("c.status = ?", 0).
		Where("c.is_purged = ?", false).
		Where("app.confirmation = ?", models.ConfirmationApproved).
		Where("app.status IN ?", []string{models.StatusInWork, models.StatusCompleted}).
		// "дата актуальна" из ТЗ: не возрождаем просроченный пропуск. NULLIF защищает
		// от пустой строки в entry_date_to (иначе ''::date упадёт).
		Where(passValidNowSQL("c")).
		Pluck("c.id", &ids).Error; err != nil {
		return 0, err
	}
	comment := fmt.Sprintf("Автомобиль %s %s снят с чёрного списка", e.CarNumber, e.MarkName)
	for _, id := range ids {
		if err := tx.Model(&models.Car{}).Where("id = ?", id).
			Updates(map[string]interface{}{"status": 1, "date_removed": nil}).Error; err != nil {
			return 0, err
		}
		if err := s.recorder.Record(ctx, tx, models.AuditEntityCar, &id, "unblacklisted", &userID, carAuditDetails{Comment: &comment}); err != nil {
			return 0, err
		}
	}
	return len(ids), nil
}

// vehicleBlacklistLabel собирает "номер марка" для логов/сообщений bulk-ошибок.
func vehicleBlacklistLabel(e models.VehicleBlacklist) string {
	return strings.TrimSpace(e.CarNumber + " " + e.MarkName)
}

// isUniqueViolation распознаёт нарушение partial unique index (повторное добавление
// активной записи) по pg-коду 23505. TranslateError в gorm-конфигах не включён, поэтому
// проверяем сам *pgconn.PgError; gorm.ErrDuplicatedKey и строковый матч - фолбэки на
// случай другого драйвера/обёртки.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	return strings.Contains(err.Error(), "23505") || strings.Contains(err.Error(), "duplicate key value")
}

// Impact - см. VehicleBlacklistService.Impact.
func (s *vehicleBlacklistService) Impact(ctx context.Context, carNumber string, markID int) (*BlacklistImpact, error) {
	return vehicleBlacklistImpact(ctx, s.db, carNumber, markID)
}
