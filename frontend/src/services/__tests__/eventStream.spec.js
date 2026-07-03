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
})

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('eventStream', () => {
  it('берёт билет (через res.json) и открывает поток с ним в URL', async () => {
    apiRequest.mockResolvedValue(ticketOk('abc123'))
    eventStream.connect()
    await flush()

    expect(apiRequest).toHaveBeenCalledWith('/events/ticket', { method: 'POST' })
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
