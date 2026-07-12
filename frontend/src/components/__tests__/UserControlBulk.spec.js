import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { nextTick } from 'vue'
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

const bulkApi = vi.hoisted(() => ({
  bulkArchiveUsers: vi.fn(),
  bulkRestoreUsers: vi.fn(),
}))
vi.mock('@/api/users', () => bulkApi)

function seedUsers() {
  return [
    { id: 1, username: 'alpha', is_active: true },
    { id: 2, username: 'beta', is_active: true },
    { id: 3, username: 'gamma', is_active: true },
  ]
}

function mountUserControl(allUsers = seedUsers()) {
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

const rowChecks = w => w.findAll('[data-testid="users-row-check"]')
const bulkBar = w => w.find('[data-testid="users-bulk-bar"]')

describe('UserControl — групповой выбор и bulk архив/восстановление', () => {
  let wrapper
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })
  afterEach(() => wrapper?.unmount())

  it('панель скрыта без выбора и появляется со счётчиком при выборе строки', async () => {
    wrapper = mountUserControl()
    await flushPromises()
    expect(bulkBar(wrapper).exists()).toBe(false)

    await rowChecks(wrapper)[0].trigger('click')
    expect(bulkBar(wrapper).exists()).toBe(true)
    expect(bulkBar(wrapper).find('.bulk-count').text()).toBe('Выбрано: 1')
    expect(wrapper.vm.selectedUsernames).toEqual(['alpha'])
  })

  it('shift-клик выделяет диапазон строк', async () => {
    wrapper = mountUserControl()
    await flushPromises()
    await rowChecks(wrapper)[0].trigger('click')
    await rowChecks(wrapper)[2].trigger('click', { shiftKey: true })
    expect([...wrapper.vm.selectedUsernames].sort()).toEqual(['alpha', 'beta', 'gamma'])
    expect(bulkBar(wrapper).find('.bulk-count').text()).toBe('Выбрано: 3')
  })

  it('select-all выбирает всех, повторный клик снимает', async () => {
    wrapper = mountUserControl()
    await flushPromises()
    await wrapper.find('[data-testid="users-select-all"]').trigger('change')
    expect(wrapper.vm.selectedUsernames).toHaveLength(3)
    await wrapper.find('[data-testid="users-select-all"]').trigger('change')
    expect(wrapper.vm.selectedUsernames).toHaveLength(0)
  })

  it('bulk-архив: подтверждение -> вызов API с username, полный успех -> сброс выбора', async () => {
    bulkApi.bulkArchiveUsers.mockResolvedValue({ success_count: 2, error_count: 0, errors: [] })
    wrapper = mountUserControl()
    await flushPromises()
    vi.spyOn(wrapper.vm, 'fetchAllUsers').mockImplementation(() => {})

    await rowChecks(wrapper)[0].trigger('click')
    await rowChecks(wrapper)[1].trigger('click')
    await wrapper.find('[data-testid="users-bulk-archive"]').trigger('click')
    expect(wrapper.vm.bulkConfirmVisible).toBe(true)

    await wrapper.vm.applyBulkArchiveRestore()
    await flushPromises()
    expect(bulkApi.bulkArchiveUsers).toHaveBeenCalledWith(['alpha', 'beta'])
    expect(wrapper.vm.selectedUsernames).toEqual([]) // сброшен после успеха
    expect(wrapper.vm.bulkConfirmVisible).toBe(false)
  })

  it('в архивном режиме кнопка Восстановить зовёт restore-API', async () => {
    bulkApi.bulkRestoreUsers.mockResolvedValue({ success_count: 1, error_count: 0, errors: [] })
    wrapper = mountUserControl([{ id: 1, username: 'arch1', is_active: false }])
    await flushPromises()
    vi.spyOn(wrapper.vm, 'fetchAllUsers').mockImplementation(() => {})
    wrapper.vm.onArchiveModeChange('archive')
    await nextTick()

    await rowChecks(wrapper)[0].trigger('click')
    expect(wrapper.find('[data-testid="users-bulk-restore"]').exists()).toBe(true)
    await wrapper.vm.startBulkOperation('restore')
    await wrapper.vm.applyBulkArchiveRestore()
    await flushPromises()
    expect(bulkApi.bulkRestoreUsers).toHaveBeenCalledWith(['arch1'])
  })
})
