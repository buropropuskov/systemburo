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
  it('показывает раскладку словами и без фигурных скобок', () => {
    const w = mount(ArchiveSettingsView, { props: { settings: SETTINGS() } })

    const levels = w.findAll('.asv__level').map((l) => l.text())
    expect(levels).toEqual([
      'год',
      'месяц двумя цифрами, месяц прописными, год',
      'дата',
      'дата, номер заявки',
    ])
    // Шаблон в исходном виде - язык того, кто настраивает систему на сервере;
    // на этом экране его быть не должно.
    expect(w.text()).not.toContain('{год}')
    expect(w.text()).not.toContain('{номер}')
  })

  it('приводит пример пути с расширением файла', () => {
    const w = mount(ArchiveSettingsView, { props: { settings: SETTINGS() } })

    // Точка после инициалов не удваивается: сервер срезает концевую точку имени
    // (Windows её всё равно отбрасывает), и пример обязан совпадать с диском.
    expect(w.find('.asv__example').text()).toContain(
      '2026/08 АВГУСТ 2026/03.08.2026/03.08.2026 №20260803-001/Автозаявка_Отдел контроля доступа_03.08.2026_Иванов И.И.xlsx',
    )
    expect(w.find('.asv__example').text()).not.toContain('И.И..xlsx')
  })

  it('к каждому порогу объясняет, что произойдёт', () => {
    const w = mount(ArchiveSettingsView, { props: { settings: SETTINGS() } })
    const text = w.text()

    expect(text).toContain('очередь встаёт')
    expect(text).toContain('придёт уведомление')
    expect(text).toContain('дописывает пропавшие файлы')
    expect(text).toContain('больше не переписываются')
  })

  it('без предельного объёма пишет, что остановит только диск', () => {
    const w = mount(ArchiveSettingsView, { props: { settings: SETTINGS({ quota_bytes: 0 }) } })
    expect(w.text()).toContain('Объём не ограничен')

    const limited = mount(ArchiveSettingsView, { props: { settings: SETTINGS({ quota_bytes: 5 * GB }) } })
    expect(limited.text()).toContain('дорастёт до 5.0 ГБ')
  })

  it('пустые шаблоны не роняют блок', () => {
    const w = mount(ArchiveSettingsView, {
      props: { settings: SETTINGS({ dir_template: '', file_template: '' }) },
    })

    expect(w.findAll('.asv__level')).toHaveLength(0)
    expect(w.find('.asv__file-rule').text()).toContain('не задано')
  })
})
