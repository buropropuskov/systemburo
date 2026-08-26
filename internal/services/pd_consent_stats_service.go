package services

import (
	"context"

	"systemburo/internal/models"

	"gorm.io/gorm"
)

// PDConsentStatsService считает, как идёт сбор согласий на обработку ПД (#1567):
// сколько человек подтвердили текущую редакцию и кто ещё нет.
//
// Мерка ровно та же, что у гейта (middleware.PDConsentGate + PDConsentGateService):
// в знаменателе только те, кого гейт реально закрывает, а согласившимся считается
// тот, чья действующая редакция не ниже требуемой. Считай мы иначе - сводка
// разошлась бы с тем, кого система пускает, и администратор принимал бы решения по
// числу, которому нельзя верить.
type PDConsentStatsService struct {
	db *gorm.DB
	// gate -- источник правды о том, работает ли запрос согласия и какой редакции.
	// Повторять его формулу здесь нельзя: два места разъедутся, и сводка начнёт
	// расходиться с тем, кого система реально закрывает.
	gate *PDConsentGateService
}

// NewPDConsentStatsService создаёт сервис сводки по сбору согласий.
func NewPDConsentStatsService(db *gorm.DB, gate *PDConsentGateService) *PDConsentStatsService {
	return &PDConsentStatsService{db: db, gate: gate}
}

// gatedUsersWhere -- кого гейт согласия реально закрывает.
//
// Супер-администратор исключён им самим (аварийная дверь). Архивные и заблокированные
// не работают в системе вовсе: архивного и забаненного раньше гейта отбивает проверка
// блокировки, и держать их в знаменателе значит вечно не досчитываться до 100%.
const gatedUsersWhere = "users.is_super_admin = false AND users.is_active = true AND users.is_banned = false"

// acceptedExists -- подзапрос «у пользователя есть действующее согласие нужной
// редакции». Условие действующего согласия скопировано с consentService.ActiveVersion:
// granted и не отозвано.
const acceptedExists = `EXISTS (
	SELECT 1 FROM pd_consents c
	WHERE c.user_id = users.id
	  AND c.consent_type = ?
	  AND c.granted = true
	  AND c.revoked_at IS NULL
	  AND c.document_version >= ?
)`

// PendingListLimit -- сколько не подтвердивших отдаём в обычном ответе. Список нужен
// администратору как рабочий («кому напомнить»), а не как выгрузка: сразу после
// подъёма редакции в него попадают все работники, и на крупной установке полный
// список - тысячи строк в разметке. За полным ходят явно, для выгрузки в файл.
const PendingListLimit = 50

// Collection возвращает сводку по сбору согласий текущей редакции вместе со списком
// тех, кто ещё не подтвердил. full=true снимает ограничение на длину списка.
func (s *PDConsentStatsService) Collection(ctx context.Context, full bool) (*models.PDConsentCollection, error) {
	req, err := s.gate.Requirement(ctx)
	if err != nil {
		return nil, err
	}
	version := req.Version
	if version < 1 {
		version = 1
	}

	var total int64
	if err := s.db.WithContext(ctx).Model(&models.User{}).
		Where(gatedUsersWhere).
		Count(&total).Error; err != nil {
		return nil, err
	}

	var accepted int64
	if err := s.db.WithContext(ctx).Model(&models.User{}).
		Where(gatedUsersWhere).
		Where(acceptedExists, ConsentTypePDProcessing, version).
		Count(&accepted).Error; err != nil {
		return nil, err
	}

	limit := PendingListLimit
	if full {
		limit = 0
	}
	pending, err := s.pendingUsers(ctx, version, limit)
	if err != nil {
		return nil, err
	}

	pendingCount := int(total) - int(accepted)
	return &models.PDConsentCollection{
		Active:       req.Enabled,
		Version:      version,
		Total:        int(total),
		Accepted:     int(accepted),
		Pending:      pendingCount,
		PendingUsers: pending,
		Truncated:    len(pending) < pendingCount,
	}, nil
}

type pendingRow struct {
	ID           int
	Username     string
	LastName     *string
	FirstName    *string
	Organization *string
}

func (s *PDConsentStatsService) pendingUsers(ctx context.Context, version, limit int) ([]models.PDConsentPendingUser, error) {
	var rows []pendingRow
	q := s.db.WithContext(ctx).Model(&models.User{}).
		Select("users.id, users.username, users.last_name, users.first_name, organizations.name AS organization").
		Joins("LEFT JOIN organizations ON organizations.id = users.organization_id").
		Where(gatedUsersWhere).
		Where("NOT "+acceptedExists, ConsentTypePDProcessing, version).
		Order("users.last_name NULLS LAST, users.username")
	if limit > 0 {
		q = q.Limit(limit)
	}
	if err := q.Scan(&rows).Error; err != nil {
		return nil, err
	}

	pending := make([]models.PDConsentPendingUser, 0, len(rows))
	for _, r := range rows {
		pending = append(pending, models.PDConsentPendingUser{
			ID:       r.ID,
			Username: r.Username,
			// fullName - общая с аудитом справочников сборка «Фамилия Имя»
			// с фолбэком на логин, чтобы строка не оказалась пустой.
			FullName:     fullName(r.LastName, r.FirstName, r.Username),
			Organization: derefStr(r.Organization),
		})
	}
	return pending, nil
}
