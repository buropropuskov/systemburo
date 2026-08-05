import { describe, it, expect } from 'vitest'

import { describeDirTemplate, describeTemplatePart, renderTemplateExample } from '../archiveTemplate'

const DIR = '{год}/{месяц_2} {МЕСЯЦ} {год}/{дата}/{дата} №{номер}'
const FILE = '{тип}_{организация}_{дата}_{заявитель}'

describe('archiveTemplate', () => {
  it('раскладывает шаблон каталогов по уровням и называет их словами', () => {
    expect(describeDirTemplate(DIR)).toEqual([
      'год',
      'месяц двумя цифрами, месяц прописными, год',
      'дата',
      'дата, номер заявки',
    ])
  })

  it('собирает пример пути из образцов значений', () => {
    expect(renderTemplateExample(DIR)).toBe('2026/08 АВГУСТ 2026/03.08.2026/03.08.2026 №20260803-001')
    expect(renderTemplateExample(FILE)).toBe('Автозаявка_Отдел контроля доступа_03.08.2026_Иванов И.И.')
  })

  it('«тип» называется наименованием вложения - так его и подставляет сервер', () => {
    // Подпись плейсхолдера в консоли говорит «тип вложения», хотя подставляется
    // наименование из справочника. Раздел показывает то, что окажется в имени файла.
    expect(describeTemplatePart('{тип}')).toBe('наименование вложения')
  })

  it('неизвестный плейсхолдер показывается как есть, а не пропадает', () => {
    // Новая подстановка на сервере не должна молча исчезать с экрана: пропажа
    // читалась бы как «уровня нет», хотя каталог создаётся.
    expect(describeTemplatePart('{новый_ключ}')).toBe('{новый_ключ}')
    expect(renderTemplateExample('{новый_ключ}/{год}')).toBe('{новый_ключ}/2026')
  })

  it('пустой шаблон не даёт ни уровней, ни примера', () => {
    expect(describeDirTemplate('')).toEqual([])
    expect(describeDirTemplate(null)).toEqual([])
    expect(renderTemplateExample(undefined)).toBe('')
  })

  it('литерал без плейсхолдеров остаётся текстом уровня', () => {
    expect(describeDirTemplate('Бланки/{год}')).toEqual(['Бланки', 'год'])
  })
})
