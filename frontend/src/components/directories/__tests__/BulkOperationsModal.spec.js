import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

const apiMock = vi.hoisted(() => ({ apiRequest: vi.fn() }))
vi.mock('@/api/client', () => apiMock)

import BulkOperationsModal from '../BulkOperationsModal.vue'
import GridSelector from '@/components/ui/GridSelector.vue'
import SelectionModal from '@/components/ui/SelectionModal.vue'
import ToggleSwitch from '@/components/ui/ToggleSwitch.vue'
import BaseDropdown from '@/components/ui/BaseDropdown.vue'

const PLACES = [
  { id: 1, name: 'Склад 1', status: 'active' },
  { id: 2, name: 'Склад 2', status: 'active' },
]
const TABLES = [
  { table: { id: 10, display_name: 'Люди', table_type: 'people' } },
  { table: { id: 11, display_name: 'Машины', table_type: 'cars' } },
]
const USERS = [
  { id: 100, username: 'ivanov', last_name: 'Иванов', first_name: 'Иван', position: 'Инженер' },
  { id: 101, username: 'petrov', last_name: 'Петров', first_name: 'Пётр', position: '' },
]

function okJson(data) {
  return { ok: true, json: vi.fn().mockResolvedValue(data) }
}

const STUBS = { teleport: true }

async function mountShown(operation, { entityType = 'organization', selectedIds = [1, 2, 3] } = {}) {
  const w = mount(BulkOperationsModal, {
    props: { show: false, operation, entityType, selectedIds },
    global: { stubs: STUBS },
  })
  await w.setProps({ show: true })
  await flushPromises()
  return w
}

const applyBtn = w => w.find('[data-testid="bulk-op-apply"]')

describe('BulkOperationsModal', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    apiMock.apiRequest.mockImplementation((path) => {
      if (path === '/unload-places') return Promise.resolve(okJson(PLACES))
      if (path === '/system-tables') return Promise.resolve(okJson(TABLES))
      if (path === '/users/all') return Promise.resolve(okJson(USERS))
      return Promise.resolve(okJson([]))
    })
  })

  it('операция type: apply эмитит выбранный тип', async () => {
    const w = await mountShown('type')
    // тип не выбран -> Применить недоступно
    expect(applyBtn(w).attributes('disabled')).toBeDefined()

    w.findComponent(BaseDropdown).vm.$emit('update:modelValue', 'Арендатор')
    await flushPromises()
    expect(applyBtn(w).attributes('disabled')).toBeUndefined()

    await applyBtn(w).trigger('click')
    expect(w.emitted('apply')[0]).toEqual([{ type: 'Арендатор' }])
  })

  it('операция type: «снять тип» эмитит null', async () => {
    const w = await mountShown('type')
    w.findComponent(BaseDropdown).vm.$emit('update:modelValue', '__none__')
    await flushPromises()
    await applyBtn(w).trigger('click')
    expect(w.emitted('apply')[0]).toEqual([{ type: null }])
  })

  it('операция unload-places: грузит места, тумблер add, apply с mode', async () => {
    const w = await mountShown('unload-places')
    expect(apiMock.apiRequest).toHaveBeenCalledWith('/unload-places')
    expect(w.findComponent(GridSelector).props('items')).toHaveLength(2)

    w.findComponent(GridSelector).vm.$emit('update:modelValue', [1])
    await flushPromises()
    await w.find('[data-testid="bulk-op-mode-add"]').trigger('click')
    await applyBtn(w).trigger('click')
    expect(w.emitted('apply')[0]).toEqual([{ unloadPlaceIds: [1], mode: 'add' }])
  })

  it('операция tables: исключает cars-таблицы, apply replace по умолчанию', async () => {
    const w = await mountShown('tables')
    const items = w.findComponent(GridSelector).props('items')
    expect(items).toHaveLength(1)
    expect(items[0]).toMatchObject({ id: 10, name: 'Люди' })

    w.findComponent(GridSelector).vm.$emit('update:modelValue', [10])
    await flushPromises()
    await applyBtn(w).trigger('click')
    expect(w.emitted('apply')[0]).toEqual([{ tableIds: [10], mode: 'replace' }])
  })

  it('операция users: выбор через SelectionModal + флаг согласования', async () => {
    const w = await mountShown('users')
    expect(apiMock.apiRequest).toHaveBeenCalledWith('/users/all')
    // ничего не выбрано -> недоступно
    expect(applyBtn(w).attributes('disabled')).toBeDefined()

    await w.find('[data-testid="bulk-op-pick-users"]').trigger('click')
    w.findComponent(SelectionModal).vm.$emit('confirm', [USERS[0]])
    await flushPromises()
    w.findComponent(ToggleSwitch).vm.$emit('update:modelValue', true)
    await flushPromises()

    await applyBtn(w).trigger('click')
    expect(w.emitted('apply')[0]).toEqual([
      { usernames: ['ivanov'], requiredApproval: true, mode: 'replace' },
    ])
  })

  it('users: повторный выбор того же пользователя не дублирует', async () => {
    const w = await mountShown('users')
    await w.find('[data-testid="bulk-op-pick-users"]').trigger('click')
    w.findComponent(SelectionModal).vm.$emit('confirm', [USERS[0]])
    w.findComponent(SelectionModal).vm.$emit('confirm', [USERS[0], USERS[1]])
    await flushPromises()
    expect(w.vm.selectedUsers.map(u => u.username)).toEqual(['ivanov', 'petrov'])
  })

  it('кнопка Отмена эмитит close', async () => {
    const w = await mountShown('type')
    await w.find('[data-testid="bulk-op-cancel"]').trigger('click')
    expect(w.emitted('close')).toBeTruthy()
  })

  it('entity-type company меняет формулировку счётчика', async () => {
    const w = await mountShown('type', { entityType: 'company' })
    expect(w.find('[data-testid="bulk-op-modal"]').text()).toContain('компаниям')
  })
})
