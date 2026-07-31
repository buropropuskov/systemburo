import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import UserControl from '@/components/UserControl.vue'
import ConfirmationModal from '@/components/ConfirmationModal.vue'

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

// Слой карточки редактирования (BaseModal :z-index) - подтверждение обязано быть выше.
const EDIT_MODAL_Z = 1001

function mountUserControl() {
  return mount(UserControl, {
    props: { allUsers: [{ id: 1, username: 'petrov', is_active: true }] },
    global: {
      mocks: {
        $bus: { on: vi.fn(), off: vi.fn(), emit: vi.fn() },
        $router: { push: vi.fn(), replace: vi.fn().mockResolvedValue(undefined) },
        $route: { path: '/admin/users', params: {} },
      },
    },
  })
}

describe('UserControl — подтверждение удаления учётной записи', () => {
  let wrapper
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })
  afterEach(() => wrapper?.unmount())

  it('открывается поверх карточки редактирования, а не под ней', async () => {
    wrapper = mountUserControl()
    await flushPromises()

    wrapper.vm.selectUser(wrapper.vm.sortedUsers[0])
    await flushPromises()
    expect(wrapper.vm.showEditModal).toBe(true)

    wrapper.vm.confirmDeleteUser(wrapper.vm.selectedUser)
    await flushPromises()

    const confirm = wrapper.findAllComponents(ConfirmationModal).find(c => c.props('show'))
    expect(confirm).toBeTruthy()
    expect(confirm.props('title')).toBe('Удаление пользователя')
    // Ровно это и было сломано: подтверждение на базовом слое 1000 пряталось под карточкой.
    expect(confirm.props('zIndex')).toBeGreaterThan(EDIT_MODAL_Z)
  })

  it('текст подтверждения называет учётную запись и необратимость', async () => {
    wrapper = mountUserControl()
    await flushPromises()

    wrapper.vm.confirmDeleteUser({ username: 'petrov' })
    await flushPromises()

    const confirm = wrapper.findAllComponents(ConfirmationModal).find(c => c.props('show'))
    expect(confirm.props('message')).toContain('petrov')
    expect(confirm.props('message')).toContain('необратимо')
    expect(confirm.props('confirmText')).toBe('Удалить')
  })

  it('отмена закрывает подтверждение и не трогает карточку', async () => {
    wrapper = mountUserControl()
    await flushPromises()

    wrapper.vm.selectUser(wrapper.vm.sortedUsers[0])
    wrapper.vm.confirmDeleteUser(wrapper.vm.selectedUser)
    await flushPromises()

    wrapper.vm.deleteConfirmUser = null
    await flushPromises()

    const shown = wrapper.findAllComponents(ConfirmationModal).filter(c => c.props('show'))
    expect(shown).toHaveLength(0)
    expect(wrapper.vm.showEditModal).toBe(true)
  })
})

describe('ConfirmationModal — слой отрисовки', () => {
  // Оверлей уезжает в body телепортом, поэтому читаем его из документа, не из wrapper.
  const overlayStyle = () => document.body.querySelector('.modal-overlay').getAttribute('style')

  it('по умолчанию базовый слой модалок, проп поднимает выше', () => {
    const base = mount(ConfirmationModal, { props: { show: true, message: 'тест' } })
    expect(overlayStyle()).toContain('z-index: 1000')
    base.unmount()

    const raised = mount(ConfirmationModal, { props: { show: true, message: 'тест', zIndex: 1002 } })
    expect(overlayStyle()).toContain('z-index: 1002')
    raised.unmount()
  })
})
