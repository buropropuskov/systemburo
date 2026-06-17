import DOMPurify from 'dompurify'

/**
 * Безопасная очистка HTML перед вставкой через v-html.
 * DOMPurify - whitelist-based санитизер, покрывает обходы которые regex-blacklist
 * пропускает (CDATA, HTML entities, mixed case, nested tags).
 *
 * Используется для admin-editable контента (инструкции в таблицах, текстовый
 * конструктор) - input не доверенный, хотя authored админом.
 *
 * Ресайз картинок (TextConstructor) хранит ширину в HTML-атрибуте `width` (число px) -
 * он входит в whitelist html-профиля DOMPurify и не вырезается, поэтому размер картинки
 * переживает санитизацию и round-trip. Контракт зафиксирован в utils/__tests__/sanitize.spec.js.
 *
 * @param {string} dirty - исходный HTML
 * @returns {string} - безопасный HTML
 */
export function sanitizeHtml(dirty) {
  if (!dirty) return ''
  return DOMPurify.sanitize(dirty, {
    USE_PROFILES: { html: true },
    FORBID_TAGS: ['style', 'script', 'iframe', 'form', 'input', 'button'],
    FORBID_ATTR: ['onerror', 'onload', 'onclick', 'onmouseover', 'onfocus']
  })
}
