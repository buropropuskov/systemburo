import { describe, it, expect, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import UserBulkOperationsModal from '@/components/UserBulkOperationsModal.vue'
import BaseDropdown from '@/components/ui/BaseDropdown.vue'

const USER_TYPES = [
  { id: 1, name: 'Арендатор', code: 'renter' },
  { id: 2, name: 'Охрана', code: 'security' },
]
const ORGS = [
  { id: 10, name: 'ООО Ромашка' },
  { id: 11, name: 'ЗАО Василёк' },
]
const COMPANIES = [
  { id: 20, name: 'Компания А' },
  { id: 21, name: 'Компания Б' },
]

const STUBS = { teleport: true }

async function mountShown(operation, extra = {}) {
  const w = mount(UserBulkOperationsModal, {
    props: {
      show: false,
      operation,
      selectedCount: 3,
      userTypes: USER_TYPES,
      organizations: ORGS,
      companies: COMPANIES,
      ...extra,
    },
    global: { stubs: STUBS },
  })
  await w.setProps({ show: true })
  await flushPromises()
  return w
}

const applyBtn = w => w.find('[data-testid="user-bulk-op-apply"]')

describe('UserBulkOperationsModal', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('операция type: опции из userTypes, apply эмитит выбранный id типа', async () => {
    const w = await mountShown('type')
    expect(w.findComponent(BaseDropdown).props('options')).toEqual(USER_TYPES)
    // тип не выбран -> Применить недоступно
    expect(applyBtn(w).attributes('disabled')).toBeDefined()

    w.findComponent(BaseDropdown).vm.$emit('update:modelValue', 1)
    await flushPromises()
    expect(applyBtn(w).attributes('disabled')).toBeUndefined()

    await applyBtn(w).trigger('click')
    expect(w.emitted('apply')[0]).toEqual([1])
  })

  it('операция organization: опции из organizations, apply эмитит id организации', async () => {
    const w = await mountShown('organization')
    expect(w.findComponent(BaseDropdown).props('options')).toEqual(ORGS)
    w.findComponent(BaseDropdown).vm.$emit('update:modelValue', 11)
    await flushPromises()
    await applyBtn(w).trigger('click')
    expect(w.emitted('apply')[0]).toEqual([11])
  })

  it('операция company: опции из companies, apply эмитит id компании', async () => {
    const w = await mountShown('company')
    expect(w.findComponent(BaseDropdown).props('options')).toEqual(COMPANIES)
    w.findComponent(BaseDropdown).vm.$emit('update:modelValue', 20)
    await flushPromises()
    await applyBtn(w).trigger('click')
    expect(w.emitted('apply')[0]).toEqual([20])
  })

  it('BaseDropdown идёт с teleport (меню не режется модалкой)', async () => {
    const w = await mountShown('type')
    expect(w.findComponent(BaseDropdown).props('teleport')).toBe(true)
  })

  it('счётчик показывает число выбранных и слово «пользователям»', async () => {
    const w = await mountShown('type', { selectedCount: 5 })
    const text = w.find('[data-testid="user-bulk-op-modal"]').text()
    expect(text).toContain('5')
    expect(text).toContain('пользователям')
  })

  it('переоткрытие с новой операцией сбрасывает выбранное значение', async () => {
    const w = await mountShown('type')
    w.findComponent(BaseDropdown).vm.$emit('update:modelValue', 1)
    await flushPromises()
    expect(w.vm.value).toBe(1)
    // закрыть и открыть заново на другой операции
    await w.setProps({ show: false })
    await w.setProps({ operation: 'organization', show: true })
    await flushPromises()
    expect(w.vm.value).toBe(null)
  })

  it('кнопка Отмена эмитит close', async () => {
    const w = await mountShown('type')
    await w.find('[data-testid="user-bulk-op-cancel"]').trigger('click')
    expect(w.emitted('close')).toBeTruthy()
  })
})
