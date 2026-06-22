/**
 * Карта контекст (source) -> действие -> ключ права для
 * VehicleDetailsModal и EmployeeDetailsModal.
 *
 * Значения:
 *   string  — permission key, требуемый для показа действия
 *   true    — действие доступно без отдельного права (по контексту/роли)
 *   false   — действие недоступно в этом контексте
 *
 * Источники (source), в которых может быть открыта модалка:
 *   'application'  — открыта из детали заявки (ApplicationDetail)
 *   'carstable'    — таблица Т/С внутри центра заявок
 *   'facttable'    — таблица "по факту" в центре
 *   'carsview'     — страница «Автомобили»
 *   'employeesview'— страница «Сотрудники»
 *   'employeeslist'— список сотрудников внутри заявки
 *   'peopletable'  — таблица людей в центре заявок
 *   'trash'        — корзина
 *   'blacklist'    — раздел «Чёрный список»
 *   'general'      — прочие/неопределённые контексты
 *
 * Действия:
 *   history         — кнопка «Полная история» (история действий с сущностью)
 *   openApplication — кнопка «Открыть заявку» (переход к связанной заявке)
 *   blacklist       — кнопка «В ЧС» / управление чёрным списком
 *   entryExit       — вкладка/секция «Въезд/Выезд» (история проходов на территорию)
 *   documents       — раздел «Документы» (прикреплённые документы сущности)
 *   passHistory     — История пропусков (выданные пропуска)
 */

/**
 * Общие действия для автомобиля по контексту.
 * @type {Record<string, Record<string, string|boolean>>}
 */
export const VEHICLE_MODAL_ACTIONS = {
  application: {
    // Открыта из деталей заявки: история есть, переход в заявку не нужен (уже в ней)
    history:         'entity.cars.read',
    openApplication: false,
    blacklist:       'page.admin.blacklist',
    entryExit:       'entity.cars.read',
    documents:       false,
    passHistory:     false,
  },
  carstable: {
    history:         'entity.cars.read',
    openApplication: 'detail.open_application', // есть application_id; гейт по detail.open_application (базовая роль)
    blacklist:       'page.admin.blacklist',
    entryExit:       'entity.cars.read',
    documents:       false,
    passHistory:     false,
  },
  facttable: {
    history:         'entity.cars.read',
    openApplication: 'detail.open_application',
    blacklist:       'page.admin.blacklist',
    entryExit:       'entity.cars.read',
    documents:       false,
    passHistory:     false,
  },
  carsview: {
    // Страница «Автомобили»: нет связи с заявкой, документы и история пропусков доступны
    history:         'entity.cars.read',
    openApplication: false,
    blacklist:       'page.admin.blacklist',
    entryExit:       'entity.cars.read',
    // Документы доступны по detail.documents (есть в базовой роли) - владелец видит
    // документы своих машин; админ/супер - всегда.
    documents:       'detail.documents',
    passHistory:     'entity.cars.read',
  },
  trash: {
    // Корзина: только чтение, без ЧС и действий
    history:         'entity.cars.read',
    openApplication: false,
    blacklist:       false,
    entryExit:       false,
    documents:       false,
    passHistory:     false,
  },
  blacklist: {
    // Открыта из раздела ЧС: переход в заявку не нужен, добавление в ЧС не нужно
    history:         'page.admin.blacklist',
    openApplication: false,
    blacklist:       false,
    entryExit:       false,
    documents:       false,
    passHistory:     false,
  },
  general: {
    history:         'entity.cars.read',
    openApplication: 'detail.open_application',
    blacklist:       'page.admin.blacklist',
    entryExit:       'entity.cars.read',
    documents:       'detail.documents',
    passHistory:     'entity.cars.read',
  },
}

/**
 * Общие действия для сотрудника по контексту.
 * @type {Record<string, Record<string, string|boolean>>}
 */
export const EMPLOYEE_MODAL_ACTIONS = {
  application: {
    // Открыта из деталей заявки: "Открыть заявку" не нужно (уже в ней)
    history:         'entity.employees.read',
    openApplication: false,
    blacklist:       'page.admin.blacklist',
    entryExit:       'entity.employees.read',
    // Документы доступны по detail.documents (есть в базовой роли): обычный юзер
    // видит документы своих сотрудников в заявке; админ/супер - всегда.
    documents:       'detail.documents',
    passHistory:     false,
  },
  employeesview: {
    // Страница «Сотрудники»: полный набор действий для имеющих право
    history:         'entity.employees.read',
    openApplication: 'detail.open_application',
    blacklist:       'page.admin.blacklist',
    entryExit:       'entity.employees.read',
    documents:       'detail.documents',
    passHistory:     'entity.employees.read',
  },
  employeeslist: {
    // Список сотрудников внутри создания/редактирования заявки: режим просмотра
    // добавляемого человека. Документы показываем всегда (true) - это собственные
    // только что введённые данные формы, а не чувствительные чужие. Кнопки действий
    // (ЧС/история/открыть заявку) гасятся пропом readonly у модалки.
    history:         false,
    openApplication: false,
    blacklist:       false,
    entryExit:       false,
    documents:       true,
    passHistory:     false,
  },
  peopletable: {
    // Таблица людей в центре заявок
    history:         'entity.employees.read',
    openApplication: 'detail.open_application',
    blacklist:       'page.admin.blacklist',
    entryExit:       'entity.employees.read',
    documents:       'detail.documents',
    passHistory:     false,
  },
  trash: {
    history:         'entity.employees.read',
    openApplication: false,
    blacklist:       false,
    entryExit:       false,
    documents:       false,
    passHistory:     false,
  },
  blacklist: {
    history:         'page.admin.blacklist',
    openApplication: false,
    blacklist:       false,
    entryExit:       false,
    documents:       false,
    passHistory:     false,
  },
  general: {
    history:         'entity.employees.read',
    openApplication: 'detail.open_application',
    blacklist:       'page.admin.blacklist',
    entryExit:       'entity.employees.read',
    documents:       'detail.documents',
    passHistory:     'entity.employees.read',
  },
}

/**
 * Возвращает ключ права для действия в данном контексте.
 * Если контекст неизвестен — возвращает значение из 'general'.
 *
 * @param {'vehicle'|'employee'} entityType
 * @param {string} source
 * @param {string} action
 * @returns {string|boolean}
 */
export function getModalActionPermission(entityType, source, action) {
  const map = entityType === 'vehicle' ? VEHICLE_MODAL_ACTIONS : EMPLOYEE_MODAL_ACTIONS
  const ctx = map[source] ?? map['general']
  if (!(action in ctx)) return false
  return ctx[action]
}
