import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import UserControl from '@/components/UserControl.vue'
import { apiRequest } from '@/api/client'

// ФИО работника, не давшего согласия на обработку персональных данных, сервер не
// присылает и помечает признаком name_hidden (#1567 S10). Карточка редактирования
// обязана отличать это от незаполненного поля: иначе правка должности или почты
// уходит на сервер с пустым ФИО и стирает настоящее.

vi.mock('@/api/settings', () => ({
  getPasswordPolicy: vi.fn().mockResolvedValue({ min_length: 8, require_letter: true, require_digit: true }),
}))
vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: true, json: vi.fn().mockResolvedValue([]) }),
}))
vi.mock('@/api/onboarding', () => ({ resetOnboardingForUser: vi.fn().mockResolvedValue({}) }))
vi.mock('@/utils/notificationSound', () => ({ playPreset: vi.fn() }))
vi.mock('@/api/users', () => ({
  bulkArchiveUsers: vi.fn(),
  bulkRestoreUsers: vi.fn(),
  bulkUpdateUsersType: vi.fn(),
  bulkAssignUsersOrganization: vi.fn(),
  bulkAssignUsersCompany: vi.fn(),
  bulkBanUsers: vi.fn(),
  bulkUnbanUsers: vi.fn(),
  resetUserLockout: vi.fn(),
}))

const hiddenUser = {
  id: 1,
  username: 'silent_user',
  is_active: true,
  last_name: null,
  first_name: null,
  middle_name: null,
  name_hidden: true,
  position: 'Инженер',
  email: 'e@example.com',
}

const openUser = {
  id: 2,
  username: 'open_user',
  is_active: true,
  last_name: 'Иванов',
  first_name: 'Иван',
  middle_name: 'Иванович',
  name_hidden: false,
  position: 'Инженер',
}

function mountUserControl(allUsers) {
  return mount(UserControl, {
    props: { allUsers },
    global: {
      mocks: {
        $bus: { on: vi.fn(), off: vi.fn(), emit: vi.fn() },
        $router: { push: vi.fn() },
        $route: { path: '/admin/users', params: {} },
      },
    },
  })
}

/** Тело последнего PUT на /users/:username/info. */
function lastInfoPayload() {
  const call = [...apiRequest.mock.calls].reverse()
    .find(([url, opts]) => /\/users\/.+\/info$/.test(url) && opts?.method === 'PUT')
  return call ? JSON.parse(call[1].body) : null
}

describe('UserControl — скрытое до согласия ФИО', () => {
  let wrapper
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    apiRequest.mockResolvedValue({ ok: true, json: vi.fn().mockResolvedValue([]) })
  })
  afterEach(() => wrapper?.unmount())

  it('в строке списка вместо прочерка показывает логин', async () => {
    wrapper = mountUserControl([hiddenUser])
    await flushPromises()

    expect(wrapper.text()).toContain('silent_user')
    expect(wrapper.find('.name-col').text()).not.toBe('-')
  })

  it('правка соседнего поля не отправляет пустое ФИО', async () => {
    wrapper = mountUserControl([hiddenUser])
    await flushPromises()

    await wrapper.vm.updateUserInfo({ ...wrapper.vm.filteredUsers[0], position: 'Главный инженер' })

    const payload = lastInfoPayload()
    expect(payload).not.toBeNull()
    expect(payload.position).toBe('Главный инженер')
    expect('last_name' in payload).toBe(false)
    expect('first_name' in payload).toBe(false)
    expect('middle_name' in payload).toBe(false)
  })

  it('у обычного работника ФИО по-прежнему отправляется', async () => {
    wrapper = mountUserControl([openUser])
    await flushPromises()

    await wrapper.vm.updateUserInfo({ ...wrapper.vm.filteredUsers[0], position: 'Главный инженер' })

    const payload = lastInfoPayload()
    expect(payload.last_name).toBe('Иванов')
    expect(payload.first_name).toBe('Иван')
  })
})
