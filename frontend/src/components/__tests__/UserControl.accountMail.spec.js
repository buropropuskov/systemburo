import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { nextTick } from 'vue'
import { createPinia, setActivePinia } from 'pinia'
import UserControl from '@/components/UserControl.vue'
import { apiRequest } from '@/api/client'

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

function mountUserControl() {
  return mount(UserControl, {
    props: { allUsers: [] },
    global: {
      mocks: {
        $bus: { on: vi.fn(), off: vi.fn(), emit: vi.fn() },
        $router: { push: vi.fn(), replace: vi.fn().mockResolvedValue(undefined) },
        $route: { path: '/admin/users', params: {} },
      },
    },
  })
}

describe('UserControl - пароль при заведении учётной записи', () => {
  let wrapper

  beforeEach(() => {
    setActivePinia(createPinia())
    apiRequest.mockClear()
  })

  afterEach(() => {
    wrapper?.unmount()
  })

  it('с адресом почты пароль необязателен: его придумает система', async () => {
    wrapper = mountUserControl()
    await flushPromises()

    await wrapper.setData({
      newUser: {
        ...wrapper.vm.newUser,
        username: 'newworker',
        type_id: 1,
        organization_id: 1,
        email: 'worker@example.org',
      },
    })

    expect(wrapper.vm.canCreateUser).toBeTruthy()
    expect(wrapper.vm.createUserHint).toBe('')
  })

  it('без адреса пустой пароль блокирует создание и подсказка называет обе замены', async () => {
    wrapper = mountUserControl()
    await flushPromises()

    await wrapper.setData({
      newUser: {
        ...wrapper.vm.newUser,
        username: 'newworker',
        type_id: 1,
        organization_id: 1,
      },
    })

    expect(wrapper.vm.canCreateUser).toBeFalsy()
    expect(wrapper.vm.createUserHint).toBe('Заполните: пароль или адрес почты')
  })

  it('пароль, заданный руками вместе с адресом, всё равно проверяется политикой', async () => {
    wrapper = mountUserControl()
    await flushPromises()

    await wrapper.setData({
      newUser: {
        ...wrapper.vm.newUser,
        username: 'newworker',
        type_id: 1,
        organization_id: 1,
        email: 'worker@example.org',
        password: 'abc',
      },
    })

    expect(wrapper.vm.canCreateUser).toBeFalsy()
    expect(wrapper.vm.createUserHint).toBe('Пароль не отвечает требованиям политики')
  })

  it('пустой пароль уходит на сервер пустой строкой вместе с адресом', async () => {
    wrapper = mountUserControl()
    await flushPromises()

    await wrapper.setData({
      newUser: {
        ...wrapper.vm.newUser,
        username: 'newworker',
        type_id: 1,
        organization_id: 1,
        email: 'worker@example.org',
      },
    })
    apiRequest.mockClear()
    await wrapper.vm.createUser()

    const call = apiRequest.mock.calls.find(([url]) => url === '/users')
    expect(call).toBeTruthy()
    const payload = JSON.parse(call[1].body)
    expect(payload.password).toBe('')
    expect(payload.email).toBe('worker@example.org')
  })

  it('подсказка про письмо появляется в форме, когда адрес заполнен', async () => {
    wrapper = mountUserControl()
    await flushPromises()

    // Модалку открываем присваиванием, как соседние спеки: setData на этом
    // поле до Teleport не доходит и окно остаётся неотрисованным.
    wrapper.vm.showCreateModal = true
    await nextTick()
    expect(document.querySelector('[data-testid="create-password-mail-note"]')).toBeNull()

    await wrapper.setData({
      newUser: { ...wrapper.vm.newUser, email: 'worker@example.org' },
    })
    await nextTick()

    const note = document.querySelector('[data-testid="create-password-mail-note"]')
    expect(note).not.toBeNull()
    expect(note.textContent).toContain('система придумает пароль')
  })
})

describe('UserControl - подсказка под сменой пароля в карточке', () => {
  let wrapper

  beforeEach(() => {
    setActivePinia(createPinia())
  })

  afterEach(() => {
    wrapper?.unmount()
  })

  it('с адресом обещает письмо и обязательную смену при первом входе', async () => {
    wrapper = mountUserControl()
    await flushPromises()

    await wrapper.setData({
      selectedUser: { username: 'worker', email: 'worker@example.org' },
    })

    expect(wrapper.vm.changePasswordNote).toContain('уйдёт работнику письмом')
    expect(wrapper.vm.changePasswordNote).toContain('при первом входе')
  })

  it('без адреса просит передать пароль лично', async () => {
    wrapper = mountUserControl()
    await flushPromises()

    await wrapper.setData({ selectedUser: { username: 'worker', email: '' } })

    expect(wrapper.vm.changePasswordNote).toContain('передайте пароль работнику лично')
  })

  // У работника без согласия на обработку данных сервер адреса не присылает.
  // Утверждать, что адреса нет, в этом случае нельзя - его просто не видно.
  it('скрытые персональные данные не выдаются за отсутствие адреса', async () => {
    wrapper = mountUserControl()
    await flushPromises()

    await wrapper.setData({
      selectedUser: { username: 'worker', email: '', pd_hidden: true },
    })

    expect(wrapper.vm.changePasswordNote).not.toContain('лично')
    expect(wrapper.vm.changePasswordNote).toContain('при первом входе')
  })
})
