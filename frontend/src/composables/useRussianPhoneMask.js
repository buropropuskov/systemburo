/**
 * Маска для российского номера телефона: +7 (999) 999 99-99.
 *
 * Применение:
 * import { formatRussianPhone } from '@/composables/useRussianPhoneMask'
 *
 * <input :value="phone" @input="phone = formatRussianPhone($event.target.value)">
 *
 * Особенности:
 * - Ведущие 8/7/+7 приводятся к +7.
 * - Любые символы кроме цифр удаляются на входе.
 * - Маска накладывается по мере ввода - частичный ввод показывается частично.
 *   Например "89" -> "+7 (9", "891" -> "+7 (91", "89123456789" -> "+7 (912) 345 67-89".
 */
export function formatRussianPhone(raw) {
  if (raw == null) return ''
  let digits = String(raw).replace(/\D/g, '')
  // Ведущая 8 или 7 - заменяем на 7 (префикс страны один и тот же).
  if (digits.length && (digits[0] === '8' || digits[0] === '7')) {
    digits = '7' + digits.slice(1)
  } else if (digits.length) {
    // Если юзер начал с другой цифры (например, 9), считаем что забыл код страны.
    digits = '7' + digits
  }
  digits = digits.slice(0, 11) // +7 + 10 цифр = 11

  const d = digits.slice(1) // после страны
  let out = '+7'
  if (d.length > 0) out += ' (' + d.slice(0, 3)
  if (d.length >= 3) out += ')'
  if (d.length > 3) out += ' ' + d.slice(3, 6)
  if (d.length > 6) out += ' ' + d.slice(6, 8)
  if (d.length > 8) out += '-' + d.slice(8, 10)
  return out
}

/**
 * Вытаскивает только цифры в формате +7XXXXXXXXXX для отправки на backend.
 * Возвращает null если номер пустой.
 */
export function phoneToE164(masked) {
  if (!masked) return null
  const digits = String(masked).replace(/\D/g, '')
  if (!digits) return null
  return '+' + digits.replace(/^8/, '7')
}
