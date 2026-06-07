/**
 * Список всех известных permission_key с описаниями.
 * Соответствует константам в internal/services/permission_keys.go.
 *
 * Naming convention (#187a):
 *   page.<route>                     -- маршруты страниц
 *   tab.<name>                       -- вкладки внутри страниц
 *   component.<name>.<verb>          -- компоненты
 *   action.<verb>.<entity>           -- действия
 *   entity.<name>.<crud>             -- CRUD сущности
 *   table.<slug>.<verb>              -- динамические таблицы (auto-generate)
 *   permission.audit.*               -- журналы и аудит
 *
 * Динамические table.* ключи создаются на бэке при создании таблицы и здесь
 * не перечисляются.
 */
export const ALL_PERMISSION_KEYS = [
  { value: 'page.center', description: 'Страница «Центр заявок»' },
  { value: 'page.cars', description: 'Страница «Автомобили»' },
  { value: 'page.employees', description: 'Страница «Сотрудники»' },
  { value: 'page.statistics', description: 'Страница «Статистика»' },
  { value: 'page.reports', description: 'Страница «Отчёты»' },
  { value: 'page.news', description: 'Страница «Обзор и новости»' },
  { value: 'page.admin', description: 'Раздел «Админка» (общий доступ)' },
  { value: 'page.personal_cabinet', description: 'Личный кабинет' },
  { value: 'page.admin.system_control', description: 'Системное управление (техработы)' },
  { value: 'page.admin.users', description: 'Управление пользователями' },
  { value: 'page.admin.feedback', description: 'Обратная связь' },
  { value: 'page.admin.blacklist', description: 'Чёрный список (машины и люди)' },

  { value: 'entity.cars.read', description: 'Просмотр автомобилей' },
  { value: 'entity.cars.write', description: 'Создание и редактирование автомобилей' },
  { value: 'entity.cars.delete', description: 'Удаление автомобилей' },
  { value: 'entity.employees.read', description: 'Просмотр сотрудников' },
  { value: 'entity.employees.write', description: 'Создание и редактирование сотрудников' },
  { value: 'entity.employees.delete', description: 'Удаление сотрудников' },

  { value: 'action.export.applications', description: 'Экспорт заявок' },
  { value: 'action.approve.application', description: 'Согласование заявок' },
  { value: 'action.forward.application', description: 'Пересылка заявок на согласование' },
  { value: 'action.ban.user', description: 'Блокировка пользователей' },

  { value: 'permission.audit.read', description: 'Просмотр журнала отказов и аудита' },
  { value: 'permission.audit.manage', description: 'Очистка и архивация журнала' },
];
