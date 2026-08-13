import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, it, expect } from 'vitest';

const FORMS_CSS = readFileSync(resolve(__dirname, '../forms.css'), 'utf8');

/**
 * Высота базовой пилюли `.lk-button`.
 *
 * Раньше высота выходила из отступов и кегля и давала 34px - на два меньше нормы
 * тач-таргета проекта. Заметил это не глаз, а гейт мобильных инвариантов, и только
 * когда на экране показался блок ошибки загрузки: кнопка «Повторить» в обычной жизни
 * не видна, поэтому расхождение жило незамеченным. jsdom каскад не считает, поэтому
 * стережём само правило.
 */
describe('forms.css — тач-таргет базовой кнопки', () => {
  it('.lk-button держит минимум 36px по высоте', () => {
    const rule = FORMS_CSS.match(/\.lk-button\s*\{([^}]*)\}/);

    expect(rule, 'правило .lk-button не найдено').not.toBeNull();
    const minHeight = rule[1].match(/min-height:\s*(\d+)px/);
    expect(minHeight, `min-height не задан: ${rule[1]}`).not.toBeNull();
    expect(Number(minHeight[1])).toBeGreaterThanOrEqual(36);
  });
});
