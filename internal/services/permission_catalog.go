package services

import (
	"sort"
	"strings"
)

// Единый каталог точечных прав (#эпик-прав). Источник правды для UI настройки
// прав и для валидации входящих ключей. Иерархия: категория-заголовок -> листья.
// Динамические table.<slug>.* в статический каталог не входят -- они приходят из
// БД (AutoGenerateForTable) и доклеиваются к ответу /permissions/catalog.
//
// Именование новых ключей продолжает конвенцию permission_keys.go:
//   page.*        -- пункты навигации,
//   header.*      -- кнопки шапки,
//   center.*      -- разделы/кнопки центра заявок,
//   detail.*      -- кнопки/разделы карточек авто/сотрудника (общие для контекстов),
//   section.*     -- разделы внутри страниц (владельческие вкладки реестров),
//   application.* -- послабления в правилах подачи заявки,
//   guide.*       -- вкладки руководства.

// Новые ключи каталога (существующие переиспользуются из permission_keys.go).
const (
	KeyPageNewApplication = "page.new_application"
	KeyPageAvailable      = "page.available"
	KeyPageTables         = "page.tables"

	KeyPageAdminMonitoring = "page.admin.monitoring"
	// Журнал доступа к персональным данным (152-ФЗ). Отдельно от permission.audit.read:
	// тот про отказы в доступе, а здесь видно, кто и когда смотрел паспорта (#1472).
	KeyPageAdminPDAudit     = "page.admin.pd_audit"
	KeyPageAdminDirectories = "page.admin.directories"
	KeyPageAdminTablesCtor  = "page.admin.tables_constructor"
	// Раздел «Файловый архив» (#1615): состояние выгрузки бланков на диск, её
	// настройки и выгрузка файлов. Слово «архив» отдельно уже занято архивными
	// заявками, поэтому раздел называется файловым и в ключе, и в интерфейсе.
	KeyPageAdminFileArchive = "page.admin.file_archive"
	// Раздел «Настройки» (#7): администраторы получают ключ через adminAll (не
	// super-only), точечно снимается личным deny-override у конкретного человека.
	KeyPageAdminSettings = "page.admin.settings"

	KeyHeaderReportProblem     = "header.report_problem"
	KeyHeaderCreateApplication = "header.create_application"

	KeyCenterArchive            = "center.archive"
	KeyCenterApplicationHistory = "center.application_history"

	KeyDetailFullHistory      = "detail.full_history"
	KeyDetailOpenApplication  = "detail.open_application"
	KeyDetailEntryExitHistory = "detail.entry_exit_history"
	KeyDetailDocuments        = "detail.documents"
	// KeyDetailDocumentsExport - выгрузка документов человека (серия и номер паспорта,
	// номер патента, иное разрешение) в заполненный бланк заявки. Право парное к
	// detail.documents и работает только вместе с ним: видеть документы на экране
	// карточки и вынести их файлом наружу - действия разного веса, и второе не должно
	// доставаться каждому, кто может подать заявку. Отзыв detail.documents гасит и
	// выгрузку, отдельным действием ходить не надо.
	//
	// В базовую роль не входит намеренно: администраторы получают ключ через adminAll,
	// остальным он выдаётся точечно. Без права бланк уходит с прочерками в этих
	// ячейках, а источник "сохранённый файл" закрывается - лежащая на диске копия
	// собрана с документами, и обезличить её при отдаче нечем.
	KeyDetailDocumentsExport = "detail.documents.export"

	KeySectionRegistryOrganization = "section.registry.organization"
	KeySectionRegistryCompany      = "section.registry.company"
	KeySectionRegistryAllSystem    = "section.registry.all_system"

	// KeyApplicationOrganizationOverride разрешает подать заявку от организации или
	// компании, которая не указана в профиле подающего (#1437): выбрать чужую запись
	// справочника или ввести наименование, которого в нём ещё нет. Без права поля
	// организации и компании в форме заявки остаются нередактируемыми, и подмена,
	// сделанная в обход формы, отклоняется на бэкенде.
	KeyApplicationOrganizationOverride = "application.organization.override"

	// KeyApplicationOrganizationModerate разрешает разбирать организации и компании,
	// заведённые из заявки со статусом «на проверке» (#1437): подтвердить запись,
	// исправить её наименование или привязать заявки к уже существующей записи.
	// Право рассчитано на принимающих: администраторы получают его через adminAll.
	// Неймспейс общий с override: разбор идёт из детали заявки, а не из админки
	// справочника, и права этой пары назначают вместе.
	KeyApplicationOrganizationModerate = "application.organization.moderate"

	KeyActionGrantAdmin = "action.grant.admin"

	// KeyActionRotatePasswords - запуск смены паролей всем работникам вручную
	// (#1910). Отдельно от page.admin.settings намеренно: правка телефона бюро и
	// сброс паролей всей организации - действия разного веса, и второе не должно
	// достаться каждому, кто может поменять первое.
	KeyActionRotatePasswords = "action.password.rotate_all"

	// KeyUserImpersonate - вход в систему от имени другого пользователя (#1912):
	// разбор проблемной учётной записи без знания её пароля. Не super-only
	// намеренно: администраторы и так меняют работникам пароли (PUT
	// /users/:username/password), то есть уже могут войти под чужим именем - только
	// молча и с потерей следа в журнале. Сделать право недоступным администратору
	// значило бы оставить их на этой практике. Рядовому пользователю право не
	// достаётся, пока его не выдадут явно, а войти от имени более полномочного не
	// даёт сам сервис.
	KeyUserImpersonate = "user.impersonate"

	// KeyActionManageFileArchive - правка настроек файлового архива: рубильник,
	// шаблоны раскладки, пороги места. Отделено от просмотра раздела: смотреть
	// состояние выгрузки полезно и дежурному, а менять раскладку - нет, сменённый
	// шаблон разводит новые файлы мимо тех, что уже лежат на диске.
	KeyActionManageFileArchive = "action.manage.file_archive"

	// KeyActionDownloadFileArchive - выгрузка файлов архива на рабочий компьютер
	// (ZIP за период, ZIP заявки, отдельный бланк). Право отдельное, потому что это
	// вынос персональных данных за пределы системы разом и помногу.
	KeyActionDownloadFileArchive = "action.download.file_archive"

	KeyGuideUser  = "guide.user"
	KeyGuideAdmin = "guide.admin"
	KeyGuideGuard = "guide.guard"
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
	// Description -- необязательная подсказка для узлов, чьё название само по
	// себе неоднозначно (#1998: page.admin выглядит зонтиком над всем разделом
	// администрирования, а на деле открывает два пункта меню и россыпь действий).
	// Показывается всплывающей подсказкой в редакторах прав; для большинства
	// узлов, где DisplayName самодостаточен, остаётся пустой.
	Description string `json:"description,omitempty"`
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
		{Key: KeyPageStatistics, DisplayName: "Аналитика", Category: CatNavigation},
		// До #1982 page.admin открывал весь раздел администрирования и название
		// "Администрирование" было точным. После переезда справочников за ним
		// остались только пункты меню «Руководство» и «Обработка данных» плюс
		// точечные действия по системе (не отдельный раздел) -- старое название
		// вводило раздающего права в заблуждение (#1998). Справочники, пользователи,
		// настройки и остальные разделы администрирования выдаются своими ключами
		// page.admin.* и этим правом не открываются.
		{
			Key:         KeyPageAdmin,
			DisplayName: "Общие административные действия",
			Category:    CatNavigation,
			Description: "Открывает пункты меню «Руководство» и «Обработка данных», а также " +
				"административные действия в разных разделах системы: привязка файлов к заявке, " +
				"перенос и отвязка записей в системных таблицах, рассылка уведомлений, тайм-слоты " +
				"бюро, журнал аудита, мониторинг запросов, сброс онбординга пользователю и настройки " +
				"согласия на обработку персональных данных. Раздел «Справочники» им не открывается " +
				"- это отдельное право page.admin.directories.",
		},
		{Key: KeyPageNews, DisplayName: "Обзор и новости", Category: CatNavigation},
		{Key: KeyPagePersonal, DisplayName: "Личный кабинет", Category: CatNavigation},

		// Шапка
		{Key: KeyHeaderReportProblem, DisplayName: "Кнопка «Сообщить о проблеме»", Category: CatHeader},
		{Key: KeyHeaderCreateApplication, DisplayName: "Кнопка «Подать заявку»", Category: CatHeader},

		// Центр заявок
		{Key: KeyCenterArchive, DisplayName: "Раздел «Архив»", Category: CatCenter},
		{Key: KeyCenterApplicationHistory, DisplayName: "Кнопка «История заявки»", Category: CatCenter},
		{Key: KeyActionForwardApplication, DisplayName: "Переслать заявку", Category: CatCenter},
		{Key: KeyActionSupplementApplication, DisplayName: "Дополнить заявку", Category: CatCenter},
		{Key: KeyActionApproveApplication, DisplayName: "Согласовать заявку", Category: CatCenter},
		{Key: KeyActionExportApplications, DisplayName: "Экспорт заявок", Category: CatCenter},
		{Key: KeyActionImportList, DisplayName: "Импорт списка из бланка", Category: CatCenter},
		{Key: KeyApplicationOrganizationOverride, DisplayName: "Подача заявки от другой организации", Category: CatCenter},
		{Key: KeyApplicationOrganizationModerate, DisplayName: "Разбор организаций на проверке", Category: CatCenter},

		// Карточка авто/сотрудника (общие действия; где кнопка уместна -- определяет контекст на фронте)
		{Key: KeyDetailFullHistory, DisplayName: "Кнопка «Полная история»", Category: CatDetail},
		{Key: KeyDetailOpenApplication, DisplayName: "Кнопка «Открыть заявку»", Category: CatDetail},
		{Key: KeyDetailEntryExitHistory, DisplayName: "Раздел «История въездов и выездов»", Category: CatDetail},
		{
			Key:         KeyDetailDocuments,
			DisplayName: "Раздел «Документы»",
			Category:    CatDetail,
			Children: []CatalogNode{
				{
					Key:         KeyDetailDocumentsExport,
					DisplayName: "Документы: выгрузка в бланк",
					Category:    CatDetail,
					Description: "Разрешает скачивать бланк заявки с паспортными данными, номером " +
						"патента и иным разрешением участников, а также забирать сохранённые копии " +
						"бланков из файлового архива. Без права бланк скачивается с прочерками в этих " +
						"ячейках. Работает только вместе с правом на раздел «Документы»: закрыт просмотр " +
						"на экране - закрыта и выгрузка.",
				},
			},
		},

		// Сотрудники и автомобили
		{Key: KeyEntityCarsRead, DisplayName: "Автомобили: просмотр", Category: CatRegistry},
		{Key: KeyEntityCarsWrite, DisplayName: "Автомобили: изменение", Category: CatRegistry},
		{Key: KeyEntityCarsDelete, DisplayName: "Автомобили: удаление", Category: CatRegistry},
		{Key: KeyEntityCarsManualAdd, DisplayName: "Автомобили: добавить вручную в таблицу", Category: CatRegistry},
		{Key: KeyEntityEmployeesRead, DisplayName: "Сотрудники: просмотр", Category: CatRegistry},
		{Key: KeyEntityEmployeesWrite, DisplayName: "Сотрудники: изменение", Category: CatRegistry},
		{Key: KeyEntityEmployeesDel, DisplayName: "Сотрудники: удаление", Category: CatRegistry},
		{Key: KeyEntityEmployeesManualAdd, DisplayName: "Сотрудники: добавить вручную в таблицу", Category: CatRegistry},
		{Key: KeySectionRegistryOrganization, DisplayName: "Раздел «...организации»", Category: CatRegistry},
		{Key: KeySectionRegistryCompany, DisplayName: "Раздел «...компании»", Category: CatRegistry},
		{Key: KeySectionRegistryAllSystem, DisplayName: "Раздел «Все ... системы»", Category: CatRegistry},

		// Администрирование
		{Key: KeyPageAdminUsers, DisplayName: "Раздел «Пользователи»", Category: CatAdmin},
		{Key: KeyPageAdminMonitoring, DisplayName: "Раздел «Мониторинг запросов»", Category: CatAdmin},
		{Key: KeyPageAdminPDAudit, DisplayName: "Журнал доступа к персональным данным", Category: CatAdmin},
		{Key: KeyPageAdminFeedback, DisplayName: "Раздел «Обратная связь»", Category: CatAdmin},
		{Key: KeyPageBlacklist, DisplayName: "Раздел «Чёрный список»", Category: CatAdmin},
		{Key: KeyPageAdminDirectories, DisplayName: "Раздел «Справочники»", Category: CatAdmin},
		{Key: KeyPageAdminTablesCtor, DisplayName: "Раздел «Конструктор таблиц»", Category: CatAdmin},
		{Key: KeyPageAdminFileArchive, DisplayName: "Раздел «Файловый архив»", Category: CatAdmin},
		{Key: KeyPageAdminSettings, DisplayName: "Раздел «Настройки»", Category: CatAdmin},
		{Key: KeyActionManageFileArchive, DisplayName: "Файловый архив: настройка", Category: CatAdmin},
		{Key: KeyActionDownloadFileArchive, DisplayName: "Файловый архив: выгрузка файлов", Category: CatAdmin},
		{Key: KeyAuditRead, DisplayName: "Журнал отказов в доступе", Category: CatAdmin},
		{Key: KeyAuditManage, DisplayName: "Управление ролями и группами", Category: CatAdmin},
		{Key: KeyActionBanUser, DisplayName: "Блокировка пользователей", Category: CatAdmin},
		{Key: KeyUserImpersonate, DisplayName: "Вход от имени пользователя", Category: CatAdmin},
		{Key: KeyActionGrantAdmin, DisplayName: "Выдача прав администратора", Category: CatAdmin, SuperOnly: true},
		{Key: KeyActionRotatePasswords, DisplayName: "Смена паролей всем работникам", Category: CatAdmin},
		{Key: KeyPageSystemControl, DisplayName: "Режим техработ", Category: CatAdmin, SuperOnly: true},

		// Обзор и новости (управление новостями -- в Администрировании, /admin/news).
		// Три раздела руководства (пользователь/охранник/админ) гейтятся по guide.<role>:
		// GET /guide/sections отдаёт только разделы, на которые есть право (guide_service.ListForUser).
		{Key: KeyGuideUser, DisplayName: "Руководство пользователя", Category: CatOverview},
		{Key: KeyGuideGuard, DisplayName: "Руководство охранника", Category: CatOverview},
		{Key: KeyGuideAdmin, DisplayName: "Руководство администратора", Category: CatOverview},
	}
}

// Catalog возвращает статическое дерево прав каталога.
func Catalog() []CatalogNode {
	return staticCatalog()
}

// catalogByKey -- плоская карта "ключ -> узел" статического каталога. Единый
// источник правды для каталожных ключей: их существования (O(1) валидация) и их
// метаданных (DisplayName/Category). В таблице permissions каталожных ключей нет
// (там только динамические table.*), поэтому метаданные берём отсюда, а не из БД (#887).
var catalogByKey = func() map[string]CatalogNode {
	m := make(map[string]CatalogNode)
	for _, n := range staticCatalog() {
		m[n.Key] = n
		for _, c := range n.Children {
			m[c.Key] = c
		}
	}
	return m
}()

// AllCatalogKeys возвращает плоский список всех статических ключей каталога.
func AllCatalogKeys() []string {
	keys := make([]string, 0, len(catalogByKey))
	for k := range catalogByKey {
		keys = append(keys, k)
	}
	return keys
}

// IsSuperOnly сообщает, что ключ доступен только супер-админу.
func IsSuperOnly(key string) bool {
	_, ok := superOnlyKeys[key]
	return ok
}

// SuperOnlyKeys возвращает отсортированный список ключей, доступных только
// супер-админу. Нужен ответу /permissions/my (#1997): PermissionSet.Has режет
// эти ключи для обычного admin, но раньше это не отражалось в Denied -- фронтовый
// стор в admin-режиме считал ключ выданным, если его нет в denied.
func SuperOnlyKeys() []string {
	keys := make([]string, 0, len(superOnlyKeys))
	for k := range superOnlyKeys {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// IsCatalogKey сообщает, что ключ есть в статическом каталоге.
func IsCatalogKey(key string) bool {
	_, ok := catalogByKey[key]
	return ok
}

// CatalogMeta возвращает узел каталога по ключу. Источник правды для метаданных
// каталожного ключа (DisplayName/Category) -- этот Go-каталог; код чтения прав
// обогащает им ответы, не полагаясь на таблицу permissions (#887).
func CatalogMeta(key string) (CatalogNode, bool) {
	n, ok := catalogByKey[key]
	return n, ok
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
