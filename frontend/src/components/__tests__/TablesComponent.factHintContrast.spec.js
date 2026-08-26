import { describe, it, expect } from 'vitest';
import fs from 'node:fs';
import path from 'node:path';

/**
 * Замок на контраст подписи «Подсказка» в СВЁРНУТОМ состоянии (мобилка, светлая
 * тема, правки w9-tab #1097).
 *
 * Фон свёрнутой карточки - `--accent-tint-solid` (лёгкий подмес акцента, 8% в
 * светлой теме поверх `--surface` - почти белый), а базовый `.fact-hint-card__toggle`
 * красит текст токеном `--hint-card-text`, рассчитанным на СПЛОШНУЮ акцентную
 * заливку развёрнутой карточки (в светлой теме `--hint-card-text` = `--accent-contrast`
 * = белый). На подмесе белый текст пропадал - «Подсказка» белым по белому.
 *
 * Чинили переопределением самой ПЕРЕМЕННОЙ `--hint-card-text` внутри
 * `.fact-hint-card--collapsed` (токеном `--accent-text`, читаемым на тинте в обеих
 * темах), а не хардкод-цветом и не отдельным правилом
 * `.fact-hint-card--collapsed .fact-hint-card__toggle { color: ... }` - последнее
 * запрещает соседний замок TablesComponent.factHint.spec.js (единый вид подписи в
 * обоих состояниях). jsdom каскад и color-mix не считает, поэтому стережём исходник.
 */
describe('Подпись "Подсказка" читается на свёрнутом (тинтованном) фоне', () => {
  const src = fs.readFileSync(
    path.resolve(__dirname, '../TablesComponent.vue'),
    'utf8',
  ).replace(/\/\*[\s\S]*?\*\//g, '');

  it('свёрнутое состояние переопределяет --hint-card-text ТОКЕНОМ, не хардкод-цветом', () => {
    const match = src.match(/\.fact-hint-card--collapsed\s*\{([^}]*)\}/);
    expect(match).not.toBeNull();
    const decls = match[1];
    expect(decls).toMatch(/--hint-card-text:\s*var\(--[a-z-]+\)/);
    expect(decls).not.toMatch(/--hint-card-text:\s*(#fff|#ffffff|white|rgba?\(\s*255)/i);
  });

  it('фикс не заводит отдельный color-оверрайд переключателя в свёрнутом состоянии', () => {
    // Единый вид (шрифт/вес/цвет) подписи между состояниями стережёт
    // TablesComponent.factHint.spec.js - здесь дублировать не нужно, но
    // убеждаемся, что фикс контраста не обошёл этот замок через прямой color.
    expect(src).not.toMatch(/\.fact-hint-card--collapsed\s+\.fact-hint-card__toggle\s*\{[^}]*color\s*:/);
  });
});
