import { describe, it, expect } from 'vitest'
import { detailLabel } from '../authEventDetail'

describe('detailLabel', () => {
  it('переводит точные известные строки', () => {
    expect(detailLabel('wrong password')).toBe('Неверный пароль')
    expect(detailLabel('account disabled')).toBe('Аккаунт отключён')
    expect(detailLabel('user not found')).toBe('Пользователь не найден')
  })

  it('reuse токена скрывает family_id за понятной подписью', () => {
    expect(detailLabel('family_id=3f2c9a1e-1234-4a5b-8c9d-000000000001'))
      .toBe('Повторное использование токена сессии')
  })

  it('момент блокировки: вытаскивает число попыток', () => {
    expect(detailLabel('locked for 15m0s after 5 failed attempts'))
      .toBe('Заблокировано после 5 неудачных попыток')
    expect(detailLabel('locked for 1h0m0s after 1 failed attempt'))
      .toBe('Заблокировано после 1 неудачных попыток')
  })

  it('попытка входа в залоченную учётку: остаток в секундах', () => {
    expect(detailLabel('locked for 847s')).toBe('Заблокировано ещё на 847 сек.')
  })

  it('пустое остаётся пустым (UI подменит на «—»)', () => {
    expect(detailLabel('')).toBe('')
    expect(detailLabel(null)).toBe('')
    expect(detailLabel(undefined)).toBe('')
  })

  it('неизвестная строка проходит как есть', () => {
    expect(detailLabel('some new backend reason')).toBe('some new backend reason')
    // свободный текст с числом не должен ложно матчить locked-паттерны
    expect(detailLabel('user had 5 failed attempts elsewhere'))
      .toBe('user had 5 failed attempts elsewhere')
  })
})
