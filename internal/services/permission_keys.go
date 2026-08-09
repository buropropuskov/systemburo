package services

// Permission key naming convention (#187):
//
//   page.<route>                     -- маршруты страниц (page.center, page.cars).
//   tab.<name>                       -- вкладки внутри страниц (tab.applications).
//   component.<name>.<verb>          -- компоненты (component.vehicle_history.export).
//   action.<verb>.<entity>           -- действия (action.delete.employee).
//   entity.<name>.<crud>             -- CRUD сущности (entity.cars.read|write|delete).
//   table.<slug>.<verb>              -- динамические таблицы (генерируются AutoGenerateForTable).
//
// Ключи зашиты как константы здесь и используются в middleware/handler-ах.
// Каталог валидных ключей передаётся фронту через GET /permissions/keys
// для построения дерева в UI.

// Префиксы permission keys (по namespace).
const (
	PrefixPage      = "page."
	PrefixTab       = "tab."
	PrefixComponent = "component."
	PrefixAction    = "action."
	PrefixEntity    = "entity."
	PrefixTable     = "table."
	PrefixAudit     = "permission.audit."
)

// Page-level keys (страницы навигационного меню).
const (
	KeyPageCenter        = "page.center"
	KeyPageCars          = "page.cars"
	KeyPageEmployees     = "page.employees"
	KeyPageStatistics    = "page.statistics"
	KeyPageReports       = "page.reports"
	KeyPageNews          = "page.news"
	KeyPageAdmin         = "page.admin"
	KeyPagePersonal      = "page.personal_cabinet"
	KeyPageSystemControl = "page.admin.system_control"
	KeyPageAdminUsers    = "page.admin.users"
	KeyPageAdminFeedback = "page.admin.feedback"
	KeyPageBlacklist     = "page.admin.blacklist"
)

// Entity-level CRUD keys.
const (
	KeyEntityCarsRead           = "entity.cars.read"
	KeyEntityCarsWrite          = "entity.cars.write"
	KeyEntityCarsDelete         = "entity.cars.delete"
	KeyEntityCarsManualAdd      = "entity.cars.manual_add"
	KeyEntityEmployeesRead      = "entity.employees.read"
	KeyEntityEmployeesWrite     = "entity.employees.write"
	KeyEntityEmployeesDel       = "entity.employees.delete"
	KeyEntityEmployeesManualAdd = "entity.employees.manual_add"
)

// Action-level keys (отдельные действия, не входящие в стандартный CRUD).
const (
	KeyActionExportApplications = "action.export.applications"
	// KeyActionImportList - массовый ввод участников/машин из заполненного Excel-бланка
	// (blank-import). Не super-only: администраторы получают его через adminAll, обычным
	// пользователям выдаётся точечно - импорт обходит форму подачи целиком, поэтому
	// закрыт правом по умолчанию.
	KeyActionImportList         = "action.import.list"
	KeyActionApproveApplication = "action.approve.application"
	KeyActionForwardApplication = "action.forward.application"
	// KeyActionSupplementApplication - дополнить уже поданную заявку (#1685). Право не
	// единственный гейт: сервис всё равно требует, чтобы дополняющий был автором заявки.
	KeyActionSupplementApplication = "action.supplement.application"
	KeyActionBanUser               = "action.ban.user"
)

// Audit-level keys (просмотр и управление журналами).
const (
	KeyAuditRead   = "permission.audit.read"
	KeyAuditManage = "permission.audit.manage"
)

// AllStaticKeys -- список всех известных static keys (без table.* которые auto-generate).
// Используется для валидации входящих запросов и для seed дефолтных групп.
func AllStaticKeys() []string {
	return []string{
		KeyPageCenter,
		KeyPageCars,
		KeyPageEmployees,
		KeyPageStatistics,
		KeyPageReports,
		KeyPageNews,
		KeyPageAdmin,
		KeyPagePersonal,
		KeyPageSystemControl,
		KeyPageAdminUsers,
		KeyPageAdminFeedback,
		KeyPageBlacklist,
		KeyEntityCarsRead,
		KeyEntityCarsWrite,
		KeyEntityCarsDelete,
		KeyEntityCarsManualAdd,
		KeyEntityEmployeesRead,
		KeyEntityEmployeesWrite,
		KeyEntityEmployeesDel,
		KeyEntityEmployeesManualAdd,
		KeyActionExportApplications,
		KeyActionApproveApplication,
		KeyActionForwardApplication,
		KeyActionSupplementApplication,
		KeyActionBanUser,
		KeyAuditRead,
		KeyAuditManage,
	}
}
