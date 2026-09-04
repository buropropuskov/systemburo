/**
 * Разделы системы для навигации.
 *
 * Один список на два потребителя: колонку «Администрирование» в меню и группу «Разделы»
 * в сквозном поиске. Держать их порознь нельзя -- новый раздел появлялся бы в меню и
 * молча отсутствовал в поиске, а именно за ответом «где это искать» в поиск и приходят.
 *
 * `permission` -- ключ права; пункт показывается, если оно есть у пользователя.
 * `keywords` -- слова, по которым раздел ищется помимо названия: за отчётами приходят
 * со словом «выгрузка», а раздел называется иначе.
 * `query` -- параметры адреса для разделов, которые живут вкладкой внутри страницы.
 */
export const ADMIN_GROUPS = [
  {
    title: 'Доступ и роли',
    items: [
      { label: 'Пользователи', icon: 'users', path: '/admin/users', permission: 'page.admin.users' },
      { label: 'Роли', icon: 'roles', path: '/admin/roles', permission: 'permission.audit.manage' },
      { label: 'Группы прав', icon: 'permission-groups', path: '/admin/permission-groups', permission: 'permission.audit.manage' },
      { label: 'Журнал отказов', icon: 'access-denials', path: '/admin/access-denials', permission: 'permission.audit.read' },
      { label: 'Доступ к перс. данным', icon: 'access-denials', path: '/admin/pd-audit', permission: 'page.admin.pd_audit' },
      { label: 'Чёрный список', icon: 'blacklist', path: '/admin/blacklist', permission: 'page.admin.blacklist' },
    ],
  },
  {
    title: 'Справочники',
    items: [
      { label: 'Организации', icon: 'organizations', path: '/admin/organizations', permission: 'page.admin.directories' },
      { label: 'Компании', icon: 'companies', path: '/admin/companies', permission: 'page.admin.directories' },
      { label: 'Места разгрузки', icon: 'unload-places', path: '/admin/unload-places', permission: 'page.admin.directories' },
      { label: 'Форматы номеров', icon: 'number-formats', path: '/admin/number-formats', permission: 'page.admin.directories' },
      { label: 'Гражданства', icon: 'citizenship', path: '/admin/citizenship', permission: 'page.admin.directories' },
      { label: 'Марки авто', icon: 'marks', path: '/admin/marks', permission: 'page.admin.directories' },
      { label: 'Типы вложений', icon: 'attachment-types', path: '/admin/attachment-types', permission: 'page.admin.directories' },
      { label: 'Типы пользователей', icon: 'user-types', path: '/admin/user-types', permission: 'page.admin.directories' },
      { label: 'Принимающие', icon: 'approvers', path: '/admin/approvers', permission: 'page.admin.directories' },
      { label: 'Документы', icon: 'documents', path: '/admin/documents', permission: 'page.admin.directories' },
      { label: 'Новости и объявления', icon: 'news', path: '/admin/news', permission: 'page.admin.directories' },
      { label: 'Руководство', icon: 'guide', path: '/admin/guide', permission: 'page.admin' },
    ],
  },
  {
    title: 'Система',
    items: [
      { label: 'Настройки', icon: 'settings', path: '/admin/settings', permission: 'page.admin.settings' },
      { label: 'Обработка данных', icon: 'data-processing', path: '/admin/data-processing', permission: 'page.admin' },
      { label: 'Конструктор таблиц', icon: 'table-constructor', path: '/table-constructor', permission: 'page.admin.tables_constructor' },
      { label: 'Техработы', icon: 'system-control', path: '/admin/system-control', permission: 'page.admin.system_control' },
      { label: 'Файловый архив', icon: 'file-archive', path: '/admin/file-archive', permission: 'page.admin.file_archive' },
    ],
  },
  {
    title: 'Аудит и связь',
    items: [
      { label: 'Обратная связь', icon: 'feedback', path: '/admin/feedback', permission: 'page.admin.feedback' },
      { label: 'Мониторинг запросов', icon: 'requests', path: '/admin/requests', permission: 'page.admin.monitoring' },
    ],
  },
];

/**
 * Основные разделы (рельс меню). Пункты без `permission` доступны любому авторизованному
 * -- ровно как в самом меню, где они не закрыты гейтом.
 */
export const MAIN_SECTIONS = [
  { label: 'Центр заявок', icon: 'center', path: '/center', permission: 'page.center' },
  { label: 'Новая заявка', icon: 'new-application', path: '/new-application', permission: 'page.new_application' },
  { label: 'Доступные мне', icon: 'available', path: '/accessible-attachments' },
  { label: 'Сотрудники', icon: 'employees', path: '/employeesview', permission: 'page.employees' },
  { label: 'Автомобили', icon: 'cars', path: '/carsview', permission: 'page.cars' },
  { label: 'Аналитика', icon: 'statistics', path: '/analytics', permission: 'page.statistics' },
  {
    // Пока раздел живёт только в поиске: своего пункта в рельсе у него нет, NavMenu
    // упёрся в порог размера файла и не может вырасти (см. issue про его разгрузку).
    label: 'Отчёты',
    icon: 'analytics',
    path: '/analytics',
    query: { tab: 'reports' },
    permission: 'page.statistics',
    keywords: ['отчёт', 'отчёты', 'выгрузка', 'экспорт', 'excel', 'сводка'],
  },
  { label: 'Обзор и новости', icon: 'news', path: '/news', permission: 'page.news' },
  { label: 'Личный кабинет', icon: 'personal-cabinet', path: '/personal-cabinet', permission: 'page.personal_cabinet' },
];
