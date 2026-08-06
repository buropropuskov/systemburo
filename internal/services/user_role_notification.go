package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"systemburo/internal/models"

	"gorm.io/gorm"
)

// UserRoleNotifier уведомляет владельца учётки, когда меняется её роль (#1748 S3).
// Саму смену выполняет PermissionGroupService.SetUserRole - его файл
// (internal/services/permission_group_service.go) off-limits для этого среза, поэтому
// обёртка не переписывает мутацию: читает роль ДО вызова, вызывает существующий метод
// как есть и уведомляет ПОСЛЕ успеха, если значение реально изменилось.
type UserRoleNotifier struct {
	db                  *gorm.DB
	permissionGroups    *PermissionGroupService
	notificationService NotificationService
}

// NewUserRoleNotifier конструирует обёртку. notificationService может быть nil
// (тесты, offline) - тогда уведомления просто не создаются.
func NewUserRoleNotifier(db *gorm.DB, permissionGroups *PermissionGroupService, notificationService NotificationService) *UserRoleNotifier {
	return &UserRoleNotifier{db: db, permissionGroups: permissionGroups, notificationService: notificationService}
}

// SetUserRole меняет роль пользователя (делегирует в PermissionGroupService без
// изменений) и уведомляет его, если значение реально изменилось. Несуществующий
// userID не считается ошибкой - ровно как в исходном SetUserRole: строка просто не
// находится ни для чтения "старой" роли, ни для UPDATE, уведомление не создаётся
// (некому).
func (n *UserRoleNotifier) SetUserRole(ctx context.Context, userID int, newRoleID *int) error {
	var user models.User
	found := n.db.WithContext(ctx).Select("id", "role_id").First(&user, userID).Error == nil
	oldRoleID := user.RoleID

	if err := n.permissionGroups.SetUserRole(ctx, userID, newRoleID); err != nil {
		return err
	}

	if !found || roleIDEqual(oldRoleID, newRoleID) {
		return nil
	}
	n.notifyRoleChanged(ctx, userID, newRoleID)
	return nil
}

// notifyRoleChanged создаёт персистентное уведомление о новой роли. Best-effort:
// ошибка не должна ломать саму смену роли, которая к этому моменту уже прошла.
func (n *UserRoleNotifier) notifyRoleChanged(ctx context.Context, userID int, newRoleID *int) {
	if n.notificationService == nil {
		return
	}

	roleName := ""
	if newRoleID != nil {
		var role models.Role
		if err := n.db.WithContext(ctx).Select("name").First(&role, *newRoleID).Error; err == nil {
			roleName = role.Name
		}
	}

	message := "Администратор изменил вашу роль."
	switch {
	case newRoleID == nil:
		message = "Администратор снял вашу роль."
	case roleName != "":
		message = fmt.Sprintf("Администратор назначил вам роль «%s».", roleName)
	}

	dataPayload := map[string]any{
		"role_id":    newRoleID,
		"role_name":  roleName,
		"changed_at": time.Now().UTC().Format(time.RFC3339),
	}
	dataBytes, err := json.Marshal(dataPayload)
	if err != nil {
		slog.Warn("не удалось сериализовать payload уведомления о смене роли", "error", err)
		return
	}
	dataStr := string(dataBytes)

	if err := n.notificationService.CreateForUser(
		ctx, userID, NotificationTypeRoleChanged,
		"Изменились роль или права", message, &dataStr,
	); err != nil {
		slog.Warn("не удалось создать уведомление о смене роли", "user_id", userID, "error", err)
	}
}

// roleIDEqual сравнивает два *int как значения (оба nil = равны).
func roleIDEqual(a, b *int) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
