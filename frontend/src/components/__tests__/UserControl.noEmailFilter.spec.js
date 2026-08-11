import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import UserControl from '@/components/UserControl.vue'

vi.mock('@/api/settings', () => ({
  getPasswordPolicy: vi.fn().mockResolvedValue({
    min_length: 8,
    require_letter: true,
    require_uppercase: false,
    require_lowercase: false,
    require_digit: true,
    require_special: false,
  }),
}))

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({
    ok: true,
    json: vi.fn().mockResolvedValue([]),
  }),
}))

vi.mock('@/api/onboarding', () => ({
  resetOnboardingForUser: vi.fn().mockResolvedValue({}),
}))

vi.mock('@/utils/notificationSound', () => ({
  playPreset: vi.fn(),
}))

const users = [
  { username: 'with_mail', is_active: true, email: 'ivanov@example.org', organization: 'Орг' },
  { username: 'without_mail', is_active: true, email: '', organization: 'Орг' },
  { username: 'null_mail', is_active: true, email: null, organization: 'Орг' },
  { username: 'archived_no_mail', is_active: false, email: '', organization: 'Орг' },
]

function mountUserControl() {
  return mount(UserControl, {
    props: { allUsers: users },
    global: { stubs: { teleport: true } },
  })
}

describe('UserControl: режим «Без почты»', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('режим есть в переключателе списка', async () => {
    const wrapper = mountUserControl()
    await flushPromises()
    expect(wrapper.vm.archiveOptions.map((o) => o.value)).toContain('no_email')
  })

  it('показывает только активных работников без адреса', async () => {
    const wrapper = mountUserControl()
    await flushPromises()

    wrapper.vm.listMode = 'no_email'
    await flushPromises()

    const shown = wrapper.vm.filteredUsers.map((u) => u.username)
    expect(shown).toEqual(['without_mail', 'null_mail'])
    // Плановая смена паролей не трогает архивные учётные записи, поэтому и здесь
    // их быть не должно - иначе бюро будет добивать адреса уволенным.
    expect(shown).not.toContain('archived_no_mail')
  })

  it('обычный режим показывает всех активных', async () => {
    const wrapper = mountUserControl()
    await flushPromises()

    expect(wrapper.vm.filteredUsers.map((u) => u.username))
      .toEqual(['with_mail', 'without_mail', 'null_mail'])
  })

  it('подпись счётчика меняется на «Без почты»', async () => {
    const wrapper = mountUserControl()
    await flushPromises()

    wrapper.vm.listMode = 'no_email'
    await flushPromises()
    expect(wrapper.vm.countLabel).toBe('Без почты')
  })
})
