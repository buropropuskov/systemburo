package services

import (
	"context"
	"log/slog"
	"net/http"
	"sort"
	"strings"

	"systemburo/internal/models"

	"github.com/labstack/echo/v5"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// managerUserTypeCode - код типа «Руководитель» в user_types. Резолвится по code, а не
// по имени: имя типа администратор может переименовать, code у системных типов заморожен.
const managerUserTypeCode = "manager"

// recipientCandidateScope - предикат «кого автор заявки может добавить получателем»:
// коллеги по организации, коллеги по компании и руководители. Себя в список не берём,
// архивных и заблокированных - тоже.
//
// Единый источник для эндпоинта выбора и для валидации readers при подаче: разъедься
// они, форма предлагала бы людей, которых бэк потом молча выбрасывает.
func recipientCandidateScope(db *gorm.DB, me models.User) *gorm.DB {
	audience := db.Where("ut.code = ?", managerUserTypeCode)
	if me.OrganizationID != nil {
		audience = audience.Or("u.organization_id = ?", *me.OrganizationID)
	}
	if me.CompanyID != nil {
		audience = audience.Or("u.company_id = ?", *me.CompanyID)
	}

	return db.Table("users u").
		Joins("LEFT JOIN user_types ut ON u.type_id = ut.id").
		Where("u.is_active = ?", true).
		Where("u.is_banned = ?", false).
		Where("u.id <> ?", me.ID).
		Where(audience)
}

// loadRecipientCandidates отдаёт кандидатов с маскировкой персональных данных: работник
// без согласия на обработку ПД (#1567) виден заглушкой, а не настоящим ФИО.
//
// Организация и компания приезжают джойнами здесь, а не в recipientCandidateScope:
// проверке присланных клиентом читателей они не нужны, и лишние таблицы в её запросе
// были бы работой впустую. Отбор кандидатов от них не зависит - джойны левые, человек
// без организации из списка не пропадает.
func loadRecipientCandidates(ctx context.Context, db *gorm.DB, me models.User) ([]models.RecipientCandidate, error) {
	result := make([]models.RecipientCandidate, 0)
	err := recipientCandidateScope(db.WithContext(ctx), me).
		Joins("LEFT JOIN organizations o ON u.organization_id = o.id").
		Joins("LEFT JOIN companies c ON u.company_id = c.id").
		Select(`u.id, u.last_name, u.first_name, u.middle_name, u.position,
			o.name as organization, c.name as company`).
		Scan(&result).Error
	if err != nil {
		slog.Error("не удалось получить кандидатов в получатели заявки", "error", err, "user_id", me.ID)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching recipient candidates")
	}

	policy, err := loadUserDisplayPolicy(ctx, db)
	if err != nil {
		return nil, err
	}
	for i := range result {
		c := &result[i]
		label := policy.label(c.ID, c.LastName, c.FirstName, c.MiddleName)
		c.DisplayLabel = &label
		c.CanSelect = !policy.restricted(c.ID)
		maskUserParts(policy.names, c.ID, &c.LastName, &c.FirstName, &c.MiddleName)
		c.PDHidden = policy.restricted(c.ID)
		if isMasked(policy.names, c.ID) {
			c.Position, c.Organization, c.Company = nil, nil, nil
		}
	}

	// Сортируем после маскировки и по тому, что видит клиент. Порядок по настоящей
	// фамилии выдал бы её первую букву: скрытый работник встал бы на своё алфавитное
	// место между видимыми однофамильцами - ровно то, что маскировка #1567 прячет.
	sort.SliceStable(result, func(i, j int) bool {
		a, b := recipientSortKey(result[i]), recipientSortKey(result[j])
		if a != b {
			return a < b
		}
		return result[i].ID < result[j].ID
	})
	return result, nil
}

// recipientSortKey - строка, по которой список выглядит упорядоченным для человека:
// видимое ФИО, а у скрытого работника - его заглушка, уехавшая в конец алфавита.
func recipientSortKey(c models.RecipientCandidate) string {
	if c.DisplayLabel != nil {
		return *c.DisplayLabel
	}
	parts := make([]string, 0, 3)
	for _, p := range []*string{c.LastName, c.FirstName, c.MiddleName} {
		if p != nil && *p != "" {
			parts = append(parts, *p)
		}
	}
	if len(parts) == 0 {
		return unnamedUserLabel
	}
	return strings.Join(parts, " ")
}

// recipientCandidateIDs - те же кандидаты, но только идентификаторами: для проверки
// присланного клиентом списка читателей.
func recipientCandidateIDs(ctx context.Context, db *gorm.DB, me models.User) (map[int]struct{}, error) {
	var ids []int
	if err := recipientCandidateScope(db.WithContext(ctx), me).Pluck("u.id", &ids).Error; err != nil {
		slog.Error("не удалось получить идентификаторы кандидатов в получатели", "error", err, "user_id", me.ID)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error validating recipients")
	}

	allowed := make(map[int]struct{}, len(ids))
	for _, id := range ids {
		allowed[id] = struct{}{}
	}
	return allowed, nil
}

// Check explicitly selected accounts in the writer transaction. Consent changes
// take an exclusive lock on the same user row. Mandatory routing is separate.
func validateManualRecipients(ctx context.Context, tx *gorm.DB, ids []int) error {
	if len(ids) == 0 {
		return nil
	}
	var locked []int
	if err := tx.WithContext(ctx).Table("users").Where("id IN ?", ids).
		Order("id").Clauses(clause.Locking{Strength: "SHARE"}).Pluck("id", &locked).Error; err != nil {
		return nameMaskUnavailable(err)
	}
	masks, err := loadConsentMasks(ctx, tx)
	if err != nil {
		return err
	}
	for _, id := range locked {
		if isMasked(masks, id) {
			return echo.NewHTTPError(http.StatusConflict, "Получатель больше недоступен. Обновите список и выберите получателя заново")
		}
	}
	return nil
}
