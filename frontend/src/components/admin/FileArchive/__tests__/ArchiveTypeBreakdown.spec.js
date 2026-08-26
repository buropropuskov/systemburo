import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'

import ArchiveTypeBreakdown from '../ArchiveTypeBreakdown.vue'

const MB = 1024 * 1024

function types() {
  return [
    { name: 'Автозаявка', bytes: 40 * MB, file_count: 50 },
    { name: 'Заявка на ввоз', bytes: 10 * MB, file_count: 12 },
    { name: 'Вложение удалено', bytes: 1 * MB, file_count: 2 },
  ]
}

describe('ArchiveTypeBreakdown', () => {
  it('показывает типы в порядке бэка, с местом и числом файлов', async () => {
    const w = mount(ArchiveTypeBreakdown, { props: { types: types() } })

    const rows = w.findAll('.archive-types__row:not(.archive-types__row--head)')
    expect(rows).toHaveLength(3)
    expect(rows[0].text()).toContain('Автозаявка')
    expect(rows[0].text()).toContain('40.0 МБ')
    expect(rows[0].text()).toContain('50')
    expect(rows[2].text()).toContain('Вложение удалено')
  })

  it('доля полосы считается от самого тяжёлого типа, а не от всего архива', async () => {
    const w = mount(ArchiveTypeBreakdown, { props: { types: types() } })

    const shares = w.findAll('.archive-types__share').map((s) => s.attributes('style'))
    expect(shares[0]).toContain('width: 100%')
    expect(shares[1]).toContain('width: 25%')
    // Хвост не вырождается в невидимую полоску: минимум остаётся заметным.
    expect(shares[2]).toContain('width: 3%')
  })

  it('пустой список - явное сообщение, а не пустая таблица', async () => {
    const w = mount(ArchiveTypeBreakdown, { props: { types: [] } })

    expect(w.find('.archive-types__table').exists()).toBe(false)
    expect(w.text()).toContain('Бланков в архиве пока нет')
  })

  it('тип без имени подписывается, а не показывается пустой строкой', async () => {
    const w = mount(ArchiveTypeBreakdown, { props: { types: [{ name: '', bytes: 5, file_count: 1 }] } })

    expect(w.text()).toContain('Без наименования')
  })
})
