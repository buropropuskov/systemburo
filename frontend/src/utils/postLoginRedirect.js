/**
 * Целевой адрес защищённой страницы, потерянный при `next('/')` из router-guard
 * (#974): push-уведомление открывает приложение спустя дни, когда сессия уже
 * протухла - человек логинится и должен попасть на заявку, из-за которой пришло
 * уведомление, а не на дефолтную ленту новостей. Гард кладёт исходный путь в
 * query при редиректе на вход, LoginComponent читает его после успешного входа.
 */

/**
 * Разрешаем только внутренний путь: начинается с одиночного '/', не
 * protocol-relative '//host/...' и без "://" - иначе query-параметр стал бы
 * открытым редиректом (он виден и настраивается вручную в адресной строке,
 * не только гардом).
 * @param {unknown} path
 * @returns {boolean}
 */
export function isSafeRedirectPath(path) {
  return typeof path === 'string'
    && path.startsWith('/')
    && !path.startsWith('//')
    && !path.includes('://');
}

/**
 * @param {string} fullPath - `to.fullPath` заблокированной навигации
 * @returns {{ path: string, query: { redirect: string } }}
 */
export function buildLoginRedirect(fullPath) {
  return { path: '/', query: { redirect: fullPath } };
}

/**
 * @param {Record<string, unknown> | undefined} query - `this.$route.query`
 * @returns {string|null} безопасный путь для перехода после входа или null
 */
export function resolveLoginRedirect(query) {
  const target = query?.redirect;
  return isSafeRedirectPath(target) ? target : null;
}
