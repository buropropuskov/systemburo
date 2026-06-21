// Набор спецсимволов синхронизирован с бэком (passwordSpecialChars в
// internal/services/password_policy.go). Классы символов считаются по ASCII.
const SPECIAL_CHARS = "!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~"

/**
 * @typedef {Object} PasswordPolicy
 * @property {number} min_length
 * @property {boolean} [require_letter]
 * @property {boolean} [require_uppercase]
 * @property {boolean} [require_lowercase]
 * @property {boolean} [require_digit]
 * @property {boolean} [require_special]
 */

/** @type {PasswordPolicy} Совпадает с models.DefaultPasswordPolicy на бэке. */
export const DEFAULT_PASSWORD_POLICY = {
  min_length: 8,
  require_letter: true,
  require_uppercase: false,
  require_lowercase: false,
  require_digit: true,
  require_special: false,
}

/**
 * @param {PasswordPolicy} policy
 * @param {string} password
 * @returns {Array<{key: string, label: string, ok: boolean}>}
 */
export function evaluatePassword(policy, password) {
  const chars = [...(password || '')]
  const hasUpper = chars.some((c) => c >= 'A' && c <= 'Z')
  const hasLower = chars.some((c) => c >= 'a' && c <= 'z')
  const hasDigit = chars.some((c) => c >= '0' && c <= '9')
  const hasLetter = hasUpper || hasLower
  const hasSpecial = chars.some((c) => SPECIAL_CHARS.includes(c))

  const rules = [
    { key: 'min_length', label: `Минимум ${policy.min_length} символов`, ok: chars.length >= policy.min_length },
  ]
  if (policy.require_letter) rules.push({ key: 'letter', label: 'Хотя бы одна буква', ok: hasLetter })
  if (policy.require_uppercase) rules.push({ key: 'uppercase', label: 'Хотя бы одна заглавная буква', ok: hasUpper })
  if (policy.require_lowercase) rules.push({ key: 'lowercase', label: 'Хотя бы одна строчная буква', ok: hasLower })
  if (policy.require_digit) rules.push({ key: 'digit', label: 'Хотя бы одна цифра', ok: hasDigit })
  if (policy.require_special) rules.push({ key: 'special', label: 'Хотя бы один спецсимвол', ok: hasSpecial })
  return rules
}

/**
 * @param {PasswordPolicy} policy
 * @param {string} password
 * @returns {boolean}
 */
export function passwordMeetsPolicy(policy, password) {
  return evaluatePassword(policy, password).every((r) => r.ok)
}

/**
 * Генерит пароль, гарантированно проходящий политику.
 * @param {PasswordPolicy} policy
 * @returns {string}
 */
export function generatePassword(policy) {
  const upper = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ'
  const lower = 'abcdefghijklmnopqrstuvwxyz'
  const digits = '0123456789'
  const special = '!#$%&?'
  const pick = (set) => set[Math.floor(Math.random() * set.length)]

  const out = []
  if (policy.require_uppercase) out.push(pick(upper))
  if (policy.require_lowercase) out.push(pick(lower))
  if (policy.require_letter && !policy.require_uppercase && !policy.require_lowercase) out.push(pick(lower + upper))
  if (policy.require_digit) out.push(pick(digits))
  if (policy.require_special) out.push(pick(special))

  const pool = lower + upper + digits + (policy.require_special ? special : '')
  const target = Math.max(policy.min_length || 0, out.length, 8)
  while (out.length < target) out.push(pick(pool))

  // перемешиваем, чтобы обязательные символы не оказались в начале
  for (let i = out.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1))
    ;[out[i], out[j]] = [out[j], out[i]]
  }
  return out.join('')
}
