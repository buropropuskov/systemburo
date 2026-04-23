import { useAuthStore } from '@/stores/auth'
import router from '@/router'

// API_BASE_URL оставляем настраиваемым для локальной разработки с отдельным backend-портом,
// но на staging/prod он пуст и префикс /api обеспечивает маршрутизацию через nginx:
// location /api/ -> backend:8080.
const API_BASE_URL = (import.meta.env.VITE_API_BASE_URL || '') + '/api'
const AUTH_ENDPOINTS = ['/login', '/refresh-token', '/logout']

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
  try {
    return await fetch(`${API_BASE_URL}${path}`, {
      ...options,
      credentials: 'include',
      signal: options.signal || controller.signal,
      headers: {
        'Content-Type': 'application/json',
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

async function baseRequest(path, options = {}) {
  const authStore = useAuthStore()
  let response = await doFetch(path, options, authStore.token)

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
