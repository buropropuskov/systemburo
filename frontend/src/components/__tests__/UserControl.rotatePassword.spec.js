import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import UserControl from '@/components/UserControl.vue'

vi.mock('@/api/settings', () => ({
  getPasswordPolicy: vi.fn().mockResolvedValue({
    min_length: 8, require_letter: true, require_digit: true,
  }),
}))

const apiRequest = vi.fn()
vi.mock('@/api/client', () => ({ apiRequest: (...args) => apiRequest(...args) }))
vi.mock('@/api/onboarding', () => ({ resetOnboardingForUser: vi.fn().mockResolvedValue({}) }))
vi.mock('@/utils/notificationSound', () => ({ playPreset: vi.fn() }))

const notify = vi.fn()
vi.mock('@/stores/deletions', () => ({ useDeletionsStore: () => ({ notify }) }))

const user = { username: 'ivanov', is_active: true, email: 'ivanov@example.org' }

function mountUserControl() {
  return mount(UserControl, {
    props: { allUsers: [user] },
    global: { stubs: { teleport: true } },
  })
}

describe('UserControl: смена пароля с отправкой письмом', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    apiRequest.mockResolvedValue({ ok: true, json: vi.fn().mockResolvedValue([]) })
  })

  it('дёргает ручку смены пароля работника и сообщает об отправке', async () => {
    const wrapper = mountUserControl()
    await flushPromises()

    await wrapper.vm.rotateUserPassword(user)
    await flushPromises()

    const calls = apiRequest.mock.calls.filter(([path]) => path === '/users/ivanov/rotate-password')
    expect(calls).toHaveLength(1)
    expect(calls[0][1]).toMatchObject({ method: 'POST' })
    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ bold: 'ivanov' }))
  })

  it('не показывает пароль в интерфейсе', async () => {
    const wrapper = mountUserControl()
    await flushPromises()

    await wrapper.vm.rotateUserPassword(user)
    await flushPromises()

    // Пароль уходит владельцу учётной записи, а не тому, кто нажал кнопку:
    // в уведомлении его быть не должно.
    const messages = notify.mock.calls.map(([arg]) => JSON.stringify(arg)).join(' ')
    expect(messages).not.toMatch(/пароль:/i)
  })

  it('отказ сервера показывается текстом', async () => {
    apiRequest.mockImplementation((path) => {
      if (path === '/users/ivanov/rotate-password') {
        return Promise.resolve({
          ok: false,
          json: vi.fn().mockResolvedValue({ message: 'у работника не указан адрес почты' }),
        })
      }
      return Promise.resolve({ ok: true, json: vi.fn().mockResolvedValue([]) })
    })
    const wrapper = mountUserControl()
    await flushPromises()

    await wrapper.vm.rotateUserPassword(user)
    await flushPromises()

    expect(notify).toHaveBeenCalledWith(expect.objectContaining({
      bold: 'у работника не указан адрес почты',
      type: 'error',
    }))
  })
})
