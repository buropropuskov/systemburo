import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
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
        $router: { push: vi.fn(), replace: vi.fn().mockResolvedValue(undefined) },
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
      'Заполните: логин, пароль или адрес почты, организацию или компанию, тип пользователя'
    )
  })

  it('заполненное поле уходит из подсказки', async () => {
    wrapper = mountUserControl()
    await flushPromises()

    wrapper.vm.newUser.username = 'testuser'
    wrapper.vm.newUser.organization_id = 1
    await nextTick()

    expect(wrapper.vm.createUserHint).toBe('Заполните: пароль или адрес почты, тип пользователя')
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

describe('UserControl - подписи организации и компании в форме создания', () => {
  it('звёздочка обязательности не стоит ни у организации, ни у компании: достаточно одного поля', () => {
    const src = readFileSync(
      resolve(__dirname, '../UserControl.vue'),
      'utf8',
    )
    expect(src).toContain('<label class="input-label">Организация</label>')
    expect(src).toContain('<label class="input-label">Компания</label>')
    expect(src).not.toContain('Организация <span class="required">*</span>')
    expect(src).not.toContain('Компания <span class="required">*</span>')
  })

  it('вместо двух звёздочек форма объясняет правило одной строкой', () => {
    const src = readFileSync(
      resolve(__dirname, '../UserControl.vue'),
      'utf8',
    )
    expect(src).toContain('Заполните организацию или компанию - достаточно одного из двух.')
  })
})
