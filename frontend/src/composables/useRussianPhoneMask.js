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
  if (!digits) return ''
  // Ведущая 8 или 7 - заменяем на 7 (префикс страны один и тот же).
  if (digits[0] === '8' || digits[0] === '7') {
    digits = '7' + digits.slice(1)
  } else {
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

/**
 * Полный ли российский номер: 11 цифр, код страны 7 или 8, код региона/оператора 3-9.
 *
 * @param {string} value - Номер в любом виде (маска, сырые цифры)
 * @returns {boolean}
 */
export function isValidRussianPhone(value) {
  const digits = String(value ?? '').replace(/\D/g, '')
  if (digits.length !== 11) return false
  if (digits[0] !== '7' && digits[0] !== '8') return false
  return digits[1] >= '3' && digits[1] <= '9'
}

/**
 * Позиция каретки в замаскированном значении: держим её у той же по счёту цифры,
 * что и до наложения маски. Без этого правка в середине номера выбрасывает
 * курсор в конец поля.
 *
 * @param {string} value - Значение поля до маски (то, что набрал пользователь)
 * @param {number} caret - Позиция каретки в этом значении
 * @param {string} masked - Результат formatRussianPhone(value)
 * @returns {number}
 */
export function caretAfterMask(value, caret, masked) {
  const source = String(value ?? '')
  const digits = source.replace(/\D/g, '')
  const before = source.slice(0, Math.max(0, caret))
  let digitsLeft = (before.match(/\d/g) || []).length
  // Маска дописывает код страны, когда набор начали с национальной цифры -
  // все цифры пользователя съезжают на одну позицию вправо.
  if (digits && digits[0] !== '7' && digits[0] !== '8') digitsLeft += 1
  if (digitsLeft === 0) return 0

  let seen = 0
  for (let i = 0; i < masked.length; i++) {
    if (masked[i] < '0' || masked[i] > '9') continue
    seen += 1
    if (seen < digitsLeft) continue
    // Если дальше идут только скобки и дефисы - забираем их с собой, иначе
    // каретка залипает перед ")" и следующая цифра выглядит вставленной не туда.
    const tail = masked.slice(i + 1)
    return /\d/.test(tail) ? i + 1 : masked.length
  }
  return masked.length
}
