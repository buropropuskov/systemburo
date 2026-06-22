import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: true, json: vi.fn().mockResolvedValue({}) }),
}))

const hasPermission = vi.fn()
vi.mock('@/stores/permissions', () => ({
  usePermissionsStore: () => ({ hasPermission, loaded: true, fetchPermissions: vi.fn() }),
}))

import TheHeader from '@/components/TheHeader/TheHeader.vue'

function mountHeader() {
  return mount(TheHeader, {
    global: {
      mocks: {
        $bus: { on: vi.fn(), off: vi.fn(), emit: vi.fn() },
        $router: { push: vi.fn() },
        $route: { path: '/' },
      },
      stubs: { FeedbackModal: true, AnnouncementModal: true, UserNotifications: true, SkeletonLine: true },
    },
  })
}

describe('TheHeader — кнопка «Сообщить о проблеме» по праву', () => {
  let wrapper
  beforeEach(() => {
    setActivePinia(createPinia())
    hasPermission.mockReset()
    global.IntersectionObserver = class {
      observe() {}
      unobserve() {}
      disconnect() {}
    }
  })
  afterEach(() => wrapper?.unmount())

  const feedbackBtn = (w) => w.find('[data-testid="header-button-feedback"]')

  it('скрыта без права header.report_problem', async () => {
    hasPermission.mockReturnValue(false)
    wrapper = mountHeader()
    await flushPromises()
    expect(feedbackBtn(wrapper).exists()).toBe(false)
  })

  it('видна при наличии права', async () => {
    hasPermission.mockImplementation((key) => key === 'header.report_problem')
    wrapper = mountHeader()
    await flushPromises()
    expect(feedbackBtn(wrapper).exists()).toBe(true)
  })
})
