// useBugReport - утилиты страницы Error500.
// buildBugContext собирает безопасный контекст (без response body), buildBugHash
// хеширует его для дедупликации, sendBugReport отправляет POST на бэк.

const LS_PREFIX = 'bug_reported:'
const SS_CTX_KEY = 'last_bug_ctx'

/**
 * Собрать безопасный контекст ошибки.
 * На страницу 500 отправляются ТОЛЬКО эти поля - без response body,
 * заголовков или чего-либо что может содержать внутренние данные.
 *
 * uiRoute - адрес страницы приложения, на которой упал запрос. Он нужен только
 * кнопке "Повторить" и остаётся в sessionStorage: в bug-report уходят route,
 * status и message, хеш инцидента тоже считается без него.
 *
 * @param {Object} params
 * @param {string} params.route - путь запроса (pathname, без query)
 * @param {number} params.httpStatus - HTTP-код (500-599)
 * @param {string} [params.message] - generic HTTP status text ("Internal Server Error")
 * @param {string} [params.uiRoute] - маршрут SPA, с которого ушёл упавший запрос
 * @returns {Object} контекст
 */
export function buildBugContext({ route, httpStatus, message = '', uiRoute = '' }) {
  return {
    route: String(route || '').slice(0, 255),
    httpStatus: Number(httpStatus) || 500,
    message: String(message || '').slice(0, 500),
    uiRoute: String(uiRoute || '').slice(0, 255),
    timestamp: new Date().toISOString(),
  }
}

/**
 * Хеш для дедупликации: sha256(route + "|" + status + "|" + message).slice(0, 16).
 * Один юзер не сможет отправить два репорта на одинаковую комбинацию (uniq в БД).
 * Используется Web Crypto - доступен в любом современном браузере (SubtleCrypto).
 *
 * @param {Object} ctx
 * @returns {Promise<string>} 16-символьный hex
 */
export async function buildBugHash(ctx) {
  const input = `${ctx.route}|${ctx.httpStatus}|${ctx.message}`
  const bytes = new TextEncoder().encode(input)
  const hashBuf = await crypto.subtle.digest('SHA-256', bytes)
  const hex = Array.from(new Uint8Array(hashBuf))
    .map((b) => b.toString(16).padStart(2, '0'))
    .join('')
  return hex.slice(0, 16)
}

/**
 * Сохранить контекст в sessionStorage, чтобы компонент Error500.vue мог
 * его прочитать после router.push('/500'). Истекает при закрытии вкладки.
 */
export function saveBugContext(ctx) {
  try {
    sessionStorage.setItem(SS_CTX_KEY, JSON.stringify(ctx))
  } catch {
    // sessionStorage недоступен (Safari private mode) - не критично.
  }
}

export function loadBugContext() {
  try {
    const raw = sessionStorage.getItem(SS_CTX_KEY)
    return raw ? JSON.parse(raw) : null
  } catch {
    return null
  }
}

/**
 * Забыть контекст последнего инцидента. Вызывается когда юзер уходит со
 * страницы /500: контекст одноразовый, иначе кнопкой "назад" можно вернуться
 * к уже закрытому инциденту, хотя новой ошибки не было.
 */
export function clearBugContext() {
  try {
    sessionStorage.removeItem(SS_CTX_KEY)
  } catch {
    // sessionStorage недоступен - чистить нечего.
  }
}

/**
 * true если юзер уже отправлял репорт на этот bug_hash в этом браузере.
 * localStorage, чтобы флаг переживал F5.
 */
export function isReported(bugHash) {
  try {
    return localStorage.getItem(LS_PREFIX + bugHash) === '1'
  } catch {
    return false
  }
}

export function markReported(bugHash) {
  try {
    localStorage.setItem(LS_PREFIX + bugHash, '1')
  } catch {
    // private mode - silent fail
  }
}
