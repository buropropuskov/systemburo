/**
 * Элемент, который надо открыть сразу после перехода: `?open=<id>`.
 *
 * Парный к строке поиска в адресе (см. searchQueryParam): `?q=` сужает список до
 * нужной записи, `?open=` раскрывает её карточку. По отдельности каждый работает сам
 * по себе - ссылку на карточку можно переслать и без строки поиска.
 *
 * Имя параметра не новое: тем же `open` уже открывается заявка в центре по переходу
 * из уведомления, и заводить второе имя для того же смысла незачем.
 */

export const OPEN_PARAM = 'open';

/**
 * @param {import('vue-router').RouteLocationNormalized} route
 * @returns {number|null} id для открытия или null, если параметра нет либо он мусорный
 */
export function readOpenIdFromRoute(route) {
  const id = Number(route?.query?.[OPEN_PARAM]);
  return Number.isInteger(id) && id > 0 ? id : null;
}

/**
 * Убрать `open` из адреса, сохранив остальные параметры.
 *
 * Чистим сразу после открытия: иначе обновление страницы (или возврат по истории)
 * снова раскрывало бы карточку, которую человек уже закрыл.
 *
 * @param {import('vue-router').Router} router
 * @param {import('vue-router').RouteLocationNormalized} route
 */
export function clearOpenFromRoute(router, route) {
  if (!(OPEN_PARAM in (route?.query || {}))) return;
  const query = { ...route.query };
  delete query[OPEN_PARAM];
  // catch гасит отмену навигации, если пользователь уже ушёл со страницы.
  router.replace({ query }).catch(() => {});
}

/**
 * Области реестра от самой широкой к самой узкой, с правом на каждую.
 *
 * Порядок важен: берём первую доступную, то есть максимально широкую из разрешённых.
 */
const REGISTRY_SCOPES = [
  ['all_system', 'section.registry.all_system'],
  ['organization', 'section.registry.organization'],
  ['company', 'section.registry.company'],
];

/**
 * Область, в которой реестр открывается при переходе из сквозного поиска.
 *
 * Поиск ищет по всему, что человеку разрешено, а реестр по умолчанию показывает
 * «Мои». Чужая найденная запись в эту область не попадает: список приезжает пустым,
 * и открывать оказывается нечего - на стенде переход к сотруднику из общесистемной
 * выдачи заканчивался пустым реестром. Поэтому при переходе по ссылке с записью
 * область расширяется до самой широкой доступной.
 *
 * Без `open` в адресе ничего не меняем: обычный заход на страницу должен открываться
 * там же, где и раньше.
 *
 * @param {object} route текущий маршрут
 * @param {(permission: string) => boolean} hasPermission проверка права
 * @param {string} [fallback] область по умолчанию
 * @returns {string}
 */
export function registryScopeForRoute(route, hasPermission, fallback = 'user') {
  if (!readOpenIdFromRoute(route)) return fallback;
  const widest = REGISTRY_SCOPES.find(([, permission]) => hasPermission(permission));
  return widest ? widest[0] : fallback;
}

/**
 * Открыть элемент из уже загруженного списка по `?open=<id>`.
 *
 * Запись ищется среди загруженных, а не догружается отдельным запросом: переход из
 * сквозного поиска приносит и строку `?q`, по которой сервер её нашёл, так что список
 * приезжает уже суженным и нужная запись в нём есть. Не нашли - `open` остаётся в
 * адресе, и следующая подгрузка (или смена фильтра) попробует снова.
 *
 * @param {{router: object, route: object, items: object[], open: (item: object) => void}} ctx
 * @returns {boolean} открыли ли карточку
 */
export function openItemFromRoute({ router, route, items, open }) {
  const id = readOpenIdFromRoute(route);
  if (!id) return false;

  const item = (items || []).find((row) => Number(row?.id) === id);
  if (!item) return false;

  open(item);
  clearOpenFromRoute(router, route);
  return true;
}
