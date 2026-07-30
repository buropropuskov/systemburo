import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { nextTick } from 'vue'
import { createPinia, setActivePinia } from 'pinia'

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: false, json: vi.fn() }),
}))

const orgApi = vi.hoisted(() => ({ getOrganizationMembers: vi.fn() }))
vi.mock('@/api/organizations', () => orgApi)

vi.mock('@/utils/dirtyTracker', () => ({
  registerDirtyTracker: vi.fn(() => () => {}),
  confirmIfAnyDirty: vi.fn().mockResolvedValue(true),
}))

import OrganizationsManagement from '../OrganizationsManagement.vue'
import { useOrganizationsStore } from '@/stores/organizations'
import { useDeletionsStore } from '@/stores/deletions'

function seedOrgs() {
  return [
    { id: 1, name: 'Альфа', type: 'Арендатор', is_active: true, user_count: 2 },
    { id: 2, name: 'Бета', type: 'Отдел', is_active: true, user_count: 0 },
    { id: 3, name: 'Гамма', type: null, is_active: true, user_count: 1 },
  ]
}

const STUBS = {
  teleport: true,
  RefreshButton: true,
  SearchComponent: true,
  ResponsibleUsersSection: true,
  SelectUnloadPlaces: true,
  SelectTables: true,
  ConfirmationModal: true,
  LoaderSpinner: true,
  OrgHistoryModal: true,
}

async function mountCmp(orgs = seedOrgs()) {
  const store = useOrganizationsStore()
  store.itemsWithUsers = orgs
  vi.spyOn(store, 'refresh').mockResolvedValue()
  vi.spyOn(store, 'createOrganization').mockResolvedValue({ ok: true, data: { id: 99 } })
  vi.spyOn(store, 'updateOrganization').mockResolvedValue({ ok: true })
  vi.spyOn(store, 'fetchOrganizationsWithUsers').mockResolvedValue()
  const del = useDeletionsStore()
  vi.spyOn(del, 'notify').mockImplementation(() => {})

  const w = mount(OrganizationsManagement, { global: { stubs: STUBS } })
  await flushPromises()
  return { w, store }
}

function rowTypes(w) {
  return w.findAll('[data-testid="orgs-row"] .type-col').map(c => c.text())
}
function rowNames(w) {
  return w.findAll('[data-testid="orgs-row"] .name-col').map(c => c.text())
}
function typeFilter(w) {
  return w.findComponent('[data-testid="orgs-type-filter"]')
}

describe('OrganizationsManagement — тип/фильтр/сортировка/участники', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    orgApi.getOrganizationMembers.mockResolvedValue([])
  })

  it('колонка «Тип» показывает значение и «не указан» для NULL', async () => {
    const { w } = await mountCmp()
    expect(rowTypes(w)).toEqual(['Арендатор', 'Отдел', 'не указан'])
  })

  it('фильтр по типу оставляет только организации выбранного типа', async () => {
    const { w } = await mountCmp()
    typeFilter(w).vm.$emit('update:modelValue', ['Отдел'])
    await nextTick()
    expect(rowNames(w)).toEqual(['Бета'])
  })

  it('фильтр «не указан» оставляет только организации без типа', async () => {
    const { w } = await mountCmp()
    typeFilter(w).vm.$emit('update:modelValue', ['__none__'])
    await nextTick()
    expect(rowNames(w)).toEqual(['Гамма'])
  })

  it('мультивыбор: «не указан» — элемент набора, а не отдельный режим (#1398)', async () => {
    const { w } = await mountCmp()
    typeFilter(w).vm.$emit('update:modelValue', ['Отдел', '__none__'])
    await nextTick()
    expect(rowNames(w)).toEqual(['Бета', 'Гамма'])
  })

  it('пустой набор типов не фильтрует', async () => {
    const { w } = await mountCmp()
    typeFilter(w).vm.$emit('update:modelValue', ['Отдел'])
    await nextTick()
    typeFilter(w).vm.$emit('update:modelValue', [])
    await nextTick()
    expect(rowNames(w)).toEqual(['Альфа', 'Бета', 'Гамма'])
  })

  it('подпись кнопки: «Тип: все» без выбора, имя при одном, счётчик при нескольких', async () => {
    const { w } = await mountCmp()
    const caption = () => typeFilter(w).find('.base-dropdown__text').text()
    expect(caption()).toBe('Тип: все')

    typeFilter(w).vm.$emit('update:modelValue', ['Отдел'])
    await nextTick()
    expect(caption()).toBe('Отдел')

    typeFilter(w).vm.$emit('update:modelValue', ['Отдел', '__none__'])
    await nextTick()
    expect(caption()).toBe('Тип: 2')
  })

  it('сортировка по типу: «не указан» первым при asc', async () => {
    const { w } = await mountCmp()
    await w.find('.header-col.type-col').trigger('click')
    await nextTick()
    // asc: '' (Гамма) < 'Арендатор' (Альфа) < 'Отдел' (Бета)
    expect(rowNames(w)).toEqual(['Гамма', 'Альфа', 'Бета'])
  })

  it('кнопка «Создать» неактивна без типа и активна с именем+типом', async () => {
    const { w, store } = await mountCmp()
    await w.find('[data-testid="orgs-add-btn"]').trigger('click')
    await nextTick()

    const saveBtn = () => w.find('[data-testid="orgs-modal-save"]')
    await w.find('[data-testid="orgs-input-name"]').setValue('Новая орг')
    await nextTick()
    expect(saveBtn().attributes('disabled')).toBeDefined()

    w.findComponent('[data-testid="orgs-input-type"]').vm.$emit('update:modelValue', 'Подрядчик')
    await nextTick()
    expect(saveBtn().attributes('disabled')).toBeUndefined()

    await saveBtn().trigger('click')
    await flushPromises()
    expect(store.createOrganization).toHaveBeenCalledWith(
      { name: 'Новая орг', type: 'Подрядчик' },
      { includeArchived: true },
    )
  })

  it('секция участников грузит и рисует привязанных пользователей', async () => {
    orgApi.getOrganizationMembers.mockResolvedValue([
      { id: 10, last_name: 'Иванов', first_name: 'Иван', middle_name: null, position: 'Директор' },
    ])
    const { w } = await mountCmp()
    await w.findAll('[data-testid="orgs-row"]')[0].trigger('click')
    await flushPromises()

    expect(orgApi.getOrganizationMembers).toHaveBeenCalledWith(1)
    const members = w.find('[data-testid="orgs-members"]')
    expect(members.text()).toContain('Пользователи, привязанные к организации')
    expect(members.find('.count-badge').text()).toBe('1')
    expect(members.text()).toContain('Иванов Иван')
    expect(members.text()).toContain('Директор')
  })

  it('сохранение деталей отправляет имя и тип вместе', async () => {
    const { w, store } = await mountCmp()
    await w.findAll('[data-testid="orgs-row"]')[0].trigger('click')
    await flushPromises()

    w.findComponent('[data-testid="orgs-detail-type"]').vm.$emit('update:modelValue', 'Организация')
    await nextTick()
    await w.find('[data-testid="orgs-save-name"]').trigger('click')
    await flushPromises()

    expect(store.updateOrganization).toHaveBeenCalledWith(
      1,
      { name: 'Альфа', type: 'Организация' },
      { includeArchived: true },
    )
  })

  it('fix 5: dirty-change ответственных/мест поднимает isDirty родителя (event-path)', async () => {
    const { w } = await mountCmp()
    await w.findAll('[data-testid="orgs-row"]')[0].trigger('click')
    await flushPromises()
    expect(w.vm.isDirty).toBe(false)

    w.findComponent({ name: 'ResponsibleUsersSection' }).vm.$emit('dirty-change', true)
    await nextTick()
    expect(w.vm.isDirty).toBe(true)

    w.findComponent({ name: 'ResponsibleUsersSection' }).vm.$emit('dirty-change', false)
    w.findComponent({ name: 'SelectUnloadPlaces' }).vm.$emit('dirty-change', true)
    await nextTick()
    expect(w.vm.isDirty).toBe(true)

    // Смена выбора сбрасывает поднятые флаги детей.
    w.findComponent({ name: 'SelectUnloadPlaces' }).vm.$emit('dirty-change', false)
    await nextTick()
    expect(w.vm.isDirty).toBe(false)
  })
})
