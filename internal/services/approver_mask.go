package services

import (
	"context"

	"gorm.io/gorm"
)

// loadApproverMasks возвращает карту user_id -> маска отображаемого имени для принимающих,
// у которых задан непустой display_name. Список принимающих мал (единицы), поэтому грузим
// один раз на запрос. Если масок нет, возвращает nil - вызывающая сторона тогда пропускает
// маскировку без накладных расходов.
func loadApproverMasks(ctx context.Context, db *gorm.DB) map[int]string {
	type row struct {
		UserID      int    `gorm:"column:user_id"`
		DisplayName string `gorm:"column:display_name"`
	}
	var rows []row
	err := db.WithContext(ctx).
		Table("application_approvers").
		Select("user_id, display_name").
		Where("display_name IS NOT NULL AND TRIM(display_name) <> ''").
		Scan(&rows).Error
	if err != nil || len(rows) == 0 {
		return nil
	}
	masks := make(map[int]string, len(rows))
	for _, r := range rows {
		masks[r.UserID] = r.DisplayName
	}
	return masks
}

// maskName возвращает маску принимающего, если для userID она задана; иначе real.
func maskName(masks map[int]string, userID *int, real string) string {
	if len(masks) == 0 || userID == nil {
		return real
	}
	if m, ok := masks[*userID]; ok {
		return m
	}
	return real
}

// maskNamePtr - вариант maskName для *string (например responsible_full_name): при
// наличии маски возвращает указатель на её копию, иначе исходное значение.
func maskNamePtr(masks map[int]string, userID *int, real *string) *string {
	if len(masks) == 0 || userID == nil {
		return real
	}
	if m, ok := masks[*userID]; ok {
		mm := m
		return &mm
	}
	return real
}
