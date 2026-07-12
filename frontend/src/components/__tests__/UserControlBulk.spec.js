import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { nextTick } from 'vue'
import { createPinia, setActivePinia } from 'pinia'
import UserControl from '@/components/UserControl.vue'
import { useUiStore } from '@/stores/ui'
import { useDeletionsStore } from '@/stores/deletions'

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

  it('частичный успех -> ui.warning с непрошедшими, выбор сброшен', async () => {
    bulkApi.bulkArchiveUsers.mockResolvedValue({ success_count: 1, error_count: 1, errors: [{ id: 2, name: 'beta', error: 'Пользователь не найден' }] })
    wrapper = mountUserControl()
    await flushPromises()
    vi.spyOn(wrapper.vm, 'fetchAllUsers').mockImplementation(() => {})
    const warn = vi.spyOn(useUiStore(), 'warning')

    await rowChecks(wrapper)[0].trigger('click')
    await rowChecks(wrapper)[1].trigger('click')
    await wrapper.vm.startBulkOperation('archive')
    await wrapper.vm.applyBulkArchiveRestore()
    await flushPromises()
    expect(warn).toHaveBeenCalledWith(expect.stringContaining('beta'))
    expect(wrapper.vm.selectedUsernames).toEqual([])
    expect(wrapper.vm.bulkConfirmVisible).toBe(false)
  })

  it('ошибка-envelope ({message}) -> error-notify, выбор НЕ сброшен, модалка держится', async () => {
    bulkApi.bulkArchiveUsers.mockResolvedValue({ message: 'Не выбраны пользователи' })
    wrapper = mountUserControl()
    await flushPromises()
    const notify = vi.spyOn(useDeletionsStore(), 'notify')

    await rowChecks(wrapper)[0].trigger('click')
    await wrapper.vm.startBulkOperation('archive')
    await wrapper.vm.applyBulkArchiveRestore()
    await flushPromises()
    expect(notify).toHaveBeenCalledWith(expect.objectContaining({ type: 'error' }))
    expect(wrapper.vm.selectedUsernames).toEqual(['alpha']) // не сброшен - можно повторить
    expect(wrapper.vm.bulkConfirmVisible).toBe(true) // модалка открыта
  })

  it('отмена подтверждения и смена архив-режима сбрасывают выбор/операцию', async () => {
    wrapper = mountUserControl()
    await flushPromises()
    await rowChecks(wrapper)[0].trigger('click')
    await wrapper.vm.startBulkOperation('archive')
    expect(wrapper.vm.bulkConfirmVisible).toBe(true)
    wrapper.vm.cancelBulkConfirm()
    expect(wrapper.vm.bulkConfirmVisible).toBe(false)
    expect(wrapper.vm.pendingBulkOp).toBe(null)
    // Отмена НЕ теряет выбор (можно повторить операцию).
    expect(wrapper.vm.selectedUsernames).toEqual(['alpha'])

    await rowChecks(wrapper)[1].trigger('click')
    expect([...wrapper.vm.selectedUsernames].sort()).toEqual(['alpha', 'beta'])
    wrapper.vm.onArchiveModeChange('archive')
    expect(wrapper.vm.selectedUsernames).toEqual([]) // выбор не переносим между режимами
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
