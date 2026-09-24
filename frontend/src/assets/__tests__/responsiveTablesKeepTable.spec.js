import { describe, it, expect } from 'vitest';
import fs from 'node:fs';
import path from 'node:path';

/**
 * Карточные правила живут в двух блоках с разными потолками: список без пометки
 * становится карточками до 899.98, список с `rt-keep-table` - только до 767.98,
 * потому что на планшете он помещается таблицей. Стили у них обязаны быть
 * одинаковыми: разведены диапазоны, а не оформление.
 *
 * Разойтись они могут молча - правку допишут в один блок, и на телефоне половина
 * списков останется с прежними отступами. Поэтому сверяем тела правил, а не верим
 * на слово.
 */

const SRC = path.resolve(__dirname, '..');
const CSS = fs.readFileSync(path.join(SRC, 'responsive-tables.css'), 'utf8');

/** Тело блока `@media <query> { ... }` со сбалансированным подсчётом скобок. */
function mediaBody(query) {
  const start = CSS.indexOf(`@media ${query} {`);
  expect(start, `нет блока @media ${query}`).toBeGreaterThan(-1);
  let i = CSS.indexOf('{', start) + 1;
  let depth = 1;
  const from = i;
  while (depth > 0 && i < CSS.length) {
    if (CSS[i] === '{') depth += 1;
    else if (CSS[i] === '}') depth -= 1;
    i += 1;
  }
  return CSS.slice(from, i - 1);
}

/**
 * Правила блока как карта «селектор без пометки -> тело», с выброшенными
 * комментариями и пробелами. Пометка снимается, чтобы две копии сравнивались
 * по существу: `:not(.rt-keep-table)` против `.rt-keep-table` - это и есть
 * единственная разрешённая разница.
 */
function rules(body) {
  const clean = body.replace(/\/\*[\s\S]*?\*\//g, '');
  const out = new Map();
  const re = /([^{}]+)\{([^{}]*)\}/g;
  let m;
  while ((m = re.exec(clean)) !== null) {
    const selector = m[1].trim()
      .replace(/:not\(\.rt-keep-table\)/g, '')
      .replace(/\.rt-keep-table/g, '')
      .replace(/\s+/g, ' ');
    if (!selector.startsWith('.rt-table')) continue;
    out.set(selector, m[2].replace(/\s+/g, ' ').trim());
  }
  return out;
}

const tablet = rules(mediaBody('(max-width: 899.98px)'));
const phone = rules(mediaBody('(max-width: 767.98px)'));

describe('карточный слой: две копии правил не расходятся', () => {
  it('в телефонном блоке столько же правил, сколько в планшетном', () => {
    expect(phone.size).toBeGreaterThan(0);
    expect([...phone.keys()].sort()).toEqual(
      [...tablet.keys()].filter((k) => phone.has(k)).sort(),
    );
    expect(phone.size).toBe([...tablet.keys()].filter((k) => phone.has(k)).length);
  });

  it.each([...new Set([...phone.keys()])])('%s объявлен одинаково в обоих блоках', (selector) => {
    expect(tablet.get(selector), `правила ${selector} нет в блоке 899.98`).toBeDefined();
    expect(phone.get(selector)).toBe(tablet.get(selector));
  });

  it('карточные правила планшетного блока помечены исключением, иначе пометка не работает', () => {
    for (const selector of phone.keys()) {
      const raw = mediaBody('(max-width: 899.98px)');
      expect(raw).toContain(selector.replace('.rt-table', '.rt-table:not(.rt-keep-table)'));
    }
  });
});

describe('пометка стоит вместе с rt-table', () => {
  const VUE = [];
  (function walk(dir) {
    for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
      const full = path.join(dir, entry.name);
      if (entry.isDirectory()) walk(full);
      else if (entry.name.endsWith('.vue')) VUE.push(full);
    }
  })(path.resolve(SRC, '..'));

  it('нет списка с rt-keep-table без rt-table - иначе правила мимо', () => {
    const broken = [];
    for (const file of VUE) {
      const src = fs.readFileSync(file, 'utf8');
      for (const m of src.matchAll(/class="([^"]*rt-keep-table[^"]*)"/g)) {
        if (!/\brt-table\b/.test(m[1])) broken.push(`${path.basename(file)}: ${m[1]}`);
      }
    }
    expect(broken).toEqual([]);
  });
});
