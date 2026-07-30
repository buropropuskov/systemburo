import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { nextTick } from 'vue'
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

function mountUserControl() {
  return mount(UserControl, {
    props: { allUsers: [] },
    global: {
      mocks: {
        $bus: { on: vi.fn(), off: vi.fn(), emit: vi.fn() },
        $router: { push: vi.fn() },
        $route: { path: '/admin/users', params: {} },
      },
    },
  })
}

describe('UserControl — подсказка на кнопке «Создать»', () => {
  let wrapper

  beforeEach(() => {
    setActivePinia(createPinia())
  })

  afterEach(() => {
    wrapper?.unmount()
  })

  it('на пустой форме перечисляет все незаполненные поля', async () => {
    wrapper = mountUserControl()
    await flushPromises()

    expect(wrapper.vm.createUserHint).toBe(
      'Заполните: логин, пароль, организацию или компанию, тип пользователя'
    )
  })

  it('заполненное поле уходит из подсказки', async () => {
    wrapper = mountUserControl()
    await flushPromises()

    wrapper.vm.newUser.username = 'testuser'
    wrapper.vm.newUser.organization_id = 1
    await nextTick()

    expect(wrapper.vm.createUserHint).toBe('Заполните: пароль, тип пользователя')
  })

  it('пароль не по политике объясняется отдельной причиной', async () => {
    wrapper = mountUserControl()
    await flushPromises()

    wrapper.vm.newUser.username = 'testuser'
    wrapper.vm.newUser.organization_id = 1
    wrapper.vm.newUser.type_id = 1
    wrapper.vm.newUser.password = 'abc'
    await nextTick()

    expect(wrapper.vm.createUserHint).toBe('Пароль не отвечает требованиям политики')
  })

  it('на заполненной форме подсказки нет', async () => {
    wrapper = mountUserControl()
    await flushPromises()

    wrapper.vm.newUser.username = 'testuser'
    wrapper.vm.newUser.company_id = 5
    wrapper.vm.newUser.type_id = 1
    wrapper.vm.newUser.password = 'Password1'
    await nextTick()

    expect(wrapper.vm.canCreateUser).toBeTruthy()
    expect(wrapper.vm.createUserHint).toBe('')
  })

  // Якорь подсказки - обёртка, а не сама кнопка: disabled-кнопка событий мыши
  // не получает, :hover на ней не сработает и подсказка не покажется.
  it('data-hint висит на обёртке вокруг заблокированной кнопки', async () => {
    wrapper = mountUserControl()
    await flushPromises()

    wrapper.vm.showCreateModal = true
    await nextTick()

    const anchor = document.querySelector('.user-create-modal .hint-anchor')
    expect(anchor).not.toBeNull()
    expect(anchor.getAttribute('data-hint')).toContain('Заполните:')

    const button = anchor.querySelector('button')
    expect(button).not.toBeNull()
    expect(button.disabled).toBe(true)
    expect(button.hasAttribute('data-hint')).toBe(false)
  })
})
