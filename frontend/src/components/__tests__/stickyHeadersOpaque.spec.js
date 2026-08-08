import { describe, it, expect } from 'vitest';
import { readdirSync, readFileSync } from 'node:fs';
import { join, resolve, relative } from 'node:path';

/**
 * Закреплённая шапка обязана быть непрозрачной: строки уезжают ПОД неё, и на
 * полупрозрачном фоне читаются сквозь заголовки. В светлой теме это почти не видно
 * (--accent-tint там rgba(...,0.08)), а в тёмной альфа 0.22 - и таблица истории
 * входов показывала «сквозь шапку видно строки».
 *
 * Проверка идёт по исходникам, а не по отрисовке: scoped-CSS в jsdom не применяется,
 * и любой mount-тест на такую регрессию слеп.
 */

const srcDir = resolve(__dirname, '../..');
const tokensCss = readFileSync(join(srcDir, 'assets/tokens.css'), 'utf8');

/** Осознанные исключения: фон намеренно полупрозрачный и подпёрт размытием. */
const ALLOWED = [
  // Закреплённый заголовок вложения на мобилке - «стекло»: контент под ним размыт
  // backdrop-filter, а не читается.
  '.create__blank-sticky',
];

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

function stickyRules() {
  const rules = [];
  for (const entry of readdirSync(srcDir, { recursive: true, withFileTypes: true })) {
    if (!entry.isFile() || !entry.name.endsWith('.vue')) continue;
    const file = join(entry.parentPath ?? entry.path, entry.name);
    const text = readFileSync(file, 'utf8');
    for (const style of text.matchAll(/<style[^>]*>([\s\S]*?)<\/style>/g)) {
      for (const rule of style[1].matchAll(/([^{}]+)\{([^{}]*)\}/g)) {
        if (!/position:\s*sticky/.test(rule[2])) continue;
        rules.push({
          file: relative(srcDir, file),
          selector: rule[1].replace(/\/\*[\s\S]*?\*\//g, '').trim().replace(/\s+/g, ' '),
          body: rule[2],
        });
      }
    }
  }
  return rules;
}

describe('закреплённые шапки непрозрачны', () => {
  const translucent = translucentTokens();
  const rules = stickyRules().filter(r => !ALLOWED.some(a => r.selector.includes(a)));

  it('находит sticky-правила для проверки', () => {
    expect(rules.length).toBeGreaterThan(5);
  });

  it('ни одна закреплённая шапка не сидит на просвечивающем фоне', () => {
    const bad = [];
    for (const rule of rules) {
      const bg = rule.body.match(/background(?:-color)?:\s*([^;]+)/);
      if (!bg) {
        bad.push(`${rule.file} ${rule.selector}: фон не задан`);
        continue;
      }
      const value = bg[1];
      if (/transparent/.test(value) || /rgba\([^)]*,\s*0?\.\d+\s*\)/.test(value)) {
        bad.push(`${rule.file} ${rule.selector}: ${value.trim()}`);
        continue;
      }
      const tokens = [...value.matchAll(/var\((--[a-z0-9-]+)/g)].map(m => m[1]);
      const leaky = tokens.filter(t => translucent.has(t));
      if (leaky.length) bad.push(`${rule.file} ${rule.selector}: ${leaky.join(', ')}`);
    }
    expect(bad, `сквозь эти шапки будет видно строки:\n${bad.join('\n')}`).toEqual([]);
  });
});
