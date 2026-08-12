package services

import (
	"context"
	"log/slog"
	"net/http"
	"sort"
	"strings"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
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
		Select(`u.id, u.username, u.last_name, u.first_name, u.middle_name, u.position,
			o.name as organization, c.name as company`).
		Scan(&result).Error
	if err != nil {
		slog.Error("не удалось получить кандидатов в получатели заявки", "error", err, "user_id", me.ID)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching recipient candidates")
	}

	masks, _ := consentMasksWithState(ctx, db)
	for i := range result {
		if _, hidden := masks[result[i].ID]; !hidden {
			continue
		}
		maskUserParts(masks, result[i].ID, &result[i].LastName, &result[i].FirstName, &result[i].MiddleName)
		result[i].PDHidden = true
	}

	// Сортируем после маскировки и по тому, что видит клиент. Порядок по настоящей
	// фамилии выдал бы её первую букву: скрытый работник встал бы на своё алфавитное
	// место между видимыми однофамильцами - ровно то, что маскировка #1567 прячет.
	sort.SliceStable(result, func(i, j int) bool {
		return recipientSortKey(result[i]) < recipientSortKey(result[j])
	})
	return result, nil
}

// recipientSortKey - строка, по которой список выглядит упорядоченным для человека:
// видимое ФИО, а у скрытого работника - его заглушка, уехавшая в конец алфавита.
func recipientSortKey(c models.RecipientCandidate) string {
	parts := make([]string, 0, 3)
	for _, p := range []*string{c.LastName, c.FirstName, c.MiddleName} {
		if p != nil && *p != "" {
			parts = append(parts, *p)
		}
	}
	if len(parts) == 0 {
		return "￿" + c.Username
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
