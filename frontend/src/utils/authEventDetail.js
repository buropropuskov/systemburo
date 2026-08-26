// Технические подписи detail auth-событий бэк пишет по-английски (152-ФЗ аудит
// не трогаем) - переводим известные строки в человекочитаемые русские. Неизвестное
// проходит как есть, пустое остаётся пустым (в UI подменяется на «—»).
const DETAIL_EXACT = {
  'wrong password': 'Неверный пароль',
  'account disabled': 'Аккаунт отключён',
  'user not found': 'Пользователь не найден',
}

/**
 * Переводит строку detail auth-события в русскую подпись.
 * @param {string} detail сырое значение detail от бэка
 * @returns {string} человекочитаемая подпись; '' для пустого входа
 */
export function detailLabel(detail) {
  if (!detail) return ''
  const d = String(detail).trim()
  if (DETAIL_EXACT[d]) return DETAIL_EXACT[d]
  if (d.startsWith('family_id=')) return 'Повторное использование токена сессии'
  // "locked for 15m0s after 5 failed attempts" - момент блокировки учётки.
  let m = d.match(/^locked for .+ after (\d+) failed attempts?$/)
  if (m) return `Заблокировано после ${m[1]} неудачных попыток`
  // "locked for 847s" - попытка входа в уже заблокированную учётку.
  m = d.match(/^locked for (\d+)s$/)
  if (m) return `Заблокировано ещё на ${m[1]} сек.`
  return d
}
