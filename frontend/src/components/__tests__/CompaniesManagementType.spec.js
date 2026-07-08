import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { nextTick } from 'vue'
import { createPinia, setActivePinia } from 'pinia'

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn().mockResolvedValue({ ok: false, json: vi.fn() }),
}))

const orgApi = vi.hoisted(() => ({ getCompanyMembers: vi.fn() }))
vi.mock('@/api/organizations', () => orgApi)

vi.mock('@/utils/dirtyTracker', () => ({
  registerDirtyTracker: vi.fn(() => () => {}),
  confirmIfAnyDirty: vi.fn().mockResolvedValue(true),
}))

import CompaniesManagement from '../CompaniesManagement.vue'
import { useCompaniesStore } from '@/stores/companies'
import { useDeletionsStore } from '@/stores/deletions'

function seedCompanies() {
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
  CompanyHistoryModal: true,
}

async function mountCmp(companies = seedCompanies()) {
  const store = useCompaniesStore()
  store.itemsWithUsers = companies
  vi.spyOn(store, 'refresh').mockResolvedValue()
  vi.spyOn(store, 'createCompany').mockResolvedValue({ ok: true, data: { id: 99 } })
  vi.spyOn(store, 'updateCompany').mockResolvedValue({ ok: true })
  vi.spyOn(store, 'fetchCompaniesWithUsers').mockResolvedValue()
  const del = useDeletionsStore()
  vi.spyOn(del, 'notify').mockImplementation(() => {})

  const w = mount(CompaniesManagement, { global: { stubs: STUBS } })
  await flushPromises()
  return { w, store }
}

function rowTypes(w) {
  return w.findAll('[data-testid="companies-row"] .type-col').map(c => c.text())
}
function rowNames(w) {
  return w.findAll('[data-testid="companies-row"] .name-col').map(c => c.text())
}

describe('CompaniesManagement — тип/фильтр/сортировка/участники', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    orgApi.getCompanyMembers.mockResolvedValue([])
  })

  it('колонка «Тип» показывает значение и «не указан» для NULL', async () => {
    const { w } = await mountCmp()
    expect(rowTypes(w)).toEqual(['Арендатор', 'Отдел', 'не указан'])
  })

  it('фильтр по типу оставляет только компании выбранного типа', async () => {
    const { w } = await mountCmp()
    w.findComponent('[data-testid="companies-type-filter"]').vm.$emit('update:modelValue', 'Отдел')
    await nextTick()
    expect(rowNames(w)).toEqual(['Бета'])
  })

  it('фильтр «не указан» оставляет только компании без типа', async () => {
    const { w } = await mountCmp()
    w.findComponent('[data-testid="companies-type-filter"]').vm.$emit('update:modelValue', '__none__')
    await nextTick()
    expect(rowNames(w)).toEqual(['Гамма'])
  })

  it('сортировка по типу: «не указан» первым при asc', async () => {
    const { w } = await mountCmp()
    await w.find('.header-col.type-col').trigger('click')
    await nextTick()
    expect(rowNames(w)).toEqual(['Гамма', 'Альфа', 'Бета'])
  })

  it('кнопка «Создать» неактивна без типа и активна с именем+типом', async () => {
    const { w, store } = await mountCmp()
    await w.find('[data-testid="companies-add-btn"]').trigger('click')
    await nextTick()

    const saveBtn = () => w.find('[data-testid="companies-modal-save"]')
    await w.find('[data-testid="companies-input-name"]').setValue('Новая ко')
    await nextTick()
    expect(saveBtn().attributes('disabled')).toBeDefined()

    w.findComponent('[data-testid="companies-input-type"]').vm.$emit('update:modelValue', 'Подрядчик')
    await nextTick()
    expect(saveBtn().attributes('disabled')).toBeUndefined()

    await saveBtn().trigger('click')
    await flushPromises()
    expect(store.createCompany).toHaveBeenCalledWith(
      { name: 'Новая ко', type: 'Подрядчик' },
      { includeArchived: true },
    )
  })

  it('секция участников грузит и рисует привязанных пользователей', async () => {
    orgApi.getCompanyMembers.mockResolvedValue([
      { id: 10, last_name: 'Петров', first_name: 'Пётр', middle_name: null, position: 'Менеджер' },
    ])
    const { w } = await mountCmp()
    await w.findAll('[data-testid="companies-row"]')[0].trigger('click')
    await flushPromises()

    expect(orgApi.getCompanyMembers).toHaveBeenCalledWith(1)
    const members = w.find('[data-testid="companies-members"]')
    expect(members.text()).toContain('Пользователи, привязанные к компании «Альфа»: 1')
    expect(members.text()).toContain('Петров Пётр')
    expect(members.text()).toContain('Менеджер')
  })

  it('сохранение деталей отправляет имя и тип вместе', async () => {
    const { w, store } = await mountCmp()
    await w.findAll('[data-testid="companies-row"]')[0].trigger('click')
    await flushPromises()

    w.findComponent('[data-testid="companies-detail-type"]').vm.$emit('update:modelValue', 'Организация')
    await nextTick()
    await w.find('[data-testid="companies-save-name"]').trigger('click')
    await flushPromises()

    expect(store.updateCompany).toHaveBeenCalledWith(
      1,
      { name: 'Альфа', type: 'Организация' },
      { includeArchived: true },
    )
  })
})
