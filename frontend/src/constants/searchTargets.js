/**
 * Куда ведёт результат сквозного поиска.
 *
 * Сервер отдаёт код сущности и её номер, а не готовый путь: маршруты живут в router.js и
 * меняются без ведома бэкенда. Карта здесь -- единственное место, где одно связывается с
 * другим.
 *
 * `acceptsQuery` помечает страницы, которые умеют принять строку поиска в адресе, а
 * `id` в маршруте -- те, что умеют сразу раскрыть карточку найденного. Пока страница
 * не умеет ни того, ни другого, переход ведёт просто в раздел: приёмники подключаются
 * по одному, не дожидаясь, пока переделают все.
 */

import { OPEN_PARAM } from '@/utils/openQueryParam';

/** Строка поиска в адресе. Канонический ключ для новых страниц. */
export const QUERY_PARAM = 'q';

function withQuery(path, query, accepts, openID) {
  const params = {};
  if (accepts && query) params[QUERY_PARAM] = query;
  // Строка поиска сужает список до найденной записи, id раскрывает её карточку.
  if (openID) params[OPEN_PARAM] = String(openID);
  return Object.keys(params).length ? { path, query: params } : { path };
}

/**
 * Чёрные списки - две вкладки на одной странице, и id записей у них независимые.
 * Без указания вкладки страница открылась бы на машинах, даже когда нашёлся человек.
 */
function blacklistRoute(tab, id, query) {
  const target = withQuery('/admin/blacklist', query, true, id);
  return { path: target.path, query: { ...(target.query || {}), tab } };
}

export const SEARCH_TARGETS = {
  unique_employee: {
    icon: 'employees',
    acceptsQuery: true,
    route: (id, q) => withQuery('/employeesview', q, true, id),
  },
  unique_car: {
    icon: 'cars',
    acceptsQuery: true,
    route: (id, q) => withQuery('/carsview', q, true, id),
  },
  // У заявок открытие карточки по номеру уже работает -- этим же параметром ходят
  // переходы из уведомлений.
  application: {
    icon: 'center',
    acceptsQuery: true,
    route: (id) => ({ path: '/center', query: { open: String(id) } }),
  },
  user: {
    icon: 'users',
    acceptsQuery: true,
    route: (id, q) => withQuery('/admin/users', q, true, id),
  },
  person_blacklist: {
    icon: 'blacklist',
    acceptsQuery: true,
    route: (id, q) => blacklistRoute('persons', id, q),
  },
  vehicle_blacklist: {
    icon: 'blacklist',
    acceptsQuery: true,
    route: (id, q) => blacklistRoute('vehicles', id, q),
  },
  organization: { icon: 'organizations', acceptsQuery: true, route: (id, q) => withQuery('/admin/organizations', q, true, id) },
  company: { icon: 'companies', acceptsQuery: true, route: (id, q) => withQuery('/admin/companies', q, true, id) },
  unload_place: { icon: 'unload-places', acceptsQuery: true, route: (id, q) => withQuery('/admin/unload-places', q, true, id) },
  system_table: { icon: 'tables', acceptsQuery: true, route: (id, q) => withQuery('/table-constructor', q, true, id) },
  mark: { icon: 'marks', acceptsQuery: true, route: (id, q) => withQuery('/admin/marks', q, true, id) },
  citizenship: { icon: 'citizenship', acceptsQuery: true, route: (id, q) => withQuery('/admin/citizenship', q, true, id) },
  license_plate_format: { icon: 'number-formats', acceptsQuery: true, route: (id, q) => withQuery('/admin/number-formats', q, true, id) },
  news: { icon: 'news', acceptsQuery: false, route: (id) => withQuery('/news', '', false, id) },
  announcement: { icon: 'news', acceptsQuery: false, route: () => ({ path: '/news' }) },
  document: { icon: 'documents', acceptsQuery: false, route: () => ({ path: '/news' }) },
  feedback: { icon: 'feedback', acceptsQuery: true, route: (id, q) => withQuery('/admin/feedback', q, true, id) },

  /** Человекочитаемые названия разделов -- нужны сообщению о том, что раздел не ответил. */
  groupTitles: {
    employees: 'Сотрудники',
    cars: 'Автомобили',
    applications: 'Заявки',
    users: 'Пользователи',
    blacklist: 'Чёрные списки',
    directories: 'Справочники',
    content: 'Новости и документы',
    feedback: 'Обратная связь',
  },
};
