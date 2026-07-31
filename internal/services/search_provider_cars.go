package services

import (
	"context"
	"fmt"
	"strings"

	"systemburo/internal/normalize"

	"gorm.io/gorm"
)

// uniqueCarSearchProvider ищет по реестру машин (unique_cars).
//
// Зеркалит buildCarsQuery (unique_car_service.go): тот же набор колонок (номер, марка,
// формат, организация, компания), та же ветка слитно-раздельного номера через
// normalize.Plate и тот же фильтр видимости -- свои записи, своей организации, своей
// компании, полный срез только по searchCanSeeAllSystem.
type uniqueCarSearchProvider struct{}

func (uniqueCarSearchProvider) Type() SearchEntityType { return SearchTypeCars }
func (uniqueCarSearchProvider) Title() string          { return "Автомобили" }
func (uniqueCarSearchProvider) PermissionKey() string  { return KeyEntityCarsRead }

func (uniqueCarSearchProvider) Search(ctx context.Context, db *gorm.DB, req searchRequest) ([]SearchItem, error) {
	cols := []string{"uc.number", "uc.mark", "lpf.name", "o.name", "c.name"}
	cond, args := ilikePatternsArgs(cols, req.Variants)
	// Номер с пробелами и без: "А 777 АА" находится по "А777АА" и наоборот. Условие
	// повторяет buildCarsQuery -- расходиться этим двум местам нельзя, иначе поиск и
	// реестр начнут отвечать по-разному на один и тот же номер.
	if strings.ContainsAny(req.Raw, "0123456789") {
		cond += " OR REPLACE(uc.number, ' ', '') ILIKE ?"
		args = append(args, "%"+normalize.Plate(req.Raw)+"%")
	}

	q := db.WithContext(ctx).
		Table("unique_cars uc").
		Joins("LEFT JOIN organizations o ON uc.organization_id = o.id").
		Joins("LEFT JOIN companies c ON uc.company_id = c.id").
		Joins("LEFT JOIN license_plate_formats lpf ON uc.format_id = lpf.id").
		Select(`uc.id AS id,
			COALESCE(uc.number, '') AS title,
			CONCAT_WS(' · ', NULLIF(uc.mark, ''), COALESCE(o.name, c.name)) AS subtitle`).
		Where(cond, args...)

	q = applyRegistryScope(q, "uc", req)

	rows := make([]searchRow, 0, req.Limit+1)
	// Ступенчатый CASE вместо similarity(): выражение дешёвое и не требует своего
	// индекса, а строк после отбора по GIN немного. Тонкая ранжировка -- в Go.
	if err := q.
		Order(gorm.Expr(`CASE
			WHEN LOWER(TRIM(COALESCE(uc.number, ''))) = LOWER(TRIM(?)) THEN 0
			WHEN LOWER(COALESCE(uc.number, '')) LIKE LOWER(?) || '%' THEN 1
			ELSE 2 END, uc.id DESC`, req.Raw, req.Raw)).
		Limit(req.Limit + 1).
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("поиск по реестру машин: %w", err)
	}

	return rowsToItems(SearchTypeCars, "unique_car", rows), nil
}
