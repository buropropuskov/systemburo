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

  it('на телефоне поиск журнала сворачивается в иконку 40x40, а не тянется на всю ширину (волна 8)', () => {
    expect(headMedia).toBeTruthy();
    expect(ruleFor(headMedia, '.proc__journal-search-icon')).toContain('width: 40px;');
    expect(ruleFor(headMedia, '.proc__journal-search-icon')).toContain('height: 40px;');
  });

  it('оверлей поиска журнала раскрывается через clip-path, не отдельным рядом', () => {
    const enterFrom = ruleFor(headMedia, '.proc-journal-search-enter-from');
    expect(enterFrom).toBe('clip-path: inset(0 0 0 100%);');
  });

  it('дропдаун роли журнала на телефоне - фиксированной ширины и первый в порядке ряда фильтров', () => {
    expect(ruleFor(headMedia, '.proc__journal-role-dropdown')).toContain('order: 1;');
    expect(ruleFor(headMedia, '.proc__journal-role-dropdown')).toContain('width: 165px;');
  });

  it('кнопка «Сбросить» в журнале той же высоты, что дата (35px), и в одном ряду с дропдауном роли', () => {
    const rule = ruleFor(headMedia, '.proc__journal-reset');
    expect(rule).toContain('height: 35px;');
    // order:2 ставит её сразу за дропдауном (order:1), до ряда даты (order:3) -
    // тот самый перенос "Сбросить в одну строку с выпадающей кнопкой" (волна 8).
    expect(rule).toContain('order: 2;');
    expect(rule).toContain('margin-left: auto;');
  });

  it('ряд даты журнала переносится на новую строку через flex-basis: 100%, а не полагается на порядок соседей', () => {
    const rule = ruleFor(headMedia, '.proc__journal-daterow');
    expect(rule).toContain('order: 3;');
    expect(rule).toContain('flex: 1 1 100%;');
  });

  // Замок на реальный баг стенда (волна 8): базовое display:contents стояло
  // ПОСЛЕ @media(max-width:768px) в файле - при равной специфичности более
  // позднее правило побеждает НЕЗАВИСИМО от того, было ли оно внутри media,
  // и мобильный display:flex молча перебивался обратно в contents на ЛЮБОЙ
  // ширине. Ряд даты и поиск-иконка вместо своей строки съезжали в общий ряд
  // с дропдауном роли.
  it('базовое .proc__journal-daterow{display:contents} объявлено РАНЬШЕ мобильного @media, а не позже', () => {
    const baseIdx = SFC.indexOf('.proc__journal-daterow {\n  display: contents;');
    const mediaIdx = SFC.indexOf('@media (max-width: 768px)');
    expect(baseIdx).toBeGreaterThan(-1);
    expect(mediaIdx).toBeGreaterThan(-1);
    expect(baseIdx).toBeLessThan(mediaIdx);
  });
});
