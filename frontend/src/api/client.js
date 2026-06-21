import { useAuthStore } from '@/stores/auth'
import { useMaintenanceStore } from '@/stores/maintenance'
import { useDeletionsStore } from '@/stores/deletions'
import router from '@/router'
import { buildBugContext, saveBugContext } from '@/composables/useBugReport'

// API_BASE_URL оставляем настраиваемым для локальной разработки с отдельным backend-портом,
// но на staging/prod он пуст и префикс /api обеспечивает маршрутизацию через nginx:
// location /api/ -> backend:8080.
const API_BASE_URL = (import.meta.env.VITE_API_BASE_URL || '') + '/api'
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

/** @internal только для тестов */
export function _resetDedup403() { _403dedup.clear() }

function shouldSilence403(path) {
  return SILENT_403_PREFIXES.some((p) => path === p || path.startsWith(p + '?') || path.startsWith(p + '/'))
}

function show403Notify(msg) {
  if (_403dedup.has(msg)) return
  _403dedup.set(msg, true)
  setTimeout(() => _403dedup.delete(msg), DEDUP_TTL_MS)
  useDeletionsStore().notify({ prefix: msg, type: 'error' })
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
    if (!options.silent403 && !shouldSilence403(path)) {
      try {
        const body = await response.clone().json()
        const msg = body?.banned
          ? 'Учётная запись заблокирована. Обратитесь к администратору.'
          : 'Недостаточно прав для этого действия.'
        show403Notify(msg)
      } catch {
        // body может быть пустым/не json -- показываем дефолтное сообщение
        show403Notify('Недостаточно прав для этого действия.')
      }
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
