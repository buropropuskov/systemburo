/**
 * Клиент real-time потока (#840). Держит одно SSE-соединение на пользователя:
 * берёт одноразовый билет защищённым запросом, открывает EventSource, при обрыве
 * переподключается с backoff, а после нескольких неудач сообщает статус 'fallback'
 * (потребитель возвращается к обычному поллингу). По сигналу от сервера дёргает
 * подписчиков нужного scope - те делают обычный запрос (event-then-fetch).
 */
import { apiRequest, API_BASE_URL } from '@/api/client'

const RECONNECT_BASE_MS = 1000
const RECONNECT_MAX_MS = 30000
const FALLBACK_AFTER_FAILURES = 3

let source = null
let refCount = 0
let attempts = 0
let reconnectTimer = null
let closedByUs = false
let status = 'disconnected'

const scopeHandlers = new Map() // scope -> Set<fn>
const statusHandlers = new Set() // fn(status)

function setStatus(next) {
  if (status === next) return
  status = next
  statusHandlers.forEach((h) => h(next))
}

function emitScope(scope, payload) {
  const set = scopeHandlers.get(scope)
  if (set) set.forEach((h) => h(payload))
}

function handleMessage(event) {
  let data
  try {
    data = JSON.parse(event.data)
  } catch {
    return // heartbeat-комментарии и мусор игнорируем
  }
  if (data && data.scope) emitScope(data.scope, data)
}

async function openStream() {
  closedByUs = false
  let ticket
  try {
    const res = await apiRequest('/events/ticket', { method: 'POST' })
    ticket = res && res.ticket
  } catch {
    // билет не выдали (нет сети / 401) - ниже уйдём в reconnect
  }
  if (closedByUs || refCount <= 0) return // disconnect пришёл, пока брали билет
  if (!ticket) {
    scheduleReconnect()
    return
  }

  source = new EventSource(`${API_BASE_URL}/events?ticket=${encodeURIComponent(ticket)}`)
  source.onopen = () => {
    attempts = 0
    setStatus('connected')
  }
  source.onmessage = handleMessage
  // Сервер закрывает поток по максимальному времени жизни - переоткрываем с новым билетом.
  source.addEventListener('reconnect', () => restart())
  source.onerror = () => {
    if (source) {
      source.close()
      source = null
    }
    scheduleReconnect()
  }
}

function scheduleReconnect() {
  if (closedByUs || refCount <= 0) return
  attempts += 1
  if (attempts >= FALLBACK_AFTER_FAILURES) setStatus('fallback')
  const delay = Math.min(RECONNECT_BASE_MS * 2 ** (attempts - 1), RECONNECT_MAX_MS)
  clearTimeout(reconnectTimer)
  reconnectTimer = setTimeout(() => openStream(), delay)
}

function restart() {
  if (source) {
    source.close()
    source = null
  }
  openStream()
}

/** connect регистрирует потребителя и при первом поднимает соединение (refcount). */
function connect() {
  refCount += 1
  if (refCount === 1) openStream()
}

/** disconnect снимает потребителя и при последнем закрывает соединение. */
function disconnect() {
  refCount = Math.max(0, refCount - 1)
  if (refCount === 0) {
    closedByUs = true
    clearTimeout(reconnectTimer)
    if (source) {
      source.close()
      source = null
    }
    attempts = 0
    setStatus('disconnected')
  }
}

/** subscribe подписывает handler на события scope. Возвращает функцию отписки. */
function subscribe(scope, handler) {
  if (!scopeHandlers.has(scope)) scopeHandlers.set(scope, new Set())
  scopeHandlers.get(scope).add(handler)
  return () => {
    const set = scopeHandlers.get(scope)
    if (!set) return
    set.delete(handler)
    if (set.size === 0) scopeHandlers.delete(scope)
  }
}

/** onStatus подписывает handler на смену статуса ('connected'|'fallback'|'disconnected'). */
function onStatus(handler) {
  statusHandlers.add(handler)
  return () => statusHandlers.delete(handler)
}

/** __resetForTests сбрасывает модульное состояние между юнит-тестами. */
function __resetForTests() {
  refCount = 0
  attempts = 0
  closedByUs = false
  status = 'disconnected'
  clearTimeout(reconnectTimer)
  if (source) {
    source.close()
    source = null
  }
  scopeHandlers.clear()
  statusHandlers.clear()
}

export default { connect, disconnect, subscribe, onStatus }
export { __resetForTests }
