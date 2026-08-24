import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import UserControl from '@/components/UserControl.vue'

vi.mock('@/api/settings', () => ({
  getPasswordPolicy: vi.fn().mockResolvedValue({ min_length: 8, require_letter: true, require_digit: true }),
}))
vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: true, json: vi.fn().mockResolvedValue([]) }),
}))
vi.mock('@/api/onboarding', () => ({ resetOnboardingForUser: vi.fn().mockResolvedValue({}) }))
vi.mock('@/utils/notificationSound', () => ({ playPreset: vi.fn() }))
vi.mock('@/api/users', () => ({
  bulkArchiveUsers: vi.fn(), bulkRestoreUsers: vi.fn(), bulkUpdateUsersType: vi.fn(),
  bulkAssignUsersOrganization: vi.fn(), bulkAssignUsersCompany: vi.fn(),
  bulkBanUsers: vi.fn(), bulkUnbanUsers: vi.fn(), resetUserLockout: vi.fn(),
}))

/**
 * Переход из сквозного поиска: найденная учётная запись раскрывается сама. Список
 * приходит в компонент пропом целиком, поэтому строка поиска в адресе не нужна -
 * достаточно id.
 */
const USERS = [
  { id: 1, username: 'ivanov', is_active: true, is_banned: false },
  { id: 2, username: 'petrov', is_active: true, is_banned: false },
]

function mountControl(query, replace = vi.fn().mockResolvedValue(undefined)) {
  return mount(UserControl, {
    props: { allUsers: [] },
    global: {
      mocks: {
        $bus: { on: vi.fn(), off: vi.fn(), emit: vi.fn() },
        $router: { push: vi.fn(), replace },
        $route: { path: '/admin/users', params: {}, query },
      },
    },
  })
}

describe('UserControl — открытие карточки по ссылке из сквозного поиска', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('карточка найденного пользователя открывается, когда приезжает список', async () => {
    const wrapper = mountControl({ open: '2' })
    await wrapper.setProps({ allUsers: USERS })
    await flushPromises()

    expect(wrapper.vm.selectedUser?.username).toBe('petrov')
  })

  it('open вычищается из адреса - обновление страницы не откроет карточку заново', async () => {
    const replace = vi.fn().mockResolvedValue(undefined)
    const wrapper = mountControl({ open: '2' }, replace)
    await wrapper.setProps({ allUsers: USERS })
    await flushPromises()

    expect(replace).toHaveBeenCalledWith({ query: {} })
  })

  it('без параметра карточка не открывается сама', async () => {
    const wrapper = mountControl({})
    await wrapper.setProps({ allUsers: USERS })
    await flushPromises()

    expect(wrapper.vm.selectedUser).toBeNull()
  })

  it('пользователя нет в списке - открывать нечего, ошибок нет', async () => {
    const wrapper = mountControl({ open: '999' })
    await wrapper.setProps({ allUsers: USERS })
    await flushPromises()

    expect(wrapper.vm.selectedUser).toBeNull()
  })
})
