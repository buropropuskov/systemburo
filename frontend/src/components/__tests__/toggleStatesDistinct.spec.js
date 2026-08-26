import { describe, it, expect } from 'vitest';
import { readdirSync, readFileSync } from 'node:fs';
import { join, resolve, relative } from 'node:path';

/**
 * Тумблер обязан выглядеть по-разному включённым и выключенным (#1877).
 *
 * У «Обязательного согласования» в настройке организаций дорожка была залита
 * `var(--accent)` и в базовом правиле, и в правиле `:checked` - тумблер всегда
 * синий, состояние читается только по положению кружка. Разметка при этом
 * исправна, v-model работает, компонент рендерится: ни jsdom-спека, ни линтер
 * такого не видят, дефект живёт исключительно в CSS.
 *
 * Тумблеры на проекте размножены копипастой (девять штук, четыре разных схемы
 * именования), поэтому проверка идёт по группе: дорожки находятся сканом
 * `<style>`-блоков, а не перечнем файлов - новый тумблер попадает под замок сам.
 *
 * Значения сравниваются и как написаны, и как резолвятся по tokens.css: два
 * разных имени токена могут указывать на один цвет (в светлой палитре
 * --surface-2 и --surface-sunken совпадают), и такая пара тоже неразличима.
 */

const srcDir = resolve(__dirname, '../..');

/** Дорожка тумблера: класс, по которому её опознаём в селекторе. */
const TRACK_NAME = /slider|switch|toggle|track/i;
/** Признак «включено» в селекторе - псевдокласс инпута или класс-модификатор. */
const CHECKED_STATE = /:checked|\.checked\b|\.active\b|--active\b|\[aria-checked=(['"]?)true\1\]/;
/** Классы-состояния сами дорожкой не являются. */
const STATE_CLASS = /^(checked|active|on|selected)$/;

const stripComments = (css) => css.replace(/\/\*[\s\S]*?\*\//g, '');

/** Переменные базовой палитры и слоя legacy-алиасов (--color-* -> --accent и т.п.). */
function baseVars() {
  const tokens = stripComments(readFileSync(join(srcDir, 'assets/tokens.css'), 'utf8'));
  const vars = {};
  for (const block of tokens.matchAll(/(\[data-theme(?:="light")?\])\s*\{([\s\S]*?)\n\}/g)) {
    for (const m of block[2].matchAll(/(--[\w-]+)\s*:\s*([^;]+);/g)) vars[m[1]] = m[2].trim();
  }
  return vars;
}

/**
 * Раскрывает var() до литерала. null, если что-то не разрешилось - тогда
 * сравнение остаётся текстовым, а не молча объявляет значения разными.
 */
function resolveVars(value, vars) {
  let out = value;
  for (let i = 0; i < 10 && out.includes('var('); i += 1) {
    const next = out.replace(/var\(\s*(--[\w-]+)\s*(?:,\s*([^()]*?)\s*)?\)/g, (all, name, fallback) => {
      if (vars[name] !== undefined) return vars[name];
      return fallback !== undefined ? fallback : all;
    });
    if (next === out) return null;
    out = next;
  }
  return out.includes('var(') ? null : out.trim().toLowerCase();
}

/** Все правила из <style>-блоков файла: селектор + объявления. */
function cssRules(source) {
  const rules = [];
  for (const style of source.matchAll(/<style[^>]*>([\s\S]*?)<\/style>/g)) {
    for (const rule of stripComments(style[1]).matchAll(/([^{}@;]+)\{([^{}]*)\}/g)) {
      rules.push({ selector: rule[1].trim().replace(/\s+/g, ' '), body: rule[2] });
    }
  }
  return rules;
}

const backgroundOf = (body) => {
  const m = [...body.matchAll(/background(?:-color)?\s*:\s*([^;}]+)/g)].pop();
  return m ? m[1].trim() : null;
};

/** Имя класса дорожки из последнего звена селектора («.switch input:checked + .slider» -> slider). */
function trackClass(selector) {
  if (/::?(before|after)\b/.test(selector)) return null;
  const last = selector.split(/[\s>+~]+/).filter(Boolean).pop() ?? '';
  const classes = [...last.matchAll(/\.([\w-]+)/g)]
    .map((m) => m[1])
    .filter((name) => !STATE_CLASS.test(name) && TRACK_NAME.test(name));
  return classes.pop() ?? null;
}

/** Пары «выключено / включено» по всем SFC. */
function togglePairs() {
  const pairs = [];
  const orphans = [];
  for (const entry of readdirSync(srcDir, { recursive: true, withFileTypes: true })) {
    if (!entry.isFile() || !entry.name.endsWith('.vue')) continue;
    const file = join(entry.parentPath ?? entry.path, entry.name);
    const rules = cssRules(readFileSync(file, 'utf8'));
    for (const rule of rules) {
      if (!CHECKED_STATE.test(rule.selector)) continue;
      const on = backgroundOf(rule.body);
      if (!on) continue;
      const track = trackClass(rule.selector);
      if (!track) continue;
      const base = rules.find((r) => r.selector === `.${track}` && backgroundOf(r.body));
      const where = `${relative(srcDir, file)} .${track}`;
      if (!base) {
        orphans.push(`${where}: есть заливка включённого состояния, а у самой дорожки фона нет`);
        continue;
      }
      pairs.push({ where, off: backgroundOf(base.body), on, onSelector: rule.selector });
    }
  }
  return { pairs, orphans };
}

describe('тумблеры различают включённое и выключенное состояние', () => {
  const vars = baseVars();
  const { pairs, orphans } = togglePairs();

  // Тумблеры лежат в справочниках, карточке заявки, расписании, окнах
  // предупреждений, подаче заявки, Центре и двух общих ui-компонентах.
  // Просевший счётчик значит «дорожку переименовали и вывели из-под замка».
  it('находит дорожки всех тумблеров проекта', () => {
    expect(pairs.length, `нашёл только: ${pairs.map((p) => p.where).join(', ')}`).toBeGreaterThanOrEqual(8);
  });

  it('у каждого включённого состояния есть заливка выключенного', () => {
    expect(orphans).toEqual([]);
  });

  it('ни один тумблер не залит одним цветом в обоих состояниях', () => {
    const same = pairs.filter(({ off, on }) => {
      if (off.toLowerCase() === on.toLowerCase()) return true;
      const [a, b] = [resolveVars(off, vars), resolveVars(on, vars)];
      return a !== null && a === b;
    });
    expect(
      same.map((p) => `${p.where}: ${p.off} = ${p.on} (${p.onSelector})`),
      'состояние тумблера будет читаться только по положению кружка',
    ).toEqual([]);
  });
});
