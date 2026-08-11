import { describe, it, expect } from 'vitest';
import fs from 'node:fs';
import path from 'node:path';

/**
 * Замок на карточку строки в таблицах проходной (#1097 S9).
 *
 * Карточка собрана каскадом из двух слоёв - глобального responsive-tables.css и
 * scoped-блока компонента, - и ломается тихо: jsdom каскад и медиазапросы не считает,
 * поведенческий тест такого не видит. Стерегём поэтому сам контракт в исходнике.
 *
 * Что уже стреляло и чего эти проверки не дают повторить:
 * - доли столбцов заданы через `flex: N 0 0`, то есть с базисом 0. В строке базис и есть
 *   ширина, `width: 100%` при нём игнорируется - без `flex: 0 0 100%` все поля сбиваются
 *   обратно в одну строку по табличным долям;
 * - подпись рисуется `::before` у ячейки, и рядом с кнопкой прохода она дублировала саму
 *   кнопку («Въезд» подписью и «Въезд» кнопкой в одной строке);
 * - пунктир снизу у последнего поля висит оторванной чертой: последней в строке идёт
 *   колонка действий без data-label, и `[data-label]:last-child` до неё не достаёт.
 */

const TABLES = [
  { file: 'CarsTable.vue', card: '.selected-table-card', row: '.selected-table-card .item-data.rt-row', head: '.items-header' },
  { file: 'PeopleTable.vue', card: '.selected-table-card', row: '.selected-table-card .item-data.rt-row', head: '.items-header' },
  { file: 'FactTable.vue', card: '.fact-table-card', row: '.fact-table-card .fact-row.rt-row', head: '.fact-header' },
];

const MOBILE = '(max-width: 767.98px)';
const TABLET_UP = '(min-width: 768px)';
const SRC = path.resolve(__dirname, '../..');

const source = (file) =>
  fs.readFileSync(path.join(SRC, 'components', file), 'utf8')
    .replace(/\/\*[\s\S]*?\*\//g, '');

/** Тела всех блоков `@media <query>` с точно таким условием. */
function mediaBlocks(src, query) {
  const blocks = [];
  const re = /@media([^{]+)\{/g;
  let match;
  while ((match = re.exec(src)) !== null) {
    if (match[1].trim() !== query) continue;
    let depth = 0;
    for (let i = match.index + match[0].length - 1; i < src.length; i += 1) {
      if (src[i] === '{') depth += 1;
      else if (src[i] === '}') {
        depth -= 1;
        if (depth === 0) {
          blocks.push(src.slice(match.index + match[0].length, i));
          break;
        }
      }
    }
  }
  return blocks;
}

/** Объявления всех правил с таким селектором (в том числе в групповых). */
function declarationsFor(css, selector) {
  const out = [];
  const re = /([^{}]+)\{([^{}]*)\}/g;
  let match;
  while ((match = re.exec(css)) !== null) {
    const selectors = match[1].split(',').map((s) => s.trim());
    if (selectors.includes(selector)) out.push(match[2]);
  }
  return out;
}

/** Селекторы всех правил, где встречается такой фрагмент объявления. */
function selectorsWith(css, declaration) {
  const out = [];
  const re = /([^{}]+)\{([^{}]*)\}/g;
  let match;
  while ((match = re.exec(css)) !== null) {
    if (!match[2].includes(declaration)) continue;
    out.push(...match[1].split(',').map((s) => s.trim()));
  }
  return out;
}

describe.each(TABLES)('Карточка строки: $file', ({ file, card, row, head }) => {
  const src = source(file);
  const mobile = mediaBlocks(src, MOBILE).join('\n');
  const cellRule = `${card} .rt-row > [data-label]`;

  it('карточка - строка с переносом, иначе кнопки прохода не встанут рядом', () => {
    const decls = declarationsFor(mobile, row).join('\n');
    expect(decls).toMatch(/flex-direction:\s*row\s*!important/);
    expect(decls).toMatch(/flex-wrap:\s*wrap\s*!important/);

    // Только на мобилке: на десктопе строка таблицы обязана остаться одной строкой.
    expect(selectorsWith(src, 'flex-wrap: wrap !important')).toEqual(
      selectorsWith(mobile, 'flex-wrap: wrap !important'),
    );
  });

  it('строку держит базис, и правило целит во все ячейки, а не только в подписанные', () => {
    // `width: 100%` при нулевом базисе не считается вовсе - ячейки делят одну строку,
    // и кнопка прохода вылезает из своей 14-пиксельной ячейки поверх соседей.
    const decls = declarationsFor(mobile, `${card} .rt-row > *`).join('\n');
    expect(decls).toMatch(/flex:\s*0\s+0\s+100%\s*!important/);

    // Ячейка без подписи (действия, «Подробнее») обязана попадать под то же правило.
    expect(declarationsFor(mobile, cellRule).join('\n'))
      .not.toMatch(/flex:\s*0\s+0\s+100%/);
  });

  it('значения выровнены влево', () => {
    const decls = declarationsFor(mobile, cellRule).join('\n');
    expect(decls).toMatch(/justify-content:\s*flex-start\s*!important/);
    expect(decls).toMatch(/text-align:\s*left\s*!important/);
  });

  it('подписи полей скрыты, кроме перечисленных исключений', () => {
    const hidden = declarationsFor(mobile, `${cellRule}::before`).join('\n');
    expect(hidden).toMatch(/display:\s*none\s*!important/);

    const shown = selectorsWith(mobile, 'display: block !important')
      .filter((s) => s.endsWith('::before'));
    expect(shown.length).toBeGreaterThan(0);
    shown.forEach((s) => expect(s.startsWith(`${card} .rt-row > .`)).toBe(true));
  });

  it('ячейки прохода делят строку и не несут пунктира', () => {
    for (const col of ['.entry-col', '.exit-col']) {
      const decls = declarationsFor(mobile, `${card} .rt-row > ${col}`).join('\n');
      expect(decls).toMatch(/flex:\s*1\s+1\s+0\s*!important/);
      expect(decls).toMatch(/border-top:\s*none\s*!important/);
    }
  });

  it('разделитель рисуется сверху у полей 2..N, снизу его нет', () => {
    expect(declarationsFor(mobile, cellRule).join('\n'))
      .toMatch(/border-bottom:\s*none\s*!important/);
    expect(declarationsFor(mobile, `${cellRule} ~ [data-label]`).join('\n'))
      .toMatch(/border-top:\s*1px dashed/);
  });

  it('из «своя строка каждому» выходят только ячейки прохода', () => {
    const exceptions = selectorsWith(mobile, 'flex: 1 1 0 !important');
    expect(exceptions.sort()).toEqual([
      `${card} .rt-row > .entry-col`,
      `${card} .rt-row > .exit-col`,
    ]);
  });

  it('второе исключение - только служебные колонки в хвосте карточки', () => {
    const byContent = selectorsWith(mobile, 'flex: 0 0 auto !important');
    byContent.forEach((s) => expect(s).toMatch(/\.(expand|actions)-col$/));
  });

  it('кнопка прохода занимает свою половину строки', () => {
    const decls = declarationsFor(
      mobile,
      `${card} .rt-row > .entry-col .action-btn`,
    ).join('\n');
    expect(decls).toMatch(/width:\s*100%/);
  });

  it('обёртка полосы заголовков убрана из потока, а не только её внутренний ряд', () => {
    // rt-head-row прячет .header-row, но обёртка остаётся и рисует свой border-bottom
    // отдельной линией в 1px перед первой карточкой (эталон, §8).
    expect(declarationsFor(mobile, `${card} ${head}`).join('\n'))
      .toMatch(/display:\s*none/);

    // Закрепление полосы живёт в своём медиазапросе и обязано уцелеть.
    expect(declarationsFor(mediaBlocks(src, TABLET_UP).join('\n'), head).join('\n'))
      .toMatch(/position:\s*sticky/);
  });
});

/** Кнопка «Подробнее» есть только там, где столбцы прячутся по приоритету. */
const WITH_CHEVRON = TABLES.filter(({ file }) => file !== 'FactTable.vue');

describe.each(WITH_CHEVRON)('Кнопка «Подробнее» в карточке: $file', ({ file, card }) => {
  const src = source(file);
  const mobile = mediaBlocks(src, MOBILE).join('\n');

  it('«Подробнее» и действия уходят в конец карточки, в этом порядке', () => {
    const order = (col) => {
      const decls = declarationsFor(mobile, `${card} .rt-row > ${col}`).join('\n');
      const match = decls.match(/order:\s*(\d+)\s*!important/);
      return match ? Number(match[1]) : null;
    };

    const expand = order('.expand-col');
    const actions = order('.actions-col');
    expect(expand).not.toBeNull();
    expect(actions).not.toBeNull();
    // Разметочные order служебных столбцов (9997-9999) соседствуют с order статуса,
    // и шеврон оказывался посреди карточки - между именем и бейджем.
    expect(expand).toBeGreaterThan(9999);
    expect(actions).toBeGreaterThan(expand);
  });

  it('тач-таргет шеврона не мёртвый по каскаду', () => {
    // Базовое `.expand-btn { width: 22px }` стоит НИЖЕ мобильного медиазапроса, и при
    // равной специфичности побеждает оно: голый `.expand-btn` внутри @media бесполезен.
    const selectors = selectorsWith(mobile, 'height: 44px');
    expect(selectors).toContain(`${card} .expand-btn`);
    expect(selectors).not.toContain('.expand-btn');
  });
});
