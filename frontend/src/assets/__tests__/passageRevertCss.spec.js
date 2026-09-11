import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

/**
 * Замок на специфичность подсветки кнопки отмены (#2437).
 *
 * jsdom не применяет стили из отдельного css-файла, поэтому проверить цвет в тесте
 * компонента нечем - а именно цвет и подвёл на стенде: подпись менялась на
 * «Отменить», заливка оставалась зелёной. Причина в специфичности: у отмеченной
 * кнопки уже стоит `.action-btn.entry-btn.active` (три класса), и правило из двух
 * классов ей проигрывало.
 *
 * Поэтому сторожим сам селектор: он обязан называть направление, иначе счёт классов
 * снова окажется меньше.
 */
const css = readFileSync(resolve(process.cwd(), 'src/assets/passage.css'), 'utf8');
// Комментарии выкидываем: в них тоже встречаются имена классов, и разбор селекторов
// спотыкался бы о собственное объяснение правила.
const rules = css.replace(/\/\*[\s\S]*?\*\//g, '');

describe('passage.css - подсветка кнопки отмены', () => {
  it('правило .revertable перевешивает .active по специфичности', () => {
    const selectors = rules
      .split('}')
      .map((block) => block.split('{')[0])
      .filter((sel) => sel.includes('.revertable'));

    expect(selectors.length).toBeGreaterThan(0);
    selectors.forEach((sel) => {
      sel.split(',').map((s) => s.trim()).filter(Boolean).forEach((one) => {
        const classes = (one.match(/\.[a-z-]+/g) || []).length;
        expect(classes, `селектор "${one}" слабее .action-btn.entry-btn.active`).toBeGreaterThanOrEqual(3);
      });
    });
  });

  it('покрыты обе кнопки прохода', () => {
    expect(css).toContain('.entry-btn.revertable');
    expect(css).toContain('.exit-btn.revertable');
  });
});
