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
import ConfirmDialog from '@/components/ConfirmDialog.vue'
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
 * Панель уведомлений закрывается по клику на document, а глобальный вопрос подтверждения
 * телепортирован в body - его кнопки для шапки выглядят как «мимо панели». Клик по ответу
 * не должен уносить панель, из которой вопрос и был задан (#2058).
 *
 * Тест жмёт НАСТОЯЩУЮ кнопку диалога, а не document.body: ответ обнуляет confirmState
 * синхронно, ещё до всплытия клика, и проверка «вопрос сейчас открыт» на этом пути слепа.
 */
describe('TheHeader — панель уведомлений и глобальный вопрос подтверждения', () => {
  let wrapper
  let dialog
  beforeEach(() => {
    setActivePinia(createPinia())
    global.IntersectionObserver = class {
      observe() {}
      unobserve() {}
      disconnect() {}
    }
  })
  afterEach(() => {
    wrapper?.unmount()
    dialog?.unmount()
  })

  it('клик мимо панели закрывает её, пока вопрос не открыт', async () => {
    wrapper = mountHeader()
    await flushPromises()
    wrapper.vm.showNotifications = true

    document.body.click()
    await flushPromises()

    expect(wrapper.vm.showNotifications).toBe(false)
  })

  it('ответ «Очистить» в диалоге панель не закрывает', async () => {
    wrapper = mountHeader()
    dialog = mount(ConfirmDialog, { attachTo: document.body })
    await flushPromises()
    wrapper.vm.showNotifications = true

    const answered = useUiStore().confirm({ message: 'Все уведомления будут удалены.' })
    await flushPromises()

    document.querySelector('[data-testid="confirm-ok"]').click()
    await flushPromises()

    await expect(answered).resolves.toBe(true)
    expect(wrapper.vm.showNotifications).toBe(true)
  })

  it('отказ в диалоге панель не закрывает', async () => {
    wrapper = mountHeader()
    dialog = mount(ConfirmDialog, { attachTo: document.body })
    await flushPromises()
    wrapper.vm.showNotifications = true

    const answered = useUiStore().confirm({ message: 'Все уведомления будут удалены.' })
    await flushPromises()

    document.querySelector('[data-testid="confirm-cancel"]').click()
    await flushPromises()

    await expect(answered).resolves.toBe(false)
    expect(wrapper.vm.showNotifications).toBe(true)
  })

  it('клик по затемнению диалога закрывает вопрос, но не панель', async () => {
    wrapper = mountHeader()
    dialog = mount(ConfirmDialog, { attachTo: document.body })
    await flushPromises()
    wrapper.vm.showNotifications = true

    const answered = useUiStore().confirm({ message: 'Все уведомления будут удалены.' })
    await flushPromises()

    document.querySelector('[data-testid="confirm-overlay"]').click()
    await flushPromises()

    await expect(answered).resolves.toBe(false)
    expect(wrapper.vm.showNotifications).toBe(true)
  })
})
