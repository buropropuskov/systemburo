import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import UserProfileHeader from '@/components/UserProfileHeader.vue'
import { useContactsStore } from '@/stores/contacts'

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
    props: { lastName: 'Иванов', firstName: 'Иван', userType: 'user', ...props },
    global: { stubs: { teleport: true, ChangePasswordModal: true } },
  })
}

describe('UserProfileHeader: адрес почты', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('без адреса показывает предупреждение, а не пустое место', async () => {
    const wrapper = mountHeader({ email: '' })
    await flushPromises()

    const badge = wrapper.find('[data-testid="cabinet-email-missing"]')
    expect(badge.exists()).toBe(true)
    expect(badge.text()).toContain('Почта не указана')
  })

  it('с адресом предупреждения нет', async () => {
    const wrapper = mountHeader({ email: 'ivanov@example.org' })
    await flushPromises()

    expect(wrapper.find('[data-testid="cabinet-email-missing"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('ivanov@example.org')
  })

  it('подсказка отправляет в бюро и подставляет его контакты', async () => {
    const wrapper = mountHeader({ email: '' })
    const contacts = useContactsStore()
    contacts.phone = '+7 900 000-00-00'
    contacts.email = 'buro@example.org'
    await flushPromises()

    expect(wrapper.vm.noEmailHint).toContain('обратитесь в бюро пропусков')
    expect(wrapper.vm.noEmailHint).toContain('+7 900 000-00-00')
    expect(wrapper.vm.noEmailHint).toContain('buro@example.org')
  })

  it('без настроенных контактов подсказка остаётся осмысленной', async () => {
    const wrapper = mountHeader({ email: '' })
    const contacts = useContactsStore()
    contacts.phone = ''
    contacts.email = ''
    await flushPromises()

    expect(wrapper.vm.noEmailHint).toContain('обратитесь в бюро пропусков')
    expect(wrapper.vm.noEmailHint).not.toContain('()')
  })
})
