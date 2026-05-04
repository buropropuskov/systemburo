/**
 * Утилиты форматирования ФИО для всех таблиц, списков и карточек.
 *
 * formatShortName({ last_name, first_name, middle_name }):
 *   "Иванов Иван Иванович" -> "Иванов И.И."
 *   "Иванов Иван"          -> "Иванов И."
 *   "Иванов"               -> "Иванов"
 *   "Иван"                 -> "Иван"     (нет фамилии - не сокращаем имя)
 *   "Иван Иванович"        -> "Иван Иванович"  (нет фамилии - и имя и отчество полностью)
 *
 * formatFullName({ last_name, first_name, middle_name }):
 *   "Иванов Иван Иванович"
 */

/**
 * @typedef {Object} NameParts
 * @property {?string} [last_name]
 * @property {?string} [first_name]
 * @property {?string} [middle_name]
 */

const trim = (v) => (v == null ? '' : String(v).trim())

/**
 * @param {NameParts} parts
 * @returns {string}
 */
export function formatShortName(parts) {
  if (!parts) return ''
  const last = trim(parts.last_name)
  const first = trim(parts.first_name)
  const middle = trim(parts.middle_name)

  if (!last) {
    return [first, middle].filter(Boolean).join(' ')
  }

  let out = last
  if (first) out += ` ${first[0].toUpperCase()}.`
  if (middle) out += `${middle[0].toUpperCase()}.`
  return out
}

/**
 * @param {NameParts} parts
 * @returns {string}
 */
export function formatFullName(parts) {
  if (!parts) return ''
  return [trim(parts.last_name), trim(parts.first_name), trim(parts.middle_name)]
    .filter(Boolean)
    .join(' ')
}
