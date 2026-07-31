package services

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

// userSearchProvider ищет по учётным записям.
//
// Сужения по владельцу нет: право page.admin.users -- это и есть доступ к разделу
// пользователей целиком, тем же ключом закрыта страница в интерфейсе, и листинг там
// тоже отдаёт всех. Поиск повторяет его доступность, не расширяя.
//
// В выдачу идёт ровно то, что видно в списке пользователей: ФИО, логин, должность,
// организация. Ни телефон, ни почта в подзаголовок не кладутся -- искать по ним удобно,
// а показывать их в подсказках означало бы рассылать контакты сотрудников шире, чем это
// делает сам раздел.
type userSearchProvider struct{}

func (userSearchProvider) Type() SearchEntityType { return SearchTypeUsers }
func (userSearchProvider) Title() string          { return "Пользователи" }
func (userSearchProvider) PermissionKey() string  { return KeyPageAdminUsers }

func (userSearchProvider) Search(ctx context.Context, db *gorm.DB, req searchRequest) ([]SearchItem, error) {
	cols := []string{
		"u.last_name", "u.first_name", "u.middle_name",
		"u.username", `u."position"`, "u.email", "u.phone",
	}
	cond, args := ilikePatternsArgs(cols, req.Variants)

	q := db.WithContext(ctx).
		Table("users u").
		Joins("LEFT JOIN organizations o ON u.organization_id = o.id").
		Joins("LEFT JOIN companies c ON u.company_id = c.id").
		Select(`u.id AS id,
			NULLIF(TRIM(CONCAT_WS(' ', u.last_name, u.first_name, u.middle_name)), '') AS title,
			CONCAT_WS(' · ', u.username, NULLIF(u."position", ''), COALESCE(o.name, c.name)) AS subtitle`).
		Where(cond, args...)

	rows := make([]searchRow, 0, req.Limit+1)
	if err := q.
		Order(gorm.Expr(`CASE
			WHEN LOWER(TRIM(COALESCE(u.last_name, ''))) = LOWER(TRIM(?)) THEN 0
			WHEN LOWER(COALESCE(u.username, '')) = LOWER(TRIM(?)) THEN 0
			WHEN LOWER(COALESCE(u.last_name, '')) LIKE LOWER(?) || '%' THEN 1
			ELSE 2 END, u.id DESC`, req.Raw, req.Raw, req.Raw)).
		Limit(req.Limit + 1).
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("поиск по пользователям: %w", err)
	}

	// У учётной записи может не быть ФИО -- тогда показываем логин, иначе строка
	// приедет с пустым заголовком и подсказка будет выглядеть сломанной.
	items := rowsToItems(SearchTypeUsers, "user", rows)
	for i := range items {
		if items[i].Title == "" {
			items[i].Title = items[i].Subtitle
		}
	}
	return items, nil
}
