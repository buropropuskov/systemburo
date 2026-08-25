import { describe, it, expect, vi } from 'vitest'
import {
  DEFAULT_PASSWORD_POLICY,
  evaluatePassword,
  passwordMeetsPolicy,
  generatePassword,
} from '@/utils/passwordPolicy'

describe('evaluatePassword', () => {
  it('дефолт: буква+цифра+8 проходит', () => {
    expect(passwordMeetsPolicy(DEFAULT_PASSWORD_POLICY, 'password1')).toBe(true)
  })
  it('дефолт: без цифры не проходит', () => {
    expect(passwordMeetsPolicy(DEFAULT_PASSWORD_POLICY, 'passwordonly')).toBe(false)
  })
  it('дефолт: короткий не проходит', () => {
    expect(passwordMeetsPolicy(DEFAULT_PASSWORD_POLICY, 'pass1')).toBe(false)
  })
  it('выключенные требования не попадают в список', () => {
    const rules = evaluatePassword({ min_length: 4 }, 'ab')
    expect(rules).toHaveLength(1)
    expect(rules[0].key).toBe('min_length')
  })
  it('спецсимвол распознаётся', () => {
    const policy = { min_length: 4, require_special: true }
    expect(passwordMeetsPolicy(policy, 'ab!9')).toBe(true)
    expect(passwordMeetsPolicy(policy, 'abcd')).toBe(false)
  })
})

describe('generatePassword', () => {
  it('генерит пароль, проходящий любую политику', () => {
    const policy = { min_length: 12, require_letter: true, require_uppercase: true, require_lowercase: true, require_digit: true, require_special: true }
    for (let i = 0; i < 30; i++) {
      const pw = generatePassword(policy)
      expect(passwordMeetsPolicy(policy, pw)).toBe(true)
    }
  })
})

describe('generatePassword: источник случайности и наборы символов', () => {
  it('не использует Math.random', () => {
    // Замок на регрессию: пароль от кнопки «Генерировать» - учётные данные
    // человека, и предсказуемый источник равносилен отсутствию пароля.
    const spy = vi.spyOn(Math, 'random')
    generatePassword({ min_length: 16, require_letter: true, require_digit: true })
    expect(spy).not.toHaveBeenCalled()
    spy.mockRestore()
  })

  it('не выдаёт визуально неоднозначные символы', () => {
    for (let i = 0; i < 200; i += 1) {
      const pwd = generatePassword({
        min_length: 24,
        require_letter: true,
        require_uppercase: true,
        require_lowercase: true,
        require_digit: true,
        require_special: true,
      })
      expect(pwd).not.toMatch(/[0O1lI]/)
    }
  })

  it('держит нижний предел длины 12 даже при короткой политике', () => {
    expect(generatePassword({ min_length: 6 }).length).toBeGreaterThanOrEqual(12)
  })

  it('выдаёт разные пароли подряд', () => {
    const seen = new Set()
    for (let i = 0; i < 100; i += 1) {
      seen.add(generatePassword({ min_length: 12, require_letter: true, require_digit: true }))
    }
    expect(seen.size).toBe(100)
  })
})
