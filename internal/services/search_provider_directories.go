package services

import (
	"context"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// directoryKind описывает одну ветку поиска по справочникам: таблицу, подпись раздела
// в выдаче и код сущности для перехода.
type directoryKind struct {
	table    string
	titleCol string
	label    string
	entity   string
}

// Справочники системы. Все они устроены одинаково (id + name + is_active) и видны
// одному и тому же кругу лиц, поэтому ищутся одним запросом через UNION ALL: семь
// отдельных обращений к базе ради семи маленьких таблиц не окупаются.
var directoryKinds = []directoryKind{
	{"organizations", "name", "Организация", "organization"},
	{"companies", "name", "Компания", "company"},
	{"unload_places", "name", "Место разгрузки", "unload_place"},
	{"system_tables", "COALESCE(NULLIF(display_name, ''), name)", "Таблица КПП", "system_table"},
	{"marks", "name", "Марка", "mark"},
	{"citizenships", "name", "Гражданство", "citizenship"},
	{"license_plate_formats", "name", "Формат номера", "license_plate_format"},
}

// directorySearchProvider ищет по справочникам.
//
// Сужения по владельцу здесь нет и быть не может: справочники общесистемные, у их
// записей нет ни организации, ни автора. Разграничение даёт само право
// page.admin.directories -- тем же ключом закрыты страницы справочников в интерфейсе,
// так что поиск повторяет их доступность один в один.
//
// В выдачу идут только действующие записи: архивные нужны в разделе, где их можно
// восстановить, а в подсказках они только мешают.
type directorySearchProvider struct{}

func (directorySearchProvider) Type() SearchEntityType { return SearchTypeDirectories }
func (directorySearchProvider) Title() string          { return "Справочники" }
func (directorySearchProvider) PermissionKey() string  { return KeyPageAdminDirectories }

func (directorySearchProvider) Search(ctx context.Context, db *gorm.DB, req searchRequest) ([]SearchItem, error) {
	branches := make([]string, 0, len(directoryKinds))
	args := make([]interface{}, 0, len(directoryKinds)*len(req.Variants)*2)

	for _, k := range directoryKinds {
		cond, condArgs := searchCondition([]string{k.titleCol}, req.Raw)
		branches = append(branches, fmt.Sprintf(
			`(SELECT id, %s AS title, ? AS subtitle, ? AS kind
			  FROM %s WHERE is_active AND (%s)
			  ORDER BY id DESC LIMIT ?)`,
			k.titleCol, k.table, cond))
		args = append(args, k.label, k.entity)
		args = append(args, condArgs...)
		args = append(args, req.Limit+1)
	}

	// Внешний LIMIT поверх объединения: без него до сортировки доезжало бы
	// len(directoryKinds) * (limit+1) строк ради limit нужных.
	sql := fmt.Sprintf(`SELECT id, title, subtitle, kind FROM (%s) d
		ORDER BY CASE
			WHEN LOWER(TRIM(title)) = LOWER(TRIM(?)) THEN 0
			WHEN LOWER(title) LIKE LOWER(?) || '%%' THEN 1
			ELSE 2 END, title
		LIMIT ?`, strings.Join(branches, " UNION ALL "))
	args = append(args, req.Raw, req.Raw, req.Limit+1)

	// Через scanKindedRows, а не rowsToItems: у каждой строки свой код сущности --
	// организация и марка ведут на разные страницы.
	return scanKindedRows(ctx, db, SearchTypeDirectories, sql, args, "поиск по справочникам")
}
