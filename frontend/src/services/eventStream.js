/**
 * Клиент real-time потока (#840). Держит одно SSE-соединение на пользователя:
 * берёт одноразовый билет защищённым запросом, открывает EventSource, при обрыве
 * переподключается с backoff, а после нескольких неудач сообщает статус 'fallback'
 * (потребитель возвращается к обычному поллингу). По сигналу от сервера дёргает
 * подписчиков нужного scope - те делают обычный запрос (event-then-fetch).
 *
 * Статусы: 'connected' | 'reconnecting' | 'fallback' | 'disconnected'. Потребитель
 * гасит поллинг только на 'connected'; на 'reconnecting'/'fallback' поллинг снова
 * подстраховывает, чтобы обрыв не задерживал обновления.
 */
import { apiRequest, API_BASE_URL } from '@/api/client'

const RECONNECT_BASE_MS = 1000
const RECONNECT_MAX_MS = 30000
const FALLBACK_AFTER_FAILURES = 3

let source = null
let refCount = 0
let attempts = 0
let reconnectTimer = null
// generation инвалидирует in-flight openStream при disconnect/новом подключении:
// сверяем локальный myGen с глобальным после await билета.
let generation = 0
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

async function fetchTicket() {
  try {
    const res = await apiRequest('/events/ticket', { method: 'POST' })
    if (!res.ok) return null
    const data = await res.json()
    return (data && data.ticket) || null
  } catch {
    return null // нет сети / refresh не удался - уйдём в reconnect
  }
}

async function openStream() {
  const myGen = ++generation
  const ticket = await fetchTicket()
  // Пока брали билет, могли отключиться (disconnect) или переоткрыться (новый gen).
  if (myGen !== generation || refCount <= 0) return
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
  // Сервер закрывает поток по максимальному времени жизни - переоткрываем новым билетом.
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
  if (refCount <= 0) return
  attempts += 1
  // Снимаем 'connected' сразу при обрыве, чтобы потребитель включил поллинг-подстраховку,
  // не дожидаясь окончательного fallback.
  setStatus(attempts >= FALLBACK_AFTER_FAILURES ? 'fallback' : 'reconnecting')
  const delay = Math.min(RECONNECT_BASE_MS * 2 ** (attempts - 1), RECONNECT_MAX_MS)
  clearTimeout(reconnectTimer)
  reconnectTimer = setTimeout(() => openStream(), delay)
}

function restart() {
  if (refCount <= 0) return // уже отключились - не дёргаем лишний билет
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
    generation += 1 // инвалидирует любой in-flight openStream
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

/** onStatus подписывает handler на смену статуса. Возвращает функцию отписки. */
function onStatus(handler) {
  statusHandlers.add(handler)
  return () => statusHandlers.delete(handler)
}

/** __resetForTests сбрасывает модульное состояние между юнит-тестами. */
function __resetForTests() {
  refCount = 0
  attempts = 0
  generation += 1
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
