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
        $router: { push: vi.fn(), replace: vi.fn().mockResolvedValue(undefined) },
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

describe('TheHeader — кнопка «Подать заявку» по праву header.create_application', () => {
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

  const submitBtn = (w) => w.find('[data-testid="header-button-submit-app"]')

  it('скрыта без права header.create_application', async () => {
    hasPermission.mockReturnValue(false)
    wrapper = mountHeader()
    await flushPromises()
    expect(submitBtn(wrapper).exists()).toBe(false)
  })

  it('видна при наличии header.create_application', async () => {
    hasPermission.mockImplementation((key) => key === 'header.create_application')
    wrapper = mountHeader()
    await flushPromises()
    expect(submitBtn(wrapper).exists()).toBe(true)
  })

  it('отвязана от page.new_application: только nav-право кнопку не показывает', async () => {
    hasPermission.mockImplementation((key) => key === 'page.new_application')
    wrapper = mountHeader()
    await flushPromises()
    expect(submitBtn(wrapper).exists()).toBe(false)
  })
})
