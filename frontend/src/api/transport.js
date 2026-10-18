// API_BASE_URL оставляем настраиваемым для локальной разработки с отдельным backend-портом,
// но на staging/prod он пуст и префикс /api обеспечивает маршрутизацию через nginx:
// location /api/ -> backend:8080.
export const API_BASE_URL = (import.meta.env.VITE_API_BASE_URL || '') + '/api'
// Таймаут по умолчанию для обычных запросов. Тяжёлые операции (подача заявки
// с массовым импортом до 2000 строк) передают свой options.signal - см.
// createExtendedTimeoutSignal ниже.
const DEFAULT_TIMEOUT_MS = 10000

/**
 * AbortSignal с собственным таймаутом - для запросов, которым DEFAULT_TIMEOUT_MS
 * не хватает (подача заявки с массовым импортом бланком). Передавать в apiRequest
 * как options.signal: doFetch подставит его вместо внутреннего контроллера и не
 * станет заводить свой 10-секундный таймер поверх него.
 *
 * @param {number} ms
 * @returns {AbortSignal}
 */
export function createExtendedTimeoutSignal(ms) {
  return AbortSignal.timeout(ms)
}

export async function doFetch(path, options, token) {
  // Свой сигнал (options.signal) - таймаут целиком на совести вызывающего кода,
  // внутренний контроллер заводить незачем: он всё равно не будет использован.
  const controller = options.signal ? null : new AbortController()
  const timeout = controller ? setTimeout(() => controller.abort(), DEFAULT_TIMEOUT_MS) : null
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
    if (timeout) clearTimeout(timeout)
  }
}
