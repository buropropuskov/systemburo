import { describe, it, expect } from 'vitest'

import { renderTemplateExample } from '../archiveTemplate'

const DIR = '{год}/{месяц_2} {МЕСЯЦ} {год}/{дата}/{дата} №{номер}'
const FILE = '{тип}_{организация}_{дата}_{заявитель}'

describe('archiveTemplate', () => {
  it('собирает пример пути из образцов значений', () => {
    expect(renderTemplateExample(DIR)).toBe('2026/08 АВГУСТ 2026/03.08.2026/03.08.2026 №20260803-001')
    expect(renderTemplateExample(FILE)).toBe('Автозаявка_Отдел контроля доступа_03.08.2026_Иванов И.И.')
  })

  it('неизвестный плейсхолдер показывается как есть, а не пропадает', () => {
    // Новая подстановка на сервере не должна молча исчезать из примера: пропажа
    // читалась бы как «уровня нет», хотя каталог создаётся.
    expect(renderTemplateExample('{новый_ключ}/{год}')).toBe('{новый_ключ}/2026')
  })

  it('пустой шаблон даёт пустой пример, а не «undefined»', () => {
    expect(renderTemplateExample(undefined)).toBe('')
    expect(renderTemplateExample(null)).toBe('')
  })

})
