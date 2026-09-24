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
 * списков останется с прежними отступами. Поэтому сверяем тела правил в ОБЕ
 * стороны: и «есть в телефонном, нет в планшетном», и наоборот.
 */

const SRC = path.resolve(__dirname, '..');
const CSS = fs.readFileSync(path.join(SRC, 'responsive-tables.css'), 'utf8');

/** Тела ВСЕХ блоков `@media <query>` со сбалансированным подсчётом скобок. */
function mediaBody(query) {
  const mark = `@media ${query} {`;
  const bodies = [];
  let from = 0;
  for (;;) {
    const start = CSS.indexOf(mark, from);
    if (start === -1) break;
    let i = CSS.indexOf('{', start) + 1;
    let depth = 1;
    const bodyStart = i;
    while (depth > 0 && i < CSS.length) {
      if (CSS[i] === '{') depth += 1;
      else if (CSS[i] === '}') depth -= 1;
      i += 1;
    }
    bodies.push(CSS.slice(bodyStart, i - 1));
    from = i;
  }
  expect(bodies.length, `нет блока @media ${query}`).toBeGreaterThan(0);
  return bodies.join('\n');
}

/**
 * Часть 1 планшетного блока - только она имеет копию. Части 2 и 3 (инлайн-кнопки
 * шапки, талон проходной) живут в том же блоке, но копии не имеют и в сверку идти
 * не должны, иначе замок пришлось бы ослаблять фильтром - а фильтр заодно
 * пропускал бы и настоящие расхождения.
 */
function part1(body) {
  const from = body.indexOf('/* --- Часть 1');
  const to = body.indexOf('/* --- Часть 3');
  expect(from, 'в блоке 899.98 нет части 1').toBeGreaterThan(-1);
  expect(to, 'в блоке 899.98 нет части 3').toBeGreaterThan(from);
  return body.slice(from, to);
}

/** Правила как карта «селектор без пометки -> тело», без комментариев и пробелов. */
function rules(body) {
  const clean = body.replace(/\/\*[\s\S]*?\*\//g, '');
  const out = new Map();
  const re = /([^{}]+)\{([^{}]*)\}/g;
  let m;
  while ((m = re.exec(clean)) !== null) {
    const selector = m[1].trim()
      .replace(/:where\(:not\(\.rt-keep-table\)\)/g, '')
      .replace(/:where\(\.rt-keep-table\)/g, '')
      .replace(/\s+/g, ' ');
    if (!selector.startsWith('.rt-table')) continue;
    out.set(selector, m[2].replace(/\s+/g, ' ').trim());
  }
  return out;
}

const tablet = rules(part1(mediaBody('(max-width: 899.98px)')));
const phone = rules(mediaBody('(max-width: 767.98px)'));

describe('карточный слой: две копии правил не расходятся', () => {
  it('набор селекторов совпадает в обе стороны', () => {
    expect(tablet.size).toBeGreaterThan(0);
    expect([...phone.keys()].sort()).toEqual([...tablet.keys()].sort());
  });

  it.each([...tablet.keys()])('%s объявлен одинаково в обоих блоках', (selector) => {
    expect(phone.get(selector), `правила ${selector} нет в телефонном блоке`).toBeDefined();
    expect(phone.get(selector)).toBe(tablet.get(selector));
  });
});

describe('пометка не добавляет специфичности', () => {
  /*
   * `.rt-table:not(.rt-keep-table) .rt-row` весит (0,3,0) против прежних (0,2,0)
   * и начинает выигрывать у scoped-правил компонентов, которым раньше проигрывал
   * (глобальный index.css грузится после чанка компонента). На окнах выбора
   * существующих это схлопывало их собственный «чип»-дизайн строки до общей
   * карточки, и текст наезжал на абсолютные чекбокс и бейдж статуса.
   */
  it('обе формы пометки завёрнуты в :where()', () => {
    const code = CSS.replace(/\/\*[\s\S]*?\*\//g, '');
    const bare = [...code.matchAll(/^[^\n{]*\.rt-keep-table[^\n{]*\{/gm)]
      .map((m) => m[0].trim())
      .filter((selector) => !/:where\(/.test(selector));
    expect(bare).toEqual([]);
  });

  it('в селекторах карточного слоя нет пометки вне :where()', () => {
    // Комментарии выбрасываем: в пояснении рядом с правилом как раз приведён
    // запрещённый вид селектора, и без чистки замок падал бы на объяснении.
    const code = CSS.replace(/\/\*[\s\S]*?\*\//g, '');
    expect(code).not.toMatch(/\.rt-table:not\(\.rt-keep-table\)/);
    expect(code).not.toMatch(/\.rt-table\.rt-keep-table/);
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
