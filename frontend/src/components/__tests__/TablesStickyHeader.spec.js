import { describe, it, expect } from 'vitest';
import fs from 'node:fs';
import path from 'node:path';

/**
 * Замок на закреплённую полосу заголовков столбцов в таблицах проходной
 * (#1097 S8 волна 4).
 *
 * Прилипание ломается тихо и не ловится ни одним поведенческим тестом: вернули
 * предку `overflow: hidden` - он снова стал скроллпортом, и sticky замер на
 * месте; убрали фон - строки поехали сквозь полосу; уронили правило ниже 768 -
 * полоса всплыла над карточками на мобилке, где заголовков нет вовсе. Проверяем
 * поэтому сам контракт в исходнике.
 */

const TABLES = [
  { file: 'CarsTable.vue', card: '.selected-table-card', head: '.items-header' },
  { file: 'PeopleTable.vue', card: '.selected-table-card', head: '.items-header' },
  { file: 'FactTable.vue', card: '.fact-table-card', head: '.fact-header' },
];

const TABLET_UP = '(min-width: 768px)';
const TABLET_EXACT = '(min-width: 768px) and (max-width: 768px)';

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

describe.each(TABLES)('Закреплённая шапка столбцов: $file', ({ file, card, head }) => {
  const src = source(file);
  const tabletUp = mediaBlocks(src, TABLET_UP).join('\n');

  it('шапка прилипает, и только от 768 - ниже её нет, там строки-карточки', () => {
    const sticky = declarationsFor(tabletUp, head).filter((d) => /position:\s*sticky/.test(d));
    expect(sticky).toHaveLength(1);

    const stickyInFile = src.match(/position:\s*sticky/g) || [];
    const stickyInTabletUp = tabletUp.match(/position:\s*sticky/g) || [];
    expect(stickyInFile).toHaveLength(stickyInTabletUp.length);
  });

  it('фон полосы непрозрачный и перекрывает уходящие под неё строки', () => {
    const [sticky] = declarationsFor(tabletUp, head).filter((d) => /position:\s*sticky/.test(d));
    expect(sticky).toMatch(/background:\s*var\(--surface\)/);
    expect(sticky).toMatch(/z-index:\s*3/);
  });

  it('карточку режет clip, а не hidden - иначе предок-скроллпорт держит sticky', () => {
    const cardDecls = declarationsFor(tabletUp, card);
    expect(cardDecls.some((d) => /overflow:\s*clip/.test(d))).toBe(true);
    expect(cardDecls.some((d) => /overflow:\s*hidden/.test(d))).toBe(false);
  });

  it('на 768 полоса встаёт под шапку приложения - там она ещё закреплена', () => {
    const exact = mediaBlocks(src, TABLET_EXACT).join('\n');
    const decls = declarationsFor(exact, head);
    expect(decls.join('\n')).toMatch(/top:\s*var\(--mobile-header-height\)/);
  });
});

it('--surface непрозрачен во всех палитрах - под полосой не должно просвечивать', () => {
  const tokens = fs.readFileSync(path.join(SRC, 'assets/tokens.css'), 'utf8');
  const values = [...tokens.matchAll(/^\s*--surface:\s*([^;]+);/gm)].map((m) => m[1].trim());

  expect(values.length).toBeGreaterThanOrEqual(2);
  values.forEach((value) => expect(value).toMatch(/^#[0-9a-f]{6}$/i));
});
