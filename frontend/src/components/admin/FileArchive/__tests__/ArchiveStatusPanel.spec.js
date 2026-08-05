import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

const api = vi.hoisted(() => ({
  getArchiveStats: vi.fn(),
}))
vi.mock('@/api/fileArchive', () => api)

import ArchiveStatusPanel from '../ArchiveStatusPanel.vue'
import BaseDropdown from '@/components/ui/BaseDropdown.vue'
import { useDeletionsStore } from '@/stores/deletions'

const GB = 1024 * 1024 * 1024

function baseStats(overrides = {}) {
  return {
    used_bytes: 2 * GB,
    free_bytes: 6 * GB,
    file_count: 120,
    periods: [
      { month: '2026-07', bytes: 2 * GB, file_count: 120 },
    ],
    statuses: { ok: 118, failed: 3, no_template: 5, pending: 0, skipped: 0, blocked: 0, orphan: 0 },
    composition: { applications: 40, blanks: 80, snapshots: 40 },
    attachment_types: [
      { name: 'Автозаявка', bytes: 1.5 * GB, file_count: 50 },
      { name: 'Заявка на ввоз', bytes: 0.5 * GB, file_count: 30 },
    ],
    last_written_at: `${new Date().getFullYear()}-07-31T11:42:00Z`,
    disk: {
      total_bytes: 10 * GB,
      free_bytes: 6 * GB,
      archive_bytes: 2 * GB,
      uploads_bytes: 1 * GB,
      database_bytes: 0.5 * GB,
      logs_bytes: 0.1 * GB,
      other_bytes: 0.4 * GB,
      partitions: [
        { labels: ['Архив', 'Загрузки', 'Логи'], total_bytes: 10 * GB, free_bytes: 6 * GB },
      ],
    },
    generated_at: '2026-07-31T12:00:00Z',
    ...overrides,
  }
}

describe('ArchiveStatusPanel', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    const del = useDeletionsStore()
    vi.spyOn(del, 'notify').mockImplementation(() => {})
  })

  it('загружает сводку при монтировании и показывает плитки', async () => {
    api.getArchiveStats.mockResolvedValue(baseStats())
    const w = mount(ArchiveStatusPanel)
    await flushPromises()

    expect(api.getArchiveStats).toHaveBeenCalledTimes(1)
    expect(w.text()).toContain('Занято архивом')
    expect(w.text()).toContain('2.0 ГБ')
    expect(w.text()).toContain('Свободно на диске')
    expect(w.text()).toContain('6.0 ГБ')
    // Момент, а не месяц: администратор смотрит сюда, чтобы понять, пишется ли
    // архив сейчас. Месяц «Июль 2026» ниже по странице живёт своей жизнью в
    // разбивке по периодам, поэтому проверяем саму плитку, а не текст панели.
    // Год записи текущего года не показываем: полный момент не помещался в
    // плитку и переносом растягивал весь ряд. Он остаётся в подсказке.
    const lastTile = w.findAll('.archive-status__tile').find((t) => t.text().includes('Последняя запись'))
    expect(lastTile.text()).toMatch(/31\.07, \d{2}:\d{2}/)
    expect(lastTile.text()).not.toContain('Июль 2026')
    expect(lastTile.attributes('title')).toMatch(/31\.07\.\d{4}/)
  })

  it('запись прошлых лет показывается с годом, а не со временем', async () => {
    // Дата строится от текущего года, иначе тест переживёт ровно до Нового года.
    const lastYear = new Date().getFullYear() - 1
    api.getArchiveStats.mockResolvedValue(baseStats({
      last_written_at: `${lastYear}-11-12T09:30:00Z`,
    }))
    const w = mount(ArchiveStatusPanel)
    await flushPromises()

    const lastTile = w.findAll('.archive-status__tile').find((t) => t.text().includes('Последняя запись'))
    expect(lastTile.text()).toContain(`12.11.${lastYear}`)
    // Минуты прошлогодней записи не нужны: важнее, что это было давно.
    expect(lastTile.text()).not.toMatch(/\d{2}:\d{2}/)
  })

  it('файлы разложены по видам: заявки, бланки, описания', async () => {
    api.getArchiveStats.mockResolvedValue(baseStats())
    const w = mount(ArchiveStatusPanel)
    await flushPromises()

    const tiles = w.findAll('.archive-status__tile')
    const byLabel = (label) => tiles.find((t) => t.text().includes(label))
    expect(byLabel('Заявок').text()).toContain('40')
    expect(byLabel('Бланков').text()).toContain('80')
    expect(byLabel('Описаний заявок').text()).toContain('40')
    // Одного числа «Файлов» больше нет: оно складывало бланки со служебными
    // описаниями и не отвечало ни на один вопрос администратора.
    expect(tiles.some((t) => t.text().includes('Файлов'))).toBe(false)
  })

  it('подписывает момент снятия сводки - она кэшируется и отстаёт от диска', async () => {
    api.getArchiveStats.mockResolvedValue(baseStats())
    const w = mount(ArchiveStatusPanel)
    await flushPromises()

    const snapshot = w.find('.archive-status__snapshot')
    expect(snapshot.exists()).toBe(true)
    expect(snapshot.text()).toContain('пять минут')
  })

  it('момента последней записи нет - показывается месяц из разбивки', async () => {
    api.getArchiveStats.mockResolvedValue(baseStats({ last_written_at: null }))
    const w = mount(ArchiveStatusPanel)
    await flushPromises()

    const lastTile = w.findAll('.archive-status__tile').find((t) => t.text().includes('Последняя запись'))
    expect(lastTile.text()).toContain('Июль 2026')
  })

  it('вложения без шаблона и ошибки берутся из карты статусов реестра', async () => {
    api.getArchiveStats.mockResolvedValue(baseStats())
    const w = mount(ArchiveStatusPanel)
    await flushPromises()

    const tiles = w.findAll('.archive-status__tile')
    const errorsTile = tiles.find((t) => t.text().includes('Ошибок'))
    const noTemplateTile = tiles.find((t) => t.text().includes('Без шаблона'))
    expect(errorsTile.text()).toContain('3')
    expect(noTemplateTile.text()).toContain('5')
  })

  it('пустой архив - «последняя запись» прочерк', async () => {
    api.getArchiveStats.mockResolvedValue(baseStats({ periods: [], last_written_at: null }))
    const w = mount(ArchiveStatusPanel)
    await flushPromises()

    const tiles = w.findAll('.archive-status__tile')
    const lastTile = tiles.find((t) => t.text().includes('Последняя запись'))
    expect(lastTile.text()).toContain('—')
  })

  it('один раздел диска - дропдаун выбора не показывается, состав полный', async () => {
    api.getArchiveStats.mockResolvedValue(baseStats())
    const w = mount(ArchiveStatusPanel)
    await flushPromises()

    expect(w.findComponent(BaseDropdown).exists()).toBe(false)
    expect(w.text()).toContain('Архив бланков: 2.0 ГБ')
    expect(w.text()).toContain('База данных: 512.0 МБ')
    expect(w.find('.archive-status__disk-caption').exists()).toBe(false)
  })

  it('несколько разделов - выбор непервичного раздела упрощает состав до занято/свободно', async () => {
    const stats = baseStats({
      disk: {
        total_bytes: 10 * GB, free_bytes: 6 * GB, archive_bytes: 2 * GB,
        uploads_bytes: 0, database_bytes: 0.5 * GB, logs_bytes: 0, other_bytes: 0.4 * GB,
        partitions: [
          { labels: ['Архив'], total_bytes: 10 * GB, free_bytes: 6 * GB },
          { labels: ['Логи'], total_bytes: 4 * GB, free_bytes: 1 * GB },
        ],
      },
    })
    api.getArchiveStats.mockResolvedValue(stats)
    const w = mount(ArchiveStatusPanel)
    await flushPromises()

    expect(w.findComponent(BaseDropdown).exists()).toBe(true)
    expect(w.findComponent(BaseDropdown).props('options')).toEqual([
      { index: 0, label: 'Архив' },
      { index: 1, label: 'Логи' },
    ])

    await w.findComponent(BaseDropdown).vm.$emit('update:modelValue', 1)
    await flushPromises()

    expect(w.text()).toContain('Занято: 3.0 ГБ') // 4 - 1 ГБ
    expect(w.text()).toContain('Свободно: 1.0 ГБ')
    // Подписи разделов приходят с сервера как есть - переименование долей на
    // фронте их не касается.
    expect(w.find('.archive-status__disk-caption').text()).toContain('Логи')
    expect(w.text()).not.toContain('Архив: ')
  })

  it('наведение на долю полосы подписывает её название, размер и долю раздела', async () => {
    api.getArchiveStats.mockResolvedValue(baseStats())
    const w = mount(ArchiveStatusPanel)
    await flushPromises()

    expect(w.find('.archive-status__disk-tip').exists()).toBe(false)

    const segments = w.findAll('.archive-status__disk-seg')
    await segments[0].trigger('mouseenter')

    const tip = w.find('.archive-status__disk-tip')
    expect(tip.exists()).toBe(true)
    expect(tip.text()).toContain('Архив')
    expect(tip.text()).toContain('2.0 ГБ')
    expect(tip.text()).toContain('20.0 %')
    expect(segments[0].classes()).toContain('archive-status__disk-seg--active')

    await segments[0].trigger('mouseleave')
    expect(w.find('.archive-status__disk-tip').exists()).toBe(false)
  })

  it('подсказка доступна с клавиатуры: фокус на доле показывает её, потеря - убирает', async () => {
    api.getArchiveStats.mockResolvedValue(baseStats())
    const w = mount(ArchiveStatusPanel)
    await flushPromises()

    const segment = w.findAll('.archive-status__disk-seg')[1]
    expect(segment.attributes('tabindex')).toBe('0')

    await segment.trigger('focus')
    expect(w.find('.archive-status__disk-tip').text()).toContain('Файлы из заявок')

    await segment.trigger('blur')
    expect(w.find('.archive-status__disk-tip').exists()).toBe(false)
  })

  it('крайние доли прижимают подсказку к краю, чтобы она не уехала за карточку', async () => {
    api.getArchiveStats.mockResolvedValue(baseStats())
    const w = mount(ArchiveStatusPanel)
    await flushPromises()

    const segments = w.findAll('.archive-status__disk-seg')
    await segments[0].trigger('mouseenter')
    // Архив - первые 20%, центр доли на 10%: подсказка прижата к левому краю.
    expect(w.find('.archive-status__disk-tip').attributes('style')).toContain('left: 0%')

    await segments[0].trigger('mouseleave')
    // Широкая доля посередине остаётся центрированной.
    await segments[segments.length - 1].trigger('mouseenter')
    expect(w.find('.archive-status__disk-tip').attributes('style')).toContain('translateX(-50%)')
  })

  it('узкая доля у правого края прижимает подсказку вправо', async () => {
    const base = baseStats()
    // Свободного почти не осталось: последняя доля - тонкая полоска у самого края.
    api.getArchiveStats.mockResolvedValue(baseStats({
      free_bytes: 0.05 * GB,
      disk: {
        ...base.disk,
        free_bytes: 0.05 * GB,
        other_bytes: 5.35 * GB,
        partitions: [{ labels: ['Архив', 'Загрузки', 'Логи'], total_bytes: 10 * GB, free_bytes: 0.05 * GB }],
      },
    }))
    const w = mount(ArchiveStatusPanel)
    await flushPromises()

    const segments = w.findAll('.archive-status__disk-seg')
    await segments[segments.length - 1].trigger('mouseenter')
    const style = w.find('.archive-status__disk-tip').attributes('style')
    expect(style).toContain('left: 100%')
    expect(style).toContain('translateX(-100%)')
  })

  it('доля меньше десятой процента подписывается «<0.1 %», а не нулём', async () => {
    api.getArchiveStats.mockResolvedValue(baseStats({
      disk: { ...baseStats().disk, logs_bytes: 1024 },
    }))
    const w = mount(ArchiveStatusPanel)
    await flushPromises()

    const logsSegment = w.findAll('.archive-status__disk-seg')[3]
    await logsSegment.trigger('mouseenter')
    const tip = w.find('.archive-status__disk-tip')
    expect(tip.text()).toContain('Журналы работы')
    expect(tip.text()).toContain('<0.1 %')
  })

  it('свободного места мало - полоса помечается предупреждением/критично', async () => {
    const warnStats = baseStats({
      disk: {
        total_bytes: 10 * GB, free_bytes: 1.5 * GB, archive_bytes: 8 * GB,
        uploads_bytes: 0, database_bytes: 0, logs_bytes: 0, other_bytes: 0.5 * GB,
        partitions: [{ labels: ['Архив'], total_bytes: 10 * GB, free_bytes: 1.5 * GB }],
      },
    })
    api.getArchiveStats.mockResolvedValue(warnStats)
    const w = mount(ArchiveStatusPanel)
    await flushPromises()
    expect(w.find('.archive-status__disk-bar').classes()).toContain('archive-status__disk-bar--warning')

    const criticalStats = baseStats({
      disk: {
        total_bytes: 10 * GB, free_bytes: 0.5 * GB, archive_bytes: 9 * GB,
        uploads_bytes: 0, database_bytes: 0, logs_bytes: 0, other_bytes: 0.5 * GB,
        partitions: [{ labels: ['Архив'], total_bytes: 10 * GB, free_bytes: 0.5 * GB }],
      },
    })
    api.getArchiveStats.mockResolvedValue(criticalStats)
    const w2 = mount(ArchiveStatusPanel)
    await flushPromises()
    expect(w2.find('.archive-status__disk-bar').classes()).toContain('archive-status__disk-bar--critical')
  })

  it('при ошибке загрузки показывает сообщение и уведомляет через notify', async () => {
    api.getArchiveStats.mockRejectedValue(new Error('Сервер недоступен'))
    const w = mount(ArchiveStatusPanel)
    await flushPromises()

    expect(w.find('.archive-status__error').text()).toBe('Сервер недоступен')
    expect(useDeletionsStore().notify).toHaveBeenCalledWith(
      expect.objectContaining({ type: 'error' }),
    )
  })

  it('defineExpose(refresh) повторно запрашивает сводку', async () => {
    api.getArchiveStats.mockResolvedValue(baseStats())
    const w = mount(ArchiveStatusPanel)
    await flushPromises()
    expect(api.getArchiveStats).toHaveBeenCalledTimes(1)

    await w.vm.refresh()
    expect(api.getArchiveStats).toHaveBeenCalledTimes(2)
  })
})
