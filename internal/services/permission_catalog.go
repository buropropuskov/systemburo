package services

import "strings"

// Единый каталог точечных прав (#эпик-прав). Источник правды для UI настройки
// прав и для валидации входящих ключей. Иерархия: категория-заголовок -> листья.
// Динамические table.<slug>.* в статический каталог не входят -- они приходят из
// БД (AutoGenerateForTable) и доклеиваются к ответу /permissions/catalog.
//
// Именование новых ключей продолжает конвенцию permission_keys.go:
//   page.*    -- пункты навигации,
//   header.*  -- кнопки шапки,
//   center.*  -- разделы/кнопки центра заявок,
//   detail.*  -- кнопки/разделы карточек авто/сотрудника (общие для контекстов),
//   section.* -- разделы внутри страниц (владельческие вкладки реестров),
//   guide.*   -- вкладки руководства,
//   news.*    -- управление новостями.

// Новые ключи каталога (существующие переиспользуются из permission_keys.go).
const (
	KeyPageNewApplication = "page.new_application"
	KeyPageAvailable      = "page.available"
	KeyPageTables         = "page.tables"
	KeyPageAnalytics      = "page.analytics"

	KeyPageAdminMonitoring  = "page.admin.monitoring"
	KeyPageAdminDirectories = "page.admin.directories"
	KeyPageAdminTablesCtor  = "page.admin.tables_constructor"

	KeyHeaderReportProblem     = "header.report_problem"
	KeyHeaderCreateApplication = "header.create_application"

	KeyCenterArchive            = "center.archive"
	KeyCenterApplicationHistory = "center.application_history"

	KeyDetailFullHistory      = "detail.full_history"
	KeyDetailOpenApplication  = "detail.open_application"
	KeyDetailBlacklist        = "detail.blacklist"
	KeyDetailEntryExitHistory = "detail.entry_exit_history"
	KeyDetailDocuments        = "detail.documents"
	KeyDetailPassHistory      = "detail.pass_history"

	KeySectionRegistryOrganization = "section.registry.organization"
	KeySectionRegistryCompany      = "section.registry.company"
	KeySectionRegistryAllSystem    = "section.registry.all_system"

	KeyActionGrantAdmin = "action.grant.admin"

	KeyNewsManage = "news.manage"
	KeyGuideUser  = "guide.user"
	KeyGuideAdmin = "guide.admin"
)

// Категории-заголовки для группировки в UI.
const (
	CatNavigation = "Навигация"
	CatHeader     = "Шапка"
	CatCenter     = "Центр заявок"
	CatDetail     = "Карточка авто/сотрудника"
	CatTables     = "Таблицы"
	CatRegistry   = "Сотрудники и автомобили"
	CatAdmin      = "Администрирование"
	CatOverview   = "Обзор и новости"
)

// CatalogNode -- узел каталога прав для UI и валидации.
type CatalogNode struct {
	Key         string        `json:"key"`
	DisplayName string        `json:"display_name"`
	Category    string        `json:"category"`
	SuperOnly   bool          `json:"super_only,omitempty"`
	Children    []CatalogNode `json:"children,omitempty"`
}

// superOnlyKeys -- права, доступные ТОЛЬКО супер-админу. Обычный администратор
// (is_admin) их не получает даже при allowAll.
var superOnlyKeys = map[string]struct{}{
	KeyPageSystemControl: {}, // режим техработ
	KeyActionGrantAdmin:  {}, // выдача тумблера "Администратор" другим
}

// staticCatalog -- статическое дерево прав (без динамических table.*).
func staticCatalog() []CatalogNode {
	return []CatalogNode{
		// Навигация
		{Key: KeyPageCenter, DisplayName: "Центр заявок", Category: CatNavigation},
		{Key: KeyPageNewApplication, DisplayName: "Новая заявка", Category: CatNavigation},
		{Key: KeyPageAvailable, DisplayName: "Доступные мне", Category: CatNavigation},
		{Key: KeyPageTables, DisplayName: "Таблицы", Category: CatNavigation},
		{Key: KeyPageEmployees, DisplayName: "Сотрудники", Category: CatNavigation},
		{Key: KeyPageCars, DisplayName: "Автомобили", Category: CatNavigation},
		{Key: KeyPageAnalytics, DisplayName: "Аналитика", Category: CatNavigation},
		{Key: KeyPageAdmin, DisplayName: "Администрирование", Category: CatNavigation},
		{Key: KeyPageNews, DisplayName: "Обзор и новости", Category: CatNavigation},
		{Key: KeyPagePersonal, DisplayName: "Личный кабинет", Category: CatNavigation},

		// Шапка
		{Key: KeyHeaderReportProblem, DisplayName: "Кнопка «Сообщить о проблеме»", Category: CatHeader},
		{Key: KeyHeaderCreateApplication, DisplayName: "Кнопка «Подать заявку»", Category: CatHeader},

		// Центр заявок
		{Key: KeyCenterArchive, DisplayName: "Раздел «Архив»", Category: CatCenter},
		{Key: KeyCenterApplicationHistory, DisplayName: "Кнопка «История заявки»", Category: CatCenter},
		{Key: KeyActionForwardApplication, DisplayName: "Переслать заявку", Category: CatCenter},
		{Key: KeyActionApproveApplication, DisplayName: "Согласовать заявку", Category: CatCenter},
		{Key: KeyActionExportApplications, DisplayName: "Экспорт заявок", Category: CatCenter},

		// Карточка авто/сотрудника (общие действия; где кнопка уместна -- определяет контекст на фронте)
		{Key: KeyDetailFullHistory, DisplayName: "Кнопка «Полная история»", Category: CatDetail},
		{Key: KeyDetailOpenApplication, DisplayName: "Кнопка «Открыть заявку»", Category: CatDetail},
		{Key: KeyDetailBlacklist, DisplayName: "Кнопка «В ЧС»", Category: CatDetail},
		{Key: KeyDetailEntryExitHistory, DisplayName: "Раздел «История въездов и выездов»", Category: CatDetail},
		{Key: KeyDetailDocuments, DisplayName: "Раздел «Документы»", Category: CatDetail},
		{Key: KeyDetailPassHistory, DisplayName: "Раздел «История проходов»", Category: CatDetail},

		// Сотрудники и автомобили
		{Key: KeyEntityCarsRead, DisplayName: "Автомобили: просмотр", Category: CatRegistry},
		{Key: KeyEntityCarsWrite, DisplayName: "Автомобили: изменение", Category: CatRegistry},
		{Key: KeyEntityCarsDelete, DisplayName: "Автомобили: удаление", Category: CatRegistry},
		{Key: KeyEntityEmployeesRead, DisplayName: "Сотрудники: просмотр", Category: CatRegistry},
		{Key: KeyEntityEmployeesWrite, DisplayName: "Сотрудники: изменение", Category: CatRegistry},
		{Key: KeyEntityEmployeesDel, DisplayName: "Сотрудники: удаление", Category: CatRegistry},
		{Key: KeySectionRegistryOrganization, DisplayName: "Раздел «...организации»", Category: CatRegistry},
		{Key: KeySectionRegistryCompany, DisplayName: "Раздел «...компании»", Category: CatRegistry},
		{Key: KeySectionRegistryAllSystem, DisplayName: "Раздел «Все ... системы»", Category: CatRegistry},

		// Администрирование
		{Key: KeyPageAdminUsers, DisplayName: "Раздел «Пользователи»", Category: CatAdmin},
		{Key: KeyPageAdminMonitoring, DisplayName: "Раздел «Мониторинг запросов»", Category: CatAdmin},
		{Key: KeyPageAdminFeedback, DisplayName: "Раздел «Обратная связь»", Category: CatAdmin},
		{Key: KeyPageBlacklist, DisplayName: "Раздел «Чёрный список»", Category: CatAdmin},
		{Key: KeyPageAdminDirectories, DisplayName: "Раздел «Справочники»", Category: CatAdmin},
		{Key: KeyPageAdminTablesCtor, DisplayName: "Раздел «Конструктор таблиц»", Category: CatAdmin},
		{Key: KeyAuditRead, DisplayName: "Журнал отказов в доступе", Category: CatAdmin},
		{Key: KeyAuditManage, DisplayName: "Управление ролями и группами", Category: CatAdmin},
		{Key: KeyActionBanUser, DisplayName: "Блокировка пользователей", Category: CatAdmin},
		{Key: KeyActionGrantAdmin, DisplayName: "Выдача прав администратора", Category: CatAdmin, SuperOnly: true},
		{Key: KeyPageSystemControl, DisplayName: "Режим техработ", Category: CatAdmin, SuperOnly: true},

		// Обзор и новости
		{Key: KeyNewsManage, DisplayName: "Управление новостями и объявлениями", Category: CatOverview},
		{Key: KeyGuideUser, DisplayName: "Руководство пользователя", Category: CatOverview},
		{Key: KeyGuideAdmin, DisplayName: "Руководство администратора", Category: CatOverview},
	}
}

// Catalog возвращает статическое дерево прав каталога.
func Catalog() []CatalogNode {
	return staticCatalog()
}

// catalogKeySet -- множество всех статических ключей каталога (для O(1) валидации).
var catalogKeySet = func() map[string]struct{} {
	m := make(map[string]struct{})
	for _, n := range staticCatalog() {
		m[n.Key] = struct{}{}
		for _, c := range n.Children {
			m[c.Key] = struct{}{}
		}
	}
	return m
}()

// AllCatalogKeys возвращает плоский список всех статических ключей каталога.
func AllCatalogKeys() []string {
	keys := make([]string, 0, len(catalogKeySet))
	for k := range catalogKeySet {
		keys = append(keys, k)
	}
	return keys
}

// IsSuperOnly сообщает, что ключ доступен только супер-админу.
func IsSuperOnly(key string) bool {
	_, ok := superOnlyKeys[key]
	return ok
}

// IsCatalogKey сообщает, что ключ есть в статическом каталоге.
func IsCatalogKey(key string) bool {
	_, ok := catalogKeySet[key]
	return ok
}

// IsValidKey сообщает, что ключ валиден для назначения: либо статический из
// каталога, либо динамический table.<slug>.* (существование конкретной table.*
// проверяется отдельно по БД там, где это критично).
func IsValidKey(key string) bool {
	if IsCatalogKey(key) {
		return true
	}
	return strings.HasPrefix(key, PrefixTable)
}
