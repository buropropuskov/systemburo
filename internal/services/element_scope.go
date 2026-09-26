package services

import (
	"context"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// ElementKind - машина или сотрудник заявки (таблицы cars/employees).
type ElementKind string

const (
	ElementCar      ElementKind = "cars"
	ElementEmployee ElementKind = "employees"
)

// elementBindingTables - таблица привязок элемента к постам. Имена идут в SQL строкой,
// поэтому только из этого списка.
var elementBindingTables = map[ElementKind]struct{ table, column string }{
	ElementCar:      {"car_target_tables", "car_id"},
	ElementEmployee: {"employee_target_tables", "employee_id"},
}

// ElementScope - какие машины и сотрудники заявок видны пользователю. История элемента,
// его статус на территории и места разгрузки отдаются только в этих границах: до этого
// любой вошедший читал их по всей системе, в том числе историю человека по ФИО.
//
// Элемент виден, если выполнено хотя бы одно:
//   - пользователь администратор (All);
//   - элемент привязан к посту, на который у пользователя есть table.<пост>.view или .trash;
//   - у пользователя есть доступ к заявке элемента (автор, ответственный, читатель;
//     принимающий видит элементы всех заявок) - зеркало applyApplicationAccessFilter;
//   - элемент принадлежит его организации или компании - зеркало реестра (applyRegistryScope).
type ElementScope struct {
	All       bool
	UserID    int
	Approver  bool
	OrgID     int
	CompanyID int
	TableIDs  []int
}

// FullElementScope - без ограничений, для внутренних вызовов без пользователя (снимок таблицы).
func FullElementScope() ElementScope { return ElementScope{All: true} }

// ElementScopeResolver собирает ElementScope пользователя из его прав и данных.
type ElementScopeResolver struct {
	db       *gorm.DB
	resolver *PermissionResolver
}

// NewElementScopeResolver создаёт ElementScopeResolver.
func NewElementScopeResolver(db *gorm.DB, resolver *PermissionResolver) *ElementScopeResolver {
	return &ElementScopeResolver{db: db, resolver: resolver}
}

// Resolve строит скоуп пользователя. allKeys - права, которые в этом месте открывают
// все элементы сразу (раздел чёрного списка ищет историю человека по всей системе).
// Заблокированному возвращается пустой скоуп: под него не подходит ни один элемент.
func (r *ElementScopeResolver) Resolve(ctx context.Context, userID int, allKeys ...string) (ElementScope, error) {
	set, err := r.resolver.Resolve(ctx, userID)
	if err != nil {
		return ElementScope{}, fmt.Errorf("failed to resolve permissions for element scope: %w", err)
	}
	if set.IsBanned() {
		return ElementScope{}, nil
	}
	if set.IsSuperAdmin() || set.IsAdmin() {
		return FullElementScope(), nil
	}
	for _, key := range allKeys {
		if set.Has(key) {
			return FullElementScope(), nil
		}
	}

	scope := ElementScope{UserID: userID}

	var owner struct {
		OrganizationID *int
		CompanyID      *int
	}
	if err := r.db.WithContext(ctx).Table("users").
		Select("organization_id, company_id").
		Where("id = ?", userID).
		Scan(&owner).Error; err != nil {
		return ElementScope{}, fmt.Errorf("failed to load user for element scope: %w", err)
	}
	if owner.OrganizationID != nil {
		scope.OrgID = *owner.OrganizationID
	}
	if owner.CompanyID != nil {
		scope.CompanyID = *owner.CompanyID
	}

	var approvers int64
	if err := r.db.WithContext(ctx).Table("application_approvers").
		Where("user_id = ?", userID).Count(&approvers).Error; err != nil {
		return ElementScope{}, fmt.Errorf("failed to check approver for element scope: %w", err)
	}
	scope.Approver = approvers > 0

	var tables []struct {
		ID   int
		Name string
	}
	if err := r.db.WithContext(ctx).Table("system_tables").
		Select("id, name").Scan(&tables).Error; err != nil {
		return ElementScope{}, fmt.Errorf("failed to load tables for element scope: %w", err)
	}
	for _, t := range tables {
		if set.Has(fmt.Sprintf("table.%s.view", t.Name)) || set.Has(fmt.Sprintf("table.%s.trash", t.Name)) {
			scope.TableIDs = append(scope.TableIDs, t.ID)
		}
	}
	return scope, nil
}

// Visible отвечает, виден ли пользователю элемент с этим id. Несуществующий элемент не виден.
func (r *ElementScopeResolver) Visible(ctx context.Context, scope ElementScope, kind ElementKind, id int) (bool, error) {
	cond, args := scope.Predicate(kind, "el", "a", "app")
	var visible bool
	query := fmt.Sprintf(`SELECT EXISTS(
		SELECT 1 FROM %s el
		LEFT JOIN attachments a ON el.attachment_id = a.id
		LEFT JOIN applications app ON a.application_id = app.id
		WHERE el.id = ? AND (%s))`, kind, cond)
	if err := r.db.WithContext(ctx).Raw(query, append([]any{id}, args...)...).Scan(&visible).Error; err != nil {
		return false, fmt.Errorf("failed to check %s %d visibility: %w", kind, id, err)
	}
	return visible, nil
}

// Predicate - SQL-условие видимости элемента для WHERE. elem, att, app - алиасы
// элемента, его вложения и заявки (заявка через LEFT JOIN: у ручных элементов её нет).
func (s ElementScope) Predicate(kind ElementKind, elem, att, app string) (string, []any) {
	if s.All {
		return "TRUE", nil
	}
	binding := elementBindingTables[kind]

	conds := []string{fmt.Sprintf(`(%[1]s.id IS NOT NULL AND (? OR %[1]s.sender_user_id = ?
		OR EXISTS(SELECT 1 FROM application_responsible_users aru WHERE aru.application_id = %[1]s.id AND aru.user_id = ?)
		OR EXISTS(SELECT 1 FROM application_viewers av WHERE av.application_id = %[1]s.id AND av.user_id = ?)))`, app)}
	args := []any{s.Approver, s.UserID, s.UserID, s.UserID}

	if len(s.TableIDs) > 0 {
		conds = append(conds, fmt.Sprintf(`EXISTS(SELECT 1 FROM %s b WHERE b.%s = %s.id AND b.table_id IN ?)`,
			binding.table, binding.column, elem))
		args = append(args, s.TableIDs)
	}
	if s.OrgID != 0 {
		conds = append(conds, fmt.Sprintf(`COALESCE(%s.organization_id, %s.organization_id) = ?`, app, att))
		args = append(args, s.OrgID)
	}
	if s.CompanyID != 0 {
		conds = append(conds, fmt.Sprintf(`COALESCE(%s.company_id, %s.company_id) = ?`, app, att))
		args = append(args, s.CompanyID)
	}
	return "(" + strings.Join(conds, " OR ") + ")", args
}
