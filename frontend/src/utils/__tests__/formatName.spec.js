import { describe, it, expect } from 'vitest'
import { formatShortName, formatFullName } from '../formatName'

describe('formatShortName', () => {
  it('полное ФИО -> Фамилия И.О.', () => {
    expect(formatShortName({ last_name: 'Иванов', first_name: 'Иван', middle_name: 'Иванович' }))
      .toBe('Иванов И.И.')
  })

  it('фамилия и имя -> Фамилия И.', () => {
    expect(formatShortName({ last_name: 'Иванов', first_name: 'Иван' })).toBe('Иванов И.')
  })

  it('только фамилия -> Фамилия', () => {
    expect(formatShortName({ last_name: 'Иванов' })).toBe('Иванов')
  })

  it('только имя -> Имя (не сокращаем)', () => {
    expect(formatShortName({ first_name: 'Иван' })).toBe('Иван')
  })

  it('имя и отчество без фамилии -> Имя Отчество (полностью)', () => {
    expect(formatShortName({ first_name: 'Иван', middle_name: 'Иванович' }))
      .toBe('Иван Иванович')
  })

  it('пустые поля -> ""', () => {
    expect(formatShortName({})).toBe('')
    expect(formatShortName(null)).toBe('')
    expect(formatShortName(undefined)).toBe('')
  })

  it('игнорирует пробелы по краям', () => {
    expect(formatShortName({ last_name: '  Иванов ', first_name: ' Иван' }))
      .toBe('Иванов И.')
  })

  it('первая буква в верхнем регистре', () => {
    expect(formatShortName({ last_name: 'иванов', first_name: 'иван' }))
      .toBe('иванов И.')
  })
})

describe('formatFullName', () => {
  it('собирает три части в одну строку', () => {
    expect(formatFullName({ last_name: 'Иванов', first_name: 'Иван', middle_name: 'Иванович' }))
      .toBe('Иванов Иван Иванович')
  })

  it('пропускает пустые части', () => {
    expect(formatFullName({ last_name: 'Иванов', middle_name: 'Иванович' }))
      .toBe('Иванов Иванович')
  })

  it('пустой объект -> ""', () => {
    expect(formatFullName({})).toBe('')
  })
})
