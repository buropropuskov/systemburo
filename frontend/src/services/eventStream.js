import { apiRequest, API_BASE_URL } from '@/api/client'

const BASE_MS = 1000
const MAX_MS = 30000
let source = null
let refCount = 0
let attempts = 0
let generation = 0
let reconnectTimer = null
let ticketController = null
let fallbackTimer = null
let status = 'disconnected'
const scopeHandlers = new Map() // scope -> Map<handler, { fallback, running }>
const statusHandlers = new Set()

// Ошибка подписчика не прерывает доставку остальным или lifecycle потока.
function invoke(handler, payload) {
  try { return Promise.resolve(handler(payload)).catch(() => {}) }
  catch { return Promise.resolve() }
}

function invokeScope(scope, handler, entry, payload, myGen) {
  if (entry.running) {
    entry.pending = { payload, generation: myGen }
    return
  }
  entry.running = true
  invoke(handler, payload).finally(() => {
    entry.running = false
    const pending = entry.pending
    entry.pending = null
    if (pending && pending.generation === generation && refCount > 0
      && scopeHandlers.get(scope)?.get(handler) === entry) {
      invokeScope(scope, handler, entry, pending.payload, pending.generation)
    }
  })
}

function updateFallbackTimer() {
  const needed = refCount > 0 && status !== 'connected' && status !== 'disconnected'
    && [...scopeHandlers.values()].some((handlers) => [...handlers.values()].some((entry) => entry.fallback))
  if (!needed) {
    clearInterval(fallbackTimer)
    fallbackTimer = null
  } else if (fallbackTimer === null) {
    const timer = setInterval(() => {
      if (fallbackTimer !== timer || status === 'connected' || status === 'disconnected') return
      const myGen = generation
      for (const [scope, handlers] of [...scopeHandlers]) {
        for (const [handler, entry] of [...handlers]) {
          if (myGen !== generation || refCount <= 0) return
          if (!entry.fallback || entry.running || scopeHandlers.get(scope)?.get(handler) !== entry) continue
          invokeScope(scope, handler, entry, { scope, type: 'resync' }, myGen)
        }
      }
    }, MAX_MS)
    fallbackTimer = timer
  }
}

function setStatus(next) {
  if (status === next) return
  status = next
  updateFallbackTimer()
  for (const handler of [...statusHandlers]) {
    if (status !== next) break
    if (statusHandlers.has(handler)) invoke(handler, next)
  }
}

function emitScope(scope, payload, myGen) {
  const handlers = scopeHandlers.get(scope)
  if (!handlers) return
  for (const [handler, entry] of [...handlers]) {
    if (generation !== myGen || refCount <= 0) return
    if (scopeHandlers.get(scope)?.get(handler) === entry) invokeScope(scope, handler, entry, payload, myGen)
  }
}

function clearConnection() {
  clearTimeout(reconnectTimer)
  reconnectTimer = null
  ticketController?.abort()
  ticketController = null
  if (source) source.close()
  source = null
}

function retryAfterMs(response) {
  const value = response.headers?.get('Retry-After')
  if (!value) return 0
  const seconds = Number(value)
  const delay = Number.isFinite(seconds) ? seconds * 1000 : Date.parse(value) - Date.now()
  return Number.isFinite(delay) ? Math.max(0, delay) : 0
}

async function openStream() {
  const myGen = ++generation
  clearConnection()
  const controller = new AbortController()
  ticketController = controller
  let ticket
  let retryAfter = 0
  try {
    const response = await apiRequest('/events/ticket', { method: 'POST', silent: true, signal: controller.signal })
    if (!response.ok) retryAfter = retryAfterMs(response)
    else ticket = (await response.json())?.ticket
  } catch {
    // Сетевой отказ повторяем; отменённую/старую попытку отсекаем ниже.
  }
  if (myGen !== generation || refCount <= 0 || controller.signal.aborted) return
  ticketController = null
  if (!ticket) {
    scheduleReconnect(myGen, retryAfter)
    return
  }
  let localSource
  try {
    localSource = new EventSource(`${API_BASE_URL}/events?ticket=${encodeURIComponent(ticket)}`)
  } catch {
    scheduleReconnect(myGen)
    return
  }
  source = localSource
  const current = () => generation === myGen && source === localSource && refCount > 0
  let opened = false
  localSource.onopen = () => {
    if (!current() || opened) return
    opened = true
    attempts = 0
    const snapshot = [...scopeHandlers].map(([scope, handlers]) => [scope, [...handlers]])
    setStatus('connected')
    for (const [scope, handlers] of snapshot) {
      for (const [handler, entry] of handlers) {
        if (!current()) return
        if (scopeHandlers.get(scope)?.get(handler) === entry) invokeScope(scope, handler, entry, { scope, type: 'resync' }, myGen)
      }
    }
  }
  localSource.onmessage = (event) => {
    if (!current()) return
    let data
    try { data = JSON.parse(event.data) } catch { return }
    if (data?.scope) emitScope(data.scope, data, myGen)
  }
  localSource.addEventListener('reconnect', () => {
    if (!current()) return
    setStatus('reconnecting')
    if (current()) openStream()
  })
  localSource.onerror = () => {
    if (!current()) return
    localSource.close()
    source = null
    scheduleReconnect(myGen)
  }
}

function scheduleReconnect(myGen, retryAfter = 0) {
  if (myGen !== generation || refCount <= 0 || reconnectTimer !== null) return
  attempts += 1
  setStatus(attempts >= 3 ? 'fallback' : 'reconnecting')
  if (myGen !== generation || refCount <= 0) return
  const base = Math.min(BASE_MS * 2 ** Math.min(attempts - 1, 5), MAX_MS)
  // Cap ограничивает наш backoff, но не нижнюю границу Retry-After.
  const jitter = Math.max(BASE_MS, Math.min(MAX_MS, base * (0.8 + Math.random() * 0.4)))
  reconnectTimer = setTimeout(() => {
    if (myGen !== generation || refCount <= 0) return
    reconnectTimer = null
    openStream()
  }, Math.max(jitter, retryAfter))
}

function connect() {
  refCount += 1
  if (refCount === 1) openStream()
}

function disconnect() {
  refCount = Math.max(0, refCount - 1)
  if (refCount === 0) {
    generation += 1
    clearConnection()
    attempts = 0
    setStatus('disconnected')
    updateFallbackTimer()
  }
}

function subscribe(scope, handler, { fallback = false } = {}) {
  if (!scopeHandlers.has(scope)) scopeHandlers.set(scope, new Map())
  const entry = { fallback, running: false }
  scopeHandlers.get(scope).set(handler, entry)
  updateFallbackTimer()
  return () => {
    const handlers = scopeHandlers.get(scope)
    if (handlers?.get(handler) !== entry) return
    handlers.delete(handler)
    if (handlers.size === 0) scopeHandlers.delete(scope)
    updateFallbackTimer()
  }
}

function onStatus(handler) {
  statusHandlers.add(handler)
  invoke(handler, status)
  return () => statusHandlers.delete(handler)
}

function __resetForTests() {
  refCount = 0
  generation += 1
  clearConnection()
  clearInterval(fallbackTimer)
  fallbackTimer = null
  attempts = 0
  status = 'disconnected'
  scopeHandlers.clear()
  statusHandlers.clear()
}

export default { connect, disconnect, subscribe, onStatus }
export { __resetForTests }
