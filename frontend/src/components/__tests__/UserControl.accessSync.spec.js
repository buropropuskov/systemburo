import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { nextTick } from 'vue'
import { createPinia, setActivePinia } from 'pinia'
import UserControl from '@/components/UserControl.vue'

vi.mock('@/api/settings', () => ({
  getPasswordPolicy: vi.fn().mockResolvedValue({
    min_length: 8, require_letter: true, require_digit: true,
  }),
}))
vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: true, json: vi.fn().mockResolvedValue([]) }),
}))
vi.mock('@/api/onboarding', () => ({ resetOnboardingForUser: vi.fn().mockResolvedValue({}) }))
vi.mock('@/utils/notificationSound', () => ({ playPreset: vi.fn() }))

function mountUserControl(allUsers = []) {
  return mount(UserControl, {
    props: { allUsers },
    global: {
      mocks: {
        $bus: { on: vi.fn(), off: vi.fn(), emit: vi.fn() },
        $router: { push: vi.fn(), replace: vi.fn().mockResolvedValue(undefined) },
        $route: { path: '/admin/users', params: {} },
      },
    },
  })
}

describe('UserControl — синхронизация открытой карточки после рефетча', () => {
  let wrapper
  beforeEach(() => setActivePinia(createPinia()))
  afterEach(() => wrapper?.unmount())

  it('обновляет роль открытой карточки при свежем allUsers', async () => {
    const initial = [{ id: 1, username: 'chop_kpp4', role_id: null, role_name: 'Пользователь' }]
    wrapper = mountUserControl(initial)
    await flushPromises()

    wrapper.vm.selectUser(initial[0])
    await nextTick()
    expect(wrapper.vm.selectedUser.role_id).toBe(null)

    // Рефетч списка (после выдачи роли в модалке) отдаёт того же юзера с новой ролью.
    await wrapper.setProps({
      allUsers: [{ id: 1, username: 'chop_kpp4', role_id: 7, role_name: 'Охранник' }],
    })
    await nextTick()
    expect(wrapper.vm.selectedUser.role_id).toBe(7)
    expect(wrapper.vm.selectedUser.role_name).toBe('Охранник')
  })

  it('не трогает selectedUser, если карточка не открыта', async () => {
    wrapper = mountUserControl([{ id: 1, username: 'a', role_id: 1 }])
    await flushPromises()
    expect(wrapper.vm.selectedUser).toBe(null)

    await wrapper.setProps({ allUsers: [{ id: 1, username: 'a', role_id: 2 }] })
    await nextTick()
    expect(wrapper.vm.selectedUser).toBe(null)
  })

  it('сохраняет введённый newPassword при пере-резолве', async () => {
    const initial = [{ id: 1, username: 'u', role_id: 1 }]
    wrapper = mountUserControl(initial)
    await flushPromises()
    wrapper.vm.selectUser(initial[0])
    wrapper.vm.selectedUser.newPassword = 'TypedPass1'
    await nextTick()

    await wrapper.setProps({ allUsers: [{ id: 1, username: 'u', role_id: 5 }] })
    await nextTick()
    expect(wrapper.vm.selectedUser.role_id).toBe(5)
    expect(wrapper.vm.selectedUser.newPassword).toBe('TypedPass1')
  })
})
