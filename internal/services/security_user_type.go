package services

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

// Работник поста не распоряжается своим паролем: и первый пароль, и его замену
// выдаёт бюро пропусков (#2280). Отсюда две проверки типа учётной записи -
// по самому работнику и по типу, который ему собираются назначить.

// isSecurityUser сообщает, что учётная запись принадлежит работнику поста.
// Несуществующий пользователь - false без ошибки: его судьбу решают другие
// проверки, выдумывать тип из пустоты незачем.
func isSecurityUser(ctx context.Context, db *gorm.DB, userID int) (bool, error) {
	var row struct{ Code string }
	err := db.WithContext(ctx).
		Table("users").
		Select("user_types.code").
		Joins("LEFT JOIN user_types ON user_types.id = users.type_id").
		Where("users.id = ?", userID).
		Scan(&row).Error
	if err != nil {
		return false, fmt.Errorf("определение типа учётной записи: %w", err)
	}
	return row.Code == securityUserTypeCode, nil
}

// isSecurityUserType сообщает, что заданный тип учётной записи - работник поста.
func isSecurityUserType(ctx context.Context, db *gorm.DB, typeID int) (bool, error) {
	var row struct{ Code string }
	err := db.WithContext(ctx).
		Table("user_types").
		Select("code").
		Where("id = ?", typeID).
		Scan(&row).Error
	if err != nil {
		return false, fmt.Errorf("определение типа учётной записи: %w", err)
	}
	return row.Code == securityUserTypeCode, nil
}
