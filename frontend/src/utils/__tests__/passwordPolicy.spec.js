import { describe, it, expect } from 'vitest'
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
