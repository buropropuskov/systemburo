import { useAuthStore } from '@/stores/auth'
import { useMaintenanceStore } from '@/stores/maintenance'
import { useDeletionsStore } from '@/stores/deletions'
import { usePermissionsStore } from '@/stores/permissions'
import router from '@/router'
import { buildBugContext, saveBugContext } from '@/composables/useBugReport'

// API_BASE_URL оставляем настраиваемым для локальной разработки с отдельным backend-портом,
// но на staging/prod он пуст и префикс /api обеспечивает маршрутизацию через nginx:
// location /api/ -> backend:8080.
export const API_BASE_URL = (import.meta.env.VITE_API_BASE_URL || '') + '/api'
const AUTH_ENDPOINTS = ['/login', '/refresh-token', '/logout']

// Пути, на которых 403 не показывает уведомление: фоновые/ожидаемые запросы,
// где 403 — штатный ответ (нет прав или сессия не авторизована).
const SILENT_403_PREFIXES = [
  '/permissions/my',
  '/permissions/catalog',
  '/users/me',
  '/users/current',
  '/unique-cars/lookup',
  '/unique-employees/lookup',
]

// Дедупликация 403-уведомлений: одинаковый текст в окне TTL показывается один раз.
const DEDUP_TTL_MS = 4000
const _403dedup = new Map()
const _429dedup = new Map()

// 429: короткий всплеск лечим ожиданием+повтором, дольше ждать не висим на UI.
const MAX_429_RETRIES = 2
const RETRY_429_CAP_MS = 6000

/** @internal только для тестов */
export function _resetDedup403() { _403dedup.clear(); _429dedup.clear() }

function shouldSilence403(path) {
  return SILENT_403_PREFIXES.some((p) => path === p || path.startsWith(p + '?') || path.startsWith(p + '/'))
}

function show403Notify(msg) {
  if (_403dedup.has(msg)) return
  _403dedup.set(msg, true)
  setTimeout(() => _403dedup.delete(msg), DEDUP_TTL_MS)
  useDeletionsStore().notify({ prefix: msg, type: 'error' })
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

function show429Notify(retryAfterSec) {
  const msg = Number.isFinite(retryAfterSec) && retryAfterSec > 0
    ? `Слишком много запросов. Повторите через ${retryAfterSec} сек.`
    : 'Слишком много запросов. Повторите чуть позже.'
  if (_429dedup.has(msg)) return
  _429dedup.set(msg, true)
  setTimeout(() => _429dedup.delete(msg), DEDUP_TTL_MS)
  useDeletionsStore().notify({ prefix: msg, type: 'warning' })
}

// Забаненному не показываем тосты 403: плашка блокировки (BanOverlay) уже всё
// объясняет, а read-only кабинет может ловить 403 на мутациях/фоновых запросах.
// Сигнал бана -- флаг стора (из /permissions/my) или поле banned в теле ответа.
async function isBanContext(response) {
  try {
    if (usePermissionsStore().banned) return true
  } catch {
    // pinia ещё не активна на раннем запросе -- молча игнорируем
  }
  try {
    const body = await response.clone().json()
    return Boolean(body?.banned)
  } catch {
    return false
  }
}

let refreshPromise = null

function isAuthEndpoint(path) {
  return AUTH_ENDPOINTS.some((p) => path === p || path.startsWith(p + '?'))
}

// performRefresh вызывает POST /refresh-token. Refresh token живёт в
// HttpOnly cookie - отправляется автоматически через credentials: 'include'.
async function performRefresh() {
  const authStore = useAuthStore()

  const response = await fetch(`${API_BASE_URL}/refresh-token`, {
    method: 'POST',
    credentials: 'include',
    headers: { 'Content-Type': 'application/json', 'Accept': 'application/json' },
    body: '{}',
  })
  if (!response.ok) throw new Error(`refresh failed: ${response.status}`)

  const body = await response.json()
  const data = body && typeof body === 'object' && 'success' in body ? body.data : body
  if (!data || !data.token) throw new Error('refresh: malformed response')
  authStore.setTokens(data.token)
  return data.token
}

function ensureRefreshed() {
  if (!refreshPromise) {
    refreshPromise = performRefresh().finally(() => {
      refreshPromise = null
    })
  }
  return refreshPromise
}

// tryRestoreSession - вызывается на монтировании App, пытается обновить сессию
// из HttpOnly cookie. Используется после F5 когда token в памяти Pinia потерян,
// но cookie сервера ещё жива.
export async function tryRestoreSession() {
  try {
    await ensureRefreshed()
    return true
  } catch {
    return false
  }
}

function wrapJsonUnwrap(response) {
  const originalJson = response.json.bind(response)
  response.json = async () => {
    const body = await originalJson()
    if (body && typeof body === 'object' && 'success' in body) {
      if (!body.success) {
        return { message: body.error }
      }
      return body.data
    }
    return body
  }
  return response
}

async function doFetch(path, options, token) {
  const controller = new AbortController()
  const timeout = setTimeout(() => controller.abort(), 10000)
  // Для FormData (multipart upload) НЕ ставим Content-Type - браузер сам добавит
  // с правильным boundary. Иначе сервер не сможет распарсить multipart.
  const isFormData = options.body instanceof FormData
  const baseHeaders = isFormData ? {} : { 'Content-Type': 'application/json' }
  try {
    return await fetch(`${API_BASE_URL}${path}`, {
      ...options,
      credentials: 'include',
      signal: options.signal || controller.signal,
      headers: {
        ...baseHeaders,
        // Accept: application/json нужен чтобы nginx с Accept-based роутингом
        // (см. nginx/staging.conf для /news и /announcements) отличал API-запрос
        // от браузерного перехода по тому же пути и отдавал JSON, а не SPA HTML.
        'Accept': 'application/json',
        ...(token && { Authorization: `Bearer ${token}` }),
        ...options.headers,
      },
    })
  } finally {
    clearTimeout(timeout)
  }
}

// Эндпоинты, на которых 5xx НЕ редиректим на /500 - это внутренние
// механизмы (refresh, отправка самого bug-report), их ответы должны
// обрабатываться локально, иначе получим infinite redirect loop.
const SKIP_500_REDIRECT = ['/bug-report', '/refresh-token', '/login', '/logout']

function shouldHandleAsServerError(path, status) {
  if (status < 500 || status > 599) return false
  if (SKIP_500_REDIRECT.some((p) => path === p || path.startsWith(p + '?'))) return false
  if (router.currentRoute.value.path === '/500') return false
  return true
}

async function baseRequest(path, options = {}) {
  const authStore = useAuthStore()
  let response = await doFetch(path, options, authStore.token)

  // 503 Service Unavailable = maintenance. Только что узнали об этом на лету -
  // обновляем стор (чтобы guard начал редиректить сразу) и шлём юзера на
  // /maintenance. Не путаем с 500 - у maintenance отдельная страница и нет
  // bug-report'а.
  if (response.status === 503 && router.currentRoute.value.path !== '/maintenance') {
    try {
      await useMaintenanceStore().fetchStatus()
    } catch {
      // не критично
    }
    router.push('/maintenance')
    return response
  }

  // 5xx -> сохраняем безопасный контекст и редиректим на /500.
  // Ответ не читаем: тело может содержать детали ошибки, которые нам
  // показывать юзеру нельзя (leak архитектуры). Используем только status и path.
  if (shouldHandleAsServerError(path, response.status)) {
    saveBugContext(buildBugContext({
      route: `${options.method || 'GET'} ${path}`,
      httpStatus: response.status,
      message: response.statusText || `HTTP ${response.status}`,
    }))
    router.push('/500')
    return response
  }

  // 403 Forbidden -- логируется в access_denials на бэке. Показываем уведомление,
  // но НЕ редиректим: вызывающий код сам решит как обработать.
  // Тихий режим: опция silent403 или путь из списка фоновых/ожидаемых запросов.
  if (response.status === 403 && !isAuthEndpoint(path)) {
    if (!options.silent403 && !shouldSilence403(path) && !(await isBanContext(response))) {
      show403Notify('Недостаточно прав для этого действия.')
    }
    return response
  }

  // 429 Too Many Requests. Rate limiter отбивает запрос ДО хендлера -> запрос
  // не выполнился, повтор безопасен (в т.ч. POST/PUT - нет частичного эффекта).
  // Короткий всплеск лечим ожиданием Retry-After (с потолком) и повтором; при
  // исчерпании попыток показываем уведомление с остатком времени. /login исключён
  // (у него своя плашка с таймером). Фоновые (options.silent) не ретраим и не тостим.
  if (response.status === 429 && !isAuthEndpoint(path)) {
    const retryAfterSec = parseInt(response.headers.get('Retry-After'), 10)
    const attempt = options._429retry || 0
    if (!options.silent && attempt < MAX_429_RETRIES) {
      const waitMs = Number.isFinite(retryAfterSec) && retryAfterSec > 0
        ? Math.min(retryAfterSec * 1000, RETRY_429_CAP_MS)
        : RETRY_429_CAP_MS
      await sleep(waitMs)
      return baseRequest(path, { ...options, _429retry: attempt + 1 })
    }
    if (!options.silent) {
      show429Notify(Number.isFinite(retryAfterSec) ? retryAfterSec : 0)
    }
    return response
  }

  if (response.status !== 401 || isAuthEndpoint(path) || options._retried) {
    return response
  }

  try {
    const newToken = await ensureRefreshed()
    response = await doFetch(path, { ...options, _retried: true }, newToken)
    return response
  } catch {
    authStore.clearTokens()
    if (router.currentRoute.value.path !== '/') {
      router.push('/')
    }
    return response
  }
}

export async function apiRequest(path, options = {}) {
  const response = await baseRequest(path, options)
  return wrapJsonUnwrap(response)
}

export async function apiRequestRaw(path, options = {}) {
  return baseRequest(path, options)
}
