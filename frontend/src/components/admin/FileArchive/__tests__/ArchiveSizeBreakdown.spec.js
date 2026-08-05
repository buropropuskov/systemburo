import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import ArchiveSizeBreakdown from '../ArchiveSizeBreakdown.vue'

const periods = [
  { month: '2026-07', bytes: 5 * 1024 * 1024, file_count: 10 },
  { month: '2026-06', bytes: 3 * 1024 * 1024, file_count: 6 },
  { month: '2025-12', bytes: 2 * 1024 * 1024, file_count: 4 },
]

describe('ArchiveSizeBreakdown', () => {
  it('пустой архив показывает заглушку, а не пустую таблицу', () => {
    const w = mount(ArchiveSizeBreakdown, { props: { periods: [] } })
    expect(w.text()).toContain('Архив пока пуст')
    expect(w.find('.rt-table').exists()).toBe(false)
  })

  it('группирует месяцы по году и суммирует место/файлы в шапке года', () => {
    const w = mount(ArchiveSizeBreakdown, { props: { periods } })
    const years = w.findAll('.archive-breakdown__row--year')
    expect(years).toHaveLength(2)
    expect(years[0].text()).toContain('2026')
    expect(years[0].text()).toContain('8.0 МБ') // 5+3 МБ
    expect(years[0].text()).toContain('16') // 10+6 файлов
    expect(years[1].text()).toContain('2025')
  })

  it('самый свежий год открыт по умолчанию, остальные свёрнуты', () => {
    const w = mount(ArchiveSizeBreakdown, { props: { periods } })
    const years = w.findAll('.archive-breakdown__row--year')
    expect(years[0].attributes('aria-expanded')).toBe('true')
    expect(years[1].attributes('aria-expanded')).toBe('false')

    // Месяцы всех лет остаются в DOM (grid-template-rows 0fr -> 1fr сжимает
    // визуально, не убирает элемент) - раскрытие проверяется классом обёртки.
    const wrappers = w.findAll('.archive-breakdown__months')
    expect(wrappers[0].classes()).toContain('archive-breakdown__months--open')
    expect(wrappers[1].classes()).not.toContain('archive-breakdown__months--open')
  })

  it('клик по году переключает раскрытие независимо от остальных', async () => {
    const w = mount(ArchiveSizeBreakdown, { props: { periods } })
    const years = w.findAll('.archive-breakdown__row--year')

    await years[1].trigger('click')
    expect(years[1].attributes('aria-expanded')).toBe('true')
    expect(years[0].attributes('aria-expanded')).toBe('true') // первый год не тронут

    await years[0].trigger('click')
    expect(years[0].attributes('aria-expanded')).toBe('false')
  })

  it('раскрытый год и его месяцы смыкаются в одну карточку', async () => {
    const w = mount(ArchiveSizeBreakdown, { props: { periods } })
    const years = w.findAll('.archive-breakdown__row--year')

    // Открытый год теряет нижние скругления: иначе его рамка обрывалась на
    // закруглениях, а список месяцев начинался прямыми углами - и боковые линии
    // выглядели оторванными.
    expect(years[0].classes()).toContain('archive-breakdown__row--year-open')
    expect(years[1].classes()).not.toContain('archive-breakdown__row--year-open')

    await years[0].trigger('click')
    expect(years[0].classes()).not.toContain('archive-breakdown__row--year-open')
  })

  it('месяц внутри года подписан русским названием', () => {
    const w = mount(ArchiveSizeBreakdown, { props: { periods } })
    expect(w.text()).toContain('Июль 2026')
    expect(w.text()).toContain('Июнь 2026')
  })
})
