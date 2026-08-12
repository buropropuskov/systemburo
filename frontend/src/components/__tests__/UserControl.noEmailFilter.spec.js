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
    // Прогоны по паролям не трогают архивные учётные записи, поэтому и здесь
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

describe('UserControl: очистка контактов', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('пустая почта уходит на сервер строкой, а не null', async () => {
    // `|| null` делал очистку невозможной: сервер трактует отсутствие ключа как
    // «не трогай поле», и стёртый адрес возвращался обратно при перезагрузке.
    const { apiRequest } = await import('@/api/client')
    const wrapper = mountUserControl()
    await flushPromises()

    await wrapper.vm.updateUserInfo({
      username: 'with_mail', pd_hidden: false, email: '', phone: '', position: 'Инженер',
    })
    await flushPromises()

    const call = apiRequest.mock.calls.find(([path]) => path === '/users/with_mail/info')
    expect(call).toBeTruthy()
    const payload = JSON.parse(call[1].body)
    expect(payload.email).toBe('')
    expect(payload.phone).toBe('')
  })

  it('скрытые до согласия контакты не отправляются вовсе', async () => {
    const { apiRequest } = await import('@/api/client')
    const wrapper = mountUserControl()
    await flushPromises()

    await wrapper.vm.updateUserInfo({
      username: 'without_mail', pd_hidden: true, email: '', position: 'Инженер',
    })
    await flushPromises()

    const call = apiRequest.mock.calls.find(([path]) => path === '/users/without_mail/info')
    const payload = JSON.parse(call[1].body)
    expect(payload).not.toHaveProperty('email')
  })
})
