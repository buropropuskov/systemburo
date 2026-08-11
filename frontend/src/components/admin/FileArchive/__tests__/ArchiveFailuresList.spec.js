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
  // Бэк собирает номер уже со знаком номера (application_service.go), фикстура
  // повторяет боевой формат - иначе дубль «№№» тестом не ловится.
  application_number: '№ 20260731-010', attachment_name: 'Автозаявка',
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

  it('при монтировании грузит ленту целиком, без фильтра по состоянию', async () => {
    api.listArchiveItems.mockResolvedValue({ items: [ROW()], meta: { total: 1, page: 1, per_page: 20 } })
    const w = mountList()
    await flushPromises()

    expect(api.listArchiveItems).toHaveBeenCalledWith({ status: '', page: 1, perPage: 20 })
    // Номер заявки, а не внутренний идентификатор: по «№10» её не найти.
    expect(w.text()).toContain('№ 20260731-010')
    expect(w.text()).not.toContain('№№')
    expect(w.text()).toContain('диск переполнен')
  })

  it('смена статуса фильтра сбрасывает страницу и перезагружает список', async () => {
    api.listArchiveItems.mockResolvedValue({ items: [], meta: { total: 0, page: 1, per_page: 20 } })
    const w = mountList()
    await flushPromises()

    await w.findComponent(BaseDropdown).vm.$emit('update:modelValue', 'no_template')
    await flushPromises()

    // Рядом с запросом страницы идёт запрос счётчика очереди, поэтому проверяем
    // факт вызова, а не последний по счёту.
    expect(api.listArchiveItems).toHaveBeenCalledWith({ status: 'no_template', page: 1, perPage: 20 })
  })

  it('пустой список показывает сообщение об отсутствии строк', async () => {
    api.listArchiveItems.mockResolvedValue({ items: [], meta: { total: 0, page: 1, per_page: 20 } })
    const w = mountList()
    await flushPromises()

    expect(w.text()).toContain('В архиве пока нет ни одной записи')
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
    // Начальная загрузка и перезагрузка после повтора, каждая со своим запросом
    // счётчика очереди.
    expect(api.listArchiveItems).toHaveBeenCalledTimes(4)
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

  it('показывает счётчик очереди отдельным запросом', async () => {
    api.listArchiveItems.mockImplementation(({ status }) => Promise.resolve(
      status === 'pending'
        ? { items: [], meta: { total: 7, page: 1, per_page: 1 } }
        : { items: [ROW()], meta: { total: 1, page: 1, per_page: 20 } },
    ))
    const w = mountList()
    await flushPromises()

    expect(api.listArchiveItems).toHaveBeenCalledWith({ status: 'pending', page: 1, perPage: 1 })
    expect(w.find('[data-testid="afl-queue-count"]').text()).toContain('7')
  })

  it('пустая очередь счётчик не показывает - нулю тут делать нечего', async () => {
    api.listArchiveItems.mockImplementation(({ status }) => Promise.resolve(
      status === 'pending'
        ? { items: [], meta: { total: 0, page: 1, per_page: 1 } }
        : { items: [ROW()], meta: { total: 1, page: 1, per_page: 20 } },
    ))
    const w = mountList()
    await flushPromises()

    expect(w.find('[data-testid="afl-queue-count"]').exists()).toBe(false)
  })

  it('записанной строке повтор не предлагается, а ждущей - показан срок попытки', async () => {
    api.listArchiveItems.mockResolvedValue({
      items: [
        ROW({ id: 1, status: 'ok', last_error: '', file_name: 'Автозаявка.xlsx', attachment_name: 'Автозаявка' }),
        ROW({ id: 2, application_id: 11, status: 'failed', next_attempt_at: '2026-07-31T10:05:00Z' }),
      ],
      meta: { total: 2, page: 1, per_page: 20 },
    })
    const w = mountList()
    await flushPromises()

    // У записанной строки показано наименование вложения, кнопки повтора нет.
    const rows = w.findAll('.afl__row:not(.afl__row--head)')
    expect(rows[0].text()).toContain('Автозаявка')
    expect(rows[0].find('[data-testid="afl-retry-row"]').exists()).toBe(false)

    // У сорвавшейся - кнопка и подпись, когда её возьмут снова: пауза до повтора
    // доходит до пяти минут, и без подписи строка выглядит зависшей.
    expect(rows[1].find('[data-testid="afl-retry-row"]').exists()).toBe(true)
    expect(rows[1].text()).toContain('Повтор в')
  })

  it('состояние без файла и без ошибки объясняется словами, а не прочерком', async () => {
    api.listArchiveItems.mockResolvedValue({
      items: [ROW({ id: 1, status: 'no_template', last_error: '', file_name: '', attachment_name: 'Автозаявка' })],
      meta: { total: 1, page: 1, per_page: 20 },
    })
    const w = mountList()
    await flushPromises()

    // Видно и какого вложения не хватает бланка, и в чём дело.
    expect(w.text()).toContain('Автозаявка')
    expect(w.text()).toContain('бланк не настроен')
  })

  it('«Повторить все» не трогает записанные строки на странице', async () => {
    api.listArchiveItems.mockResolvedValue({
      items: [
        ROW({ id: 1, application_id: 10, status: 'ok', last_error: '' }),
        ROW({ id: 2, application_id: 11, status: 'failed' }),
      ],
      meta: { total: 2, page: 1, per_page: 20 },
    })
    api.reexportApplication.mockResolvedValue({ items: [] })
    const w = mountList()
    await flushPromises()

    await w.find('[data-testid="afl-retry-all"]').trigger('click')
    await flushPromises()

    expect(api.reexportApplication).toHaveBeenCalledTimes(1)
    expect(api.reexportApplication).toHaveBeenCalledWith(11)
  })

  it('сервер ещё без новых полей - показываем идентификатор, а не «удалена»', async () => {
    // Фронт выкатывается раньше бэкенда: пока поля нет, писать «Заявка удалена»
    // про живую заявку нельзя. Отсутствие поля и пустое значение - разные вещи.
    const legacy = { id: 1, application_id: 77, attachment_id: 5, status: 'ok', updated_at: '2026-07-31T10:00:00Z' }
    api.listArchiveItems.mockResolvedValue({ items: [legacy], meta: { total: 1, page: 1, per_page: 20 } })
    const w = mountList()
    await flushPromises()

    expect(w.text()).toContain('№77')
    expect(w.text()).not.toContain('Заявка удалена')
    expect(w.text()).not.toContain('Вложение удалено')
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

    expect(api.listArchiveItems).toHaveBeenCalledWith({ status: '', page: 2, perPage: 20 })
  })
})
