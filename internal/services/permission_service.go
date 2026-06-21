package services

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// PermissionService определяет интерфейс управления разрешениями.
type PermissionService interface {
	GetMyPermissions(ctx context.Context, username string) ([]models.UserPermissionResponse, error)
	GetUserPermissions(ctx context.Context, isSuperAdmin bool, userID int) ([]models.UserPermissionResponse, error)
	UpdateUserPermissions(ctx context.Context, isSuperAdmin bool, actorID int, userID int, req models.UpdatePermissionsRequest) error
	GetPermissionTree(ctx context.Context) ([]models.PermissionTreeNode, error)
	GetCatalog(ctx context.Context) ([]CatalogNode, error)
	AutoGenerateForTable(ctx context.Context, tableID int, tableName string) error
	HasPermission(ctx context.Context, userID int, key string) (bool, error)
	HasPermissionValue(ctx context.Context, userID int, key string, value string) (bool, error)
	GrantDefaultPermissions(ctx context.Context, userID int) error
	GrantPermission(ctx context.Context, userID int, key, value string) error
}

type permissionService struct {
	db *gorm.DB
}

// NewPermissionService создаёт реализацию PermissionService.
func NewPermissionService(db *gorm.DB) PermissionService {
	return &permissionService{db: db}
}

// GetMyPermissions возвращает разрешения текущего пользователя по username.
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

// GetUserPermissions возвращает разрешения указанного пользователя (admin-only).
func (s *permissionService) GetUserPermissions(ctx context.Context, isSuperAdmin bool, userID int) ([]models.UserPermissionResponse, error) {
	if !isSuperAdmin {
		return nil, echo.NewHTTPError(http.StatusForbidden, "Доступ только для супер-администратора")
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
		Select("p.key, p.category, p.display_name, up.value, u.username as granted_by_name").
		Joins("JOIN permissions p ON p.key = up.permission_key").
		Joins("LEFT JOIN users u ON u.id = up.granted_by").
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

// UpdateUserPermissions обновляет набор разрешений пользователя (admin-only).
func (s *permissionService) UpdateUserPermissions(ctx context.Context, isSuperAdmin bool, actorID int, userID int, req models.UpdatePermissionsRequest) error {
	if !isSuperAdmin {
		return echo.NewHTTPError(http.StatusForbidden, "Доступ только для супер-администратора")
	}

	// Verify user exists
	var count int64
	if err := s.db.WithContext(ctx).Model(&models.User{}).Where("id = ?", userID).Count(&count).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка БД")
	}
	if count == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "Пользователь не найден")
	}

	// Валидация ключей: каталожные валидны сразу, остальные (динамические table.*
	// и legacy-ключи из таблицы permissions) проверяются по БД.
	var nonCatalogKeys []string
	for _, p := range req.Permissions {
		if !IsCatalogKey(p.Key) {
			nonCatalogKeys = append(nonCatalogKeys, p.Key)
		}
	}
	if len(nonCatalogKeys) > 0 {
		var existingCount int64
		if err := s.db.WithContext(ctx).Model(&models.Permission{}).Where("key IN ?", nonCatalogKeys).Count(&existingCount).Error; err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка БД")
		}
		if int(existingCount) != len(nonCatalogKeys) {
			return echo.NewHTTPError(http.StatusBadRequest, "Некоторые ключи разрешений не существуют")
		}
	}

	adminID := actorID

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

// GetPermissionTree возвращает дерево разрешений с группировкой по родительским ключам.
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

// GetCatalog возвращает полный каталог прав: статическое дерево (Catalog) плюс
// динамические права таблиц (table.<slug>.*) из БД под категорией "Таблицы".
func (s *permissionService) GetCatalog(ctx context.Context) ([]CatalogNode, error) {
	nodes := Catalog()

	var tablePerms []models.Permission
	if err := s.db.WithContext(ctx).
		Where("category = ?", "table").
		Order("display_name, key").
		Find(&tablePerms).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка получения каталога прав")
	}
	for _, p := range tablePerms {
		nodes = append(nodes, CatalogNode{
			Key:         p.Key,
			DisplayName: p.DisplayName,
			Category:    CatTables,
		})
	}
	return nodes, nil
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

// tableVerbs -- набор прав, генерируемых для каждой системной таблицы.
// Порядок = порядок отображения в UI. Глагол view -- родитель (доступ к таблице),
// остальные -- действия внутри неё.
var tableVerbs = []struct{ Verb, Title string }{
	{"view", "Доступ к таблице"},
	{"entry", "Отметка въезда/входа"},
	{"exit", "Отметка выезда/выхода"},
	{"detail", "Открытие карточки из таблицы"},
	{"history", "История таблицы"},
	{"export", "Экспорт"},
	{"trash", "Корзина"},
	{"delete", "Удаление записи"},
}

// AutoGenerateForTable создаёт права для системной таблицы (по одному на глагол).
func (s *permissionService) AutoGenerateForTable(ctx context.Context, tableID int, tableName string) error {
	displayName := tableName

	permissions := make([]models.Permission, 0, len(tableVerbs))
	for _, v := range tableVerbs {
		permissions = append(permissions, models.Permission{
			Key:         fmt.Sprintf("table.%s.%s", tableName, v.Verb),
			Category:    "table",
			EntityID:    &tableID,
			DisplayName: fmt.Sprintf("%s: %s", displayName, v.Title),
			ParentKey:   nil,
		})
	}

	for i := range permissions {
		var existing models.Permission
		err := s.db.WithContext(ctx).Where("key = ?", permissions[i].Key).First(&existing).Error
		if err == gorm.ErrRecordNotFound {
			if err := s.db.WithContext(ctx).Create(&permissions[i]).Error; err != nil {
				slog.Error("не удалось создать разрешение", "key", permissions[i].Key, "error", err)
				return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка создания разрешений")
			}
		} else if err != nil {
			slog.Error("не удалось проверить разрешение", "key", permissions[i].Key, "error", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка создания разрешений")
		}
	}

	slog.Info("разрешения для таблицы созданы", "table_id", tableID, "table_name", tableName)
	return nil
}

// HasPermission проверяет наличие разрешения с значением allow у пользователя.
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

// HasPermissionValue проверяет наличие разрешения с конкретным значением у пользователя.
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

// GrantDefaultPermissions назначает набор разрешений по умолчанию новому пользователю.
func (s *permissionService) GrantDefaultPermissions(ctx context.Context, userID int) error {
	defaults := []struct {
		Key   string
		Value string
	}{
		{"tab.cars.view", "allow"},
		{"tab.employees.view", "allow"},
		{"tab.overview.view", "allow"},
		{"tab.profile.view", "allow"},
		{"tab.applications.view", "allow"},
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

// GrantPermission назначает указанное разрешение пользователю (идемпотентно).
func (s *permissionService) GrantPermission(ctx context.Context, userID int, key, value string) error {
	up := models.UserPermission{
		UserID:        userID,
		PermissionKey: key,
		Value:         value,
	}
	return s.db.WithContext(ctx).
		Where("user_id = ? AND permission_key = ?", userID, key).
		FirstOrCreate(&up).Error
}
