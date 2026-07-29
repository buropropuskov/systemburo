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
 * Готовит сохранённый номер к показу: приводит к маске +7 (999) 999 99-99.
 * Номер, не похожий на российский (добавочный, несколько номеров через
 * запятую, короткий сервисный), возвращается как есть - маска исказила бы его.
 *
 * @param {string|null|undefined} value - Номер из базы в любом виде
 * @returns {string}
 */
export function formatRussianPhoneForDisplay(value) {
  if (!value) return ''
  const digits = String(value).replace(/\D/g, '')
  const isRussian = isValidRussianPhone(digits) || isValidRussianPhone(`7${digits}`)
  return isRussian ? formatRussianPhone(digits) : String(value)
}

/**
 * Убирает ближайшую цифру от каретки. Нужно при стирании разделителя маски:
 * сам разделитель браузер уже удалил, но набор цифр не изменился, маска вернёт
 * прежнюю строку - и Backspace выглядит нажатым впустую.
 *
 * @param {string} value - Значение поля после нативного удаления
 * @param {number} caret - Позиция каретки в нём
 * @param {boolean} [forward] - Delete вместо Backspace: искать цифру справа
 * @returns {{value: string, caret: number}}
 */
export function dropAdjacentDigit(value, caret, forward = false) {
  const source = String(value ?? '')
  const at = Math.max(0, Math.min(caret, source.length))
  const isDigit = (ch) => ch >= '0' && ch <= '9'

  if (forward) {
    for (let i = at; i < source.length; i++) {
      if (isDigit(source[i])) return { value: source.slice(0, i) + source.slice(i + 1), caret: i }
    }
    return { value: source, caret: at }
  }
  for (let i = at - 1; i >= 0; i--) {
    if (isDigit(source[i])) return { value: source.slice(0, i) + source.slice(i + 1), caret: i }
  }
  return { value: source, caret: at }
}

/**
 * Накладывает маску на поле ввода: обновляет показ и держит каретку у той же
 * цифры. Поля телефона сидят на one-way `:value`, поэтому синхронизировать
 * элемент приходится руками - иначе курсор улетает в конец при правке середины.
 *
 * @param {HTMLInputElement} input - Поле телефона
 * @param {InputEvent} event - Событие input (нужен inputType для стирания)
 * @param {string} previous - Значение поля до этого ввода
 * @returns {string} Замаскированное значение для модели
 */
export function applyPhoneMask(input, event, previous) {
  let value = input.value
  let caret = input.selectionStart ?? value.length

  // Стёрли разделитель маски: набор цифр не изменился, маска вернёт ту же
  // строку - убираем соседнюю цифру, иначе Backspace приходится жать дважды.
  const digits = (str) => String(str ?? '').replace(/\D/g, '')
  if (event?.inputType?.startsWith('delete') && digits(value) === digits(previous)) {
    ({ value, caret } = dropAdjacentDigit(value, caret, event.inputType === 'deleteContentForward'))
  }

  const masked = formatRussianPhone(value)
  const position = caretAfterMask(value, caret, masked)
  input.value = masked
  input.setSelectionRange(position, position)
  return masked
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
