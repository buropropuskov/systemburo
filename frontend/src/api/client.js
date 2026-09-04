import { useAuthStore } from '@/stores/auth'
import { useMaintenanceStore } from '@/stores/maintenance'
import { useDeletionsStore } from '@/stores/deletions'
import { usePermissionsStore } from '@/stores/permissions'
import { usePDConsentStore } from '@/stores/pdConsent'
import { usePasswordChangeStore } from '@/stores/passwordChange'
import router from '@/router'
import { buildBugContext, saveBugContext } from '@/composables/useBugReport'
import { interceptRead } from './readInterceptor'
import { syncServerTime } from '@/utils/serverTime'

// API_BASE_URL оставляем настраиваемым для локальной разработки с отдельным backend-портом,
// но на staging/prod он пуст и префикс /api обеспечивает маршрутизацию через nginx:
// location /api/ -> backend:8080.
export const API_BASE_URL = (import.meta.env.VITE_API_BASE_URL || '') + '/api'
const AUTH_ENDPOINTS = ['/login', '/refresh-token', '/logout']

// Пути, на которых 403 не показывает уведомление: фоновые/ожидаемые запросы,
// где 403 — штатный ответ (нет прав или сессия не авторизована).
// Экспортируется для замка adminEndpointsFromUserFlows.spec.js: он сверяет вызовы
// с пользовательских экранов с гейтами роутов бэкенда, и список тихих путей ему
// нужен тем же, что и клиенту, - вторая копия разъехалась бы первой же правкой.
export const SILENT_403_PREFIXES = [
  '/permissions/my',
  '/permissions/catalog',
  '/users/me',
  '/users/current',
  '/unique-cars/lookup',
  '/unique-employees/lookup',
  // Подсказки справочников (#1437): фоновый запрос на ввод. Форма и так спрашивает их
  // только при праве, но если право снимут посреди сессии, тост о нём не нужен.
  '/organizations/suggest',
  '/companies/suggest',
  // Билет real-time потока (#1567): пока не дано согласие на обработку ПД, гейт
  // отбивает его 403, а поток поднимается фоном - тост тут не о чем.
  '/events/ticket',
  // Сквозной поиск: запрос уходит на каждый введённый символ. Если права сняли
  // посреди сессии, тост об этом всплывал бы под каждую букву.
  '/search',
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

// Гейт согласия на обработку ПД (#1567) отбивает protected-запросы 403 с маркером
// consent_required. Опознаём его по МАРКЕРУ ОТВЕТА, а не по флагу стора: устаревший
// флаг заглушил бы настоящие отказы в правах. Подняв флаг, показываем окно согласия
// вместо стены тостов - это же путь, по которому клиент узнаёт о поднятой в другой
// вкладке редакции.
async function isConsentRequired(response) {
  if (response.headers.get('X-PD-Consent-Required') === '1') return true
  try {
    const body = await response.clone().json()
    return Boolean(body?.consent_required)
  } catch {
    return false
  }
}

// Гейт обязательной смены пароля (#1911) отбивает protected-запросы 403 с кодом
// PASSWORD_CHANGE_REQUIRED. Опознаём по маркеру ОТВЕТА по той же причине, что и
// требование согласия: устаревший флаг стора заглушил бы настоящие отказы в правах.
// Подняв флаг, показываем окно смены пароля вместо стены тостов - иначе человек
// видит только «недостаточно прав» и не понимает, что от него хотят.
const PASSWORD_CHANGE_REQUIRED_CODE = 'PASSWORD_CHANGE_REQUIRED'

async function isPasswordChangeRequired(response) {
  if (response.headers.get('X-Password-Change-Required') === '1') return true
  try {
    const body = await response.clone().json()
    return body?.code === PASSWORD_CHANGE_REQUIRED_CODE
  } catch {
    return false
  }
}

let refreshPromise = null

function isAuthEndpoint(path) {
  return AUTH_ENDPOINTS.some((p) => path === p || path.startsWith(p + '?'))
}

// RefreshFailedError различает ДВЕ разные причины провала /refresh-token. sessionInvalid
// решает, стирать ли токены и уводить на форму входа (см. catch в baseRequest).
class RefreshFailedError extends Error {
  constructor(message, sessionInvalid) {
    super(message)
    this.sessionInvalid = sessionInvalid
  }
}

// Кратковременная недоступность базы (#2016) не должна разлогинивать того, чей
// запрос в этот момент фоном продлевал сессию: 401 - токен реально отклонён
// (истёк/отозван/переиспользован), ретраить нечего. Любой другой статус (500 от
// сбоя базы, сетевой обрыв) - лечим тем же приёмом, что и 429 в baseRequest:
// несколько попыток с паузой вместо немедленного разлогина.
const REFRESH_RETRY_DELAYS_MS = [500, 1500]

// performRefresh вызывает POST /refresh-token. Refresh token живёт в
// HttpOnly cookie - отправляется автоматически через credentials: 'include'.
async function performRefresh() {
  const authStore = useAuthStore()

  let lastStatus = 0
  for (let attempt = 0; attempt <= REFRESH_RETRY_DELAYS_MS.length; attempt++) {
    const response = await fetch(`${API_BASE_URL}/refresh-token`, {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json', 'Accept': 'application/json' },
      body: '{}',
    })
    if (response.ok) {
      const body = await response.json()
      const data = body && typeof body === 'object' && 'success' in body ? body.data : body
      if (!data || !data.token) throw new RefreshFailedError('refresh: malformed response', false)
      authStore.setTokens(data.token)
      return data.token
    }
    lastStatus = response.status
    if (response.status === 401 || attempt === REFRESH_RETRY_DELAYS_MS.length) break
    await sleep(REFRESH_RETRY_DELAYS_MS[attempt])
  }
  throw new RefreshFailedError(`refresh failed: ${lastStatus}`, lastStatus === 401)
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

/**
 * Обычный fetch, попутно сверяющий часы интерфейса с сервером (#2298).
 *
 * Заголовок Date приходит с любым ответом, поэтому синхронизация идёт на обычном
 * трафике: отдельный метод «который час» не нужен, и публичных роутов не прибавляется.
 */
async function fetchAndSyncClock(url, init) {
  const response = await fetch(url, init);
  syncServerTime(response);
  return response;
}

async function doFetch(path, options, token) {
  // Свой сигнал (options.signal) - таймаут целиком на совести вызывающего кода,
  // внутренний контроллер заводить незачем: он всё равно не будет использован.
  const controller = options.signal ? null : new AbortController()
  const timeout = controller ? setTimeout(() => controller.abort(), DEFAULT_TIMEOUT_MS) : null
  // Для FormData (multipart upload) НЕ ставим Content-Type - браузер сам добавит
  // с правильным boundary. Иначе сервер не сможет распарсить multipart.
  const isFormData = options.body instanceof FormData
  const baseHeaders = isFormData ? {} : { 'Content-Type': 'application/json' }
  try {
    return await fetchAndSyncClock(`${API_BASE_URL}${path}`, {
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
  // Демонстрационные данные онбординга (см. readInterceptor.js) - только чтение.
  const demo = interceptRead(path, options)
  if (demo) return demo
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
      // Страница инцидента предлагает вернуться туда, где всё упало - сама она
      // адрес узнать уже не может, к её монтированию currentRoute это /500.
      uiRoute: router.currentRoute.value.fullPath,
    }))
    router.push('/500')
    return response
  }

  // 403 Forbidden -- логируется в access_denials на бэке. Показываем уведомление,
  // но НЕ редиректим: вызывающий код сам решит как обработать.
  // Тихий режим: опция silent403 или путь из списка фоновых/ожидаемых запросов.
  if (response.status === 403 && !isAuthEndpoint(path)) {
    if (await isConsentRequired(response)) {
      try {
        usePDConsentStore().markRequiredFromResponse()
      } catch {
        // pinia ещё не активна на раннем запросе -- окно поднимет App при загрузке
      }
      return response
    }
    if (await isPasswordChangeRequired(response)) {
      try {
        usePasswordChangeStore().markRequiredFromResponse()
      } catch {
        // pinia ещё не активна на раннем запросе -- окно поднимет следующий отказ
      }
      return response
    }
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

  // Маркер режима «войти как пользователь» истёк (#1912). Обновлять сессию здесь
  // нельзя: cookie осталась администраторской, и обновление вернуло бы его
  // собственный маркер - запросы пошли бы уже от администратора, а полоса на
  // экране продолжала бы называть чужое имя. Честный исход - закрыть режим и
  // сказать об этом; неудавшийся запрос не повторяем, его делал другой человек.
  if (response.status === 401 && !isAuthEndpoint(path) && authStore.isImpersonating) {
    await authStore.endImpersonation({ recordExit: false })
    useDeletionsStore().notify({
      prefix: 'Сеанс работы от имени другого пользователя истёк',
      type: 'warning',
    })
    return response
  }

  if (response.status !== 401 || isAuthEndpoint(path) || options._retried) {
    return response
  }

  try {
    const newToken = await ensureRefreshed()
    response = await doFetch(path, { ...options, _retried: true }, newToken)
    return response
  } catch (err) {
    // Сессию рвём только когда токен ДЕЙСТВИТЕЛЬНО недействителен (401 от
    // /refresh-token, после ретраев). Сбой сервера/сети (#2016) сессию не трогает -
    // человек остаётся на своём экране, а исходный 401 уходит вызывающему коду.
    if (!err?.sessionInvalid) {
      return response
    }
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
