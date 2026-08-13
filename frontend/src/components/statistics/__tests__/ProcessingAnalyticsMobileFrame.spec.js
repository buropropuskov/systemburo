import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, it, expect } from 'vitest';

// jsdom не считает медиа-запросы и каскад - CSS-контракт мобильной раскладки
// стережём чтением сырого SFC, как в ApplicationAttachmentSupplementMarks.spec.js.
// Комментарии перед селектором вырезаем: без этого они склеиваются с первым
// селектором списка в один "селектор" при разборе по запятой и ломают матч.
const SFC = readFileSync(resolve(__dirname, '../ProcessingAnalytics.vue'), 'utf8')
  .replace(/\/\*[\s\S]*?\*\//g, '');

/**
 * Вырезать содержимое @media-блока с учётом вложенных {} (внутри лежат обычные
 * правила, наивный [^{}]* их не переживёт).
 */
function mediaBlock(src, query) {
  const start = src.indexOf(query);
  if (start === -1) return null;
  const openIdx = src.indexOf('{', start);
  let depth = 0;
  for (let i = openIdx; i < src.length; i += 1) {
    if (src[i] === '{') depth += 1;
    else if (src[i] === '}') {
      depth -= 1;
      if (depth === 0) return src.slice(openIdx + 1, i);
    }
  }
  return null;
}

/** Тело правила для селектора, входящего в список через запятую перед {}. */
function ruleFor(block, selector) {
  const regex = /([^{}]+)\{([^}]*)\}/g;
  let m;
  while ((m = regex.exec(block))) {
    const selectors = m[1].split(',').map((s) => s.replace(/\s+/g, ' ').trim());
    if (selectors.includes(selector)) return m[2].replace(/\s+/g, ' ').trim();
  }
  return null;
}

describe('ProcessingAnalytics — мобильный каркас карточек (#1097 w7)', () => {
  const cardMedia = mediaBlock(SFC, '@media (max-width: 767.98px)');
  const headMedia = mediaBlock(SFC, '@media (max-width: 768px)');

  it('снимает рамку/паддинг у обёртки только когда есть карточки-строки (:has(.rt-row))', () => {
    expect(cardMedia).toBeTruthy();
    const ratingsRule = ruleFor(cardMedia, '.proc__card--table:has(.rt-row)');
    const journalRule = ruleFor(cardMedia, '.proc__journal:has(.rt-row)');
    expect(ratingsRule).toBeTruthy();
    expect(journalRule).toBeTruthy();
    expect(ratingsRule).toContain('border: none');
    expect(ratingsRule).toContain('padding: 0');
    expect(journalRule).toContain('border: none');
  });

  it('не снимает рамку у голого .proc__card--table/.proc__journal без гварда', () => {
    // Без :has() правило схлопнуло бы рамку и у пустой выборки (текст без единого
    // признака таблицы) - гварда для базового селектора существовать не должно.
    expect(ruleFor(cardMedia, '.proc__card--table')).toBeNull();
    expect(ruleFor(cardMedia, '.proc__journal')).toBeNull();
  });

  it('поиск в журнале растягивается на всю ширину ряда фильтров на телефоне', () => {
    expect(headMedia).toBeTruthy();
    expect(ruleFor(headMedia, '.proc__journal-search')).toBe('width: 100%;');
  });

  it('кнопка «Сбросить» в журнале той же высоты, что поиск и диапазон дат (35px)', () => {
    expect(ruleFor(headMedia, '.proc__journal-reset')).toBe('height: 35px;');
  });
});
