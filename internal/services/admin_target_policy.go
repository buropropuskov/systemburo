package services

import (
	"context"
	"errors"
	"net/http"

	"systemburo/internal/apperr"
	"systemburo/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// lockAdminTarget блокирует актуальные строки инициатора и цели по возрастанию ID.
// Порядок общий для чувствительных команд, в том числе с переставленными ролями.
// Вызывающий сохраняет транзакцию открытой до защищённой записи.
func lockAdminTarget(ctx context.Context, tx *gorm.DB, actorID, targetID int) (*models.User, *models.User, error) {
	var users []models.User
	if err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id IN ?", []int{actorID, targetID}).Order("id").Find(&users).Error; err != nil {
		return nil, nil, apperr.New(http.StatusServiceUnavailable, "Не удалось проверить учётные записи", err)
	}
	var actor, target *models.User
	for i := range users {
		if users[i].ID == actorID {
			actor = &users[i]
		}
		if users[i].ID == targetID {
			target = &users[i]
		}
	}
	if actor == nil {
		return nil, nil, apperr.Forbidden("Учётная запись инициатора недоступна")
	}
	if target == nil {
		return nil, nil, apperr.NotFound("Пользователь не найден")
	}
	return actor, target, nil
}

func loadAdminTarget(ctx context.Context, db *gorm.DB, username string) (*models.User, error) {
	var user models.User
	if err := db.WithContext(ctx).Where("username = ?", username).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.NotFound("Пользователь не найден")
		}
		return nil, apperr.New(http.StatusServiceUnavailable, "Не удалось прочитать пользователя", err)
	}
	return &user, nil
}

func authorizeAdminActor(ctx context.Context, db *gorm.DB, actor *models.User, permission string) error {
	if !actor.IsActive || actor.IsBanned {
		return apperr.Forbidden("Учётная запись инициатора недоступна")
	}
	// Новый resolver читает действующие права и персональные запреты в этой транзакции.
	set, err := NewPermissionResolver(db).Resolve(ctx, actor.ID)
	if err != nil {
		return apperr.New(http.StatusServiceUnavailable, "Не удалось проверить права", err)
	}
	if !set.Has(permission) {
		return apperr.Forbidden("Недостаточно прав для этого действия")
	}
	return nil
}

// checkAdminTargetRank использует сохранённые признаки ранга: бан и архив
// обнуляют эффективные права, но не снимают защиту административной цели.
func checkAdminTargetRank(actor, target *models.User, denySelf bool) error {
	if denySelf && actor.ID == target.ID {
		return apperr.Forbidden("Свой пароль нужно менять с подтверждением текущего пароля")
	}
	if (target.IsAdmin || target.IsSuperAdmin) && !actor.IsSuperAdmin {
		return apperr.Forbidden("Управлять паролем и адресом почты администратора может только супер-администратор")
	}
	return nil
}
