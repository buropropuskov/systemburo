import { describe, it, expect } from 'vitest'
import {
  filterCyrillicLetters,
  filterLatinLetters,
  filterBothLetters,
  filterMixedCyrillic,
  filterMixedLatin,
  filterMixedBoth,
  validatePartValue,
  formatPartValue,
  initializeNumberParts,
  formatNumberForDisplay,
} from '../useNumberFormat'

describe('filterCyrillicLetters', () => {
  it.each([
    ['АВЕКМНОРСТУХ', 'АВЕКМНОРСТУХ'],
    ['АВЕ123', 'АВЕ'],
    ['', ''],
    ['БГДЖ', ''],
    ['АБВГДЕ', 'АВЕ'],
    ['АМ999ОР', 'АМОР'],
  ])('"%s" -> "%s" (default allowed)', (input, expected) => {
    expect(filterCyrillicLetters(input, null)).toBe(expected)
  })

  it.each([
    ['АБВГ', 'АБ', 'АБ'],
    ['АБВГ', 'ВГ', 'ВГ'],
    ['АБВГ', 'Д', ''],
  ])('"%s" with allowedLetters="%s" -> "%s"', (input, allowed, expected) => {
    expect(filterCyrillicLetters(input, allowed)).toBe(expected)
  })
})

describe('filterLatinLetters', () => {
  it.each([
    ['ABC123', 'ABC'],
    ['HELLO', 'HELLO'],
    ['abc', ''],
    ['', ''],
    ['A1B2C3', 'ABC'],
    ['123', ''],
  ])('"%s" -> "%s" (default allowed)', (input, expected) => {
    expect(filterLatinLetters(input, null)).toBe(expected)
  })

  it.each([
    ['ABCD', 'AC', 'AC'],
    ['ABCD', 'BD', 'BD'],
    ['ABCD', 'XY', ''],
  ])('"%s" with allowedLetters="%s" -> "%s"', (input, allowed, expected) => {
    expect(filterLatinLetters(input, allowed)).toBe(expected)
  })
})

describe('filterBothLetters', () => {
  it.each([
    ['АBC', 'АBC'],
    ['А1B2', 'АB'],
    ['123', ''],
    ['', ''],
    ['АвВс', 'АВ'],
    ['АВCDE', 'АВCDE'],
  ])('"%s" -> "%s" (default allowed)', (input, expected) => {
    expect(filterBothLetters(input, null)).toBe(expected)
  })

  it.each([
    ['АБCD', 'АБ', 'АБ'],
    ['АBСD', 'АB', 'АB'],
  ])('"%s" with allowedLetters="%s" -> "%s"', (input, allowed, expected) => {
    expect(filterBothLetters(input, allowed)).toBe(expected)
  })
})

describe('filterMixedCyrillic', () => {
  it.each([
    ['А123В', '123АВ'],
    ['999', '999'],
    ['АВЕ', 'АВЕ'],
    ['БГД123', '123'],
    ['', ''],
  ])('"%s" -> "%s"', (input, expected) => {
    expect(filterMixedCyrillic(input, null)).toBe(expected)
  })
})

describe('filterMixedLatin', () => {
  it.each([
    ['A123B', '123AB'],
    ['999', '999'],
    ['ABC', 'ABC'],
    ['abc123', '123'],
    ['', ''],
  ])('"%s" -> "%s"', (input, expected) => {
    expect(filterMixedLatin(input, null)).toBe(expected)
  })
})

describe('filterMixedBoth', () => {
  it.each([
    ['А1B2', '12АB'],
    ['999', '999'],
    ['АBC', 'АBC'],
    ['', ''],
  ])('"%s" -> "%s"', (input, expected) => {
    expect(filterMixedBoth(input, null)).toBe(expected)
  })
})

describe('validatePartValue', () => {
  describe('cell_type=numbers', () => {
    const cell = { cell_type: 'numbers', max_length: 3 }

    it.each([
      ['123', '123'],
      ['12345', '123'],
      ['abc', ''],
      ['1a2b3c', '123'],
      ['', ''],
    ])('"%s" -> "%s"', (input, expected) => {
      expect(validatePartValue(input, cell)).toBe(expected)
    })
  })

  describe('cell_type=letters, cyrillic', () => {
    const cell = { cell_type: 'letters', alphabet_type: 'cyrillic', max_length: 3 }

    it.each([
      ['аве', 'АВЕ'],
      ['АВЕК', 'АВЕ'],
      ['бгд', ''],
      ['123', ''],
    ])('"%s" -> "%s"', (input, expected) => {
      expect(validatePartValue(input, cell)).toBe(expected)
    })
  })

  describe('cell_type=letters, latin', () => {
    const cell = { cell_type: 'letters', alphabet_type: 'latin', max_length: 2 }

    it.each([
      ['abc', 'AB'],
      ['XYZ', 'XY'],
      ['123', ''],
    ])('"%s" -> "%s"', (input, expected) => {
      expect(validatePartValue(input, cell)).toBe(expected)
    })
  })

  describe('cell_type=letters, both', () => {
    const cell = { cell_type: 'letters', alphabet_type: 'both', max_length: 3 }

    it.each([
      ['аBв', 'АBВ'],
      ['XАY', 'XАY'],
    ])('"%s" -> "%s"', (input, expected) => {
      expect(validatePartValue(input, cell)).toBe(expected)
    })
  })

  describe('cell_type=mixed, cyrillic', () => {
    const cell = { cell_type: 'mixed', alphabet_type: 'cyrillic', max_length: 5 }

    it('keeps digits and allowed Cyrillic letters', () => {
      expect(validatePartValue('а1в2е', cell)).toBe('12АВЕ')
    })
  })

  describe('cell_type=mixed, latin', () => {
    const cell = { cell_type: 'mixed', alphabet_type: 'latin', max_length: 5 }

    it('keeps digits and uppercase Latin letters', () => {
      expect(validatePartValue('a1b2c', cell)).toBe('12ABC')
    })
  })

  describe('cell_type=mixed, both', () => {
    const cell = { cell_type: 'mixed', alphabet_type: 'both', max_length: 5 }

    it('keeps digits, Latin and Cyrillic uppercase', () => {
      expect(validatePartValue('а1B2', cell)).toBe('12АB')
    })
  })

  it('respects allowed_letters on cell', () => {
    const cell = { cell_type: 'letters', alphabet_type: 'cyrillic', max_length: 10, allowed_letters: 'АВ' }
    expect(validatePartValue('АВЕКМ', cell)).toBe('АВ')
  })
})

describe('formatPartValue', () => {
  it('pads left with zeros', () => {
    const cell = { cell_type: 'numbers', max_length: 3, padding_side: 'left', padding_char: '0' }
    expect(formatPartValue('1', cell)).toBe('001')
  })

  it('pads right with zeros', () => {
    const cell = { cell_type: 'numbers', max_length: 3, padding_side: 'right', padding_char: '0' }
    expect(formatPartValue('1', cell)).toBe('100')
  })

  it('defaults padding_char to 0', () => {
    const cell = { cell_type: 'numbers', max_length: 3, padding_side: 'left' }
    expect(formatPartValue('5', cell)).toBe('005')
  })

  it('does not pad when length matches max_length', () => {
    const cell = { cell_type: 'numbers', max_length: 3, padding_side: 'left', padding_char: '0' }
    expect(formatPartValue('123', cell)).toBe('123')
  })

  it('does not pad when length exceeds max_length', () => {
    const cell = { cell_type: 'numbers', max_length: 3, padding_side: 'left', padding_char: '0' }
    expect(formatPartValue('1234', cell)).toBe('1234')
  })

  it('returns value as-is for non-numbers cell_type', () => {
    const cell = { cell_type: 'letters', max_length: 3, padding_side: 'left', padding_char: '0' }
    expect(formatPartValue('A', cell)).toBe('A')
  })

  it('returns empty string for empty value', () => {
    const cell = { cell_type: 'numbers', max_length: 3, padding_side: 'left', padding_char: '0' }
    expect(formatPartValue('', cell)).toBe('')
  })
})

describe('initializeNumberParts', () => {
  it('creates array of empty strings matching cells length', () => {
    const format = { cells: [{}, {}, {}] }
    expect(initializeNumberParts(format)).toEqual(['', '', ''])
  })

  it('returns empty array for single cell', () => {
    const format = { cells: [{}] }
    expect(initializeNumberParts(format)).toEqual([''])
  })

  it('returns empty array when format is null', () => {
    expect(initializeNumberParts(null)).toEqual([])
  })

  it('returns empty array when format is undefined', () => {
    expect(initializeNumberParts(undefined)).toEqual([])
  })
})

describe('formatNumberForDisplay', () => {
  // Стандартный российский формат: буква, 3 цифры, 2 буквы, 2-3 цифры региона
  // ("В 746 КУ 964") - тот же состав ячеек, каким форма ручного ввода собирает номер.
  const RU_FORMAT = {
    format: { id: 1, name: 'Российский', is_default: true },
    cells: [
      { cell_type: 'letters', alphabet_type: 'cyrillic', max_length: 1 },
      { cell_type: 'numbers', max_length: 3 },
      { cell_type: 'letters', alphabet_type: 'cyrillic', max_length: 2 },
      { cell_type: 'numbers', max_length: 3, min_length: 2 },
    ],
  }

  it('раскладывает слитный номер по формату и собирает с пробелами', () => {
    expect(formatNumberForDisplay('В746КУ964', [RU_FORMAT])).toBe('В 746 КУ 964')
  })

  it('номер, уже собранный с пробелами, остаётся с пробелами', () => {
    expect(formatNumberForDisplay('В 746 КУ 964', [RU_FORMAT])).toBe('В 746 КУ 964')
  })

  it('приводит буквы к верхнему регистру, как и раскладка по ячейкам', () => {
    expect(formatNumberForDisplay('в746ку964', [RU_FORMAT])).toBe('В 746 КУ 964')
  })

  it('номер, не подошедший ни под один формат, возвращается без изменений', () => {
    expect(formatNumberForDisplay('ABCDEFGH123456', [RU_FORMAT])).toBe('ABCDEFGH123456')
  })

  it('без списка форматов возвращает номер как есть', () => {
    expect(formatNumberForDisplay('В746КУ964', [])).toBe('В746КУ964')
    expect(formatNumberForDisplay('В746КУ964', null)).toBe('В746КУ964')
  })

  it('пустой номер возвращается как есть', () => {
    expect(formatNumberForDisplay('', [RU_FORMAT])).toBe('')
    expect(formatNumberForDisplay(null, [RU_FORMAT])).toBe(null)
  })
})
