import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'

// #974: navigator.serviceWorker и Notification нет в jsdom - реальные обёртки
// над ними (utils/webPushSubscription.js) мокаются целиком, а Notification
// подменяется глобально, чтобы composable мог прочитать permission/дёрнуть
// requestPermission как в настоящем браузере.

vi.mock('@/api/webPush', () => ({
  getWebPushStatus: vi.fn(),
  subscribeWebPush: vi.fn(),
  unsubscribeWebPush: vi.fn(),
}))
vi.mock('@/utils/webPushSubscription', () => ({
  isPushSupported: vi.fn(),
  getCurrentSubscription: vi.fn(),
  subscribeToPush: vi.fn(),
  unsubscribeLocal: vi.fn(),
}))
vi.mock('@/utils/webPushPlatform', () => ({
  needsIosHomeScreenInstall: vi.fn(() => false),
  iosNeedsSafari: vi.fn(() => false),
}))
const notify = vi.fn()
vi.mock('@/stores/deletions', () => ({ useDeletionsStore: () => ({ notify }) }))

import WebPushSettings from '../WebPushSettings.vue'
import { getWebPushStatus, subscribeWebPush, unsubscribeWebPush } from '@/api/webPush'
import {
  isPushSupported,
  getCurrentSubscription,
  subscribeToPush,
  unsubscribeLocal,
} from '@/utils/webPushSubscription'
import { needsIosHomeScreenInstall, iosNeedsSafari } from '@/utils/webPushPlatform'

function mountBlock() {
  return mount(WebPushSettings)
}

describe('WebPushSettings', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    notify.mockClear()
    isPushSupported.mockReturnValue(true)
    needsIosHomeScreenInstall.mockReturnValue(false)
    // Возврат мока clearAllMocks не трогает - без явного сброса состояние iOS-подсказки
    // из предыдущего теста утекает в следующий.
    iosNeedsSafari.mockReturnValue(false)
    getCurrentSubscription.mockResolvedValue(null)
    getWebPushStatus.mockResolvedValue({ public_key: 'server-key', enabled: true, devices: [] })
    vi.stubGlobal('Notification', { permission: 'default', requestPermission: vi.fn().mockResolvedValue('granted') })
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    vi.restoreAllMocks()
  })

  it('браузер без поддержки push - своё сообщение, кнопки нет', async () => {
    isPushSupported.mockReturnValue(false)
    const wrapper = mountBlock()
    await flushPromises()

    expect(wrapper.find('[data-testid="webpush-unsupported"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="webpush-enable"]').exists()).toBe(false)
  })

  it('iOS вне экрана "Домой" - пошаговая инструкция приоритетнее фичедетекта, формулировка без "только iPhone"', async () => {
    needsIosHomeScreenInstall.mockReturnValue(true)
    const wrapper = mountBlock()
    await flushPromises()

    const hint = wrapper.find('[data-testid="webpush-ios-hint"]')
    expect(hint.text()).toContain('iOS (iPhone и iPad)')
    expect(hint.text()).toContain('экран «Домой»')
    expect(hint.findAll('.webpush__steps li')).toHaveLength(4)
    expect(hint.text()).toContain('Поделиться')
    expect(wrapper.find('[data-testid="webpush-enable"]').exists()).toBe(false)
  })

  // Живой iPhone (#974): зашли через Chrome и получили инструкцию для Safari, которую
  // из Chrome выполнить нельзя - ярлык оттуда push не даёт.
  it('сторонний браузер на iOS - ведём в Safari, а не предлагаем ставить на «Домой»', async () => {
    iosNeedsSafari.mockReturnValue(true)
    needsIosHomeScreenInstall.mockReturnValue(false)
    const wrapper = mountBlock()
    await flushPromises()

    const hint = wrapper.find('[data-testid="webpush-ios-safari-hint"]')
    expect(hint.exists()).toBe(true)
    // Текст обязан снимать возражение «я принципиально не пользуюсь Safari»: он нужен
    // один раз для установки, повседневный браузер остаётся прежним.
    expect(hint.text()).toContain('Менять браузер не нужно')
    expect(hint.text()).toContain('один раз')
    expect(hint.find('[data-testid="webpush-copy-url"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="webpush-ios-hint"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="webpush-enable"]').exists()).toBe(false)
  })

  it('сервер не настроил push - блок неактивен с честной подписью', async () => {
    getWebPushStatus.mockResolvedValue({ public_key: null, enabled: false, devices: [] })
    const wrapper = mountBlock()
    await flushPromises()

    expect(wrapper.find('[data-testid="webpush-not-configured"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="webpush-enable"]').exists()).toBe(false)
  })

  it('разрешение запрещено в браузере - кнопка недоступна, есть подсказка про замок', async () => {
    vi.stubGlobal('Notification', { permission: 'denied', requestPermission: vi.fn() })
    const wrapper = mountBlock()
    await flushPromises()

    expect(wrapper.find('[data-testid="webpush-denied"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="webpush-denied"]').text()).toContain('замка')
    expect(wrapper.find('[data-testid="webpush-enable"]').exists()).toBe(false)
  })

  it('разрешение не запрошено - "Включить" запрашивает разрешение и шлёт подписку на сервер', async () => {
    const fakeSubscription = {
      endpoint: 'https://push.example/device-1',
      toJSON: () => ({ keys: { p256dh: 'p-key', auth: 'a-key' } }),
    }
    subscribeToPush.mockResolvedValue(fakeSubscription)
    subscribeWebPush.mockResolvedValue({ id: 1 })

    const wrapper = mountBlock()
    await flushPromises()
    expect(wrapper.find('[data-testid="webpush-default"]').exists()).toBe(true)

    await wrapper.find('[data-testid="webpush-enable"]').trigger('click')
    await flushPromises()

    expect(Notification.requestPermission).toHaveBeenCalled()
    expect(subscribeToPush).toHaveBeenCalledWith('server-key')
    expect(subscribeWebPush).toHaveBeenCalledWith({
      endpoint: 'https://push.example/device-1',
      p256dh: 'p-key',
      auth: 'a-key',
    })
    expect(wrapper.find('[data-testid="webpush-enabled"]').exists()).toBe(true)
  })

  it('сервер отказал регистрировать подписку - локальная подписка снимается, уведомление об ошибке', async () => {
    const fakeSubscription = { endpoint: 'https://push.example/device-1', toJSON: () => ({ keys: {} }) }
    subscribeToPush.mockResolvedValue(fakeSubscription)
    subscribeWebPush.mockRejectedValue(new Error('Сервер занят'))

    const wrapper = mountBlock()
    await flushPromises()
    await wrapper.find('[data-testid="webpush-enable"]').trigger('click')
    await flushPromises()

    expect(unsubscribeLocal).toHaveBeenCalledWith(fakeSubscription)
    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ type: 'error' }))
    expect(wrapper.find('[data-testid="webpush-default"]').exists()).toBe(true)
  })

  it('включено на устройстве - "Отключить" зовёт unsubscribe в браузере И удаление на сервере', async () => {
    const fakeSubscription = { endpoint: 'https://push.example/device-1', toJSON: () => ({ keys: {} }) }
    getCurrentSubscription.mockResolvedValue(fakeSubscription)
    unsubscribeWebPush.mockResolvedValue({})

    const wrapper = mountBlock()
    await flushPromises()
    expect(wrapper.find('[data-testid="webpush-enabled"]').exists()).toBe(true)

    await wrapper.find('[data-testid="webpush-disable"]').trigger('click')
    await flushPromises()

    expect(unsubscribeWebPush).toHaveBeenCalledWith('https://push.example/device-1')
    expect(unsubscribeLocal).toHaveBeenCalledWith(fakeSubscription)
    expect(wrapper.find('[data-testid="webpush-default"]').exists()).toBe(true)
  })

  it('список устройств рисует user agent и даты добавления/доставки', async () => {
    const fakeSubscription = { endpoint: 'https://push.example/device-1', toJSON: () => ({ keys: {} }) }
    getCurrentSubscription.mockResolvedValue(fakeSubscription)
    getWebPushStatus.mockResolvedValue({
      public_key: 'server-key',
      enabled: true,
      devices: [{
        id: 1,
        user_agent: 'Mozilla/5.0 Chrome/124',
        created_at: '2026-08-01T10:00:00Z',
        last_success_at: null,
      }],
    })

    const wrapper = mountBlock()
    await flushPromises()

    const devicesText = wrapper.find('.webpush__devices').text()
    expect(devicesText).toContain('Chrome/124')
    expect(devicesText).toContain('никогда')
  })
})
