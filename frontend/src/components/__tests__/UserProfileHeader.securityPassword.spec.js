import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import UserProfileHeader from '@/components/UserProfileHeader.vue'

vi.mock('@/api/pdConsent', () => ({
  listMyConsents: vi.fn().mockResolvedValue([]),
  revokeMyConsent: vi.fn().mockResolvedValue({}),
}))

vi.mock('@/api/settings', () => ({
  getPublicContacts: vi.fn().mockResolvedValue({ phone: '+7 900 000-00-00', email: 'buro@example.org' }),
  getPasswordPolicy: vi.fn().mockResolvedValue({ min_length: 8, require_letter: true, require_digit: true }),
}))

function mountHeader(props = {}) {
  return mount(UserProfileHeader, {
    props: { lastName: 'Киселёва', firstName: 'Юлия', userType: 'user', ...props },
    global: { stubs: { teleport: true, ChangePasswordModal: true } },
  })
}

// Пароль работника поста ведёт бюро пропусков (#2280). Кнопка в кабинете - не
// единственная защита (сервер отклоняет и прямой запрос), но показывать её значит
// обещать действие, которое закончится отказом.
describe('UserProfileHeader: смена пароля работником поста', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('охраннику кнопку смены пароля не показывает', async () => {
    const wrapper = mountHeader({ userType: 'security' })
    await flushPromises()

    expect(wrapper.find('[data-testid="cabinet-change-password"]').exists()).toBe(false)
  })

  it('остальным кнопка остаётся', async () => {
    const wrapper = mountHeader({ userType: 'user' })
    await flushPromises()

    expect(wrapper.find('[data-testid="cabinet-change-password"]').exists()).toBe(true)
  })
})
