/**
 * Куда ведёт результат сквозного поиска.
 *
 * Сервер отдаёт код сущности и её номер, а не готовый путь: маршруты живут в router.js и
 * меняются без ведома бэкенда. Карта здесь -- единственное место, где одно связывается с
 * другим.
 *
 * `acceptsQuery` помечает страницы, которые умеют принять строку поиска в адресе. Пока
 * страница этого не умеет, переход ведёт просто в раздел -- это позволяет подключать
 * приёмники по одному, не дожидаясь, пока их переделают все.
 */

/** Строка поиска в адресе. Канонический ключ для новых страниц. */
export const QUERY_PARAM = 'q';

function withQuery(path, query, accepts) {
  if (!accepts || !query) return { path };
  return { path, query: { [QUERY_PARAM]: query } };
}

export const SEARCH_TARGETS = {
  unique_employee: {
    icon: 'employees',
    acceptsQuery: false,
    route: (id, q) => withQuery('/employeesview', q, false),
  },
  unique_car: {
    icon: 'cars',
    acceptsQuery: false,
    route: (id, q) => withQuery('/carsview', q, false),
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
    acceptsQuery: false,
    route: (id, q) => withQuery('/admin/users', q, false),
  },
  person_blacklist: {
    icon: 'blacklist',
    acceptsQuery: false,
    route: (id, q) => withQuery('/admin/blacklist', q, false),
  },
  vehicle_blacklist: {
    icon: 'blacklist',
    acceptsQuery: false,
    route: (id, q) => withQuery('/admin/blacklist', q, false),
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
