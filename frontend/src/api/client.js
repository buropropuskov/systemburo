import { useAuthStore } from '@/stores/auth'
import router from '@/router'

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || ''
const AUTH_ENDPOINTS = ['/login', '/refresh-token', '/logout']

let refreshPromise = null

function isAuthEndpoint(path) {
  return AUTH_ENDPOINTS.some((p) => path === p || path.startsWith(p + '?'))
}

async function performRefresh() {
  const authStore = useAuthStore()
  const refreshToken = authStore.refreshToken
  if (!refreshToken) throw new Error('no refresh token')

  const response = await fetch(`${API_BASE_URL}/refresh-token`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ refresh_token: refreshToken }),
  })
  if (!response.ok) throw new Error(`refresh failed: ${response.status}`)

  const body = await response.json()
  const data = body && typeof body === 'object' && 'success' in body ? body.data : body
  if (!data || !data.token || !data.refreshToken) throw new Error('refresh: malformed response')
  authStore.setTokens(data.token, data.refreshToken)
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
