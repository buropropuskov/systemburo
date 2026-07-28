import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import NotFound from '@/views/NotFound.vue'
import { useAuthStore } from '@/stores/auth'

// isAuthenticated читает exp из JWT, поэтому строкой-заглушкой не обойтись.
const liveToken = () => {
  const payload = btoa(JSON.stringify({ exp: Math.floor(Date.now() / 1000) + 3600 }))
  return `header.${payload}.signature`
}

const mountAt = (path) => {
  const push = []
  const wrapper = mount(NotFound, {
    global: {
      mocks: {
        $route: { path },
        $router: { push: (to) => push.push(to), back: () => push.push('back') },
      },
    },
  })
  return { wrapper, push }
}

describe('views/NotFound', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('показывает адрес, по которому юзер промахнулся', () => {
    const { wrapper } = mountAt('/kakoy-to-put')
    expect(wrapper.text()).toContain('404')
    expect(wrapper.text()).toContain('Страница не найдена')
    expect(wrapper.text()).toContain('/kakoy-to-put')
  })

  it('на явном /404 адрес не показывает - он ничего не объясняет', () => {
    const { wrapper } = mountAt('/404')
    expect(wrapper.text()).toContain('Страница не найдена')
    expect(wrapper.find('.not-found__path').exists()).toBe(false)
  })

  it('гостя отправляет на вход, авторизованного - в новости', async () => {
    const { wrapper, push } = mountAt('/kakoy-to-put')
    await wrapper.get('[data-testid="not-found-home"]').trigger('click')
    expect(push).toEqual(['/'])

    useAuthStore().token = liveToken()
    await wrapper.get('[data-testid="not-found-home"]').trigger('click')
    expect(push).toEqual(['/', '/news'])
  })
})
