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
  // Список пользователей приходит целиком, поэтому строка поиска в адресе не нужна -
  // карточка находится по id и раскрывается сразу.
  user: {
    icon: 'users',
    acceptsQuery: false,
    route: (id) => withQuery('/admin/users', '', false, id),
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
  organization: { icon: 'organizations', acceptsQuery: false, route: (id, q) => withQuery('/admin/organizations', q, false) },
  company: { icon: 'companies', acceptsQuery: false, route: (id, q) => withQuery('/admin/companies', q, false) },
  unload_place: { icon: 'unload-places', acceptsQuery: false, route: (id, q) => withQuery('/admin/unload-places', q, false) },
  system_table: { icon: 'tables', acceptsQuery: false, route: () => ({ path: '/table-constructor' }) },
  mark: { icon: 'marks', acceptsQuery: false, route: (id, q) => withQuery('/admin/marks', q, false) },
  citizenship: { icon: 'citizenship', acceptsQuery: false, route: (id, q) => withQuery('/admin/citizenship', q, false) },
  license_plate_format: { icon: 'number-formats', acceptsQuery: false, route: (id, q) => withQuery('/admin/number-formats', q, false) },
  news: { icon: 'news', acceptsQuery: false, route: () => ({ path: '/news' }) },
  announcement: { icon: 'news', acceptsQuery: false, route: () => ({ path: '/news' }) },
  document: { icon: 'documents', acceptsQuery: false, route: () => ({ path: '/news' }) },
  feedback: { icon: 'feedback', acceptsQuery: false, route: (id, q) => withQuery('/admin/feedback', q, false) },

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
