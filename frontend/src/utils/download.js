/**
 * Извлекает имя файла из заголовка Content-Disposition: сначала RFC 5987
 * filename* (UTF-8 percent-encoded, для кириллицы), затем базовый filename,
 * иначе fallback. Порядок важен - filename* приоритетнее ASCII-фолбэка.
 * @param {string} cd  значение заголовка Content-Disposition (может быть пустым)
 * @param {string} fallback  имя по умолчанию, если заголовок пуст/без имени
 * @returns {string}
 */
export function parseContentDispositionFilename(cd, fallback) {
  const header = cd || '';
  const utf8Match = header.match(/filename\*=UTF-8''(.+)/i);
  if (utf8Match) return decodeURIComponent(utf8Match[1]);
  const basicMatch = header.match(/filename="?([^";]+)"?/);
  return basicMatch ? basicMatch[1] : fallback;
}
