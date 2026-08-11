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

  it('без адреса ряд контактов не показывает предупреждений', async () => {
    const wrapper = mountHeader({ email: '' })
    await flushPromises()

    // Владелец попросил убрать бейдж «Почта не указана»: пустая почта - обычное
    // состояние карточки, а не проблема, о которой надо кричать работнику.
    expect(wrapper.text()).not.toContain('Почта не указана')
  })

  it('с адресом показывает его и подсказку про бюро', async () => {
    const wrapper = mountHeader({ email: 'ivanov@example.org' })
    const contacts = useContactsStore()
    contacts.phone = '+7 900 000-00-00'
    contacts.email = 'buro@example.org'
    await flushPromises()

    expect(wrapper.text()).toContain('ivanov@example.org')
    expect(wrapper.vm.emailOwnerHint).toContain('бюро пропусков')
    expect(wrapper.vm.emailOwnerHint).toContain('+7 900 000-00-00')
  })

  it('без настроенных контактов подсказка остаётся осмысленной', async () => {
    const wrapper = mountHeader({ email: 'ivanov@example.org' })
    const contacts = useContactsStore()
    contacts.phone = ''
    contacts.email = ''
    await flushPromises()

    expect(wrapper.vm.emailOwnerHint).toContain('бюро пропусков')
    expect(wrapper.vm.emailOwnerHint).not.toContain('()')
  })
})

