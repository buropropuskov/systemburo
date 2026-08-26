import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'

import ArchiveSettingsView from '../ArchiveSettingsView.vue'

const GB = 1024 * 1024 * 1024

const SETTINGS = (over = {}) => ({
  enabled: true,
  dir_template: '{год}/{месяц_2} {МЕСЯЦ} {год}/{дата}/{дата} №{номер}',
  file_template: '{тип}_{организация}_{дата}_{заявитель}',
  quota_bytes: 0,
  min_free_bytes: 2 * GB,
  warn_percent: 80,
  recheck_days: 30,
  freeze_after_days: 30,
  zip_max_bytes: 2 * GB,
  ...over,
})

describe('ArchiveSettingsView', () => {
  it('показывает раскладку деревом готовых имён, без фигурных скобок', () => {
    const w = mount(ArchiveSettingsView, { props: { settings: SETTINGS() } })

    const nodes = w.findAll('.asv__tree-node').map((n) => n.text().replace('└', '').trim())
    expect(nodes).toEqual([
      '2026',
      '08 АВГУСТ 2026',
      '03.08.2026',
      '03.08.2026 №20260803-001',
      'Автозаявка_Отдел контроля доступа_03.08.2026_Иванов И.И.xlsx',
    ])
    // Шаблон в исходном виде - язык того, кто настраивает систему на сервере;
    // на этом экране его быть не должно.
    expect(w.text()).not.toContain('{год}')
    expect(w.text()).not.toContain('{номер}')
  })

  it('имя файла - последний узел дерева, точка не удваивается', () => {
    const w = mount(ArchiveSettingsView, { props: { settings: SETTINGS() } })

    // Сервер срезает концевую точку имени (Windows её всё равно отбрасывает),
    // поэтому «Иванов И.И.» и расширение дают одну точку, а не две.
    const file = w.find('.asv__tree-file')
    expect(file.text()).toContain('Иванов И.И.xlsx')
    expect(file.text()).not.toContain('И.И..xlsx')
  })

  it('каждое правило читается парой «когда - что тогда»', () => {
    const w = mount(ArchiveSettingsView, { props: { settings: SETTINGS() } })

    const rows = w.findAll('.asv__row').map((r) => r.text().replace(/\s+/g, ' '))
    expect(rows.some((r) => r.includes('Свободно меньше 2.0 ГБ') && r.includes('запись встаёт'))).toBe(true)
    expect(rows.some((r) => r.includes('Раздел занят на 80 %') && r.includes('уведомление'))).toBe(true)
    expect(rows.some((r) => r.includes('Каждую ночь') && r.includes('пропавшее дописывается'))).toBe(true)
    expect(rows.some((r) => r.includes('Через 30 дн.') && r.includes('замораживаются'))).toBe(true)
  })

  it('незаданный предел объёма не занимает строку в перечне правил', () => {
    const w = mount(ArchiveSettingsView, { props: { settings: SETTINGS({ quota_bytes: 0 }) } })
    // Отсутствие правила - это не правило: оно уходит в сноску под перечень.
    expect(w.findAll('.asv__row')).toHaveLength(5)
    expect(w.find('.asv__note').text()).toContain('не задан')

    const limited = mount(ArchiveSettingsView, { props: { settings: SETTINGS({ quota_bytes: 5 * GB }) } })
    expect(limited.findAll('.asv__row')).toHaveLength(6)
    expect(limited.text()).toContain('Архив дорос до 5.0 ГБ')
    expect(limited.find('.asv__note').exists()).toBe(false)
  })

  it('пустые шаблоны не роняют блок', () => {
    const w = mount(ArchiveSettingsView, {
      props: { settings: SETTINGS({ dir_template: '', file_template: '' }) },
    })

    expect(w.findAll('.asv__tree-node')).toHaveLength(0)
  })
})
