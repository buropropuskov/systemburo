import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: true, json: vi.fn().mockResolvedValue({}) }),
}))

vi.mock('@/stores/permissions', () => ({
  usePermissionsStore: () => ({ hasPermission: () => true, loaded: true, fetchPermissions: vi.fn() }),
}))

import TheHeader from '@/components/TheHeader/TheHeader.vue'
import { useUiStore } from '@/stores/ui'

function mountHeader() {
  return mount(TheHeader, {
    attachTo: document.body,
    global: {
      mocks: {
        $bus: { on: vi.fn(), off: vi.fn(), emit: vi.fn() },
        $router: { push: vi.fn(), replace: vi.fn().mockResolvedValue(undefined) },
        $route: { path: '/' },
      },
      stubs: { FeedbackModal: true, AnnouncementModal: true, UserNotifications: true, SkeletonLine: true },
    },
  })
}

/**
 * Глобальный ConfirmDialog телепортирован в body, поэтому клик по его кнопкам для
 * шапки выглядит как «мимо панели». Без гейта панель уведомлений закрывалась бы ровно
 * в тот момент, когда человек отвечает на её же вопрос об очистке (#2058).
 */
describe('TheHeader — панель уведомлений и глобальный вопрос подтверждения', () => {
  let wrapper
  beforeEach(() => {
    setActivePinia(createPinia())
    global.IntersectionObserver = class {
      observe() {}
      unobserve() {}
      disconnect() {}
    }
  })
  afterEach(() => wrapper?.unmount())

  it('клик мимо панели закрывает её, пока вопрос не открыт', async () => {
    wrapper = mountHeader()
    await flushPromises()
    wrapper.vm.showNotifications = true

    document.body.click()
    await flushPromises()

    expect(wrapper.vm.showNotifications).toBe(false)
  })

  it('клик по кнопкам открытого вопроса панель не закрывает', async () => {
    wrapper = mountHeader()
    await flushPromises()
    wrapper.vm.showNotifications = true
    useUiStore().confirm({ message: 'Все уведомления будут удалены.' })

    document.body.click()
    await flushPromises()

    expect(wrapper.vm.showNotifications).toBe(true)
  })
})
