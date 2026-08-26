import { describe, it, expect } from 'vitest';
import fs from 'node:fs';
import path from 'node:path';

/**
 * Замок на подпись «Подсказка» (TablesComponent.vue, талон проходной, волна 6/7).
 *
 * Владелец: «Не надо менять шрифт, цвет надписи "Подсказка"». Свёрнутое состояние
 * несло отдельный `.fact-hint-card--collapsed .fact-hint-card__toggle` с 13px/500 и
 * акцентным цветом против базовых 14px/600/`--hint-card-text` развёрнутого - подпись
 * визуально прыгала при сворачивании. jsdom каскад и медиазапросы не считает,
 * поэтому стережём исходник, а не поведение компонента.
 */
describe('Подпись "Подсказка" таблицы по факту - одинаковый вид в обоих состояниях', () => {
  const src = fs.readFileSync(
    path.resolve(__dirname, '../TablesComponent.vue'),
    'utf8',
  ).replace(/\/\*[\s\S]*?\*\//g, '');

  it('свёрнутое состояние не переопределяет цвет/кегль/вес переключателя', () => {
    // Правило существовало ровно для этого различия - его не должно остаться вовсе.
    expect(src).not.toMatch(/\.fact-hint-card--collapsed\s+\.fact-hint-card__toggle\s*\{/);
  });

  it('базовый переключатель задаёт вид один раз для обоих состояний', () => {
    const match = src.match(/\.fact-hint-card__toggle\s*\{([^}]*)\}/);
    expect(match).not.toBeNull();
    const decls = match[1];
    expect(decls).toMatch(/font-size:\s*14px/);
    expect(decls).toMatch(/font-weight:\s*600/);
    expect(decls).toMatch(/color:\s*var\(--hint-card-text\)/);
  });
});
