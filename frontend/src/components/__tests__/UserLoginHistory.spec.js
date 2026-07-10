import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'

vi.mock('@/api/users', () => ({ getUserAuthEvents: vi.fn() }))
vi.mock('@/stores/deletions', () => ({ useDeletionsStore: () => ({ notify: vi.fn() }) }))

// Лёгкий фейк ExcelJS: экспорт не должен тянуть реальную сборку книги в jsdom.
vi.mock('exceljs', () => {
  const cell = { fill: {}, font: {}, alignment: {}, border: {} }
  const row = { height: 0, eachCell: (fn) => fn(cell), getCell: () => cell }
  const worksheet = { addRow: () => row, getCell: () => cell, columns: [] }
  const workbook = {
    addWorksheet: () => worksheet,
    xlsx: { writeBuffer: vi.fn().mockResolvedValue(new ArrayBuffer(8)) },
  }
  return { default: { Workbook: vi.fn(() => workbook) } }
})

import UserLoginHistory from '@/components/UserLoginHistory.vue'
import { getUserAuthEvents } from '@/api/users'

const CHROME_UA = 'Mozilla/5.0 (Windows NT 10.0; Win64) AppleWebKit/537.36 Chrome/120 Safari/537.36'
const FIREFOX_UA = 'Mozilla/5.0 (X11; Linux x86_64; rv:120) Gecko/20100101 Firefox/120'

const sampleRows = [
  { id: 3, event_type: 'login_success', success: true, ip_address: '10.0.0.1', user_agent: CHROME_UA, detail: '', created_at: '2026-07-10T09:00:00Z' },
  { id: 2, event_type: 'login_failed', success: false, ip_address: '10.0.0.1', user_agent: CHROME_UA, detail: 'wrong password', created_at: '2026-07-10T08:00:00Z' },
  { id: 1, event_type: 'account_locked', success: false, ip_address: '203.0.113.9', user_agent: FIREFOX_UA, detail: '10 attempts', created_at: '2026-07-09T22:00:00Z' },
]

const pageOf = (items, total) => ({ items, total, page: 1, limit: 25 })

function mountLH(props = {}) {
  return mount(UserLoginHistory, {
    props: { username: 'ivanov', currentUserName: 'Иванов И.', ...props },
    global: { stubs: { BaseDropdown: true } },
  })
}

describe('UserLoginHistory', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    getUserAuthEvents.mockResolvedValue(pageOf(sampleRows, 30))
    global.URL.createObjectURL = vi.fn(() => 'blob:x')
    global.URL.revokeObjectURL = vi.fn()
  })
  afterEach(() => vi.restoreAllMocks())

  it('грузит историю при монтировании и рендерит строки с человекочитаемыми бейджами', async () => {
    const wrapper = mountLH()
    await flushPromises()

    expect(getUserAuthEvents).toHaveBeenCalledWith('ivanov', {
      page: 1, limit: 25, category: '', from: '', to: '',
    })
    const rows = wrapper.findAll('tbody tr')
    expect(rows).toHaveLength(3)

    const text = wrapper.text()
    expect(text).toContain('Вход выполнен')
    expect(text).toContain('Неудачный вход')
    expect(text).toContain('Аккаунт заблокирован')
    // Сырые коды не протекают в интерфейс.
    expect(text).not.toContain('login_success')
    expect(text).not.toContain('account_locked')
    wrapper.unmount()
  })

  it('парсит User-Agent в браузер и ОС', async () => {
    const wrapper = mountLH()
    await flushPromises()
    const text = wrapper.text()
    expect(text).toContain('Chrome')
    expect(text).toContain('Windows 10/11')
    expect(text).toContain('Firefox')
    expect(text).toContain('Linux')
    wrapper.unmount()
  })

  it('смена категории перезапрашивает с параметром category и сбросом на первую страницу', async () => {
    const wrapper = mountLH()
    await flushPromises()
    wrapper.vm.page = 3

    wrapper.vm.onCategoryChange('failed')
    await flushPromises()

    expect(wrapper.vm.page).toBe(1)
    expect(getUserAuthEvents).toHaveBeenLastCalledWith('ivanov', {
      page: 1, limit: 25, category: 'failed', from: '', to: '',
    })
    wrapper.unmount()
  })

  it('пагинация запрашивает следующую страницу', async () => {
    const wrapper = mountLH()
    await flushPromises()
    // total 30, limit 25 -> 2 страницы.
    expect(wrapper.vm.totalPages).toBe(2)

    wrapper.vm.goToPage(2)
    await flushPromises()
    expect(getUserAuthEvents).toHaveBeenLastCalledWith('ivanov', expect.objectContaining({ page: 2 }))
    wrapper.unmount()
  })

  it('клиентский поиск фильтрует загруженную страницу по устройству', async () => {
    const wrapper = mountLH()
    await flushPromises()
    expect(wrapper.vm.visibleItems).toHaveLength(3)

    await wrapper.setData({ search: 'firefox' })
    expect(wrapper.vm.visibleItems).toHaveLength(1)
    expect(wrapper.vm.visibleItems[0].id).toBe(1)
    wrapper.unmount()
  })

  it('экспорт тянет расширенную выборку (лимит бэка) по текущим фильтрам', async () => {
    const wrapper = mountLH()
    await flushPromises()
    getUserAuthEvents.mockClear()

    await wrapper.vm.exportToExcel()

    expect(getUserAuthEvents).toHaveBeenCalledWith('ivanov', expect.objectContaining({ limit: 100 }))
    wrapper.unmount()
  })

  it('пустой ответ показывает состояние "Записей о входах нет"', async () => {
    getUserAuthEvents.mockResolvedValue(pageOf([], 0))
    const wrapper = mountLH()
    await flushPromises()
    expect(wrapper.text()).toContain('Записей о входах нет')
    wrapper.unmount()
  })
})
