package services

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"systemburo/internal/auth"
	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// PermissionService определяет интерфейс управления разрешениями.
type PermissionService interface {
	GetMyPermissions(ctx context.Context, username string) ([]models.UserPermissionResponse, error)
	GetUserPermissions(ctx context.Context, typeID int, userID int) ([]models.UserPermissionResponse, error)
	UpdateUserPermissions(ctx context.Context, typeID int, userID int, req models.UpdatePermissionsRequest) error
	GetPermissionTree(ctx context.Context) ([]models.PermissionTreeNode, error)
	AutoGenerateForTable(ctx context.Context, tableID int, tableName string) error
	HasPermission(ctx context.Context, userID int, key string) (bool, error)
	HasPermissionValue(ctx context.Context, userID int, key string, value string) (bool, error)
	GrantDefaultPermissions(ctx context.Context, userID int) error
}

type permissionService struct {
	db *gorm.DB
}

// NewPermissionService создаёт реализацию PermissionService.
func NewPermissionService(db *gorm.DB) PermissionService {
	return &permissionService{db: db}
}

func (s *permissionService) GetMyPermissions(ctx context.Context, username string) ([]models.UserPermissionResponse, error) {
	var userID int
	if err := s.db.WithContext(ctx).
		Table("users").
		Select("id").
		Where("username = ?", username).
		Row().Scan(&userID); err != nil {
		return nil, echo.NewHTTPError(http.StatusNotFound, "Пользователь не найден")
	}

	return s.getUserPermissionsList(ctx, userID)
}

func (s *permissionService) GetUserPermissions(ctx context.Context, typeID int, userID int) ([]models.UserPermissionResponse, error) {
	if err := auth.CheckAdminByTypeID(s.db, ctx, typeID); err != nil {
		return nil, err
	}

	// Verify user exists
	var count int64
	if err := s.db.WithContext(ctx).Model(&models.User{}).Where("id = ?", userID).Count(&count).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка БД")
	}
	if count == 0 {
		return nil, echo.NewHTTPError(http.StatusNotFound, "Пользователь не найден")
	}

	return s.getUserPermissionsList(ctx, userID)
}

func (s *permissionService) getUserPermissionsList(ctx context.Context, userID int) ([]models.UserPermissionResponse, error) {
	var results []models.UserPermissionResponse

	err := s.db.WithContext(ctx).
		Table("user_permissions up").
		Select("p.key, p.category, p.display_name, up.value").
		Joins("JOIN permissions p ON p.key = up.permission_key").
		Where("up.user_id = ?", userID).
		Order("p.category, p.key").
		Scan(&results).Error
	if err != nil {
		slog.Error("не удалось получить разрешения пользователя", "user_id", userID, "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка получения разрешений")
	}

	if results == nil {
		results = []models.UserPermissionResponse{}
	}
	return results, nil
}

func (s *permissionService) UpdateUserPermissions(ctx context.Context, typeID int, userID int, req models.UpdatePermissionsRequest) error {
	if err := auth.CheckAdminByTypeID(s.db, ctx, typeID); err != nil {
		return err
	}

	// Verify user exists
	var count int64
	if err := s.db.WithContext(ctx).Model(&models.User{}).Where("id = ?", userID).Count(&count).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка БД")
	}
	if count == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "Пользователь не найден")
	}

	// Validate all permission keys exist
	keys := make([]string, len(req.Permissions))
	for i, p := range req.Permissions {
		keys[i] = p.Key
	}

	var existingCount int64
	if err := s.db.WithContext(ctx).Model(&models.Permission{}).Where("key IN ?", keys).Count(&existingCount).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка БД")
	}
	if int(existingCount) != len(keys) {
		return echo.NewHTTPError(http.StatusBadRequest, "Некоторые ключи разрешений не существуют")
	}

	// Get admin user ID for granted_by
	var adminID int
	err := s.db.WithContext(ctx).
		Table("users").
		Select("id").
		Joins("JOIN user_types ON users.type_id = user_types.id").
		Where("user_types.id = ?", typeID).
		Row().Scan(&adminID)
	if err != nil {
		adminID = 0
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, p := range req.Permissions {
			up := models.UserPermission{
				UserID:        userID,
				PermissionKey: p.Key,
				Value:         p.Value,
			}
			if adminID > 0 {
				up.GrantedBy = &adminID
			}

			// Upsert: update value if exists, create if not
			result := tx.Where("user_id = ? AND permission_key = ?", userID, p.Key).
				Assign(models.UserPermission{Value: p.Value, GrantedBy: up.GrantedBy}).
				FirstOrCreate(&up)
			if result.Error != nil {
				slog.Error("не удалось обновить разрешение", "user_id", userID, "key", p.Key, "error", result.Error)
				return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка обновления разрешений")
			}
		}
		return nil
	})
}

func (s *permissionService) GetPermissionTree(ctx context.Context) ([]models.PermissionTreeNode, error) {
	var permissions []models.Permission
	if err := s.db.WithContext(ctx).Order("category, key").Find(&permissions).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка получения разрешений")
	}

	// Build tree: group by parent_key
	byParent := make(map[string][]models.Permission)
	var roots []models.Permission

	for _, p := range permissions {
		if p.ParentKey == nil {
			roots = append(roots, p)
		} else {
			byParent[*p.ParentKey] = append(byParent[*p.ParentKey], p)
		}
	}

	tree := make([]models.PermissionTreeNode, 0, len(roots))
	for _, r := range roots {
		node := s.buildTreeNode(r, byParent)
		tree = append(tree, node)
	}

	return tree, nil
}

func (s *permissionService) buildTreeNode(p models.Permission, byParent map[string][]models.Permission) models.PermissionTreeNode {
	node := models.PermissionTreeNode{
		Key:         p.Key,
		DisplayName: p.DisplayName,
		Category:    p.Category,
	}

	children, ok := byParent[p.Key]
	if ok {
		node.Children = make([]models.PermissionTreeNode, 0, len(children))
		for _, child := range children {
			node.Children = append(node.Children, s.buildTreeNode(child, byParent))
		}
	}

	return node
}

func (s *permissionService) AutoGenerateForTable(ctx context.Context, tableID int, tableName string) error {
	displayName := tableName

	permissions := []models.Permission{
		{
			Key:         fmt.Sprintf("table.%s.view", tableName),
			Category:    "table",
			EntityID:    &tableID,
			DisplayName: fmt.Sprintf("Просмотр таблицы %s", displayName),
			ParentKey:   nil,
		},
		{
			Key:         fmt.Sprintf("table.%s.edit", tableName),
			Category:    "table",
			EntityID:    &tableID,
			DisplayName: fmt.Sprintf("Редактирование таблицы %s", displayName),
			ParentKey:   nil,
		},
	}

	for _, p := range permissions {
		// Use FirstOrCreate to avoid duplicate key errors
		result := s.db.WithContext(ctx).
			Where("key = ?", p.Key).
			FirstOrCreate(&p)
		if result.Error != nil {
			slog.Error("не удалось создать разрешение", "key", p.Key, "error", result.Error)
			return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка создания разрешений")
		}
	}

	slog.Info("разрешения для таблицы созданы", "table_id", tableID, "table_name", tableName)
	return nil
}

func (s *permissionService) HasPermission(ctx context.Context, userID int, key string) (bool, error) {
	var up models.UserPermission
	err := s.db.WithContext(ctx).
		Where("user_id = ? AND permission_key = ? AND value = ?", userID, key, "allow").
		First(&up).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return false, nil
		}
		return false, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка проверки разрешений")
	}
	return true, nil
}

func (s *permissionService) HasPermissionValue(ctx context.Context, userID int, key string, value string) (bool, error) {
	var up models.UserPermission
	err := s.db.WithContext(ctx).
		Where("user_id = ? AND permission_key = ? AND value = ?", userID, key, value).
		First(&up).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return false, nil
		}
		return false, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка проверки разрешений")
	}
	return true, nil
}

func (s *permissionService) GrantDefaultPermissions(ctx context.Context, userID int) error {
	defaults := []struct {
		Key   string
		Value string
	}{
		{"tab.cars.view", "allow"},
		{"tab.employees.view", "allow"},
		{"tab.overview.view", "allow"},
		{"tab.profile.view", "allow"},
	}

	for _, d := range defaults {
		up := models.UserPermission{
			UserID:        userID,
			PermissionKey: d.Key,
			Value:         d.Value,
		}
		// Use FirstOrCreate to be idempotent
		if err := s.db.WithContext(ctx).
			Where("user_id = ? AND permission_key = ?", userID, d.Key).
			FirstOrCreate(&up).Error; err != nil {
			slog.Error("не удалось назначить разрешение по умолчанию", "user_id", userID, "key", d.Key, "error", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка назначения разрешений")
		}
	}

	return nil
}
