import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { reactive } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

/**
 * Список уведомлений на шаге тура.
 *
 * Затемнение тура лежит выше шапки, поэтому список, открытый по просьбе шага,
 * оказывался за тёмной пеленой, а сам шаг оставался на колокольчике. Теперь про
 * список есть свой шаг: тур поднимает сигнал `reveal.open`, шапка открывает
 * список, и он подсвечивается как цель. Здесь проверяем сторону шапки.
 */

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: true, json: vi.fn().mockResolvedValue({}) }),
}))

vi.mock('@/stores/permissions', () => ({
  usePermissionsStore: () => ({ hasPermission: () => true, loaded: true, fetchPermissions: vi.fn() }),
}))

// Реактивный: computed шапки читает сигнал из стора, обычный объект его не двигает.
const onboardingState = reactive({ revealOpen: null })
vi.mock('@/stores/onboarding', () => ({ useOnboardingStore: () => onboardingState }))

import TheHeader from '@/components/TheHeader/TheHeader.vue'

function mountHeader() {
  return mount(TheHeader, {
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

describe('TheHeader - список уведомлений для онбординга', () => {
  let wrapper
  beforeEach(() => {
    setActivePinia(createPinia())
    onboardingState.revealOpen = null
    global.IntersectionObserver = class {
      observe() {}
      unobserve() {}
      disconnect() {}
    }
  })
  afterEach(() => wrapper?.unmount())

  it('сигнал тура открывает список, гашение - закрывает', async () => {
    wrapper = mountHeader()
    expect(wrapper.vm.showNotifications).toBe(false)

    onboardingState.revealOpen = 'notifications'
    await flushPromises()
    expect(wrapper.vm.showNotifications).toBe(true)

    onboardingState.revealOpen = null
    await flushPromises()
    expect(wrapper.vm.showNotifications).toBe(false)
  })

  it('список, открытый человеком по просьбе шага, тур закрывает за собой', async () => {
    wrapper = mountHeader()
    wrapper.vm.showNotifications = true
    await flushPromises()

    onboardingState.revealOpen = 'notifications'
    await flushPromises()
    expect(wrapper.vm.showNotifications).toBe(true)

    onboardingState.revealOpen = null
    await flushPromises()
    expect(wrapper.vm.showNotifications).toBe(false)
  })

  it('чужая цель раскрытия список не открывает', async () => {
    wrapper = mountHeader()
    onboardingState.revealOpen = 'search-panel'
    await flushPromises()
    expect(wrapper.vm.showNotifications).toBe(false)
  })

  it('пока список держит тур, клик по документу его не гасит', async () => {
    wrapper = mountHeader()
    onboardingState.revealOpen = 'notifications'
    await flushPromises()

    document.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await flushPromises()
    expect(wrapper.vm.showNotifications).toBe(true)
  })

  it('без тура клик по документу закрывает список как прежде', async () => {
    wrapper = mountHeader()
    wrapper.vm.showNotifications = true
    await flushPromises()

    document.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await flushPromises()
    expect(wrapper.vm.showNotifications).toBe(false)
  })
})
