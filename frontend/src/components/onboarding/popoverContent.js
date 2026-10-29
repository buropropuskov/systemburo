import { sanitizeHtml } from '@/utils/sanitize.js';
import { getDemo } from './onboardingDemo';

export function showsDemo(step) { return Boolean(step?.demo) && !step?.element; }

/** Dynamic values and static demo assets use the same sanitized content on all layouts. */
export function fillDynamic(text, dynamic) {
  if (!text || !dynamic) return text;
  return Object.entries(dynamic).reduce((acc, [key, selector]) => {
    const value = document.querySelector(selector)?.textContent?.trim();
    return acc.replaceAll(`{${key}}`, value || 'выбранный');
  }, text);
}

export function buildPopoverHtml(step) {
  let html = fillDynamic(step.description || '', step.dynamic);
  const demo = showsDemo(step) ? getDemo(step.demo) : null;
  if (demo) {
    const caption = demo.caption
      ? `<figcaption class="ob-popover__demo-caption">${demo.caption}</figcaption>` : '';
    html += `<figure class="ob-popover__demo"><img class="ob-popover__demo-img" src="${demo.src}" width="${demo.width}" height="${demo.height}" alt="${demo.alt}" />${caption}</figure>`;
  }
  return sanitizeHtml(html);
}
