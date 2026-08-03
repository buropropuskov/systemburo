import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

const api = vi.hoisted(() => ({
  listArchiveItems: vi.fn(),
  reexportApplication: vi.fn(),
}))
vi.mock('@/api/fileArchive', () => api)

import ArchiveFailuresList from '../ArchiveFailuresList.vue'
import BaseDropdown from '@/components/ui/BaseDropdown.vue'
import { useDeletionsStore } from '@/stores/deletions'

const ROW = (over = {}) => ({
  id: 1, application_id: 10, attachment_id: 1, status: 'failed',
  last_error: 'диск переполнен', updated_at: '2026-07-31T10:00:00Z', ...over,
})

function mountList() {
  setActivePinia(createPinia())
  vi.spyOn(useDeletionsStore(), 'notify').mockImplementation(() => {})
  return mount(ArchiveFailuresList)
}

describe('ArchiveFailuresList', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('при монтировании грузит статус failed первой страницей', async () => {
    api.listArchiveItems.mockResolvedValue({ items: [ROW()], meta: { total: 1, page: 1, per_page: 20 } })
    const w = mountList()
    await flushPromises()

    expect(api.listArchiveItems).toHaveBeenCalledWith({ status: 'failed', page: 1, perPage: 20 })
    expect(w.text()).toContain('№10')
    expect(w.text()).toContain('диск переполнен')
  })

  it('смена статуса фильтра сбрасывает страницу и перезагружает список', async () => {
    api.listArchiveItems.mockResolvedValue({ items: [], meta: { total: 0, page: 1, per_page: 20 } })
    const w = mountList()
    await flushPromises()

    await w.findComponent(BaseDropdown).vm.$emit('update:modelValue', 'no_template')
    await flushPromises()

    expect(api.listArchiveItems).toHaveBeenLastCalledWith({ status: 'no_template', page: 1, perPage: 20 })
  })

  it('пустой список показывает сообщение об отсутствии строк', async () => {
    api.listArchiveItems.mockResolvedValue({ items: [], meta: { total: 0, page: 1, per_page: 20 } })
    const w = mountList()
    await flushPromises()

    expect(w.text()).toContain('Строк с этим статусом нет')
  })

  it('ошибку загрузки показывает и уведомляет', async () => {
    api.listArchiveItems.mockRejectedValue(new Error('Сервер недоступен'))
    const w = mountList()
    await flushPromises()

    expect(w.find('.form-error').text()).toBe('Сервер недоступен')
    expect(useDeletionsStore().notify).toHaveBeenCalledWith(expect.objectContaining({ type: 'error' }))
  })

  it('«Повторить» построчно пересоздаёт заявку и перезагружает список', async () => {
    api.listArchiveItems.mockResolvedValue({ items: [ROW()], meta: { total: 1, page: 1, per_page: 20 } })
    api.reexportApplication.mockResolvedValue({ application_id: 10, items: [] })
    const w = mountList()
    await flushPromises()

    await w.find('[data-testid="afl-retry-row"]').trigger('click')
    await flushPromises()

    expect(api.reexportApplication).toHaveBeenCalledWith(10)
    expect(api.listArchiveItems).toHaveBeenCalledTimes(2) // начальная загрузка + перезагрузка после повтора
    expect(useDeletionsStore().notify).toHaveBeenCalledWith(
      expect.objectContaining({ bold: '№10' }),
    )
  })

  it('ошибку построчного повтора уведомляет, но не ломает список', async () => {
    api.listArchiveItems.mockResolvedValue({ items: [ROW()], meta: { total: 1, page: 1, per_page: 20 } })
    api.reexportApplication.mockRejectedValue(new Error('Файловый архив недоступен'))
    const w = mountList()
    await flushPromises()

    await w.find('[data-testid="afl-retry-row"]').trigger('click')
    await flushPromises()

    expect(useDeletionsStore().notify).toHaveBeenCalledWith(expect.objectContaining({ type: 'error' }))
  })

  it('«Повторить все» вызывает пересоздание для каждой уникальной заявки страницы', async () => {
    api.listArchiveItems.mockResolvedValue({
      items: [ROW({ id: 1, application_id: 10 }), ROW({ id: 2, application_id: 10, attachment_id: 2 }), ROW({ id: 3, application_id: 11 })],
      meta: { total: 3, page: 1, per_page: 20 },
    })
    api.reexportApplication.mockResolvedValue({ items: [] })
    const w = mountList()
    await flushPromises()

    await w.find('[data-testid="afl-retry-all"]').trigger('click')
    await flushPromises()

    expect(api.reexportApplication).toHaveBeenCalledTimes(2) // дедуп по application_id: 10 и 11, не 3 вызова
    expect(api.reexportApplication).toHaveBeenCalledWith(10)
    expect(api.reexportApplication).toHaveBeenCalledWith(11)
    expect(useDeletionsStore().notify).toHaveBeenCalledWith(
      expect.objectContaining({ bold: '2 заявок' }),
    )
  })

  it('переход по страницам передаёт номер страницы в запрос', async () => {
    api.listArchiveItems.mockResolvedValue({
      items: Array.from({ length: 20 }, (_, i) => ROW({ id: i + 1, application_id: i + 1 })),
      meta: { total: 40, page: 1, per_page: 20 },
    })
    const w = mountList()
    await flushPromises()

    await w.findComponent({ name: 'UiPager' }).vm.$emit('update:page', 2)
    await flushPromises()

    expect(api.listArchiveItems).toHaveBeenLastCalledWith({ status: 'failed', page: 2, perPage: 20 })
  })
})
