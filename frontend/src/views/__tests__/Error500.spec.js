import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import Error500 from '@/views/Error500.vue'
import { loadBugContext } from '@/composables/useBugReport'
import { useAuthStore } from '@/stores/auth'

vi.mock('@/api/client', () => ({ apiRequest: vi.fn() }))
vi.mock('@/composables/useBugReport', () => ({
  loadBugContext: vi.fn(),
  buildBugHash: vi.fn().mockResolvedValue('deadbeefdeadbeef'),
  isReported: vi.fn(() => false),
  markReported: vi.fn(),
}))

// isAuthenticated читает exp из JWT, поэтому строкой-заглушкой не обойтись.
const liveToken = () => {
  const payload = btoa(JSON.stringify({ exp: Math.floor(Date.now() / 1000) + 3600 }))
  return `header.${payload}.signature`
}

const context = (uiRoute) => ({
  route: 'GET /applications?page=2',
  httpStatus: 500,
  message: 'Internal Server Error',
  timestamp: new Date().toISOString(),
  ...(uiRoute === undefined ? {} : { uiRoute }),
})

async function mountPage(ctx) {
  loadBugContext.mockReturnValue(ctx)
  const router = { push: vi.fn(), replace: vi.fn() }
  const wrapper = mount(Error500, { global: { mocks: { $router: router } } })
  await flushPromises()
  return { wrapper, router }
}

describe('views/Error500', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('повтор возвращает на страницу, где упал запрос, не оставляя /500 в истории', async () => {
    const { wrapper, router } = await mountPage(context('/center?status=new'))
    await wrapper.get('[data-testid="error-500-retry"]').trigger('click')
    expect(router.replace).toHaveBeenCalledWith('/center?status=new')
    expect(router.push).not.toHaveBeenCalled()
  })

  it('называет упавшую страницу в карточке инцидента', async () => {
    const { wrapper } = await mountPage(context('/center?status=new'))
    expect(wrapper.get('.err500__code').text()).toContain('PAGE     /center?status=new')
  })

  it('не предлагает повтор, когда упала та же страница, куда ведёт "На главную"', async () => {
    useAuthStore().token = liveToken()
    const { wrapper } = await mountPage(context('/news'))
    expect(wrapper.find('[data-testid="error-500-retry"]').exists()).toBe(false)
  })

  it('не предлагает повтор по контексту без адреса страницы', async () => {
    const { wrapper } = await mountPage(context())
    expect(wrapper.find('[data-testid="error-500-retry"]').exists()).toBe(false)
  })

  it('не предлагает повторить саму страницу инцидента', async () => {
    const { wrapper } = await mountPage(context('/500'))
    expect(wrapper.find('[data-testid="error-500-retry"]').exists()).toBe(false)
  })

  it('гостя отправляет на вход, авторизованного - в новости', async () => {
    const { wrapper, router } = await mountPage(context('/center'))
    await wrapper.get('[data-testid="error-500-home"]').trigger('click')
    expect(router.push).toHaveBeenLastCalledWith('/')

    useAuthStore().token = liveToken()
    await wrapper.get('[data-testid="error-500-brand"]').trigger('click')
    expect(router.push).toHaveBeenLastCalledWith('/news')
  })
})
