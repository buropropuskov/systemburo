import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'

// #974: после успешного входа компонент раньше всегда уводил на /news. Если
// гард привёл сюда с защищённого адреса (ссылка/push-уведомление без сессии),
// человек должен попасть именно туда - адрес лежит в query.redirect.

vi.mock('@/api/client', () => ({ apiRequest: vi.fn() }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ setTokens: vi.fn() }) }))
vi.mock('@/stores/contacts', () => ({ useContactsStore: () => ({ fetch: vi.fn(), email: '', phone: '' }) }))

import LoginComponent from '@/components/LoginComponent.vue'
import { apiRequest } from '@/api/client'

function okResp(token) {
  return {
    ok: true,
    status: 200,
    statusText: '',
    headers: { get: () => null },
    json: async () => ({ token }),
    text: async () => JSON.stringify({ token }),
  }
}

function mountLogin(query = {}) {
  return mount(LoginComponent, {
    global: {
      stubs: { PasswordRecoveryModal: true },
      mocks: {
        $router: { push: vi.fn() },
        $route: { query },
      },
    },
  })
}

async function submit(wrapper) {
  wrapper.vm.formData.username = 'ivanov'
  wrapper.vm.formData.password = 'secret'
  const p = wrapper.vm.handleSubmit()
  // handleSubmit ждёт 100мс до запроса, затем 1500мс анимации успеха до push.
  await vi.advanceTimersByTimeAsync(2000)
  await p
}

describe('LoginComponent — возврат на исходный адрес после входа (#974)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    localStorage.clear()
    vi.useFakeTimers()
  })
  afterEach(() => {
    vi.useRealTimers()
    vi.restoreAllMocks()
  })

  it('с query.redirect уводит на сохранённый защищённый адрес', async () => {
    apiRequest.mockResolvedValue(okResp('tok'))
    const wrapper = mountLogin({ redirect: '/table/cars' })
    await submit(wrapper)

    expect(wrapper.vm.$router.push).toHaveBeenCalledWith('/table/cars')
    wrapper.unmount()
  })

  it('без query.redirect уводит на /news как раньше', async () => {
    apiRequest.mockResolvedValue(okResp('tok'))
    const wrapper = mountLogin()
    await submit(wrapper)

    expect(wrapper.vm.$router.push).toHaveBeenCalledWith('/news')
    wrapper.unmount()
  })

  it('небезопасный query.redirect игнорируется, уводит на /news', async () => {
    apiRequest.mockResolvedValue(okResp('tok'))
    const wrapper = mountLogin({ redirect: 'https://evil.example' })
    await submit(wrapper)

    expect(wrapper.vm.$router.push).toHaveBeenCalledWith('/news')
    wrapper.unmount()
  })
})
