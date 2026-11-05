import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import eventStream, { __resetForTests } from '@/services/eventStream'
import { apiRequest } from '@/api/client'

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn(),
  API_BASE_URL: '/api',
}))

class MockEventSource {
  static instances = []
  constructor(url) {
    this.url = url
    this.listeners = {}
    this.closed = false
    this.onopen = null
    this.onmessage = null
    this.onerror = null
    MockEventSource.instances.push(this)
  }
  addEventListener(type, cb) {
    this.listeners[type] = cb
  }
  close() {
    this.closed = true
  }
  static last() {
    return this.instances[this.instances.length - 1]
  }
  static reset() {
    this.instances = []
  }
}

// apiRequest возвращает Response-подобный объект: данные достаются через res.json()
// (см. wrapJsonUnwrap в client.js). Мок обязан повторять этот контракт, иначе тест
// зелёный на форме, которой в проде нет.
const ticketOk = (ticket) => ({ ok: true, json: async () => ({ ticket }) })
const ticketFail = () => ({ ok: false, json: async () => ({}) })

// billet + res.json() = два await, плюс запас на реконнект-цепочки.
const flush = async () => {
  for (let i = 0; i < 5; i++) await Promise.resolve()
}

beforeEach(() => {
  MockEventSource.reset()
  vi.stubGlobal('EventSource', MockEventSource)
  apiRequest.mockReset()
  __resetForTests()
  vi.spyOn(Math, 'random').mockReturnValue(0.5)
})

afterEach(() => {
  __resetForTests()
  vi.useRealTimers()
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

describe('eventStream', () => {
  it('берёт билет (через res.json) и открывает поток с ним в URL', async () => {
    apiRequest.mockResolvedValue(ticketOk('abc123'))
    eventStream.connect()
    await flush()

    expect(apiRequest).toHaveBeenCalledWith('/events/ticket', { method: 'POST', silent: true, signal: expect.any(AbortSignal) })
    expect(MockEventSource.last().url).toBe('/api/events?ticket=abc123')
  })

  it('роутит сообщение по scope подписчику', async () => {
    apiRequest.mockResolvedValue(ticketOk('t'))
    const handler = vi.fn()
    eventStream.subscribe('applications-center', handler)
    eventStream.connect()
    await flush()

    MockEventSource.last().onmessage({
      data: JSON.stringify({ type: 'applications.refresh', scope: 'applications-center' }),
    })
    expect(handler).toHaveBeenCalledTimes(1)
    expect(handler.mock.calls[0][0].scope).toBe('applications-center')
  })

  it('игнорирует heartbeat/невалидный JSON без падения', async () => {
    apiRequest.mockResolvedValue(ticketOk('t'))
    const handler = vi.fn()
    eventStream.subscribe('applications-center', handler)
    eventStream.connect()
    await flush()

    expect(() => MockEventSource.last().onmessage({ data: ': ping' })).not.toThrow()
    expect(handler).not.toHaveBeenCalled()
  })

  it('onopen выставляет статус connected', async () => {
    apiRequest.mockResolvedValue(ticketOk('t'))
    const statuses = []
    eventStream.onStatus((s) => statuses.push(s))
    eventStream.connect()
    await flush()

    MockEventSource.last().onopen()
    expect(statuses).toContain('connected')
  })

  it('отписанный scope-подписчик не получает событий', async () => {
    apiRequest.mockResolvedValue(ticketOk('t'))
    const handler = vi.fn()
    const off = eventStream.subscribe('applications-center', handler)
    eventStream.connect()
    await flush()
    off()

    MockEventSource.last().onmessage({ data: JSON.stringify({ scope: 'applications-center' }) })
    expect(handler).not.toHaveBeenCalled()
  })

  it('disconnect закрывает поток только когда снят последний потребитель (refcount)', async () => {
    apiRequest.mockResolvedValue(ticketOk('t'))
    eventStream.connect()
    eventStream.connect()
    await flush()
    const es = MockEventSource.last()

    eventStream.disconnect()
    expect(es.closed).toBe(false)
    eventStream.disconnect()
    expect(es.closed).toBe(true)
  })

  it('disconnect во время получения билета не создаёт EventSource (гонка)', async () => {
    let resolveTicket
    apiRequest.mockReturnValue(new Promise((r) => { resolveTicket = r }))
    eventStream.connect()
    eventStream.disconnect() // отключились, пока билет ещё в пути
    resolveTicket(ticketOk('late'))
    await flush()

    expect(MockEventSource.instances.length).toBe(0)
  })

  it('onerror демоутит статус до reconnecting (поллинг подстрахует)', async () => {
    apiRequest.mockResolvedValue(ticketOk('t'))
    const statuses = []
    eventStream.onStatus((s) => statuses.push(s))
    eventStream.connect()
    await flush()
    MockEventSource.last().onopen()

    MockEventSource.last().onerror()
    expect(statuses).toContain('reconnecting')
    expect(statuses[statuses.length - 1]).not.toBe('connected')
  })

  it('после нескольких неудач билета уходит в fallback, затем восстанавливается в connected', async () => {
    vi.useFakeTimers()
    apiRequest.mockResolvedValue(ticketFail())
    const statuses = []
    eventStream.onStatus((s) => statuses.push(s))

    eventStream.connect() // #1 fail -> reconnecting
    await flush()
    await vi.advanceTimersByTimeAsync(1000) // #2 fail
    await vi.advanceTimersByTimeAsync(2000) // #3 fail -> fallback
    expect(statuses).toContain('fallback')

    // Билет снова выдаётся - следующая попытка поднимает поток.
    apiRequest.mockResolvedValue(ticketOk('ok'))
    await vi.advanceTimersByTimeAsync(4000)
    MockEventSource.last().onopen()
    expect(statuses[statuses.length - 1]).toBe('connected')
    vi.useRealTimers()
  })

  it('серверное событие reconnect переоткрывает поток новым билетом', async () => {
    apiRequest.mockResolvedValue(ticketOk('t1'))
    eventStream.connect()
    await flush()
    const first = MockEventSource.last()

    apiRequest.mockResolvedValue(ticketOk('t2'))
    first.listeners.reconnect()
    await flush()

    expect(first.closed).toBe(true)
    expect(MockEventSource.last().url).toBe('/api/events?ticket=t2')
  })
})

// Управляемые часы/случайность: никакого ожидания реальной сети или reconnect.
describe('eventStream recovery boundaries', () => {
  beforeEach(() => vi.useFakeTimers())

  it('все callbacks закрытого source игнорируются после замены', async () => {
    apiRequest.mockResolvedValue(ticketOk('one'))
    const handler = vi.fn()
    eventStream.subscribe('news', handler)
    eventStream.connect()
    await flush()
    const old = MockEventSource.last()
    old.listeners.reconnect()
    await flush()
    const current = MockEventSource.last()
    current.onopen()
    handler.mockClear()
    old.onopen()
    old.onmessage({ data: JSON.stringify({ scope: 'news' }) })
    old.onerror()
    old.listeners.reconnect()
    await vi.advanceTimersByTimeAsync(60000)
    expect(current.closed).toBe(false)
    expect(apiRequest).toHaveBeenCalledTimes(2)
    expect(handler).not.toHaveBeenCalled()
    const late = vi.fn()
    eventStream.onStatus(late)
    expect(late).toHaveBeenCalledWith('connected')
  })

  it('disconnect отменяет только собственный ticket, поздний ответ не создаёт source', async () => {
    let resolveOld
    apiRequest.mockReturnValueOnce(new Promise((resolve) => { resolveOld = resolve }))
      .mockResolvedValue(ticketOk('new'))
    eventStream.connect()
    const signal = apiRequest.mock.calls[0][1].signal
    eventStream.disconnect()
    expect(signal.aborted).toBe(true)
    eventStream.connect()
    await flush()
    resolveOld(ticketOk('old'))
    await flush()
    expect(MockEventSource.instances).toHaveLength(1)
    expect(MockEventSource.last().url).toContain('ticket=new')
    expect(apiRequest.mock.calls[1][1].signal.aborted).toBe(false)
  })

  it('дубли onerror не создают две reconnect цепочки; disconnect чистит timer', async () => {
    apiRequest.mockResolvedValue(ticketOk('t'))
    eventStream.connect()
    await flush()
    const old = MockEventSource.last()
    old.onerror()
    old.onerror()
    old.listeners.reconnect()
    await vi.advanceTimersByTimeAsync(1000)
    expect(apiRequest).toHaveBeenCalledTimes(2)
    MockEventSource.last().onerror()
    eventStream.disconnect()
    await vi.advanceTimersByTimeAsync(60000)
    expect(apiRequest).toHaveBeenCalledTimes(2)
  })

  it.each([429, 503])('Retry-After при %s может превышать cap собственного backoff', async (status) => {
    apiRequest.mockResolvedValueOnce({ ok: false, status, headers: { get: () => '75' } })
      .mockResolvedValue(ticketOk('later'))
    eventStream.connect()
    await flush()
    await vi.advanceTimersByTimeAsync(74999)
    expect(apiRequest).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(1)
    expect(apiRequest).toHaveBeenCalledTimes(2)
  })

  it('HTTP-date Retry-After отсчитывается от управляемого времени', async () => {
    vi.setSystemTime(new Date('2026-01-01T00:00:00Z'))
    apiRequest.mockResolvedValueOnce({ ok: false, headers: { get: () => 'Thu, 01 Jan 2026 00:01:00 GMT' } })
      .mockResolvedValue(ticketOk('later'))
    eventStream.connect()
    await flush()
    await vi.advanceTimersByTimeAsync(59999)
    expect(apiRequest).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(1)
    expect(apiRequest).toHaveBeenCalledTimes(2)
  })

  it.each([0, 1])('jitter с random=%s остаётся внутри 1..30s', async (random) => {
    Math.random.mockReturnValue(random)
    apiRequest.mockResolvedValue(ticketFail())
    eventStream.connect()
    await flush()
    for (let i = 0; i < 9; i++) {
      const before = apiRequest.mock.calls.length
      await vi.advanceTimersByTimeAsync(999)
      expect(apiRequest).toHaveBeenCalledTimes(before)
      await vi.advanceTimersToNextTimerAsync()
      expect(apiRequest).toHaveBeenCalledTimes(before + 1)
    }
    const before = apiRequest.mock.calls.length
    await vi.advanceTimersByTimeAsync(30000)
    expect(apiRequest).toHaveBeenCalledTimes(before + 1)
  })

  it('open resync один раз берёт snapshot и изолирует ошибки', async () => {
    apiRequest.mockResolvedValue(ticketOk('t'))
    const added = vi.fn()
    const removed = vi.fn()
    let off
    eventStream.subscribe('news', () => {
      off()
      eventStream.subscribe('news', added)
      throw new Error('consumer')
    })
    off = eventStream.subscribe('news', removed)
    const other = vi.fn().mockRejectedValue(new Error('async consumer'))
    eventStream.subscribe('available', other)
    eventStream.connect()
    await flush()
    const es = MockEventSource.last()
    es.onopen()
    es.onopen()
    await flush()
    expect(removed).not.toHaveBeenCalled()
    expect(added).not.toHaveBeenCalled()
    expect(other).toHaveBeenCalledTimes(1)
    expect(other).toHaveBeenCalledWith({ scope: 'available', type: 'resync' })
  })

  it('resync прекращается, если первый callback сменил сессию', async () => {
    apiRequest.mockResolvedValue(ticketOk('t'))
    eventStream.subscribe('news', () => eventStream.disconnect())
    const handler = vi.fn()
    eventStream.subscribe('available', handler)
    eventStream.connect()
    await flush()
    MockEventSource.last().onopen()
    expect(handler).not.toHaveBeenCalled()
  })

  it('один fallback timer, только opt-in, pending callback не перекрывает себя, cleanup', async () => {
    apiRequest.mockResolvedValue({ ok: false, headers: { get: () => '600' } })
    let finish
    const slow = vi.fn().mockImplementation(() => new Promise((resolve) => { finish = resolve }))
    const regular = vi.fn()
    const other = vi.fn()
    const offSlow = eventStream.subscribe('news', slow, { fallback: true })
    const offOther = eventStream.subscribe('available', other, { fallback: true })
    eventStream.subscribe('notifications', regular)
    eventStream.connect()
    await flush()
    expect(vi.getTimerCount()).toBe(2) // один reconnect и один общий fallback
    await vi.advanceTimersByTimeAsync(90000)
    expect(slow).toHaveBeenCalledTimes(1)
    expect(other).toHaveBeenCalledTimes(3)
    expect(regular).not.toHaveBeenCalled()
    finish()
    await flush()
    await vi.advanceTimersByTimeAsync(30000)
    expect(slow).toHaveBeenCalledTimes(2)
    offSlow()
    offOther()
    expect(vi.getTimerCount()).toBe(1)
    finish()
    await flush()
    eventStream.disconnect()
    expect(vi.getTimerCount()).toBe(0)
  })

  it('connected выключает fallback; поздний status получает connected сразу', async () => {
    apiRequest.mockResolvedValueOnce(ticketFail()).mockResolvedValue(ticketOk('ok'))
    const handler = vi.fn()
    eventStream.subscribe('news', handler, { fallback: true })
    eventStream.connect()
    await flush()
    await vi.advanceTimersByTimeAsync(1000)
    MockEventSource.last().onopen()
    handler.mockClear()
    const status = vi.fn()
    eventStream.onStatus(status)
    expect(status).toHaveBeenCalledExactlyOnceWith('connected')
    await vi.advanceTimersByTimeAsync(90000)
    expect(handler).not.toHaveBeenCalled()
    expect(vi.getTimerCount()).toBe(0)
  })
})

it('resync coalesces behind a pending fallback and is cancelled on unsubscribe', async () => {
  vi.useFakeTimers()
  apiRequest.mockResolvedValueOnce({ ok: false, headers: { get: () => '31' } })
    .mockResolvedValue(ticketOk('ok'))
  let finish
  const handler = vi.fn().mockImplementation(() => new Promise((resolve) => { finish = resolve }))
  const off = eventStream.subscribe('news', handler, { fallback: true })
  eventStream.connect()
  await flush()
  await vi.advanceTimersByTimeAsync(30000)
  expect(handler).toHaveBeenCalledTimes(1)
  await vi.advanceTimersByTimeAsync(1000)
  MockEventSource.last().onopen()
  expect(handler).toHaveBeenCalledTimes(1)
  finish()
  await flush()
  expect(handler).toHaveBeenCalledTimes(2)
  MockEventSource.last().onmessage({ data: JSON.stringify({ scope: 'news' }) })
  off()
  finish()
  await flush()
  expect(handler).toHaveBeenCalledTimes(2)
})
