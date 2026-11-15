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

/**
 * Логин с собачкой - единый вид во всех списках и карточках. Собачка отличает
 * логин от фамилии в соседней колонке; в подборе согласующих такой вид был с
 * самого начала, остальные экраны приведены к нему (#1567).
 *
 * @param {?string} username
 * @returns {string} пустая строка, если логина нет
 */
export function formatLogin(username) {
  const login = trim(username)
  if (!login) return ''
  return login.startsWith('@') ? login : `@${login}`
}

/**
 * Подпись учётной записи: разрешённое сервером имя или нейтральная подпись.
 * Логин используется отдельно только там, где разрешено управление аккаунтом.
 *
 * @param {NameParts & {username?: string, display_label?: string}} user
 * @returns {string}
 */
export function formatUserLabel(user) {
  if (!user) return ''
  // Серверная подпись не подставляется в редактируемые части настоящего ФИО.
  if (user.pd_hidden) return 'Имя скрыто'
  const displayLabel = trim(user.display_label)
  if (displayLabel) return displayLabel
  return formatShortName(user) || 'Пользователь'
}

/** Full public identity; authentication logins are never a display fallback. */
export function formatPublicUserName(user) {
  if (!user) return 'Пользователь'
  if (user.pd_hidden) return 'Имя скрыто'
  const label = trim(user.display_label)
  if (label) return label
  return formatFullName(user) || 'Пользователь'
}
