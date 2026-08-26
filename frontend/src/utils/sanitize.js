import DOMPurify from 'dompurify'

/**
 * Безопасная очистка HTML перед вставкой через v-html.
 * DOMPurify - whitelist-based санитизер, покрывает обходы которые regex-blacklist
 * пропускает (CDATA, HTML entities, mixed case, nested tags).
 *
 * Используется для admin-editable контента (инструкции в таблицах, текстовый
 * конструктор) - input не доверенный, хотя authored админом.
 *
 * Ресайз картинок (TextConstructor) хранит размер в HTML-атрибутах `width`/`height` (число px) -
 * оба входят в whitelist html-профиля DOMPurify и не вырезаются, поэтому размер картинки
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

/**
 * Извлечь ПЛОСКИЙ текст из HTML - для однострочного превью с обрезкой "..".
 * Сообщение заявки хранится как rich-HTML из TextConstructor (`<h1><strong>..`);
 * в компактной карточке показываем только текст без тегов. DOMParser('text/html')
 * инертен (не исполняет скрипты и не грузит ресурсы, в отличие от innerHTML),
 * textContent декодирует HTML-сущности. Пробелы/переносы схлопываются в один пробел,
 * чтобы многострочный текст лёг в одну строку под ellipsis.
 *
 * @param {string} html - исходный HTML
 * @returns {string} - плоский текст
 */
export function stripHtml(html) {
  if (!html) return ''
  const doc = new DOMParser().parseFromString(html, 'text/html')
  return (doc.body.textContent || '').replace(/\s+/g, ' ').trim()
}
