/**
 * Строка поиска страницы в адресе.
 *
 * Нужна двум сценариям: переход из сквозного поиска сразу с набранным запросом и
 * обычная ссылка на отфильтрованный список, которую можно переслать или сохранить.
 *
 * Оформлено функциями, а не composable: страницы-приёмники держат строку поиска в
 * `data()` со своей обработкой ввода, и подменять её на ref значило бы переписывать
 * компонент целиком ради связки с адресом.
 */

/** Канонический параметр. Историческое `search` читается как синоним при старте. */
export const QUERY_PARAM = 'q';

/**
 * Прочитать строку поиска из адреса. Вызывается синхронно при создании компонента:
 * список должен уйти на сервер сразу с нужным запросом, без лишней первой загрузки.
 *
 * @param {import('vue-router').RouteLocationNormalized} route
 * @param {string[]} [aliases] устаревшие имена параметра, читаются только при старте
 * @returns {string}
 */
export function readSearchFromRoute(route, aliases = ['search']) {
  const pick = (name) => (typeof route?.query?.[name] === 'string' ? route.query[name] : '');
  return pick(QUERY_PARAM) || aliases.map(pick).find(Boolean) || '';
}

/**
 * Отразить строку поиска в адресе, не трогая остальные параметры.
 *
 * Соседние параметры сохраняются осознанно: рядом живут признак архива в центре заявок
 * и фильтры по типу с организацией в доступных вложениях -- замена всего объекта их бы
 * стёрла. Пустая строка убирает параметр, чтобы в адресе не болтался `?q=`.
 *
 * @param {import('vue-router').Router} router
 * @param {import('vue-router').RouteLocationNormalized} route
 * @param {string} value
 * @param {string[]} [aliases] устаревшие имена, вычищаются при первой же записи
 */
export function writeSearchToRoute(router, route, value, aliases = ['search']) {
  const query = { ...route.query };
  const trimmed = (value ?? '').trim();

  if (trimmed) query[QUERY_PARAM] = trimmed;
  else delete query[QUERY_PARAM];
  aliases.forEach((a) => { delete query[a]; });

  const same = JSON.stringify(query) === JSON.stringify(route.query);
  if (same) return;

  // catch гасит отмену навигации при быстром вводе: vue-router отклоняет replace,
  // которую тут же сменила следующая. Это гонка ввода, а не ошибка приложения.
  router.replace({ query }).catch(() => {});
}
