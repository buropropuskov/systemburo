package services

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"time"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// PermissionService определяет интерфейс управления разрешениями.
type PermissionService interface {
	GetMyPermissions(ctx context.Context, username string) ([]models.UserPermissionResponse, error)
	GetUserPermissions(ctx context.Context, userID int) ([]models.UserPermissionResponse, error)
	UpdateUserPermissions(ctx context.Context, isSuperAdmin bool, actorID int, userID int, req models.UpdatePermissionsRequest) error
	GetCatalog(ctx context.Context) ([]CatalogNode, error)
	AutoGenerateForTable(ctx context.Context, tableID int, tableName string) error
	ReconcileAllTablePermissions(ctx context.Context) error
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
func (s *permissionService) GetUserPermissions(ctx context.Context, userID int) ([]models.UserPermissionResponse, error) {
	// Доступ гейтится route-middleware permission.audit.manage (super + admin).

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

	// Читаем из user_permission_overrides (источник точечных прав). LEFT JOIN, а
	// не INNER: каталожные ключи (page.*, header.* ...) - Go-константы, их нет
	// строкой в permissions, и INNER JOIN их выбрасывал -> тумблер слетал после
	// F5 (#867). Ключ берём из up.permission_key, чтобы он был и для каталожных.
	// category/display_name из p.* осмысленны только для динамических table.* -
	// каталожные обогащаем ниже из Go-каталога (единый SoT, #887).
	err := s.db.WithContext(ctx).
		Table("user_permission_overrides up").
		Select("up.permission_key as key, p.category, p.display_name, up.value, u.username as granted_by_name").
		Joins("LEFT JOIN permissions p ON p.key = up.permission_key").
		Joins("LEFT JOIN users u ON u.id = up.granted_by").
		Where("up.user_id = ?", userID).
		Order("up.permission_key").
		Scan(&results).Error
	if err != nil {
		slog.Error("не удалось получить разрешения пользователя", "user_id", userID, "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка получения разрешений")
	}

	// Метаданные каталожных ключей - из Go-каталога (единый источник правды, #887):
	// в permissions их нет, поэтому LEFT JOIN дал бы пустые category/display_name.
	for i := range results {
		if meta, ok := CatalogMeta(results[i].Key); ok {
			results[i].Category = meta.Category
			results[i].DisplayName = meta.DisplayName
		}
	}

	if results == nil {
		results = []models.UserPermissionResponse{}
	}
	return results, nil
}

// UpdateUserPermissions обновляет набор разрешений пользователя (admin-only).
func (s *permissionService) UpdateUserPermissions(ctx context.Context, isSuperAdmin bool, actorID int, userID int, req models.UpdatePermissionsRequest) error {
	// Доступ гейтится route-middleware permission.audit.manage (super + admin).
	// Но super-only ключи (выдача админки, техработы) через override может
	// раздавать только супер-админ - иначе admin поднял бы себе/другим super-права.
	if !isSuperAdmin {
		for _, p := range req.Permissions {
			if IsSuperOnly(p.Key) {
				return echo.NewHTTPError(http.StatusForbidden, "Эти права может выдавать только супер-администратор")
			}
		}
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

	now := time.Now()
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, p := range req.Permissions {
			// Точечные права пишем в user_permission_overrides - именно их читает
			// резолвер (computeSet). Раньше писали в legacy user_permissions, которую
			// резолвер не смотрит, поэтому выдача прав не имела эффекта (#867).
			ov := models.UserPermissionOverride{
				UserID:        userID,
				PermissionKey: p.Key,
				Value:         p.Value,
				GrantedAt:     now,
			}
			if adminID > 0 {
				ov.GrantedBy = &adminID
			}

			// Upsert: update value if exists, create if not
			result := tx.Where("user_id = ? AND permission_key = ?", userID, p.Key).
				Assign(models.UserPermissionOverride{Value: p.Value, GrantedBy: ov.GrantedBy, GrantedAt: now}).
				FirstOrCreate(&ov)
			if result.Error != nil {
				slog.Error("не удалось обновить override прав", "user_id", userID, "key", p.Key, "error", result.Error)
				return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка обновления разрешений")
			}
		}
		return nil
	})
}

// GetCatalog возвращает полный каталог прав: статическое дерево (Catalog) плюс
// динамические права таблиц (table.<slug>.*) из БД под категорией "Таблицы".
// Права таблиц, ушедших в архив или удалённых насовсем, из выдачи убраны
// (#1881): выбирать их в редакторах доступа некому. Скрытие касается ТОЛЬКО
// этой витрины - сами права остаются в БД, продолжают действовать (резолвер и
// middleware каталог не читают) и возвращаются в каталог при восстановлении
// таблицы.
func (s *permissionService) GetCatalog(ctx context.Context) ([]CatalogNode, error) {
	nodes := Catalog()

	var tablePerms []models.Permission
	if err := s.db.WithContext(ctx).
		Where("category = ?", "table").
		// Право видно, только если его таблица СУЩЕСТВУЕТ и активна. Форма
		// EXISTS выбрана ради нерезолвящегося entity_id: осиротевшая ссылка на
		// удалённую таблицу и NULL одинаково не находят строку и одинаково
		// выпадают из витрины - имя такому праву брать неоткуда, и в интерфейсе
		// оно выводилось служебным слагом. Предикат активности тот же, что у
		// самих таблиц (system_table_service): "активна" = is_active = true.
		Where("EXISTS (SELECT 1 FROM system_tables st WHERE st.id = permissions.entity_id AND st.is_active = true)").
		Find(&tablePerms).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка получения каталога прав")
	}
	if len(tablePerms) == 0 {
		return nodes, nil
	}

	byID, bySlug, err := s.tableNameMaps(ctx)
	if err != nil {
		return nil, err
	}

	type tableEntry struct {
		node    CatalogNode
		name    string
		verbIdx int
	}
	entries := make([]tableEntry, 0, len(tablePerms))
	for _, p := range tablePerms {
		label, name, verbIdx := tablePermLabel(p, byID, bySlug)
		entries = append(entries, tableEntry{
			node:    CatalogNode{Key: p.Key, DisplayName: label, Category: CatTables},
			name:    name,
			verbIdx: verbIdx,
		})
	}
	// Группируем права по таблице (имя), внутри - по порядку глаголов.
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].name != entries[j].name {
			return entries[i].name < entries[j].name
		}
		return entries[i].verbIdx < entries[j].verbIdx
	})
	for _, e := range entries {
		nodes = append(nodes, e.node)
	}
	return nodes, nil
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
	{"versions", "Сохранённые версии"},
	{"export", "Экспорт"},
	{"report", "Отчёт по проходам"},
	{"trash", "Корзина"},
	{"delete", "Удаление записи"},
}

// tableVerbTitle -- глагол права таблицы -> человекочитаемое действие.
var tableVerbTitle = func() map[string]string {
	m := make(map[string]string, len(tableVerbs))
	for _, v := range tableVerbs {
		m[v.Verb] = v.Title
	}
	return m
}()

// tableVerbOrder -- порядок глаголов для стабильной сортировки прав таблицы в UI.
var tableVerbOrder = func() map[string]int {
	m := make(map[string]int, len(tableVerbs))
	for i, v := range tableVerbs {
		m[v.Verb] = i
	}
	return m
}()

// tableNameMaps грузит карты "id -> имя" и "slug -> имя" системных таблиц для
// живых лейблов прав. Имя = display_name (или name, если display_name пуст).
func (s *permissionService) tableNameMaps(ctx context.Context) (map[int]string, map[string]string, error) {
	var tables []models.SystemTable
	if err := s.db.WithContext(ctx).Select("id", "name", "display_name").Find(&tables).Error; err != nil {
		return nil, nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка получения каталога прав")
	}
	byID := make(map[int]string, len(tables))
	bySlug := make(map[string]string, len(tables))
	for _, t := range tables {
		name := t.Name
		if t.DisplayName != nil && *t.DisplayName != "" {
			name = *t.DisplayName
		}
		byID[t.ID] = name
		bySlug[t.Name] = name
	}
	return byID, bySlug, nil
}

// tablePermLabel строит человеческий лейбл права таблицы "<имя>: <действие>" из
// ключа table.<slug>.<verb>. Имя берём живым из system_tables (по entity_id, иначе
// по slug) - в UI «КПП №4», а не системное «kpp_4», и переименование сразу видно.
func tablePermLabel(p models.Permission, byID map[int]string, bySlug map[string]string) (label, name string, verbIdx int) {
	rest := strings.TrimPrefix(p.Key, "table.")
	slug, verb := rest, ""
	if i := strings.LastIndex(rest, "."); i >= 0 {
		slug, verb = rest[:i], rest[i+1:]
	}
	name = slug
	resolved := false
	if p.EntityID != nil {
		if n := byID[*p.EntityID]; n != "" {
			name, resolved = n, true
		}
	}
	if !resolved {
		if n := bySlug[slug]; n != "" {
			name = n
		}
	}
	title, ok := tableVerbTitle[verb]
	if !ok {
		title = verb
	}
	idx, ok := tableVerbOrder[verb]
	if !ok {
		idx = len(tableVerbs)
	}
	return fmt.Sprintf("%s: %s", name, title), name, idx
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

// ReconcileAllTablePermissions догенерирует недостающие права table.<slug>.<verb>
// для всех существующих таблиц (идемпотентно). AutoGenerateForTable пишет права лишь
// при создании таблицы, поэтому при добавлении нового глагола в tableVerbs старые
// таблицы остались бы без соответствующего права (его нельзя было бы выдать в дереве).
// Вызывается на старте: за один проход подбирает то, чего не хватает, и молчит, если
// всё на месте.
func (s *permissionService) ReconcileAllTablePermissions(ctx context.Context) error {
	var tables []models.SystemTable
	if err := s.db.WithContext(ctx).Select("id", "name").Find(&tables).Error; err != nil {
		return fmt.Errorf("failed to list tables for permission reconcile: %w", err)
	}

	var existingKeys []string
	if err := s.db.WithContext(ctx).Model(&models.Permission{}).
		Where("category = ?", "table").
		Pluck("key", &existingKeys).Error; err != nil {
		return fmt.Errorf("failed to load existing table permissions: %w", err)
	}
	have := make(map[string]struct{}, len(existingKeys))
	for _, k := range existingKeys {
		have[k] = struct{}{}
	}

	created := 0
	for _, t := range tables {
		for _, v := range tableVerbs {
			key := fmt.Sprintf("table.%s.%s", t.Name, v.Verb)
			if _, ok := have[key]; ok {
				continue
			}
			tableID := t.ID
			perm := models.Permission{
				Key:         key,
				Category:    "table",
				EntityID:    &tableID,
				DisplayName: fmt.Sprintf("%s: %s", t.Name, v.Title),
				ParentKey:   nil,
			}
			if err := s.db.WithContext(ctx).Create(&perm).Error; err != nil {
				return fmt.Errorf("failed to create permission %s: %w", key, err)
			}
			created++
		}
	}
	if created > 0 {
		slog.Info("догенерированы недостающие права таблиц", "created", created)
	}
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
