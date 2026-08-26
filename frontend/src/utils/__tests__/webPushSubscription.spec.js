import { describe, it, expect, vi, afterEach } from 'vitest'
import { isPushSupported, urlBase64ToUint8Array } from '../webPushSubscription'

describe('isPushSupported', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
    delete navigator.serviceWorker
  })

  it('true, когда serviceWorker/PushManager/Notification есть', () => {
    vi.stubGlobal('PushManager', function PushManager() {})
    vi.stubGlobal('Notification', { permission: 'default' })
    Object.defineProperty(navigator, 'serviceWorker', { value: {}, configurable: true })

    expect(isPushSupported()).toBe(true)
  })

  it('false без PushManager (например, старый браузер)', () => {
    vi.stubGlobal('Notification', { permission: 'default' })
    Object.defineProperty(navigator, 'serviceWorker', { value: {}, configurable: true })

    expect(isPushSupported()).toBe(false)
  })

  it('false без serviceWorker в navigator', () => {
    vi.stubGlobal('PushManager', function PushManager() {})
    vi.stubGlobal('Notification', { permission: 'default' })
    delete navigator.serviceWorker

    expect(isPushSupported()).toBe(false)
  })
})

describe('urlBase64ToUint8Array', () => {
  it('декодирует base64url VAPID-ключ в Uint8Array ожидаемой длины', () => {
    // "hello" в стандартном base64 - 'aGVsbG8=', base64url той же строки без padding.
    const result = urlBase64ToUint8Array('aGVsbG8')
    expect(result).toBeInstanceOf(Uint8Array);
    expect(Array.from(result)).toEqual([104, 101, 108, 108, 111]) // 'hello' по кодам символов
  })

  it('корректно обрабатывает "-" и "_" (base64url-алфавит)', () => {
    // байты [0xfb, 0xff] -> стандартный base64 "+/8=", base64url "-_8".
    const result = urlBase64ToUint8Array('-_8')
    expect(Array.from(result)).toEqual([0xfb, 0xff])
  })
})
