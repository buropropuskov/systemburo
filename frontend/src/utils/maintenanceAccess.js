// Гейт режима технических работ на клиенте. Пока режим включён, обычного
// пользователя уводит на /maintenance с любой страницы: бэкенд всё равно
// ответит 503, а объяснение он получит на самой странице работ.

/**
 * Ссылка `/?admin` - дверь для супер-администратора. До входа он для guard
 * такой же «не супер-админ», как все, и без этого исключения не добрался бы
 * до формы входа, а значит и до выключения режима. Обычные пользователи формы
 * не видят: даже пройдя по ссылке, они получат 503 на попытке войти.
 *
 * @param {import('vue-router').RouteLocationNormalized} to
 * @returns {boolean}
 */
export function isAdminLoginEscape(to) {
  return to.name === 'LoginComponent' && to.query?.admin !== undefined;
}

/**
 * Нужно ли увести пользователя на страницу технических работ.
 * Сами /maintenance и /500 остаются доступны, иначе редирект зациклится.
 *
 * @param {import('vue-router').RouteLocationNormalized} to
 * @param {{ enabled: boolean, isSuperAdmin: boolean }} state
 * @returns {boolean}
 */
export function shouldRedirectToMaintenance(to, { enabled, isSuperAdmin }) {
  if (!enabled || isSuperAdmin) return false;
  if (isAdminLoginEscape(to)) return false;
  return to.name !== 'Maintenance' && to.name !== 'Error500';
}
