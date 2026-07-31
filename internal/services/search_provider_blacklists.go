package services

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

// blacklistSearchProvider ищет по чёрным спискам людей и машин.
//
// Оба списка закрыты правом page.admin.blacklist -- тем же, которым закрыты их листинги
// (#1531 как раз закрывал выгрузку ЧС без права). Сужать записи по владельцу нечем:
// список общесистемный.
//
// Причина попадания в список (reason) не ищется и не показывается. В ней пишут
// обстоятельства инцидента, и в подсказке, всплывающей от набора трёх букв, ей не место;
// открыть карточку в разделе это не мешает.
//
// Списки объединены одним запросом: оба маленькие, у обоих одно право и одинаковая форма
// строки, а два обращения к базе на один раздел выдачи не окупаются.
type blacklistSearchProvider struct{}

func (blacklistSearchProvider) Type() SearchEntityType { return SearchTypeBlacklist }
func (blacklistSearchProvider) Title() string          { return "Чёрные списки" }
func (blacklistSearchProvider) PermissionKey() string  { return KeyPageBlacklist }

func (blacklistSearchProvider) Search(ctx context.Context, db *gorm.DB, req searchRequest) ([]SearchItem, error) {
	personCond, personArgs := searchCondition(
		[]string{"last_name", "first_name", "middle_name"}, req.Raw)
	vehicleCond, vehicleArgs := searchCondition(
		[]string{"car_number", "mark_name"}, req.Raw)

	sql := fmt.Sprintf(`SELECT id, title, subtitle, kind FROM (
		(SELECT id,
			TRIM(CONCAT_WS(' ', last_name, first_name, middle_name)) AS title,
			'Человек в чёрном списке' AS subtitle,
			'person_blacklist' AS kind
		 FROM person_blacklists WHERE is_active AND (%s)
		 ORDER BY id DESC LIMIT ?)
		UNION ALL
		(SELECT id,
			COALESCE(car_number, '') AS title,
			CONCAT_WS(' · ', 'Машина в чёрном списке', NULLIF(mark_name, '')) AS subtitle,
			'vehicle_blacklist' AS kind
		 FROM vehicle_blacklists WHERE is_active AND (%s)
		 ORDER BY id DESC LIMIT ?)
	) b
	ORDER BY CASE
		WHEN LOWER(TRIM(title)) = LOWER(TRIM(?)) THEN 0
		WHEN LOWER(title) LIKE LOWER(?) || '%%' THEN 1
		ELSE 2 END, title
	LIMIT ?`, personCond, vehicleCond)

	args := make([]interface{}, 0, len(personArgs)+len(vehicleArgs)+5)
	args = append(args, personArgs...)
	args = append(args, req.Limit+1)
	args = append(args, vehicleArgs...)
	args = append(args, req.Limit+1)
	args = append(args, req.Raw, req.Raw, req.Limit+1)

	// Человек и машина ведут на разные вкладки раздела, отсюда свой код сущности
	// у каждой строки.
	return scanKindedRows(ctx, db, SearchTypeBlacklist, sql, args, "поиск по чёрным спискам")
}
