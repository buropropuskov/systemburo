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
	// Скрытые до согласия персональные данные не должны находиться и через поиск:
	// пока маскировка работает, по почте и телефону не ищем вовсе, а скрытое ФИО
	// подменяем логином уже в выдаче. Иначе подсказка подтверждала бы, чей это
	// адрес, - то же раскрытие, только другим путём.
	masks := loadConsentMasks(ctx, db)
	cols := []string{
		"u.last_name", "u.first_name", "u.middle_name",
		"u.username", `u."position"`,
	}
	if len(masks) == 0 {
		cols = append(cols, "u.email", "u.phone")
	}
	cond, args := searchCondition(cols, req.Raw)

	rows := make([]searchRow, 0, req.Limit+1)
	err := withTrigramThreshold(ctx, db, func(tx *gorm.DB) error {
		q := tx.
			Table("users u").
			Joins("LEFT JOIN organizations o ON u.organization_id = o.id").
			Joins("LEFT JOIN companies c ON u.company_id = c.id").
			Select(`u.id AS id,
				NULLIF(TRIM(CONCAT_WS(' ', u.last_name, u.first_name, u.middle_name)), '') AS title,
				CONCAT_WS(' · ', u.username, NULLIF(u."position", ''), COALESCE(o.name, c.name),
					CASE WHEN u.is_active THEN NULL ELSE 'в архиве' END) AS subtitle,
				`+matchRankExprAny("u.last_name", "u.username"), req.Raw, req.Raw, req.Raw, req.Raw).
			Where(cond, args...)

		// Архивные учётные записи ниже действующих: на стенде их 92 из 109, и по
		// фамилии они вытесняли живого человека из выдачи целиком. Не прячем совсем -
		// архивную запись ищут, чтобы восстановить; в подзаголовке она помечена.
		return q.
			Order("u.is_active DESC, match_rank, u.id DESC").
			Limit(req.Limit + 1).
			Scan(&rows).Error
	})
	if err != nil {
		return nil, fmt.Errorf("поиск по пользователям: %w", err)
	}

	// У учётной записи может не быть ФИО -- тогда показываем логин, иначе строка
	// приедет с пустым заголовком и подсказка будет выглядеть сломанной. Скрытое до
	// согласия ФИО подменяем тем же логином.
	items := rowsToItems(SearchTypeUsers, "user", rows)
	for i := range items {
		if mask, hidden := masks[items[i].ID]; hidden {
			items[i].Title = mask
			continue
		}
		if items[i].Title == "" {
			items[i].Title = items[i].Subtitle
		}
	}
	return items, nil
}
