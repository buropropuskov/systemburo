import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { nextTick } from 'vue'
import { createPinia, setActivePinia } from 'pinia'
import UserControl from '@/components/UserControl.vue'

// Мок политики: минимум 8 символов + буква + цифра (по умолчанию)
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

// UserControl дёргает apiRequest напрямую и через pinia-stores.
// Мокаем транспорт и все API-зависимости чтобы монтирование не падало.
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
    props: {
      allUsers: [],
    },
    global: {
      mocks: {
        $bus: { on: vi.fn(), off: vi.fn(), emit: vi.fn() },
        $router: { push: vi.fn(), replace: vi.fn().mockResolvedValue(undefined) },
        $route: { path: '/admin/users', params: {} },
      },
    },
  })
}

describe('UserControl — политика пароля при создании', () => {
  let wrapper

  beforeEach(() => {
    setActivePinia(createPinia())
  })

  afterEach(() => {
    wrapper?.unmount()
  })

  it('слабый пароль блокирует создание', async () => {
    wrapper = mountUserControl()
    await flushPromises()

    wrapper.vm.newUser.username = 'testuser'
    wrapper.vm.newUser.type_id = 1
    wrapper.vm.newUser.organization_id = 1
    wrapper.vm.newUser.password = 'abc'
    await nextTick()
    expect(wrapper.vm.canCreateUser).toBeFalsy()
  })

  it('пароль по политике разрешает создание', async () => {
    wrapper = mountUserControl()
    await flushPromises()

    wrapper.vm.newUser.username = 'testuser'
    wrapper.vm.newUser.type_id = 1
    wrapper.vm.newUser.organization_id = 1
    wrapper.vm.newUser.password = 'password123'
    await nextTick()
    expect(wrapper.vm.canCreateUser).toBeTruthy()
  })

  it('createPasswordValid: false при слабом пароле', async () => {
    wrapper = mountUserControl()
    await flushPromises()

    wrapper.vm.newUser.password = 'short'
    await nextTick()
    expect(wrapper.vm.createPasswordValid).toBe(false)
  })

  it('createPasswordValid: true при пароле по политике', async () => {
    wrapper = mountUserControl()
    await flushPromises()

    wrapper.vm.newUser.password = 'Str0ngPass'
    await nextTick()
    expect(wrapper.vm.createPasswordValid).toBe(true)
  })

  it('changePasswordValid: false при пустом пароле', async () => {
    wrapper = mountUserControl()
    await flushPromises()

    wrapper.vm.selectedUser = { username: 'u', newPassword: '' }
    await nextTick()
    expect(wrapper.vm.changePasswordValid).toBe(false)
  })

  it('changePasswordValid: true при пароле по политике', async () => {
    wrapper = mountUserControl()
    await flushPromises()

    wrapper.vm.selectedUser = { username: 'u', newPassword: 'GoodPass1' }
    await nextTick()
    expect(wrapper.vm.changePasswordValid).toBe(true)
  })
})
