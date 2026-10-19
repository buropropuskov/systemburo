package services

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// Общая область чтения списков, поиска и конкретных записей. Она не даёт права
// изменения или административного раскрытия документов.
type registryReadScope struct {
	userID                           int
	orgID, companyID                 *int
	organization, company, allSystem bool
}

func newRegistryReadScope(set PermissionSet, userID int, orgID, companyID *int) registryReadScope {
	return registryReadScope{userID: userID, orgID: orgID, companyID: companyID,
		organization: set.Has(KeySectionRegistryOrganization), company: set.Has(KeySectionRegistryCompany),
		allSystem: set.Has(KeySectionRegistryAllSystem)}
}

func (s registryReadScope) authorizeFilter(filter string) error {
	if (filter == "organization" && !s.organization) || (filter == "company" && !s.company) ||
		(filter == "all_system" && !s.allSystem) {
		return echo.NewHTTPError(http.StatusForbidden, "Доступ запрещён")
	}
	return nil
}

func registrySameID(a, b *int) bool { return a != nil && b != nil && *a > 0 && *a == *b }

func (s registryReadScope) contains(userID, orgID, companyID *int) bool {
	return s.allSystem || (userID != nil && *userID == s.userID) ||
		(s.organization && registrySameID(s.orgID, orgID)) || (s.company && registrySameID(s.companyID, companyID))
}

// alias задаётся кодом, а не пользователем. Публичные списки предварительно
// проверяют запрошенный раздел через authorizeFilter.
func (s registryReadScope) apply(q *gorm.DB, alias, filter string) *gorm.DB {
	if filter == "all_system" && s.allSystem {
		return q
	}
	column := func(name string) string { return fmt.Sprintf("%s.%s", alias, name) }
	if filter == "organization" && s.organization && s.orgID != nil && *s.orgID > 0 {
		return q.Where(column("organization_id")+" = ?", *s.orgID)
	}
	if filter == "company" && s.company && s.companyID != nil && *s.companyID > 0 {
		return q.Where(column("company_id")+" = ?", *s.companyID)
	}
	cond, args := column("user_id")+" = ?", []any{s.userID}
	if filter == "all" {
		if s.organization && s.orgID != nil && *s.orgID > 0 {
			cond += " OR " + column("organization_id") + " = ?"
			args = append(args, *s.orgID)
		}
		if s.company && s.companyID != nil && *s.companyID > 0 {
			cond += " OR " + column("company_id") + " = ?"
			args = append(args, *s.companyID)
		}
	}
	return q.Where(cond, args...)
}
