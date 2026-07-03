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

const flush = () => Promise.resolve()

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
  it('берёт билет и открывает поток с ним в URL', async () => {
    apiRequest.mockResolvedValue({ ticket: 'abc123' })
    eventStream.connect()
    await flush()

    expect(apiRequest).toHaveBeenCalledWith('/events/ticket', { method: 'POST' })
    expect(MockEventSource.last().url).toBe('/api/events?ticket=abc123')
  })

  it('роутит сообщение по scope подписчику', async () => {
    apiRequest.mockResolvedValue({ ticket: 't' })
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
    apiRequest.mockResolvedValue({ ticket: 't' })
    const handler = vi.fn()
    eventStream.subscribe('applications-center', handler)
    eventStream.connect()
    await flush()

    expect(() => MockEventSource.last().onmessage({ data: ': ping' })).not.toThrow()
    expect(handler).not.toHaveBeenCalled()
  })

  it('onopen выставляет статус connected', async () => {
    apiRequest.mockResolvedValue({ ticket: 't' })
    const statuses = []
    eventStream.onStatus((s) => statuses.push(s))
    eventStream.connect()
    await flush()

    MockEventSource.last().onopen()
    expect(statuses).toContain('connected')
  })

  it('не отписанный scope-подписчик не получает событий после unsubscribe', async () => {
    apiRequest.mockResolvedValue({ ticket: 't' })
    const handler = vi.fn()
    const off = eventStream.subscribe('applications-center', handler)
    eventStream.connect()
    await flush()
    off()

    MockEventSource.last().onmessage({
      data: JSON.stringify({ scope: 'applications-center' }),
    })
    expect(handler).not.toHaveBeenCalled()
  })

  it('disconnect закрывает поток только когда снят последний потребитель (refcount)', async () => {
    apiRequest.mockResolvedValue({ ticket: 't' })
    eventStream.connect()
    eventStream.connect()
    await flush()
    const es = MockEventSource.last()

    eventStream.disconnect()
    expect(es.closed).toBe(false) // ещё один потребитель держит
    eventStream.disconnect()
    expect(es.closed).toBe(true)
  })

  it('после нескольких неудач билета уходит в fallback', async () => {
    vi.useFakeTimers()
    apiRequest.mockRejectedValue(new Error('no ticket'))
    const statuses = []
    eventStream.onStatus((s) => statuses.push(s))

    eventStream.connect() // openStream #1 (reject -> reconnect #1)
    await flush()
    await vi.advanceTimersByTimeAsync(1000) // #2
    await vi.advanceTimersByTimeAsync(2000) // #3 -> fallback

    expect(statuses).toContain('fallback')
    vi.useRealTimers()
  })

  it('серверное событие reconnect переоткрывает поток новым билетом', async () => {
    apiRequest.mockResolvedValue({ ticket: 't1' })
    eventStream.connect()
    await flush()
    const first = MockEventSource.last()

    apiRequest.mockResolvedValue({ ticket: 't2' })
    first.listeners.reconnect() // сервер закрыл по maxLifetime
    await flush()

    expect(first.closed).toBe(true)
    expect(MockEventSource.last().url).toBe('/api/events?ticket=t2')
  })
})
