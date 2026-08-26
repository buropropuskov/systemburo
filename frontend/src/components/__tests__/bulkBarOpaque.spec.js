import { describe, it, expect } from 'vitest';
import { readdirSync, readFileSync } from 'node:fs';
import { join, resolve, relative } from 'node:path';

/**
 * Панель групповых операций - оверлей ПОВЕРХ шапки карточки (position: absolute,
 * не reflow, урок #510). Значит её фон обязан быть непрозрачным: на --accent-tint
 * (0.08 в светлой теме, 0.22 в тёмной) сквозь панель читалась закрытая ею шапка -
 * «Выбрано: 2» ложилось на «Номера автомобилей по заявке», кнопки на «Обновить».
 *
 * Правило размножено копипастой по всем справочникам и таблицам, поэтому проверка
 * идёт по группе, а не по одному компоненту: новый *Management.vue с групповыми
 * операциями обязан унаследовать непрозрачный фон.
 */

const srcDir = resolve(__dirname, '../..');
const tokensCss = readFileSync(join(srcDir, 'assets/tokens.css'), 'utf8');

/** Токены с альфой меньше единицы - фон на них просвечивает. */
function translucentTokens() {
  const found = new Set();
  for (const m of tokensCss.matchAll(/--([a-z0-9-]+):\s*([^;]+);/g)) {
    if (/transparent/.test(m[2]) || /rgba\([^)]*,\s*0?\.\d+\s*\)/.test(m[2])) {
      found.add(`--${m[1]}`);
    }
  }
  return found;
}

/** Правила .bulk-bar, задающие фон (мобильные @media правят только раскладку). */
function bulkBarRules() {
  const rules = [];
  for (const entry of readdirSync(srcDir, { recursive: true, withFileTypes: true })) {
    if (!entry.isFile() || !entry.name.endsWith('.vue')) continue;
    const file = join(entry.parentPath ?? entry.path, entry.name);
    const text = readFileSync(file, 'utf8');
    for (const rule of text.matchAll(/\.bulk-bar\s*\{([^}]*)\}/g)) {
      const bg = rule[1].match(/background(?:-color)?:\s*([^;]+)/);
      if (bg) rules.push({ file: relative(srcDir, file), background: bg[1].trim() });
    }
  }
  return rules;
}

describe('панель групповых операций непрозрачна', () => {
  const translucent = translucentTokens();
  const rules = bulkBarRules();

  // Паттерн живёт в справочниках, таблицах проходной, конструкторе таблиц и
  // пользователях: если счётчик просел, правило где-то потеряли, а не «стало чище».
  it('находит панель во всех компонентах с групповыми операциями', () => {
    expect(rules.length, `нашёл только: ${rules.map(r => r.file).join(', ')}`).toBeGreaterThanOrEqual(10);
  });

  it('ни одна панель не сидит на просвечивающем фоне', () => {
    const bad = rules.filter(({ background }) => {
      if (/transparent/.test(background) || /rgba\([^)]*,\s*0?\.\d+\s*\)/.test(background)) return true;
      return [...background.matchAll(/var\((--[a-z0-9-]+)/g)].some(m => translucent.has(m[1]));
    });
    expect(
      bad.map(b => `${b.file}: ${b.background}`),
      'сквозь эти панели будет видно шапку карточки',
    ).toEqual([]);
  });
});
