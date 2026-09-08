package services

import (
	"context"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"systemburo/internal/crypto"
	"systemburo/internal/models"
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
	cond, args := searchCondition(cols, req.Raw)

	if contact, cargs := contactMatchCondition(req.Raw, masks); contact != "" {
		cond = "(" + cond + " OR " + contact + ")"
		args = append(args, cargs...)
	}

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

// contactMatchCondition - условие точного совпадения по почте или телефону.
//
// Контакты зашифрованы (#2351), искать по ним подстрокой нельзя. Остаётся точное
// совпадение по свёртке: человек находится, если ввести адрес или номер целиком.
// Нормализация та же, что при записи, иначе «+7 900 …» не нашёл бы «8900…».
//
// Записи под маской из этого условия исключаются поимённо - тот же запрет, что и на
// показ: подтвердить точным совпадением, чей это адрес, значит раскрыть его другим
// путём. Именно поимённо, а не отключением поиска целиком: масок на живой установке
// почти всегда больше нуля (на стенде 59 из 109), и общий выключатель означал бы,
// что по контактам не находится вообще никто - что и случилось после выката #2351.
func contactMatchCondition(raw string, masks map[int]string) (string, []any) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}

	key := crypto.GetGlobalKey()
	args := []any{
		crypto.ComputeHMAC(models.NormalizeEmailForHMAC(raw), key),
		crypto.ComputeHMAC(models.NormalizePhoneForHMAC(raw), key),
	}
	cond := "(u.email_hmac = ? OR u.phone_hmac = ?)"
	if len(masks) == 0 {
		return cond, args
	}

	hidden := make([]int, 0, len(masks))
	for id := range masks {
		hidden = append(hidden, id)
	}
	return "(" + cond + " AND u.id NOT IN (?))", append(args, hidden)
}
